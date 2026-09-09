-- Consolidated rest2build product schema.
--
-- This migration supports both new databases and deployments that already
-- applied the former 231-237 product migrations. Keep every statement
-- idempotent because existing production databases will execute this file once.

CREATE TABLE IF NOT EXISTS codex_continuity_threads (
    id BIGSERIAL PRIMARY KEY,
    continuity_id VARCHAR(64) NOT NULL UNIQUE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    api_key_id BIGINT NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    session_hash VARCHAR(64) NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'active',
    version BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT chk_codex_continuity_threads_status CHECK (status IN ('active', 'deleted')),
    CONSTRAINT chk_codex_continuity_threads_version CHECK (version >= 0)
);

CREATE TABLE IF NOT EXISTS codex_continuity_turns (
    id BIGSERIAL PRIMARY KEY,
    thread_id BIGINT NOT NULL REFERENCES codex_continuity_threads(id) ON DELETE CASCADE,
    sequence BIGINT NOT NULL,
    request_id VARCHAR(128) NOT NULL,
    state VARCHAR(16) NOT NULL DEFAULT 'completed',
    replay_input_encrypted TEXT NOT NULL,
    replay_sha256 VARCHAR(64) NOT NULL,
    replay_bytes BIGINT NOT NULL,
    upstream_account_id BIGINT REFERENCES accounts(id) ON DELETE SET NULL,
    upstream_response_id VARCHAR(128) NOT NULL DEFAULT '',
    client_window_id VARCHAR(128) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    committed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_codex_continuity_turn_sequence UNIQUE (thread_id, sequence),
    CONSTRAINT uq_codex_continuity_turn_request UNIQUE (thread_id, request_id),
    CONSTRAINT chk_codex_continuity_turns_state CHECK (state IN ('completed', 'failed', 'aborted')),
    CONSTRAINT chk_codex_continuity_turns_size CHECK (replay_bytes >= 0)
);

ALTER TABLE codex_continuity_turns
    ADD COLUMN IF NOT EXISTS client_window_id VARCHAR(128) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_codex_continuity_threads_owner
    ON codex_continuity_threads(user_id, api_key_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_codex_continuity_threads_expiry
    ON codex_continuity_threads(expires_at);
CREATE INDEX IF NOT EXISTS idx_codex_continuity_turns_latest
    ON codex_continuity_turns(thread_id, sequence DESC) WHERE state = 'completed';

CREATE TABLE IF NOT EXISTS user_lifecycle_emails (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event VARCHAR(64) NOT NULL,
    sent_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, event)
);

CREATE INDEX IF NOT EXISTS idx_user_lifecycle_emails_event_sent
    ON user_lifecycle_emails(event, sent_at DESC);

