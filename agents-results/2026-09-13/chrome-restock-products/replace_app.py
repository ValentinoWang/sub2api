"""Usage: replace_app.py DEPLOY_DIRECTORY CANDIDATE OLD_IMAGE_ID BACKUP ENVIRONMENT MAINTENANCE_RECEIPT ROLLBACK_PROOF"""
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tarfile
import time

from release_common import candidate as load_candidate, receipt, verify_migrations
from backup_app import read_migrations



def run(args, env=None):
    return subprocess.check_output(args, stderr=subprocess.PIPE, env=env, text=True)


def sha(path):
    digest = hashlib.sha256()
    with path.open('rb') as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b''):
            digest.update(chunk)
    return digest.hexdigest()


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


def invariants(app):
    return {'env': sorted(app['Config']['Env']),
            'mounts': sorted([{key: m.get(key) for key in
                              ('Type', 'Source', 'Destination', 'RW', 'Propagation')}
                             for m in app['Mounts']], key=lambda m: m['Destination']),
            'ports': app['HostConfig']['PortBindings']}


def main():
    deploy_directory, candidate_file, expected_old_image_id, backup_directory, environment, notice_file, rollback_file = sys.argv[1:]
    candidate_path = Path(candidate_file).resolve()
    release = load_candidate(candidate_path)
    release_image = release['image']
    notice = receipt(notice_file, candidate_path, environment, 'MAINTENANCE_ACTIVE')
    require(type(notice['maintenance_id']) is int and notice['maintenance_id'] > 0,
            'Successful maintenance announcement ID required')
    proof = json.loads(Path(rollback_file).read_text())
    require(proof.get('candidate_sha256') == sha(candidate_path) and
            proof.get('rollback_image_id') == expected_old_image_id and
            proof.get('application_rollback_on_migrated_database_passed') is True,
            'Candidate-bound migrated database rollback proof required')
    os.umask(0o077)
    backup = Path(backup_directory).expanduser().resolve()
    os.chdir(Path(deploy_directory).expanduser().resolve())
    manifest = json.loads((backup / 'manifest.json').read_text())
    required = {'app-inspect.json', 'compose-resolved.json', 'compose-config.tar.gz',
                'db.dump', 'db-toc.txt', 'app-data.tar.gz', 'migrations.json'}
    require(set(manifest['files']) == required and manifest['toc_entries'] > 0,
            'Incomplete verified backup')
    require(manifest['schema_version'] == 2 and manifest['candidate_sha256'] == sha(candidate_path),
            'Backup candidate mismatch')
    require(manifest['environment'] == environment and manifest['deploy_directory'] == str(Path.cwd()),
            'Backup target mismatch')
    require(0 <= time.time() - manifest['started_at'] <= 1800, 'Fresh backup required within 30 minutes')
    for name, record in manifest['files'].items():
        path = backup / name
        require(path.is_file() and not path.is_symlink(), 'Backup file is unavailable')
        require(path.stat().st_size == record['bytes'] and sha(path) == record['sha256'],
                'Backup integrity check failed')
    before = json.loads(run(['docker', 'inspect', 'sub2api']))[0]
    backed_up = json.loads((backup / 'app-inspect.json').read_text())[0]
    require(before['Id'] == backed_up['Id'], 'Application changed since backup')
    require(before['Image'] == expected_old_image_id == backed_up['Image'],
            'Current image changed since backup or inventory')
    require(invariants(before) == invariants(backed_up), 'Runtime changed since backup')
    require(before['State']['Status'] == 'running', 'Current application is not running')
    run(['docker', 'image', 'inspect', expected_old_image_id])
    candidate = json.loads(run(['docker', 'image', 'inspect', release_image]))[0]
    require(candidate['Id'] == release['image_id'], 'Candidate image ID mismatch')
    labels = candidate['Config']['Labels']
    require(labels.get('org.opencontainers.image.revision') == release['commit'],
            'Candidate commit mismatch')
    require(labels.get('org.opencontainers.image.version') == release['version'],
            'Candidate version mismatch')
    require(candidate['Architecture'] == 'amd64' and candidate['Os'] == 'linux',
            'Candidate platform mismatch')
    previous_migrations = json.loads((backup / 'migrations.json').read_text())
    require(read_migrations() == previous_migrations, 'Migrations changed since backup')
    verify_migrations(release, previous_migrations, previous_migrations + [
        {'version': name, 'checksum': checksum} for name, checksum in release['added_migrations'].items()])
    command = ['docker', 'compose', '-f', 'docker-compose.local.yml']
    current_config = json.loads(run(command + ['config', '--format', 'json']))
    require(current_config == json.loads((backup / 'compose-resolved.json').read_text()),
            'Compose configuration changed since backup')
    previous_env = dict(item.split('=', 1) for item in before['Config']['Env'] if '=' in item)
    require(all(previous_env.get(key) == str(value) for key, value in
                current_config['services']['sub2api'].get('environment', {}).items()),
            'Compose environment differs from running application')
    env_path = Path('.env')
    require(env_path.is_file() and not env_path.is_symlink(), 'Environment must be a regular file')
    original = env_path.read_bytes()
    with tarfile.open(backup / 'compose-config.tar.gz', 'r:gz') as archive:
        require(archive.extractfile('.env').read() == original, 'Environment changed since backup')
    text = original.decode()
    pattern = r'(?m)^SUB2API_IMAGE=.*$'
    require(len(re.findall(pattern, text)) <= 1, 'Ambiguous image configuration')
    require('\n' not in release_image and '\r' not in release_image, 'Invalid image reference')
    updated = (re.sub(pattern, lambda _: 'SUB2API_IMAGE=' + release_image, text)
               if re.search(pattern, text) else text.rstrip('\n') + '\nSUB2API_IMAGE=' + release_image + '\n')
    started = False
    try:
        env_path.write_text(updated)
        env_path.chmod(0o600)
        next_config = json.loads(run(command + ['config', '--format', 'json']))
        require(next_config['services']['sub2api']['image'] == release_image,
                'Compose image override prevents deployment')
        expected_config = json.loads(json.dumps(current_config))
        expected_config['services']['sub2api']['image'] = release_image
        require(next_config == expected_config, 'Compose configuration drift')
        started = True
        run(command + ['up', '-d', '--no-deps', '--pull', 'never', '--force-recreate', 'sub2api'])
        after = json.loads(run(['docker', 'inspect', 'sub2api']))[0]
        require(after['Image'] == candidate['Id'], 'Container image mismatch')
        require(invariants(after) == invariants(before), 'Runtime environment, mounts or ports changed')
        require(after['State']['Status'] == 'running', 'New application is not running')
        deadline = time.monotonic() + 120
        while after['State'].get('Health', {}).get('Status') == 'starting' and time.monotonic() < deadline:
            time.sleep(2)
            after = json.loads(run(['docker', 'inspect', 'sub2api']))[0]
        require(after['State'].get('Health', {}).get('Status') == 'healthy', 'Candidate health failed')
        verify_migrations(release, previous_migrations, read_migrations())
        print(json.dumps({'status': 'RECREATED', 'candidate_sha256': sha(candidate_path),
                          'environment': environment, 'maintenance_id': notice['maintenance_id'],
                          'migrations_verified': True, 'image_id': after['Image'],
                          'state': after['State']['Status'],
                          'health': after['State'].get('Health', {}).get('Status'),
                          'runtime_invariants_preserved': True}))
    except Exception:
        env_path.write_bytes(original)
        env_path.chmod(0o600)
        if started:
            rollback_env = dict(os.environ, SUB2API_IMAGE=expected_old_image_id)
            result = subprocess.run(command + ['up', '-d', '--no-deps', '--pull', 'never',
                                               '--force-recreate', 'sub2api'], env=rollback_env,
                                    stdout=subprocess.PIPE, stderr=subprocess.PIPE)
            restored = json.loads(run(['docker', 'inspect', 'sub2api']))[0]
            restored_ok = (result.returncode == 0 and restored['Image'] == expected_old_image_id
                           and invariants(restored) == invariants(before)
                           and restored['State']['Status'] == 'running')
            print(json.dumps({'status': 'FAILED', 'rollback_command_exit': result.returncode,
                              'rollback_runtime_verified': restored_ok,
                              'database_restored': False, 'maintenance_requires_followup': True}))
        raise


if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        print(json.dumps({'status': 'REPLACEMENT_FAILED', 'error_type': type(error).__name__}), file=sys.stderr)
        sys.exit(1)
