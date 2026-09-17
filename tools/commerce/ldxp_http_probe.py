#!/usr/bin/env python3
"""Local, read-only LDXP HTTP verification. No card creation or upload operations."""
import argparse
import hashlib
import hmac
from http.cookiejar import Cookie, CookieJar
from http.server import BaseHTTPRequestHandler, HTTPServer
import json
import os
from pathlib import Path
import secrets
import stat
import sys
import tempfile
import time
from urllib.error import HTTPError, URLError
from urllib.parse import parse_qs
from urllib.request import HTTPCookieProcessor, HTTPRedirectHandler, ProxyHandler, Request, build_opener

ORIGIN = "https://www.ldxp.cn"
LOCAL_SITE = "http://127.0.0.1:8080"
DEFAULT_PRIVATE = Path.home() / ".local/share/sub2api/ldxp-http-probe"
DEFAULT_ADMIN = Path.home() / ".local/share/sub2api/commerce-release/dev-admin.key"
READ_ROUTES = frozenset({"/merchantApi/system/config", "/merchantApi/user/userinfo",
                        "/merchantApi/Goods/list", "/merchantApi/goodsCardStorage/list"})
LOGIN_ROUTES = frozenset({"/merchantApi/user/checkSafeMode", "/merchantApi/user/login"})
ERRORS = {
    "login_required": "请先登录小铺，或重新登录更新已失效的授权。",
    "login_rejected": "小铺拒绝登录，请检查账号密码或账号状态。",
    "verification_required": "该账号需要网页安全验证，请先在小铺完成验证。",
    "upstream_rejected": "小铺接口拒绝了请求，验证已停止。",
    "upstream_unavailable": "小铺暂时不可用，已暂停并等待重新核对。",
    "network_error": "无法连接小铺，验证已停止。",
    "non_json": "小铺返回了网页或非 JSON 内容，验证已停止。",
    "invalid_response": "接口结构或库存发生变化，本次核对不完整。",
    "unsafe_session": "登录态文件权限或结构不符合要求。",
    "local_unavailable": "无法读取本地 Sub2API 的测试商品配置。",
    "invalid_input": "请求无效。",
}


class ProbeError(Exception):
    def __init__(self, kind, diagnostics=None):
        self.kind = kind
        self.diagnostics = diagnostics or {}
        super().__init__(ERRORS[kind])


def response_diagnostics(route, response=None, raw=None):
    # Only fixed routes and transport facts are retained; never headers, bodies or URLs with queries.
    known = READ_ROUTES | LOGIN_ROUTES | {'/merchantApi/GoodsCardStorage/add'}
    result = {'route': route if route in known else 'other', 'checked_at': time.time()}
    if response is not None:
        status = getattr(response, 'status', getattr(response, 'code', None))
        if type(status) is int:
            result['http_status'] = status
        headers = getattr(response, 'headers', None)
        media = headers.get('Content-Type', '').split(';', 1)[0].strip().lower() if headers else ''
        result['content_type'] = media if media in ('application/json', 'text/html', 'text/plain', 'application/problem+json') else 'other'
    if raw is not None:
        result.update(body_bytes=len(raw), body_sha256=hashlib.sha256(raw).hexdigest())
        # This explicit WAF marker was observed on the merchant's ESA verification page.
        # It is only an operator notification signal, never proof of authorization.
        if result.get('content_type') == 'text/html' and b'CF_APP_WAF' in raw and b'captcha' in raw.lower():
            result['response_kind'] = 'browser_verification'
    return result


class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        raise ProbeError("upstream_rejected")


def opener(jar=None):
    handlers = [ProxyHandler({}), NoRedirect()]
    if jar is not None:
        handlers.append(HTTPCookieProcessor(jar))
    return build_opener(*handlers)


def private_read(path):
    try:
        with os.fdopen(os.open(path, os.O_RDONLY | os.O_NOFOLLOW), "rb") as stream:
            info = os.fstat(stream.fileno())
            if not stat.S_ISREG(info.st_mode) or info.st_uid != os.getuid() or info.st_mode & 0o077:
                raise ProbeError("unsafe_session")
            data = stream.read(1_000_001)
            if len(data) > 1_000_000:
                raise ProbeError("unsafe_session")
            return data
    except OSError as exc:
        raise ProbeError("unsafe_session") from exc


