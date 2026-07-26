from __future__ import annotations

import hashlib
import secrets
import sqlite3
import time
from contextlib import contextmanager
from pathlib import Path
from typing import Any, Iterator

from .config import Plan
from .redemption import access_token, code_digest, code_hint, normalize_code
from .xianyu import ProductMapping, XianyuOrderEvent


ORDER_SELECT = """
SELECT o.id, o.access_token_hash, o.plan_id, o.plan_name, o.amount_cents,
       o.duration_days, o.traffic_gb, o.ip_limit, o.status, o.alipay_trade_no,
       o.client_email, o.subscription_url, o.provision_expiry_ms, o.error,
       o.source, o.created_at, o.updated_at,
       f.status AS fulfillment_status, f.api_status, f.vpn_status,
       f.sub2api_user_id, f.sub2api_group_id, f.api_validity_days,
       f.api_redeem_code_id, f.api_error, f.vpn_error,
       f.api_fulfillment_type, f.api_balance_cents, f.api_balance_cny_cents,
       f.api_credited_balance_cents, f.api_exchange_rate,
       f.api_exchange_rate_source, f.api_exchange_rate_updated_at, f.vpn_required
FROM orders AS o
LEFT JOIN order_fulfillments AS f ON f.order_id = o.id
"""

PROVISIONING_LEASE_SECONDS = 120


