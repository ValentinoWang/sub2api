from __future__ import annotations

import json
import sqlite3
import tempfile
import unittest
from unittest import mock
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path

from xui_sales.config import Plan, Settings
from xui_sales.fulfillment import fulfill_order, provision_order
from xui_sales.redemption import generate_code
from xui_sales.store import OrderStore
from xui_sales.sub2api import Sub2APIClient, Sub2APIError
from xui_sales.web import create_app
from xui_sales.xianyu import (
    XianyuCallbackError,
    calculate_signature,
    verify_signature,
)


class FakeProvisioner:
    def __init__(self, failures: int = 0):
        self.failures = failures
        self.calls = 0

    def healthcheck(self) -> None:
        return None

    def provision(self, order: dict[str, object]) -> str:
        self.calls += 1
        if self.calls <= self.failures:
            raise OSError("simulated timeout")
        return f"https://subscription.invalid/sub/{order['id']}"


class FakeSub2API:
    def __init__(self, failures: int = 0, user_id: int = 123):
        self.failures = failures
        self.user_id = user_id
        self.resolve_calls = 0
        self.grant_calls = 0
        self.balance_grant_calls = 0
        self.balance_quote_calls = 0

    def healthcheck(self) -> None:
        return None

    def resolve_user(self, email: str) -> int:
        self.resolve_calls += 1
        if email.strip().lower() != "buyer@example.com":
            raise Sub2APIError("未找到唯一匹配的 Sub2API 账户")
        return self.user_id

    def grant_subscription(self, order: dict[str, object]) -> int:
        self.grant_calls += 1
        if self.grant_calls <= self.failures:
            raise Sub2APIError("simulated timeout")
        if order["sub2api_user_id"] != self.user_id:
            raise AssertionError("unexpected Sub2API user")
        return 9001

    def grant_balance(self, order: dict[str, object]) -> int:
        self.balance_grant_calls += 1
        if self.balance_grant_calls <= self.failures:
            raise Sub2APIError("simulated timeout")
        if order["sub2api_user_id"] != self.user_id:
            raise AssertionError("unexpected Sub2API user")
        expected = order.get("api_credited_balance_cents") or order["api_balance_cents"]
        if expected != 1000:
            raise AssertionError("unexpected Sub2API balance")
        return 9002

    def balance_quote(self, cny_cents: int) -> dict[str, object]:
        self.balance_quote_calls += 1
        if cny_cents != 7200:
            raise AssertionError("unexpected CNY amount")
        return {
            "cny_cents": cny_cents,
            "usd_cents": 1000,
            "usd_cny_rate": "7.2",
            "source": "test",
            "updated_at": "2026-07-26T00:00:00Z",
        }


