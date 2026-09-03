from __future__ import annotations

import base64
import json
from datetime import datetime
from pathlib import Path
from typing import Mapping

from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import padding


def canonical_content(params: Mapping[str, str]) -> bytes:
    return "&".join(
        f"{key}={value}"
        for key, value in sorted(params.items())
        if key not in {"sign", "sign_type"} and value != ""
    ).encode("utf-8")


class AlipayClient:
    def __init__(
        self,
        app_id: str,
        gateway: str,
        merchant_private_key_file: Path,
        alipay_public_key_file: Path,
        notify_url: str,
        return_url: str,
    ):
        self.app_id = app_id
        self.gateway = gateway
        self.notify_url = notify_url
        self.return_url = return_url
        self._private_key = serialization.load_pem_private_key(
            merchant_private_key_file.read_bytes(), password=None
        )
        self._alipay_public_key = serialization.load_pem_public_key(
            alipay_public_key_file.read_bytes()
        )

    def sign(self, params: Mapping[str, str]) -> str:
        signature = self._private_key.sign(
            canonical_content(params), padding.PKCS1v15(), hashes.SHA256()
        )
        return base64.b64encode(signature).decode("ascii")

    def verify_notification(self, params: Mapping[str, str]) -> bool:
        signature_text = params.get("sign", "")
        if not signature_text:
            return False
        try:
            signature = base64.b64decode(signature_text, validate=True)
            self._alipay_public_key.verify(
                signature,
                canonical_content(params),
                padding.PKCS1v15(),
                hashes.SHA256(),
            )
            return True
        except (ValueError, TypeError):
            return False
        except Exception:
            return False

    def page_pay_parameters(
        self, order_id: str, amount_text: str, subject: str, return_url: str | None = None
    ) -> dict[str, str]:
        biz_content = json.dumps(
            {
                "out_trade_no": order_id,
                "product_code": "FAST_INSTANT_TRADE_PAY",
                "subject": subject,
                "total_amount": amount_text,
            },
            ensure_ascii=False,
            separators=(",", ":"),
        )
        params = {
            "app_id": self.app_id,
            "method": "alipay.trade.page.pay",
            "format": "JSON",
            "charset": "utf-8",
            "sign_type": "RSA2",
            "timestamp": datetime.now().strftime("%Y-%m-%d %H:%M:%S"),
            "version": "1.0",
            "notify_url": self.notify_url,
            "return_url": return_url or self.return_url,
            "biz_content": biz_content,
        }
        params["sign"] = self.sign(params)
        return params
