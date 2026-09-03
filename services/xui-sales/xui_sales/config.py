from __future__ import annotations

import json
import os
from dataclasses import dataclass
from decimal import Decimal, InvalidOperation
from pathlib import Path
from typing import Any
from urllib.parse import urlparse, urlunparse


@dataclass(frozen=True)
class Plan:
    id: str
    name: str
    description: str
    price_cents: int
    duration_days: int
    traffic_gb: int
    ip_limit: int
    enabled: bool
    sub2api_group_id: int | None = None
    api_validity_days: int | None = None
    sub2api_balance_cents: int | None = None

    @property
    def is_bundle(self) -> bool:
        return self.sub2api_group_id is not None

    @property
    def is_api_balance(self) -> bool:
        return self.sub2api_balance_cents is not None

    @property
    def requires_sub2api(self) -> bool:
        return self.is_bundle or self.is_api_balance

    @property
    def vpn_required(self) -> bool:
        return not self.is_api_balance

    @property
    def price_text(self) -> str:
        return f"{self.price_cents / 100:.2f}"

    @property
    def sub2api_balance_text(self) -> str:
        if self.sub2api_balance_cents is None:
            return ""
        return f"{self.sub2api_balance_cents / 100:.2f}"


def _required_env(name: str) -> str:
    value = os.environ.get(name, "").strip()
    if not value:
        raise RuntimeError(f"required environment variable is empty: {name}")
    return value


def _parse_bool(name: str, default: bool = False) -> bool:
    raw = os.environ.get(name)
    if raw is None:
        return default
    if raw.lower() in {"1", "true", "yes", "on"}:
        return True
    if raw.lower() in {"0", "false", "no", "off"}:
        return False
    raise RuntimeError(f"{name} must be a boolean")


def load_plans(path: Path) -> dict[str, Plan]:
    try:
        raw: Any = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise RuntimeError(f"cannot load plans from {path}: {exc}") from exc
    if not isinstance(raw, list) or not raw:
        raise RuntimeError("plans file must contain a non-empty JSON array")

    plans: dict[str, Plan] = {}
    for index, item in enumerate(raw):
        if not isinstance(item, dict):
            raise RuntimeError(f"plan {index} must be an object")
        try:
            plan_id = str(item["id"]).strip()
            price = Decimal(str(item["price_cny"]))
            price_cents = int(price * 100)
            balance_raw = item.get("sub2api_balance")
            balance = Decimal(str(balance_raw)) if balance_raw is not None else None
            balance_cents = int(balance * 100) if balance is not None else None
            plan = Plan(
                id=plan_id,
                name=str(item["name"]).strip(),
                description=str(item.get("description", "")).strip(),
                price_cents=price_cents,
                duration_days=int(item["duration_days"]),
                traffic_gb=int(item["traffic_gb"]),
                ip_limit=int(item["ip_limit"]),
                enabled=bool(item.get("enabled", True)),
                sub2api_group_id=(
                    int(item["sub2api_group_id"])
                    if item.get("sub2api_group_id") is not None
                    else None
                ),
                api_validity_days=(
                    int(item["api_validity_days"])
                    if item.get("api_validity_days") is not None
                    else None
                ),
                sub2api_balance_cents=balance_cents,
            )
        except (KeyError, TypeError, ValueError, InvalidOperation) as exc:
            raise RuntimeError(f"invalid plan at index {index}: {exc}") from exc
        if not plan.id or not plan.name:
            raise RuntimeError(f"plan {index} has an empty id or name")
        if plan.id in plans:
            raise RuntimeError(f"duplicate plan id: {plan.id}")
        if price != Decimal(price_cents) / 100 or price_cents < 0:
            raise RuntimeError(f"plan {plan.id} price must be non-negative with at most 2 decimals")
        if plan.enabled and price_cents == 0:
            raise RuntimeError(f"enabled plan {plan.id} must have a positive price")
        if plan.duration_days <= 0 or plan.traffic_gb <= 0 or plan.ip_limit < 0:
            raise RuntimeError(f"plan {plan.id} has invalid limits")
        if (plan.sub2api_group_id is None) != (plan.api_validity_days is None):
            raise RuntimeError(
                f"plan {plan.id} must set sub2api_group_id and api_validity_days together"
            )
        if plan.sub2api_group_id is not None and (
            plan.sub2api_group_id <= 0 or plan.api_validity_days <= 0
        ):
            raise RuntimeError(f"plan {plan.id} has invalid Sub2API subscription settings")
        if plan.is_bundle and plan.is_api_balance:
            raise RuntimeError(f"plan {plan.id} cannot combine subscription and balance grants")
        if balance is not None and (
            balance != Decimal(balance_cents) / 100 or balance_cents <= 0
        ):
            raise RuntimeError(
                f"plan {plan.id} Sub2API balance must be positive with at most 2 decimals"
            )
        plans[plan.id] = plan
    return plans


