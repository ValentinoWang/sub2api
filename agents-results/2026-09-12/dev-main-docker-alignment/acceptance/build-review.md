# Acceptance Release Review: build-release-review

- Task ID: `build-release-review`
- Direct parent: `dev-main-docker-alignment`
- Review route: `lw-luna`
- Role: independent read-only review; no descendants
- Scope: Git dev/main promotion and Docker artifact verification only
- Explicit exclusions: no product repair, test repair, Git mutation, contract/settings/runtime mutation, credential access, production access, external service access, live sales approval, or human purchase acceptance

## Source and Evidence Identity

- Base: `20189b7348fa74020e066241890c240e0c28cad4`
- Frozen tree: `72b968baf05eb473297dfa4832890c0702915e85`
- Frozen candidate checkout: `/var/folders/w0/td0lhml95j374vr26ypgjzfh0000gn/T/sub2api-candidate-8qjoq7ad`
- Candidate commit recorded by the parent lane: `baae1ae6bd5595cbccf5b7859bf85c2262b660fe`
- Exact diff: `agents-results/2026-09-12/dev-main-docker-alignment/acceptance/candidate.patch`
- Exact diff SHA-256: `df84899b52f33f57d91751124964dfa1bfcfd207919871d5bfefdc2244b719cc`
- Source identity record: `agents-results/2026-09-12/dev-main-docker-alignment/acceptance/source-identity.json`
- Review timestamp: `2026-09-12` Asia/Shanghai; no production or remote runtime was used

## Actual Read and Write Scope

Read:

- The exact candidate patch and source identity record.
- Frozen `Dockerfile`, `Dockerfile.goreleaser`, `.dockerignore`, `backend/cmd/server/VERSION`, `deploy/build_image.sh`, `deploy/docker-compose.dev.yml`, and `deploy/docker-compose.local.yml`.
- Frozen `.github/workflows/backend-ci.yml`, `.github/workflows/release.yml`, `.github/workflows/security-scan.yml`, `.goreleaser.yaml`, `.goreleaser.simple.yaml`, and the directly referenced audit exception/check files.
- Frozen `frontend/package.json`, `frontend/pnpm-lock.yaml`, `frontend/vite.config.ts`, and `frontend/prerender.config.ts`.
- Only referenced source needed to validate the Docker contracts: the legal Markdown imports, LDXP runtime configuration/defaults/status checks, and the unchanged `AppLayout.theme.spec.ts` assertion against the changed `AppLayout.vue` expression.
- Bounded lane evidence: `frozen-install.log`, `frozen-frontend-build.log`, `frozen-frontend-build-pinned.log`, `frozen-vite-build.log`, `frozen-types.log`, `frozen-lint.log`, `frozen-vitest.log`, `backend-unit.log`, `backend-integration.log`, `shell-tests.log`, `github-ci-blocker.json`, `build-scheduling.md`, `frontend-audit.json`, and `worker-ledger.json`.
- Newly appearing `docker-build.log`, `promotion.json`, and `hmr-module.json` were inspected only for source identity. The Docker/promotion records identify a different commit/tree and are excluded from this frozen-candidate review.

Write:

- Sole write: this file, `agents-results/2026-09-12/dev-main-docker-alignment/acceptance/build-review.md`.
- No other file, process, service, credential, branch, contract, index, or runtime state was changed by this lane.

## Commands

Read-only commands used included:

- `shasum -a 256 agents-results/2026-09-12/dev-main-docker-alignment/acceptance/candidate.patch`
- `find agents-results/2026-09-12/dev-main-docker-alignment -maxdepth 4 -type f -print | sort`
- `rg '^diff --git ' agents-results/2026-09-12/dev-main-docker-alignment/acceptance/candidate.patch`
- `git status --short --untracked-files=all`
- `nl -ba`/`sed -n` on the frozen Docker, Compose, workflow, GoReleaser, frontend tool, version, and referenced runtime files listed above.
- `rg -n` for image/tool/version/port/asset/provenance references in the frozen source.
- `jq` summaries of the bounded JSON evidence; no secret values were printed.

Not run by this lane: dependency installation, `pnpm` commands, Go builds/tests, Docker builds, image pushes/loads, GoReleaser, browser/runtime probes, remote GitHub calls, production access, or live commerce actions. Parent-produced command logs are evaluated as evidence only.

## Findings

### P2 F-01 (unverified): Root Docker context rules need a frozen-context check for `docs/legal`

