BEGIN;
CREATE TABLE IF NOT EXISTS cost_center_traffic_samples (
 source varchar(120) NOT NULL,
 interface varchar(64) NOT NULL,
 observed_at timestamptz NOT NULL,
 payload jsonb NOT NULL CHECK(jsonb_typeof(payload)='object'),
 PRIMARY KEY(source,interface,observed_at)
);
DROP TRIGGER IF EXISTS cost_center_traffic_no_rewrite ON cost_center_traffic_samples;
CREATE TRIGGER cost_center_traffic_no_rewrite BEFORE UPDATE OR DELETE ON cost_center_traffic_samples FOR EACH ROW EXECUTE FUNCTION cost_center_append_only_guard();
DROP TRIGGER IF EXISTS cost_center_traffic_no_truncate ON cost_center_traffic_samples;
CREATE TRIGGER cost_center_traffic_no_truncate BEFORE TRUNCATE ON cost_center_traffic_samples FOR EACH STATEMENT EXECUTE FUNCTION cost_center_append_only_guard();
COMMIT;
