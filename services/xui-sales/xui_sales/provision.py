from __future__ import annotations

import json
import ssl
import urllib.error
import urllib.parse
import urllib.request
from dataclasses import dataclass
from typing import Any


class ProvisioningError(RuntimeError):
    pass


@dataclass(frozen=True)
class XUIProvisioner:
    base_url: str
    api_token: str
    inbound_id: int
    flow: str
    subscription_base_url: str
    insecure_local_tls: bool = False
    timeout_seconds: int = 8

    def _context(self) -> ssl.SSLContext:
        if self.insecure_local_tls:
            return ssl._create_unverified_context()
        return ssl.create_default_context()

    def _request(self, method: str, path: str, payload: dict[str, Any] | None = None) -> dict[str, Any]:
        body = None
        headers = {
            "Authorization": f"Bearer {self.api_token}",
            "Accept": "application/json",
        }
        if payload is not None:
            body = json.dumps(payload, separators=(",", ":")).encode("utf-8")
            headers["Content-Type"] = "application/json"
        req = urllib.request.Request(
            self.base_url + path, data=body, headers=headers, method=method
        )
        try:
            with urllib.request.urlopen(
                req, context=self._context(), timeout=self.timeout_seconds
            ) as response:
                result = json.load(response)
        except (urllib.error.URLError, TimeoutError, json.JSONDecodeError) as exc:
            raise ProvisioningError(f"x-ui request failed: {type(exc).__name__}") from exc
        if not isinstance(result, dict) or result.get("success") is not True:
            message = result.get("msg", "unknown response") if isinstance(result, dict) else "invalid response"
            raise ProvisioningError(f"x-ui rejected request: {message}")
        return result

    def _find_existing(self, email: str) -> dict[str, Any] | None:
        path = "/panel/api/clients/get/" + urllib.parse.quote(email, safe="")
        try:
            result = self._request("GET", path)
        except ProvisioningError as exc:
            if "not found" in str(exc).lower():
                return None
            raise
        obj = result.get("obj")
        if not isinstance(obj, dict) or not isinstance(obj.get("client"), dict):
            raise ProvisioningError("x-ui returned an invalid client object")
        return obj

    def healthcheck(self) -> None:
        self._request("GET", "/panel/api/server/status")

    def provision(self, order: dict[str, Any]) -> str:
        email = order["client_email"]
        comment = f"xui-sales order {order['id']}"
        existing = self._find_existing(email)
        if existing is None:
            payload = {
                "client": {
                    "email": email,
                    "flow": self.flow,
                    "totalGB": int(order["traffic_gb"]) * 1024 * 1024 * 1024,
                    "expiryTime": int(order["provision_expiry_ms"]),
                    "limitIp": int(order["ip_limit"]),
                    "enable": True,
                    "comment": comment,
                },
                "inboundIds": [self.inbound_id],
            }
            try:
                self._request("POST", "/panel/api/clients/add", payload)
            except ProvisioningError:
                # A lost response after a successful create is resolved by the read below.
                pass
            existing = self._find_existing(email)
        client = existing["client"]
        inbound_ids = existing.get("inboundIds") or []
        if client.get("comment") != comment or self.inbound_id not in inbound_ids:
            raise ProvisioningError("existing x-ui client does not match this order")
        sub_id = str(client.get("subId", "")).strip()
        if not sub_id:
            raise ProvisioningError("x-ui client has no subscription id")
        return f"{self.subscription_base_url}/sub/{urllib.parse.quote(sub_id, safe='')}"
