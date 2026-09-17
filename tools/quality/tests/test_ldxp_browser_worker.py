"""Synthetic browser merchant, worker injection, and owned LaunchAgent migration tests."""
import hashlib
from pathlib import Path
import plistlib
import sys
from types import SimpleNamespace
import unittest
from unittest.mock import Mock, patch

sys.path.insert(0, str(Path(__file__).resolve().parent))
import test_ldxp_http_restock as fixtures
import test_ldxp_manual_recheck as manual
import ldxp_browser_restock as browser_worker

r = fixtures.restock


class BrowserMerchantTests(unittest.TestCase):
    setUp = fixtures.RestockTests.setUp

    def test_expected_subject_binds_browser_identity_and_load_save_do_not_import_http_session(self):
        session = Mock()
        session.request.return_value = {'id': 'synthetic-merchant'}
        digest = hashlib.sha256(b'synthetic-merchant').hexdigest()
        merchant = browser_worker.BrowserMerchant(session, digest)
        with patch.object(r.probe, 'private_read', side_effect=AssertionError('No HTTP session import')), \
                patch.object(r.probe, 'write_private', side_effect=AssertionError('No HTTP session export')):
            self.assertIs(merchant.load(), merchant)
            merchant.save()
            self.assertEqual(merchant.post('/merchantApi/user/userinfo', {}), {'id': 'synthetic-merchant'})
        for value in ({'id': 'foreign-merchant'}, {}, None):
            with self.subTest(value=value):
                session.request.return_value = value
                with self.assertRaises(r.RestockError) as error:
                    merchant.post('/merchantApi/user/userinfo', {})
                self.assertEqual(error.exception.kind, 'binding_changed')

    def test_inventory_keeps_core_double_scan_and_goods_identity_validation(self):
        rows = [{'id': i + 1, 'goods_id': fixtures.GOODS_ID, 'status': '0', 'secret': code}
                for i, code in enumerate(fixtures.CODES)]
        session = Mock()
        session.request.return_value = {'total': 2, 'list': rows}
        merchant = browser_worker.BrowserMerchant(session)
        self.assertEqual(merchant.inventory_hashes(fixtures.GOODS_ID), fixtures.hashes(fixtures.CODES))
        self.assertEqual(session.request.call_count, 2)
        for call in session.request.call_args_list:
            self.assertEqual(call.args[0], '/merchantApi/goodsCardStorage/list')
            self.assertEqual(call.args[1]['status'], '0')
        session.request.return_value = {'total': 1, 'list': [rows[0] | {'goods_id': 99}]}
        with self.assertRaises(r.probe.ProbeError) as error:
            merchant.inventory_hashes(fixtures.GOODS_ID)
        self.assertEqual(error.exception.kind, 'invalid_response')

    def test_browser_upload_retains_core_batch_limit_and_code_validation(self):
        session = Mock()
        merchant = browser_worker.BrowserMerchant(session)
        codes = [format(i, '032x') for i in range(21)]
        for invalid in ([], codes, [codes[0], codes[0]], ['invalid-code']):
            with self.subTest(count=len(invalid)):
                with self.assertRaises(r.RestockError):
                    merchant.upload(fixtures.GOODS_ID, invalid)
        session.request.assert_not_called()
        merchant.upload(fixtures.GOODS_ID, codes[:20])
        route, body = session.request.call_args.args
        self.assertEqual(route, '/merchantApi/GoodsCardStorage/add')
        self.assertEqual(body['goods_id'], fixtures.GOODS_ID)
        self.assertEqual(body['content'].splitlines(), codes[:20])
        self.assertEqual(body['remove_repeat'], 1)

    def test_expected_subject_file_accepts_only_fixed_origin_and_sha256(self):
        self.assertIsNone(browser_worker.expected_subject(self.private))
        path = self.private / 'dedup-capability.json'
        for value in ({'origin': 'https://evil.invalid', 'merchant_subject_sha256': 'a' * 64},
                      {'origin': r.probe.ORIGIN, 'merchant_subject_sha256': 'not-a-digest'}):
            r.probe.write_private(path, value)
            with self.assertRaises(r.RestockError):
                browser_worker.expected_subject(self.private)
        r.probe.write_private(path, {'origin': r.probe.ORIGIN, 'merchant_subject_sha256': 'a' * 64})
        self.assertEqual(browser_worker.expected_subject(self.private), 'a' * 64)


