"""Synthetic browser isolation and loopback control boundaries; no browser or network."""
from email.message import Message
import io
import json
from pathlib import Path
import sys
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import Mock, call, patch
from urllib.parse import urlencode

sys.path.insert(0, str(Path(__file__).resolve().parents[2] / 'commerce'))
import ldxp_browser_session as browser


class FakePage:
    def __init__(self):
        self.url = browser.probe.ORIGIN + '/merchant/'
        self.closed = False
        self.is_closed = Mock(side_effect=lambda: self.closed)
        self.goto = Mock()
        self.wait_for_timeout = Mock()
        self.bring_to_front = Mock()
        self.evaluate = Mock(return_value={'status': 200, 'content_type': 'application/json',
                                          'text': '{"code":1,"data":{"ok":true}}'})


class FakeContext:
    def __init__(self):
        self.page = FakePage()
        self.new_page = Mock(return_value=self.page)
        self.on = Mock()
        self.close = Mock()
        self.add_cookies = Mock(side_effect=AssertionError('No HTTP cookie migration'))
        self.add_init_script = Mock(side_effect=AssertionError('No HTTP token migration'))
        self.storage_state = Mock(side_effect=AssertionError('No credential export'))
        self.cdp = Mock()
        self.cdp.send.side_effect = lambda method, *_args: {'windowId': 17} if method == 'Browser.getWindowForTarget' else {}
        self.new_cdp_session = Mock(return_value=self.cdp)


class BrowserSessionTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        activation = patch.object(browser.subprocess, 'run', return_value=SimpleNamespace(returncode=0))
        self.activation = activation.start()
        self.addCleanup(activation.stop)
        self.driver = Mock()
        self.driver.chromium.launch_persistent_context.side_effect = lambda *_args, **_kwargs: FakeContext()
        self.factory = Mock()
        self.factory.return_value.start.return_value = self.driver
        self.session = browser.BrowserSession(self.root / 'browser', playwright_factory=self.factory)
        self.addCleanup(self.session.close)
        network = patch('socket.socket.connect', side_effect=AssertionError('Synthetic tests cannot connect'))
        network.start()
        self.addCleanup(network.stop)

    def error_kind(self, expected, action):
        with self.assertRaises(browser.probe.ProbeError) as error:
            action()
        self.assertEqual(error.exception.kind, expected)
        return error.exception

    def test_repeat_open_keeps_profile_context_and_page_without_http_session_import(self):
        with patch.object(browser.probe, 'private_read', side_effect=AssertionError('No HTTP session read')):
            self.session.open()
            context, page = self.session.context, self.session.page
            self.session.open()
        self.assertIs(self.session.context, context)
        self.assertIs(self.session.page, page)
        page.goto.assert_called_once_with(browser.probe.ORIGIN + '/merchant/', wait_until='domcontentloaded', timeout=30000)
        self.assertEqual(page.bring_to_front.call_count, 2)
        self.factory.assert_called_once_with()
        self.driver.chromium.launch_persistent_context.assert_called_once_with(
            str(self.session.profile), channel='chrome', headless=False,
            viewport=None, accept_downloads=False)
        context.new_page.assert_called_once_with()
        context.add_cookies.assert_not_called()
        context.add_init_script.assert_not_called()
        context.storage_state.assert_not_called()
        self.assertEqual(self.session.profile.stat().st_mode & 0o777, 0o700)

    def test_headful_open_restores_minimized_window_before_bringing_page_forward(self):
        context = FakeContext()
        actions = Mock()
        actions.attach_mock(context.cdp.send, 'send')
        actions.attach_mock(context.cdp.detach, 'detach')
        actions.attach_mock(self.activation, 'activate')
        actions.attach_mock(context.page.bring_to_front, 'focus')
        self.driver.chromium.launch_persistent_context.side_effect = None
        self.driver.chromium.launch_persistent_context.return_value = context
        with patch.object(browser.sys, 'platform', 'darwin'):
            self.session.open()
        context.new_cdp_session.assert_called_once_with(context.page)
        self.assertEqual(actions.mock_calls, [
            call.send('Browser.getWindowForTarget'),
            call.send('Browser.setWindowBounds', {'windowId': 17, 'bounds': {'windowState': 'normal'}}),
            call.detach(),
            call.activate([
                '/usr/bin/open', '-W', '-n', '-a', '/Applications/Google Chrome.app',
                '--args', '--user-data-dir=' + str(self.session.profile),
            ], capture_output=True, timeout=10, check=False),
            call.focus(),
        ])

    def test_headless_open_does_not_require_native_window_control(self):
        self.session.headless = True
        with patch.object(browser.sys, 'platform', 'darwin'):
            self.session.open()
        self.session.context.new_cdp_session.assert_not_called()
        self.activation.assert_not_called()
        self.session.page.bring_to_front.assert_called_once_with()

    def test_native_activation_failure_and_timeout_are_typed_without_secret_output(self):
        failures = [
            SimpleNamespace(returncode=1, stdout=b'synthetic-private-sentinel', stderr=b'synthetic-private-sentinel'),
            browser.subprocess.TimeoutExpired('synthetic-open', 10, output=b'synthetic-private-sentinel'),
            OSError('synthetic-private-sentinel'),
        ]
        with patch.object(browser.sys, 'platform', 'darwin'):
            for failure in failures:
                with self.subTest(kind=type(failure).__name__):
                    self.activation.side_effect = failure if isinstance(failure, Exception) else None
                    self.activation.return_value = failure
                    error = self.error_kind('network_error', self.session.open)
                    self.assertNotIn('synthetic-private-sentinel', str(error) + json.dumps(error.diagnostics) + json.dumps(self.session.view))
                    self.session.page.bring_to_front.assert_not_called()
            self.activation.side_effect = None
            self.activation.return_value = SimpleNamespace(returncode=0)
            self.session.open()
        self.session.page.bring_to_front.assert_called_once_with()
        self.assertEqual(self.driver.chromium.launch_persistent_context.call_count, 1)

    def test_profile_lock_excludes_second_session_and_releases_on_close(self):
        self.session.open()
        second = browser.BrowserSession(self.session.directory, playwright_factory=self.factory)
        self.addCleanup(second.close)
        self.error_kind('unsafe_session', second.open)
        self.session.close()
        second.open()
        self.assertIsNotNone(second.context)
        self.assertIsNotNone(second.lock_fd)

    def test_closed_page_pump_preserves_live_context_and_reopens_without_second_launch(self):
        target_closed = type('TargetClosedError', (Exception,), {})
        for failure in (target_closed('synthetic closed page'), RuntimeError('synthetic page event failure')):
            with self.subTest(failure=type(failure).__name__):
                self.session.open()
                context, page = self.session.context, self.session.page
                replacement = FakePage()
                context.new_page.return_value = replacement
                page.closed = True
                page.wait_for_timeout = Mock(side_effect=failure)
                self.session.pump()
                self.assertIs(self.session.context, context, 'A page error is not a context-close event')
                context.close.assert_not_called()
                self.driver.stop.assert_not_called()
                self.assertFalse(self.session.view['window_open'])
                self.session.open()
                self.assertIs(self.session.context, context)
                self.assertIs(self.session.page, replacement)
                self.assertEqual(self.driver.chromium.launch_persistent_context.call_count, 1)
                replacement.goto.assert_called_once_with(browser.probe.ORIGIN + '/merchant/',
                                                        wait_until='domcontentloaded', timeout=30000)
                replacement.bring_to_front.assert_called_once_with()

    def test_context_close_event_still_allows_a_new_persistent_context(self):
        self.session.open()
        old = self.session.context
        callbacks = [call.args[1] for call in old.on.call_args_list if call.args[0] == 'close']
        self.assertEqual(len(callbacks), 1)
        callbacks[0]()
        self.assertIsNone(self.session.context)
        self.session.open()
        self.assertIsNot(self.session.context, old)
        self.assertEqual(self.driver.chromium.launch_persistent_context.call_count, 2)

    def test_profile_permission_and_symlink_boundaries_are_typed(self):
        for kind in ('public_home', 'public_profile', 'linked_home', 'linked_profile', 'profile_file', 'linked_lock'):
            with self.subTest(kind=kind):
                directory = self.root / kind
                if kind == 'linked_home':
                    target = self.root / 'actual-home'
                    target.mkdir(mode=0o700)
                    directory.symlink_to(target, target_is_directory=True)
                else:
                    directory.mkdir(mode=0o755 if kind == 'public_home' else 0o700)
                    directory.chmod(0o755 if kind == 'public_home' else 0o700)
                if kind == 'public_profile':
                    (directory / 'profile').mkdir(mode=0o755)
                    (directory / 'profile').chmod(0o755)
                if kind == 'linked_profile':
                    target = self.root / 'actual-profile'
                    target.mkdir(mode=0o700)
                    (directory / 'profile').symlink_to(target, target_is_directory=True)
                if kind == 'profile_file':
                    (directory / 'profile').write_text('not a profile directory')
                if kind == 'linked_lock':
                    target = self.root / 'lock-target'
                    target.write_text('untouched')
                    (directory / 'browser.lock').symlink_to(target)
                session = browser.BrowserSession(directory, playwright_factory=self.factory)
                self.addCleanup(session.close)
                self.error_kind('unsafe_session', session.open)
        self.factory.assert_not_called()

    def test_only_fixed_origin_and_allowlisted_route_can_reach_page(self):
        self.session.open()
        page = self.session.page
        for url in ('http://www.ldxp.cn/merchant/', 'https://ldxp.cn/merchant/',
                    'https://www.ldxp.cn.evil.invalid/', 'https://www.ldxp.cn@evil.invalid/',
                    'https://evil.invalid/', 'https://www.ldxp.cn:444/merchant/'):
            with self.subTest(url=url):
                page.url = url
                self.error_kind('upstream_rejected', lambda: self.session.request('/merchantApi/user/userinfo', {}))
        page.url = browser.probe.ORIGIN + '/merchant/'
        for route in ('https://evil.invalid/', '//evil.invalid/', '/merchantApi/user/login',
                      '/merchantApi/goodsCardStorage/list?secret=x', '/merchantApi/GoodsCardStorage/delete'):
            with self.subTest(route=route):
                self.error_kind('invalid_input', lambda: self.session.request(route, {}))
        self.error_kind('invalid_input', lambda: self.session.request('/merchantApi/user/userinfo', []))
        page.evaluate.assert_not_called()
        self.assertEqual(self.session.request('/merchantApi/user/userinfo', {}), {'ok': True})
        args = page.evaluate.call_args.args[1]
        self.assertEqual(args, {'origin': browser.probe.ORIGIN, 'route': '/merchantApi/user/userinfo', 'payload': {}})

    def test_missing_or_closed_page_is_login_required_without_evaluation(self):
        self.error_kind('login_required', lambda: self.session.request('/merchantApi/user/userinfo', {}))
        self.session.open()
        self.session.page.closed = True
        self.error_kind('login_required', lambda: self.session.request('/merchantApi/user/userinfo', {}))
        self.session.page.evaluate.assert_not_called()

    def test_api_challenge_waits_for_operator_open_before_navigating_once(self):
        self.session.open()
        page = self.session.page
        for response, reason in (
            ({'status': 200, 'content_type': 'text/html', 'text': '<script>CF_APP_WAF captcha</script>'}, 'verification_required'),
            ({'status': 401, 'content_type': 'application/json', 'text': '{"code":401}'}, 'login_required'),
        ):
            with self.subTest(reason=reason):
                before = page.goto.call_count
                page.evaluate.return_value = response
                self.error_kind(reason, lambda: self.session.request('/merchantApi/user/userinfo', {}))
                self.assertEqual(page.goto.call_count, before, 'API failure must not navigate automatically')
                self.session.open()
                self.assertEqual(page.goto.call_count, before + 1)
                self.session.open()
                self.assertEqual(page.goto.call_count, before + 1)
        page.evaluate.return_value = {'error': 'verification_required'}
        before = page.goto.call_count
        self.error_kind('verification_required', lambda: self.session.request('/merchantApi/user/userinfo', {}))
        self.session.open()
        self.assertEqual(page.goto.call_count, before, 'A visible challenge must not be reset by another Open click')

    def test_typed_login_and_captcha_errors_never_expose_page_secrets(self):
        self.session.open()
        cases = [
            ({'error': 'verification_required', 'detail': 'synthetic-private-sentinel'}, 'verification_required'),
            ({'error': 'login_required', 'detail': 'synthetic-private-sentinel'}, 'login_required'),
            ({'error': 'unexpected-private-kind'}, 'invalid_response'),
            ({'status': 200, 'content_type': 'text/html', 'text': '<script>CF_APP_WAF captcha synthetic-private-sentinel</script>'}, 'verification_required'),
            ({'status': 401, 'content_type': 'text/plain', 'text': 'synthetic-private-sentinel'}, 'login_required'),
            ({'status': 503, 'content_type': 'text/plain', 'text': 'synthetic-private-sentinel'}, 'upstream_unavailable'),
            ({'status': 200, 'content_type': 'application/json', 'text': '{"code":403,"message":"synthetic-private-sentinel"}'}, 'login_required'),
        ]
        for value, kind in cases:
            with self.subTest(kind=kind, shape=list(value)):
                self.session.page.evaluate.return_value = value
                error = self.error_kind(kind, lambda: self.session.request('/merchantApi/user/userinfo', {}))
                public = str(error) + json.dumps(error.diagnostics) + json.dumps(self.session.view)
                self.assertNotIn('synthetic-private-sentinel', public)
                self.assertNotIn('unexpected-private-kind', public)
        self.session.page.evaluate.side_effect = RuntimeError('synthetic-private-sentinel')
        error = self.error_kind('network_error', lambda: self.session.request('/merchantApi/user/userinfo', {}))
        self.assertNotIn('synthetic-private-sentinel', str(error))

    def test_navigation_failure_is_typed_and_close_releases_owned_profile(self):
        context = FakeContext()
        context.page.goto.side_effect = RuntimeError('synthetic-private-sentinel')
        self.driver.chromium.launch_persistent_context.side_effect = None
        self.driver.chromium.launch_persistent_context.return_value = context
        error = self.error_kind('network_error', self.session.open)
        self.assertNotIn('synthetic-private-sentinel', str(error))
        self.session.close()
        self.assertIsNone(self.session.lock_fd)
        context.close.assert_called_once()
        self.driver.stop.assert_called_once()

    def test_close_failure_still_stops_driver_and_releases_profile_lock(self):
        self.session.open()
        context = self.session.context
        context.close.side_effect = RuntimeError('synthetic context close failure')
        with self.assertRaises(RuntimeError):
            self.session.close()
        self.driver.stop.assert_called_once()
        self.assertIsNone(self.session.lock_fd)
        second = browser.BrowserSession(self.session.directory, playwright_factory=self.factory)
        self.addCleanup(second.close)
        second.open()
        self.assertIsNotNone(second.context)


