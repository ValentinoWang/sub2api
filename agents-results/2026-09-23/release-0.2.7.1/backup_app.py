"""Usage: backup_app.py DEPLOY_DIRECTORY NEW_PRIVATE_BACKUP_DIRECTORY CANDIDATE ENVIRONMENT"""
import gzip
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tarfile
import time

from release_common import candidate, require

MIGRATIONS_SQL = "SELECT COALESCE(json_agg(json_build_object('version',filename,'checksum',checksum) ORDER BY filename),'[]'::json) FROM schema_migrations"


def read_migrations():
    return json.loads(run(['docker', 'exec', 'sub2api-postgres', 'sh', '-c',
                           'exec psql -X -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -At -c "$1"',
                           'sh', MIGRATIONS_SQL]))


def run(args):
    return subprocess.check_output(args, stderr=subprocess.PIPE)


def sha(path):
    digest = hashlib.sha256()
    with path.open('rb') as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b''):
            digest.update(chunk)
    return digest.hexdigest()


def main():
    require(len(sys.argv) == 5, 'Expected deployment, new private backup, candidate and environment')
    candidate_path = Path(sys.argv[3]).resolve()
    candidate(candidate_path)
    environment = sys.argv[4]
    require(environment in ('dev', 'prod'), 'Invalid environment')
    started_at = time.time()
    deploy = Path(sys.argv[1]).expanduser().resolve()
    backup = Path(sys.argv[2]).expanduser().resolve()
    os.umask(0o077)
    os.chdir(deploy)
    if backup.exists():
        raise RuntimeError('Backup directory already exists; select a new directory')
    if not (deploy / '.env').is_file() or not (deploy / 'docker-compose.local.yml').is_file():
        raise RuntimeError('Required deployment files are missing')
    backup.mkdir(parents=True, mode=0o700)
    app_bytes = run(['docker', 'inspect', 'sub2api'])
    app = json.loads(app_bytes)[0]
    if app['State']['Status'] != 'running':
        raise RuntimeError('Application is not running')
    if len([m for m in app['Mounts'] if m['Destination'] == '/app/data']) != 1:
        raise RuntimeError('Expected exactly one application data mount')
    migrations_before = read_migrations()
    (backup / 'migrations.json').write_text(json.dumps(migrations_before) + '\n')
    (backup / 'app-inspect.json').write_bytes(app_bytes)
    (backup / 'compose-resolved.json').write_bytes(run([
        'docker', 'compose', '-f', 'docker-compose.local.yml', 'config', '--format', 'json']))
    config_files = sorted({deploy / '.env', *deploy.glob('docker-compose*.yml'),
                           *deploy.glob('docker-compose*.yaml')})
    with tarfile.open(backup / 'compose-config.tar.gz', 'w:gz') as archive:
        for path in config_files:
            if path.is_symlink() or not path.is_file():
                raise RuntimeError('Deployment configuration must be a regular file')
            archive.add(path, arcname=path.name, recursive=False)
    with (backup / 'db.dump').open('wb') as output:
        subprocess.run(['docker', 'exec', 'sub2api-postgres', 'sh', '-c',
                        'exec pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc'],
                       stdout=output, stderr=subprocess.PIPE, check=True)
    with (backup / 'db.dump').open('rb') as source:
        toc = subprocess.run(['docker', 'exec', '-i', 'sub2api-postgres', 'pg_restore', '-l'],
                             stdin=source, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                             check=True).stdout
    (backup / 'db-toc.txt').write_bytes(toc)
    toc_entries = sum(bool(line.strip()) and not line.startswith(b';') for line in toc.splitlines())
    if toc_entries == 0:
        raise RuntimeError('Database archive has no table-of-contents entries')
    with (backup / 'app-data.tar.gz').open('wb') as output:
        subprocess.run(['docker', 'exec', 'sub2api', 'tar', '-czf', '-', '-C', '/app/data', '.'],
                       stdout=output, stderr=subprocess.PIPE, check=True)
    archive_counts = {}
    for name in ('compose-config.tar.gz', 'app-data.tar.gz'):
        with gzip.open(backup / name, 'rb') as stream:
            while stream.read(1024 * 1024):
                pass
        with tarfile.open(backup / name, 'r:gz') as archive:
            archive_counts[name] = sum(1 for _ in archive)
        if archive_counts[name] == 0:
            raise RuntimeError('Empty backup archive')
    after = json.loads(run(['docker', 'inspect', 'sub2api']))[0]
    if any(after[key] != app[key] for key in ('Id', 'Image', 'Mounts')):
        raise RuntimeError('Application changed during backup')
    require(read_migrations() == migrations_before, 'Migration history changed during backup')
    files = {p.name: {'sha256': sha(p), 'bytes': p.stat().st_size}
             for p in sorted(backup.iterdir()) if p.is_file()}
    manifest = {'schema_version': 2, 'candidate_sha256': sha(candidate_path),
                'environment': environment, 'deploy_directory': str(deploy),
                'created_at': time.time(), 'started_at': started_at,
                'old_image_id': app['Image'], 'files': files, 'file_count': len(files), 'toc_entries': toc_entries,
                'archive_entries': archive_counts}
    (backup / 'manifest.json').write_text(json.dumps(manifest, indent=2) + '\n')
    print(json.dumps(manifest))


if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        # Subprocess diagnostics and archive contents can contain private values.
        print(json.dumps({'status': 'BACKUP_FAILED', 'error_type': type(error).__name__}), file=sys.stderr)
        sys.exit(1)
