-- A verified late payment may restore a coupon reservation after another
-- checkout consumed the nominal final use. Keep the factual counter nonnegative
-- instead of rejecting settlement of money that the provider has confirmed.
ALTER TABLE membership_coupons DROP CONSTRAINT IF EXISTS membership_coupons_check;
ALTER TABLE membership_coupons DROP CONSTRAINT IF EXISTS membership_coupons_used_check;
ALTER TABLE membership_coupons
    ADD CONSTRAINT membership_coupons_used_nonnegative_check CHECK (used >= 0);
