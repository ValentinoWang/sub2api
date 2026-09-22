"""A dedicated merchant browser shared by the operator and the restock worker."""
import fcntl
import hmac
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
from pathlib import Path
import secrets
import subprocess
import sys
import threading
import time
from types import SimpleNamespace
from urllib.parse import parse_qs, urlsplit

import ldxp_http_probe as probe

CONTROL_PORT = 52401
CONTROL_ORIGIN = f'http://127.0.0.1:{CONTROL_PORT}'
BROWSER_HOME = Path.home() / '.local/share/sub2api/ldxp-browser'
REQUEST_ROUTES = probe.READ_ROUTES | {'/merchantApi/GoodsCardStorage/add'}

# Credentials stay in the merchant page. No cookies or tokens are exported to Python.
PAGE_REQUEST = r'''async ({origin, route, payload}) => {
  if (location.origin !== origin) return {error: 'upstream_rejected'};
  const visible = selector => Array.from(document.querySelectorAll(selector)).some(e =>
    e.getClientRects().length && getComputedStyle(e).visibility !== 'hidden');
  if (visible('#aliyunCaptcha-sliding-slider, #captcha-element'))
    return {error: 'verification_required'};
  let auth;
  try { auth = JSON.parse(localStorage.getItem('auth-token')); } catch {}
  if (!auth || typeof auth.value !== 'string' || !auth.value ||
      typeof auth.expiry !== 'number' || auth.expiry <= Date.now())
    return {error: 'login_required'};
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), 25000);
  try {
    const response = await fetch(origin + route, {
      method: 'POST', credentials: 'include', redirect: 'error', cache: 'no-store',
      headers: {'Content-Type': 'application/json', 'Accept': 'application/json', 'Merchant-Token': auth.value},
      body: JSON.stringify(payload), signal: controller.signal,
    });
    const reader = response.body.getReader();
    const chunks = []; let size = 0;
    while (true) {
      const {done, value} = await reader.read();
      if (done) break;
      size += value.byteLength;
      if (size > 2000000) { await reader.cancel(); return {error: 'invalid_response'}; }
      chunks.push(value);
    }
    const bytes = new Uint8Array(size); let offset = 0;
    for (const chunk of chunks) { bytes.set(chunk, offset); offset += chunk.byteLength; }
    return {status: response.status, content_type: response.headers.get('Content-Type') || '',
            text: new TextDecoder().decode(bytes)};
  } catch { return {error: 'network_error'}; }
  finally { clearTimeout(timer); }
}'''


