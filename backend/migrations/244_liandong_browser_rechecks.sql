-- Queue operator-requested verification without granting merchant authorization.
CREATE TABLE IF NOT EXISTS liandong_browser_rechecks (
    id VARCHAR(32) PRIMARY KEY CHECK (id ~ '^[0-9a-f]{32}$'),
    device_id VARCHAR(32) NOT NULL REFERENCES liandong_browser_devices(id) ON DELETE RESTRICT,
    requested_by BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    state VARCHAR(16) NOT NULL DEFAULT 'queued' CHECK (state IN ('queued','checking','passed','failed')),
    reason VARCHAR(64) NOT NULL DEFAULT '',
    resumed BOOLEAN NOT NULL DEFAULT FALSE,
    requested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    CHECK ((state='queued' AND reason='' AND NOT resumed AND started_at IS NULL AND finished_at IS NULL)
        OR (state='checking' AND reason='' AND NOT resumed AND started_at IS NOT NULL AND finished_at IS NULL)
        OR (state='passed' AND reason='' AND started_at IS NOT NULL AND finished_at IS NOT NULL)
        OR (state='failed' AND reason<>'' AND NOT resumed AND finished_at IS NOT NULL))
);
CREATE UNIQUE INDEX IF NOT EXISTS liandong_browser_one_pending_recheck
    ON liandong_browser_rechecks(device_id) WHERE state IN ('queued','checking');
CREATE INDEX IF NOT EXISTS liandong_browser_latest_recheck
    ON liandong_browser_rechecks(device_id,requested_at DESC,id DESC);
