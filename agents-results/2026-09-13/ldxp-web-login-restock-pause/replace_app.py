"""Usage: python3 - DEPLOY_DIRECTORY IMAGE OLD_IMAGE_ID BACKUP_DIRECTORY < replace_app.py"""
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tarfile

EXPECTED_COMMIT = 'e77fce24d8e94f826e604e1c0895e90de93cd4ba'
EXPECTED_VERSION = '0.2.4.2'


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
    deploy_directory, release_image, expected_old_image_id, backup_directory = sys.argv[1:]
    os.umask(0o077)
    backup = Path(backup_directory).expanduser().resolve()
    os.chdir(Path(deploy_directory).expanduser().resolve())
    manifest = json.loads((backup / 'manifest.json').read_text())
    required = {'app-inspect.json', 'compose-resolved.json', 'compose-config.tar.gz',
                'db.dump', 'db-toc.txt', 'app-data.tar.gz'}
    require(set(manifest['files']) == required and manifest['toc_entries'] > 0,
            'Incomplete verified backup')
    for name, record in manifest['files'].items():
        path = backup / name
        require(path.is_file() and not path.is_symlink(), 'Backup file is unavailable')
        require(path.stat().st_size == record['bytes'] and sha(path) == record['sha256'],
                'Backup integrity check failed')
    before = json.loads(run(['docker', 'inspect', 'sub2api']))[0]
    backed_up = json.loads((backup / 'app-inspect.json').read_text())[0]
    require(before['Image'] == expected_old_image_id == backed_up['Image'],
            'Current image changed since backup or inventory')
    require(invariants(before) == invariants(backed_up), 'Runtime changed since backup')
    require(before['State']['Status'] == 'running', 'Current application is not running')
    run(['docker', 'image', 'inspect', expected_old_image_id])
    candidate = json.loads(run(['docker', 'image', 'inspect', release_image]))[0]
    labels = candidate['Config']['Labels']
    require(labels.get('org.opencontainers.image.revision') == EXPECTED_COMMIT,
            'Candidate commit mismatch')
    require(labels.get('org.opencontainers.image.version') == EXPECTED_VERSION,
            'Candidate version mismatch')
    require(candidate['Architecture'] == 'amd64' and candidate['Os'] == 'linux',
            'Candidate platform mismatch')
    command = ['docker', 'compose', '-f', 'docker-compose.local.yml']
    current_config = json.loads(run(command + ['config', '--format', 'json']))
    require(current_config == json.loads((backup / 'compose-resolved.json').read_text()),
            'Compose configuration changed since backup')
    previous_env = dict(item.split('=', 1) for item in before['Config']['Env'] if '=' in item)
    require(all(previous_env.get(key) == str(value) for key, value in
                current_config['services']['sub2api'].get('environment', {}).items()),
            'Compose environment differs from running application')
    env_path = Path('.env')
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
        print(json.dumps({'status': 'RECREATED', 'image_id': after['Image'],
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
                              'rollback_runtime_verified': restored_ok}))
        raise


if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        print(json.dumps({'status': 'REPLACEMENT_FAILED', 'error_type': type(error).__name__}), file=sys.stderr)
        sys.exit(1)