class BrowserControlTests(unittest.TestCase):
    def setUp(self):
        activation = patch.object(browser.subprocess, 'run', return_value=SimpleNamespace(returncode=0))
        self.activation = activation.start()
        self.addCleanup(activation.stop)
        self.session = SimpleNamespace(view={'window_open': False, 'reason': 'synthetic-private-sentinel'},
                                       open=Mock(), request=Mock(), pump=Mock())
        server = Mock(server_port=52499)
        with patch.object(browser, 'ThreadingHTTPServer', return_value=server) as constructor:
            self.control = browser.BrowserControl(self.session, port=52499)
        self.addCleanup(self.control.close)
        self.assertEqual(constructor.call_args.args[0], ('127.0.0.1', 52499))

    def request(self, method, path, *, host=None, origin=None, data='', media='application/x-www-form-urlencoded', size=None, accept=None):
        handler = object.__new__(self.control.handler())
        handler.command, handler.path = method, path
        handler.request_version = 'HTTP/1.1'
        handler.requestline = method + ' ' + path + ' HTTP/1.1'
        handler.headers = Message()
        handler.headers['Host'] = host if host is not None else '127.0.0.1:52499'
        if origin is not None:
            handler.headers['Origin'] = origin
        if accept is not None:
            handler.headers['Accept'] = accept
        handler.headers['Content-Type'] = media
        raw = data.encode()
        handler.headers['Content-Length'] = str(len(raw) if size is None else size)
        handler.rfile, handler.wfile = io.BytesIO(raw), io.BytesIO()
        getattr(handler, 'do_' + method)()
        status, body = handler.wfile.getvalue().split(b'\r\n', 1)[0], handler.wfile.getvalue()
        return int(status.split()[1]), body.decode()

    def test_helper_enforces_host_origin_nonce_media_and_action_allowlist(self):
        valid = urlencode({'nonce': self.control.nonce})
        invalid = [
            {'host': 'evil.invalid'}, {'host': 'localhost:52499'}, {'origin': 'https://evil.invalid'},
            {'origin': None}, {'origin': 'null'}, {'data': 'nonce=wrong'},
            {'data': valid + '&nonce=other'}, {'data': valid + '&action=upload'},
            {'media': 'application/json'}, {'size': 0}, {'size': 4097}, {'size': 'invalid'},
            {'path': '/upload'}, {'path': '/resume'}, {'path': '/open?operation=upload'},
        ]
        for accept in (None, 'application/json'):
            for changed in invalid:
                with self.subTest(changed=changed, accept=accept):
                    kwargs = {'path': '/open', 'origin': self.control.origin, 'data': valid, 'accept': accept} | changed
                    status, body = self.request('POST', **kwargs)
                    self.assertEqual(status, 403)
                    self.assertNotIn('synthetic-private-sentinel', body)
                    self.assertFalse(self.control.open_requested.is_set())
        self.session.open.assert_not_called()
        self.session.request.assert_not_called()

    def test_helper_only_queues_open_event_until_worker_tick(self):
        status, body = self.request('POST', '/open', origin=self.control.origin,
                                    data=urlencode({'nonce': self.control.nonce}), accept='application/json')
        self.assertEqual(status, 202)
        self.assertEqual(json.loads(body.split('\r\n\r\n', 1)[1]), {'requested': True})
        self.assertTrue(self.control.open_requested.is_set())
        self.session.open.assert_not_called()
        self.session.request.assert_not_called()
        status, pending = self.request('GET', '/status')
        self.assertEqual(status, 200)
        pending = json.loads(pending.split('\r\n\r\n', 1)[1])
        self.assertEqual(pending['open_state'], 'pending')
        self.assertTrue(pending['open_pending'])
        self.control.tick()
        self.control.tick()
        self.session.open.assert_called_once_with()
        self.assertEqual(self.session.pump.call_count, 2)
        self.session.request.assert_not_called()
        self.assertFalse(self.control.open_requested.is_set())
        self.assertNotIn('synthetic-private-sentinel', body)

    def test_form_open_redirects_to_live_result_page_without_claiming_success(self):
        status, body = self.request('POST', '/open', origin=self.control.origin,
                                    data=urlencode({'nonce': self.control.nonce}))
        self.assertEqual(status, 303)
        self.assertIn('Location: /\r\n', body)
        self.session.open.assert_not_called()
        self.assertTrue(self.control.open_requested.is_set())
        self.assertIn('已请求打开窗口，请稍候', self.request('GET', '/')[1])

    def test_helper_page_has_no_merchant_secrets_and_rejects_foreign_host(self):
        status, body = self.request('GET', '/')
        self.assertEqual(status, 200)
        self.assertIn('Cache-Control: no-store', body)
        self.assertIn('X-Frame-Options: DENY', body)
        self.assertNotIn('synthetic-private-sentinel', body)
        for path, host in (('/unknown', '127.0.0.1:52499'), ('/', 'evil.invalid')):
            self.assertEqual(self.request('GET', path, host=host)[0], 404)
        self.session.request.assert_not_called()

    def test_open_failure_is_observable_after_queued_request_without_secret_details(self):
        def failed_open():
            self.session.view = {'window_open': False, 'reason': 'network_error'}
            raise browser.probe.ProbeError('network_error', {'detail': 'synthetic-private-sentinel'})

        self.session.open.side_effect = failed_open
        self.assertEqual(self.request('POST', '/open', origin=self.control.origin,
                                      data=urlencode({'nonce': self.control.nonce}), accept='application/json')[0], 202)
        self.control.tick()
        status, response = self.request('GET', '/status')
        self.assertEqual(status, 200, 'Queued acknowledgement must be followed by a readable execution result')
        result = json.loads(response.split('\r\n\r\n', 1)[1])
        self.assertFalse(result['window_open'])
        self.assertEqual(result['reason'], 'network_error')
        self.assertEqual(result['open_state'], 'failed')
        self.assertFalse(result['open_pending'])
        self.assertNotIn('synthetic-private-sentinel', response)
        status, page = self.request('GET', '/')
        self.assertEqual(status, 200)
        self.assertIn('打开失败', page)
        self.assertNotIn('synthetic-private-sentinel', page)
        self.assertEqual(self.request('GET', '/status', host='evil.invalid')[0], 404)
        self.session.request.assert_not_called()

    def test_native_activation_failures_are_visible_through_helper_status(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        factory = Mock()
        factory.return_value.start.return_value.chromium.launch_persistent_context.return_value = FakeContext()
        actual_session = browser.BrowserSession(Path(temporary.name) / 'dedicated-profile', playwright_factory=factory)
        self.addCleanup(actual_session.close)
        self.control.session = actual_session
        failures = [SimpleNamespace(returncode=1, stderr=b'synthetic-private-sentinel'),
                    browser.subprocess.TimeoutExpired('synthetic-open', 10, output=b'synthetic-private-sentinel')]
        with patch.object(browser.sys, 'platform', 'darwin'):
            for failure in failures:
                with self.subTest(kind=type(failure).__name__):
                    self.activation.side_effect = failure if isinstance(failure, Exception) else None
                    self.activation.return_value = failure
                    self.assertEqual(self.request('POST', '/open', origin=self.control.origin,
                                                 data=urlencode({'nonce': self.control.nonce}), accept='application/json')[0], 202)
                    self.control.tick()
                    status, response = self.request('GET', '/status')
                    self.assertEqual(status, 200)
                    result = json.loads(response.split('\r\n\r\n', 1)[1])
                    self.assertEqual(result['open_state'], 'failed')
                    self.assertEqual(result['reason'], 'network_error')
                    self.assertFalse(result['window_open'])
                    self.assertFalse(result['open_pending'])
                    self.assertNotIn('synthetic-private-sentinel', response)
                    self.assertIn('打开失败', self.request('GET', '/')[1])


if __name__ == '__main__':
    unittest.main()
