from __future__ import annotations

import base64
import hashlib
import hmac
import re
import secrets


CODE_PATTERN = re.compile(r"LA[A-Z2-7]{26}")


class RedemptionError(ValueError):
    pass


def normalize_code(raw: str) -> str:
    normalized = "".join(character for character in raw.upper() if character.isalnum())
    if CODE_PATTERN.fullmatch(normalized) is None:
        raise RedemptionError("兑换码格式不正确")
    return normalized


def generate_code() -> str:
    payload = base64.b32encode(secrets.token_bytes(16)).decode("ascii").rstrip("=")
    normalized = "LA" + payload
    assert CODE_PATTERN.fullmatch(normalized) is not None
    return "-".join((normalized[:2], normalized[2:8], normalized[8:14], normalized[14:20], normalized[20:]))


def code_digest(normalized_code: str, pepper: bytes) -> bytes:
    return hmac.digest(pepper, b"code:" + normalized_code.encode("ascii"), "sha256")


def access_token(normalized_code: str, pepper: bytes) -> str:
    digest = hmac.digest(pepper, b"access:" + normalized_code.encode("ascii"), "sha256")
    return base64.urlsafe_b64encode(digest).decode("ascii").rstrip("=")


def code_hint(normalized_code: str) -> str:
    return normalized_code[-6:]
