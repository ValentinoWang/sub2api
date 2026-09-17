# In-progress production release

Authorized: merge all local Sub2API branches, push fork/main, deploy production. User has repeatedly said continue; do not ask permission again.

- Final merge commit: 507a820c6792782ac07f9962a06c6224ce6cba70, version 0.2.4.4.
- Release worktree: /Users/vsiyo/.local/share/sub2api/worktrees/release-0.2.4.4-20260915, branch release/0.2.4.4.
- Parents: dev 8373160e1 and codex/chrome-restock-products 0c8afe26a. Ours merge retains superseded branch in history; tree exactly matches dev. No local branch outside this ancestry.
- Main worktree remains dev with existing untracked evidence and two dirty generated human acceptance logs. Preserve them. Old worktree has untracked evidence; archive before any cleanup.
- fork/main and fork/dev still e77fce24d. No push/build/cutover yet.
- CI session 1761 (local-ci-4) running at backend-unit. Previous local-ci 1/2/3 failed environment selection: Node22, missing Harness absolute path, Python3.9. No source changes. Fixed launcher run-ci.sh in this task. Tools /tmp/sub2api-release-tools-20260915/bin pins Node24.19, pnpm9.15.9, Python3.12, golangci-lint2.13 built Go1.27, govulncheck1.8. Environment HARNESS_ENGINEERING_HOME=/Users/vsiyo/Desktop/Opensource_Tool/Harness_Engineering.
- Docker Desktop started; local app already 0.2.4.4-8373160e1977 healthy, production 0.2.4.2-e77fce24d8e9 healthy. Production old image ID sha256:b439ad050f7b0c8e529eddfade85657d78f133b8d497e6ac353cffabb26f7573, present locally for rollback test.
- Production SSH ubuntu@43.156.50.78 key /Users/vsiyo/Desktop/Key/MacbookAir.pem, deployment /home/ubuntu/sub2api/deploy/docker-compose.local.yml.
- Admin keys at /Users/vsiyo/.local/share/sub2api/commerce-release/{dev,prod}-admin.key, permissions600, both GET admin settings verified. Never print keys.
- Production baseline inventory/gateway/migration proof in acceptance/. Real model gpt-5.6-luna streamed response.completed via existing admin user1 APIkey3. Store only sanitized results.
- CRITICAL: production has only6 redeem_codes and zero batches, shop reported thousands in goods642224 etc. Do not claim or enable automatic stock or configure unverified products. Existing purchase setting https://wzyp.cn/shop/MGDY0ZE4 verified Chrome /purchase. New public catalog returns404 until configured, preserving shop fallback.
- Copied existing tested release helpers and rollback image verification into task, source hashes in helper-provenance.json; all24 helper tests passed. Use make_candidate after actual build, verify_local_images with production old image, backup/replace with candidate and maintenance receipt. Never forge rollback proof. Existing repair/verify actions reference deleted toolkit; use only maintenance/recovery actions and verify new APIs directly.
- Only added migration241; all base checksums match production,9 historical extras retained. Production backup should be fresh before cutover.
- Next: complete CI, push/readback main (and align dev), build from final Git archive, verify image/rollback on isolated empty PostgreSQL, save/hash/upload/load, publish maintenance ID, fresh backup, only recreate sub2api, health/functional/model/route/migrations readbacks, recovery, protected cache cleanup and evidence/worktree consolidation. No human PASS can be claimed.

## Latest update: Git delivered, build running

