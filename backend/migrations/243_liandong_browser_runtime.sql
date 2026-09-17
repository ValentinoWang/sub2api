-- Operational observations do not authorize merchant access or replenishment.
CREATE TABLE IF NOT EXISTS liandong_browser_runtime (
    device_id VARCHAR(32) PRIMARY KEY REFERENCES liandong_browser_devices(id) ON DELETE CASCADE,
    report JSONB NOT NULL CHECK (jsonb_typeof(report) = 'object'),
    reported_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
