"""Runtime warnings are observable, never merchant authorization evidence."""
import io
import json
import sys
import unittest
from pathlib import Path
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parent))
import test_ldxp_http_restock as fixtures

r = fixtures.restock


class RuntimeNotificationTests(unittest.TestCase):
    setUp = fixtures.RestockTests.setUp

    def test_verification_page_has_only_a_safe_notification_signal(self):
        merchant = r.probe.Merchant(self.private / 'unused.json')
        raw = b'<script>CF_APP_WAF captcha private-token-or-card</script>'
        response = io.BytesIO(raw)
        response.status = 200
        response.headers = {'Content-Type': 'text/html; charset=utf-8'}
        with patch.object(merchant.http, 'open', return_value=response):
            with self.assertRaises(r.probe.ProbeError) as error:
                merchant.post('/merchantApi/user/userinfo', {})
        self.assertEqual(error.exception.kind, 'non_json')
        self.assertEqual(error.exception.diagnostics['response_kind'], 'browser_verification')
        self.assertNotIn('private-token-or-card', json.dumps(error.exception.diagnostics))

    def test_generic_html_does_not_claim_a_captcha(self):
        response = io.BytesIO()
        response.status = 200
        response.headers = {'Content-Type': 'text/html'}
        data = r.probe.response_diagnostics('/merchantApi/user/userinfo', response, b'<html>maintenance</html>')
        self.assertNotIn('response_kind', data)

    def test_paused_worker_reports_verification_without_authentication_heartbeat(self):
        self.state.update(paused=True, reason='non_json')
        r.save_state(self.private, self.state)
        report = {'status': 'PAUSED', 'error': 'non_json',
                  'diagnostics': {'response_kind': 'browser_verification', 'checked_at': 1789542000,
                                  'body': 'private-token-or-card'}, 'next_recheck_at': 1789542900}
        backend = fixtures.Backend([])
        r.publish_runtime(self.private, backend, report)
        payload = backend.bodies('/runtime')[0]
        self.assertEqual(payload['state'], 'paused')
        self.assertEqual(payload['reason'], 'non_json')
        self.assertTrue(payload['browser_verification_required'])
        self.assertIn('checked_at', payload)
        self.assertIn('next_check_at', payload)
        self.assertNotIn('private-', json.dumps(payload))
        self.assertEqual(backend.bodies('/heartbeat'), [])
        self.assertEqual(backend.bodies('/resume'), [])
        self.assertTrue(r.read_state(self.private)['paused'])

    def test_success_report_clears_only_the_display_warning(self):
        self.state.update(last_success_at=1789543000)
        r.save_state(self.private, self.state)
        backend = fixtures.Backend([])
        report = {'status': 'RUNNING'}
        r.publish_runtime(self.private, backend, report)
        self.assertEqual(backend.bodies('/runtime')[0]['state'], 'running')
        self.assertFalse(backend.bodies('/runtime')[0]['browser_verification_required'])
        self.assertEqual(backend.bodies('/heartbeat'), [])
        self.assertEqual(backend.bodies('/resume'), [])

    def test_failed_notification_cannot_change_worker_pause_or_expose_errors(self):
        self.state.update(paused=True, reason='inventory_mismatch')
        r.save_state(self.private, self.state)
        backend = fixtures.Backend([])
        report = {'status': 'PAUSED'}
        with patch.object(backend, 'call', side_effect=r.RestockError('backend_error')):
            r.publish_runtime(self.private, backend, report)
        self.assertFalse(report['runtime_reported'])
        self.assertEqual(r.read_state(self.private)['reason'], 'inventory_mismatch')


if __name__ == '__main__':
    unittest.main()
