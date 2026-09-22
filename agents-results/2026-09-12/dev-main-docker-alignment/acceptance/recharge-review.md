# recharge-release-review

## Review identity

- `task_id`: `recharge-release-review`
- `direct_parent`: `dev-main-docker-alignment`
- `review_route`: `lw-luna` via `/Users/vsiyo/.codex/workers/run-lw-luna.sh`
- `authority`: independent read-only review; no descendants
- `proposed_lane_result`: `FAILED`
- `release_decision`: not made here; this lane does not declare final release `READY`

## Source tuple

- `base`: `20189b7348fa74020e066241890c240e0c28cad4`
- `tree`: `72b968baf05eb473297dfa4832890c0702915e85`
- `extracted_tree`: `/var/folders/w0/td0lhml95j374vr26ypgjzfh0000gn/T/sub2api-candidate-8qjoq7ad`
- `exact_diff`: `/Users/vsiyo/Desktop/Opensource_Tool/Sub2api/agents-results/2026-09-12/dev-main-docker-alignment/acceptance/candidate.patch`
- `exact_diff_sha256`: `df84899b52f33f57d91751124964dfa1bfcfd207919871d5bfefdc2244b719cc`
- `declared_scope`: Git `dev/main` and Docker artifact verification; no production cutover or live commerce approval

## Actual scope

Read-only review covered the exact candidate diff and the referenced frozen-tree source for:

- `frontend/src/components/payment`, including `LiandongRechargePanel.vue`, `liandongPurchase.ts`, `useRedeemCode.ts`, `paymentFlow.ts`, and related tests
- `frontend/src/views/user/PaymentView.vue`, `RedeemView.vue`, and related user tests
- `frontend/src/views/admin/LiandongToolkitView.vue` and its tests
- `frontend/src/api/liandongToolkit.ts`, `redeem.ts`, related API tests, settings/types/url helpers
- router, feature flags, sidebar access, and related route tests
- narrowly referenced backend Liandong handler/service code and tests for the restock contract
- parent source identity and worker-ledger metadata; no secrets were used as evidence

Only this file was written:

`/Users/vsiyo/Desktop/Opensource_Tool/Sub2api/agents-results/2026-09-12/dev-main-docker-alignment/acceptance/recharge-review.md`

No product source, tests, Git state, contracts, settings, runtime, credentials, database, external service, or parent evidence file was changed.

## Commands

Read-only commands used:

- `rg -n ...` for scoped source, test, flag, route, and diff-hunk discovery
- `nl -ba ... | sed -n ...` for exact line inspection
- `sed -n ... candidate.patch` for the candidate hunks
- `sha256sum .../candidate.patch` for diff identity
- `git status --short --untracked-files=all` for worktree observation

Not run by this lane: dependency installation, `pnpm` tests/build/lint/type checks, Go tests/build, Docker build, browser/E2E, runtime probes, production access, live purchase, or external-service calls.

## Findings

### M1 - FAILED: store-only mode exposes a non-functional subscription tab

- `severity`: Medium
- `failure_class`: user-visible feature-flag state-space regression; channel-isolation coverage gap
- `failure_origin`: `frontend/src/views/user/PaymentView.vue` unconditionally derives the native subscription tab while the native-payment-disabled mount path skips the checkout request
- `files`: `frontend/src/views/user/PaymentView.vue:4-8, 223-228, 585-612, 1197-1204`; `frontend/src/router/index.ts:1055-1062`; `frontend/src/views/user/__tests__/PaymentView.spec.ts:389-400`
- `repro`:
  1. Load public settings with `payment_enabled: false` and `purchase_subscription_enabled: true`.
  2. Open `/purchase`; the route guard intentionally permits this store-only combination.
  3. `PaymentView` renders the store recharge tab and also renders the `subscription` tab because `tabs` always appends it.
  4. Select `subscription`. The early `onMounted` return does not call `paymentAPI.getCheckoutInfo()`, so `checkout.plans` remains `[]`; the view displays `payment.noPlans`.
