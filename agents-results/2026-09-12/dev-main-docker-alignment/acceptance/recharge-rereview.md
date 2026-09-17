# Coordinator capture of independent worker return

Source: recharge-rereview.log final response. Worker returned this in stdout; coordinator preserved it here.

M1: **VERIFIED (M1 only)**

Source: commit `64f459e05d2ec50ac997dd6933794a1fcccaac61` (`fix: hide native subscription checkout in store-only mode`).

- `PaymentView.vue:608-615` adds the native subscription tab only when `payment_enabled !== false`.
- `PaymentView.vue:1199-1205` forces `activeTab` to `recharge` and returns before stale `?tab=subscription` handling when native payment is disabled.
- Recharge tests `PaymentView.spec.ts:389-416` cover store-only mode and stale subscription queries.
- With `payment_enabled=true`, `PaymentView.vue:611-612` preserves the native subscription tab. The recharge tests also cover native-only and dual-channel states at `430-458`.

`failure_class`: feature-flag state-space/channel-isolation regression.  
`failure_origin`: unconditional native-tab exposure and missing disabled-mode tab normalization. Both are resolved in the frozen source.

Risk: the parent’s reported 33 payment/panel tests were not rerun here; the native-preservation test does not explicitly click/assert the subscription tab. No production or overall release readiness is claimed.
