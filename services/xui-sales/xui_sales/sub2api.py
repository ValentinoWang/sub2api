from __future__ import annotations

import json
import urllib.error
import urllib.parse
import urllib.request
from dataclasses import dataclass
from datetime import datetime, timezone
from decimal import Decimal, InvalidOperation, ROUND_HALF_UP
from hashlib import sha256
from typing import Any


class Sub2APIError(RuntimeError):
    pass


@dataclass(frozen=True)
class Sub2APIClient:
    base_url: str
    admin_api_key: str
    timeout_seconds: int = 8

    def _request(
        self,
        method: str,
        path: str,
        payload: dict[str, Any] | None = None,
        idempotency_key: str | None = None,
    ) -> Any:
        body = None
        headers = {"x-api-key": self.admin_api_key, "Accept": "application/json"}
        if payload is not None:
            body = json.dumps(payload, separators=(",", ":")).encode("utf-8")
            headers["Content-Type"] = "application/json"
        if idempotency_key is not None:
            headers["Idempotency-Key"] = idempotency_key
        request = urllib.request.Request(
            self.base_url + path, data=body, headers=headers, method=method
        )
        try:
            with urllib.request.urlopen(request, timeout=self.timeout_seconds) as response:
                result = json.load(response)
        except urllib.error.HTTPError as exc:
            try:
                detail = json.load(exc).get("message", exc.reason)
            except (AttributeError, json.JSONDecodeError):
                detail = exc.reason
            raise Sub2APIError(f"Sub2API rejected request: {detail}") from exc
        except (urllib.error.URLError, TimeoutError, json.JSONDecodeError) as exc:
            raise Sub2APIError(f"Sub2API request failed: {type(exc).__name__}") from exc
        if not isinstance(result, dict) or result.get("code") not in {0, "0"}:
            detail = result.get("message", "invalid response") if isinstance(result, dict) else "invalid response"
            raise Sub2APIError(f"Sub2API rejected request: {detail}")
        return result.get("data")

    @staticmethod
    def internal_code(order_id: str) -> str:
        return "BND" + sha256(order_id.encode("utf-8")).hexdigest()[:24].upper()

    @staticmethod
    def internal_balance_code(order_id: str) -> str:
        return "BAL" + sha256(order_id.encode("utf-8")).hexdigest()[:24].upper()

    def healthcheck(self) -> None:
        self._request("GET", "/api/v1/admin/groups/all")

    def balance_quote(self, cny_cents: int) -> dict[str, Any]:
        if cny_cents <= 0:
            raise Sub2APIError("人民币充值金额必须大于 0")
        data = self._request("GET", "/api/v1/admin/payment/config")
        if not isinstance(data, dict):
            raise Sub2APIError("Sub2API returned an invalid payment config")
        try:
            rate = Decimal(str(data["balance_exchange_rate_usd_to_cny"]))
            updated_at = datetime.fromisoformat(
                str(data["balance_exchange_rate_updated_at"]).replace("Z", "+00:00")
            )
        except (KeyError, InvalidOperation, ValueError) as exc:
            raise Sub2APIError("Sub2API 实时汇率尚未配置") from exc
        if rate < Decimal("4") or rate > Decimal("10"):
            raise Sub2APIError("Sub2API USD/CNY 汇率超出安全范围")
        if updated_at.tzinfo is None:
            raise Sub2APIError("Sub2API 汇率更新时间缺少时区")
        age_seconds = (datetime.now(timezone.utc) - updated_at.astimezone(timezone.utc)).total_seconds()
        if age_seconds < -300 or age_seconds > 86400:
            raise Sub2APIError("Sub2API 实时汇率已过期，请稍后重试")
        usd_cents = int(
            (Decimal(cny_cents) / rate).quantize(Decimal("1"), rounding=ROUND_HALF_UP)
        )
        if usd_cents <= 0:
            raise Sub2APIError("人民币金额换算后的美元余额过小")
        return {
            "cny_cents": cny_cents,
            "usd_cents": usd_cents,
            "usd_cny_rate": str(rate),
            "source": str(data.get("balance_exchange_rate_source", "")).strip(),
            "updated_at": updated_at.astimezone(timezone.utc).isoformat().replace("+00:00", "Z"),
        }

    def resolve_user(self, raw_email: str) -> int:
        email = raw_email.strip().lower()
        if not email or "@" not in email or len(email) > 254:
            raise Sub2APIError("请输入有效的 Sub2API 账户邮箱")
        query = urllib.parse.urlencode(
            {"page": 1, "page_size": 100, "search": email, "include_subscriptions": "false"}
        )
        data = self._request("GET", f"/api/v1/admin/users?{query}")
        items = data.get("items") if isinstance(data, dict) else None
        if not isinstance(items, list):
            raise Sub2APIError("Sub2API returned an invalid user list")
        exact = [
            item
            for item in items
            if isinstance(item, dict) and str(item.get("email", "")).strip().lower() == email
        ]
        if len(exact) != 1:
            raise Sub2APIError("未找到唯一匹配的 Sub2API 账户")
        if exact[0].get("status") != "active":
            raise Sub2APIError("Sub2API 账户当前不可用")
        user_id = exact[0].get("id")
        if not isinstance(user_id, int) or user_id <= 0:
            raise Sub2APIError("Sub2API returned an invalid user")
        return user_id

    def grant_subscription(self, order: dict[str, Any]) -> int:
        code = self.internal_code(str(order["id"]))
        user_id = int(order["sub2api_user_id"])
        group_id = int(order["sub2api_group_id"])
        validity_days = int(order["api_validity_days"])
        payload = {
            "code": code,
            "type": "subscription",
            "value": validity_days,
            "user_id": user_id,
            "group_id": group_id,
            "validity_days": validity_days,
            "notes": f"bundle fulfillment {order['id']}",
        }
        idempotency_key = f"bundle:{order['id']}:api:v1"
        try:
            data = self._request(
                "POST",
                "/api/v1/admin/redeem-codes/create-and-redeem",
                payload,
                idempotency_key,
            )
        except Sub2APIError as original:
            try:
                return self._find_completed_grant(code, user_id, group_id)
            except Sub2APIError:
                raise original
        return self._validate_grant(data, user_id, group_id)

    def grant_balance(self, order: dict[str, Any]) -> int:
        code = self.internal_balance_code(str(order["id"]))
        user_id = int(order["sub2api_user_id"])
        balance_cents = int(order.get("api_credited_balance_cents") or order["api_balance_cents"])
        if balance_cents <= 0:
            raise Sub2APIError("balance grant must be positive")
        payload = {
            "code": code,
            "type": "balance",
            "value": balance_cents / 100,
            "user_id": user_id,
            "notes": f"balance fulfillment {order['id']}",
        }
        try:
            data = self._request(
                "POST",
                "/api/v1/admin/redeem-codes/create-and-redeem",
                payload,
                f"balance:{order['id']}:api:v1",
            )
        except Sub2APIError as original:
            try:
                return self._find_completed_balance_grant(code, user_id, balance_cents)
            except Sub2APIError:
                raise original
        redeem_code = data.get("redeem_code") if isinstance(data, dict) else None
        if not isinstance(redeem_code, dict):
            raise Sub2APIError("Sub2API returned an invalid grant result")
        return self._validate_balance_code(redeem_code, user_id, balance_cents)

    def _find_completed_balance_grant(
        self, code: str, user_id: int, balance_cents: int
    ) -> int:
        query = urllib.parse.urlencode({"page": 1, "page_size": 20, "search": code})
        data = self._request("GET", f"/api/v1/admin/redeem-codes?{query}")
        items = data.get("items") if isinstance(data, dict) else None
        if not isinstance(items, list):
            raise Sub2APIError("Sub2API returned an invalid redeem-code list")
        exact = [item for item in items if isinstance(item, dict) and item.get("code") == code]
        if len(exact) != 1:
            raise Sub2APIError("Sub2API balance grant could not be reconciled")
        return self._validate_balance_code(exact[0], user_id, balance_cents)

    @staticmethod
    def _validate_balance_code(item: dict[str, Any], user_id: int, balance_cents: int) -> int:
        try:
            value = Decimal(str(item.get("value")))
        except InvalidOperation as exc:
            raise Sub2APIError("Sub2API returned an invalid balance value") from exc
        if (
            item.get("status") != "used"
            or item.get("used_by") != user_id
            or item.get("type") != "balance"
            or value != Decimal(balance_cents) / 100
        ):
            raise Sub2APIError("Sub2API balance grant does not match this order")
        redeem_id = item.get("id")
        if not isinstance(redeem_id, int) or redeem_id <= 0:
            raise Sub2APIError("Sub2API returned an invalid redeem-code id")
        return redeem_id

    def _find_completed_grant(self, code: str, user_id: int, group_id: int) -> int:
        query = urllib.parse.urlencode({"page": 1, "page_size": 20, "search": code})
        data = self._request("GET", f"/api/v1/admin/redeem-codes?{query}")
        items = data.get("items") if isinstance(data, dict) else None
        if not isinstance(items, list):
            raise Sub2APIError("Sub2API returned an invalid redeem-code list")
        exact = [item for item in items if isinstance(item, dict) and item.get("code") == code]
        if len(exact) != 1:
            raise Sub2APIError("Sub2API grant could not be reconciled")
        return self._validate_redeem_code(exact[0], user_id, group_id)

    def _validate_grant(self, data: Any, user_id: int, group_id: int) -> int:
        redeem_code = data.get("redeem_code") if isinstance(data, dict) else None
        if not isinstance(redeem_code, dict):
            raise Sub2APIError("Sub2API returned an invalid grant result")
        return self._validate_redeem_code(redeem_code, user_id, group_id)

    @staticmethod
    def _validate_redeem_code(item: dict[str, Any], user_id: int, group_id: int) -> int:
        if (
            item.get("status") != "used"
            or item.get("used_by") != user_id
            or item.get("group_id") != group_id
        ):
            raise Sub2APIError("Sub2API grant does not match this order")
        redeem_id = item.get("id")
        if not isinstance(redeem_id, int) or redeem_id <= 0:
            raise Sub2APIError("Sub2API returned an invalid redeem-code id")
        return redeem_id
