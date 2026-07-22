-- Durable, tenant-isolated Codex continuity ledger.
-- Payloads are AES-GCM encrypted by the repository before insertion.

CREATE TABLE IF NOT EXISTS codex_continuity_threads (
    id                        BIGSERIAL PRIMARY KEY,
    continuity_id             VARCHAR(64) NOT NULL UNIQUE,
    user_id                   BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    api_key_id                BIGINT NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    session_hash              VARCHAR(64) NOT NULL,
    status                    VARCHAR(16) NOT NULL DEFAULT 'active',
    version                   BIGINT NOT NULL DEFAULT 0,
    created_at                TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at                TIMESTAMPTZ NOT NULL,
    CONSTRAINT chk_codex_continuity_threads_status
        CHECK (status IN ('active', 'deleted')),
    CONSTRAINT chk_codex_continuity_threads_version
        CHECK (version >= 0)
);

CREATE TABLE IF NOT EXISTS codex_continuity_turns (
    id                        BIGSERIAL PRIMARY KEY,
    thread_id                 BIGINT NOT NULL REFERENCES codex_continuity_threads(id) ON DELETE CASCADE,
    sequence                  BIGINT NOT NULL,
    request_id                VARCHAR(128) NOT NULL,
    state                     VARCHAR(16) NOT NULL DEFAULT 'completed',
    replay_input_encrypted    TEXT NOT NULL,
    replay_sha256             VARCHAR(64) NOT NULL,
    replay_bytes              BIGINT NOT NULL,
    upstream_account_id       BIGINT REFERENCES accounts(id) ON DELETE SET NULL,
    upstream_response_id      VARCHAR(128) NOT NULL DEFAULT '',
    created_at                TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    committed_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_codex_continuity_turn_sequence UNIQUE (thread_id, sequence),
    CONSTRAINT uq_codex_continuity_turn_request UNIQUE (thread_id, request_id),
    CONSTRAINT chk_codex_continuity_turns_state
        CHECK (state IN ('completed', 'failed', 'aborted')),
    CONSTRAINT chk_codex_continuity_turns_size
        CHECK (replay_bytes >= 0)
);

CREATE INDEX IF NOT EXISTS idx_codex_continuity_threads_owner
    ON codex_continuity_threads(user_id, api_key_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_codex_continuity_threads_expiry
    ON codex_continuity_threads(expires_at);
CREATE INDEX IF NOT EXISTS idx_codex_continuity_turns_latest
    ON codex_continuity_turns(thread_id, sequence DESC)
    WHERE state = 'completed';