class OrderStore:
    def __init__(self, path: Path, redemption_pepper: bytes):
        self.path = path
        self.redemption_pepper = redemption_pepper
        path.parent.mkdir(parents=True, exist_ok=True)
        self._init_schema()

    def _connect(self) -> sqlite3.Connection:
        conn = sqlite3.connect(self.path, timeout=5, isolation_level=None)
        conn.row_factory = sqlite3.Row
        conn.execute("PRAGMA foreign_keys=ON")
        conn.execute("PRAGMA busy_timeout=5000")
        return conn

    @contextmanager
    def _connection(self) -> Iterator[sqlite3.Connection]:
        conn = self._connect()
        try:
            yield conn
        finally:
            conn.close()

    @contextmanager
    def _transaction(self, immediate: bool = False) -> Iterator[sqlite3.Connection]:
        conn = self._connect()
        try:
            conn.execute("BEGIN IMMEDIATE" if immediate else "BEGIN")
            yield conn
            conn.execute("COMMIT")
        except Exception:
            conn.execute("ROLLBACK")
            raise
        finally:
            conn.close()

    def _init_schema(self) -> None:
        with self._connection() as conn:
            conn.execute("PRAGMA journal_mode=WAL")
            conn.execute("PRAGMA synchronous=FULL")
            conn.executescript(
                """
                CREATE TABLE IF NOT EXISTS orders (
                    id TEXT PRIMARY KEY,
                    access_token_hash TEXT NOT NULL,
                    plan_id TEXT NOT NULL,
                    plan_name TEXT NOT NULL,
                    amount_cents INTEGER NOT NULL CHECK (amount_cents > 0),
                    duration_days INTEGER NOT NULL CHECK (duration_days > 0),
                    traffic_gb INTEGER NOT NULL CHECK (traffic_gb > 0),
                    ip_limit INTEGER NOT NULL CHECK (ip_limit >= 0),
                    status TEXT NOT NULL CHECK (
                        status IN ('pending', 'paid', 'provisioning', 'active', 'provision_failed')
                    ),
                    alipay_trade_no TEXT,
                    client_email TEXT NOT NULL UNIQUE,
                    subscription_url TEXT,
                    provision_expiry_ms INTEGER,
                    error TEXT,
                    source TEXT NOT NULL DEFAULT 'alipay' CHECK (source IN ('alipay', 'xianyu_redeem')),
                    created_at INTEGER NOT NULL,
                    updated_at INTEGER NOT NULL
                );
                CREATE UNIQUE INDEX IF NOT EXISTS orders_alipay_trade_no
                    ON orders(alipay_trade_no) WHERE alipay_trade_no IS NOT NULL;

                CREATE TABLE IF NOT EXISTS order_fulfillments (
                    order_id TEXT PRIMARY KEY REFERENCES orders(id),
                    status TEXT NOT NULL CHECK (
                        status IN (
                            'api_pending', 'api_active', 'vpn_pending', 'vpn_active',
                            'partial_failed', 'active'
                        )
                    ),
                    api_status TEXT NOT NULL CHECK (
                        api_status IN ('api_not_required', 'api_pending', 'api_active', 'api_failed')
                    ),
                    vpn_status TEXT NOT NULL CHECK (
                        vpn_status IN ('vpn_pending', 'vpn_active', 'vpn_failed')
                    ),
                    sub2api_user_id INTEGER,
                    sub2api_group_id INTEGER,
                    api_validity_days INTEGER,
                    api_redeem_code_id INTEGER,
                    api_error TEXT,
                    vpn_error TEXT,
                    api_lease_until INTEGER,
                    vpn_lease_until INTEGER,
                    created_at INTEGER NOT NULL,
                    updated_at INTEGER NOT NULL,
                    CHECK (
                        (api_status='api_not_required' AND sub2api_user_id IS NULL
                         AND sub2api_group_id IS NULL AND api_validity_days IS NULL)
                        OR
                        (api_status!='api_not_required' AND sub2api_user_id > 0
                         AND sub2api_group_id > 0 AND api_validity_days > 0)
                    )
                );
                CREATE INDEX IF NOT EXISTS order_fulfillments_status
                    ON order_fulfillments(status, updated_at);

                CREATE TABLE IF NOT EXISTS redemption_batches (
                    id TEXT PRIMARY KEY,
                    plan_id TEXT NOT NULL,
                    plan_name TEXT NOT NULL,
                    code_count INTEGER NOT NULL CHECK (code_count > 0),
                    created_at INTEGER NOT NULL
                );

                CREATE TABLE IF NOT EXISTS redemption_codes (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    batch_id TEXT NOT NULL REFERENCES redemption_batches(id),
                    code_hash BLOB NOT NULL UNIQUE,
                    code_hint TEXT NOT NULL,
                    plan_id TEXT NOT NULL,
                    plan_name TEXT NOT NULL,
                    duration_days INTEGER NOT NULL CHECK (duration_days > 0),
                    traffic_gb INTEGER NOT NULL CHECK (traffic_gb > 0),
                    ip_limit INTEGER NOT NULL CHECK (ip_limit >= 0),
                    status TEXT NOT NULL CHECK (
                        status IN ('available', 'redeeming', 'redeemed', 'revoked')
                    ),
                    redeemed_order_id TEXT UNIQUE REFERENCES orders(id),
                    xianyu_order_no TEXT UNIQUE,
                    created_at INTEGER NOT NULL,
                    redeemed_at INTEGER
                );
                CREATE INDEX IF NOT EXISTS redemption_codes_batch
                    ON redemption_codes(batch_id);
                CREATE INDEX IF NOT EXISTS redemption_codes_status
                    ON redemption_codes(status);

                CREATE TABLE IF NOT EXISTS xianyu_orders (
                    order_no TEXT PRIMARY KEY,
                    seller_id INTEGER NOT NULL,
                    user_name TEXT NOT NULL,
                    order_type INTEGER NOT NULL,
                    order_status INTEGER NOT NULL,
                    refund_status INTEGER NOT NULL,
                    modify_time INTEGER NOT NULL,
                    product_id TEXT NOT NULL,
                    item_id TEXT NOT NULL,
                    plan_id TEXT NOT NULL,
                    raw_body_sha256 TEXT NOT NULL,
                    received_at INTEGER NOT NULL,
                    updated_at INTEGER NOT NULL
                );

                CREATE TABLE IF NOT EXISTS xianyu_order_events (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    order_no TEXT NOT NULL,
                    order_status INTEGER NOT NULL,
                    refund_status INTEGER NOT NULL,
                    modify_time INTEGER NOT NULL,
                    raw_body_sha256 TEXT NOT NULL UNIQUE,
                    received_at INTEGER NOT NULL
                );
                CREATE INDEX IF NOT EXISTS xianyu_order_events_order
                    ON xianyu_order_events(order_no, modify_time);
                """
            )
            columns = {row[1] for row in conn.execute("PRAGMA table_info(orders)")}
            if "source" not in columns:
                conn.execute(
                    "ALTER TABLE orders ADD COLUMN source TEXT NOT NULL DEFAULT 'alipay'"
                )
            redemption_columns = {
                row[1] for row in conn.execute("PRAGMA table_info(redemption_codes)")
            }
            for name in ("sub2api_group_id", "api_validity_days"):
                if name not in redemption_columns:
                    conn.execute(f"ALTER TABLE redemption_codes ADD COLUMN {name} INTEGER")
            if "api_fulfillment_type" not in redemption_columns:
                conn.execute(
                    "ALTER TABLE redemption_codes ADD COLUMN api_fulfillment_type "
                    "TEXT NOT NULL DEFAULT 'none'"
                )
            if "api_balance_cents" not in redemption_columns:
                conn.execute("ALTER TABLE redemption_codes ADD COLUMN api_balance_cents INTEGER")
            if "api_balance_cny_cents" not in redemption_columns:
                conn.execute("ALTER TABLE redemption_codes ADD COLUMN api_balance_cny_cents INTEGER")

            fulfillment_columns = {
                row[1] for row in conn.execute("PRAGMA table_info(order_fulfillments)")
            }
            if "api_fulfillment_type" not in fulfillment_columns:
                conn.execute(
                    "ALTER TABLE order_fulfillments ADD COLUMN api_fulfillment_type "
                    "TEXT NOT NULL DEFAULT 'none'"
                )
            if "api_balance_cents" not in fulfillment_columns:
                conn.execute("ALTER TABLE order_fulfillments ADD COLUMN api_balance_cents INTEGER")
            if "api_balance_cny_cents" not in fulfillment_columns:
                conn.execute("ALTER TABLE order_fulfillments ADD COLUMN api_balance_cny_cents INTEGER")
            if "api_credited_balance_cents" not in fulfillment_columns:
                conn.execute("ALTER TABLE order_fulfillments ADD COLUMN api_credited_balance_cents INTEGER")
            if "api_exchange_rate" not in fulfillment_columns:
                conn.execute("ALTER TABLE order_fulfillments ADD COLUMN api_exchange_rate TEXT")
            if "api_exchange_rate_source" not in fulfillment_columns:
                conn.execute("ALTER TABLE order_fulfillments ADD COLUMN api_exchange_rate_source TEXT")
            if "api_exchange_rate_updated_at" not in fulfillment_columns:
                conn.execute("ALTER TABLE order_fulfillments ADD COLUMN api_exchange_rate_updated_at TEXT")
            if "vpn_required" not in fulfillment_columns:
                conn.execute(
                    "ALTER TABLE order_fulfillments ADD COLUMN vpn_required "
                    "INTEGER NOT NULL DEFAULT 1"
                )

    @staticmethod
    def _token_hash(token: str) -> str:
        return hashlib.sha256(token.encode("utf-8")).hexdigest()

    @staticmethod
    def _row(row: sqlite3.Row | None) -> dict[str, Any] | None:
        return dict(row) if row is not None else None

    @staticmethod
    def _insert_fulfillment(
        conn: sqlite3.Connection,
        order_id: str,
        now: int,
        sub2api_user_id: int | None = None,
        sub2api_group_id: int | None = None,
        api_validity_days: int | None = None,
        api_balance_cents: int | None = None,
        api_balance_cny_cents: int | None = None,
        vpn_required: bool = True,
    ) -> None:
        is_bundle = sub2api_group_id is not None
        if is_bundle != (sub2api_user_id is not None and api_validity_days is not None):
            raise ValueError("bundle fulfillment requires complete Sub2API identity and plan data")
        is_balance = api_balance_cents is not None or api_balance_cny_cents is not None
        if is_bundle and is_balance:
            raise ValueError("fulfillment cannot grant a subscription and balance")
        if is_balance and (
            sub2api_user_id is None
            or (api_balance_cents is not None and api_balance_cents <= 0)
            or (api_balance_cny_cents is not None and api_balance_cny_cents <= 0)
            or vpn_required
        ):
            raise ValueError("balance fulfillment requires a user, positive balance, and no VPN")
        api_type = "subscription" if is_bundle else "balance" if is_balance else "none"
        api_required = api_type != "none"
        conn.execute(
            """
            INSERT INTO order_fulfillments (
                order_id, status, api_status, vpn_status, sub2api_user_id,
                sub2api_group_id, api_validity_days, api_fulfillment_type,
                api_balance_cents, api_balance_cny_cents, vpn_required, created_at, updated_at
            ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
            """,
            (
                order_id,
                "api_pending" if api_required else "vpn_pending",
                "api_pending" if api_required else "api_not_required",
                "vpn_pending" if vpn_required else "vpn_active",
                sub2api_user_id,
                sub2api_group_id,
                api_validity_days,
                api_type,
                api_balance_cents,
                api_balance_cny_cents,
                int(vpn_required),
                now,
                now,
            ),
        )

    @staticmethod
    def _ensure_legacy_fulfillment(conn: sqlite3.Connection, order: dict[str, Any]) -> None:
        if order.get("fulfillment_status") is not None:
            return
        now = int(time.time())
        active = order["status"] == "active"
        conn.execute(
            """
            INSERT OR IGNORE INTO order_fulfillments (
                order_id, status, api_status, vpn_status, api_fulfillment_type,
                vpn_required, created_at, updated_at
            ) VALUES (?, ?, 'api_not_required', ?, 'none', 1, ?, ?)
            """,
            (
                order["id"],
                "active" if active else "vpn_pending",
                "vpn_active" if active else "vpn_pending",
                now,
                now,
            ),
        )

    def create_order(self, plan: Plan) -> tuple[dict[str, Any], str]:
        order_id = f"XS{time.strftime('%Y%m%d')}{secrets.token_hex(8).upper()}"
        token = secrets.token_urlsafe(32)
        now = int(time.time())
        email = f"order-{order_id.lower()}@sales.invalid"
        with self._transaction(immediate=True) as conn:
            conn.execute(
                """
                INSERT INTO orders (
                    id, access_token_hash, plan_id, plan_name, amount_cents,
                    duration_days, traffic_gb, ip_limit, status, client_email,
                    created_at, updated_at
                ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'pending', ?, ?, ?)
                """,
                (
                    order_id,
                    self._token_hash(token),
                    plan.id,
                    plan.name,
                    plan.price_cents,
                    plan.duration_days,
                    plan.traffic_gb,
                    plan.ip_limit,
                    email,
                    now,
                    now,
                ),
            )
            self._insert_fulfillment(conn, order_id, now)
            row = conn.execute(ORDER_SELECT + " WHERE o.id=?", (order_id,)).fetchone()
        order = self._row(row)
        assert order is not None, "inserted order could not be read"
        return order, token

    def get(self, order_id: str) -> dict[str, Any] | None:
        with self._connection() as conn:
            return self._row(conn.execute(ORDER_SELECT + " WHERE o.id=?", (order_id,)).fetchone())

    def get_authorized(self, order_id: str, token: str) -> dict[str, Any] | None:
        order = self.get(order_id)
        if order is None:
            return None
        if not secrets.compare_digest(order["access_token_hash"], self._token_hash(token)):
            return None
        return order

    def mark_paid(self, order_id: str, trade_no: str) -> dict[str, Any]:
        now = int(time.time())
        with self._transaction(immediate=True) as conn:
            row = conn.execute(ORDER_SELECT + " WHERE o.id=?", (order_id,)).fetchone()
            if row is None:
                raise KeyError(order_id)
            order = dict(row)
            if order["alipay_trade_no"] not in {None, trade_no}:
                raise ValueError("order is already linked to a different Alipay trade")
            if order["status"] == "pending":
                conn.execute(
                    "UPDATE orders SET status='paid', alipay_trade_no=?, updated_at=? WHERE id=?",
                    (trade_no, now, order_id),
                )
            elif order["alipay_trade_no"] is None:
                conn.execute(
                    "UPDATE orders SET alipay_trade_no=?, updated_at=? WHERE id=?",
                    (trade_no, now, order_id),
                )
            row = conn.execute(ORDER_SELECT + " WHERE o.id=?", (order_id,)).fetchone()
        result = self._row(row)
        assert result is not None, "paid order disappeared"
        return result

    def claim_provisioning(self, order_id: str) -> dict[str, Any] | None:
        now = int(time.time())
        with self._transaction(immediate=True) as conn:
            row = conn.execute(ORDER_SELECT + " WHERE o.id=?", (order_id,)).fetchone()
            if row is None:
                raise KeyError(order_id)
            order = dict(row)
            self._ensure_legacy_fulfillment(conn, order)
            if not order["vpn_required"]:
                raise ValueError(f"order {order_id} does not require VPN provisioning")
            if order["status"] == "active":
                return order
            if order["status"] == "provisioning" and order["updated_at"] > now - PROVISIONING_LEASE_SECONDS:
                return None
            if order["status"] not in {"paid", "provision_failed"}:
                if order["status"] != "provisioning":
                    raise ValueError(f"order {order_id} is not paid")
            expiry_ms = order["provision_expiry_ms"]
            if expiry_ms is None:
                expiry_ms = (now + order["duration_days"] * 86400) * 1000
            conn.execute(
                """
                UPDATE orders
                SET status='provisioning', provision_expiry_ms=?, error=NULL, updated_at=?
                WHERE id=?
                """,
                (expiry_ms, now, order_id),
            )
            conn.execute(
                """
                UPDATE order_fulfillments
                SET vpn_status='vpn_pending', vpn_lease_until=?, vpn_error=NULL,
                    status='vpn_pending', updated_at=?
                WHERE order_id=?
                """,
                (now + PROVISIONING_LEASE_SECONDS, now, order_id),
            )
            row = conn.execute(ORDER_SELECT + " WHERE o.id=?", (order_id,)).fetchone()
        result = self._row(row)
        assert result is not None, "claimed order disappeared"
        return result

    def mark_active(self, order_id: str, subscription_url: str) -> None:
        now = int(time.time())
        with self._transaction(immediate=True) as conn:
            cur = conn.execute(
                """
                UPDATE orders
                SET status='active', subscription_url=?, error=NULL, updated_at=?
                WHERE id=? AND status='provisioning'
                """,
                (subscription_url, now, order_id),
            )
            if cur.rowcount != 1:
                raise RuntimeError(f"order {order_id} left provisioning unexpectedly")
            fulfillment = conn.execute(
                "SELECT api_status FROM order_fulfillments WHERE order_id=?", (order_id,)
            ).fetchone()
            if fulfillment is None:
                raise RuntimeError(f"order {order_id} has no fulfillment state")
            api_done = fulfillment["api_status"] in {"api_not_required", "api_active"}
            next_status = "active" if api_done else "vpn_active"
            if fulfillment["api_status"] == "api_failed":
                next_status = "partial_failed"
            conn.execute(
                """
                UPDATE order_fulfillments
                SET vpn_status='vpn_active', vpn_lease_until=NULL, vpn_error=NULL,
                    status=?, updated_at=?
                WHERE order_id=?
                """,
                (next_status, now, order_id),
            )
            if api_done:
                conn.execute(
                    """
                    UPDATE redemption_codes
                    SET status='redeemed'
                    WHERE redeemed_order_id=? AND status='redeeming'
                    """,
                    (order_id,),
                )

    def mark_provision_failed(self, order_id: str, error: str) -> None:
        now = int(time.time())
        safe_error = error[:500]
        with self._transaction(immediate=True) as conn:
            cur = conn.execute(
                """
                UPDATE orders
                SET status='provision_failed', error=?, updated_at=?
                WHERE id=? AND status='provisioning'
                """,
                (safe_error, now, order_id),
            )
            if cur.rowcount != 1:
                raise RuntimeError(f"order {order_id} left provisioning unexpectedly")
            conn.execute(
                """
                UPDATE order_fulfillments
                SET vpn_status='vpn_failed', vpn_lease_until=NULL, vpn_error=?,
                    status='partial_failed', updated_at=?
                WHERE order_id=?
                """,
                (safe_error, now, order_id),
            )

    def claim_api_fulfillment(self, order_id: str) -> dict[str, Any] | None:
        now = int(time.time())
        with self._transaction(immediate=True) as conn:
            row = conn.execute(ORDER_SELECT + " WHERE o.id=?", (order_id,)).fetchone()
            if row is None:
                raise KeyError(order_id)
            order = dict(row)
            if order["api_status"] in {"api_not_required", "api_active"}:
                return order
            lease = conn.execute(
                "SELECT api_lease_until FROM order_fulfillments WHERE order_id=?", (order_id,)
            ).fetchone()
            if lease is None:
                raise RuntimeError(f"order {order_id} has no fulfillment state")
            if lease["api_lease_until"] is not None and lease["api_lease_until"] > now:
                return None
            conn.execute(
                """
                UPDATE order_fulfillments
                SET api_status='api_pending', api_lease_until=?, api_error=NULL,
                    status='api_pending', updated_at=?
                WHERE order_id=?
                """,
                (now + PROVISIONING_LEASE_SECONDS, now, order_id),
            )
            row = conn.execute(ORDER_SELECT + " WHERE o.id=?", (order_id,)).fetchone()
        result = self._row(row)
        assert result is not None, "claimed API fulfillment disappeared"
        return result

    def freeze_balance_quote(self, order_id: str, quote: dict[str, Any]) -> dict[str, Any]:
        now = int(time.time())
        with self._transaction(immediate=True) as conn:
            row = conn.execute(
                "SELECT api_fulfillment_type, api_balance_cny_cents, "
                "api_credited_balance_cents FROM order_fulfillments WHERE order_id=?",
                (order_id,),
            ).fetchone()
            if row is None or row["api_fulfillment_type"] != "balance":
                raise RuntimeError(f"order {order_id} is not a balance fulfillment")
            if row["api_credited_balance_cents"] is None:
                if int(quote["cny_cents"]) != row["api_balance_cny_cents"]:
                    raise RuntimeError("exchange-rate quote does not match the order amount")
                cur = conn.execute(
                    """
                    UPDATE order_fulfillments
                    SET api_credited_balance_cents=?, api_exchange_rate=?,
                        api_exchange_rate_source=?, api_exchange_rate_updated_at=?, updated_at=?
                    WHERE order_id=? AND api_credited_balance_cents IS NULL
                    """,
                    (
                        int(quote["usd_cents"]),
                        str(quote["usd_cny_rate"]),
                        str(quote["source"]),
                        str(quote["updated_at"]),
                        now,
                        order_id,
                    ),
                )
                if cur.rowcount != 1:
                    raise RuntimeError("balance quote changed concurrently")
            result = conn.execute(ORDER_SELECT + " WHERE o.id=?", (order_id,)).fetchone()
        frozen = self._row(result)
        assert frozen is not None, "quoted balance order disappeared"
        return frozen

    def mark_api_active(self, order_id: str, redeem_code_id: int) -> None:
        now = int(time.time())
        with self._transaction(immediate=True) as conn:
            row = conn.execute(
                "SELECT api_status, vpn_status FROM order_fulfillments WHERE order_id=?",
                (order_id,),
            ).fetchone()
            if row is None or row["api_status"] != "api_pending":
                raise RuntimeError(f"order {order_id} left API fulfillment unexpectedly")
            fully_active = row["vpn_status"] == "vpn_active"
            conn.execute(
                """
                UPDATE order_fulfillments
                SET api_status='api_active', api_redeem_code_id=?, api_lease_until=NULL,
                    api_error=NULL, status=?, updated_at=?
                WHERE order_id=?
                """,
                (redeem_code_id, "active" if fully_active else "api_active", now, order_id),
            )
            if fully_active:
                conn.execute(
                    """
                    UPDATE redemption_codes SET status='redeemed'
                    WHERE redeemed_order_id=? AND status='redeeming'
                    """,
                    (order_id,),
                )

    def mark_api_failed(self, order_id: str, error: str) -> None:
        now = int(time.time())
        with self._transaction(immediate=True) as conn:
            cur = conn.execute(
                """
                UPDATE order_fulfillments
                SET api_status='api_failed', api_lease_until=NULL, api_error=?,
                    status='partial_failed', updated_at=?
                WHERE order_id=? AND api_status='api_pending'
                """,
                (error[:500], now, order_id),
            )
            if cur.rowcount != 1:
                raise RuntimeError(f"order {order_id} left API fulfillment unexpectedly")

    def create_redemption_batch(self, plan: Plan, plaintext_codes: list[str]) -> str:
        if not plaintext_codes:
            raise ValueError("redemption batch must contain at least one code")
        normalized_codes = [normalize_code(code) for code in plaintext_codes]
        if len(set(normalized_codes)) != len(normalized_codes):
            raise ValueError("redemption batch contains duplicate codes")
        batch_id = f"RB{time.strftime('%Y%m%d')}{secrets.token_hex(6).upper()}"
        now = int(time.time())
        with self._transaction(immediate=True) as conn:
            conn.execute(
                """
                INSERT INTO redemption_batches (
                    id, plan_id, plan_name, code_count, created_at
                ) VALUES (?, ?, ?, ?, ?)
                """,
                (batch_id, plan.id, plan.name, len(normalized_codes), now),
            )
            conn.executemany(
                """
                INSERT INTO redemption_codes (
                    batch_id, code_hash, code_hint, plan_id, plan_name,
                    duration_days, traffic_gb, ip_limit, status, created_at,
                    sub2api_group_id, api_validity_days, api_fulfillment_type,
                    api_balance_cents, api_balance_cny_cents
                ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'available', ?, ?, ?, ?, ?, ?)
                """,
                [
                    (
                        batch_id,
                        code_digest(code, self.redemption_pepper),
                        code_hint(code),
                        plan.id,
                        plan.name,
                        plan.duration_days,
                        plan.traffic_gb,
                        plan.ip_limit,
                        now,
                        plan.sub2api_group_id,
                        plan.api_validity_days,
                        (
                            "subscription"
                            if plan.is_bundle
                            else "balance"
                            if plan.is_api_balance
                            else "none"
                        ),
                        plan.sub2api_balance_cents,
                        plan.price_cents if plan.is_api_balance else None,
                    )
                    for code in normalized_codes
                ],
            )
        return batch_id

    def discard_available_batch(self, batch_id: str) -> None:
        with self._transaction(immediate=True) as conn:
            counts = conn.execute(
                """
                SELECT COUNT(*) AS total,
                       SUM(CASE WHEN status='available' THEN 1 ELSE 0 END) AS available
                FROM redemption_codes WHERE batch_id=?
                """,
                (batch_id,),
            ).fetchone()
            if counts is None or counts["total"] == 0:
                raise KeyError(batch_id)
            if counts["total"] != counts["available"]:
                raise RuntimeError("cannot discard a batch after redemption has started")
            conn.execute("DELETE FROM redemption_codes WHERE batch_id=?", (batch_id,))
            conn.execute("DELETE FROM redemption_batches WHERE id=?", (batch_id,))

    def redemption_requirements(self, raw_code: str) -> dict[str, Any]:
        normalized = normalize_code(raw_code)
        digest = code_digest(normalized, self.redemption_pepper)
        with self._connection() as conn:
            row = conn.execute(
                """
                SELECT sub2api_group_id, api_validity_days, api_fulfillment_type,
                       api_balance_cents, api_balance_cny_cents
                FROM redemption_codes WHERE code_hash=?
                """,
                (digest,),
            ).fetchone()
        if row is None:
            raise KeyError("unknown redemption code")
        return dict(row)

    def redeem_code(
        self, raw_code: str, sub2api_user_id: int | None = None
    ) -> tuple[dict[str, Any], str]:
        normalized = normalize_code(raw_code)
        token = access_token(normalized, self.redemption_pepper)
        digest = code_digest(normalized, self.redemption_pepper)
        now = int(time.time())

        with self._transaction(immediate=True) as conn:
            code_row = conn.execute(
                "SELECT * FROM redemption_codes WHERE code_hash=?", (digest,)
            ).fetchone()
            if code_row is None:
                raise KeyError("unknown redemption code")
            code = dict(code_row)
            if code["status"] == "revoked":
                raise ValueError("兑换码已停用")
            api_type = code["api_fulfillment_type"]
            api_required = api_type in {"subscription", "balance"}
            if api_required and (sub2api_user_id is None or sub2api_user_id <= 0):
                raise ValueError("该套餐需要有效的 Sub2API 账户")
            if not api_required and sub2api_user_id is not None:
                raise ValueError("VPN 套餐不能绑定 Sub2API 账户")
            if code["redeemed_order_id"] is not None:
                row = conn.execute(
                    ORDER_SELECT + " WHERE o.id=?", (code["redeemed_order_id"],)
                ).fetchone()
                result = self._row(row)
                assert result is not None, "redeemed code references a missing order"
                if api_required and result["sub2api_user_id"] != sub2api_user_id:
                    raise ValueError("兑换码已绑定其他 Sub2API 账户")
                return result, token

            order_id = f"XR{time.strftime('%Y%m%d')}{secrets.token_hex(8).upper()}"
            email = f"order-{order_id.lower()}@sales.invalid"
            conn.execute(
                """
                INSERT INTO orders (
                    id, access_token_hash, plan_id, plan_name, amount_cents,
                    duration_days, traffic_gb, ip_limit, status, client_email, source,
                    created_at, updated_at
                ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'paid', ?, 'xianyu_redeem', ?, ?)
                """,
                (
                    order_id,
                    self._token_hash(token),
                    code["plan_id"],
                    code["plan_name"],
                    code.get("api_balance_cny_cents") or 1,
                    code["duration_days"],
                    code["traffic_gb"],
                    code["ip_limit"],
                    email,
                    now,
                    now,
                ),
            )
            self._insert_fulfillment(
                conn,
                order_id,
                now,
                sub2api_user_id=sub2api_user_id,
                sub2api_group_id=code["sub2api_group_id"],
                api_validity_days=code["api_validity_days"],
                api_balance_cents=code["api_balance_cents"],
                api_balance_cny_cents=code.get("api_balance_cny_cents"),
                vpn_required=api_type != "balance",
            )
            cur = conn.execute(
                """
                UPDATE redemption_codes
                SET status='redeeming', redeemed_order_id=?, redeemed_at=?
                WHERE id=? AND status='available'
                """,
                (order_id, now, code["id"]),
            )
            if cur.rowcount != 1:
                raise RuntimeError("redemption code changed during redemption")
            row = conn.execute(ORDER_SELECT + " WHERE o.id=?", (order_id,)).fetchone()
        result = self._row(row)
        assert result is not None, "redeemed order could not be read"
        return result, token

    def record_xianyu_event(
        self,
        event: XianyuOrderEvent,
        mapping: ProductMapping,
        raw_body_sha256: str,
    ) -> None:
        now = int(time.time())
        with self._transaction(immediate=True) as conn:
            conn.execute(
                """
                INSERT OR IGNORE INTO xianyu_order_events (
                    order_no, order_status, refund_status, modify_time,
                    raw_body_sha256, received_at
                ) VALUES (?, ?, ?, ?, ?, ?)
                """,
                (
                    event.order_no,
                    event.order_status,
                    event.refund_status,
                    event.modify_time,
                    raw_body_sha256,
                    now,
                ),
            )
            conn.execute(
                """
                INSERT INTO xianyu_orders (
                    order_no, seller_id, user_name, order_type, order_status,
                    refund_status, modify_time, product_id, item_id, plan_id,
                    raw_body_sha256, received_at, updated_at
                ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                ON CONFLICT(order_no) DO UPDATE SET
                    seller_id=excluded.seller_id,
                    user_name=excluded.user_name,
                    order_type=excluded.order_type,
                    order_status=excluded.order_status,
                    refund_status=excluded.refund_status,
                    modify_time=excluded.modify_time,
                    product_id=excluded.product_id,
                    item_id=excluded.item_id,
                    plan_id=excluded.plan_id,
                    raw_body_sha256=excluded.raw_body_sha256,
                    updated_at=excluded.updated_at
                WHERE excluded.modify_time >= xianyu_orders.modify_time
                """,
                (
                    event.order_no,
                    event.seller_id,
                    event.user_name,
                    event.order_type,
                    event.order_status,
                    event.refund_status,
                    event.modify_time,
                    event.product_id,
                    event.item_id,
                    mapping.plan_id,
                    raw_body_sha256,
                    now,
                    now,
                ),
            )

    def retryable_order_ids(self, minimum_age_seconds: int = 60, limit: int = 20) -> list[str]:
        cutoff = int(time.time()) - minimum_age_seconds
        with self._connection() as conn:
            rows = conn.execute(
                """
                SELECT o.id FROM orders AS o
                LEFT JOIN order_fulfillments AS f ON f.order_id=o.id
                WHERE (
                    (f.order_id IS NOT NULL AND f.status!='active' AND f.updated_at <= ?)
                    OR
                    (f.order_id IS NULL AND (
                        o.status IN ('paid', 'provision_failed')
                        OR (o.status='provisioning' AND o.updated_at <= ?)
                    ) AND o.updated_at <= ?)
                )
                ORDER BY COALESCE(f.updated_at, o.updated_at) ASC
                LIMIT ?
                """,
                (cutoff, int(time.time()) - PROVISIONING_LEASE_SECONDS, cutoff, limit),
            ).fetchall()
        return [str(row["id"]) for row in rows]

    def inventory_counts(self) -> list[dict[str, Any]]:
        with self._connection() as conn:
            rows = conn.execute(
                """
                SELECT batch_id, plan_id, status, COUNT(*) AS count
                FROM redemption_codes
                GROUP BY batch_id, plan_id, status
                ORDER BY batch_id, status
                """
            ).fetchall()
        return [dict(row) for row in rows]