def write_private(path, value):
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    directory = path.parent.lstat()
    if not stat.S_ISDIR(directory.st_mode) or directory.st_uid != os.getuid() or directory.st_mode & 0o077:
        raise ProbeError("unsafe_session")
    fd, temporary = tempfile.mkstemp(dir=path.parent, prefix=".saving-")
    try:
        with os.fdopen(fd, "w") as stream:
            json.dump(value, stream)
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(temporary, path)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


class Merchant:
    def __init__(self, session_path):
        self.session_path = Path(session_path)
        self.jar = CookieJar()
        self.token = ""
        self.http = opener(self.jar)

    def load(self):
        if not self.session_path.exists():
            raise ProbeError("login_required")
        try:
            data = json.loads(private_read(self.session_path))
            if data["schema"] != 1 or data["origin"] != ORIGIN or not isinstance(data["token"], str) or not data["token"]:
                raise ValueError()
            self.token = data["token"]
            for row in data["cookies"]:
                if row["domain"].lstrip(".") not in ("ldxp.cn", "www.ldxp.cn"):
                    raise ValueError()
                self.jar.set_cookie(Cookie(**row))
        except (ValueError, TypeError, KeyError) as exc:
            raise ProbeError("unsafe_session") from exc
        return self

    def save(self):
        cookies = []
        for cookie in self.jar:
            row = vars(cookie).copy()
            row["rest"] = row.pop("_rest")
            cookies.append(row)
        write_private(self.session_path, {"schema": 1, "origin": ORIGIN, "token": self.token,
                                        "cookies": cookies, "saved_at": time.time()})

    def post(self, route, payload, *, login=False):
        if route not in READ_ROUTES and not (login and route in LOGIN_ROUTES):
            raise ProbeError("invalid_input")
        return self._request(route, payload)

    def _request(self, route, payload):
        headers = {"Content-Type": "application/json", "Accept": "application/json",
                   "Origin": ORIGIN, "Referer": ORIGIN + "/merchant/login",
                   "User-Agent": "Mozilla/5.0"}
        if self.token:
            headers["Merchant-Token"] = self.token
        request = Request(ORIGIN + route, data=json.dumps(payload).encode(), headers=headers, method="POST")
        diagnostic = response_diagnostics(route)
        try:
            with self.http.open(request, timeout=25) as response:
                raw = response.read(2_000_001)
                diagnostic = response_diagnostics(route, response, raw)
        except HTTPError as exc:
            kind = ('login_required' if exc.code in (401, 403) else
                    'upstream_unavailable' if exc.code == 429 or 500 <= exc.code <= 599 else 'upstream_rejected')
            raise ProbeError(kind, response_diagnostics(route, exc)) from exc
        except (URLError, TimeoutError, OSError) as exc:
            failure = getattr(exc, 'reason', exc)
            diagnostic['transport_error'] = 'timeout' if isinstance(failure, TimeoutError) else 'connection_error'
            raise ProbeError("network_error", diagnostic) from exc
        if len(raw) > 2_000_000:
            raise ProbeError("invalid_response", diagnostic)
        try:
            result = json.loads(raw)
        except ValueError as exc:
            raise ProbeError("non_json", diagnostic) from exc
        if not isinstance(result, dict):
            raise ProbeError("invalid_response")
        if result.get("code") != 1:
            kind = "login_required" if result.get("code") in (401, 403) else "upstream_rejected"
            raise ProbeError("login_rejected" if route in LOGIN_ROUTES else kind)
        return result.get("data")

    def login(self, username, password):
        if not username or not password or len(username) > 256 or len(password) > 1024:
            raise ProbeError("invalid_input")
        self.post("/merchantApi/system/config", {})
        credentials = {"username": username, "password": password}
        safe = self.post("/merchantApi/user/checkSafeMode", credentials, login=True)
        if not isinstance(safe, dict) or safe.get("safe_mode") not in (0, "0", False):
            raise ProbeError("verification_required")
        result = self.post("/merchantApi/user/login", credentials, login=True)
        if not isinstance(result, dict) or not isinstance(result.get("merchant_token"), str) or not result["merchant_token"]:
            raise ProbeError("invalid_response")
        self.token = result["merchant_token"]
        self.post("/merchantApi/user/userinfo", {})
        self.save()

    def rows(self, route, filters, *, page_size=100):
        if type(page_size) is not int or not 1 <= page_size <= 1000:
            raise ProbeError("invalid_input")
        rows, seen, expected = [], set(), None
        for page in range(1, 101):
            result = self.post(route, dict(filters, current=page, pageSize=page_size))
            if not isinstance(result, dict) or type(result.get("total")) is not int or not 0 <= result["total"] <= 10000 or not isinstance(result.get("list"), list):
                raise ProbeError("invalid_response")
            if expected is not None and result["total"] != expected:
                raise ProbeError("invalid_response")
            expected = result["total"]
            for row in result["list"]:
                if not isinstance(row, dict) or type(row.get("id")) not in (int, str) or not str(row["id"]) or str(row["id"]) in seen:
                    raise ProbeError("invalid_response")
                seen.add(str(row["id"]))
                rows.append(row)
            if len(rows) == expected:
                return rows
            if len(rows) > expected or not result["list"]:
                raise ProbeError("invalid_response")
            time.sleep(1)
        raise ProbeError("invalid_response")

    def inventory_hashes(self, goods_id):
        def scan():
            hashes = set()
            for row in self.rows("/merchantApi/goodsCardStorage/list", {"goods_id": goods_id, "status": "0", "first": "", "keywords": ""}):
                code = row.get("secret")
                if not isinstance(code, str) or not code or code.strip() != code or "\n" in code or "\r" in code:
                    raise ProbeError("invalid_response")
                if "goods_id" in row and str(row["goods_id"]) != str(goods_id):
                    raise ProbeError("invalid_response")
                if "status" in row and str(row["status"]) != "0":
                    raise ProbeError("invalid_response")
                digest = hashlib.sha256(code.encode()).hexdigest()
                if digest in hashes:
                    raise ProbeError("invalid_response")
                hashes.add(digest)
            return hashes
        first, second = scan(), scan()
        if first != second:
            raise ProbeError("invalid_response")
        return sorted(second)

    def inventory(self, goods_id):
        hashes = self.inventory_hashes(goods_id)
        return {"unsold_count": len(hashes), "two_scans_agree": True}


