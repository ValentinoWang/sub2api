# Public Release Review: public-release-review

## Review identity and source tuple

- `task_id`: `public-release-review`
- `direct_parent`: `dev-main-docker-alignment`
- `route`: `lw-luna`
- `review_mode`: independent read-only static review; no descendants
- `candidate_tree`: `72b968baf05eb473297dfa4832890c0702915e85`
- `base_tree`: `20189b7348fa74020e066241890c240e0c28cad4`
- `frozen_source`: `/var/folders/w0/td0lhml95j374vr26ypgjzfh0000gn/T/sub2api-candidate-8qjoq7ad`
- `candidate_patch`: `/Users/vsiyo/Desktop/Opensource_Tool/Sub2api/agents-results/2026-09-12/dev-main-docker-alignment/acceptance/candidate.patch`
- `candidate_patch_sha256`: `df84899b52f33f57d91751124964dfa1bfcfd207919871d5bfefdc2244b719cc`
- `declared_scope`: Git `dev/main` alignment and Docker artifact verification; no production cutover, live sales, or live commerce approval

## Actual read and write scope

Read-only project scope:

- The exact candidate patch and the frozen candidate tree.
- The referenced frontend public surfaces: shared public/layout components, `HomeView`, `DashboardView`, tutorial and troubleshooting views, experience content/navigation, locale entries, payment/redeem views, router/settings/types, and related tests.
- Source identity and parent review context at `acceptance/source-identity.json`, `acceptance/public-prompt.txt`, and `acceptance/review-disposition.md`.
- The release-review procedure and template for report structure.

The only write target for this lane is this file:
`/Users/vsiyo/Desktop/Opensource_Tool/Sub2api/agents-results/2026-09-12/dev-main-docker-alignment/acceptance/public-review.md`.

No product source, tests, Git state, contracts, settings, runtime, credentials, external service, or production state was changed. No production source or endpoint was accessed.

## Commands and method

Read-only inspection used `rg --files`, `rg -n`, `sed`, `nl -ba`, `wc -l`, `stat`, and `shasum -a 256` against the frozen source, the exact patch, and the listed acceptance context. A Git identity query against the extracted tree was not applicable because the frozen extraction has no `.git`; the source tuple was taken from the supplied task and `source-identity.json`.

No dependency install, Go command, Docker command, build, test, browser run, network request, production access, or live purchase was performed by this lane. Parent-owned build and test evidence was not rerun or rewritten.

## Findings

### M-01 - MEDIUM - Native subscription tab remains visible but cannot load plans when native payment is disabled

- `severity`: `MEDIUM`
- `release_blocking_for_this_lane`: `Yes`
- `failure_class`: user-facing feature-gating/state regression; misleading empty purchase state
- `failure_origin`: candidate `PaymentView.vue` channel split and early-return path, with the related regression assertion not checking tab visibility or selection
- `files_and_lines`:
  - `frontend/src/router/index.ts:406-416` defines authenticated `/purchase` with `requiresPurchase`.
  - `frontend/src/router/index.ts:1055-1063` allows that route when `payment_enabled` is `false` and `purchase_subscription_enabled` is `true`.
  - `frontend/src/views/user/PaymentView.vue:573-577` initializes `checkout.plans` to an empty array.
  - `frontend/src/views/user/PaymentView.vue:608-612` adds the `subscription` tab unconditionally whenever the component is rendered.
  - `frontend/src/views/user/PaymentView.vue:1197-1203` skips `paymentAPI.getCheckoutInfo()` and leaves the empty checkout state when `payment_enabled` is `false`.
  - `frontend/src/views/user/PaymentView.vue:223-228` renders the subscription empty state from `checkout.plans.length === 0`.
  - `frontend/src/views/user/__tests__/PaymentView.spec.ts:389-400` names the scenario `"only the store flow"` but does not assert that the subscription tab is absent or unusable state is unreachable.

#### Static reproduction

