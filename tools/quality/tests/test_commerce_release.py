#!/usr/bin/env python3
"""Verify live collection and release rollback using isolated transports and files."""

import copy
import hashlib
import json
from pathlib import Path
import sys
import tempfile
import unittest
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import commerce_release as release
from test_commerce_environment_parity import snapshot


class FakeTransport:
    def __init__(self):
        self.snapshots = {environment: snapshot(environment) for environment in ('dev', 'prod')}
        for environment in ('dev', 'prod'):
            identity = json.dumps(['123' if environment == 'dev' else '456', 'fixture'])
            self.snapshots[environment]['database_identity'] = hashlib.sha256(identity.encode()).hexdigest()
        self.writes = []
        self.commands = []
        self.fail_write = None
        self.fail_rollback = False

    def command(self, config, argv):
        self.commands.append(argv)
        data = self.snapshots[config['environment']]
        if argv[1] == 'inspect':
            return data['image_id'] + '\n'
        if argv[1] == 'image':
            return json.dumps({'org.opencontainers.image.version': data['version'],
                               'org.opencontainers.image.revision': data['source_commit']})
        if argv[-1] == release.MIGRATIONS_SQL:
            return json.dumps(data['migrations'])
        if argv[-1] == release.IDENTITY_SQL:
            return json.dumps(['123' if config['environment'] == 'dev' else '456', 'fixture'])
        return '-- Generated dump\n\\restrict random\nCREATE TABLE fixture(id integer);\n\\unrestrict random\n'

    def api(self, config, method, route, body=None):
        environment = config['environment']
        state = self.snapshots[environment]
        if method == 'GET':
            return copy.deepcopy({key: state[key] for key in release.BUSINESS_KEYS})
        self.writes.append((environment, method, route, copy.deepcopy(body)))
        if route == release.WORKER_ROUTE:
            state['restock_enabled'] = body['enabled']
        elif route == release.SETTINGS_ROUTE:
            state['purchase_enabled'] = body['purchase_subscription_enabled']
        else:
            raise AssertionError('unexpected mutation route')
        if len(self.writes) == self.fail_write or (self.fail_rollback and len(self.writes) > self.fail_write):
            raise release.ReleaseError('simulated lost response after commit')
        return {}


class CommerceReleaseTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='commerce-release-test-')
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.transport = FakeTransport()
        self.config = {environment: {'environment': environment, 'app_container': 'app',
                                    'db_container': 'db'} for environment in ('dev', 'prod')}

    def add_acceptance(self):
        for environment in ('dev', 'prod'):
            data = release.collect(self.config[environment], environment, self.transport)
            directory = self.root / environment
            directory.mkdir()
            record = directory / 'record.md'
            record.write_text('Isolated test fixture evidence; not real acceptance.\n')
            check = {'passed': True, 'evidence_file': record.name,
                     'evidence_sha256': hashlib.sha256(record.read_bytes()).hexdigest()}
            evidence = {'environment': environment, 'binding_sha256': release.binding_sha256(data),
                        'checks': {field: copy.deepcopy(check) for field in release.parity.ACCEPTANCE_FIELDS}}
            path = directory / 'acceptance.json'
            path.write_text(json.dumps(evidence))
            self.config[environment]['acceptance_file'] = str(path)

    def test_collector_uses_immutable_image_and_does_not_emit_raw_database_identity(self):
        data = release.collect(self.config['dev'], 'dev', self.transport)
        self.assertEqual(data['image_id'], self.transport.snapshots['dev']['image_id'])
        self.assertEqual(len(data['database_identity']), 64)
        self.assertNotIn('123', data['database_identity'])
        self.assertNotIn('acceptance', data)
        self.assertTrue(any('pg_dump' in ' '.join(argv) for argv in self.transport.commands))
        self.assertTrue(any(release.MIGRATIONS_SQL in argv for argv in self.transport.commands))

    def test_artifact_passes_without_acceptance_and_sales_blocks_without_writes(self):
        result = release.execute(self.config, 'artifact', False, self.transport)
        self.assertTrue(result['passed'])
        result = release.execute(self.config, 'sales', True, self.transport)
        self.assertFalse(result['passed'])
        self.assertEqual(self.transport.writes, [])

    def test_every_invocation_recollects_live_state(self):
        self.assertTrue(release.execute(self.config, 'artifact', False, self.transport)['passed'])
        self.transport.snapshots['prod']['image_id'] = 'sha256:' + '0' * 64
        self.assertFalse(release.execute(self.config, 'artifact', False, self.transport)['passed'])

    def test_api_must_match_the_inspected_database(self):
        self.transport.snapshots['prod']['database_identity'] = '0' * 64
        with self.assertRaisesRegex(release.ReleaseError, 'identities differ'):
            release.collect(self.config['prod'], 'prod', self.transport)

    def test_fractional_json_and_nonfinite_numbers_fail_without_rounding(self):
        for raw in ('{"cny_amount":5.000000000000000000000001}', '{"x":1e400}', '{"x":NaN}'):
            with self.assertRaises(release.ReleaseError):
                release.parse_json(raw)

    def test_bound_record_enables_workers_before_purchase_with_sparse_payload(self):
        self.add_acceptance()
        result = release.execute(self.config, 'sales', True, self.transport)
        self.assertTrue(result['passed'])
        self.assertTrue(result['enablement']['enabled'])
        self.assertEqual(self.transport.writes, [
            ('dev', 'POST', release.WORKER_ROUTE, {'enabled': True}),
            ('prod', 'POST', release.WORKER_ROUTE, {'enabled': True}),
            ('prod', 'PUT', release.SETTINGS_ROUTE, {'purchase_subscription_enabled': True}),
        ])

    def test_disabled_five_yuan_product_cannot_open_sales(self):
        for data in self.transport.snapshots.values():
            data['products'][0]['enabled'] = False
        self.add_acceptance()
        self.assertTrue(release.execute(self.config, 'sales', False, self.transport)['passed'])
        result = release.execute(self.config, 'sales', True, self.transport)
        self.assertFalse(result['passed'])
        self.assertEqual(self.transport.writes, [])

    def test_lost_response_after_each_possible_write_restores_flags(self):
        self.add_acceptance()
        for failed_write in (1, 2, 3):
            with self.subTest(failed_write=failed_write):
                self.transport = FakeTransport()
                self.transport.fail_write = failed_write
                result = release.execute(self.config, 'sales', True, self.transport)
                self.assertFalse(result['passed'])
                self.assertIn('restored and verified', result['blockers'][-1])
                for data in self.transport.snapshots.values():
                    self.assertFalse(data['restock_enabled'])
                    self.assertFalse(data['purchase_enabled'])

    def test_rollback_failure_is_reported(self):
        self.add_acceptance()
        self.transport.fail_write = 3
        self.transport.fail_rollback = True
        result = release.execute(self.config, 'sales', True, self.transport)
        self.assertFalse(result['passed'])
        self.assertIn('rollback incomplete', result['blockers'][-1])

    def test_preexisting_enabled_flag_is_not_disabled_on_failure(self):
        self.transport.snapshots['dev']['restock_enabled'] = True
        self.add_acceptance()
        self.transport.fail_write = 2
        result = release.execute(self.config, 'sales', True, self.transport)
        self.assertFalse(result['passed'])
        self.assertTrue(self.transport.snapshots['dev']['restock_enabled'])
        self.assertFalse(any(write[0] == 'dev' for write in self.transport.writes))

    def test_enable_sales_requires_sales_stage(self):
        with self.assertRaises(release.ReleaseError):
            release.execute(self.config, 'artifact', True, self.transport)
        self.assertEqual(self.transport.commands, [])

    def test_stale_image_or_product_binding_rejects_acceptance(self):
        self.add_acceptance()
        for field in ('image_id', 'products'):
            with self.subTest(field=field):
                self.transport = FakeTransport()
                if field == 'image_id':
                    self.transport.snapshots['prod'][field] = 'sha256:' + '0' * 64
                else:
                    self.transport.snapshots['prod'][field][0]['goods_id'] = 301
                with self.assertRaisesRegex(release.ReleaseError, 'binding is stale'):
                    release.execute(self.config, 'sales', True, self.transport)
                self.assertEqual(self.transport.writes, [])

    def test_changed_evidence_bytes_are_rejected(self):
        self.add_acceptance()
        (self.root / 'dev' / 'record.md').write_text('changed')
        with self.assertRaisesRegex(release.ReleaseError, 'hash mismatch'):
            release.execute(self.config, 'sales', True, self.transport)
        self.assertEqual(self.transport.writes, [])

    def test_false_acceptance_is_never_upgraded(self):
        self.add_acceptance()
        path = Path(self.config['prod']['acceptance_file'])
        evidence = json.loads(path.read_text())
        evidence['checks']['duplicate_redeem']['passed'] = False
        path.write_text(json.dumps(evidence))
        result = release.execute(self.config, 'sales', True, self.transport)
        self.assertFalse(result['passed'])
        self.assertEqual(self.transport.writes, [])

    def test_config_drift_between_guard_and_mutation_blocks(self):
        self.add_acceptance()
        original = self.transport.api
        calls = 0
        def drifting_api(config, method, route, body=None):
            nonlocal calls
            calls += 1
            if calls == 3:
                self.transport.snapshots['dev']['products'][0]['usd_credit'] = '50'
            return original(config, method, route, body)
        self.transport.api = drifting_api
        result = release.execute(self.config, 'sales', True, self.transport)
        self.assertFalse(result['passed'])
        self.assertEqual(self.transport.writes, [])

    def test_final_readback_failure_rolls_back_purchase(self):
        self.add_acceptance()
        original = self.transport.api
        def failing_readback(config, method, route, body=None):
            if method == 'GET' and len(self.transport.writes) == 3:
                raise release.ReleaseError('readback unavailable')
            return original(config, method, route, body)
        self.transport.api = failing_readback
        result = release.execute(self.config, 'sales', True, self.transport)
        self.assertFalse(result['passed'])
        self.assertFalse(self.transport.snapshots['prod']['purchase_enabled'])
        self.assertFalse(self.transport.snapshots['prod']['restock_enabled'])

    def test_schema_normalizes_generated_noise_but_keeps_function_body(self):
        first = '-- Dumped from 1\n\\restrict abc\nCREATE TABLE t(a int);\n\\unrestrict abc\n'
        second = '-- Dumped from 2\n\\restrict xyz\n\nCREATE TABLE t(a int);\n\\unrestrict xyz\n'
        self.assertEqual(release.normalize_schema(first), release.normalize_schema(second))
        self.assertNotEqual(release.normalize_schema(first), release.normalize_schema(first.replace('int', 'text')))
        function = 'CREATE FUNCTION f() RETURNS text AS $$\n-- body comment\n\nSELECT 1;\n$$ LANGUAGE sql;'
        self.assertIn('-- body comment\n\n', release.normalize_schema(function))

    def test_config_rejects_unknown_fields_injection_and_credential_urls(self):
        base = {environment: {'url': 'https://' + environment + '.invalid', 'app_container': 'app',
                              'db_container': 'db', 'admin_key_file': 'admin.key'}
                for environment in ('dev', 'prod')}
        for field, value in (('url', 'http://prod.invalid'), ('url', 'https://user:secret@prod.invalid'),
                             ('app_container', '-x;echo secret'), ('ssh_host', '-oProxyCommand=bad'),
                             ('plaintext_password', 'secret')):
            with self.subTest(field=field):
                config = copy.deepcopy(base)
                config['prod'][field] = value
                path = self.root / 'config.json'
                path.write_text(json.dumps(config))
                with self.assertRaises(release.ReleaseError):
                    release.load_config(path)

    def test_key_requires_mode_600_and_does_not_accept_symlinks(self):
        path = self.root / 'admin.key'
        path.write_text('fixture-key\n')
        path.chmod(0o600)
        self.assertEqual(release.read_admin_key(path), 'fixture-key')
        path.chmod(0o644)
        with self.assertRaises(release.ReleaseError):
            release.read_admin_key(path)
        path.chmod(0o600)
        link = self.root / 'link.key'
        link.symlink_to(path)
        with self.assertRaises(release.ReleaseError):
            release.read_admin_key(link)

    def test_transport_errors_do_not_expose_raw_output(self):
        with patch.object(release.subprocess, 'run') as run:
            run.return_value.returncode = 1
            run.return_value.stdout = b'fixture-sensitive-response'
            run.return_value.stderr = b'fixture-sensitive-response'
            with self.assertRaises(release.ReleaseError) as raised:
                release.Transport().command({}, ['docker', 'inspect'])
            self.assertNotIn('fixture-sensitive', str(raised.exception))

    def test_ssh_quotes_remote_arguments_and_never_disables_host_validation(self):
        with patch.object(release.subprocess, 'run') as run:
            run.return_value.returncode = 0
            run.return_value.stdout = b'ok'
            release.Transport().command({'ssh_host': 'fixture', 'ssh_key': '/tmp/key path'},
                                        ['docker', 'exec', 'db', 'sh', '-c', 'printf "$POSTGRES_DB"'])
            argv = run.call_args.args[0]
            self.assertIn('StrictHostKeyChecking=yes', argv)
            self.assertIn('/tmp/key path', argv)
            self.assertTrue(argv[-1].endswith("'printf \"$POSTGRES_DB\"'"))


if __name__ == '__main__':
    unittest.main()
