"""Real Chromium, synthetic merchant only: persistent operator/worker session proof."""
import json
from pathlib import Path
import sys
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / 'commerce'))
sys.path.insert(0, str(Path(__file__).resolve().parent / 'tests'))
from playwright.sync_api import sync_playwright
import ldxp_browser_restock as browser
from ldxp_browser_session import BrowserSession
import test_ldxp_http_restock as fixtures


HTML = '''<!doctype html><html><body>
<h1>Synthetic merchant fixture — no live site</h1>
<div id="captcha-element">Synthetic operator handoff
<button id="complete">Complete synthetic verification</button></div>
<script>
const challenge = document.getElementById('captcha-element');
if (localStorage.getItem('auth-token')) challenge.remove();
document.getElementById('complete')?.addEventListener('click', () => {
 localStorage.setItem('auth-token', JSON.stringify({value:'synthetic-token',expiry:Date.now()+3600000}));
 document.cookie='fixture=approved;path=/;SameSite=Lax;Max-Age=3600'; challenge.remove();
});
</script></body></html>'''


class BrowserLiveTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        self.codes = []
        self.calls = []
        self.uploads = 0
        self.upload_block = False
        self.sessions = []
        self.addCleanup(lambda: [s.close() for s in reversed(self.sessions)])

    def route(self, route):
        request = route.request
        from urllib.parse import urlsplit
        parsed = urlsplit(request.url)
        if parsed.netloc != 'www.ldxp.cn':
            route.abort()
            return
        if request.method == 'GET' and parsed.path == '/merchant/':
            route.fulfill(status=200, content_type='text/html', body=HTML)
            return
        self.calls.append(parsed.path)
        headers = request.all_headers()
        self.assertEqual(headers.get('merchant-token'), 'synthetic-token')
        self.assertIn('fixture=approved', headers.get('cookie', ''))
        payload = request.post_data_json
        if parsed.path == '/merchantApi/user/userinfo':
            data = {'id': 'synthetic-merchant'}
        elif parsed.path == '/merchantApi/Goods/list':
            data = {'total': 1, 'list': [fixtures.ROW]}
        elif parsed.path == '/merchantApi/goodsCardStorage/list':
            data = {'total': len(self.codes), 'list': [
                {'id': index + 1, 'secret': code, 'goods_id': fixtures.GOODS_ID, 'status': '0'}
                for index, code in enumerate(self.codes)]}
        elif parsed.path == '/merchantApi/GoodsCardStorage/add':
            self.uploads += 1
            self.codes.extend(payload['content'].splitlines())
            if self.upload_block:
                route.fulfill(status=200, content_type='text/html', body='<html>CF_APP_WAF captcha</html>')
                return
            data = {}
        else:
            self.fail('Unexpected synthetic endpoint: ' + parsed.path)
        route.fulfill(status=200, content_type='application/json', body=json.dumps({'code': 1, 'data': data}))

    def factory(self):
        real = sync_playwright().start()

        def launch(*args, **kwargs):
            context = real.chromium.launch_persistent_context(*args, **kwargs)
            # Interception is installed before the very first navigation. No merchant is contacted.
            context.route('**/*', self.route)
            return context

        driver = SimpleNamespace(chromium=SimpleNamespace(launch_persistent_context=launch), stop=real.stop)
        return SimpleNamespace(start=lambda: driver)

    def session(self):
        session = BrowserSession(self.root / 'browser', playwright_factory=self.factory, headless=True)
        self.sessions.append(session)
        session.open()
        return session

    def state(self):
        directory = self.root / 'worker'
        value = {'schema': 1, 'site': browser.restock.probe.LOCAL_SITE, 'device_id': fixtures.DEVICE_ID,
                 'pins': {str(fixtures.GOODS_ID): {
                     'goods_id': fixtures.GOODS_ID, 'cny_amount': 5, 'usd_credit': 5,
                     'external_url': fixtures.PRODUCT['external_url'], 'merchant_name': fixtures.ROW['name'],
                 }}, 'paused': False, 'reason': '', 'target_stock': 2,
                 'products': {str(fixtures.GOODS_ID): {'phase': 'idle'}}}
        browser.restock.save_state(directory, value)
        return directory

    def test_operator_handoff_and_restock_use_same_context_then_persist_after_restart(self):
        session = self.session()
        merchant = browser.BrowserMerchant(session)
        with self.assertRaises(browser.restock.probe.ProbeError) as error:
            merchant.post('/merchantApi/user/userinfo', {})
        self.assertEqual(error.exception.kind, 'verification_required')
        self.assertEqual(self.calls, [])
        original_context = session.context
        session.page.evaluate("window.operatorDraft = 'keep this page'")
        session.open()
        self.assertEqual(session.page.evaluate('window.operatorDraft'), 'keep this page')
        session.page.get_by_role('button', name='Complete synthetic verification').click()
        with patch.object(browser.restock.probe, 'opener', side_effect=AssertionError('No independent HTTP client')):
            self.assertEqual(merchant.post('/merchantApi/user/userinfo', {})['id'], 'synthetic-merchant')
        self.assertIs(session.context, original_context)
        session.close()
        reopened = self.session()
        self.assertEqual(reopened.page.locator('#captcha-element').count(), 0)
        self.assertEqual(browser.BrowserMerchant(reopened).post('/merchantApi/user/userinfo', {})['id'], 'synthetic-merchant')

    def test_real_browser_keeps_existing_reconciliation_and_no_duplicate_upload(self):
        session = self.session()
        merchant = browser.BrowserMerchant(session)
        directory = self.state()
        backend = fixtures.Backend([fixtures.inventory(), fixtures.inventory(2, batch_resolved=True)], fixtures.batch())
        paused = browser.restock.cycle(directory, merchant, backend, fixtures.DEVICE_ID)
        self.assertEqual(paused['error'], 'verification_required')
        self.assertEqual(backend.bodies('/claim'), [])
        session.page.get_by_role('button', name='Complete synthetic verification').click()
        check_backend = fixtures.Backend([fixtures.inventory()])
        _, result = browser.restock.check_requested_verification(directory, merchant, check_backend, fixtures.DEVICE_ID, None)
        self.assertEqual(result, {'state': 'passed', 'reason': '', 'resumed': True})
        self.assertEqual(self.uploads, 0)
        self.assertEqual(check_backend.bodies('/claim'), [])
        report = browser.restock.cycle(directory, merchant, backend, fixtures.DEVICE_ID)
        self.assertEqual(report['status'], 'RUNNING')
        self.assertEqual(report['uploaded'], 2)
        self.assertEqual(self.uploads, 1)
        repeat = browser.restock.cycle(directory, merchant, fixtures.Backend([fixtures.inventory(2)]), fixtures.DEVICE_ID)
        self.assertEqual(repeat['uploaded'], 0)
        self.assertEqual(self.uploads, 1)

    def test_closed_operator_tab_reopens_inside_the_existing_profile_owner(self):
        session = self.session()
        context = session.context
        session.page.close()
        session.pump()
        self.assertIs(session.context, context)
        session.open()
        self.assertIs(session.context, context)
        self.assertFalse(session.page.is_closed())
        self.assertEqual(session.page.locator('#captcha-element').count(), 1)

    def test_verification_response_after_upload_preserves_uncertain_batch(self):
        session = self.session()
        session.page.get_by_role('button', name='Complete synthetic verification').click()
        self.upload_block = True
        directory = self.state()
        backend = fixtures.Backend([fixtures.inventory()], fixtures.batch())
        result = browser.restock.cycle(directory, browser.BrowserMerchant(session), backend, fixtures.DEVICE_ID)
        self.assertEqual(result['status'], 'PAUSED')
        self.assertEqual(result['error'], 'verification_required')
        self.assertEqual(self.uploads, 1)
        state = browser.restock.read_state(directory)
        self.assertEqual(state['products'][str(fixtures.GOODS_ID)]['batch_id'], 'synthetic-batch')
        self.assertEqual(state['products'][str(fixtures.GOODS_ID)]['phase'], 'uncertain')
        self.assertNotIn('synthetic-token', json.dumps(state))


if __name__ == '__main__':
    unittest.main(verbosity=2)
