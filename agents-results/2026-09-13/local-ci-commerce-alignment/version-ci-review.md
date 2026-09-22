# Version and local CI review

Date: 2026-09-13. Scope: local CI and release identity. This is an implementation/evidence report, not an SSOT declaration or human acceptance.

## Changes

- `AGENTS.md`: replaces the fixed public `0.2.1`/three-part rule with the current user instruction: `0.2.4.1`, `0.2.4.2`, etc. until upstream officially releases `0.2.4`. Local CI is the promotion authority; GitHub Actions/billing is not a prerequisite.
- `backend/cmd/server/VERSION`: next candidate is `0.2.4.1`. No historical image, release record, Git tag, or deployed binary was modified.
- `deploy/build_image.sh`: permits canonical numeric three- and four-part versions while retaining `git archive` of the selected commit, version-plus-commit image tags, full commit build arguments and OCI revision labels. Working-tree bytes never enter the context.
- `backend/scripts/resolve-version.sh`: accepts valid three-/four-part exact tags; ignores nonrelease tags; rejects invalid fallback VERSION files.
- `backend/Makefile`: integration tests use `go test -p 1 -tags=integration ./...`, preserving all packages/tests and avoiding concurrent package creation of the shared testcontainers reaper.
- `tools/quality/run_local_ci.sh`: one serial local CI entry; isolated committed snapshot, private pnpm 9.15.9 shim when required, frozen install, Node 24 and declared Go toolchain checks, resource controls (`GOMAXPROCS=2`, Go package parallelism 1, linter concurrency 2, Vitest workers 1–2), per-stage logs/exit codes and durable JSON summary. Existing directories containing summary/stage records cannot be overwritten.
- `tools/quality/test_version_scripts.py`: seven deterministic tests using temporary Git repositories and a mock Docker executable; no actual Docker daemon/image/container mutations.
- No frontend display-version constant or `0.2.x` literal exists in `frontend/src`; no unused display constant was introduced.

## CI stages and command

```bash
CI_NODE_BIN_DIR=/path/to/node24/bin \
  bash tools/quality/run_local_ci.sh \
  agents-results/2026-09-13/local-ci-commerce-alignment/acceptance/local-ci-candidate
```

Run after the candidate source commit. The script rejects tracked source modifications and untracked source except for explicit evidence paths: `agents-results/`, `acceptance/human-acceptance-log.json`, `acceptance/human-acceptance-log.md`, and the pre-existing `acceptance/human/2026-W37/`. Other acceptance/source changes still fail, preventing validation of a stale source commit. It archives HEAD into a private temporary snapshot and never installs into or builds over the live HMR workspace. Required tools must be in PATH; the optional `CI_NODE_BIN_DIR` only affects this process. `HARNESS_ENGINEERING_HOME` may identify the central acceptance checker; the persistent sibling Harness checkout is auto-resolved when present.

Stages: source preflight; snapshot; toolchain; frozen install; acceptance layout; deployment shell guards; version guards; Go unit; serialized Go integration; golangci-lint; frontend lint; frontend typecheck; full Vitest; frontend production build; govulncheck; pnpm production audit with existing documented exception checker. No duplicate untagged backend suite, xui-sales suite, or Codex migration suite is added. A failed stage aborts subsequent execution and records them as `NOT_RUN`. Security-audit transport/error responses cannot become a false successful empty report.

The acceptance-layout check runs read-only against the original checkout because its central checker needs Git tracking and project evidence paths. All source compilation, dependency installation, tests, lint and security checks run in the isolated snapshot. `summary.json` binds the source commit/tree, start/end times, exit status, and stage logs. Integration tests require Docker for isolated fixtures; this script does not restart application services or access production.

## Focused verification

