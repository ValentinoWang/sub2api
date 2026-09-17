"""Read-only recovery after a non-JSON gateway response; no challenge execution."""
import hashlib
import io
import json
import sys
from pathlib import Path
import unittest
from unittest.mock import patch
from urllib.error import HTTPError

sys.path.insert(0, str(Path(__file__).resolve().parent))
import test_ldxp_http_restock as base
import test_ldxp_http_recovery as fixtures

r = base.restock


class ResponseRecoveryTests(unittest.TestCase):
    setUp = base.RestockTests.setUp
    saved = base.RestockTests.saved

    def seed(self, reason='non_json'):
        self.state.update(paused=True, reason=reason)
        r.save_state(self.private, self.state)
        merchant = fixtures.RecoveryMerchant(base.CODES)
        backend = fixtures.RecoveryBackend(merchant, pending={})
        backend.known[base.GOODS_ID] = set(base.hashes(base.CODES))
        return merchant, backend

    def cycle(self, merchant, backend, at=fixtures.NOW, **kwargs):
        with patch.object(r.time, 'time', return_value=at):
            return r.cycle(self.private, merchant, backend, base.DEVICE_ID,
                           recovery=fixtures.POLICY, **kwargs)

    def test_html_pause_recovers_only_after_authorization_and_inventory_pass(self):
        merchant, backend = self.seed()
        report = self.cycle(merchant, backend)
        self.assertEqual(report['status'], 'RUNNING')
        self.assertFalse(self.saved()['paused'])
        self.assertTrue(backend.bodies('/inventory'))
        self.assertEqual(backend.bodies('/claim'), [])
        self.assertEqual(merchant.uploads, [])

    def test_repeated_html_is_read_only_and_backs_off_across_process_state(self):
        merchant, backend = self.seed()
        merchant.login_error = r.probe.ProbeError('non_json')
        first = self.cycle(merchant, backend)
        self.assertEqual(first['status'], 'PAUSED')
        self.assertEqual(first['next_recheck_at'], fixtures.NOW + 60)
        self.assertEqual(backend.calls, [])
        self.assertEqual(merchant.uploads, [])
        fresh = fixtures.RecoveryMerchant(base.CODES)
        fresh.login_error = r.probe.ProbeError('non_json')
        with patch.object(fresh, 'post', wraps=fresh.post) as request:
            early = self.cycle(fresh, backend, fixtures.NOW + 59)
            request.assert_not_called()
        self.assertEqual(early['next_recheck_at'], first['next_recheck_at'])
        second = self.cycle(fresh, backend, fixtures.NOW + 60)
        self.assertEqual(second['next_recheck_at'], fixtures.NOW + 180)
        fresh.login_error = None
        final = self.cycle(fresh, backend, fixtures.NOW + 180)
        self.assertEqual(final['status'], 'RUNNING')
        self.assertNotIn('response_recheck', self.saved())
        self.assertEqual(fresh.uploads, [])

    def test_recovery_does_not_override_login_or_inventory_failure(self):
        for cause in ('login_required', 'inventory_mismatch'):
            with self.subTest(cause=cause):
                merchant, backend = self.seed()
                if cause == 'login_required':
                    merchant.login_error = r.probe.ProbeError(cause)
                else:
                    merchant.stock[base.GOODS_ID].append('f' * 32)
                report = self.cycle(merchant, backend)
                self.assertEqual(report['error'], cause)
                self.assertEqual(report['status'], 'PAUSED')
                self.assertEqual(backend.bodies('/resume'), [])
                self.assertEqual(backend.bodies('/claim'), [])
                self.assertEqual(merchant.uploads, [])

    def test_html_check_cannot_convert_manual_pause_into_automatic_recovery(self):
        for protected in ('manual', 'login_required', 'inventory_mismatch', 'backend_auth'):
            with self.subTest(protected=protected):
                merchant, backend = self.seed(protected)
                merchant.login_error = r.probe.ProbeError('non_json')
                checked = self.cycle(merchant, backend, write=False)
                self.assertEqual(checked['error'], protected)
                self.assertEqual(checked['verification_error'], 'non_json')
                merchant.login_error = None
                self.assertEqual(self.cycle(merchant, backend)['error'], protected)
                self.assertEqual(merchant.uploads, [])

    def test_manual_pause_file_prevents_recheck_and_writes(self):
        merchant, backend = self.seed()
        r.probe.write_private(self.private / 'pause', {'requested_at': fixtures.NOW})
        with patch.object(merchant, 'post') as request:
            self.assertEqual(self.cycle(merchant, backend)['error'], 'manual')
            request.assert_not_called()
        self.assertEqual(backend.calls, [])

    def test_response_metadata_does_not_include_body_or_credentials(self):
        client = r.probe.Merchant(self.private / 'unused-session.json')
        client.token = 'private-token-sentinel'
        raw = b'<script>private-token-sentinel private-code-sentinel captcha</script>'
        response = io.BytesIO(raw)
        response.status = 200
        response.headers = {'Content-Type': 'text/html; charset=utf-8'}
        with patch.object(client.http, 'open', return_value=response):
            with self.assertRaises(r.probe.ProbeError) as error:
                client.post('/merchantApi/user/userinfo', {})
        diagnostic = error.exception.diagnostics
        self.assertEqual(error.exception.kind, 'non_json')
        self.assertEqual(diagnostic['http_status'], 200)
        self.assertEqual(diagnostic['content_type'], 'text/html')
        self.assertEqual(diagnostic['route'], '/merchantApi/user/userinfo')
        self.assertEqual(diagnostic['body_sha256'], hashlib.sha256(raw).hexdigest())
        self.assertNotIn('private-', json.dumps(diagnostic))
        self.assertNotIn('captcha', json.dumps(diagnostic))

    def test_only_transient_http_statuses_are_recheckable(self):
        for status in (401, 403, 429, 500, 502, 503, 504):
            with self.subTest(status=status):
                client = r.probe.Merchant(self.private / 'unused-session.json')
                failure = HTTPError('https://www.ldxp.cn/merchantApi/user/userinfo', status,
                                    'private-response', {}, io.BytesIO(b'private-body'))
                with patch.object(client.http, 'open', side_effect=failure):
                    with self.assertRaises(r.probe.ProbeError) as error:
                        client.post('/merchantApi/user/userinfo', {})
                self.assertEqual(error.exception.kind, 'login_required' if status in (401, 403) else 'upstream_unavailable')
                self.assertNotIn('private-', json.dumps(error.exception.diagnostics))


if __name__ == '__main__':
    unittest.main()
