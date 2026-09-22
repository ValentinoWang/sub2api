BEGIN;
CREATE TABLE IF NOT EXISTS cost_center_quota_observations (
 event_key char(64) PRIMARY KEY,
 account_id bigint NOT NULL CHECK(account_id>0),
 observed_at timestamptz NOT NULL,
 kind text NOT NULL CHECK(kind IN ('snapshot','after_reset')),
 payload jsonb NOT NULL CHECK(jsonb_typeof(payload)='object')
);
CREATE INDEX IF NOT EXISTS cost_center_quota_account_time ON cost_center_quota_observations(account_id,observed_at);
DROP TRIGGER IF EXISTS cost_center_quota_no_rewrite ON cost_center_quota_observations;
CREATE TRIGGER cost_center_quota_no_rewrite BEFORE UPDATE OR DELETE ON cost_center_quota_observations
 FOR EACH ROW EXECUTE FUNCTION cost_center_append_only_guard();
DROP TRIGGER IF EXISTS cost_center_quota_no_truncate ON cost_center_quota_observations;
CREATE TRIGGER cost_center_quota_no_truncate BEFORE TRUNCATE ON cost_center_quota_observations
 FOR EACH STATEMENT EXECUTE FUNCTION cost_center_append_only_guard();
COMMIT;