class BrowserSession:
    def __init__(self, directory=BROWSER_HOME, *, playwright_factory=None, headless=False):
        self.directory = Path(directory)
        self.profile = self.directory / 'profile'
        self.playwright_factory = playwright_factory
        self.headless = headless
        self.driver = None
        self.context = None
        self.page = None
        self.needs_navigation = True
        self.lock_fd = None
        self.view = {'window_open': False, 'reason': 'login_required'}

    def _lock(self):
        if self.lock_fd is not None:
            return
        fd = None
        try:
            self.directory.mkdir(parents=True, exist_ok=True, mode=0o700)
            info = self.directory.lstat()
            if self.directory.is_symlink() or info.st_uid != os.getuid() or info.st_mode & 0o077:
                raise probe.ProbeError('unsafe_session')
            if self.profile.is_symlink():
                raise probe.ProbeError('unsafe_session')
            self.profile.mkdir(exist_ok=True, mode=0o700)
            info = self.profile.stat()
            if info.st_uid != os.getuid() or info.st_mode & 0o077:
                raise probe.ProbeError('unsafe_session')
            fd = os.open(self.directory / 'browser.lock', os.O_CREAT | os.O_RDWR | os.O_NOFOLLOW, 0o600)
            fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except (OSError, probe.ProbeError):
            if fd is not None:
                os.close(fd)
            raise probe.ProbeError('unsafe_session') from None
        self.lock_fd = fd

    def open(self):
        self._lock()
        try:
            if self.driver is None:
                if self.playwright_factory is None:
                    from playwright.sync_api import sync_playwright
                    self.playwright_factory = sync_playwright
                self.driver = self.playwright_factory().start()
            if self.context is None:
                self.context = self.driver.chromium.launch_persistent_context(
                    str(self.profile), channel='chrome', headless=self.headless,
                    viewport=None, accept_downloads=False,
                )
                self.context.on('close', self._closed)
            if self.page is None or self.page.is_closed():
                self.page = self.context.new_page()
                self.needs_navigation = True
            if urlsplit(self.page.url).netloc != 'www.ldxp.cn' or urlsplit(self.page.url).scheme != 'https':
                self.needs_navigation = True
            if self.needs_navigation:
                self.page.goto(probe.ORIGIN + '/merchant/', wait_until='domcontentloaded', timeout=30000)
                self.needs_navigation = False
            if not self.headless:
                window = self.context.new_cdp_session(self.page)
                try:
                    identity = window.send('Browser.getWindowForTarget')
                    window.send('Browser.setWindowBounds', {'windowId': identity['windowId'], 'bounds': {'windowState': 'normal'}})
                finally:
                    window.detach()
                if sys.platform == 'darwin':
                    # LaunchServices activates the dedicated profile's existing Chrome
                    # process; focusing a CDP tab alone can leave the macOS app hidden.
                    activated = subprocess.run([
                        '/usr/bin/open', '-W', '-n', '-a', '/Applications/Google Chrome.app',
                        '--args', '--user-data-dir=' + str(self.profile),
                    ], capture_output=True, timeout=10, check=False)
                    if activated.returncode:
                        raise probe.ProbeError('network_error')
            self.page.bring_to_front()
            self.view = {'window_open': True, 'reason': ''}
        except Exception:
            self.view = {'window_open': self.page is not None, 'reason': 'network_error'}
            raise probe.ProbeError('network_error') from None

    def _closed(self, *_):
        self.context = None
        self.page = None
        self.view = {'window_open': False, 'reason': 'login_required'}

    def pump(self):
        if self.page is not None:
            try:
                # Service browser-close events even while the restock scheduler is paused.
                self.page.wait_for_timeout(1)
            except Exception:
                # A closed tab does not mean its persistent browser has exited.
                # Retaining the context avoids launching a second owner of the profile.
                self.page = None
                self.view = {'window_open': False, 'reason': 'login_required'}

    def request(self, route, payload):
        if route not in REQUEST_ROUTES or not isinstance(payload, dict):
            raise probe.ProbeError('invalid_input')
        if self.page is None or self.page.is_closed():
            raise probe.ProbeError('login_required')
        if urlsplit(self.page.url).netloc != 'www.ldxp.cn' or urlsplit(self.page.url).scheme != 'https':
            raise probe.ProbeError('upstream_rejected')
        try:
            result = self.page.evaluate(PAGE_REQUEST, {'origin': probe.ORIGIN, 'route': route, 'payload': payload})
        except Exception:
            raise probe.ProbeError('network_error') from None
        diagnostic = {'route': route, 'checked_at': time.time()}
        if not isinstance(result, dict):
            raise probe.ProbeError('invalid_response', diagnostic)
        error = result.get('error')
        if error:
            if error not in ('login_required', 'verification_required', 'upstream_rejected', 'network_error', 'invalid_response'):
                error = 'invalid_response'
            self.view = {'window_open': True, 'reason': error}
            if error == 'login_required':
                self.needs_navigation = True
            raise probe.ProbeError(error, diagnostic)
        if type(result.get('status')) is not int or not isinstance(result.get('text'), str):
            raise probe.ProbeError('invalid_response', diagnostic)
        raw = result['text'].encode('utf-8')
        response = SimpleNamespace(status=result['status'], headers={'Content-Type': result.get('content_type', '')})
        diagnostic = probe.response_diagnostics(route, response, raw)
        if diagnostic.get('response_kind') == 'browser_verification':
            self.view = {'window_open': True, 'reason': 'verification_required'}
            self.needs_navigation = True
            raise probe.ProbeError('verification_required', diagnostic)
        if result['status'] in (401, 403):
            self.needs_navigation = True
            raise probe.ProbeError('login_required', diagnostic)
        if result['status'] == 429 or result['status'] >= 500:
            raise probe.ProbeError('upstream_unavailable', diagnostic)
        if not 200 <= result['status'] < 300:
            raise probe.ProbeError('upstream_rejected', diagnostic)
        if len(raw) > 2_000_000:
            raise probe.ProbeError('invalid_response', diagnostic)
        try:
            value = json.loads(raw)
        except ValueError:
            raise probe.ProbeError('non_json', diagnostic) from None
        if not isinstance(value, dict):
            raise probe.ProbeError('invalid_response', diagnostic)
        if value.get('code') != 1:
            if value.get('code') in (401, 403):
                self.needs_navigation = True
            raise probe.ProbeError('login_required' if value.get('code') in (401, 403) else 'upstream_rejected', diagnostic)
        self.view = {'window_open': True, 'reason': ''}
        return value.get('data')

    def close(self):
        try:
            if self.context:
                self.context.close()
        finally:
            try:
                if self.driver:
                    self.driver.stop()
            finally:
                self.driver = None
                self.context = None
                self.page = None
                if self.lock_fd is not None:
                    os.close(self.lock_fd)
                    self.lock_fd = None