@dataclass(frozen=True)
class Settings:
    database_path: Path
    plans_path: Path
    public_base_url: str
    secret_key: str
    payments_enabled: bool
    alipay_gateway: str
    alipay_app_id: str
    alipay_seller_id: str
    alipay_private_key_path: Path | None
    alipay_public_key_path: Path | None
    xui_base_url: str
    xui_api_token: str
    xui_inbound_id: int
    xui_flow: str
    xui_insecure_local_tls: bool
    subscription_base_url: str
    redemption_pepper: bytes
    xianyu_callbacks_enabled: bool
    xianyu_app_id: str
    xianyu_app_secret: str
    xianyu_signature_merchant_id: str | None
    xianyu_products_path: Path | None
    xianyu_timestamp_skew_seconds: int
    sub2api_enabled: bool = False
    sub2api_base_url: str = ""
    sub2api_admin_api_key: str = ""
    sub2api_timeout_seconds: int = 8

    @classmethod
    def from_env(cls) -> "Settings":
        public_base_url = _required_env("PUBLIC_BASE_URL").rstrip("/")
        parsed_public = urlparse(public_base_url)
        if parsed_public.scheme != "https" or not parsed_public.netloc:
            raise RuntimeError("PUBLIC_BASE_URL must be an absolute https URL")

        payments_enabled = _parse_bool("PAYMENTS_ENABLED", False)
        alipay_app_id = os.environ.get("ALIPAY_APP_ID", "").strip()
        private_path_raw = os.environ.get("ALIPAY_PRIVATE_KEY_FILE", "").strip()
        public_path_raw = os.environ.get("ALIPAY_PUBLIC_KEY_FILE", "").strip()
        alipay_seller_id = os.environ.get("ALIPAY_SELLER_ID", "").strip()
        if payments_enabled and (
            not alipay_app_id or not alipay_seller_id or not private_path_raw or not public_path_raw
        ):
            raise RuntimeError(
                "payments are enabled but ALIPAY_APP_ID, ALIPAY_SELLER_ID, or RSA key files are missing"
            )
        alipay_gateway = os.environ.get(
            "ALIPAY_GATEWAY", "https://openapi.alipay.com/gateway.do"
        ).strip()
        if alipay_gateway not in {
            "https://openapi.alipay.com/gateway.do",
            "https://openapi-sandbox.dl.alipaydev.com/gateway.do",
        }:
            raise RuntimeError("ALIPAY_GATEWAY must be an official Alipay gateway")

        xui_base_url = os.environ.get("XUI_BASE_URL", "").strip().rstrip("/")
        if not xui_base_url:
            installed_url = _required_env("XUI_ACCESS_URL")
            parsed_installed = urlparse(installed_url)
            if parsed_installed.scheme != "https" or not parsed_installed.port:
                raise RuntimeError("XUI_ACCESS_URL cannot be converted to a loopback URL")
            xui_base_url = urlunparse(
                ("https", f"127.0.0.1:{parsed_installed.port}", parsed_installed.path.rstrip("/"), "", "", "")
            )
        parsed_xui = urlparse(xui_base_url)
        if parsed_xui.scheme != "https" or not parsed_xui.netloc:
            raise RuntimeError("XUI_BASE_URL must be an absolute https URL")
        insecure_local = _parse_bool("XUI_INSECURE_LOCAL_TLS", False)
        if insecure_local and parsed_xui.hostname not in {"127.0.0.1", "localhost", "::1"}:
            raise RuntimeError("XUI_INSECURE_LOCAL_TLS is allowed only for a loopback URL")

        redemption_pepper_path = Path(_required_env("REDEMPTION_PEPPER_FILE"))
        redemption_pepper = redemption_pepper_path.read_bytes().strip()
        if len(redemption_pepper) < 32:
            raise RuntimeError("REDEMPTION_PEPPER_FILE must contain at least 32 bytes")

        xianyu_callbacks_enabled = _parse_bool("XIANYU_CALLBACKS_ENABLED", False)
        xianyu_app_id = os.environ.get("XIANYU_APP_ID", "").strip()
        xianyu_secret_path_raw = os.environ.get("XIANYU_APP_SECRET_FILE", "").strip()
        xianyu_products_path_raw = os.environ.get("XIANYU_PRODUCTS_PATH", "").strip()
        xianyu_app_secret = ""
        if xianyu_secret_path_raw:
            xianyu_app_secret = Path(xianyu_secret_path_raw).read_text(encoding="utf-8").strip()
        if xianyu_callbacks_enabled and (
            not xianyu_app_id or not xianyu_app_secret or not xianyu_products_path_raw
        ):
            raise RuntimeError(
                "Xianyu callbacks are enabled but app ID, app secret, or product mappings are missing"
            )
        timestamp_skew = int(os.environ.get("XIANYU_TIMESTAMP_SKEW_SECONDS", "300"))
        if not 1 <= timestamp_skew <= 900:
            raise RuntimeError("XIANYU_TIMESTAMP_SKEW_SECONDS must be between 1 and 900")

        sub2api_enabled = _parse_bool("SUB2API_ENABLED", False)
        sub2api_base_url = os.environ.get("SUB2API_BASE_URL", "").strip().rstrip("/")
        sub2api_key_path_raw = os.environ.get("SUB2API_ADMIN_API_KEY_FILE", "").strip()
        sub2api_admin_api_key = ""
        if sub2api_key_path_raw:
            sub2api_admin_api_key = Path(sub2api_key_path_raw).read_text(encoding="utf-8").strip()
        sub2api_timeout_seconds = int(os.environ.get("SUB2API_TIMEOUT_SECONDS", "8"))
        if sub2api_enabled:
            parsed_sub2api = urlparse(sub2api_base_url)
            if parsed_sub2api.scheme not in {"http", "https"} or not parsed_sub2api.netloc:
                raise RuntimeError("SUB2API_BASE_URL must be an absolute http(s) URL")
            if parsed_sub2api.scheme == "http" and parsed_sub2api.hostname not in {
                "127.0.0.1",
                "localhost",
                "::1",
            }:
                raise RuntimeError("plain HTTP SUB2API_BASE_URL is allowed only over loopback")
            if not sub2api_admin_api_key:
                raise RuntimeError("SUB2API_ADMIN_API_KEY_FILE is empty")
        if not 1 <= sub2api_timeout_seconds <= 30:
            raise RuntimeError("SUB2API_TIMEOUT_SECONDS must be between 1 and 30")

        return cls(
            database_path=Path(os.environ.get("DATABASE_PATH", "/var/lib/xui-sales/orders.sqlite3")),
            plans_path=Path(os.environ.get("PLANS_PATH", "/etc/xui-sales/plans.json")),
            public_base_url=public_base_url,
            secret_key=(
                Path(_required_env("APP_SECRET_KEY_FILE")).read_text(encoding="utf-8").strip()
                if os.environ.get("APP_SECRET_KEY_FILE")
                else _required_env("APP_SECRET_KEY")
            ),
            payments_enabled=payments_enabled,
            alipay_gateway=alipay_gateway,
            alipay_app_id=alipay_app_id,
            alipay_seller_id=alipay_seller_id,
            alipay_private_key_path=Path(private_path_raw) if private_path_raw else None,
            alipay_public_key_path=Path(public_path_raw) if public_path_raw else None,
            xui_base_url=xui_base_url,
            xui_api_token=(
                Path(_required_env("XUI_API_TOKEN_FILE")).read_text(encoding="utf-8").strip()
                if os.environ.get("XUI_API_TOKEN_FILE")
                else _required_env("XUI_API_TOKEN")
            ),
            xui_inbound_id=int(os.environ.get("XUI_INBOUND_ID", "1")),
            xui_flow=os.environ.get("XUI_FLOW", "xtls-rprx-vision").strip(),
            xui_insecure_local_tls=insecure_local,
            subscription_base_url=_required_env("SUBSCRIPTION_BASE_URL").rstrip("/"),
            redemption_pepper=redemption_pepper,
            xianyu_callbacks_enabled=xianyu_callbacks_enabled,
            xianyu_app_id=xianyu_app_id,
            xianyu_app_secret=xianyu_app_secret,
            xianyu_signature_merchant_id=(
                os.environ.get("XIANYU_SIGNATURE_MERCHANT_ID", "").strip() or None
            ),
            xianyu_products_path=(
                Path(xianyu_products_path_raw) if xianyu_products_path_raw else None
            ),
            xianyu_timestamp_skew_seconds=timestamp_skew,
            sub2api_enabled=sub2api_enabled,
            sub2api_base_url=sub2api_base_url,
            sub2api_admin_api_key=sub2api_admin_api_key,
            sub2api_timeout_seconds=sub2api_timeout_seconds,
        )