def local_products(key_path):
    key = private_read(key_path).decode().strip()
    request = Request(LOCAL_SITE + "/api/v1/admin/tools/ldxp/browser/status", headers={"x-api-key": key})
    try:
        with opener().open(request, timeout=10) as response:
            data = json.load(response)
        if data["code"] != 0 or not isinstance(data["data"]["products"], list):
            raise ValueError()
        return data["data"]["products"]
    except (OSError, ValueError, KeyError, TypeError, ProbeError) as exc:
        raise ProbeError("local_unavailable") from exc


def verify(session_path, key_path, evidence_dir):
    result = {"started_at": time.time(), "pid": os.getpid(), "status": "FAILED",
              "browser_used": False, "inventory_written": False, "production_accessed": False,
              "merchant_origin": ORIGIN, "sub2api_origin": LOCAL_SITE}
    try:
        merchant = Merchant(session_path).load()
        merchant.post("/merchantApi/user/userinfo", {})
        result["merchant_authenticated"] = True
        catalog = merchant.rows("/merchantApi/Goods/list", {"goods_type": "card", "is_proxy": 0, "status": 999})
        result["merchant_product_count"] = len(catalog)
        goods = {str(row["id"]): row for row in catalog}
        checks = []
        for product in local_products(key_path):
            goods_id = product["goods_id"]
            check = {"goods_id": goods_id, "cny_amount": product["cny_amount"],
                     "usd_credit": product["usd_credit"], "found_in_merchant": str(goods_id) in goods}
            if check["found_in_merchant"]:
                check.update(merchant.inventory(goods_id))
            checks.append(check)
        result["test_products"] = checks
        result["local_mappings_present"] = bool(checks) and all(c["found_in_merchant"] for c in checks)
        merchant.save()
        result["status"] = "READ_ONLY_VERIFIED" if result["local_mappings_present"] else "HTTP_ONLY"
    except ProbeError as exc:
        result.update(error=exc.kind, message=str(exc))
    except Exception:
        result.update(error="invalid_response", message=ERRORS["invalid_response"])
    result["finished_at"] = time.time()
    evidence_dir = Path(evidence_dir)
    evidence_dir.mkdir(parents=True, exist_ok=True)
    destination = evidence_dir / ("probe-" + time.strftime("%Y%m%dT%H%M%S") + "-" + secrets.token_hex(4) + ".json")
    with destination.open("x") as stream:
        json.dump(result, stream, ensure_ascii=False, indent=2)
        stream.write("\n")
    return result


