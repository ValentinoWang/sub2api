from __future__ import annotations

import json
import threading
import unittest
import datetime as dt
from decimal import Decimal
from http.server import BaseHTTPRequestHandler, HTTPServer
from unittest import mock

from xui_sales.exchange_rate_sync import (
    ExchangeRateSyncError,
    _extract_usd_cny,
    _read_admin_api_key,
    update_sub2api,
)


class ExchangeRateSyncTests(unittest.TestCase):
    def test_extract_rate_rejects_out_of_range_values(self) -> None:
        with self.assertRaises(ExchangeRateSyncError):
            _extract_usd_cny({"rates": {"CNY": 100}})

    def test_update_writes_rate_multiplier_source_and_timestamp(self) -> None:
        received: dict[str, object] = {}

        class Handler(BaseHTTPRequestHandler):
            def do_PUT(self) -> None:
                received["path"] = self.path
                received["key"] = self.headers.get("x-api-key")
                received["body"] = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.end_headers()
                self.wfile.write(b'{"code":0,"data":{"message":"updated"}}')

            def log_message(self, *_args: object) -> None:
                return None

        server = HTTPServer(("127.0.0.1", 0), Handler)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        self.addCleanup(server.shutdown)
        self.addCleanup(server.server_close)

        with mock.patch("xui_sales.exchange_rate_sync.datetime") as clock:
            clock.now.return_value = dt.datetime(2026, 7, 26, tzinfo=dt.timezone.utc)
            payload = update_sub2api(
                f"http://127.0.0.1:{server.server_port}", "secret", Decimal("7.2"), "test"
            )

        self.assertEqual("/api/v1/admin/payment/config", received["path"])
        self.assertEqual("secret", received["key"])
        self.assertEqual(7.2, payload["balance_exchange_rate_usd_to_cny"])
        self.assertEqual(0.13888889, payload["balance_recharge_multiplier"])
        self.assertEqual("2026-07-26T00:00:00Z", payload["balance_exchange_rate_updated_at"])

    def test_admin_api_key_can_be_read_from_systemd_credential(self) -> None:
        with self.subTest("file credential"):
            import tempfile

            with tempfile.NamedTemporaryFile(mode="w", encoding="utf-8") as handle:
                handle.write("secret-from-file\n")
                handle.flush()
                with mock.patch.dict(
                    "os.environ",
                    {"SUB2API_ADMIN_API_KEY_FILE": handle.name},
                    clear=True,
                ):
                    self.assertEqual("secret-from-file", _read_admin_api_key())


if __name__ == "__main__":
    unittest.main()
