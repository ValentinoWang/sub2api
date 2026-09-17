# CNY denomination: current-code audit

Read-only assessment; implementation awaits the user's conversion-direction answer. The confirmed paid-order recovery remains USD 5 in the existing ledger.

Recommended architecture: preserve the canonical USD ledger and supplier prices, introduce a separate credit denomination setting (`CNY`, 7 CNY credit units per USD), render monetary credit values times 7, and convert credit inputs back by dividing by 7. Do not migrate every numeric database column or change the existing recharge multiplier to 7.

At the user's confirmed sales rule, paying CNY 5 buys USD 5 of internal credit and would display as CNY 35 credit units. A USD 0.10 request displays as CNY 0.70 consumed. This preserves purchasing power. The paid cash amount, gateway fees and refund cash remain in the original transaction currency.

## Existing authoritative calculation boundaries

- `backend/internal/service/payment_order.go`: recharge order `amount` is credited USD; `pay_amount` is the independent payment-currency amount. The recharge multiplier genuinely changes the USD credit issued.
- `backend/internal/service/billing_service.go` and `gateway_usage_billing.go`: supplier token prices, actual cost, user quota, subscription usage and account-side billing use the existing USD basis.
- `backend/internal/service/payment_amounts.go`: cash refund derives proportionally from payment amount and refunded credit. `payment_currency.go` uses the order currency snapshot.
- `SUBSCRIPTION_USD_TO_CNY_RATE` controls subscription checkout, not credit denomination. Reusing it as a global display factor can double-convert subscription prices.

## Implementation scope

- Add a credit-specific formatter and input conversion adapter. `frontend/src/utils/format.ts` currently defaults to USD but does not cover the many directly rendered dollar amounts. `components/payment/currency.ts` must remain an actual-payment-currency formatter.
- Convert user balance, frozen credit, redeem results, credit limits, notifications, affiliate credits, consumption statistics and credit exports. Preserve concurrency/count/duration and percent fields.
- Convert administrative credit inputs as well as displays: user balance grants, default registration credits, redeem-code face values of balance type, API key quotas, group/subscription monetary usage limits and user platform quotas. Loading and resaving an unchanged form must preserve its canonical value exactly.
- Keep upstream original balances/prices explicitly USD or original currency; an optional CNY reference value must not replace their source currency.
- Distinguish credit received from cash paid on purchase/QR/Stripe/status/order/refund screens. Historical orders retain their payment currency.
- Include CSV/XLSX exports, backend-generated redeem-code CSV, usage charts and notification emails, with explicit currency labels.

## Required verification

- Credit USD 10 renders CNY 70; spending USD 0.1 renders CNY 0.7 and leaves CNY 69.3.
- Entering a CNY 70 credit limit submits USD 10; opening and resaving does not compound conversion.
- A CNY 70 payment remains CNY 70; upstream USD 10 remains USD 10; a non-balance redemption value is unchanged.
- Partial refunds preserve the paid-cash/credited-value ratio, including fees; historical USD and CNY orders retain their original amounts.
- Subscription conversion enabled/disabled never applies denomination twice.
- Zero/unlimited, negative adjustments, percentage thresholds and very small token charges retain correct semantics and precision.
- Existing canonical billing precision and idempotency checks continue to pass.

No full-site CNY conversion or production deployment is claimed by this audit.
