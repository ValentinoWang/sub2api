-- Browser transport reuses the existing entitlement ledger and durable batches.
CREATE TABLE liandong_browser_config (
    id BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (id),
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    products JSONB NOT NULL DEFAULT '[]'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE liandong_browser_devices (
    id VARCHAR(32) PRIMARY KEY,
    name VARCHAR(80) NOT NULL,
    goods_ids JSONB NOT NULL,
    key_sha256 VARCHAR(64) NOT NULL UNIQUE,
    revoked BOOLEAN NOT NULL DEFAULT FALSE,
    paused_reason TEXT NOT NULL DEFAULT '',
    last_seen_at TIMESTAMPTZ,
    authorization_verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE liandong_browser_inventory (
    goods_id BIGINT PRIMARY KEY,
    device_id VARCHAR(32) NOT NULL REFERENCES liandong_browser_devices(id),
    identity_verified BOOLEAN NOT NULL,
    code_hashes JSONB NOT NULL DEFAULT '[]'::jsonb,
    observed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
ALTER TABLE liandong_restock_batches ADD COLUMN browser_device_id VARCHAR(32) REFERENCES liandong_browser_devices(id);
ALTER TABLE liandong_restock_batch_codes ADD COLUMN redeem_code_id BIGINT REFERENCES redeem_codes(id);
CREATE UNIQUE INDEX liandong_browser_one_open_batch ON liandong_restock_batches(goods_id)
    WHERE browser_device_id IS NOT NULL AND status IN ('pending', 'needs_reconciliation');
