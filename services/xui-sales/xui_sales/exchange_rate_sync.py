from __future__ import annotations

import json
import os
import urllib.error
import urllib.request
from datetime import datetime, timezone
from decimal import Decimal, InvalidOperation, ROUND_HALF_UP
from typing import Any


DEFAULT_SOURCES = (
    ("frankfurter", "https://api.frankfurter.app/latest?from=USD&to=CNY"),
    ("open-er-api", "https://open.er-api.com/v6/latest/USD"),
)


class ExchangeRateSyncError(RuntimeError):
    pass


def _extract_usd_cny(payload: Any) -> Decimal:
    if not isinstance(payload, dict):
        raise ExchangeRateSyncError("exchange-rate response must be an object")
    rates = payload.get("rates")
    if not isinstance(rates, dict):
        raise ExchangeRateSyncError("exchange-rate response has no rates object")
    try:
        rate = Decimal(str(rates["CNY"]))
    except (KeyError, InvalidOperation) as exc:
        raise ExchangeRateSyncError("exchange-rate response has no valid CNY rate") from exc
    if rate < Decimal("4") or rate > Decimal("10"):
        raise ExchangeRateSyncError("USD/CNY rate is outside the safety range")
    return rate


def fetch_usd_cny(timeout_seconds: int = 8) -> tuple[Decimal, str]:
    errors: list[str] = []
    for source, url in DEFAULT_SOURCES:
        request = urllib.request.Request(url, headers={"Accept": "application/json"})
        try:
            with urllib.request.urlopen(request, timeout=timeout_seconds) as response:
                return _extract_usd_cny(json.load(response)), source
        except (urllib.error.URLError, TimeoutError, json.JSONDecodeError, ExchangeRateSyncError) as exc:
            errors.append(f"{source}: {type(exc).__name__}")
    raise ExchangeRateSyncError("all exchange-rate sources failed: " + ", ".join(errors))


def update_sub2api(
    base_url: str,
    admin_api_key: str,
    usd_cny_rate: Decimal,
    source: str,
    timeout_seconds: int = 8,
) -> dict[str, Any]:
    if not base_url.startswith(("http://", "https://")):
        raise ExchangeRateSyncError("SUB2API_BASE_URL must be an absolute HTTP(S) URL")
    if not admin_api_key:
        raise ExchangeRateSyncError("SUB2API_ADMIN_API_KEY is required")

    multiplier = (Decimal("1") / usd_cny_rate).quantize(
        Decimal("0.00000001"), rounding=ROUND_HALF_UP
    )
    updated_at = datetime.now(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")
    payload = {
        "balance_recharge_multiplier": float(multiplier),
        "subscription_usd_to_cny_rate": float(usd_cny_rate),
        "balance_exchange_rate_usd_to_cny": float(usd_cny_rate),
        "balance_exchange_rate_source": source,
        "balance_exchange_rate_updated_at": updated_at,
    }
    request = urllib.request.Request(
        base_url.rstrip("/") + "/api/v1/admin/payment/config",
        data=json.dumps(payload, separators=(",", ":")).encode("utf-8"),
        headers={
            "Accept": "application/json",
            "Content-Type": "application/json",
            "x-api-key": admin_api_key,
        },
        method="PUT",
    )
    try:
        with urllib.request.urlopen(request, timeout=timeout_seconds) as response:
            result = json.load(response)
    except (urllib.error.URLError, TimeoutError, json.JSONDecodeError) as exc:
        raise ExchangeRateSyncError(f"Sub2API update failed: {type(exc).__name__}") from exc
    if not isinstance(result, dict) or result.get("code") not in {0, "0"}:
        raise ExchangeRateSyncError("Sub2API rejected the exchange-rate update")
    return payload


def _read_admin_api_key() -> str:
    key = os.environ.get("SUB2API_ADMIN_API_KEY", "").strip()
    if key:
        return key
    key_file = os.environ.get("SUB2API_ADMIN_API_KEY_FILE", "").strip()
    if not key_file:
        return ""
    try:
        with open(key_file, encoding="utf-8") as handle:
            return handle.read().strip()
    except OSError as exc:
        raise ExchangeRateSyncError("cannot read SUB2API_ADMIN_API_KEY_FILE") from exc


def main() -> None:
    timeout = int(os.environ.get("EXCHANGE_RATE_TIMEOUT_SECONDS", "8"))
    rate, source = fetch_usd_cny(timeout)
    payload = update_sub2api(
        os.environ.get("SUB2API_BASE_URL", "").strip(),
        _read_admin_api_key(),
        rate,
        source,
        timeout,
    )
    print(json.dumps(payload, ensure_ascii=True, separators=(",", ":")))


if __name__ == "__main__":
    main()
