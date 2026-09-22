"""Restore only the backed-up application after post-cutover verification fails.

Usage: rollback_app.py DEPLOY_DIRECTORY CANDIDATE BACKUP ENVIRONMENT MAINTENANCE_RECEIPT ROLLBACK_PROOF
"""
import json
import os
from pathlib import Path
import sys
import tarfile
import time

from backup_app import read_migrations
from release_common import candidate, receipt, require, verify_migrations
from replace_app import invariants, run, sha


def main():
    deploy, candidate_file, backup_directory, environment, notice_file, proof_file = sys.argv[1:]
    candidate_path = Path(candidate_file).resolve()
    release = candidate(candidate_path)
    receipt(notice_file, candidate_path, environment, 'MAINTENANCE_ACTIVE')
    backup = Path(backup_directory).resolve()
    manifest = json.loads((backup / 'manifest.json').read_text())
    proof = json.loads(Path(proof_file).read_text())
    require(manifest['candidate_sha256'] == sha(candidate_path) and manifest['environment'] == environment,
            'Backup release mismatch')
    require(proof.get('candidate_sha256') == sha(candidate_path) and
            proof.get('rollback_image_id') == manifest['old_image_id'] and
            proof.get('application_rollback_on_migrated_database_passed') is True,
            'Migrated database rollback proof required')
    for name, record in manifest['files'].items():
        require(Path(name).name == name, 'Invalid backup member')
        path = backup / name
        require(path.is_file() and not path.is_symlink() and sha(path) == record['sha256'] and
                path.stat().st_size == record['bytes'], 'Backup integrity failed')
    os.chdir(Path(deploy).resolve())
    require(manifest['deploy_directory'] == str(Path.cwd()), 'Wrong deployment directory')
    saved = json.loads((backup / 'app-inspect.json').read_text())[0]
    current = json.loads(run(['docker', 'inspect', 'sub2api']))[0]
    require(current['Image'] == release['image_id'] and invariants(current) == invariants(saved),
            'Current runtime changed after release')
    verify_migrations(release, json.loads((backup / 'migrations.json').read_text()), read_migrations())
    command = ['docker', 'compose', '-f', 'docker-compose.local.yml']
    previous_config = json.loads((backup / 'compose-resolved.json').read_text())
    expected_config = json.loads(json.dumps(previous_config))
    expected_config['services']['sub2api']['image'] = release['image']
    require(json.loads(run(command + ['config', '--format', 'json'])) == expected_config,
            'Compose changed since cutover')
    run(['docker', 'image', 'inspect', manifest['old_image_id']])
    with tarfile.open(backup / 'compose-config.tar.gz', 'r:gz') as archive:
        original = archive.extractfile('.env').read()
    env_path = Path('.env')
    require(not env_path.is_symlink(), 'Environment must be a regular file')
    env_path.write_bytes(original)
    env_path.chmod(0o600)
    require(json.loads(run(command + ['config', '--format', 'json'])) == previous_config,
            'Original compose restoration failed')
    rollback_env = dict(os.environ, SUB2API_IMAGE=manifest['old_image_id'])
    run(command + ['up', '-d', '--no-deps', '--pull', 'never', '--force-recreate', 'sub2api'], env=rollback_env)
    deadline = time.monotonic() + 120
    while True:
        restored = json.loads(run(['docker', 'inspect', 'sub2api']))[0]
        health = restored['State'].get('Health', {}).get('Status')
        if health != 'starting' or time.monotonic() >= deadline:
            break
        time.sleep(2)
    require(restored['Image'] == saved['Image'] and invariants(restored) == invariants(saved) and
            restored['State']['Status'] == 'running' and health == 'healthy', 'Rollback verification failed')
    print(json.dumps({'status': 'APPLICATION_ROLLED_BACK', 'environment': environment,
                      'image_id': restored['Image'], 'database_restored': False,
                      'functional_acceptance_still_required': True, 'maintenance_requires_followup': True}))


if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        print(json.dumps({'status': 'ROLLBACK_FAILED', 'error_type': type(error).__name__,
                          'maintenance_requires_followup': True}), file=sys.stderr)
        sys.exit(1)