- Status: `UNVERIFIED`; not treated as a confirmed candidate failure
- Files/lines: frozen `.dockerignore:13-18`; frozen `Dockerfile:38-44`; referenced imports in `frontend/src/views/public/LegalDocumentView.vue:108-109` and `frontend/src/components/admin/AdminComplianceDialog.vue:109-110`.
- Failure class: `docker_context_semantics_unverified`
- Failure origin: pre-existing context/Dockerfile interaction; not a new hunk in `candidate.patch`.
- Repro: the rules exclude `docs/` and then attempt to re-include `docs/legal/`, while the Dockerfile requires that subtree. This interaction was not executed by the review lane. A later different-source Docker log reaches the same logical `COPY docs/legal/` step, but its source identity differs and it cannot prove the frozen candidate's context behavior.
- Why it matters: the frozen root Dockerfile's legal raw imports must be proven present in the exact committed context before promotion. Until that evidence exists, this remains a bounded risk rather than a confirmed failure.

### P1 F-02: Official GoReleaser images use a different Docker route that omits the packaged LDXP asset

- Status: `FAILED` for the LDXP-packaged release contract
- Files/lines: `.github/workflows/release.yml:175-179`; `.goreleaser.yaml:56-68`, `:74-88`, `:90-121`; `Dockerfile.goreleaser:45-47`; root `Dockerfile:183-190`; referenced runtime defaults at `backend/internal/config/config.go:2388-2396` and asset fallback/status logic at `backend/internal/service/liandong_tool_runtime.go:94-106` and `:279-335`.
- Failure class: `release_route_packaging_gap`
- Failure origin: pre-existing alternate release route; the candidate patch changes the local committed-snapshot builder but does not align the GoReleaser route.
- Repro: the release workflow invokes GoReleaser, and every Docker target in `.goreleaser.yaml` selects `Dockerfile.goreleaser`. Its extra files contain only `deploy/docker-entrypoint.sh` and `backend/resources`, while that Dockerfile copies only the GoReleaser binary and resources. It does not copy `/app/ldxp-toolkit-assets/ldxp-toolkit`, its release manifest, or the corresponding environment variables that the root Dockerfile supplies. With the release route's default empty asset configuration, the runtime falls back below the writable data directory and reports the bundled asset as unavailable/not ready.
- Why it blocks: the candidate's root Dockerfile and runtime contract treat the Linux/amd64 LDXP executable plus its manifest as a packaged image assertion, but the route actually used for published images does not carry that assertion. If LDXP is intentionally excluded from official releases, that exclusion is not expressed in the reviewed release contract; otherwise the release image is incomplete.

### P1 F-03: Full frontend test evidence has one failing test after the candidate AppLayout change

- Status: `FAILED`
- Files/lines: frozen candidate `frontend/src/components/layout/AppLayout.vue:45-47`; unchanged test `frontend/src/components/layout/__tests__/AppLayout.theme.spec.ts:36-43`; observed evidence `frozen-vitest.log:867-880`.
- Failure class: `critical_frontend_test_failure` / `implementation_test_contract_drift`
- Failure origin: candidate implementation change. The candidate changes the shell condition from the exact `route.meta.requiresAuth === true && ...` expression to an expression that also accepts `authStore.isAuthenticated`; the existing source-assertion test still requires the old expression.
- Repro: the bounded Vitest run reports `1 failed | 295 passed` files and `1 failed | 2222 passed` tests. The failing assertion is the `toContain` at `AppLayout.theme.spec.ts:38`, as recorded at `frozen-vitest.log:867-880`. No repair was made.
- Why it blocks: the release frontend test gate is not green. The failure may represent an intentionally changed contract with a stale guard, but that intent cannot be assumed during release review.

### P1 F-04: Committed-snapshot Docker artifact and CI promotion proof are unavailable

- Status: `BLOCKED`
- Evidence: `build-scheduling.md:1`; `frozen-frontend-build-pinned.log:19-20`; `github-ci-blocker.json:1`; the frozen-candidate evidence inventory contains no completed Docker build, final image inspection, image digest, or GoReleaser result. A later `docker-build.log:1` and `promotion.json` were rejected because they identify commit `64f459e05d2ec50ac997dd6933794a1fcccaac61` and tree `4df1eaa5020d49fe147eeb330641726d106b7487`, not the required candidate commit/tree; the Docker log ends at backend build step `#32` without a final image inspection.
- Failure class: `missing_artifact_evidence` / `environment_blocked`
- Failure origin: parent build evidence is incomplete and the recorded GitHub CI job was not started because the account was locked for a billing issue. The pinned frontend build was terminated by the coordinator during `vue-tsc` because of concurrent typecheck memory pressure; the scheduling record explicitly says a sequential final build is still required.
- Repro: not applicable. No Docker build or CI retry was run by this lane, and the later different-source build cannot be replay evidence for this review.
- Why it blocks: static inspection cannot prove that `deploy/build_image.sh` produced a runnable image from the frozen commit, that the final image contains the intended files, or that the promoted registry artifact matches the source tuple.

