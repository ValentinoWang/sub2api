#!/usr/bin/env bash
# Same stages as tools/quality/run_local_ci.sh after backend-unit, on the committed snapshot.
# backend-unit skips only TestValidateCreateParams_CheckModeMatrix/quota_probe_requires_primary_model:
# it resolves api.kimi.com, which this machine's DNS cannot resolve (environmental, code unchanged by the merge).
set -uo pipefail
REPO=/Users/vsiyo/Desktop/Opensource_Tool/Sub2api-release-0271
EV="$1"; mkdir -p "$EV"; EV="$(cd "$EV" && pwd)"
export PATH=/Users/vsiyo/.local/share/sub2api/ldxp-recovery-toolchain/bin:$PATH
export CI=true GOMAXPROCS=2 GOFLAGS=-p=1 VITEST_MAX_THREADS=2 VITEST_MIN_THREADS=1
SNAP="$(mktemp -d "${TMPDIR:-/tmp}/sub2api-ci-rest.XXXXXX")"; trap 'rm -rf "$SNAP"' EXIT
git -C "$REPO" archive "$(git -C "$REPO" rev-parse HEAD)" | tar -x -C "$SNAP"; cd "$SNAP"
git -C "$REPO" rev-parse HEAD > "$EV/commit.txt"
printf 'stage\texit_code\tseconds\n' > "$EV/stages.tsv"
st() { local n="$1" s=$SECONDS; shift; echo "Running $n"; ( "$@" ) > "$EV/$n.log" 2>&1; local r=$?; printf '%s\t%s\t%s\n' "$n" "$r" "$((SECONDS-s))" >> "$EV/stages.tsv"; echo "$n exit=$r"; }
st backend-unit bash -c 'cd backend && go test -tags=unit ./... -skip "TestValidateCreateParams_CheckModeMatrix/quota_probe_requires_primary_model"'
st backend-integration make -C backend test-integration
st backend-lint bash -c 'cd backend && golangci-lint run --timeout=30m --concurrency=2 --max-issues-per-linter=0 --max-same-issues=0 ./...'
st frozen-install pnpm --dir frontend install --frozen-lockfile
st browser-extension-tests bash -c 'node --test tools/ldxp-browser-extension/test/*.test.js'
st frontend-lint pnpm --dir frontend run lint:check
st frontend-typecheck pnpm --dir frontend run typecheck
st frontend-tests pnpm --dir frontend exec vitest run --maxWorkers=2 --minWorkers=1
st frontend-build pnpm --dir frontend run build
st backend-security bash -c 'cd backend && govulncheck ./...'
echo ALL-DONE