class BrowserControl:
    def __init__(self, session, *, port=CONTROL_PORT):
        self.session = session
        self.nonce = secrets.token_urlsafe(32)
        self.open_requested = threading.Event()
        self.open_state = 'idle'
        self.server = ThreadingHTTPServer(('127.0.0.1', port), self.handler())
        self.origin = f'http://127.0.0.1:{self.server.server_port}'
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)

    def handler(self):
        control = self

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *_):
                pass

            def reply(self, status, body, media='text/html; charset=utf-8'):
                raw = body.encode('utf-8')
                self.send_response(status)
                self.send_header('Content-Type', media)
                self.send_header('Content-Length', str(len(raw)))
                self.send_header('Cache-Control', 'no-store')
                self.send_header('X-Frame-Options', 'DENY')
                self.send_header('Content-Security-Policy', "default-src 'none'; connect-src 'self'; style-src 'unsafe-inline'; script-src 'nonce-" + control.nonce + "'; form-action 'self'; frame-ancestors 'none'")
                self.end_headers()
                self.wfile.write(raw)

            def valid_host(self):
                return self.headers.get('Host') == urlsplit(control.origin).netloc

            def do_GET(self):
                if not self.valid_host() or self.path not in ('/', '/status'):
                    return self.reply(404, 'Not found')
                if self.path == '/status':
                    return self.reply(200, json.dumps(control.session.view | {'open_state': control.open_state,
                        'open_pending': control.open_requested.is_set()}), 'application/json')
                html = Path(__file__).with_suffix('.html').read_text()
                state = '专用窗口已打开' if control.session.view['window_open'] else '专用窗口尚未打开'
                if control.open_requested.is_set():
                    state = '已请求打开窗口，请稍候'
                elif control.open_state == 'failed':
                    state = '打开失败，请点击重试。补货仍保持暂停。'
                self.reply(200, html.replace('{{NONCE}}', control.nonce).replace('{{STATE}}', state))

            def do_POST(self):
                if not self.valid_host() or self.path != '/open' or self.headers.get('Origin') != control.origin:
                    return self.reply(403, 'Forbidden')
                try:
                    size = int(self.headers.get('Content-Length', '0'))
                    if not 0 < size <= 4096 or self.headers.get_content_type() != 'application/x-www-form-urlencoded':
                        raise ValueError()
                    fields = parse_qs(self.rfile.read(size).decode(), max_num_fields=2)
                    if set(fields) != {'nonce'} or len(fields['nonce']) != 1 or not hmac.compare_digest(fields['nonce'][0], control.nonce):
                        raise ValueError()
                except (ValueError, UnicodeError):
                    return self.reply(403, 'Forbidden')
                control.open_requested.set()
                control.open_state = 'pending'
                if 'application/json' in self.headers.get('Accept', ''):
                    return self.reply(202, '{"requested":true}', 'application/json')
                self.send_response(303)
                self.send_header('Location', '/')
                self.send_header('Content-Length', '0')
                self.end_headers()

        return Handler

    def start(self):
        self.thread.start()

    def tick(self):
        self.session.pump()
        if self.open_requested.is_set():
            self.open_requested.clear()
            try:
                self.session.open()
                self.open_state = 'opened'
            except probe.ProbeError as exc:
                self.open_state = 'failed'
                self.session.view = {'window_open': False, 'reason': exc.kind}
                # The status remains visible; the next check reports the typed failure.
                return

    def close(self):
        if self.thread.is_alive():
            self.server.shutdown()
        self.server.server_close()
        if self.thread.is_alive():
            self.thread.join(timeout=5)
