#!/usr/bin/env python3
"""Run local native costing with a private cost database and an unchanged read-only source."""
import argparse
import json
import os
from pathlib import Path
import secrets
import subprocess
import time
from urllib.parse import quote, urlencode

ROOT = Path(__file__).resolve().parents[2]
STATE = Path.home() / '.local/share/sub2api-cost-local'
DATABASE = 'sub2api-cost-local-postgres'
SERVER = 'sub2api-cost-local'
NETWORK = 'sub2api-cost-local-net'


def docker(*args, input=None, check=True, timeout=45):
    result = subprocess.run(['docker', *args], input=input, text=True, capture_output=True, timeout=timeout)
    if check and result.returncode:
        # Docker diagnostics can include environment values; never echo them.
        raise RuntimeError('Docker operation failed: ' + args[0])
    return result


def inspect(name):
    result = docker('inspect', name, check=False)
    return json.loads(result.stdout)[0] if result.returncode == 0 else None


def save_private(path, data):
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
    with os.fdopen(fd, 'w') as stream:
        stream.write(data)
    os.chmod(path, 0o600)


def start():
    STATE.mkdir(mode=0o700, parents=True, exist_ok=True)
    binary = STATE / 'cost-local'
    if not binary.is_file():
        raise RuntimeError('Build backend/cmd/cost-local for linux/arm64 into the private runtime directory first')
    source = inspect('sub2api')
    if source is None or not source['State']['Running']:
        raise RuntimeError('Existing local Sub2API is required')
    source_env = dict(item.split('=', 1) for item in source['Config']['Env'])
    source_networks = list(source['NetworkSettings']['Networks'])
    if len(source_networks) != 1:
        raise RuntimeError('Choose the explicit source container network before starting')
    config_path = STATE / 'database-private.json'
    if config_path.exists():
        config = json.loads(config_path.read_text())
    else:
        config = dict(owner_password=secrets.token_urlsafe(32), runtime_password=secrets.token_urlsafe(32))
        save_private(config_path, json.dumps(config))
    save_private(STATE / 'postgres.env', 'POSTGRES_USER=cost_owner\nPOSTGRES_DB=cost_ledger\nPOSTGRES_PASSWORD=' + config['owner_password'] + '\n')
    if docker('network', 'inspect', NETWORK, check=False).returncode:
        docker('network', 'create', NETWORK)
    (STATE / 'pgdata').mkdir(mode=0o700, exist_ok=True)
    if inspect(DATABASE) is None:
        docker('run', '-d', '--name', DATABASE, '--label', 'sub2api.task=cost-native-completion', '--restart', 'unless-stopped', '--network', NETWORK,
               '--env-file', str(STATE / 'postgres.env'), '--mount', 'type=bind,src=' + str(STATE / 'pgdata') + ',dst=/var/lib/postgresql/data', 'postgres:16-alpine')
    else:
        docker('start', DATABASE)
    for _ in range(30):
        if docker('exec', DATABASE, 'pg_isready', '-h', '127.0.0.1', '-U', 'cost_owner', '-d', 'cost_ledger', check=False, timeout=8).returncode == 0:
            break
        time.sleep(1)
    else:
        raise RuntimeError('Private cost database did not become ready')
    for path in sorted((ROOT / 'backend/internal/costing/migrations').glob('*.sql')):
        docker('exec', '-i', DATABASE, 'psql', '-U', 'cost_owner', '-d', 'cost_ledger', '-v', 'ON_ERROR_STOP=1', input=path.read_text())
    # The private owner handles schema; the runtime role only appends observations and ledger entries.
    role_sql = """DO $$ BEGIN IF NOT EXISTS(SELECT 1 FROM pg_roles WHERE rolname='cost_runtime') THEN CREATE ROLE cost_runtime LOGIN; END IF; END $$;
ALTER ROLE cost_runtime PASSWORD '%s';
GRANT CONNECT ON DATABASE cost_ledger TO cost_runtime;
GRANT USAGE ON SCHEMA public TO cost_runtime;
GRANT SELECT,INSERT ON cost_center_events,cost_center_quota_observations,cost_center_public_references,cost_center_account_observations,cost_center_source_snapshots TO cost_runtime;
GRANT SELECT,UPDATE ON cost_center_lock TO cost_runtime;
""" % config['runtime_password']
    docker('exec', '-i', DATABASE, 'psql', '-U', 'cost_owner', '-d', 'cost_ledger', '-v', 'ON_ERROR_STOP=1', input=role_sql)
    def dsn(user, password, host, port, database, readonly=False):
        query = {'sslmode': 'disable', 'connect_timeout': '5'}
        if readonly:
            query['default_transaction_read_only'] = 'on'
        return 'postgres://' + quote(user, safe='') + ':' + quote(password, safe='') + '@' + host + ':' + str(port) + '/' + quote(database, safe='') + '?' + urlencode(query)
    source_dsn = dsn(source_env['DATABASE_USER'], source_env['DATABASE_PASSWORD'], source_env['DATABASE_HOST'], source_env['DATABASE_PORT'], source_env['DATABASE_DBNAME'], True)
    cost_dsn = dsn('cost_runtime', config['runtime_password'], DATABASE, 5432, 'cost_ledger')
    save_private(STATE / 'runtime.env', 'SUB2API_COST_LOCAL_ONLY=yes\nSUB2API_COST_LEDGER_DSN=' + cost_dsn + '\nSUB2API_COST_SOURCE_DSN=' + source_dsn + '\n')
    if inspect(SERVER) is not None:
        docker('rm', '-f', SERVER)
    docker('create', '--name', SERVER, '--label', 'sub2api.task=cost-native-completion', '--restart', 'unless-stopped', '--network', source_networks[0],
           '-p', '127.0.0.1::8081', '--env-file', str(STATE / 'runtime.env'), '--mount', 'type=bind,src=' + str(binary) + ',dst=/opt/cost-local,readonly',
           'alpine:3.21', '/opt/cost-local', '--addr', '0.0.0.0:8081', '--authority', 'http://sub2api:8080')
    docker('network', 'connect', NETWORK, SERVER)
    docker('start', SERVER)
    address = docker('port', SERVER, '8081').stdout.strip()
    if not address.startswith('127.0.0.1:'):
        raise RuntimeError('Local publication must be loopback only')
    target = 'http://' + address
    env_path = ROOT / 'frontend/.env.local'
    lines = env_path.read_text().splitlines() if env_path.exists() else []
    lines = [line for line in lines if not line.startswith('VITE_COST_LOCAL_PROXY=')]
    lines.append('VITE_COST_LOCAL_PROXY=' + target)
    save_private(env_path, '\n'.join(lines) + '\n')
    save_private(STATE / 'runtime.json', json.dumps(dict(target=target, server=SERVER, database=DATABASE, source_access='read_only', source_container='sub2api', local_only=True), indent=2) + '\n')
    print(json.dumps(dict(status='STARTED', target=target, source_access='read_only', private_state=str(STATE)), ensure_ascii=False))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.parse_args()
    start()