class BrowserWorkerTests(unittest.TestCase):
    setUp = fixtures.RestockTests.setUp

    def test_factory_replaces_http_transport_and_tick_runs_between_normal_cycles(self):
        merchant = fixtures.Merchant(fixtures.CODES)
        backend = manual.RecheckBackend([fixtures.inventory(2)])
        factory, tick = Mock(return_value=merchant), Mock()
        clock = manual.PollClock(3)
        args = SimpleNamespace(private_dir=self.private, session=None, evidence=self.root / 'evidence', notify=False)
        with patch.object(r, 'device_backend', return_value=(backend, fixtures.DEVICE_ID)), \
                patch.object(r, 'Merchant', side_effect=AssertionError('No HTTP factory')), \
                patch.object(r.time, 'monotonic', side_effect=lambda: clock.now), \
                patch.object(r, 'receipt'), patch.object(r, 'attention'):
            self.assertEqual(r.run_worker(args, {'interval_seconds': 60}, clock,
                             merchant_factory=factory, on_tick=tick, execution_mode='browser'), 0)
        factory.assert_called_once_with()
        self.assertEqual(tick.call_count, 3)
        self.assertEqual(clock.waits, [5] * 3)
        self.assertEqual(backend.bodies('/runtime')[0]['execution_mode'], 'browser')
        self.assertEqual(merchant.uploads, [])

    def test_cached_recheck_ack_never_constructs_browser_or_http_merchant(self):
        state = r.read_state(self.private)
        verdict = {'state': 'passed', 'reason': '', 'resumed': False}
        state.update(manual_recheck_result={'id': manual.REQUEST['id'], 'result': verdict},
                     last_report={'status': 'VERIFIED', 'uploaded': 0})
        r.save_state(self.private, state)
        backend = manual.RecheckBackend()
        factory = Mock(side_effect=AssertionError('No browser access during ACK replay'))
        with patch.object(r, 'Merchant', side_effect=AssertionError('No HTTP fallback')):
            report = r.process_recheck(self.private, None, backend, fixtures.DEVICE_ID, manual.REQUEST, None,
                                       merchant_factory=factory, execution_mode='browser')
        factory.assert_not_called()
        self.assertTrue(report['recheck_result_reported'])
        self.assertEqual(backend.bodies('/recheck/' + manual.REQUEST['id'] + '/result'), [verdict])
        self.assertEqual(backend.bodies('/runtime')[0]['execution_mode'], 'browser')
        self.assertEqual(backend.bodies('/resume'), [])

    def test_browser_run_shares_one_session_and_closes_resources_after_worker_error(self):
        args = SimpleNamespace(private_dir=self.private, browser_home=self.root / 'browser')
        session, control = Mock(), Mock()
        subject = 'a' * 64

        def worker(_args, _config, _stopped, **kwargs):
            merchant = kwargs['merchant_factory']()
            self.assertIsInstance(merchant, browser_worker.BrowserMerchant)
            self.assertIs(merchant.session, session)
            self.assertEqual(merchant.expected_subject, subject)
            self.assertEqual(kwargs['on_tick'], control.tick)
            self.assertEqual(kwargs['execution_mode'], 'browser')
            raise r.RestockError('backend_error')

        session.open.side_effect = r.probe.ProbeError('verification_required')
        with patch.object(browser_worker, 'expected_subject', return_value=subject), \
                patch.object(browser_worker, 'BrowserSession', return_value=session), \
                patch.object(browser_worker, 'BrowserControl', return_value=control), \
                patch.object(r, 'run_worker', side_effect=worker):
            with self.assertRaises(r.RestockError):
                browser_worker.run(args, {'interval_seconds': 60}, Mock())
        control.start.assert_called_once_with()
        session.open.assert_called_once_with()
        control.close.assert_called_once_with()
        session.close.assert_called_once_with()


