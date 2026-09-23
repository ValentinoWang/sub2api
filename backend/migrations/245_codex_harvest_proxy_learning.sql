-- Codex 打票代理学习记录：按 (代理, 账号, 身份, 模型) 统计采票结果。
-- 学习代数在重置时递增，使重置前发出的采票反馈不再写入。
CREATE TABLE IF NOT EXISTS codex_harvest_learning_epoch (
    id SMALLINT PRIMARY KEY CHECK (id = 1),
    generation BIGINT NOT NULL DEFAULT 1
);
INSERT INTO codex_harvest_learning_epoch (id, generation) VALUES (1, 1) ON CONFLICT DO NOTHING;

CREATE TABLE IF NOT EXISTS codex_harvest_nodes (
    id BIGSERIAL PRIMARY KEY,
    proxy_id BIGINT NOT NULL,
    proxy_name VARCHAR(255) NOT NULL DEFAULT '',
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    identity VARCHAR(64) NOT NULL DEFAULT '',
    model VARCHAR(200) NOT NULL,
    successes BIGINT NOT NULL DEFAULT 0,
    misses BIGINT NOT NULL DEFAULT 0,
    network_errors BIGINT NOT NULL DEFAULT 0,
    account_errors BIGINT NOT NULL DEFAULT 0,
    consecutive_failures INTEGER NOT NULL DEFAULT 0,
    last_success TIMESTAMPTZ,
    cooldown_until TIMESTAMPTZ,
    latency_ms BIGINT NOT NULL DEFAULT 0,
    last_result VARCHAR(40) NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (proxy_id, account_id, identity, model)
);
CREATE INDEX IF NOT EXISTS idx_codex_harvest_nodes_scope
    ON codex_harvest_nodes (account_id, identity, model);
CREATE INDEX IF NOT EXISTS idx_codex_harvest_nodes_updated
    ON codex_harvest_nodes (updated_at DESC, id DESC);

-- 打票代理改由代理池（openai_codex_harvest_controls_v1.proxy_ids）提供，单一代理地址设置停用。
DELETE FROM settings WHERE key = 'openai_codex_ticket_harvest_proxy_url';
