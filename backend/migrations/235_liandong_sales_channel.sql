CREATE TABLE IF NOT EXISTS liandong_product_mappings (
    id BIGSERIAL PRIMARY KEY,
    mapping_key VARCHAR(128) NOT NULL UNIQUE,
    goods_id BIGINT NOT NULL,
    cny_amount DECIMAL(20,2) NOT NULL CHECK (cny_amount > 0),
    grant_type VARCHAR(20) NOT NULL DEFAULT 'balance' CHECK (grant_type IN ('balance', 'subscription')),
    grant_value DECIMAL(20,8) NOT NULL CHECK (grant_value > 0),
    group_id BIGINT,
    validity_days INT,
    external_url TEXT NOT NULL DEFAULT '',
    version INT NOT NULL DEFAULT 1 CHECK (version > 0),
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    target_stock INT NOT NULL DEFAULT 50000 CHECK (target_stock > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK ((grant_type = 'balance' AND group_id IS NULL AND validity_days IS NULL)
        OR (grant_type = 'subscription' AND group_id IS NOT NULL AND validity_days IS NOT NULL AND validity_days > 0))
);
CREATE INDEX IF NOT EXISTS idx_liandong_product_mappings_goods
    ON liandong_product_mappings(goods_id, enabled);
CREATE UNIQUE INDEX IF NOT EXISTS uq_liandong_product_mappings_active_goods
    ON liandong_product_mappings(goods_id) WHERE enabled;

CREATE TABLE IF NOT EXISTS liandong_restock_jobs (
    job_id VARCHAR(64) PRIMARY KEY,
    status VARCHAR(32) NOT NULL CHECK (status IN ('queued', 'running', 'completed', 'failed', 'needs_reconciliation')),
    selected_goods JSONB NOT NULL DEFAULT '[]'::jsonb,
    summary JSONB NOT NULL DEFAULT '{}'::jsonb,
    error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_liandong_restock_jobs_status
    ON liandong_restock_jobs(status, updated_at);

CREATE TABLE IF NOT EXISTS liandong_restock_batches (
    batch_id VARCHAR(64) PRIMARY KEY,
    job_id VARCHAR(64),
    goods_id BIGINT NOT NULL,
    cny_amount DECIMAL(20,2) NOT NULL,
    grant_value DECIMAL(20,8) NOT NULL,
    code_count INT NOT NULL CHECK (code_count > 0),
    code_sha256 VARCHAR(64) NOT NULL,
    status VARCHAR(20) NOT NULL CHECK (status IN ('pending', 'uploaded', 'failed')),
    remote_stock_before INT,
    remote_stock_after INT,
    error TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    uploaded_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    mapping_key VARCHAR(128),
    mapping_version INT NOT NULL DEFAULT 1,
    grant_type VARCHAR(20) NOT NULL DEFAULT 'balance',
    external_url TEXT NOT NULL DEFAULT '',
    target_stock INT NOT NULL DEFAULT 50000,
    planned_count INT NOT NULL DEFAULT 0,
    mapping_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb
);
CREATE INDEX IF NOT EXISTS idx_liandong_restock_batches_status
    ON liandong_restock_batches(status, updated_at);
CREATE INDEX IF NOT EXISTS idx_liandong_restock_batches_job
    ON liandong_restock_batches(job_id, created_at);

CREATE TABLE IF NOT EXISTS liandong_restock_batch_codes (
    batch_id VARCHAR(64) NOT NULL REFERENCES liandong_restock_batches(batch_id) ON DELETE CASCADE,
    code_sha256 VARCHAR(64) NOT NULL,
    code_hint VARCHAR(32) NOT NULL,
    ordinal INT NOT NULL CHECK (ordinal >= 0),
    PRIMARY KEY (batch_id, ordinal),
    UNIQUE (batch_id, code_sha256)
);

CREATE TABLE IF NOT EXISTS liandong_restock_segments (
    batch_id VARCHAR(64) NOT NULL REFERENCES liandong_restock_batches(batch_id) ON DELETE CASCADE,
    segment_no INT NOT NULL CHECK (segment_no >= 0),
    ordinal_start INT NOT NULL CHECK (ordinal_start >= 0),
    code_count INT NOT NULL CHECK (code_count > 0 AND code_count <= 1000),
    code_sha256 VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL CHECK (status IN ('pending', 'codes_created', 'uploaded', 'failed', 'needs_reconciliation')),
    remote_acknowledged BOOLEAN NOT NULL DEFAULT FALSE,
    error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    uploaded_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (batch_id, segment_no)
);
CREATE INDEX IF NOT EXISTS idx_liandong_restock_segments_status
    ON liandong_restock_segments(status, updated_at);

-- The first version of this migration created the tables above without the
-- target/job/segment additions. Keep existing rows and add only missing
-- columns/constraints when that version has already run in a deployment.
ALTER TABLE liandong_product_mappings
    ADD COLUMN IF NOT EXISTS target_stock INT NOT NULL DEFAULT 50000;

ALTER TABLE liandong_restock_batches
    ADD COLUMN IF NOT EXISTS job_id VARCHAR(64),
    ADD COLUMN IF NOT EXISTS mapping_key VARCHAR(128),
    ADD COLUMN IF NOT EXISTS mapping_version INT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS grant_type VARCHAR(20) NOT NULL DEFAULT 'balance',
    ADD COLUMN IF NOT EXISTS external_url TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS target_stock INT NOT NULL DEFAULT 50000,
    ADD COLUMN IF NOT EXISTS planned_count INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS mapping_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE liandong_restock_batches
    ALTER COLUMN status TYPE VARCHAR(32);
ALTER TABLE liandong_restock_batches
    DROP CONSTRAINT IF EXISTS liandong_restock_batches_status_check;
ALTER TABLE liandong_restock_batches
    ADD CONSTRAINT liandong_restock_batches_status_check
    CHECK (status IN ('pending', 'uploaded', 'failed', 'needs_reconciliation'));

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'liandong_restock_batches_job_id_fkey'
          AND conrelid = 'liandong_restock_batches'::regclass
    ) THEN
        ALTER TABLE liandong_restock_batches
            ADD CONSTRAINT liandong_restock_batches_job_id_fkey
            FOREIGN KEY (job_id) REFERENCES liandong_restock_jobs(job_id) ON DELETE SET NULL;
    END IF;
END
$$;
