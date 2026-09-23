-- Codex 打票流水：只保留最近 200 条，重启后用于恢复管理页面的流水列表。
CREATE TABLE IF NOT EXISTS codex_harvest_flow_events (
    id BIGSERIAL PRIMARY KEY,
    event_id VARCHAR(64) NOT NULL UNIQUE,
    at TIMESTAMPTZ NOT NULL,
    stage VARCHAR(32) NOT NULL DEFAULT '',
    kind VARCHAR(32) NOT NULL DEFAULT '',
    account_id BIGINT NOT NULL DEFAULT 0,
    account_name VARCHAR(80) NOT NULL DEFAULT '',
    model VARCHAR(64) NOT NULL DEFAULT '',
    proxy_id BIGINT NOT NULL DEFAULT 0,
    proxy_name VARCHAR(160) NOT NULL DEFAULT '',
    http_status INTEGER NOT NULL DEFAULT 0,
    length INTEGER NOT NULL DEFAULT 0,
    accepted BOOLEAN NOT NULL DEFAULT FALSE,
    manual BOOLEAN NOT NULL DEFAULT FALSE,
    result VARCHAR(64) NOT NULL DEFAULT '',
    detail VARCHAR(240) NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_codex_harvest_flow_events_at
    ON codex_harvest_flow_events (at ASC, id ASC);