CREATE TABLE IF NOT EXISTS user_first_topup_bonus (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    order_id BIGINT NOT NULL,
    bonus_amount DOUBLE PRECISION NOT NULL,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS liandong_product_mappings (
    id BIGSERIAL PRIMARY KEY,
    mapping_key VARCHAR(128) NOT NULL UNIQUE,
    goods_id BIGINT NOT NULL,
    cny_amount DECIMAL(20,2) NOT NULL CHECK (cny_amount > 0),
    grant_type VARCHAR(20) NOT NULL DEFAULT 'balance' CHECK (grant_type = 'balance'),
    grant_value DECIMAL(20,8) NOT NULL CHECK (grant_value > 0),
    group_id BIGINT,
    validity_days INT,
    external_url TEXT NOT NULL DEFAULT '',
    version INT NOT NULL DEFAULT 1 CHECK (version > 0),
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    target_stock INT NOT NULL DEFAULT 50000 CHECK (target_stock > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (grant_type = 'balance' AND group_id IS NULL AND validity_days IS NULL)
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
    status VARCHAR(32) NOT NULL CHECK (status IN ('pending', 'uploaded', 'failed', 'needs_reconciliation')),
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

ALTER TABLE liandong_product_mappings
    ADD COLUMN IF NOT EXISTS target_stock INT NOT NULL DEFAULT 50000;

ALTER TABLE liandong_product_mappings
    DROP CONSTRAINT IF EXISTS liandong_product_mappings_grant_type_check,
    DROP CONSTRAINT IF EXISTS liandong_product_mappings_check,
    DROP CONSTRAINT IF EXISTS liandong_product_mappings_balance_grant_check;
ALTER TABLE liandong_product_mappings
    ADD CONSTRAINT liandong_product_mappings_balance_grant_check
    CHECK (grant_type = 'balance' AND group_id IS NULL AND validity_days IS NULL);

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
    DROP CONSTRAINT IF EXISTS liandong_restock_batches_grant_type_balance_check;
ALTER TABLE liandong_restock_batches
    ADD CONSTRAINT liandong_restock_batches_grant_type_balance_check
    CHECK (grant_type = 'balance');

CREATE INDEX IF NOT EXISTS idx_liandong_restock_batches_job
    ON liandong_restock_batches(job_id, created_at);

ALTER TABLE liandong_restock_batches ALTER COLUMN status TYPE VARCHAR(32);
ALTER TABLE liandong_restock_batches DROP CONSTRAINT IF EXISTS liandong_restock_batches_status_check;
ALTER TABLE liandong_restock_batches
    ADD CONSTRAINT liandong_restock_batches_status_check
    CHECK (status IN ('pending', 'uploaded', 'failed', 'needs_reconciliation'));

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'liandong_restock_batches_job_id_fkey'
          AND conrelid = 'liandong_restock_batches'::regclass
    ) THEN
        ALTER TABLE liandong_restock_batches
            ADD CONSTRAINT liandong_restock_batches_job_id_fkey
            FOREIGN KEY (job_id) REFERENCES liandong_restock_jobs(job_id) ON DELETE SET NULL;
    END IF;
END
$$;

CREATE TABLE IF NOT EXISTS membership_products (
    sku TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    price_minor BIGINT NOT NULL DEFAULT 0 CHECK (price_minor >= 0),
    currency TEXT NOT NULL DEFAULT 'CNY' CHECK (currency = 'CNY'),
    period_days INTEGER NOT NULL DEFAULT 30 CHECK (period_days BETWEEN 1 AND 366),
    channel TEXT NOT NULL CHECK (channel IN ('gpt', 'gptpro')),
    credential_mode TEXT NOT NULL DEFAULT 'account_id' CHECK (credential_mode IN ('account_id', 'session')),
    for_sale BOOLEAN NOT NULL DEFAULT FALSE,
    paused BOOLEAN NOT NULL DEFAULT TRUE,
    verified_run_id UUID,
    verified_at TIMESTAMPTZ,
    upstream_available INTEGER CHECK (upstream_available >= 0),
    inventory_checked_at TIMESTAMPTZ,
    poll_seconds INTEGER NOT NULL DEFAULT 5 CHECK (poll_seconds BETWEEN 5 AND 3600),
    wait_seconds INTEGER NOT NULL DEFAULT 600 CHECK (wait_seconds BETWEEN 60 AND 86400),
    eta_minutes INTEGER NOT NULL DEFAULT 30 CHECK (eta_minutes BETWEEN 1 AND 1440),
    failures INTEGER NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (NOT for_sale OR (verified_at IS NOT NULL AND verified_run_id IS NOT NULL AND price_minor > 0))
);

INSERT INTO membership_products(sku, name, channel, credential_mode, poll_seconds, wait_seconds, eta_minutes) VALUES
    ('chatgpt_pro_20x', 'Codex/GPT Pro20x', 'gptpro', 'session', 180, 86400, 240),
    ('chatgpt_plus', 'Codex/GPT Plus', 'gpt', 'account_id', 5, 600, 30)
ON CONFLICT (sku) DO NOTHING;

CREATE TABLE IF NOT EXISTS membership_coupons (
    code TEXT PRIMARY KEY,
    discount_minor BIGINT NOT NULL CHECK (discount_minor > 0),
    max_uses INTEGER NOT NULL CHECK (max_uses > 0),
    used INTEGER NOT NULL DEFAULT 0 CHECK (used >= 0 AND used <= max_uses),
    expires_at TIMESTAMPTZ NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TABLE IF NOT EXISTS membership_orders (
    id UUID PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    sku TEXT NOT NULL REFERENCES membership_products(sku),
    kind TEXT NOT NULL CHECK (kind IN ('customer', 'validation')),
    idempotency_key TEXT NOT NULL CHECK (length(idempotency_key) BETWEEN 8 AND 128),
    payment_state TEXT NOT NULL DEFAULT 'created' CHECK (payment_state IN ('created', 'pending', 'paid', 'closed', 'refund_pending', 'refunded', 'manual_review')),
    price_minor BIGINT NOT NULL CHECK (price_minor >= 0),
    discount_minor BIGINT NOT NULL DEFAULT 0 CHECK (discount_minor >= 0),
    coupon_code TEXT REFERENCES membership_coupons(code),
    period_days INTEGER NOT NULL,
    channel TEXT NOT NULL,
    credential_mode TEXT NOT NULL,
    target_hash TEXT,
    target_masked TEXT NOT NULL DEFAULT '',
    credential_ref TEXT,
    credential_expires_at TIMESTAMPTZ,
    consent_version TEXT,
    consent_at TIMESTAMPTZ,
    input_required BOOLEAN NOT NULL DEFAULT TRUE,
    paid_at TIMESTAMPTZ,
    refund_requested_at TIMESTAMPTZ,
    refund_reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, idempotency_key),
    CHECK (kind <> 'validation' OR (price_minor = 0 AND payment_state = 'created'))
);

CREATE TABLE IF NOT EXISTS membership_cdks (
    id UUID PRIMARY KEY,
    sku TEXT NOT NULL REFERENCES membership_products(sku),
    channel TEXT NOT NULL,
    ciphertext TEXT NOT NULL,
    fingerprint TEXT NOT NULL UNIQUE,
    cost_minor BIGINT NOT NULL DEFAULT 0 CHECK (cost_minor >= 0),
    state TEXT NOT NULL DEFAULT 'available' CHECK (state IN ('available', 'reserved', 'submitted', 'spent', 'uncertain', 'retired')),
    order_id UUID UNIQUE REFERENCES membership_orders(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS membership_tasks (
    id UUID PRIMARY KEY,
    order_id UUID NOT NULL UNIQUE REFERENCES membership_orders(id),
    state TEXT NOT NULL DEFAULT 'awaiting_input' CHECK (state IN ('awaiting_input', 'queued', 'submitted', 'processing', 'review_required', 'succeeded', 'failed', 'canceled')),
    channel TEXT NOT NULL,
    cdk_id UUID REFERENCES membership_cdks(id),
    target_hash TEXT,
    lease_token UUID,
    lease_until TIMESTAMPTZ,
    next_run_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deadline_at TIMESTAMPTZ,
    error_code TEXT NOT NULL DEFAULT '',
    query_errors INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS membership_active_target ON membership_tasks(target_hash)
    WHERE target_hash IS NOT NULL AND state NOT IN ('succeeded', 'failed', 'canceled');
CREATE INDEX IF NOT EXISTS membership_task_due ON membership_tasks(next_run_at)
    WHERE state IN ('queued', 'submitted', 'processing');

CREATE TABLE IF NOT EXISTS membership_attempts (
    id UUID PRIMARY KEY,
    task_id UUID NOT NULL REFERENCES membership_tasks(id),
    cdk_id UUID NOT NULL REFERENCES membership_cdks(id),
    state TEXT NOT NULL CHECK (state IN ('intent', 'submitted', 'processing', 'review_required', 'succeeded', 'failed', 'not_submitted')),
    upstream_task_id TEXT NOT NULL DEFAULT '',
    error_code TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS membership_one_unresolved_attempt ON membership_attempts(task_id)
    WHERE state NOT IN ('failed', 'not_submitted');

CREATE TABLE IF NOT EXISTS membership_payment_links (
    payment_order_id BIGINT PRIMARY KEY REFERENCES payment_orders(id),
    order_id UUID NOT NULL REFERENCES membership_orders(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS membership_one_payment_per_order
    ON membership_payment_links(order_id);

CREATE TABLE IF NOT EXISTS membership_payment_redirect_tickets (
    ticket_hash BYTEA PRIMARY KEY CHECK (octet_length(ticket_hash) = 32),
    order_id UUID NOT NULL REFERENCES membership_orders(id),
    payment_order_id BIGINT NOT NULL REFERENCES payment_orders(id),
    user_id BIGINT NOT NULL REFERENCES users(id),
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS membership_payment_redirect_ticket_binding
    ON membership_payment_redirect_tickets(payment_order_id, order_id, user_id);
CREATE INDEX IF NOT EXISTS membership_payment_redirect_ticket_active
    ON membership_payment_redirect_tickets(expires_at) WHERE consumed_at IS NULL;

CREATE TABLE IF NOT EXISTS membership_events (
    id UUID PRIMARY KEY,
    order_id UUID REFERENCES membership_orders(id),
    action TEXT NOT NULL,
    actor TEXT NOT NULL,
    state TEXT NOT NULL DEFAULT '',
    error_code TEXT NOT NULL DEFAULT '',
    evidence_ref TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE OR REPLACE FUNCTION membership_reject_event_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'membership audit events are append-only';
END;
$$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_trigger
        WHERE tgname = 'membership_events_immutable'
          AND tgrelid = 'membership_events'::regclass
    ) THEN
        CREATE TRIGGER membership_events_immutable BEFORE UPDATE OR DELETE ON membership_events
            FOR EACH ROW EXECUTE FUNCTION membership_reject_event_mutation();
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_trigger
        WHERE tgname = 'membership_events_no_truncate'
          AND tgrelid = 'membership_events'::regclass
    ) THEN
        CREATE TRIGGER membership_events_no_truncate BEFORE TRUNCATE ON membership_events
            FOR EACH STATEMENT EXECUTE FUNCTION membership_reject_event_mutation();
    END IF;
END
$$;

CREATE TABLE IF NOT EXISTS membership_event_exports (
    event_id UUID PRIMARY KEY REFERENCES membership_events(id),
    exported_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    object_key TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS membership_notifications (
    event_id UUID PRIMARY KEY REFERENCES membership_events(id),
    order_id UUID NOT NULL REFERENCES membership_orders(id),
    sent_at TIMESTAMPTZ,
    attempts INTEGER NOT NULL DEFAULT 0,
    next_run_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS membership_ledger (
    id UUID PRIMARY KEY,
    order_id UUID NOT NULL REFERENCES membership_orders(id),
    kind TEXT NOT NULL CHECK (kind IN ('payment', 'fee', 'supplier_cost', 'refund')),
    amount_minor BIGINT NOT NULL CHECK (amount_minor >= 0),
    reference TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(order_id, kind, reference)
);

ALTER TABLE membership_coupons DROP CONSTRAINT IF EXISTS membership_coupons_check;
ALTER TABLE membership_coupons DROP CONSTRAINT IF EXISTS membership_coupons_used_check;
ALTER TABLE membership_coupons DROP CONSTRAINT IF EXISTS membership_coupons_used_nonnegative_check;
ALTER TABLE membership_coupons
    ADD CONSTRAINT membership_coupons_used_nonnegative_check CHECK (used >= 0);

-- The former rest2build migrations were consolidated into this file. Record
-- their original filename/checksum identities so a rollback to an older image
-- does not try to execute those non-idempotent files over the consolidated
-- schema. Existing rows must match exactly; a mismatch still fails closed.
DO $$
DECLARE
    legacy RECORD;
    existing_checksum TEXT;
BEGIN
    FOR legacy IN
        SELECT * FROM (VALUES
            ('231_codex_continuity.sql', 'fb0fe071f9dc46bf545881a4dbd37c363d578a2a17bf350dcef2a4ca26c49a7d'),
            ('232_codex_continuity_client_window.sql', 'c23cf6e17abd5c716852bbe5f47fe7b3031ecf76d9191c4e557ad5570819413e'),
            ('233_user_lifecycle_emails.sql', '51ac6f9cbadde06905353f2a7e2c57a4a1d2a07f23b07ca8995b4a6918f9d052'),
            ('234_user_first_topup_bonus.sql', 'e1a5f5f407730fde09855df2ecf1811b225c440b9912d831bd8c9cc7dd0b1c65'),
            ('235_liandong_sales_channel.sql', 'a63b03515aa530efe77d49181a2985da953e568b7d91706572c4943bfb002594'),
            ('236_membership_fulfillment.sql', '9439418945b02b697848eca8ef76a42d3e59f79e6176ee5b437958ca2b8f0e55'),
            ('237_membership_coupon_late_payment.sql', '076e78175d192a1ab0a9059fb353925c39ad15c327637f19e9d99336149ee91d')
        ) AS aliases(filename, checksum)
    LOOP
        SELECT checksum INTO existing_checksum
        FROM schema_migrations
        WHERE filename = legacy.filename;

        IF existing_checksum IS NULL THEN
            INSERT INTO schema_migrations(filename, checksum)
            VALUES (legacy.filename, legacy.checksum);
        ELSIF existing_checksum <> legacy.checksum THEN
            RAISE EXCEPTION 'legacy migration % checksum mismatch (db=%, expected=%)',
                legacy.filename, existing_checksum, legacy.checksum;
        END IF;
        existing_checksum := NULL;
    END LOOP;
END
$$;
