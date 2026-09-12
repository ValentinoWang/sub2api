-- These records reserve local rights only; merchant settlement is external.
CREATE TABLE IF NOT EXISTS liandong_code_refunds (
    external_order_no VARCHAR(128) PRIMARY KEY CHECK (btrim(external_order_no) <> ''),
    batch_id VARCHAR(64) NOT NULL,
    code_sha256 VARCHAR(64) NOT NULL UNIQUE CHECK (code_sha256 ~ '^[0-9a-f]{64}$'),
    redeem_code_id BIGINT NOT NULL UNIQUE REFERENCES redeem_codes(id) ON DELETE RESTRICT,
    status VARCHAR(32) NOT NULL DEFAULT 'reserved'
        CHECK (status IN ('reserved', 'merchant_reference_recorded')),
    merchant_refund_reference VARCHAR(128) UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    confirmed_at TIMESTAMPTZ,
    FOREIGN KEY (batch_id, code_sha256)
        REFERENCES liandong_restock_batch_codes(batch_id, code_sha256) ON DELETE RESTRICT,
    CHECK ((status = 'reserved' AND merchant_refund_reference IS NULL AND confirmed_at IS NULL)
        OR (status = 'merchant_reference_recorded' AND btrim(merchant_refund_reference) <> ''
            AND merchant_refund_reference IS NOT NULL AND confirmed_at IS NOT NULL))
);

-- A row lock also serializes this guard with redeem_codes' conditional Use update.
CREATE OR REPLACE FUNCTION protect_liandong_refund_record() RETURNS TRIGGER AS $$
DECLARE
    code_row redeem_codes%ROWTYPE;
BEGIN
    IF TG_OP IN ('DELETE', 'TRUNCATE') THEN
        RAISE EXCEPTION 'Liandong refund records are permanent' USING ERRCODE = '23514';
    ELSIF TG_OP = 'UPDATE' THEN
        IF NEW.external_order_no IS DISTINCT FROM OLD.external_order_no
            OR NEW.batch_id IS DISTINCT FROM OLD.batch_id
            OR NEW.code_sha256 IS DISTINCT FROM OLD.code_sha256
            OR NEW.redeem_code_id IS DISTINCT FROM OLD.redeem_code_id
            OR NEW.created_at IS DISTINCT FROM OLD.created_at
            OR (OLD.status = 'merchant_reference_recorded' AND NEW IS DISTINCT FROM OLD) THEN
            RAISE EXCEPTION 'Liandong refund identity and confirmation are immutable' USING ERRCODE = '23514';
        END IF;
    ELSE
        SELECT * INTO code_row FROM redeem_codes WHERE id = NEW.redeem_code_id FOR UPDATE;
        IF NOT FOUND OR code_row.status <> 'disabled' OR code_row.used_by IS NOT NULL
            OR code_row.used_at IS NOT NULL
            OR encode(sha256(convert_to(code_row.code, 'UTF8')), 'hex') <> NEW.code_sha256 THEN
            RAISE EXCEPTION 'Liandong refund requires frozen unused rights' USING ERRCODE = '23514';
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS liandong_refund_record_guard ON liandong_code_refunds;
CREATE TRIGGER liandong_refund_record_guard BEFORE INSERT OR UPDATE OR DELETE
    ON liandong_code_refunds FOR EACH ROW EXECUTE FUNCTION protect_liandong_refund_record();

DROP TRIGGER IF EXISTS liandong_refund_truncate_guard ON liandong_code_refunds;
CREATE TRIGGER liandong_refund_truncate_guard BEFORE TRUNCATE
    ON liandong_code_refunds FOR EACH STATEMENT EXECUTE FUNCTION protect_liandong_refund_record();

CREATE OR REPLACE FUNCTION protect_liandong_refund_code() RETURNS TRIGGER AS $$
BEGIN
    IF EXISTS (SELECT 1 FROM liandong_code_refunds WHERE redeem_code_id = OLD.id) THEN
        RAISE EXCEPTION 'Liandong refund rights are permanently frozen' USING ERRCODE = '23514';
    END IF;
    IF TG_OP = 'DELETE' THEN RETURN OLD; END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS liandong_refund_code_guard ON redeem_codes;
CREATE TRIGGER liandong_refund_code_guard BEFORE UPDATE OR DELETE
    ON redeem_codes FOR EACH ROW EXECUTE FUNCTION protect_liandong_refund_code();