class FulfillmentTests(unittest.TestCase):
    def setUp(self) -> None:
        self.tempdir = tempfile.TemporaryDirectory()
        self.root = Path(self.tempdir.name)
        self.pepper = b"test-pepper-32-bytes-minimum-value"
        self.plan = Plan("monthly", "30 day plan", "", 100, 30, 100, 3, False)
        self.plans_path = self.root / "plans.json"
        self.plans_path.write_text(
            json.dumps(
                [
                    {
                        "id": self.plan.id,
                        "name": self.plan.name,
                        "description": "",
                        "price_cny": "0.00",
                        "duration_days": self.plan.duration_days,
                        "traffic_gb": self.plan.traffic_gb,
                        "ip_limit": self.plan.ip_limit,
                        "enabled": False,
                    }
                ]
            ),
            encoding="utf-8",
        )
        self.mapping_path = self.root / "mappings.json"
        self.mapping_path.write_text(
            json.dumps([{"product_id": "101", "item_id": "202", "plan_id": "monthly"}]),
            encoding="utf-8",
        )
        self.store = OrderStore(self.root / "orders.sqlite3", self.pepper)

    def tearDown(self) -> None:
        self.tempdir.cleanup()

    def settings(self) -> Settings:
        return Settings(
            database_path=self.root / "orders.sqlite3",
            plans_path=self.plans_path,
            public_base_url="https://sales.invalid",
            secret_key="test-secret",
            payments_enabled=False,
            alipay_gateway="https://openapi.alipay.com/gateway.do",
            alipay_app_id="",
            alipay_seller_id="",
            alipay_private_key_path=None,
            alipay_public_key_path=None,
            xui_base_url="https://127.0.0.1:2053",
            xui_api_token="test-token",
            xui_inbound_id=1,
            xui_flow="xtls-rprx-vision",
            xui_insecure_local_tls=True,
            subscription_base_url="https://subscription.invalid",
            redemption_pepper=self.pepper,
            xianyu_callbacks_enabled=True,
            xianyu_app_id="app-123",
            xianyu_app_secret="secret-456",
            xianyu_signature_merchant_id=None,
            xianyu_products_path=self.mapping_path,
            xianyu_timestamp_skew_seconds=300,
        )

    def create_code(self) -> str:
        code = generate_code()
        self.store.create_redemption_batch(self.plan, [code])
        return code

    def create_bundle_code(self) -> str:
        code = generate_code()
        plan = Plan(
            "bundle",
            "API and VPN bundle",
            "",
            100,
            30,
            100,
            3,
            False,
            sub2api_group_id=456,
            api_validity_days=30,
        )
        self.store.create_redemption_batch(plan, [code])
        return code

    def create_balance_code(self) -> str:
        code = generate_code()
        plan = Plan(
            "api-balance-10",
            "API balance 10 USD",
            "",
            7200,
            1,
            1,
            0,
            False,
            sub2api_balance_cents=1000,
        )
        self.store.create_redemption_batch(plan, [code])
        return code

    def test_concurrent_double_redemption_creates_one_order(self) -> None:
        code = self.create_code()
        database_bytes = (self.root / "orders.sqlite3").read_bytes()
        self.assertNotIn(code.encode(), database_bytes)
        self.assertNotIn(code.replace("-", "").encode(), database_bytes)
        with ThreadPoolExecutor(max_workers=8) as executor:
            results = list(executor.map(lambda _: self.store.redeem_code(code), range(16)))
        self.assertEqual(1, len({result[0]["id"] for result in results}))
        self.assertEqual(1, len({result[1] for result in results}))
        with sqlite3.connect(self.root / "orders.sqlite3") as connection:
            self.assertEqual(1, connection.execute("SELECT COUNT(*) FROM orders").fetchone()[0])

    def test_failed_provisioning_is_recoverable(self) -> None:
        code = self.create_code()
        order, _ = self.store.redeem_code(code)
        provisioner = FakeProvisioner(failures=1)
        with self.assertRaises(OSError):
            provision_order(self.store, provisioner, order["id"])
        self.assertEqual("provision_failed", self.store.get(order["id"])["status"])
        self.assertTrue(provision_order(self.store, provisioner, order["id"]))
        self.assertEqual("active", self.store.get(order["id"])["status"])
        self.assertEqual("redeemed", self.store.inventory_counts()[0]["status"])

    def test_api_success_vpn_failure_retries_only_vpn(self) -> None:
        code = self.create_bundle_code()
        order, _ = self.store.redeem_code(code, sub2api_user_id=123)
        sub2api = FakeSub2API()
        provisioner = FakeProvisioner(failures=1)
        with self.assertRaises(OSError):
            fulfill_order(self.store, provisioner, sub2api, order["id"])
        failed = self.store.get(order["id"])
        self.assertEqual("api_active", failed["api_status"])
        self.assertEqual("vpn_failed", failed["vpn_status"])
        self.assertEqual("partial_failed", failed["fulfillment_status"])
        self.assertTrue(fulfill_order(self.store, provisioner, sub2api, order["id"]))
        self.assertEqual(1, sub2api.grant_calls)
        self.assertEqual(2, provisioner.calls)
        self.assertEqual("active", self.store.get(order["id"])["fulfillment_status"])

    def test_vpn_success_api_failure_retries_only_api(self) -> None:
        code = self.create_bundle_code()
        order, _ = self.store.redeem_code(code, sub2api_user_id=123)
        sub2api = FakeSub2API(failures=1)
        provisioner = FakeProvisioner()
        with self.assertRaises(Sub2APIError):
            fulfill_order(self.store, provisioner, sub2api, order["id"])
        failed = self.store.get(order["id"])
        self.assertEqual("api_failed", failed["api_status"])
        self.assertEqual("vpn_active", failed["vpn_status"])
        self.assertEqual("partial_failed", failed["fulfillment_status"])
        self.assertTrue(fulfill_order(self.store, provisioner, sub2api, order["id"]))
        self.assertEqual(2, sub2api.grant_calls)
        self.assertEqual(1, provisioner.calls)

    def test_balance_only_fulfillment_never_calls_xui(self) -> None:
        code = self.create_balance_code()
        order, _ = self.store.redeem_code(code, sub2api_user_id=123)
        sub2api = FakeSub2API()
        provisioner = FakeProvisioner()

        self.assertTrue(fulfill_order(self.store, provisioner, sub2api, order["id"]))

        active = self.store.get(order["id"])
        self.assertEqual("active", active["fulfillment_status"])
        self.assertEqual("api_active", active["api_status"])
        self.assertFalse(active["vpn_required"])
        self.assertEqual(1, sub2api.balance_grant_calls)
        self.assertEqual(1, sub2api.balance_quote_calls)
        self.assertEqual(0, sub2api.grant_calls)
        self.assertEqual(0, provisioner.calls)
        self.assertEqual("redeemed", self.store.inventory_counts()[0]["status"])

    def test_balance_only_failure_retries_without_xui(self) -> None:
        code = self.create_balance_code()
        order, _ = self.store.redeem_code(code, sub2api_user_id=123)
        sub2api = FakeSub2API(failures=1)
        provisioner = FakeProvisioner()

        with self.assertRaises(Sub2APIError):
            fulfill_order(self.store, provisioner, sub2api, order["id"])
        self.assertEqual("api_failed", self.store.get(order["id"])["api_status"])
        self.assertTrue(fulfill_order(self.store, provisioner, sub2api, order["id"]))
        self.assertEqual(2, sub2api.balance_grant_calls)
        self.assertEqual(1, sub2api.balance_quote_calls)
        self.assertEqual(0, provisioner.calls)

    def test_balance_only_redeem_route_is_idempotent_without_xui(self) -> None:
        code = self.create_balance_code()
        balance_plans = self.root / "balance-plans.json"
        balance_plans.write_text(
            json.dumps(
                [
                    {
                        "id": "api-balance-10",
                        "name": "API balance 10 USD",
                        "price_cny": "10.00",
                        "duration_days": 1,
                        "traffic_gb": 1,
                        "ip_limit": 0,
                        "enabled": True,
                        "sub2api_balance": "10.00",
                    }
                ]
            ),
            encoding="utf-8",
        )
        settings = Settings(
            **{
                **self.settings().__dict__,
                "plans_path": balance_plans,
                "xianyu_callbacks_enabled": False,
            }
        )
        sub2api = FakeSub2API()
        provisioner = FakeProvisioner()
        app = create_app(
            settings=settings, store=self.store, provisioner=provisioner, sub2api=sub2api
        )
        payload = {"code": code, "sub2api_email": "buyer@example.com"}
        client = app.test_client()

        first = client.post("/redeem", data=payload)
        second = client.post("/redeem", data=payload)

        self.assertEqual(303, first.status_code)
        self.assertEqual(first.headers["Location"], second.headers["Location"])
        self.assertEqual(1, sub2api.balance_grant_calls)
        self.assertEqual(0, provisioner.calls)

    def test_lost_sub2api_response_is_reconciled_by_internal_code(self) -> None:
        order = {
            "id": "XR20260725ABCDEF",
            "sub2api_user_id": 123,
            "sub2api_group_id": 456,
            "api_validity_days": 30,
        }

        class LostResponseClient(Sub2APIClient):
            def _request(self, method, path, payload=None, idempotency_key=None):
                if method == "POST":
                    raise Sub2APIError("Sub2API request failed: TimeoutError")
                return {
                    "items": [
                        {
                            "id": 77,
                            "code": self.internal_code(str(order["id"])),
                            "status": "used",
                            "used_by": 123,
                            "group_id": 456,
                        }
                    ]
                }

        client = LostResponseClient("http://127.0.0.1:18080", "secret")
        self.assertEqual(77, client.grant_subscription(order))

    def test_lost_balance_response_is_reconciled_by_internal_code(self) -> None:
        order = {
            "id": "XR20260725BALANCE",
            "sub2api_user_id": 123,
            "api_balance_cents": 1000,
        }

        class LostBalanceResponseClient(Sub2APIClient):
            def _request(self, method, path, payload=None, idempotency_key=None):
                if method == "POST":
                    raise Sub2APIError("Sub2API request failed: TimeoutError")
                return {
                    "items": [
                        {
                            "id": 78,
                            "code": self.internal_balance_code(str(order["id"])),
                            "type": "balance",
                            "value": 10,
                            "status": "used",
                            "used_by": 123,
                        }
                    ]
                }

        client = LostBalanceResponseClient("http://127.0.0.1:18080", "secret")
        self.assertEqual(78, client.grant_balance(order))

    def test_invalid_bundle_user_does_not_consume_code(self) -> None:
        code = self.create_bundle_code()
        bundle_plans = self.root / "bundle-plans.json"
        bundle_plans.write_text(
            json.dumps(
                [
                    {
                        "id": "bundle",
                        "name": "API and VPN bundle",
                        "price_cny": "0.00",
                        "duration_days": 30,
                        "traffic_gb": 100,
                        "ip_limit": 3,
                        "enabled": False,
                        "sub2api_group_id": 456,
                        "api_validity_days": 30,
                    }
                ]
            ),
            encoding="utf-8",
        )
        settings = self.settings()
        settings = Settings(
            **{
                **settings.__dict__,
                "plans_path": bundle_plans,
                "xianyu_callbacks_enabled": False,
            }
        )
        app = create_app(
            settings=settings,
            store=self.store,
            provisioner=FakeProvisioner(),
            sub2api=FakeSub2API(),
        )
        response = app.test_client().post(
            "/redeem", data={"code": code, "sub2api_email": "unknown@example.com"}
        )
        self.assertEqual(400, response.status_code)
        with sqlite3.connect(self.root / "orders.sqlite3") as connection:
            self.assertEqual(0, connection.execute("SELECT COUNT(*) FROM orders").fetchone()[0])
            self.assertEqual(
                "available",
                connection.execute("SELECT status FROM redemption_codes").fetchone()[0],
            )

    def test_bundle_redeem_route_is_idempotent(self) -> None:
        code = self.create_bundle_code()
        bundle_plans = self.root / "bundle-plans.json"
        bundle_plans.write_text(
            json.dumps(
                [
                    {
                        "id": "bundle",
                        "name": "API and VPN bundle",
                        "price_cny": "0.00",
                        "duration_days": 30,
                        "traffic_gb": 100,
                        "ip_limit": 3,
                        "enabled": False,
                        "sub2api_group_id": 456,
                        "api_validity_days": 30,
                    }
                ]
            ),
            encoding="utf-8",
        )
        settings = Settings(
            **{
                **self.settings().__dict__,
                "plans_path": bundle_plans,
                "xianyu_callbacks_enabled": False,
            }
        )
        sub2api = FakeSub2API()
        provisioner = FakeProvisioner()
        app = create_app(
            settings=settings, store=self.store, provisioner=provisioner, sub2api=sub2api
        )
        client = app.test_client()
        payload = {"code": code, "sub2api_email": "buyer@example.com"}
        first = client.post("/redeem", data=payload)
        second = client.post("/redeem", data=payload)
        self.assertEqual(303, first.status_code)
        self.assertEqual(first.headers["Location"], second.headers["Location"])
        self.assertEqual(1, sub2api.grant_calls)
        self.assertEqual(1, provisioner.calls)

    def test_signature_example_and_timestamp_gate(self) -> None:
        body = b'{"product_id":"219530767978565"}'
        signature = calculate_signature(
            body,
            "203413189371893",
            1636087298,
            "o9wl81dncmvby3ijpq7eur456zhgtaxs",
        )
        self.assertEqual("c26c8a48809141f3dd80bd9b9ddb41ea", signature)
        verify_signature(
            body,
            "203413189371893",
            "1636087298",
            signature,
            "203413189371893",
            "o9wl81dncmvby3ijpq7eur456zhgtaxs",
            300,
            now=1636087298,
        )
        with self.assertRaisesRegex(XianyuCallbackError, "stale"):
            verify_signature(
                body,
                "203413189371893",
                "1636087298",
                signature,
                "203413189371893",
                "o9wl81dncmvby3ijpq7eur456zhgtaxs",
                300,
                now=1636087599,
            )

    def test_callback_replay_unknown_product_and_out_of_order(self) -> None:
        app = create_app(
            settings=self.settings(), store=self.store, provisioner=FakeProvisioner()
        )
        client = app.test_client()
        payload = {
            "seller_id": 1,
            "user_name": "buyer",
            "order_no": "123456789",
            "order_type": 7,
            "order_status": 12,
            "refund_status": 0,
            "modify_time": 2000,
            "product_id": 101,
            "item_id": 202,
        }

        def send(data: dict[str, object], timestamp: int = 2_000) -> object:
            body = json.dumps(data, separators=(",", ":")).encode()
            sign = calculate_signature(body, "app-123", timestamp, "secret-456")
            return client.post(
                f"/callbacks/xianyu?appid=app-123&timestamp={timestamp}&sign={sign}",
                data=body,
                content_type="application/json",
                environ_overrides={"REMOTE_ADDR": "198.51.100.1"},
            )

        with mock.patch("xui_sales.xianyu.time.time", return_value=2_000):
            first = send(payload)
            replay = send(payload)
            unknown = send({**payload, "item_id": 999, "modify_time": 2001})
            older = send({**payload, "order_status": 11, "modify_time": 1999})
            stale = send(payload, 1_699)
            forged = client.post(
                "/callbacks/xianyu?appid=app-123&timestamp=2000&sign=00000000000000000000000000000000",
                data=json.dumps(payload, separators=(",", ":")).encode(),
                content_type="application/json",
            )
        self.assertEqual(200, first.status_code)
        self.assertEqual(200, replay.status_code)
        self.assertEqual(400, unknown.status_code)
        self.assertEqual(200, older.status_code)
        self.assertEqual(400, stale.status_code)
        self.assertEqual(400, forged.status_code)
        with sqlite3.connect(self.root / "orders.sqlite3") as connection:
            self.assertEqual(
                12,
                connection.execute(
                    "SELECT order_status FROM xianyu_orders WHERE order_no='123456789'"
                ).fetchone()[0],
            )
            self.assertEqual(
                2,
                connection.execute(
                    "SELECT COUNT(*) FROM xianyu_order_events WHERE order_no='123456789'"
                ).fetchone()[0],
            )

    def test_redeem_route_is_idempotent(self) -> None:
        code = self.create_code()
        provisioner = FakeProvisioner()
        app = create_app(
            settings=self.settings(), store=self.store, provisioner=provisioner
        )
        client = app.test_client()
        page = client.get("/").get_data(as_text=True)
        self.assertNotIn("xianyu_order_no", page)
        self.assertNotIn("闲鱼订单号", page)
        self.assertEqual(1, page.count('<input id="redemption-code"'))
        first = client.post("/redeem", data={"code": code})
        second = client.post("/redeem", data={"code": code})
        self.assertEqual(303, first.status_code)
        self.assertEqual(first.headers["Location"], second.headers["Location"])
        self.assertEqual(1, provisioner.calls)


if __name__ == "__main__":
    unittest.main()