class BrowserServiceInstallTests(unittest.TestCase):
    setUp = fixtures.RestockTests.setUp

    def setup_agents(self, old=True, browser_script=None):
        self.state.update(paused=True, reason='verification_required')
        r.save_state(self.private, self.state)
        r.probe.write_private(self.private / 'pause', {'requested_at': 1})
        fake_home = self.root / 'fake-home'
        agents = fake_home / 'Library/LaunchAgents'
        agents.mkdir(parents=True)
        if old:
            (agents / (browser_worker.OLD_LABEL + '.plist')).write_bytes(plistlib.dumps({
                'Label': browser_worker.OLD_LABEL,
                'ProgramArguments': [sys.executable, '-B', str(Path(r.__file__).resolve()), 'run'],
            }))
        if browser_script:
            (agents / (browser_worker.LABEL + '.plist')).write_bytes(plistlib.dumps({
                'Label': browser_worker.LABEL, 'ProgramArguments': [browser_script],
            }))
        args = SimpleNamespace(private_dir=self.private, browser_home=self.root / 'profile-home',
                               config=self.root / 'config.json')
        return args, fake_home, agents

    def launchctl(self):
        bootstrapped = False

        def call(command, **kwargs):
            nonlocal bootstrapped
            if command[1] == 'bootstrap':
                bootstrapped = True
                return SimpleNamespace(returncode=0)
            if command[1] == 'bootout':
                return SimpleNamespace(returncode=0)
            if command[1] == 'print' and command[2].endswith('/' + browser_worker.OLD_LABEL):
                return SimpleNamespace(returncode=0, stdout=str(Path(r.__file__).resolve()).encode())
            return SimpleNamespace(returncode=0 if bootstrapped else 1,
                                   stdout=str(Path(browser_worker.__file__).resolve()).encode())
        return call

    def install(self, args, fake_home, transport):
        with patch.object(browser_worker.Path, 'home', return_value=fake_home), \
                patch.object(browser_worker.sys, 'platform', 'darwin'), \
                patch('importlib.metadata.version', return_value='1.58.0'), \
                patch.object(browser_worker.subprocess, 'run', side_effect=transport) as calls:
            report = browser_worker.install_service(args, {'interval_seconds': 60})
        return report, calls

    def test_recognized_old_agent_is_backed_up_and_retired_without_changing_pause(self):
        args, fake_home, agents = self.setup_agents()
        old = agents / (browser_worker.OLD_LABEL + '.plist')
        prior = old.read_bytes()
        state, pause = (self.private / 'state.json').read_bytes(), (self.private / 'pause').read_bytes()
        report, calls = self.install(args, fake_home, self.launchctl())
        self.assertEqual(report['execution_mode'], 'browser')
        self.assertFalse(old.exists())
        backups = list((self.private / 'backups').glob('*/' + old.name))
        self.assertEqual(len(backups), 1)
        self.assertEqual(backups[0].read_bytes(), prior)
        self.assertEqual((self.private / 'state.json').read_bytes(), state)
        self.assertEqual((self.private / 'pause').read_bytes(), pause)
        spec = plistlib.loads((agents / (browser_worker.LABEL + '.plist')).read_bytes())
        self.assertEqual(spec['ProgramArguments'][3], 'run')
        self.assertIn('--browser-home', spec['ProgramArguments'])
        self.assertNotIn('--session', spec['ProgramArguments'])
        self.assertEqual(spec['ProcessType'], 'Interactive')
        self.assertTrue(any(call.args[0][1] == 'bootout' for call in calls.call_args_list))

    def test_unknown_old_agent_or_unowned_running_label_cannot_be_retired(self):
        args, fake_home, agents = self.setup_agents()
        path = agents / (browser_worker.OLD_LABEL + '.plist')
        unknown = plistlib.dumps({'ProgramArguments': ['/usr/bin/other']})
        path.write_bytes(unknown)
        calls = []

        def transport(command, **kwargs):
            calls.append(command)
            return SimpleNamespace(returncode=0, stdout=b'program = /usr/bin/other')

        with self.assertRaises(r.RestockError) as error:
            self.install(args, fake_home, transport)
        self.assertEqual(error.exception.kind, 'binding_changed')
        self.assertEqual(path.read_bytes(), unknown)
        self.assertEqual([command[1] for command in calls], ['print'])
        path.unlink()
        calls.clear()
        with self.assertRaises(r.RestockError):
            self.install(args, fake_home, transport)
        self.assertEqual([command[1] for command in calls], ['print'])
        self.assertFalse((agents / (browser_worker.LABEL + '.plist')).exists())

    def test_unknown_browser_agent_is_detected_before_retiring_working_http_agent(self):
        args, fake_home, agents = self.setup_agents(browser_script='/usr/bin/unrelated')
        old = agents / (browser_worker.OLD_LABEL + '.plist')
        prior = old.read_bytes()
        calls = []
        real = self.launchctl()

        def transport(command, **kwargs):
            calls.append(command)
            return real(command, **kwargs)

        with self.assertRaises(r.RestockError) as error:
            self.install(args, fake_home, transport)
        self.assertEqual(error.exception.kind, 'binding_changed')
        self.assertFalse(any(command[1] == 'bootout' for command in calls))
        self.assertEqual(old.read_bytes(), prior)

    def test_backup_failure_does_not_unload_the_existing_service(self):
        args, fake_home, agents = self.setup_agents()
        calls = []
        real = self.launchctl()

        def transport(command, **kwargs):
            calls.append(command)
            return real(command, **kwargs)

        with patch.object(browser_worker.shutil, 'copy2', side_effect=OSError('synthetic backup failure')):
            with self.assertRaises((OSError, r.RestockError)):
                self.install(args, fake_home, transport)
        self.assertFalse(any(command[1] == 'bootout' for command in calls))
        self.assertTrue((agents / (browser_worker.OLD_LABEL + '.plist')).exists())


if __name__ == '__main__':
    unittest.main()