- Complete CI local-ci-4 PASSED all19 stages; 299 frontend files/2200 tests. Evidence copied/hash-verified into PRIMARY repo task acceptance/local-ci-4.
- main/dev/fork-main/fork-dev all507a820c6792782ac07f9962a06c6224ce6cba70. git ls-remote and GitHub API SHA readback saved github-readback.json. PRIMARY working branch now main.
- Both extra worktrees and branches removed after evidence copy and hash checks. Only primary worktree remains. Old241 evidence files and legacy extension ZIP preserved in /Users/vsiyo/.local/share/sub2api/archives/chrome-restock-products-20260915. No credentials in ignored old worktree files. Existing primary untracked evidence/two dirty projections preserved.
- Go/test/golangci caches cleaned after allCI exited, recovered space to9.4GiB. No running host Go tasks when cleaned.
- Docker builder sub2api-release-20260915 created docker-container, default-load=true, memory4g, CPUquota200000/period100000. Default builder selection unchanged.
- Build RUNNING session80969 from PRIMARY repo via BUILDX_BUILDER=sub2api-release-20260915 BUILDKIT_PROGRESS=plain bash deploy/build_image.sh507a820c... (space before commit), log acceptance/image-build.log. At frontend dependency install (~330/973), Go modules parallel. Runtime image not yet built/verified. Need monitor disk (last4.9GiB free).
- Local baseline gateway also completed successfully; dev-gateway-before.json. Both dev/prod API key3 legitimate admin user1 group3. Never print key.
- CUA initialized; releaseTab is claimed Chrome3 tab1852177886, now https://www.ai.rest2build.lol/purchase; user authenticated. DOM/screenshot proves shop link and redemption textbox before deployment. Browser plugin page access restriction remains; do not bypass.
- NEXT after image completion: inspect ID/platform/OCI and version. Generate prod candidate with basee77 and dev candidate with base837 (local already migration241). Run existing verify_local_images.py separately with oldprodimage and oldlocalimage to create real bound rollback proofs. Explicit postgres-image postgres:18.1-alpine3.23 exists after CI (postgres:18-alpine tag previously absent), redis8-alpine exists. Candidate same image for both. Helpers copied into PRIMARY task.
- Then archive/hash/transfer/load sameimage; maintenance/backup/replace dev then prod; use matching candidate/proof for each. Fresh backups required<=30min. Clear exact Redis update_check_cache only. New toolkit installation API removed; do NOT run old repair/verify actions. Use new browser status and existing admin/version/public settings/SSE to validate.
- CRITICAL unverified merchant inventory still unresolved; leave production product config unconfigured, so new public catalog404 preserves current shop fallback. Do not fabricate inventory/stock readiness. User authorized deployment, not a false business acceptance.
- Finally recovery notices, clean owned BuildKit builder/cache and temporary task tools, refresh report/cleanup ledger. Local and production app should end at SAME final image; primary .env preserved except image. Production has not yet been touched by this release.

## Resume update during image compilation

- Build session 80969 still active at Go compile (frontend completed). Dedicated builder CPU quota raised to 4 cores; 4 GiB memory unchanged. Native arm64 Go cross-compiles amd64. No build errors; cold dependency compile is slow. Disk ~4.3 GiB.
- Completed host `go clean -modcache` after checking no host Go compiler/test process; Docker module cache is independent. Preserve all rollback images/data.
- Remote helper directory created `/home/ubuntu/sub2api-release-0.2.4.4-507a820c`; release_common.py, backup_app.py, replace_app.py, rollback_app.py uploaded. Production unchanged/healthy.
- Added operational `verify_live.py` for candidate-bound sanitized FUNCTIONAL_ACCEPTANCE receipt (health, version, retained config, page shells, real SSE response.completed). Syntax checked; not yet executed. No secrets written. It uses existing user1 APIkey3 group3.
- IMPORTANT current DEV baseline differs from prod: purchase URL `https://wzyp.cn/item/e8yrh4`, browser enabled=true, products=5. Preserve it; do not incorrectly require disabled/empty dev config. PROD expected new browser=false/products=0 and shop URL `https://wzyp.cn/shop/MGDY0ZE4`; old production browser/status returns404.
- CUA getState works, getTab Chrome3 tab1852177886 repeatedly returns Debugger unattached. Browser rendering after release remains unverified until reconnected; do not bypass extension restrictions.
- Memory used MEMORY.md:848 for maintenance discipline; final citation required.

## COMPLETED

Git publication and same-image local/production deployment are complete. Authoritative operational result: REPORT.md. Prod second cutover passed after bounded retired-default guard correction (28 tests). Maintenance21 archived, recovery22 visible. Both environments healthy on image4e7bf76b9e2c, real SSE verified. BuildKit/task caches removed. Business inventory/redemption/automatic replenishment acceptance remains pending; production worker not enabled.
