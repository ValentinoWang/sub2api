BEGIN;
ALTER TABLE cost_center_events DROP CONSTRAINT IF EXISTS cost_center_events_kind_check;
ALTER TABLE cost_center_events ADD CONSTRAINT cost_center_events_kind_check CHECK
 (kind IN ('purchase','delivery','allocate','void','credit_lot','credit_use','account_interval','traffic','replay','forecast'));
COMMIT;
