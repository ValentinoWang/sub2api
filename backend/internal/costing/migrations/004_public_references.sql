BEGIN;
CREATE TABLE IF NOT EXISTS cost_center_public_references (
 source text NOT NULL CHECK(source='https://codexradar.com/en/'),
 retrieved_at timestamptz NOT NULL,
 source_hash char(64) NOT NULL,
 payload jsonb NOT NULL CHECK(jsonb_typeof(payload)='object'),
 PRIMARY KEY(source,retrieved_at)
);
DROP TRIGGER IF EXISTS cost_center_public_no_rewrite ON cost_center_public_references;
CREATE TRIGGER cost_center_public_no_rewrite BEFORE UPDATE OR DELETE ON cost_center_public_references FOR EACH ROW EXECUTE FUNCTION cost_center_append_only_guard();
DROP TRIGGER IF EXISTS cost_center_public_no_truncate ON cost_center_public_references;
CREATE TRIGGER cost_center_public_no_truncate BEFORE TRUNCATE ON cost_center_public_references FOR EACH STATEMENT EXECUTE FUNCTION cost_center_append_only_guard();
COMMIT;