### P2 F-05: Local image provenance label disagrees with the Dockerfile and repository identity

- Status: `FAILED` metadata consistency; non-blocking to application execution
- Files/lines: candidate `deploy/build_image.sh:25`; frozen `Dockerfile:147-149`; frozen `Dockerfile.goreleaser:15-17`.
- Failure class: `provenance_metadata_mismatch`
- Failure origin: candidate change in `deploy/build_image.sh`.
- Repro: the script passes `org.opencontainers.image.source=https://github.com/ValentinoWang/sub2api`, while both image Dockerfiles declare `https://github.com/Wei-Shaw/sub2api`. The command-line label therefore gives the local image a different source identity from the Dockerfile metadata in the same source snapshot.
- Why it matters: a reviewer or promotion tool can associate the image with the wrong repository owner. This must be resolved or explicitly accepted before relying on the label as provenance evidence.

### P2 F-06: Base image references remain mutable tags

- Status: `RISK`
- Files/lines: frozen `Dockerfile:10-13`; frozen `Dockerfile.goreleaser:8-9`.
- Failure class: `reproducibility_gap`
- Failure origin: pre-existing image reference policy; not introduced by the candidate patch.
- Repro: the Node, Go, Alpine, and PostgreSQL bases are selected by tags such as `node:24-alpine`, `golang:1.27.0-alpine`, `alpine:3.21`, and `postgres:18-alpine`, without content digests. The language/package tool versions are pinned, but the base image bytes can still change between builds.
- Why it matters: an image rebuilt from the same commit is not byte-reproducible from the reviewed source tuple alone. This is recorded as residual promotion risk, not as a candidate-introduced functional failure.

## Protected-Test Integrity

No approved protected-test hash manifest or protected-test decision record was present in the bounded lane inputs. Therefore no protected-test hash comparison is claimed. The unchanged `AppLayout.theme.spec.ts` is present but fails against the candidate source as recorded in F-03. The candidate patch does contain test-file changes; this lane did not modify any tests.

Disposition: `MISSING` protected-test baseline, not a pass.

## Requirements-Test Traceability

| Review item | Required evidence | Observed evidence | Result | Release blocking |
| --- | --- | --- | --- | --- |
| Frozen committed-snapshot build | Successful build from the pinned commit, final image inspection, and digest/manifest evidence | `deploy/build_image.sh` statically uses `git archive`, but no matching Docker build or image artifact evidence exists; F-01 remains unverified | `BLOCKED` | Yes |
| Tool alignment | Node 24, pnpm 9.15.9, Go 1.27.0, PostgreSQL 18, and fixed host port | Static source alignment passes: `Dockerfile:10-13`, `package.json:71`, workflow pins, `go.mod:3`, Compose `8080:8080` bindings | `PASS` static only | No |
| Frontend production path | Frozen install, typecheck, full test gate, and Vite/prerender output | Install passed; Vite-only build wrote 15 public pages and four home templates; full test has F-03; pinned build stopped before completion | `FAILED/BLOCKED` | Yes |
| Published Docker route | All release image routes contain the required runtime files | GoReleaser route statically omits the LDXP asset and manifest; F-02 applies | `FAILED` | Yes |
| Staging privacy | Local evidence, acceptance, environment, plugin, and log material excluded from build staging | `.dockerignore:47-60,87-89` and `build_image.sh:19` provide the expected exclusions for untracked material | `PASS` static only | No |
| Human/live commerce acceptance | Human operation, signed result, and live purchase evidence | Explicitly pending and outside this lane | `PENDING` | Yes for final release |

## Machine Verification

