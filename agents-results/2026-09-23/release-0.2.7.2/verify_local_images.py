"""Verify local candidate boot and old-image rollback on a disposable migrated database."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import secrets
import signal
import subprocess
import tempfile
import time
import urllib.error
import urllib.request

from release_common import candidate, migration_map, require, sha


LABEL = 'io.sub2api.isolated-image-verification'
MIGRATIONS_SQL = "SELECT COALESCE(json_agg(json_build_object('version',filename,'checksum',checksum) ORDER BY filename),'[]'::json) FROM schema_migrations"
BOUNDARIES = [
    'Disposable empty PostgreSQL database; no local or production data copied.',
    'Only boot, public settings, synthetic administrator authentication, browser status and rollback basic APIs checked.',
    'Synthetic administrator compliance acknowledgement exists only in the disposable test database.',
    'No real upstream, Chrome extension, merchant inventory, payment, redemption or populated-database acceptance.',
    'No production release, human acceptance, database downgrade or real-workload rollback acceptance.',
]
SAFE_FAILURE_HINTS = frozenset({
    'A local Unix Docker socket is required', 'Candidate image ID mismatch',
    'Candidate commit mismatch', 'Candidate version mismatch', 'Candidate platform mismatch',
    'Exactly one app port required', 'Non-loopback publication rejected',
    'Reserved or invalid port rejected', 'Candidate migration manifest is not fully applied',
    'Unsafe API port', 'Relative API path required', 'Unexpected HTTP status',
    'Unsuccessful API response', 'HTTP redirect rejected', 'Container ID not returned',
    'Readiness timed out', 'Started image identity mismatch', 'Public settings missing',
    'Synthetic admin login failed', 'Access token missing', 'Authenticated profile mismatch',
    'Owned resources remain', 'Invalid rollback image ID',
    'Candidate cannot be its own rollback image', 'Rollback platform mismatch',
    'PostgreSQL 18 required', 'Database is not empty', 'Test compliance phrase missing',
    'Test compliance fixture not accepted', 'Unexpected browser status in empty database',
    'Migrations changed while stopping candidate', 'Rollback changed migration ledger',
    'Rollback changed database schema', 'Temporary Docker resource cleanup incomplete',
    'Unexpected verification network configuration',
})


def record_error(evidence, error):
    evidence['error_type'] = type(error).__name__
    # Only this script's literal assertions are printable, never arbitrary exception text.
    if type(error) is RuntimeError and str(error) in SAFE_FAILURE_HINTS:
        evidence['error_hint'] = str(error)


def docker(*args, timeout=60):
    # Never print Docker output: inspect and error messages can contain credentials.
    return subprocess.check_output(['docker', *args], stderr=subprocess.PIPE,
                                   text=True, timeout=timeout).strip()


def local_daemon():
    endpoint = os.environ.get('DOCKER_HOST')
    # DOCKER_CONTEXT takes precedence over DOCKER_HOST in the CLI.
    if not endpoint or os.environ.get('DOCKER_CONTEXT'):
        context = docker('context', 'show')
        endpoint = json.loads(docker('context', 'inspect', context))[0]['Endpoints']['docker']['Host']
    require(endpoint.startswith('unix://'), 'A local Unix Docker socket is required')


def inspect_image(reference):
    return json.loads(docker('image', 'inspect', reference))[0]


def validate_candidate_image(value, inspected):
    require(inspected['Id'] == value['image_id'], 'Candidate image ID mismatch')
    labels = inspected['Config'].get('Labels') or {}
    require(labels.get('org.opencontainers.image.revision') == value['commit'], 'Candidate commit mismatch')
    require(labels.get('org.opencontainers.image.version') == value['version'], 'Candidate version mismatch')
    require(inspected['Os'] == 'linux' and inspected['Architecture'] == 'amd64', 'Candidate platform mismatch')


def loopback_port(inspected):
    bindings = inspected['NetworkSettings']['Ports'].get('8080/tcp')
    require(isinstance(bindings, list) and len(bindings) == 1, 'Exactly one app port required')
    require(bindings[0]['HostIp'] == '127.0.0.1', 'Non-loopback publication rejected')
    port = int(bindings[0]['HostPort'])
    require(1024 <= port <= 65535 and port not in (8080, 4174), 'Reserved or invalid port rejected')
    return port


def schema_sha256(dump):
    # pg_dump 18 generates a random psql restriction key for each invocation.
    lines = [line for line in dump.splitlines()
             if not line.startswith(('\\restrict ', '\\unrestrict '))]
    return hashlib.sha256(('\n'.join(lines) + '\n').encode()).hexdigest()


def candidate_migrations(value, rows):
    actual = migration_map(rows)
    expected = value['base_migrations'] | value['added_migrations']
    require(all(actual.get(name) == checksum for name, checksum in expected.items()),
            'Candidate migration manifest is not fully applied')
    # Consolidation migrations intentionally add retired filename/checksum aliases.
    return sorted(actual.keys() - expected.keys())


class API:
    def __init__(self, port):
        require(1024 <= port <= 65535 and port not in (8080, 4174), 'Unsafe API port')
        self.base = f'http://127.0.0.1:{port}'
        self.opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())

    def request(self, path, body=None, token=None):
        require(path.startswith('/') and not path.startswith('//'), 'Relative API path required')
        headers = {'Accept': 'application/json', 'User-Agent': 'Sub2API-isolated-image-test'}
        if token:
            headers['Authorization'] = 'Bearer ' + token
        data = None if body is None else json.dumps(body).encode()
        if data is not None:
            headers['Content-Type'] = 'application/json'
        request = urllib.request.Request(self.base + path, data=data, headers=headers)
        with self.opener.open(request, timeout=5) as response:
            require(response.status == 200, 'Unexpected HTTP status')
            return json.load(response)

    def data(self, path, body=None, token=None):
        result = self.request(path, body, token)
        require(isinstance(result, dict) and result.get('code') == 0 and 'data' in result,
                'Unsuccessful API response')
        return result['data']


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        raise RuntimeError('HTTP redirect rejected')


class Sandbox:
    def __init__(self, directory, timeout, evidence=None):
        self.directory = directory
        self.timeout = timeout
        self.evidence = evidence if evidence is not None else {}
        self.run_id = secrets.token_hex(16)
        self.prefix = 'sub2api-image-check-' + self.run_id
        self.network = self.prefix + '-network'
        self.pg = self.prefix + '-postgres'
        self.redis = self.prefix + '-redis'
        self.email = 'image-check-' + self.run_id + '@example.invalid'
        self.password = secrets.token_urlsafe(32)
        db_password = secrets.token_urlsafe(32)
        self.pg_env = self.env_file('postgres.env', {
            'POSTGRES_USER': 'imagecheck', 'POSTGRES_DB': 'imagecheck',
            'POSTGRES_PASSWORD': db_password, 'PGDATA': '/var/lib/postgresql/18/docker',
        })
        self.app_env = self.env_file('app.env', {
            'AUTO_SETUP': 'true', 'SERVER_HOST': '0.0.0.0', 'SERVER_PORT': '8080',
            'SERVER_MODE': 'release', 'RUN_MODE': 'standard', 'DATA_DIR': '/app/data',
            'DATABASE_HOST': self.pg, 'DATABASE_PORT': '5432', 'DATABASE_USER': 'imagecheck',
            'DATABASE_PASSWORD': db_password, 'DATABASE_DBNAME': 'imagecheck',
            'DATABASE_SSLMODE': 'disable', 'DATABASE_MAX_OPEN_CONNS': '10',
            'DATABASE_MAX_IDLE_CONNS': '2', 'REDIS_HOST': self.redis, 'REDIS_PORT': '6379',
            'REDIS_DB': '0', 'REDIS_POOL_SIZE': '16', 'REDIS_MIN_IDLE_CONNS': '1',
            'ADMIN_EMAIL': self.email, 'ADMIN_PASSWORD': self.password,
            'JWT_SECRET': secrets.token_hex(32), 'TOTP_ENCRYPTION_KEY': secrets.token_hex(32),
            'TZ': 'Asia/Shanghai', 'SETUP_MIGRATION_TIMEOUT_SECONDS': str(timeout),
        })

    def env_file(self, filename, fields):
        path = self.directory / filename
        with open(path, 'x', opener=lambda p, flags: os.open(p, flags, 0o600)) as output:
            output.write(''.join(f'{key}={value}\n' for key, value in fields.items()))
        return str(path)

    def start(self, name, image_id, extra, command=()):
        result = docker('run', '--detach', '--pull', 'never', '--name', name,
                        '--label', f'{LABEL}={self.run_id}', '--network', self.network,
                        '--security-opt', 'no-new-privileges:true', '--log-driver', 'none',
                        *extra, image_id, *command)
        require(re.fullmatch(r'[0-9a-f]{64}', result), 'Container ID not returned')
        return result

    def create_network(self):
        # Internal bridge networks suppress publication on some Docker engines.
        # A dedicated bridge supports loopback publishing; disable outbound NAT.
        docker('network', 'create', '--driver', 'bridge',
               '--opt', 'com.docker.network.bridge.enable_ip_masquerade=false',
               '--label', f'{LABEL}={self.run_id}', self.network)
        network = json.loads(docker('network', 'inspect', self.network))[0]
        require(network['Driver'] == 'bridge' and network['Internal'] is False
                and network['Options'].get('com.docker.network.bridge.enable_ip_masquerade') == 'false'
                and network['Labels'].get(LABEL) == self.run_id,
                'Unexpected verification network configuration')
        self.evidence['network'] = {'driver': 'bridge', 'internal': False,
                                    'outbound_ip_masquerade': False}

    def wait_for(self, check):
        deadline = time.monotonic() + self.timeout
        while True:
            try:
                if check():
                    return
            except (subprocess.SubprocessError, urllib.error.URLError, OSError, ValueError):
                pass
            require(time.monotonic() < deadline, 'Readiness timed out')
            time.sleep(2)

    def sql(self, statement):
        return docker('exec', self.pg, 'psql', '-X', '-v', 'ON_ERROR_STOP=1',
                      '-U', 'imagecheck', '-d', 'imagecheck', '-At', '-c', statement)

    def migrations(self):
        return json.loads(self.sql(MIGRATIONS_SQL))

    def schema_hash(self):
        return schema_sha256(docker('exec', self.pg, 'pg_dump', '-U', 'imagecheck',
                                    '-d', 'imagecheck', '--schema-only', '--no-owner', '--no-privileges'))

    def start_app(self, phase, image_id):
        self.evidence['stage'] = phase + '_start_container'
        cid = self.start(self.prefix + '-' + phase, image_id,
                         ['--env-file', self.app_env, '--publish', '127.0.0.1::8080',
                          '--tmpfs', '/app/data:rw,nosuid,size=128m'])
        self.evidence['stage'] = phase + '_inspect_identity'
        inspected = json.loads(docker('inspect', cid))[0]
        require(inspected['Image'] == image_id, 'Started image identity mismatch')
        self.evidence['stage'] = phase + '_loopback_port'
        api = API(loopback_port(inspected))
        self.evidence['stage'] = phase + '_health'
        self.wait_for(lambda: api.request('/health').get('status') == 'ok')
        return cid, api

    def authenticate(self, api, phase):
        self.evidence['stage'] = phase + '_public_settings_request'
        settings = api.data('/api/v1/settings/public')
        self.evidence['stage'] = phase + '_public_settings_assertion'
        require(isinstance(settings, dict) and bool(settings), 'Public settings missing')
        self.evidence['stage'] = phase + '_admin_login_request'
        login = api.data('/api/v1/auth/login', {'email': self.email, 'password': self.password})
        self.evidence['stage'] = phase + '_admin_login_assertion'
        require(login.get('user', {}).get('role') == 'admin', 'Synthetic admin login failed')
        token = login.get('access_token')
        require(isinstance(token, str) and bool(token), 'Access token missing')
        self.evidence['stage'] = phase + '_admin_profile_request'
        profile = api.data('/api/v1/user/profile', token=token)
        self.evidence['stage'] = phase + '_admin_profile_assertion'
        require(profile.get('role') == 'admin' and profile.get('email') == self.email,
                'Authenticated profile mismatch')
        return token

    def cleanup(self):
        # Labels are unique to this invocation, including resources created before an interrupted CLI call.
        errors = []
        for kind, list_args, remove_args in (
            ('containers', ('ps', '-aq'), ('rm', '--force', '--volumes')),
            ('networks', ('network', 'ls', '-q'), ('network', 'rm')),
        ):
            try:
                ids = docker(*list_args, '--filter', f'label={LABEL}={self.run_id}').splitlines()
                for resource_id in ids:
                    docker(*remove_args, resource_id)
                require(not docker(*list_args, '--filter', f'label={LABEL}={self.run_id}'),
                        'Owned resources remain')
            except (subprocess.SubprocessError, OSError, RuntimeError):
                errors.append(kind)
        return errors


def verify(args, evidence):
    value = candidate(args.candidate)
    evidence['candidate_sha256'] = sha(args.candidate)
    evidence['candidate_image_id'] = value['image_id']
    evidence['stage'] = 'local_daemon_and_image_identity'
    local_daemon()
    validate_candidate_image(value, inspect_image(value['image']))
    old = inspect_image(args.rollback_image)
    require(re.fullmatch(r'sha256:[0-9a-f]{64}', old['Id']), 'Invalid rollback image ID')
    require(old['Id'] != value['image_id'], 'Candidate cannot be its own rollback image')
    require(old['Os'] == 'linux' and old['Architecture'] == 'amd64', 'Rollback platform mismatch')
    evidence['rollback_image_id'] = old['Id']
    postgres = inspect_image(args.postgres_image)
    redis = inspect_image(args.redis_image)
    evidence['dependency_image_ids'] = {'postgres': postgres['Id'], 'redis': redis['Id']}
    with tempfile.TemporaryDirectory(prefix='sub2api-image-check-') as directory:
        sandbox = Sandbox(Path(directory), args.timeout, evidence)
        evidence['resource_label'] = f'{LABEL}={sandbox.run_id}'
        try:
            evidence['stage'] = 'isolated_dependencies'
            sandbox.create_network()
            sandbox.start(sandbox.pg, postgres['Id'], ['--env-file', sandbox.pg_env,
                          '--tmpfs', '/var/lib/postgresql:rw,nosuid,size=1536m'])
            sandbox.start(sandbox.redis, redis['Id'], ['--tmpfs', '/data:rw,nosuid,size=64m'],
                          ['redis-server', '--save', '', '--appendonly', 'no'])
            sandbox.wait_for(lambda: docker('exec', sandbox.pg, 'pg_isready', '-h', '127.0.0.1', '-U', 'imagecheck', '-d', 'imagecheck').endswith('accepting connections'))
            sandbox.wait_for(lambda: docker('exec', sandbox.redis, 'redis-cli', 'ping') == 'PONG')
            require(sandbox.sql('SHOW server_version_num').startswith('18'), 'PostgreSQL 18 required')
            require(sandbox.sql("SELECT count(*) FROM information_schema.tables WHERE table_schema='public'") == '0', 'Database is not empty')
            evidence['empty_database_verified'] = True
            candidate_id, api = sandbox.start_app('candidate', value['image_id'])
            token = sandbox.authenticate(api, 'candidate')
            evidence['stage'] = 'candidate_compliance_status_request'
            compliance = api.data('/api/v1/admin/compliance', token=token)
            if compliance.get('required') is True:
                evidence['stage'] = 'candidate_compliance_phrase_assertion'
                phrase = compliance.get('ack_phrase_en')
                require(isinstance(phrase, str) and bool(phrase), 'Test compliance phrase missing')
                evidence['stage'] = 'candidate_compliance_accept_request'
                accepted = api.data('/api/v1/admin/compliance/accept',
                                    {'phrase': phrase, 'language': 'en'}, token=token)
                evidence['stage'] = 'candidate_compliance_accept_assertion'
                require(accepted.get('required') is False, 'Test compliance fixture not accepted')
            evidence['stage'] = 'candidate_browser_status_request'
            status = api.data('/api/v1/admin/tools/ldxp/browser/status', token=token)
            evidence['stage'] = 'candidate_browser_status_assertion'
            require(status.get('enabled') is False and status.get('products') == []
                    and status.get('devices') == [] and status.get('batches') == [],
                    'Unexpected browser status in empty database')
            evidence['stage'] = 'candidate_migration_manifest'
            before = sandbox.migrations()
            evidence['migration_aliases_not_in_manifest'] = candidate_migrations(value, before)
            evidence['candidate_checks'] = {'health': True, 'public_settings': True, 'admin_login': True,
                                            'admin_profile': True, 'browser_status_empty': True,
                                            'migration_manifest_covered': True}
            evidence['stage'] = 'candidate_stop'
            docker('stop', '--time', '20', candidate_id)
            # Capture a quiescent baseline so app startup is the only possible schema actor.
            evidence['stage'] = 'candidate_quiescent_schema_baseline'
            require(sandbox.migrations() == before, 'Migrations changed while stopping candidate')
            before_schema = sandbox.schema_hash()
            rollback_id, old_api = sandbox.start_app('rollback', old['Id'])
            sandbox.authenticate(old_api, 'rollback')
            evidence['stage'] = 'rollback_stop'
            docker('stop', '--time', '20', rollback_id)
            evidence['stage'] = 'rollback_migration_ledger_assertion'
            after = sandbox.migrations()
            require(after == before, 'Rollback changed migration ledger')
            evidence['stage'] = 'rollback_schema_assertion'
            after_schema = sandbox.schema_hash()
            require(after_schema == before_schema, 'Rollback changed database schema')
            evidence['migration_count'] = len(before)
            evidence['migrations_sha256'] = hashlib.sha256(json.dumps(before, sort_keys=True).encode()).hexdigest()
            evidence['schema_before_rollback_sha256'] = before_schema
            evidence['schema_after_rollback_sha256'] = after_schema
            evidence['rollback_checks'] = {'health': True, 'public_settings': True, 'admin_login': True,
                                           'admin_profile': True, 'same_database': True,
                                           'migration_ledger_unchanged': True, 'schema_unchanged': True}
        finally:
            errors = sandbox.cleanup()
            evidence['cleanup_passed'] = not errors
            evidence['cleanup_failed_resource_types'] = errors
            require(not errors, 'Temporary Docker resource cleanup incomplete')
    evidence['stage'] = 'completed'
    evidence['application_rollback_on_migrated_database_passed'] = True


def interrupted(signum, frame):
    raise KeyboardInterrupt


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--candidate', type=Path, required=True)
    parser.add_argument('--rollback-image', required=True)
    parser.add_argument('--output', type=Path, required=True, help='New JSON evidence path; never overwritten')
    parser.add_argument('--postgres-image', default='postgres:18-alpine')
    parser.add_argument('--redis-image', default='redis:8-alpine')
    parser.add_argument('--timeout', type=int, default=300)
    args = parser.parse_args()
    require(30 <= args.timeout <= 900, 'Timeout must be 30..900 seconds')
    os.umask(0o077)
    evidence = {'schema_version': 1, 'environment': 'isolated-local', 'created_at': time.time(),
                'status': 'FAILED', 'application_rollback_on_migrated_database_passed': False,
                'verification_boundaries': BOUNDARIES}
    # Reserve the output before any Docker operation to prevent replacement of old evidence.
    with args.output.open('x') as output:
        signal.signal(signal.SIGTERM, interrupted)
        code = 1
        try:
            verify(args, evidence)
            evidence['status'] = 'PASSED'
            code = 0
        except (Exception, KeyboardInterrupt) as error:
            record_error(evidence, error)
        evidence['finished_at'] = time.time()
        json.dump(evidence, output, indent=2)
        output.write('\n')
    print(json.dumps({'status': evidence['status'], 'stage': evidence.get('stage'),
                      'error_hint': evidence.get('error_hint'),
                      'evidence': str(args.output), 'cleanup_passed': evidence.get('cleanup_passed'),
                      'application_rollback_on_migrated_database_passed': evidence['application_rollback_on_migrated_database_passed']}))
    return code


if __name__ == '__main__':
    raise SystemExit(main())