- `why`: The accepted store-only behavior is to expose the external-store purchase/redemption flow. The subscription tab is a native checkout surface, but this flag combination has no native checkout data. The user can therefore enter a visible flow that cannot show or purchase a plan. This contradicts the store-only test name and the channel boundary even though the store panel itself renders correctly.
- `test_gap`: The added test at `PaymentView.spec.ts:389-400` asserts the store panel, absence of `AmountInput`, absence of the recharge-channel selector, and absence of the checkout request. It does not assert that the subscription tab is absent or inaccessible, so it passes while this regression remains.

No other independent product finding was established in the bounded static review.

## Area dispositions and evidence

### Redemption: no static finding

- `useRedeemCode.ts:16-45` rejects empty/in-flight duplicates, trims the submitted code, clears the input only after server success, retains the server result, and treats refresh failure as a warning rather than as redemption failure.
- `LiandongRechargePanel.vue:20-46` keeps the buyer link separate from same-page redemption, uses `target="_blank"` with `rel="noopener noreferrer"`, and renders the server-returned credit/balance.
- `RedeemView.vue:364-371, 432-447` uses the same composable and refreshes history after a confirmed result.
- `LiandongRechargePanel.spec.ts:36-105` meaningfully covers the buyer link, trimmed code, server amount, duplicate submit, refresh failure, rejected/already-used code, unsafe/missing URL, and store switch.

Direct `RedeemView` integration coverage is absent from the scoped file inventory. The user view's history refresh, success-toast path, contact-info load, and exact rendering are therefore unverified in this lane; this is recorded as a coverage limitation, not an invented runtime failure.

### Channel isolation: failed for M1

- `featureFlags.ts:161-166` and `router/index.ts:1055-1062` consistently define the purchase route as available when either native payment or the external store channel is enabled.
- `PaymentView.vue:585-606` correctly separates the store and online recharge channels and `PaymentView.vue:4-33` renders only the selected recharge channel.
- `PaymentView.spec.ts:415-443` covers native-only and both-channel selection, while `feature-access.spec.ts:179-205` covers route access. Neither covers the store-only subscription-tab state described in M1.

### Restock toggle: no static finding; execution unverified

- `LiandongToolkitView.vue:540-541, 827-849` gates concurrent actions, blocks enabling without configuration, performs persisted-status readback, rejects mismatched readback, and preserves an error on ambiguous writes.
- `liandongToolkit.ts:241-244` calls the dedicated `/admin/liandong/restock/enable` endpoint.
- The backend handler delegates the boolean to `SetEnabled` (`backend/internal/handler/admin/payment_liandong.go:89-108`), and the service persists the flag after configuration checks while cancelling an active run on disable (`backend/internal/service/liandong_restock_service.go:738-765`).
- Frontend tests cover duplicate submission, readiness, disable-while-running, readback failure, mismatch, and ambiguous write (`frontend/src/views/admin/__tests__/LiandongToolkitView.spec.ts:158-234`). API path coverage is present at `frontend/src/api/__tests__/liandongToolkit.spec.ts:39-44`; the narrowly read backend service test covers cancellation and persisted disable state at `backend/internal/service/liandong_restock_service_test.go:312-358`.

These are source/test-presence observations only. This lane intentionally did not execute them or claim backend/runtime behavior.

## Unverified boundaries

- No dependency, build, lint, type, unit, integration, Docker, browser, or runtime execution was performed here; parent-run evidence remains outside this lane's execution authority.
- No LDXP credentials, merchant endpoint, inventory, payment, redemption, or production data was accessed.
- Human/live purchase acceptance and real inventory reconciliation remain pending. Automated or static evidence cannot substitute for that acceptance.
- This report does not approve opening sales, enabling payment, promoting a release, or performing a production cutover.

## Proposed disposition

`FAILED` for the `recharge-release-review` lane because M1 is a reproducible user-visible channel-isolation failure in the reviewed frozen tree. Redemption and restock logic have no additional static finding, but their dynamic execution and live commerce behavior remain unverified. The parent release reviewer must resolve or explicitly disposition M1 and separately complete the required human/live acceptance before any final release decision; this lane does not declare `READY`.
