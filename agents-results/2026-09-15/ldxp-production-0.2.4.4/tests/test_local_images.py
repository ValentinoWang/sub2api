"""Offline isolation and false-positive regression tests; never invokes Docker."""
import argparse
import copy
import json
import os
from pathlib import Path
import stat
import sys
import tempfile
import unittest
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import verify_local_images as images


class ImageVerificationTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.directory = Path(self.temp.name)
        self.path = self.directory / 'candidate.json'
        self.value = {'schema_version': 1, 'commit': 'a' * 40, 'base_commit': 'b' * 40,
                      'version': '0.2.4.3', 'image_id': 'sha256:' + 'c' * 64, 'platform': 'linux/amd64',
                      'image': 'sub2api-local:0.2.4.3-' + 'a' * 12,
                      'base_migrations': {'239_base.sql': 'd' * 64},
                      'added_migrations': {'241_browser.sql': 'e' * 64}}
        self.path.write_text(json.dumps(self.value))
        self.inspected = {'Id': self.value['image_id'], 'Os': 'linux', 'Architecture': 'amd64',
                          'Config': {'Labels': {'org.opencontainers.image.revision': 'a' * 40,
                                                'org.opencontainers.image.version': '0.2.4.3'}}}
        self.rows = [{'version': name, 'checksum': checksum} for name, checksum in
                     (self.value['base_migrations'] | self.value['added_migrations']).items()]
        self.args = argparse.Namespace(candidate=self.path, rollback_image='old-image',
                                       postgres_image='postgres:18-alpine', redis_image='redis:8-alpine', timeout=30)

    def test_rejects_remote_daemon_before_any_mutation(self):
        with patch.dict(os.environ, {'DOCKER_HOST': 'ssh://production'}, clear=True), patch.object(images, 'docker') as cli:
            with self.assertRaises(RuntimeError):
                images.local_daemon()
            cli.assert_not_called()

    def test_explicit_context_overrides_local_host(self):
        responses = ['remote', json.dumps([{'Endpoints': {'docker': {'Host': 'tcp://remote:2376'}}}])]
        with patch.dict(os.environ, {'DOCKER_HOST': 'unix:///tmp/local', 'DOCKER_CONTEXT': 'remote'}, clear=True), patch.object(images, 'docker', side_effect=responses):
            with self.assertRaises(RuntimeError):
                images.local_daemon()

    def test_rejects_tag_retarget_and_wrong_source(self):
        images.validate_candidate_image(self.value, self.inspected)
        for change in ('id', 'revision', 'platform'):
            bad = copy.deepcopy(self.inspected)
            if change == 'id':
                bad['Id'] = 'sha256:' + 'f' * 64
            elif change == 'revision':
                bad['Config']['Labels']['org.opencontainers.image.revision'] = 'b' * 40
            else:
                bad['Architecture'] = 'arm64'
            with self.assertRaises(RuntimeError):
                images.validate_candidate_image(self.value, bad)

    def test_rejects_real_service_and_public_ports(self):
        for address, port in [('127.0.0.1', '8080'), ('127.0.0.1', '4174'), ('0.0.0.0', '49153'), ('::', '49153')]:
            inspected = {'NetworkSettings': {'Ports': {'8080/tcp': [{'HostIp': address, 'HostPort': port}]}}}
            with self.assertRaises(RuntimeError):
                images.loopback_port(inspected)
        inspected['NetworkSettings']['Ports']['8080/tcp'] = [{'HostIp': '127.0.0.1', 'HostPort': '49153'}]
        self.assertEqual(images.loopback_port(inspected), 49153)

    def test_rejects_missing_or_changed_migration_with_aliases_preserved(self):
        alias = {'version': '238_retired.sql', 'checksum': 'f' * 64}
        self.assertEqual(images.candidate_migrations(self.value, self.rows + [alias]), ['238_retired.sql'])
        for rows in (self.rows[:-1], self.rows + [self.rows[0]],
                     [dict(row, checksum='f' * 64) for row in self.rows]):
            with self.assertRaises(RuntimeError):
                images.candidate_migrations(self.value, rows)

    def test_schema_comparison_ignores_only_generated_restriction_key(self):
        first = '\\restrict RANDOM1\nCREATE TABLE example (id integer);\n\\unrestrict RANDOM1\n'
        self.assertEqual(images.schema_sha256(first), images.schema_sha256(first.replace('RANDOM1', 'RANDOM2')))
        self.assertNotEqual(images.schema_sha256(first), images.schema_sha256(first.replace('integer', 'text')))

    def test_credentials_are_private_and_run_does_not_publish_dependencies(self):
        sandbox = images.Sandbox(self.directory, 30)
        self.assertEqual(stat.S_IMODE(Path(sandbox.app_env).stat().st_mode), 0o600)
        self.assertEqual(stat.S_IMODE(Path(sandbox.pg_env).stat().st_mode), 0o600)
        with patch.object(images, 'docker', return_value='a' * 64) as cli:
            sandbox.start(sandbox.pg, 'sha256:' + 'e' * 64, ['--env-file', sandbox.pg_env])
        args = cli.call_args.args
        self.assertNotIn(sandbox.password, args)
        self.assertNotIn('--publish', args)
        self.assertEqual(args[args.index('--pull') + 1], 'never')
        self.assertEqual(args[args.index('--network') + 1], sandbox.network)

    def test_cleanup_removes_only_its_label_and_verifies_absence(self):
        sandbox = images.Sandbox(self.directory, 30)
        with patch.object(images, 'docker', side_effect=['owned-cid', '', '', 'owned-netid', '', '']) as cli:
            self.assertEqual(sandbox.cleanup(), [])
        filters = [call.args for call in cli.call_args_list if '--filter' in call.args]
        self.assertEqual(len(filters), 4)
        self.assertTrue(all(call[-1] == f'label={images.LABEL}={sandbox.run_id}' for call in filters))
        self.assertIn(('rm', '--force', '--volumes', 'owned-cid'), [call.args for call in cli.call_args_list])

    def simulate(self, *, schema_drift=False, migration_drift=False, cleanup_error=False, api_failure=False):
        rows = self.rows
        state = {'cleaned': False}

        class FakeAPI:
            def data(self, path, *args, **kwargs):
                if api_failure:
                    raise RuntimeError('simulated API failure')
                if path.endswith('/compliance'):
                    return {'required': False}
                return {'enabled': False, 'products': [], 'devices': [], 'batches': []}

        class FakeSandbox:
            def __init__(self, *args):
                self.run_id, self.network, self.pg, self.redis, self.pg_env = 'owned', 'owned-network', 'owned-pg', 'owned-redis', 'private.env'
                self.app_count = 0

            def start(self, *args):
                return 'owned-container'

            def create_network(self):
                return None

            def start_app(self, phase, image_id):
                self.app_count += 1
                return 'owned-' + phase, FakeAPI()

            def wait_for(self, check):
                return None

            def sql(self, statement):
                return '180000' if statement.startswith('SHOW') else '0'

            def authenticate(self, api, phase):
                return 'private-test-token'

            def migrations(self):
                return rows[:-1] if migration_drift and self.app_count == 2 else rows

            def schema_hash(self):
                return 'changed' if schema_drift and self.app_count == 2 else 'stable'

            def cleanup(self):
                state['cleaned'] = True
                return ['containers'] if cleanup_error else []

        old = dict(self.inspected, Id='sha256:' + 'f' * 64)
        evidence = {'application_rollback_on_migrated_database_passed': False}
        with patch.object(images, 'local_daemon'), patch.object(images, 'inspect_image', side_effect=[self.inspected, old, old, old]), patch.object(images, 'docker'), patch.object(images, 'Sandbox', FakeSandbox):
            if schema_drift or migration_drift or cleanup_error or api_failure:
                with self.assertRaises(RuntimeError):
                    images.verify(self.args, evidence)
            else:
                images.verify(self.args, evidence)
        self.assertTrue(state['cleaned'])
        return evidence

    def test_complete_mock_flow_binds_proof_to_candidate_and_rollback(self):
        evidence = self.simulate()
        self.assertEqual(evidence['candidate_sha256'], images.sha(self.path))
        self.assertEqual(evidence['rollback_image_id'], 'sha256:' + 'f' * 64)
        self.assertTrue(evidence['application_rollback_on_migrated_database_passed'])
        self.assertTrue(evidence['cleanup_passed'])

    def test_no_positive_proof_after_schema_migration_api_or_cleanup_failure(self):
        for condition in ('schema_drift', 'migration_drift', 'api_failure', 'cleanup_error'):
            with self.subTest(condition=condition):
                evidence = self.simulate(**{condition: True})
                self.assertIs(evidence['application_rollback_on_migrated_database_passed'], False)

    def test_redirect_cannot_escape_loopback(self):
        with self.assertRaises(RuntimeError):
            images.NoRedirect().redirect_request(None, None, 302, '', {}, 'https://production.example')

    def test_error_evidence_allows_only_static_runtime_assertions(self):
        evidence = {}
        images.record_error(evidence, RuntimeError('Exactly one app port required'))
        self.assertEqual(evidence['error_hint'], 'Exactly one app port required')
        for error in (RuntimeError('private-test-token'), ValueError('Exactly one app port required'),
                      images.subprocess.CalledProcessError(1, ['docker', 'secret'])):
            evidence = {}
            images.record_error(evidence, error)
            self.assertNotIn('error_hint', evidence)
            self.assertNotIn('secret', json.dumps(evidence))
            self.assertNotIn('private-test-token', json.dumps(evidence))

    def test_missing_published_port_identifies_exact_stage(self):
        evidence = {}
        sandbox = images.Sandbox(self.directory, 30, evidence)
        inspected = {'Image': self.value['image_id'], 'NetworkSettings': {'Ports': {}}}
        with patch.object(sandbox, 'start', return_value='owned-cid'), patch.object(images, 'docker', return_value=json.dumps([inspected])):
            with self.assertRaisesRegex(RuntimeError, 'Exactly one app port required'):
                sandbox.start_app('candidate', self.value['image_id'])
        self.assertEqual(evidence['stage'], 'candidate_loopback_port')

    def test_profile_assertion_identifies_exact_stage_without_recording_response(self):
        evidence = {}
        sandbox = images.Sandbox(self.directory, 30, evidence)
        api = unittest.mock.Mock()
        api.data.side_effect = [{'registration_enabled': False},
                                {'user': {'role': 'admin'}, 'access_token': 'secret'},
                                {'role': 'user', 'email': sandbox.email}]
        with self.assertRaisesRegex(RuntimeError, 'Authenticated profile mismatch'):
            sandbox.authenticate(api, 'candidate')
        self.assertEqual(evidence, {'stage': 'candidate_admin_profile_assertion'})

    def test_bridge_network_checks_publication_compatible_isolation(self):
        sandbox = images.Sandbox(self.directory, 30)
        network = {'Driver': 'bridge', 'Internal': False,
                   'Options': {'com.docker.network.bridge.enable_ip_masquerade': 'false'},
                   'Labels': {images.LABEL: sandbox.run_id}}
        with patch.object(images, 'docker', side_effect=['owned-net', json.dumps([network])]) as cli:
            sandbox.create_network()
        self.assertNotIn('--internal', cli.call_args_list[0].args)
        for field, value in [('Internal', True), ('Driver', 'host'),
                             ('Options', {}), ('Labels', {})]:
            with self.subTest(field=field), patch.object(images, 'docker', side_effect=['owned-net', json.dumps([dict(network, **{field: value})])]):
                with self.assertRaisesRegex(RuntimeError, 'Unexpected verification network configuration'):
                    sandbox.create_network()


if __name__ == '__main__':
    unittest.main()
