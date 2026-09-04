-- Tracks lifecycle marketing emails (welcome / inactivity / win-back) so each is sent at most
-- once per user per event. Rows are owned by the user_lifecycle service; safe to truncate.
CREATE TABLE IF NOT EXISTS user_lifecycle_emails (
    user_id    BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event      VARCHAR(64)  NOT NULL,
    sent_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, event)
);

CREATE INDEX IF NOT EXISTS idx_user_lifecycle_emails_event_sent
    ON user_lifecycle_emails(event, sent_at DESC);
