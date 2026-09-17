"""Synthetic manual verification, durable acknowledgements, and daemon scheduling."""
import copy
from pathlib import Path
import plistlib
import sys
from types import SimpleNamespace
import unittest
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parent))
import test_ldxp_http_restock as fixtures

r = fixtures.restock
NOW = 2_000_000_000
REQUEST = {'id': 'c' * 32, 'device_id': fixtures.DEVICE_ID, 'state': 'checking'}


class RecheckBackend(fixtures.Backend):
    def __init__(self, inventories=()):
        super().__init__(inventories)
        self.requests = []
        self.ack_errors = []

    def call(self, route, body=None, method=None):
        if route == '/recheck/claim':
            self.calls.append((route, copy.deepcopy(body), method))
            value = self.requests.pop(0) if self.requests else None
            if isinstance(value, Exception):
                raise value
            return {'recheck': copy.deepcopy(value)}
        if route.startswith('/recheck/') and route.endswith('/result'):
            self.calls.append((route, copy.deepcopy(body), method))
            if self.ack_errors:
                error = self.ack_errors.pop(0)
                if error:
                    raise error
            return {'recheck': {'id': route.split('/')[2], **body}}
        return super().call(route, body, method)


class PollClock:
    def __init__(self, polls, on_wait=None):
        self.now = 0
        self.polls = polls
        self.waits = []
        self.on_wait = on_wait

    def is_set(self):
        return len(self.waits) >= self.polls

    def wait(self, seconds):
        if self.on_wait:
            self.on_wait()
        self.waits.append(seconds)
        self.now += seconds
        return self.is_set()