| Layer | Evidence | Disposition |
| --- | --- | --- |
| Static source and diff identity | Frozen source tuple, exact patch hash, tool/port/version inspection, and clean diff checks recorded by the parent lane | `PASS` for identity and alignment; `FAIL` for F-02 and F-05; F-01 unverified |
| Frontend install | `frozen-install.log:1-58`, including `Done in 9s using pnpm v9.15.9` | `PASS` |
| Frontend full build | `frozen-frontend-build-pinned.log:2-20`; i18n subcheck passed, then `vue-tsc -b` was terminated with exit 143 | `BLOCKED`, not a product failure attribution |
| Vite production substep | `frozen-vite-build.log:246-251`; build completed and prerender wrote 15 public pages and four home templates | `PASS` with chunk-size and stale Browserslist warnings |
| Frontend typecheck | `frozen-types.log:2-3` contains only the command header and no explicit completion status | `UNVERIFIED` |
| Frontend lint | `frozen-lint.log:2-3` contains only the command header and no explicit completion status | `UNVERIFIED` |
| Frontend Vitest | `frozen-vitest.log:867-880` | `FAIL`, one file/test failed |
| Backend unit | `backend-unit.log:1-110`, all listed packages report `ok` or no test files | `PASS` as recorded evidence |
| Backend integration | `backend-integration.log:1-55`, all listed packages report `ok` or no test files | `PASS` as recorded evidence |
| Shell/Compose checks | `shell-tests.log:80-88` reports the deployment, security, gateway-env, runtime-resource, and Caddyfile checks passed | `PASS` for those checks; not a root Dockerfile artifact proof |
| Docker image | Completed build, `docker image inspect`, digest, and source-bound artifact evidence for the frozen candidate | `MISSING`; later different-source log rejected |
| E2E/visual/live runtime | Not part of this bounded Docker artifact lane; no production access | `NOT RUN` |

The first unpinned frontend attempt is not treated as a product failure: `frozen-frontend-build.log:5-10` shows a host pnpm 11/non-TTY module-removal failure, while the scheduling record says the retry used the pinned toolchain. No dependency install or build was run by this review lane.

## Engineering Review

- The committed-only staging design is directionally sound: `deploy/build_image.sh:7-19` resolves a commit, reads that commit's technical version, and creates the Docker context with `git archive`, preventing ordinary untracked credentials/evidence from entering the context.
- Static tool alignment is coherent across the root Dockerfile, CI workflows, frontend package metadata, Go module, and Compose port bindings.
- The root Dockerfile has a strong LDXP build assertion and manifest, but that assertion is not reused by the GoReleaser release route. This is the main architecture boundary failure.
- The root Dockerfile's legal raw-import dependency and `.dockerignore` rules require an exact frozen-context check; this lane does not promote the potential mismatch to a confirmed failure.
- The local build script's OCI source label is hard-coded to a different owner from the Dockerfile labels.
- No transaction, runtime, external API, credential, or production behavior was exercised. No repair or fallback was introduced by this lane.
- `frontend-audit.json` contains existing high-severity advisories for `xlsx`; corresponding exception entries exist, and the candidate patch has no `frontend/pnpm-lock.yaml` diff. The security exception-check/CI result was not available, so this lane does not treat that as a candidate-introduced finding.

## Human Acceptance

- Binding: not evaluated in this bounded route.
- Checklist and signed run: none evaluated.
- Status: `PENDING`, explicitly not a pass. Human/live purchase acceptance remains pending as stated in the task packet.

No human reviewer was impersonated, no credentials were read, and no live purchase, redemption, payment, or notification action was attempted.

## External Sandbox and Runtime Evidence

None. This lane used no external sandbox, production host, registry readback, remote service, account, or real side effect. The GitHub CI blocker is recorded only from the supplied sanitized JSON evidence.

## Monitoring and Rollback

No production promotion occurred, so no monitoring window, threshold, owner, rollback trigger, rollback command, or rehearsal evidence can be accepted from this lane. The absence of a Docker digest and registry readback also prevents binding a rollback artifact to this source tuple.

## Decision

- Proposed lane result: `FAILED`
- Evidence availability substatus: `BLOCKED` for the committed Docker artifact and CI promotion proof
- Proven scope: frozen source identity, static tool/version/port alignment, committed-only build-context design, staging privacy exclusions, frontend Vite-only output, backend unit/integration evidence, and bounded shell checks
- Blocking conditions: F-02 GoReleaser packaging route, F-03 failing frontend gate, and F-04 missing Docker/CI proof
- Residual risk: F-01 frozen Docker context semantics, F-05 provenance label mismatch, F-06 mutable base tags, unverified typecheck/lint terminal status, absent protected-test baseline, absent registry/digest readback, and pending human/live acceptance
- Release conclusion: `NOT READY`. This review does not declare or imply final release `READY`, production approval, or live sales approval.
