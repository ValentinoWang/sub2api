-- Additive cost-ledger schema; apply explicitly to an isolated acceptance database first.
-- NOT automatically run by the HTTP service. No existing billing/account tables are changed.
BEGIN;
CREATE TABLE IF NOT EXISTS cost_center_lock (
 id smallint PRIMARY KEY CHECK (id = 1),
 schema_version integer NOT NULL CHECK (schema_version = 1)
);
INSERT INTO cost_center_lock(id,schema_version) VALUES (1,1) ON CONFLICT (id) DO NOTHING;
CREATE TABLE IF NOT EXISTS cost_center_events (
 seq bigint PRIMARY KEY CHECK (seq > 0),
 event_id varchar(32) UNIQUE NOT NULL,
 idempotency_key varchar(120) UNIQUE NOT NULL,
 request_hash char(64) NOT NULL,
 actor_id bigint NOT NULL CHECK (actor_id > 0),
 recorded_at timestamptz NOT NULL,
 kind varchar(16) NOT NULL CHECK (kind IN ('purchase','delivery','allocate','void')),
 payload jsonb NOT NULL CHECK (jsonb_typeof(payload)='object'),
 CHECK (payload->>'id' = event_id),
 CHECK ((payload->>'sequence')::bigint = seq),
 CHECK (payload->>'idempotency_key' = idempotency_key),
 CHECK (payload->>'request_hash' = request_hash),
 CHECK ((payload->>'actor_id')::bigint = actor_id),
 CHECK (payload->'command'->>'kind' = kind)
);
CREATE OR REPLACE FUNCTION cost_center_append_only_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'cost_center_events is append-only; use a void command for a booking error'; END;
$$;
DROP TRIGGER IF EXISTS cost_center_no_rewrite ON cost_center_events;
CREATE TRIGGER cost_center_no_rewrite BEFORE UPDATE OR DELETE ON cost_center_events
 FOR EACH ROW EXECUTE FUNCTION cost_center_append_only_guard();
DROP TRIGGER IF EXISTS cost_center_no_truncate ON cost_center_events;
CREATE TRIGGER cost_center_no_truncate BEFORE TRUNCATE ON cost_center_events
 FOR EACH STATEMENT EXECUTE FUNCTION cost_center_append_only_guard();
COMMIT;
-- Runtime role needs SELECT/INSERT on cost_center_events and SELECT/UPDATE on cost_center_lock.
-- It should NOT own these tables or have ALTER/TRUNCATE/DELETE privileges. Table owners can bypass guards.
