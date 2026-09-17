# Membership and route coverage lint repair

Status: scoped repair and tests PASS. Full candidate CI remains owned by the root task.

The red baseline is `acceptance/local-ci-candidate-3/backend-lint.log`. The linter's default repeated-issue cap concealed more identical unchecked cleanup calls; all instances within the assigned membership package were fixed.

## Source changes

- Added `backend/internal/membership/cleanup.go`: resource Close failures and unexpected transaction Rollback failures are handled with fixed sanitized log messages. Expected `sql.ErrTxDone` after commit/rollback is ignored. Cleanup remains in the original deferred or immediate position and never changes the authoritative operation return value.
- Updated `admin.go`, `browser.go`, `engine.go`, `payment.go`, and `worker.go` to use those cleanup helpers. Advisory unlock SQL, background context, and unlock-before-connection-close order remain unchanged; unexpected unlock failures now produce fixed sanitized diagnostics.
- `engine.go`: equivalent tagged credential-mode switch and equivalent De Morgan character predicate. All 1,114,112 Unicode code points were compared against the old predicate; no difference found.
- `worker.go`: equivalent tagged fulfillment-state switch. The later `NotSubmitted` override still wins and sets the reserved state.
- `secrets.go`: explicitly acknowledges `hash.Hash.Write`'s guaranteed success while writing exactly the original `purpose + NUL + text` bytes. No hash input, key, encoding, or return format changed.
- `membership_test.go`: close helper declares one sqlmock Close expectation per actual open pooled connection, checks Close, and checks remaining expectations. Existing behavior assertions are preserved. Worker tests reserve one connection while transactions need another, so a single Close expectation is insufficient.
- `backend/internal/server/routes/handler_route_coverage_test.go`: explicitly checks the strings.Builder write result; route source bytes and reachability assertions remain unchanged.

## Verification

Commands run from `backend/` unless noted:

- `GOMAXPROCS=2 go test -p 1 -tags=unit ./internal/membership ./internal/server/routes -count=1`: PASS. Final evidence: `acceptance/membership-lint-fix-tests-2.log`.
- `/tmp/sub2api-local-ci-tools-20260913/bin/golangci-lint run --timeout=10m --concurrency=2 ./internal/membership`: PASS, `0 issues.` Evidence: `acceptance/membership-lint-fix-lint.log`. This ran before the final pooled-connection expectation refinement in the test cleanup helper.
- A subsequent two-package lint attempt was rejected by the linter's concurrency lock while the service lane was running lint. Evidence: `acceptance/membership-routes-lint-fix-lint.log`. It is not a PASS; no lock override or linter disabling was used. Root will run whole-backend lint after all lanes freeze.
- `gofmt -l` across the nine scoped files: no output.
- `git diff --check`: PASS.
- The intermediate test log `acceptance/membership-lint-fix-tests.log` preserves the initial one-Close-expectation fixture failure and the original route package PASS; the corrected final test log is authoritative for this repair.

No linter settings or suppressions, product transactions, production/local services, migrations, hash bytes, or state transitions were changed. No commit, full CI, build, or deployment was performed by this lane.