- RED: ran the new committed-image provenance/isolation test with the original HEAD versions of both build scripts in a temporary fixture. It failed exactly with `Invalid technical version: 0.2.4.1` before Docker invocation.
- GREEN: `python3 tools/quality/test_version_scripts.py` — 7 tests passed. Covers four-part file/tag, historical three-part tag/build, nonrelease tag fallback, invalid VERSION rejection, and committed version/content/provenance even when the working VERSION/content are dirty and an untracked file exists.
- `bash -n tools/quality/run_local_ci.sh deploy/build_image.sh` — passed.
- `sh -n backend/scripts/resolve-version.sh` — passed.
- `make -n -C backend test-integration` — prints `go test -p 1 -tags=integration ./...`.
- `git diff --check` — passed at scoped verification time.
- Dirty-source CI negative case: `bash tools/quality/run_local_ci.sh agents-results/2026-09-13/local-ci-commerce-alignment/acceptance/local-ci-dirty-source-guard` exited 1 at `source-preflight`, before snapshot/install. Durable result: `acceptance/local-ci-dirty-source-guard/summary.json`; every following stage is `NOT_RUN`.

## Remaining execution boundary

Full local CI and actual Docker build are assigned to the parent task and were not run in this lane. At initial inspection this shell resolved Node 22.22.2, pnpm 11.19.0, Go 1.26.5 and no golangci-lint; the runner requires Node 24, pnpm 9.15.9, the Go version declared by `backend/go.mod`, golangci-lint and govulncheck. Current toolchain installation/provisioning and full-stage results must be reported by the parent. No commit, push, GitHub account change, Docker runtime change, credentials, or production access occurred in this lane.


## Follow-up: task-private tools and explicit evidence exclusions

The runner now accepts only the named existing acceptance evidence paths above, with no broad acceptance exclusion. `tools/quality/test_local_ci.py` verifies four cases: named evidence passes source preflight; dirty source fails; an unrelated acceptance README fails; pnpm 11 is replaced by the private Corepack shim invoking exactly `pnpm@9.15.9 --version`. All four pass. Mock toolchain failure stops these fixtures before dependency installation, Docker access or full application CI.

The stage list includes `local-ci-guards` (`python3 tools/quality/test_local_ci.py`) and `commerce-parity-guards` (`python3 -m unittest discover -s tools/quality/tests`). This discovery includes commerce parity and runtime collector guards as supplied by the parallel implementation lane. Linter preflight requires version 2.13.x.

Task-private tool directory: `/tmp/sub2api-local-ci-tools-20260913`. No user-global Node, pnpm activation, Go binary directory, shell profile or Codex configuration was changed. The pinned govulncheck build reuses the standard Go module/build cache; its executable lives only in this task directory.

Verified binaries:

- Node 24.21.0, official `node-v24.21.0-darwin-arm64.tar.gz`; SHA-256 `bed7eea5325e1108f32ce5228ddd6a5f0f08a499ee42aa7442aea583702f6057`, matching the official Node SHASUMS256 entry.
- golangci-lint 2.13.0, official `golangci-lint-2.13.0-darwin-arm64.tar.gz`; SHA-256 `72eae670097978e61b78a773a25c27439e35d54094fd986cee5eeeb25d7144fd`, matching the official release checksums. Reports built with go1.27.0 from f838df1e.
- govulncheck 1.8.0, installed with `GOMAXPROCS=2 GOBIN=/tmp/sub2api-local-ci-tools-20260913/bin go install -p=1 golang.org/x/vuln/cmd/govulncheck@v1.8.0` from backend directory.
- pnpm 9.15.9 verified through Corepack with `COREPACK_HOME` in the task directory.
- Backend Go resolves go1.27.0 via the existing auto toolchain configuration; no global Go configuration was changed.

Exact launch after all candidate source changes are committed:

```bash
source /tmp/sub2api-local-ci-tools-20260913/env.sh
bash tools/quality/run_local_ci.sh agents-results/2026-09-13/local-ci-commerce-alignment/acceptance/local-ci-candidate
```

Equivalent inline tool selection:

