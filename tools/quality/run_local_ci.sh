#!/usr/bin/env bash
# Run the committed candidate in a private snapshot; never modify HMR dependencies.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT"
EVIDENCE_DIR="${1:-agents-results/$(date +%F)/local-ci/acceptance/$(date -u +%Y%m%dT%H%M%SZ)-$$}"
mkdir -p "$EVIDENCE_DIR"
EVIDENCE_DIR="$(cd "$EVIDENCE_DIR" && pwd)"
if [[ -e "$EVIDENCE_DIR/stages.tsv" || -e "$EVIDENCE_DIR/summary.json" ]]; then
  echo "Use a new evidence directory: $EVIDENCE_DIR" >&2
  exit 2
fi
printf 'stage\texit_code\tseconds\tlog\n' > "$EVIDENCE_DIR/stages.tsv"
COMMIT="$(git rev-parse HEAD)"
TREE="$(git rev-parse HEAD^{tree})"
STARTED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
SNAPSHOT=""
STAGES=(source-preflight snapshot toolchain frozen-install acceptance-layout deploy-guards version-guards local-ci-guards commerce-parity-guards backend-unit backend-integration backend-lint frontend-lint frontend-typecheck frontend-tests frontend-build backend-security frontend-security)

finish() {
  local code=$?
  trap - EXIT
  python3 - "$EVIDENCE_DIR" "$COMMIT" "$TREE" "$STARTED_AT" "$code" "${STAGES[@]}" <<'PY'
import csv, datetime, json, pathlib, sys
root, commit, tree, started, code, *planned = sys.argv[1:]
root = pathlib.Path(root)
with (root / 'stages.tsv').open() as stream:
    rows = list(csv.DictReader(stream, delimiter='\t'))
for row in rows:
    row['exit_code'] = int(row['exit_code'])
    row['seconds'] = int(row['seconds'])
    row['status'] = 'PASS' if row['exit_code'] == 0 else 'FAIL'
executed = {row['stage'] for row in rows}
rows.extend({'stage': name, 'status': 'NOT_RUN', 'exit_code': None} for name in planned if name not in executed)
summary = dict(commit=commit, tree=tree, started_at=started,
               finished_at=datetime.datetime.now(datetime.timezone.utc).isoformat(),
               exit_code=int(code), status='PASS' if code == '0' else 'FAIL', stages=rows)
tmp = root / 'summary.json.tmp'
tmp.write_text(json.dumps(summary, indent=2) + '\n')
tmp.replace(root / 'summary.json')
PY
  [[ -z "$SNAPSHOT" ]] || rm -rf "$SNAPSHOT"
  printf 'Local CI exit %s; evidence: %s/summary.json\n' "$code" "$EVIDENCE_DIR"
  exit "$code"
}
trap finish EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

run_stage() {
  local name="$1" start=$SECONDS result
  shift
  printf 'Running %s\n' "$name"
  set +e
  (set -euo pipefail; "$@") > "$EVIDENCE_DIR/$name.log" 2>&1
  result=$?
  set -e
  printf '%s\t%s\t%s\t%s\n' "$name" "$result" "$((SECONDS-start))" "$name.log" >> "$EVIDENCE_DIR/stages.tsv"
  if [[ "$result" -ne 0 ]]; then
    tail -n 40 "$EVIDENCE_DIR/$name.log"
    return "$result"
  fi
}