def serve(args):
    nonce = secrets.token_urlsafe(32)
    class Handler(BaseHTTPRequestHandler):
        def log_message(self, *_):
            pass

        def reply(self, status, body, kind="application/json"):
            encoded = body.encode()
            self.send_response(status)
            self.send_header("Content-Type", kind + "; charset=utf-8")
            self.send_header("Content-Length", str(len(encoded)))
            self.send_header("Cache-Control", "no-store")
            self.send_header("X-Frame-Options", "DENY")
            self.send_header("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'")
            self.end_headers()
            self.wfile.write(encoded)

        def valid_host(self):
            return self.headers.get("Host") == f"127.0.0.1:{self.server.server_port}"

        def do_GET(self):
            if not self.valid_host() or self.path != "/":
                return self.reply(404, "{}")
            html = Path(__file__).with_suffix(".html").read_text()
            self.reply(200, html.replace("{{NONCE}}", nonce), "text/html")

        def do_POST(self):
            origin = f"http://127.0.0.1:{self.server.server_port}"
            if not self.valid_host() or self.headers.get("Origin") != origin:
                return self.reply(403, "{}")
            try:
                size = int(self.headers.get("Content-Length", "0"))
                if not 0 < size <= 16384 or self.headers.get_content_type() != "application/x-www-form-urlencoded":
                    raise ProbeError("invalid_input")
                fields = parse_qs(self.rfile.read(size).decode(), max_num_fields=8)
                if not hmac.compare_digest(fields.get("nonce", [""])[0], nonce):
                    return self.reply(403, "{}")
                if self.path == "/login":
                    Merchant(args.session).login(fields.get("username", [""])[0], fields.get("password", [""])[0])
                elif self.path != "/verify":
                    raise ProbeError("invalid_input")
                result = verify(args.session, args.admin_key, args.evidence)
                self.reply(200, json.dumps(result, ensure_ascii=False, indent=2))
            except ProbeError as exc:
                self.reply(400, json.dumps({"status": "PAUSED", "error": exc.kind, "message": str(exc)}, ensure_ascii=False))
            except Exception:
                self.reply(400, json.dumps({"status": "PAUSED", "error": "invalid_input", "message": ERRORS["invalid_input"]}, ensure_ascii=False))

    server = HTTPServer(("127.0.0.1", 0), Handler)
    server.timeout = 1
    print(json.dumps({"url": f"http://127.0.0.1:{server.server_port}/", "scope": "login-and-read-only-local-verification"}), flush=True)
    try:
        server.serve_forever()
    finally:
        server.server_close()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=("serve", "verify"))
    parser.add_argument("--session", type=Path, default=DEFAULT_PRIVATE / "session.json")
    parser.add_argument("--admin-key", type=Path, default=DEFAULT_ADMIN)
    parser.add_argument("--evidence", type=Path, required=True)
    args = parser.parse_args()
    os.umask(0o077)
    if args.action == "serve":
        serve(args)
        return 0
    result = verify(args.session, args.admin_key, args.evidence)
    print(json.dumps(result, ensure_ascii=False))
    return 0 if result["status"] == "READ_ONLY_VERIFIED" else 1


if __name__ == "__main__":
    try:
        sys.exit(main())
    except KeyboardInterrupt:
        sys.exit(0)
