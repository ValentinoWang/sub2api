-- Per-user claim ledger for the first top-up bonus.
--
-- The payment audit log is keyed on (order_id, action), which cannot stop a second bonus when a
-- user has two orders in flight, or when a bonused order is later refunded and a new order looks
-- like a "first" top-up again. A user-keyed row makes the grant atomic and permanent.
CREATE TABLE IF NOT EXISTS user_first_topup_bonus (
    user_id      BIGINT       PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    order_id     BIGINT       NOT NULL,
    bonus_amount DOUBLE PRECISION NOT NULL,
    granted_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
