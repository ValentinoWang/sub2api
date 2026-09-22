# Local CI fixture review

Status: fixture repair and focused verification PASS. This is machine evidence, not production or human acceptance.

## Changes

- `backend/internal/repository/auth_identity_legacy_migration_integration_test.go`: reset only the six auth fixture tables with explicit `TRUNCATE ... RESTART IDENTITY`, without `users`, unrelated profile/grant tables, or `CASCADE`. The enclosing test transaction still rolls back all fixture data. Existing migration assertions remain unchanged and retain exact user/identity/report keys.
- Added `TestAuthIdentityLegacyFixtureCleanupPreservesMembershipAudit`: seed a related user, membership order, and audit event; run auth fixture cleanup; require the complete relationship and original event action to remain. This protects the cleanup boundary with populated unrelated data.
- `backend/internal/repository/api_key_cache_subscriber_test.go`: wait for Redis `PUBSUB NUMSUB` to acknowledge the subscription, publish exactly once, explicitly require receipt, then retain the existing active-subscription and context-cancellation assertions. Always cancel the context during cleanup. The previous `Eventually` repeatedly published into a single-slot callback channel, allowing the test's synchronous handler to block on excess messages. No production subscriber change is required.
- `backend/internal/middleware/rate_limiter_integration_test.go`: unchanged. The reaper conflict arises because testcontainers-go v0.40.0 derives one session ID from the parent `go test` process, while its reaper creation mutex is process-local. Multiple test packages can simultaneously attempt the same reaper name. Parent delegated `backend/Makefile` to the other implementation lane for package-serial integration execution using `-p 1`; this lane verified both affected integration packages with that execution mode. No reaper disabling, Docker deletion, or test skipping was introduced.

## Verification

All Go commands below ran from `backend/`. PostgreSQL and real Redis integration tests used testcontainers with dynamically mapped ports and fresh isolated data. The subscriber stress test used miniredis. No local application database, production database, service restart, or credentials were used.

| Check | Command | Result | Evidence |
| --- | --- | --- | --- |
| Original legacy failure | `go test -p 1 -tags=integration ./internal/repository -run '^TestAuthIdentityLegacyExternalBackfillMigration$' -count=1` | Expected FAIL: membership audit events are append-only | [legacy-fixture-red.log](acceptance/legacy-fixture-red.log) |
| New preservation regression with old helper | `go test -p 1 -tags=integration -overlay=<temporary-overlay.json> ./internal/repository -run '^TestAuthIdentityLegacyFixtureCleanupPreservesMembershipAudit$' -count=1` | Expected FAIL: old `users ... CASCADE` reaches protected audit table | [fixture-preservation-red.log](acceptance/fixture-preservation-red.log) |
| Complete affected integration packages | `go test -p 1 -tags=integration ./internal/repository ./internal/middleware -count=1` | PASS: repository 82.391s, middleware 24.214s | [fixture-packages-green.log](acceptance/fixture-packages-green.log) |
| Preservation plus existing audit guards | `go test -p 1 -tags=integration ./internal/repository -run '^(TestAuthIdentityLegacyFixtureCleanupPreservesMembershipAudit\|TestRest2BuildProductSchemaAppliesAndEnforcesMembershipInvariants)$' -count=1` | PASS; existing membership test continues to reject UPDATE, DELETE, and TRUNCATE of audit events | [fixture-preservation-green.log](acceptance/fixture-preservation-green.log) |
| Subscriber stress and race detector | `go test -race ./internal/repository -run '^TestAPIKeyCacheSubscriber_BlocksUntilContextCancellation$' -count=100` | PASS, 100 repetitions, no detected race | [subscriber-green.log](acceptance/subscriber-green.log) |
| Diff hygiene | `git diff --check` | PASS | Executed after final source changes |

The old subscriber test passed 30 isolated repetitions in this run ([subscriber-red.log](acceptance/subscriber-red.log)); it is an intermittent failure, not claimed to be newly reproduced. Its actual failure is preserved in the prior full CI log, [backend-integration.log](../../2026-09-12/dev-main-docker-alignment/acceptance/backend-integration.log). The source-level callback backpressure mechanism explains why single-package runs can pass while loaded full CI fails.

The temporary red overlay retained the new regression test while replacing only `truncateAuthIdentityLegacyFixtureTables` with its HEAD implementation. It did not modify working-tree production code or weaken audit protection.

## Verified source hashes

- `backend/internal/repository/api_key_cache_subscriber_test.go`: `2b4f018e51680e9d6100cec1b84d81f20a7bfd18c95c617c9a4e158eb10de9ea`
- `backend/internal/repository/auth_identity_legacy_migration_integration_test.go`: `74fcf367c5964d431a5f6dcc590a5dceea4e2377a9c31f64613927bdec52f772`