```bash
CI_NODE_BIN_DIR=/tmp/sub2api-local-ci-tools-20260913/node-v24.21.0-darwin-arm64/bin \
COREPACK_HOME=/tmp/sub2api-local-ci-tools-20260913/corepack \
PATH="/tmp/sub2api-local-ci-tools-20260913/bin:/tmp/sub2api-local-ci-tools-20260913/node-v24.21.0-darwin-arm64/bin:$PATH" \
bash tools/quality/run_local_ci.sh agents-results/2026-09-13/local-ci-commerce-alignment/acceptance/local-ci-candidate
```

Full local CI remains deliberately unexecuted in this lane until the parent combines the refund and parity changes. The task-private directory can be removed after delivery when no CI process is using it; do not remove the shared Go cache or active application dependencies as part of that cleanup.


## Review hardening: test filters and audit report integrity

- `run_local_ci.sh` now exports fixed `GOFLAGS=-p=1`, replacing inherited flags rather than appending to them. This environment value also takes precedence over persisted Go environment defaults. A fixture supplies `-run=^$ -skip=.* -short` and verifies the invoked Go tool sees only `-p=1`.
- `tools/check_pnpm_audit_exceptions.py` validates audit shape before matching exceptions: exactly one supported findings object, complete nonnegative integer severity counts, a consistent optional total, valid package/severity/advisory details, and equality between reported counts and finding entries. Metadata-only reports, high findings with empty `via`, unknown/missing details, invalid totals and unresolved dependency entries fail closed.
- Local CI passes the actual pnpm audit exit code to the checker. Only exit 0 with no high/critical findings or exit 1 with high/critical findings can proceed to exception evaluation. Error responses and inconsistent statuses fail.
- The checker retains the existing exception policy; no dependency, exception expiry or allowed advisory was changed. The existing `frontend/audit.json` validates successfully with the existing exception file and audit exit 1.
- RED: the original HEAD audit checker reproduced two false passes: metadata-only high count and high vulnerability `via: []`. The previous inherited-GOFLAGS assignment reproduced the retained filter flags. All three new guards failed on those old implementations.
- GREEN: `PYTHONDONTWRITEBYTECODE=1 python3 tools/quality/test_local_ci.py` passes 5 tests; `PYTHONDONTWRITEBYTECODE=1 python3 tools/quality/tests/test_pnpm_audit_exceptions.py` passes 11 tests. The security tests are automatically included in the existing `commerce-parity-guards` unittest discovery. Shell syntax and whitespace checks pass. No full builds or complete CI run occurred in this lane.


## Candidate CI correction: explicit toolchain failures on macOS Bash

The first candidate run (`b8a0aabf8`) stopped at `local-ci-guards`; three fixture expectations incorrectly appeared to fail only in the complete runtime environment. Live reproduction showed the fixture's `CI_NODE_BIN_DIR` correctly selected its mock node and Go. The root cause was the production runner's reliance on `set -e` for bare `[[ ... ]]` comparisons under macOS Bash 3.2.57: a false conditional inside a function did not abort it, so later successful linter/scanner/Docker commands could turn a version mismatch into a successful stage. Earlier tests failed downstream because some real tools were unavailable, which masked the invalid guard behavior.

`run_local_ci.sh` now returns failure explicitly, with a diagnostic, on each Node, pnpm, Go and golangci-lint version mismatch. `test_local_ci.py` keeps the original stage assertions and adds sentinel mock executables so no fixture can fall through into real package-manager, linter, scanner or Docker operations. A table test independently mismatches all four tool versions, verifies the exact diagnostic, and requires the install stage to remain `NOT_RUN`.

RED: all four mismatch subcases fail against the original candidate runner. GREEN: after sourcing `/tmp/sub2api-local-ci-tools-20260913/env.sh`, `PYTHONDONTWRITEBYTECODE=1 python3 tools/quality/test_local_ci.py` passes six tests, including the four-case table. Shell syntax and `git diff --check` pass. No commit or full CI/build was executed by this correction lane; the parent owns the next candidate run.
