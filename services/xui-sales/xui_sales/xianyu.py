from __future__ import annotations

import hashlib
import hmac
import json
import time
from dataclasses import dataclass
from pathlib import Path
from typing import Any


class XianyuCallbackError(ValueError):
    pass


@dataclass(frozen=True)
class ProductMapping:
    product_id: str
    item_id: str
    plan_id: str


@dataclass(frozen=True)
class XianyuOrderEvent:
    seller_id: int
    user_name: str
    order_no: str
    order_type: int
    order_status: int
    refund_status: int
    modify_time: int
    product_id: str
    item_id: str

    @classmethod
    def from_payload(cls, payload: Any) -> "XianyuOrderEvent":
        if not isinstance(payload, dict):
            raise XianyuCallbackError("request body must be a JSON object")
        integer_fields = (
            "seller_id",
            "order_type",
            "order_status",
            "refund_status",
            "modify_time",
        )
        values: dict[str, Any] = {}
        for field in integer_fields:
            value = payload.get(field)
            if isinstance(value, bool) or not isinstance(value, int):
                raise XianyuCallbackError(f"{field} must be an integer")
            values[field] = value
        for field in ("user_name", "order_no"):
            value = payload.get(field)
            if not isinstance(value, str) or not value.strip():
                raise XianyuCallbackError(f"{field} must be a non-empty string")
            values[field] = value.strip()
        for field in ("product_id", "item_id"):
            value = payload.get(field)
            if isinstance(value, bool) or not isinstance(value, (str, int)):
                raise XianyuCallbackError(f"{field} must be a string or integer")
            values[field] = str(value).strip()
            if not values[field]:
                raise XianyuCallbackError(f"{field} must not be empty")
        if values["seller_id"] <= 0:
            raise XianyuCallbackError("seller_id must be positive")
        if values["order_type"] not in {1, 2, 3, 4, 7, 8, 9, 10}:
            raise XianyuCallbackError("unsupported order_type")
        if values["order_status"] not in {11, 12, 21, 22, 23, 24}:
            raise XianyuCallbackError("unsupported order_status")
        if values["refund_status"] not in {0, 1, 2, 3, 4, 5, 6, 8}:
            raise XianyuCallbackError("unsupported refund_status")
        if values["modify_time"] <= 0:
            raise XianyuCallbackError("modify_time must be positive")
        if not values["order_no"].isdigit() or not 8 <= len(values["order_no"]) <= 40:
            raise XianyuCallbackError("invalid order_no")
        if len(values["user_name"]) > 200:
            raise XianyuCallbackError("user_name is too long")
        return cls(**values)


def load_product_mappings(path: Path, plan_ids: set[str]) -> dict[tuple[str, str], ProductMapping]:
    try:
        raw = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise RuntimeError(f"cannot load Xianyu product mappings from {path}: {exc}") from exc
    if not isinstance(raw, list):
        raise RuntimeError("Xianyu product mappings must be a JSON array")
    if not raw:
        raise RuntimeError("Xianyu product mappings must not be empty when callbacks are enabled")
    mappings: dict[tuple[str, str], ProductMapping] = {}
    for index, item in enumerate(raw):
        if not isinstance(item, dict):
            raise RuntimeError(f"Xianyu mapping {index} must be an object")
        try:
            mapping = ProductMapping(
                product_id=str(item["product_id"]).strip(),
                item_id=str(item["item_id"]).strip(),
                plan_id=str(item["plan_id"]).strip(),
            )
        except KeyError as exc:
            raise RuntimeError(f"Xianyu mapping {index} is missing {exc.args[0]}") from exc
        if not mapping.product_id or not mapping.item_id:
            raise RuntimeError(f"Xianyu mapping {index} has an empty product or item ID")
        if mapping.plan_id not in plan_ids:
            raise RuntimeError(f"Xianyu mapping {index} references unknown plan {mapping.plan_id}")
        key = (mapping.product_id, mapping.item_id)
        if key in mappings:
            raise RuntimeError(f"duplicate Xianyu mapping for {key}")
        mappings[key] = mapping
    return mappings


def calculate_signature(
    raw_body: bytes,
    app_id: str,
    timestamp: int,
    app_secret: str,
    merchant_id: str | None = None,
) -> str:
    body_md5 = hashlib.md5(raw_body).hexdigest()  # noqa: S324 - provider contract
    parts = [app_id, body_md5, str(timestamp)]
    if merchant_id is not None:
        parts.append(merchant_id)
    parts.append(app_secret)
    return hashlib.md5(",".join(parts).encode("utf-8")).hexdigest()  # noqa: S324


def verify_signature(
    raw_body: bytes,
    app_id: str,
    timestamp_raw: str,
    signature: str,
    expected_app_id: str,
    app_secret: str,
    max_skew_seconds: int,
    merchant_id: str | None = None,
    now: int | None = None,
) -> None:
    if not hmac.compare_digest(app_id, expected_app_id):
        raise XianyuCallbackError("unexpected app identifier")
    try:
        timestamp = int(timestamp_raw)
    except ValueError as exc:
        raise XianyuCallbackError("invalid timestamp") from exc
    current_time = int(time.time()) if now is None else now
    if abs(current_time - timestamp) > max_skew_seconds:
        raise XianyuCallbackError("stale timestamp")
    expected = calculate_signature(raw_body, app_id, timestamp, app_secret, merchant_id)
    if not hmac.compare_digest(signature.lower(), expected):
        raise XianyuCallbackError("invalid signature")
