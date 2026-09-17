"""Synthetic verification of session isolation and strictly read-only LDXP probing."""
import io
from http.cookiejar import Cookie
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch
from urllib.error import HTTPError
from urllib.request import Request, urlopen

sys.path.insert(0, str(Path(__file__).resolve().parents[2] / "commerce"))
import ldxp_http_probe as probe


class ProbeTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.path = self.root / "private" / "session.json"
        self.client = probe.Merchant(self.path)

    def test_token_and_cookie_survive_new_client_without_password(self):
        self.client.token = "synthetic-token"
        self.client.jar.set_cookie(Cookie(0, "session", "synthetic-cookie", None, False,
            "www.ldxp.cn", False, False, "/", True, True, None, True, None, None, {}, False))
        self.client.save()
        loaded = probe.Merchant(self.path).load()
        self.assertEqual(loaded.token, "synthetic-token")
        request = Request(probe.ORIGIN + "/merchantApi/Goods/list")
        loaded.jar.add_cookie_header(request)
        self.assertIn("synthetic-cookie", request.get_header("Cookie"))
        self.assertEqual(0o600, self.path.stat().st_mode & 0o777)
        self.assertEqual(0o700, self.path.parent.stat().st_mode & 0o777)
        self.assertNotIn("password", self.path.read_text())

    def test_insecure_session_and_symlink_rejected(self):
        self.client.token = "synthetic"
        self.client.save()
        self.path.chmod(0o644)
        with self.assertRaises(probe.ProbeError):
            probe.Merchant(self.path).load()
        self.path.chmod(0o600)
        link = self.root / "session-link"
        link.symlink_to(self.path)
        with self.assertRaises(probe.ProbeError):
            probe.Merchant(link).load()

    def test_foreign_cookie_and_origin_rejected(self):
        self.client.token = "synthetic"
        self.client.save()
        data = json.loads(self.path.read_text())
        data["origin"] = "https://example.com"
        probe.write_private(self.path, data)
        with self.assertRaises(probe.ProbeError):
            probe.Merchant(self.path).load()

    def test_all_mutating_merchant_routes_are_unavailable(self):
        for route in ("/merchantApi/GoodsCardStorage/add", "/merchantApi/Goods/delete", "https://example.com"):
            with self.subTest(route=route), patch.object(self.client.http, "open") as remote:
                with self.assertRaises(probe.ProbeError):
                    self.client.post(route, {}, login=True)
                remote.assert_not_called()

    def test_redirect_cannot_forward_credentials(self):
        with self.assertRaises(probe.ProbeError):
            probe.NoRedirect().redirect_request(None, None, 302, "", {}, "https://example.com")

    def test_html_and_auth_failures_never_include_remote_body(self):
        for raw in (b'<html>synthetic-private-value</html>', b'{"code":401,"msg":"synthetic-private-value"}'):
            with self.subTest(raw=raw), patch.object(self.client.http, "open", return_value=io.BytesIO(raw)):
                with self.assertRaises(probe.ProbeError) as error:
                    self.client.post("/merchantApi/user/userinfo", {})
                self.assertNotIn("synthetic-private-value", str(error.exception))

    def test_safe_mode_stops_before_login_and_save(self):
        with patch.object(self.client, "post", side_effect=[{}, {"safe_mode": 1}]) as remote:
            with self.assertRaises(probe.ProbeError) as error:
                self.client.login("synthetic-user", "synthetic-password")
            self.assertEqual(error.exception.kind, "verification_required")
            self.assertEqual(remote.call_count, 2)
            self.assertFalse(self.path.exists())

    def test_login_validates_identity_before_persisting(self):
        with patch.object(self.client, "post", side_effect=[{}, {"safe_mode": 0}, {"merchant_token": "synthetic"}, probe.ProbeError("login_required")]):
            with self.assertRaises(probe.ProbeError):
                self.client.login("synthetic-user", "synthetic-password")
            self.assertFalse(self.path.exists())

    def test_successful_login_saves_only_session(self):
        with patch.object(self.client, "post", side_effect=[{}, {"safe_mode": 0}, {"merchant_token": "synthetic"}, {"id": 1}]):
            self.client.login("synthetic-user", "synthetic-password")
        self.assertNotIn("synthetic-password", self.path.read_text())
        self.assertNotIn("synthetic-user", self.path.read_text())

    def test_pagination_drift_is_rejected(self):
        with patch.object(self.client, "post", side_effect=[{"total": 2, "list": [{"id": 1}]}, {"total": 1, "list": [{"id": 2}]}]), patch.object(probe.time, "sleep"):
            with self.assertRaises(probe.ProbeError):
                self.client.rows("/merchantApi/Goods/list", {})

    def test_inventory_checks_identity_and_double_scan(self):
        row = {"id": 1, "goods_id": 42, "status": "0", "secret": "synthetic-card"}
        with patch.object(self.client, "rows", side_effect=[[row], [row]]):
            self.assertEqual(self.client.inventory(42), {"unsold_count": 1, "two_scans_agree": True})
        for scans in ([[row], []], [[row, dict(row, id=2)]], [[dict(row, goods_id=99)]], [[dict(row, status=1)]]):
            with self.subTest(scans=scans), patch.object(self.client, "rows", side_effect=scans):
                with self.assertRaises(probe.ProbeError):
                    self.client.inventory(42)

    def test_missing_session_receipt_is_truthful(self):
        result = probe.verify(self.path, self.root / "absent.key", self.root / "evidence")
        self.assertEqual(result["status"], "FAILED")
        self.assertEqual(result["error"], "login_required")
        self.assertFalse(result["browser_used"])
        self.assertFalse(result["inventory_written"])

    def test_verification_queries_all_card_statuses_and_requires_test_mapping(self):
        with patch.object(probe, "Merchant") as merchant, patch.object(probe, "local_products", return_value=[
            {"goods_id": 42, "cny_amount": 5, "usd_credit": 5}]):
            client = merchant.return_value.load.return_value
            client.rows.return_value = [{"id": 42}]
            client.inventory.return_value = {"unsold_count": 3, "two_scans_agree": True}
            result = probe.verify(self.path, self.root / "key", self.root / "evidence")
            client.rows.assert_called_once_with("/merchantApi/Goods/list", {"goods_type": "card", "is_proxy": 0, "status": 999})
            self.assertEqual(result["status"], "READ_ONLY_VERIFIED")
            client.rows.return_value = []
            result = probe.verify(self.path, self.root / "key", self.root / "evidence")
            self.assertEqual(result["status"], "HTTP_ONLY")

    def test_loopback_form_requires_origin_host_and_nonce(self):
        proc = subprocess.Popen([sys.executable, "-B", probe.__file__, "serve", "--evidence", str(self.root / "evidence"),
                                 "--session", str(self.path)], stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
        try:
            address = json.loads(proc.stdout.readline())["url"]
            self.assertTrue(address.startswith("http://127.0.0.1:"))
            with urlopen(address, timeout=5) as response:
                self.assertIn("frame-ancestors 'none'", response.headers["Content-Security-Policy"])
                self.assertIn("no-store", response.headers["Cache-Control"])
            for headers in ({"Origin": "https://example.com"}, {"Origin": address.rstrip("/")}, {"Host": "evil.example"}):
                with self.subTest(headers=headers), self.assertRaises(HTTPError) as error:
                    urlopen(Request(address + "login", data=b"nonce=wrong", headers=headers), timeout=5)
                self.assertEqual(error.exception.code, 403)
        finally:
            proc.terminate()
            proc.communicate(timeout=5)


if __name__ == "__main__":
    unittest.main()
