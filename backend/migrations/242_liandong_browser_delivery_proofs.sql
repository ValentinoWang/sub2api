-- Preserve the first complete delivery attestation without mixing sold cards into stock.
CREATE TABLE IF NOT EXISTS liandong_browser_delivery_proofs (
    batch_id VARCHAR(64) PRIMARY KEY REFERENCES liandong_restock_batches(batch_id) ON DELETE RESTRICT,
    device_id VARCHAR(32) NOT NULL REFERENCES liandong_browser_devices(id) ON DELETE RESTRICT,
    goods_id BIGINT NOT NULL CHECK (goods_id > 0),
    delivery_proof_version INT NOT NULL DEFAULT 1 CHECK (delivery_proof_version = 1),
    code_count INT NOT NULL CHECK (code_count BETWEEN 1 AND 20),
    code_sha256 VARCHAR(64) NOT NULL CHECK (code_sha256 ~ '^[0-9a-f]{64}$'),
    code_hashes JSONB NOT NULL CHECK (jsonb_typeof(code_hashes) = 'array'),
    unsold_batch_hashes JSONB NOT NULL CHECK (jsonb_typeof(unsold_batch_hashes) = 'array'),
    sold_proofs JSONB NOT NULL CHECK (jsonb_typeof(sold_proofs) = 'array'),
    observed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (jsonb_array_length(code_hashes) = code_count),
    CHECK (jsonb_array_length(unsold_batch_hashes) + jsonb_array_length(sold_proofs) = code_count)
);
