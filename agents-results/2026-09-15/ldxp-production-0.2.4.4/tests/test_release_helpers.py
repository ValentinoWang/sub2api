"""Offline failure-path simulations. No Docker, credentials or live API calls."""
import argparse
import copy
import io
import json
import os
from pathlib import Path
import sys
import tempfile
import tarfile
import time
import unittest
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import release_api
import release_common as common
import replace_app


class ReleaseTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name).resolve()
        self.path = self.root / 'candidate.json'
        self.value = {'schema_version': 1, 'commit': 'a' * 40, 'base_commit': 'b' * 40,
                      'version': '0.2.4.3', 'image_id': 'sha256:' + 'c' * 64, 'platform': 'linux/amd64',
                      'image': 'sub2api-local:0.2.4.3-' + 'a' * 12,
                      'base_migrations': {'239_base.sql': 'd' * 64},
                      'added_migrations': {'241_browser.sql': 'e' * 64}}
        self.path.write_text(json.dumps(self.value))
        self.notice = self.root / 'notice.json'
        self.write_receipt(self.notice, status='MAINTENANCE_ACTIVE', maintenance_id=23)
        self.args = argparse.Namespace(action='recovery', candidate=self.path, environment='dev',
                                       maintenance=self.notice, acceptance=None)

    def write_receipt(self, path, **fields):
        value = {'candidate_sha256': common.sha(self.path), 'environment': 'dev', 'created_at': time.time(), **fields}
        path.write_text(json.dumps(value))

    def test_candidate_rejects_short_commit_and_wrong_tag(self):
        self.assertEqual(common.candidate(self.path)['version'], '0.2.4.3')
        for field, value in [('commit', 'a' * 12), ('image', 'sub2api-local:latest')]:
            invalid = dict(self.value, **{field: value})
            self.path.write_text(json.dumps(invalid))
            with self.assertRaises(RuntimeError):
                common.candidate(self.path)

    def test_receipt_rejects_stale_wrong_environment_and_candidate(self):
        self.assertEqual(common.receipt(self.notice, self.path, 'dev', 'MAINTENANCE_ACTIVE')['maintenance_id'], 23)
        for fields in ({'created_at': time.time() - 1801}, {'environment': 'prod'}, {'candidate_sha256': 'f' * 64}):
            self.write_receipt(self.notice, status='MAINTENANCE_ACTIVE', **fields)
            with self.assertRaises(RuntimeError):
                common.receipt(self.notice, self.path, 'dev', 'MAINTENANCE_ACTIVE')

    def test_exact_migration_delta_preserves_retired_rows(self):
        before = [{'version': '239_base.sql', 'checksum': 'd' * 64},
                  {'version': '100_retired.sql', 'checksum': 'f' * 64}]
        after = before + [{'version': '241_browser.sql', 'checksum': 'e' * 64}]
        common.verify_migrations(self.value, before, after)
        for invalid in (before, after[:-1] + [{'version': '241_browser.sql', 'checksum': 'f' * 64}], after[1:]):
            with self.assertRaises(RuntimeError):
                common.verify_migrations(self.value, before, invalid)

    def test_recovery_refuses_without_functional_acceptance(self):
        calls = []
        def call(route, method='GET', body=None):
            calls.append(method)
            return {'title': '系统维护通知', 'status': 'active'}
        with self.assertRaises(RuntimeError):
            release_api.execute(self.args, call)
        self.assertEqual(calls, ['GET'])

    def test_failed_recovery_readback_does_not_archive_maintenance(self):
        acceptance = self.root / 'acceptance.json'
        self.write_receipt(acceptance, status='FUNCTIONAL_ACCEPTANCE', health_passed=True, functional_checks_passed=True)
        self.args.acceptance = acceptance
        calls = []
        def call(route, method='GET', body=None):
            calls.append((route, method, body))
            if method == 'POST':
                return {'id': 24}
            if route.endswith('/24'):
                raise RuntimeError('simulated transport failure')
            return {'title': '系统维护通知', 'status': 'active'}
        with self.assertRaises(RuntimeError):
            release_api.execute(self.args, call)
        self.assertFalse(any(method == 'PUT' for _, method, _ in calls))

    def test_recovery_archives_only_after_confirmed_publication(self):
        acceptance = self.root / 'acceptance.json'
        self.write_receipt(acceptance, status='FUNCTIONAL_ACCEPTANCE', health_passed=True, functional_checks_passed=True)
        self.args.acceptance = acceptance
        calls = []
        archived = False
        def call(route, method='GET', body=None):
            nonlocal archived
            calls.append((route, method))
            if method == 'POST':
                return {'id': 24}
            if method == 'PUT':
                archived = True
                return {}
            if route.endswith('/24'):
                return {'id': 24, 'status': 'active', 'notify_mode': 'popup'}
            return {'title': '系统维护通知', 'status': 'archived' if archived else 'active'}
        self.assertEqual(release_api.execute(self.args, call)['status'], 'RECOVERED')
        self.assertLess(calls.index(('/api/v1/admin/announcements/24', 'GET')),
                        calls.index(('/api/v1/admin/announcements/23', 'PUT')))

    def simulate_replace(self, bad_migration=False, tampered_backup=False):
        deploy, backup = self.root / 'deploy', self.root / 'backup'
        deploy.mkdir()
        backup.mkdir()
        old_id = 'sha256:' + 'f' * 64
        (deploy / '.env').write_text('SUB2API_IMAGE=old\n')
        before = {'Id': 'container-before', 'Image': old_id,
                  'Config': {'Env': ['SAFE_SETTING=1']}, 'Mounts': [],
                  'HostConfig': {'PortBindings': {}}, 'State': {'Status': 'running', 'Health': {'Status': 'healthy'}}}
        after = dict(before, Id='container-after', Image=self.value['image_id'])
        old_config = {'services': {'sub2api': {'image': 'old', 'environment': {'SAFE_SETTING': '1'}}}}
        old_migrations = [{'version': '239_base.sql', 'checksum': 'd' * 64}]
        new_migrations = old_migrations + [{'version': '241_browser.sql', 'checksum': 'e' * 64}]
        (backup / 'app-inspect.json').write_text(json.dumps([before]))
        (backup / 'compose-resolved.json').write_text(json.dumps(old_config))
        (backup / 'migrations.json').write_text(json.dumps(old_migrations))
        with tarfile.open(backup / 'compose-config.tar.gz', 'w:gz') as archive:
            archive.add(deploy / '.env', arcname='.env')
        for name in ('db.dump', 'db-toc.txt', 'app-data.tar.gz'):
            (backup / name).write_bytes(b'fixture-only')
        manifest = {'schema_version': 2, 'candidate_sha256': common.sha(self.path), 'environment': 'dev',
                    'deploy_directory': str(deploy), 'started_at': time.time(), 'toc_entries': 1,
                    'old_image_id': old_id, 'files': {p.name: {'bytes': p.stat().st_size, 'sha256': common.sha(p)}
                                                    for p in backup.iterdir()}}
        (backup / 'manifest.json').write_text(json.dumps(manifest))
        if tampered_backup:
            (backup / 'db.dump').write_bytes(b'tampered')
        proof = self.root / 'rollback.json'
        proof.write_text(json.dumps({'candidate_sha256': common.sha(self.path), 'rollback_image_id': old_id,
                                     'application_rollback_on_migrated_database_passed': True}))
        state = {'up': False, 'rolled_back': False}
        def run(argv, env=None):
            if argv[:3] == ['docker', 'image', 'inspect']:
                return json.dumps([{'Id': self.value['image_id'], 'Config': {'Labels': {
                    'org.opencontainers.image.revision': self.value['commit'],
                    'org.opencontainers.image.version': self.value['version']}}, 'Architecture': 'amd64', 'Os': 'linux'}])
            if argv[:2] == ['docker', 'inspect']:
                return json.dumps([before if not state['up'] or state['rolled_back'] else after])
            if 'config' in argv:
                config = copy.deepcopy(old_config)
                if 'SUB2API_IMAGE=old' not in (deploy / '.env').read_text():
                    config['services']['sub2api']['image'] = self.value['image']
                return json.dumps(config)
            if 'up' in argv:
                state['up'] = True
                return ''
            raise AssertionError('Unexpected command')
        def rollback(argv, **kwargs):
            self.assertIn('--no-deps', argv)
            self.assertEqual(kwargs['env']['SUB2API_IMAGE'], old_id)
            state['rolled_back'] = True
            return argparse.Namespace(returncode=0)
        previous_cwd = Path.cwd()
        self.addCleanup(os.chdir, previous_cwd)
        argv = ['replace_app.py', str(deploy), str(self.path), old_id, str(backup), 'dev', str(self.notice), str(proof)]
        with patch.object(sys, 'argv', argv), patch.object(replace_app, 'run', side_effect=run), \
                patch.object(replace_app, 'read_migrations', side_effect=[old_migrations, old_migrations if bad_migration else new_migrations]), \
                patch.object(replace_app.subprocess, 'run', side_effect=rollback), patch('sys.stdout', new_callable=io.StringIO):
            if bad_migration or tampered_backup:
                with self.assertRaises(RuntimeError):
                    replace_app.main()
            else:
                replace_app.main()
        return state, deploy

    def test_tampered_backup_stops_before_container_command(self):
        state, _ = self.simulate_replace(tampered_backup=True)
        self.assertFalse(state['up'])

    def test_missing_migration_rolls_back_application_and_environment(self):
        state, deploy = self.simulate_replace(bad_migration=True)
        self.assertTrue(state['rolled_back'])
        self.assertEqual((deploy / '.env').read_text(), 'SUB2API_IMAGE=old\n')

    def test_candidate_success_keeps_new_image_and_exact_migrations(self):
        state, deploy = self.simulate_replace()
        self.assertTrue(state['up'])
        self.assertFalse(state['rolled_back'])
        self.assertIn(self.value['image'], (deploy / '.env').read_text())


if __name__ == '__main__':
    unittest.main()
