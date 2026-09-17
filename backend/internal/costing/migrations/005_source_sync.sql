BEGIN;
CREATE TABLE IF NOT EXISTS cost_center_account_observations (
 source varchar(120) NOT NULL,
 account_id bigint NOT NULL CHECK(account_id>0),
 source_hash char(64) NOT NULL,
 observed_at timestamptz NOT NULL,
 payload jsonb NOT NULL CHECK(jsonb_typeof(payload)='object'),
 PRIMARY KEY(source,account_id,source_hash)
);
CREATE TABLE IF NOT EXISTS cost_center_source_snapshots (
 snapshot_id char(64) PRIMARY KEY,
 source varchar(120) NOT NULL,
 scope_start timestamptz NOT NULL,
 scope_end timestamptz NOT NULL CHECK(scope_end>scope_start),
 observed_at timestamptz NOT NULL,
 actor_id bigint NOT NULL CHECK(actor_id>0),
 source_hash char(64) NOT NULL,
 request_count bigint NOT NULL CHECK(request_count>=0),
 payload jsonb NOT NULL CHECK(jsonb_typeof(payload)='object')
);
CREATE INDEX IF NOT EXISTS cost_center_source_scope ON cost_center_source_snapshots(source,scope_start,scope_end,observed_at);
DROP TRIGGER IF EXISTS cost_center_account_no_rewrite ON cost_center_account_observations;
CREATE TRIGGER cost_center_account_no_rewrite BEFORE UPDATE OR DELETE ON cost_center_account_observations FOR EACH ROW EXECUTE FUNCTION cost_center_append_only_guard();
DROP TRIGGER IF EXISTS cost_center_account_no_truncate ON cost_center_account_observations;
CREATE TRIGGER cost_center_account_no_truncate BEFORE TRUNCATE ON cost_center_account_observations FOR EACH STATEMENT EXECUTE FUNCTION cost_center_append_only_guard();
DROP TRIGGER IF EXISTS cost_center_source_no_rewrite ON cost_center_source_snapshots;
CREATE TRIGGER cost_center_source_no_rewrite BEFORE UPDATE OR DELETE ON cost_center_source_snapshots FOR EACH ROW EXECUTE FUNCTION cost_center_append_only_guard();
DROP TRIGGER IF EXISTS cost_center_source_no_truncate ON cost_center_source_snapshots;
CREATE TRIGGER cost_center_source_no_truncate BEFORE TRUNCATE ON cost_center_source_snapshots FOR EACH STATEMENT EXECUTE FUNCTION cost_center_append_only_guard();
COMMIT;
