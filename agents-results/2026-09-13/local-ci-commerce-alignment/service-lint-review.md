# Service lint correction review

Scope: service fixes from the candidate-3 lint failure and the additional same-category findings revealed by an untruncated service lint run. No commit, deployment, credentials, production access or full CI run by this lane.

## Changes

- `liandong_refund.go` and `liandong_restock_core.go`: lowercase error text to satisfy ST1005 while preserving sentinel identities and return paths.
- `liandong_restock_core.go`: HMAC input remains exactly `%s:%d` (batch ID and ordinal), now written with `fmt.Fprintf`; HMAC hash writes are infallible under hash.Hash's contract. Removed the unused `deriveCodes` wrapper: no production or test caller remains; all callers use the error-returning `deriveCodesChecked`.
- Code exports and `openai_compact_body_signal.go` assemble the same raw bytes using slice append, eliminating ignored bytes.Buffer write return values. Compaction does not decode/reencode large numeric values.
- `liandong_tool_runtime.go`: equivalent ASCII-range boolean expression; allowed characters and first-character requirement are unchanged.
- `liandong_restock_core.go`, `payment_order_lifecycle.go`, and `payment_refund.go`: deferred SQL row cleanup joins its error into the named returned error, preserving earlier scan/query errors.
- `proxy_subscription_service.go`: HTTP body Close errors are checked and reported with a static diagnostic; cleanup does not reinterpret the result of an already-applied controller request or leak response/credential text.
- `liandong_refund_test.go` and `liandong_restock_persistence_test.go`: explicit expected sqlmock DB.Close calls with checked cleanup and final mock verification. Export reader cleanup is also checked.
- `proxy_subscription_service_test.go`: checked YAML assertions; MIHOMO_BIN resolution through exec.LookPath, absolute/canonical path resolution, regular-file and executable validation before invoking fixed argv with no shell.

## G702 boundary

The taint analyzer still flags the two optional Mihomo integration test launches after executable validation. Exactly these two call sites have a G702 annotation explaining that MIHOMO_BIN is selected by the test operator, is resolved by `validatedMihomoTestBinary`, and cannot be controlled by subscription content. All config paths remain separate argv values and no shell is invoked. No global linter rule or unchecked arbitrary command exception was added. The validator has regression cases for directory, command-text/missing-file and nonexecutable rejection, and for successful canonical executable resolution without executing the fixture.

## Verification

The first untruncated service run (`--max-issues-per-linter=0 --max-same-issues=0`) exposed 65 findings, including rows/body cleanup and additional capitalized Liandong errors hidden by the candidate log's display caps. This lane corrected its assigned files; the inventory lane owns `liandong_restock_service.go` and `liandong_restock_core_test.go`.

The full-backend lint run collected all lanes with:

```bash
GOMAXPROCS=2 GOFLAGS=-p=1 /tmp/sub2api-local-ci-tools-20260913/bin/golangci-lint run --timeout=30m --concurrency=2 --max-issues-per-linter=0 --max-same-issues=0 ./...
```

Working directory: backend. Complete output: `acceptance/service-lint-followup.log`. Result: exit 1 with exactly two remaining repository findings (`internal/repository/user_lifecycle_repo.go:56` and `:76`, unchecked SQL Rows.Close). The service scope is clear; these two repository findings have been sent to the parent for its repository owner. Both local G702 justifications were accepted.

`gofmt` and `git diff --check` pass. New focused assertions cover unchanged raw compaction bytes, large-integer precision, replay idempotence, and validated executable selection. Final full local CI's unit-test stage is assigned to the parent and will execute these assertions; this lane avoids a redundant concurrent service compile while that run starts. No claim of post-change unit-test completion is made here until that evidence exists.


## Final lint result

After the parent corrected the two repository row-close findings and froze all product source, the same untruncated full-backend command returned exit 0 with `0 issues.`. Complete final output: `acceptance/service-lint-final.log`. The prior `service-lint-followup.log` is retained as the intermediate failing evidence.

This establishes a clean full-backend lint result for all current lanes. Final CI unit/integration/frontend/security execution and its source commit binding remain the parent's next step; no final CI or unit-test success is inferred from lint alone.