source_preflight() {
  # Explicit existing evidence exclusions; other acceptance/source changes still fail.
  local exclusions=(
    ':(exclude)agents-results'
    ':(exclude)acceptance/human-acceptance-log.json'
    ':(exclude)acceptance/human-acceptance-log.md'
    ':(exclude)acceptance/human/2026-W37'
  )
  git diff --exit-code HEAD -- . "${exclusions[@]}" >/dev/null || { echo 'Commit source changes before running local CI.' >&2; return 1; }
  local extra
  extra="$(git ls-files --others --exclude-standard -- . "${exclusions[@]}")"
  [[ -z "$extra" ]] || { printf 'Uncommitted files:\n%s\n' "$extra" >&2; return 1; }
}
create_snapshot() {
  git archive "$COMMIT" | tar -x -C "$SNAPSHOT"
}
preflight_toolchain() {
  command -v node
  [[ "$(node -p 'process.versions.node.split(".")[0]')" == 24 ]] || {
    echo 'Local CI requires Node 24.' >&2; return 1;
  }
  node --version
  [[ "$(pnpm --version)" == 9.15.9 ]] || {
    echo 'Local CI requires pnpm 9.15.9.' >&2; return 1;
  }
  pnpm --version
  local expected actual
  expected="$(awk '$1 == "go" {print $2; exit}' backend/go.mod)"
  actual="$(cd backend && go env GOVERSION)"
  [[ "$actual" == "go$expected" ]] || {
    printf 'Local CI requires Go %s, got %s.\n' "$expected" "$actual" >&2; return 1;
  }
  printf '%s\n' "$actual"
  local lint_version
  lint_version="$(golangci-lint version)"
  printf '%s\n' "$lint_version"
  [[ "$lint_version" =~ version[[:space:]]2\.13(\.|[[:space:]]) ]] || {
    echo 'Local CI requires golangci-lint 2.13.x.' >&2; return 1;
  }
  govulncheck -version
  docker info --format '{{.ServerVersion}}'
}
deploy_guards() {
  bash -n deploy/apple-container.sh
  bash deploy/tests/apple-container-test.sh
  sh deploy/tests/docker-compose-security-test.sh
  sh deploy/tests/docker-compose-gateway-env-test.sh
  sh deploy/tests/docker-runtime-resources-test.sh
  sh deploy/test-caddyfile-cache.sh
}
frontend_security() {
  local audit_code=0
  pnpm --dir frontend audit --prod --audit-level=high --json > "$EVIDENCE_DIR/pnpm-audit.json" || audit_code=$?
  printf 'pnpm audit exit: %s\n' "$audit_code"
  # Exceptions apply only to a complete report with a consistent audit exit status.
  python3 tools/check_pnpm_audit_exceptions.py --audit "$EVIDENCE_DIR/pnpm-audit.json" --exceptions .github/audit-exceptions.yml --audit-exit-code "$audit_code"
}

if [[ -n "${CI_NODE_BIN_DIR:-}" ]]; then
  export PATH="$CI_NODE_BIN_DIR:$PATH"
fi
export CI=true GOMAXPROCS="${CI_GO_PROCS:-2}"
export GOFLAGS="-p=1"
export VITEST_MAX_THREADS=2 VITEST_MIN_THREADS=1
if [[ -z "${HARNESS_ENGINEERING_HOME:-}" && -d "$REPO_ROOT/../Harness_Engineering" ]]; then
  export HARNESS_ENGINEERING_HOME="$(cd "$REPO_ROOT/../Harness_Engineering" && pwd)"
fi
run_stage source-preflight source_preflight
SNAPSHOT="$(mktemp -d "${TMPDIR:-/tmp}/sub2api-local-ci.XXXXXX")"
run_stage snapshot create_snapshot
# A private shim also pins recursive `pnpm run` calls without changing global settings.
if [[ "$(pnpm --version 2>/dev/null || true)" != 9.15.9 ]]; then
  mkdir "$SNAPSHOT/.ci-bin"
  cat > "$SNAPSHOT/.ci-bin/pnpm" <<'SHIM'
#!/usr/bin/env bash
exec corepack pnpm@9.15.9 "$@"
SHIM
  chmod +x "$SNAPSHOT/.ci-bin/pnpm"
  export PATH="$SNAPSHOT/.ci-bin:$PATH"
fi
cd "$SNAPSHOT"
run_stage toolchain preflight_toolchain
run_stage frozen-install pnpm --dir frontend install --frozen-lockfile
# The central checker needs the real checkout's Git tracking and evidence paths.
run_stage acceptance-layout bash "$REPO_ROOT/tools/quality/run_acceptance_artifact_layout_guard.sh"
run_stage deploy-guards deploy_guards
run_stage version-guards python3 tools/quality/test_version_scripts.py
run_stage local-ci-guards python3 tools/quality/test_local_ci.py
run_stage commerce-parity-guards python3 -m unittest discover -s tools/quality/tests
run_stage backend-unit make -C backend test-unit
run_stage backend-integration make -C backend test-integration
run_stage backend-lint bash -c 'cd backend && golangci-lint run --timeout=30m --concurrency=2 ./...'
run_stage frontend-lint pnpm --dir frontend run lint:check
run_stage frontend-typecheck pnpm --dir frontend run typecheck
run_stage frontend-tests pnpm --dir frontend exec vitest run --maxWorkers=2 --minWorkers=1
run_stage frontend-build pnpm --dir frontend run build
run_stage backend-security bash -c 'cd backend && govulncheck ./...'
run_stage frontend-security frontend_security