1. Use the settings tuple covered by the new test: `payment_enabled: false` and `purchase_subscription_enabled: true`.
2. The purchase route guard does not redirect because the store subscription flag is enabled.
3. On mount, `PaymentView` returns before loading checkout data, so `checkout.plans` remains `[]` and `loading` becomes `false`.
4. The store flag makes `rechargeAvailable` true, so `tabs` contains both `recharge` and the unconditional `subscription` entry. The tab switcher is therefore rendered.
5. Selecting the `subscription` tab sets `activeTab` to `subscription`, then the subscription plan branch renders the `payment.noPlans` empty state. The English locale says `No subscription plans available` (`frontend/src/i18n/locales/en/misc.ts:549-551`); the corresponding Chinese entry is at `frontend/src/i18n/locales/zh/misc.ts:573-575`.

This is deterministic from the source and does not require a backend response. It leaves a buyer-facing tab that presents a native subscription surface with no plans while the native payment path is explicitly disabled. The store panel still exists, so this is a partial-flow regression rather than a total purchase outage.

#### Evidence and disposition

The candidate patch adds the early return and the unconditional subscription tab in the same `PaymentView` change. The related test verifies that `LiandongRechargePanel` is present, `AmountInput` is absent, the recharge-channel selector is absent, and checkout is not called, but it never exercises the tab list or a click on `subscription`. The test title therefore overstates the behavior it protects.

Required owner action is to reconcile tab visibility and the disabled-native-payment state, then add focused coverage for the settings tuple and subscription-tab interaction before this surface is accepted. This lane made no repair.

## Protected-test integrity

No protected-test baseline hashes were supplied to or independently verified by this lane. The related `PaymentView` test was statically inspected as cited above; no test runner was invoked. Parent-owned protected-test and machine evidence remains separate and unmodified.

## Requirements and evidence traceability

No acceptance-contract or `AC-*` identifiers were supplied in this lane packet. This lane does not invent an AC mapping or change acceptance projections. The finding applies to the candidate's user-facing purchase-flow behavior under the explicit native-disabled/store-enabled settings tuple.

## Machine verification

| Layer | Result | Scope and evidence |
| --- | --- | --- |
| Static source review | `FAIL` | M-01 is reproducible from the frozen source and candidate patch. |
| Unit tests | `NOT RUN` | Explicitly excluded from this lane; parent is running them. |
| Integration/contract tests | `NOT RUN` | No commands were run by this lane. |
| Critical E2E/visual checks | `NOT RUN` | No browser or runtime access was authorized. |
| Docker artifact/build verification | `NOT RUN` | Parent-owned artifact evidence was not duplicated or reclassified. |
| Non-functional checks | `NOT RUN` | Outside this bounded static lane. |

## Engineering review

The bounded review found M-01 in the payment surface. No additional reader-facing regression or incorrect claim was reported without direct source evidence. Backend transaction behavior, dependency installation, image publication, and runtime observability were not reviewed in this lane.

## Human acceptance

Human acceptance remains pending and is not a pass. This lane performed no human checklist execution, live purchase, redemption, sales approval, or production approval. Machine or static evidence cannot substitute for those activities.

## External sandbox and runtime evidence

None. No external sandbox, production account, credential, network request, or runtime side effect was used.

## Monitoring and rollback

Not assessed by this lane. Monitoring thresholds, rollback readiness, and release-owner closeout remain parent/release-owner responsibilities and are not implied by this static review.

## Unverified and explicitly excluded points

- `frontend/prerender.config.ts:322` contains a hardcoded `ONLINE` value in the base tree; it was not treated as a candidate regression.
- Tutorial URLs, installation commands, and real Windows/WSL execution were not externally verified here; no incorrect-claim finding is made from that absence alone.
- Protected-test hashes, parent build/test results, authoritative remote tip, Docker artifact identity, human signatures, live purchase behavior, production behavior, and sales enablement were not independently verified by this lane.

## Proposed result

- `proposed_lane_result`: `FAILED`
- `equivalent_release_state`: `NOT READY`
- `proven_scope`: static review of the pinned candidate tree and exact patch for reader-facing frontend regressions and incorrect claims
- `failure_reason`: confirmed M-01 in the purchase flow
- `conditions`: repair M-01, add focused tab-state regression coverage, and perform a fresh bounded re-review; separately complete the parent-owned machine, artifact, human, and live-commerce gates
- `final_release_READY`: not declared