class ManualRecheckTests(unittest.TestCase):
    setUp = fixtures.RestockTests.setUp
    saved = fixtures.RestockTests.saved
    service_fixture = fixtures.RestockTests.service_fixture

    def seed(self, reason='non_json', codes=fixtures.CODES):
        state = copy.deepcopy(self.state)
        state.update(paused=bool(reason), reason=reason,
                     response_recheck={'attempts': 3, 'next_at': NOW + 900})
        if not reason:
            state.pop('response_recheck', None)
        r.save_state(self.private, state)
        return fixtures.Merchant(codes), RecheckBackend([fixtures.inventory(len(codes))])

    def verify(self, merchant, backend):
        with patch.object(r.time, 'time', return_value=NOW):
            return r.check_requested_verification(self.private, merchant, backend, fixtures.DEVICE_ID, None)

    def process(self, merchant, backend, request=REQUEST):
        with patch.object(r.time, 'time', return_value=NOW):
            return r.process_recheck(self.private, merchant, backend, fixtures.DEVICE_ID, request, None)

    def assert_no_stock_writes(self, merchant, backend):
        self.assertEqual(merchant.uploads, [])
        self.assertEqual(backend.bodies('/claim'), [])
        self.assertFalse(any(route.startswith('/batches/') for route, _, _ in backend.calls))

    def worker(self, backend, clock, merchant=None, load_error=None):
        args = SimpleNamespace(private_dir=self.private, session=self.private / 'synthetic-session.json',
                               evidence=self.root / 'evidence', notify=False)
        reports = []
        with patch.object(r, 'device_backend', return_value=(backend, fixtures.DEVICE_ID)), \
                patch.object(r, 'Merchant') as factory, \
                patch.object(r.time, 'monotonic', side_effect=lambda: clock.now), \
                patch.object(r.time, 'time', side_effect=lambda: NOW + clock.now), \
                patch.object(r, 'receipt', side_effect=lambda report, evidence: reports.append(copy.deepcopy(report))), \
                patch.object(r, 'attention'):
            factory.return_value.load.return_value = merchant
            if load_error:
                factory.return_value.load.side_effect = load_error
            code = r.run_worker(args, {'interval_seconds': 60}, clock)
            loads = factory.return_value.load.call_count
        return code, reports, loads

    def test_manual_check_bypasses_waf_backoff_but_proves_login_and_complete_inventory(self):
        merchant, backend = self.seed()
        with patch.object(merchant, 'post', wraps=merchant.post) as login, \
                patch.object(merchant, 'inventory_hashes', wraps=merchant.inventory_hashes) as inventory:
            report, result = self.verify(merchant, backend)
        login.assert_called_once_with('/merchantApi/user/userinfo', {})
        inventory.assert_called_once_with(fixtures.GOODS_ID)
        self.assertEqual(backend.bodies('/inventory'), [{
            'goods_id': fixtures.GOODS_ID, 'complete': True,
            'total': 2, 'hashes': fixtures.hashes(fixtures.CODES),
        }])
        routes = [route for route, _, _ in backend.calls]
        self.assertLess(routes.index('/inventory'), routes.index('/resume'))
        self.assertEqual(result, {'state': 'passed', 'reason': '', 'resumed': True})
        self.assertEqual(report['status'], 'RUNNING')
        self.assertFalse(self.saved()['paused'])
        self.assertNotIn('response_recheck', self.saved())
        self.assert_no_stock_writes(merchant, backend)

    def test_failed_login_or_inventory_never_resumes_or_uploads(self):
        for failure in ('login_required', 'non_json', 'inventory_mismatch', 'uncertain'):
            with self.subTest(failure=failure):
                merchant, backend = self.seed()
                if failure in ('login_required', 'non_json'):
                    merchant.login_error = r.probe.ProbeError(failure)
                elif failure == 'inventory_mismatch':
                    backend.inventories = [fixtures.inventory(0, identity_verified=False, blocked=True)]
                else:
                    state = self.saved()
                    state['products'][str(fixtures.GOODS_ID)] = {'phase': 'uncertain', 'batch_id': 'synthetic-batch'}
                    r.save_state(self.private, state)
                    backend.inventories = [fixtures.inventory(2, blocked=True, pending_batch=fixtures.batch(status='uncertain'))]
                report, result = self.verify(merchant, backend)
                self.assertEqual(result, {'state': 'failed', 'reason': failure, 'resumed': False})
                self.assertEqual(report['status'], 'PAUSED')
                self.assertTrue(self.saved()['paused'])
                self.assertEqual(backend.bodies('/resume'), [])
                self.assert_no_stock_writes(merchant, backend)

    def test_every_product_must_pass_before_resume(self):
        merchant, backend = self.seed()
        other_id = 99
        other = fixtures.PRODUCT | {'goods_id': other_id, 'external_url': 'https://www.ldxp.cn/goods/99'}
        row = fixtures.ROW | {'id': other_id, 'name': 'Another synthetic product'}
        state = self.saved()
        state['pins'][str(other_id)] = r.identity(other, row)
        state['products'][str(other_id)] = {'phase': 'idle'}
        r.save_state(self.private, state)
        merchant.rows_data.append(row)
        backend.config['device']['goods_ids'].append(other_id)
        backend.config['products'].append(other)
        backend.inventories.append(fixtures.inventory(0, identity_verified=False, blocked=True))
        with patch.object(merchant, 'inventory_hashes', return_value=fixtures.hashes(fixtures.CODES)):
            _, result = self.verify(merchant, backend)
        self.assertEqual(result['state'], 'failed')
        self.assertEqual([body['goods_id'] for body in backend.bodies('/inventory')], [fixtures.GOODS_ID, other_id])
        self.assertEqual(backend.bodies('/resume'), [])
        self.assert_no_stock_writes(merchant, backend)

    def test_manual_pause_file_prevents_all_merchant_checks_and_survives(self):
        merchant, backend = self.seed()
        r.probe.write_private(self.private / 'pause', {'requested_at': NOW})
        original = (self.private / 'state.json').read_bytes()
        with patch.object(merchant, 'post') as login:
            _, result = self.verify(merchant, backend)
        login.assert_not_called()
        self.assertEqual(result, {'state': 'failed', 'reason': 'manual', 'resumed': False})
        self.assertEqual(backend.calls, [])
        self.assertTrue((self.private / 'pause').exists())
        self.assertEqual((self.private / 'state.json').read_bytes(), original)

    def test_protected_pause_cannot_be_cleared_by_successful_verification(self):
        for reason in ('manual', 'disabled', 'inventory_mismatch', 'backend_auth', 'recovery_exhausted'):
            with self.subTest(reason=reason):
                merchant, backend = self.seed(reason)
                report, result = self.verify(merchant, backend)
                self.assertEqual(result, {'state': 'passed', 'reason': '', 'resumed': False})
                self.assertEqual(report['status'], 'VERIFIED')
                self.assertTrue(self.saved()['paused'])
                self.assertEqual(self.saved()['reason'], reason)
                self.assertEqual(backend.bodies('/resume'), [])
                self.assert_no_stock_writes(merchant, backend)

    def test_disabled_backend_prevents_resume_after_valid_stock(self):
        merchant, backend = self.seed()
        backend.config['enabled'] = False
        _, result = self.verify(merchant, backend)
        self.assertEqual(result, {'state': 'passed', 'reason': '', 'resumed': False})
        self.assertTrue(self.saved()['paused'])
        self.assertEqual(self.saved()['reason'], 'disabled')
        self.assertEqual(backend.bodies('/resume'), [])
        self.assert_no_stock_writes(merchant, backend)

    def test_pause_arriving_during_scan_prevents_resume(self):
        merchant, backend = self.seed()

        def inventory_then_pause(goods_id):
            r.probe.write_private(self.private / 'pause', {'requested_at': NOW})
            return fixtures.hashes(fixtures.CODES)

        with patch.object(merchant, 'inventory_hashes', side_effect=inventory_then_pause):
            _, result = self.verify(merchant, backend)
        self.assertFalse(result['resumed'])
        self.assertEqual(self.saved()['reason'], 'manual')
        self.assertTrue((self.private / 'pause').exists())
        self.assertEqual(backend.bodies('/resume'), [])

    def test_lost_result_ack_replays_persisted_verdict_without_merchant_recheck(self):
        merchant, backend = self.seed()
        backend.ack_errors = [r.RestockError('backend_error'), None]
        first = self.process(merchant, backend)
        self.assertFalse(first['recheck_result_reported'])
        self.assertEqual(self.saved()['manual_recheck_result']['id'], REQUEST['id'])
        fresh = fixtures.Merchant()
        fresh.login_error = AssertionError('A saved verdict must not repeat merchant authorization')
        with patch.object(fresh, 'inventory_hashes', side_effect=AssertionError('No second scan')):
            second = self.process(fresh, backend)
        self.assertTrue(second['recheck_result_reported'])
        self.assertEqual(backend.bodies('/resume'), [{}])
        self.assertEqual(backend.bodies('/recheck/' + REQUEST['id'] + '/result'), [
            {'state': 'passed', 'reason': '', 'resumed': True},
            {'state': 'passed', 'reason': '', 'resumed': True},
        ])
        self.assert_no_stock_writes(fresh, backend)

    def test_new_request_id_requires_fresh_verification(self):
        merchant, backend = self.seed()
        self.process(merchant, backend)
        fresh = fixtures.Merchant()
        fresh.login_error = r.probe.ProbeError('login_required')
        report = self.process(fresh, backend, REQUEST | {'id': 'd' * 32})
        self.assertEqual(report['error'], 'login_required')
        self.assertEqual(backend.bodies('/recheck/' + 'd' * 32 + '/result'), [
            {'state': 'failed', 'reason': 'login_required', 'resumed': False},
        ])

    def test_failed_verdict_ack_replay_cannot_turn_into_a_new_success(self):
        merchant, backend = self.seed()
        merchant.login_error = r.probe.ProbeError('non_json')
        backend.ack_errors = [r.RestockError('backend_error'), None]
        self.assertFalse(self.process(merchant, backend)['recheck_result_reported'])
        fresh = fixtures.Merchant(fixtures.CODES)
        with patch.object(fresh, 'post', side_effect=AssertionError('No new verification for an existing verdict')):
            self.assertTrue(self.process(fresh, backend)['recheck_result_reported'])
        expected = {'state': 'failed', 'reason': 'non_json', 'resumed': False}
        self.assertEqual(backend.bodies('/recheck/' + REQUEST['id'] + '/result'), [expected, expected])
        self.assertEqual(backend.bodies('/resume'), [])

    def test_resume_failure_is_reported_failed_after_full_verification(self):
        merchant, backend = self.seed()
        original = backend.call

        def fail_resume(route, body=None, method=None):
            if route == '/resume':
                backend.calls.append((route, copy.deepcopy(body), method))
                raise r.RestockError('backend_error')
            return original(route, body, method)

        with patch.object(backend, 'call', side_effect=fail_resume):
            report = self.process(merchant, backend)
        self.assertEqual(report['error'], 'backend_error')
        self.assertTrue(self.saved()['paused'])
        self.assertEqual(backend.bodies('/recheck/' + REQUEST['id'] + '/result'), [
            {'state': 'failed', 'reason': 'backend_error', 'resumed': False},
        ])
        self.assert_no_stock_writes(merchant, backend)

    def test_invalid_request_identity_fails_closed_before_merchant_calls(self):
        invalid = [None, {}, REQUEST | {'device_id': 'another-device'}, REQUEST | {'state': 'queued'},
                   REQUEST | {'id': 'short'}, REQUEST | {'id': 'C' * 32},
                   REQUEST | {'id': None}, REQUEST | {'id': 17}]
        for request in invalid:
            with self.subTest(request=request):
                merchant, backend = self.seed()
                with patch.object(merchant, 'post') as login:
                    with self.assertRaises(r.RestockError) as error:
                        self.process(merchant, backend, request)
                self.assertEqual(error.exception.kind, 'backend_error')
                login.assert_not_called()
                self.assertEqual(backend.calls, [])

    def test_daemon_keeps_polling_paused_worker_and_releases_cycle_lock(self):
        merchant, backend = self.seed('')
        merchant.login_error = r.probe.ProbeError('non_json')
        clock = PollClock(14)

        def assert_poll_locks():
            with r.exclusive(self.private):
                pass
            with self.assertRaises(r.RestockError) as error:
                with r.exclusive(self.private, 'scheduler.lock'):
                    pass
            self.assertEqual(error.exception.kind, 'busy')

        clock.on_wait = assert_poll_locks
        with patch.object(merchant, 'post', wraps=merchant.post) as login:
            code, reports, loads = self.worker(backend, clock, merchant)
        self.assertEqual(code, 0)
        self.assertEqual(clock.waits, [5] * 14)
        self.assertEqual(len(backend.bodies('/recheck/claim')), 14)
        self.assertEqual(login.call_count, 2)
        self.assertEqual(loads, 2)
        self.assertEqual([report['status'] for report in reports], ['PAUSED', 'PAUSED'])
        with r.exclusive(self.private), r.exclusive(self.private, 'scheduler.lock'):
            pass
        self.assert_no_stock_writes(merchant, backend)

    def test_daemon_manual_request_checks_before_next_sixty_second_cycle(self):
        merchant, backend = self.seed()
        backend.requests = [None, None, REQUEST, None]
        clock = PollClock(4)
        with patch.object(merchant, 'post', wraps=merchant.post) as login:
            _, reports, _ = self.worker(backend, clock, merchant)
        login.assert_called_once()
        self.assertEqual(clock.waits, [5] * 4)
        self.assertEqual([report['status'] for report in reports], ['PAUSED', 'RUNNING'])
        self.assertEqual(backend.bodies('/resume'), [{}])
        self.assertEqual(len(backend.bodies('/recheck/' + REQUEST['id'] + '/result')), 1)
        self.assert_no_stock_writes(merchant, backend)

    def test_claim_failure_releases_lock_and_next_poll_continues(self):
        merchant, backend = self.seed()
        backend.requests = [r.RestockError('backend_error'), REQUEST]
        clock = PollClock(2, on_wait=lambda: self.assert_worker_lock_available())
        _, reports, _ = self.worker(backend, clock, merchant)
        self.assertEqual(len(backend.bodies('/recheck/claim')), 2)
        self.assertEqual(reports[-1]['status'], 'RUNNING')
        self.assert_no_stock_writes(merchant, backend)

    def assert_worker_lock_available(self):
        with r.exclusive(self.private):
            pass

    def test_cached_ack_does_not_depend_on_session_file_surviving_restart(self):
        merchant, backend = self.seed()
        backend.ack_errors = [r.RestockError('backend_error'), None]
        self.process(merchant, backend)
        backend.requests = [REQUEST]
        self.worker(backend, PollClock(1), load_error=r.probe.ProbeError('login_required'))
        self.assertEqual(backend.bodies('/recheck/' + REQUEST['id'] + '/result'), [
            {'state': 'passed', 'reason': '', 'resumed': True},
            {'state': 'passed', 'reason': '', 'resumed': True},
        ])

    def test_invalid_claim_is_not_acknowledged_even_when_session_is_missing(self):
        for claimed in (REQUEST | {'state': 'queued'}, REQUEST | {'id': None}, REQUEST | {'id': 17},
                        REQUEST | {'device_id': 'another-device'}):
            with self.subTest(claimed=claimed):
                _, backend = self.seed()
                backend.requests = [claimed]
                _, reports, _ = self.worker(backend, PollClock(1), load_error=r.probe.ProbeError('login_required'))
                self.assertEqual(reports[-1]['error'], 'backend_error')
                self.assertEqual(self.saved()['reason'], 'non_json')
                self.assertFalse(any(route.endswith('/result') for route, _, _ in backend.calls))
                self.assertEqual(backend.bodies('/resume'), [])

    def test_paused_service_install_preserves_pause_and_schedules_daemon(self):
        self.seed()
        r.probe.write_private(self.private / 'pause', {'requested_at': NOW})
        original = (self.private / 'state.json').read_bytes()
        pause = (self.private / 'pause').read_bytes()
        args, config, fake_home, destination = self.service_fixture()
        outcomes = [SimpleNamespace(returncode=1), SimpleNamespace(returncode=0), SimpleNamespace(returncode=0)]
        with patch.object(r.Path, 'home', return_value=fake_home), patch.object(r.sys, 'platform', 'darwin'), \
                patch.object(r.subprocess, 'run', side_effect=outcomes):
            report = r.install_service(args, config)
        self.assertEqual(report['status'], 'SCHEDULED')
        self.assertEqual((self.private / 'state.json').read_bytes(), original)
        self.assertEqual((self.private / 'pause').read_bytes(), pause)
        spec = plistlib.loads(destination.read_bytes())
        self.assertEqual(spec['ProgramArguments'][3], 'run')
        self.assertEqual(spec['StartInterval'], config['interval_seconds'])
        self.assertEqual(spec['Umask'], 0o077)


if __name__ == '__main__':
    unittest.main()
