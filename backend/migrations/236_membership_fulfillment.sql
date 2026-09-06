CREATE TABLE membership_products (
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
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (NOT for_sale OR (verified_at IS NOT NULL AND verified_run_id IS NOT NULL AND price_minor > 0))
);
INSERT INTO membership_products(sku,name,channel,credential_mode,poll_seconds,wait_seconds,eta_minutes) VALUES
('chatgpt_pro_20x','Codex/GPT Pro20x','gptpro','session',180,86400,240),
('chatgpt_plus','Codex/GPT Plus','gpt','account_id',5,600,30);

CREATE TABLE membership_coupons (
    code TEXT PRIMARY KEY,
    discount_minor BIGINT NOT NULL CHECK (discount_minor > 0),
    max_uses INTEGER NOT NULL CHECK (max_uses > 0),
    used INTEGER NOT NULL DEFAULT 0 CHECK (used >= 0 AND used <= max_uses),
    expires_at TIMESTAMPTZ NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE
);
CREATE TABLE membership_orders (
    id UUID PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    sku TEXT NOT NULL REFERENCES membership_products(sku),
    kind TEXT NOT NULL CHECK (kind IN ('customer','validation')),
    idempotency_key TEXT NOT NULL CHECK (length(idempotency_key) BETWEEN 8 AND 128),
    payment_state TEXT NOT NULL DEFAULT 'created' CHECK (payment_state IN ('created','pending','paid','closed','refund_pending','refunded','manual_review')),
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
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(user_id,idempotency_key),
    CHECK (kind <> 'validation' OR (price_minor = 0 AND payment_state = 'created'))
);
CREATE TABLE membership_cdks (
    id UUID PRIMARY KEY,
    sku TEXT NOT NULL REFERENCES membership_products(sku),
    channel TEXT NOT NULL,
    ciphertext TEXT NOT NULL,
    fingerprint TEXT NOT NULL UNIQUE,
    cost_minor BIGINT NOT NULL DEFAULT 0 CHECK (cost_minor >= 0),
    state TEXT NOT NULL DEFAULT 'available' CHECK (state IN ('available','reserved','submitted','spent','uncertain','retired')),
    order_id UUID UNIQUE REFERENCES membership_orders(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE membership_tasks (
    id UUID PRIMARY KEY,
    order_id UUID NOT NULL UNIQUE REFERENCES membership_orders(id),
    state TEXT NOT NULL DEFAULT 'awaiting_input' CHECK (state IN ('awaiting_input','queued','submitted','processing','review_required','succeeded','failed','canceled')),
    channel TEXT NOT NULL,
    cdk_id UUID REFERENCES membership_cdks(id),
    target_hash TEXT,
    lease_token UUID,
    lease_until TIMESTAMPTZ,
    next_run_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deadline_at TIMESTAMPTZ,
    error_code TEXT NOT NULL DEFAULT '',
    query_errors INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX membership_active_target ON membership_tasks(target_hash)
WHERE target_hash IS NOT NULL AND state NOT IN ('succeeded','failed','canceled');
CREATE INDEX membership_task_due ON membership_tasks(next_run_at) WHERE state IN ('queued','submitted','processing');
CREATE TABLE membership_attempts (
    id UUID PRIMARY KEY,
    task_id UUID NOT NULL REFERENCES membership_tasks(id),
    cdk_id UUID NOT NULL REFERENCES membership_cdks(id),
    state TEXT NOT NULL CHECK (state IN ('intent','submitted','processing','review_required','succeeded','failed','not_submitted')),
    upstream_task_id TEXT NOT NULL DEFAULT '',
    error_code TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX membership_one_unresolved_attempt ON membership_attempts(task_id)
WHERE state NOT IN ('failed','not_submitted');
CREATE TABLE membership_payment_links (
    payment_order_id BIGINT PRIMARY KEY REFERENCES payment_orders(id),
    order_id UUID NOT NULL REFERENCES membership_orders(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX membership_one_payment_per_order ON membership_payment_links(order_id);
CREATE TABLE membership_payment_redirect_tickets (
    ticket_hash BYTEA PRIMARY KEY CHECK (octet_length(ticket_hash) = 32),
    order_id UUID NOT NULL REFERENCES membership_orders(id),
    payment_order_id BIGINT NOT NULL REFERENCES payment_orders(id),
    user_id BIGINT NOT NULL REFERENCES users(id),
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ
);
CREATE INDEX membership_payment_redirect_ticket_binding ON membership_payment_redirect_tickets(payment_order_id,order_id,user_id);
CREATE INDEX membership_payment_redirect_ticket_active ON membership_payment_redirect_tickets(expires_at) WHERE consumed_at IS NULL;
CREATE TABLE membership_events (
    id UUID PRIMARY KEY,
    order_id UUID REFERENCES membership_orders(id),
    action TEXT NOT NULL,
    actor TEXT NOT NULL,
    state TEXT NOT NULL DEFAULT '',
    error_code TEXT NOT NULL DEFAULT '',
    evidence_ref TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE OR REPLACE FUNCTION membership_reject_event_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'membership audit events are append-only'; END; $$;
CREATE TRIGGER membership_events_immutable BEFORE UPDATE OR DELETE ON membership_events
FOR EACH ROW EXECUTE FUNCTION membership_reject_event_mutation();
CREATE TRIGGER membership_events_no_truncate BEFORE TRUNCATE ON membership_events
FOR EACH STATEMENT EXECUTE FUNCTION membership_reject_event_mutation();
CREATE TABLE membership_event_exports (
    event_id UUID PRIMARY KEY REFERENCES membership_events(id),
    exported_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    object_key TEXT NOT NULL
);
CREATE TABLE membership_notifications (
    event_id UUID PRIMARY KEY REFERENCES membership_events(id),
    order_id UUID NOT NULL REFERENCES membership_orders(id),
    sent_at TIMESTAMPTZ,
    attempts INTEGER NOT NULL DEFAULT 0,
    next_run_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE membership_ledger (
    id UUID PRIMARY KEY,
    order_id UUID NOT NULL REFERENCES membership_orders(id),
    kind TEXT NOT NULL CHECK (kind IN ('payment','fee','supplier_cost','refund')),
    amount_minor BIGINT NOT NULL CHECK (amount_minor >= 0),
    reference TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(order_id,kind,reference)
);
