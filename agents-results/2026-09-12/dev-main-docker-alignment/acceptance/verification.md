# Git and Docker alignment result

Date: 2026-09-13T00:01:56.306039

## Completed scope

User-authorized order: local committed source -> user-owned GitHub dev -> fast-forward main -> build from main. Remote dev and main both read back as `64f459e05d2ec50ac997dd6933794a1fcccaac61`. The upstream Wei-Shaw remote was not modified.

Technical version: `0.2.6`. Public release label remains `0.2.1`.

Image: `sub2api-local:0.2.6-64f459e05d2e`

Image content ID: `sha256:ea8ebd15578451c1e414b8dfa980ae7de7b400e2a5d6335c4fe1c5f229bcd857`

Platform: `linux/amd64`.

Portable image archive: `/Users/vsiyo/.local/share/sub2api/releases/0.2.6-64f459e05d2e/sub2api-image.tar.gz` (56522691 bytes).

Archive SHA-256: `bf9fc1330edd6c1b56ac1f45c193d469914130b505cfbdb24a11c80a1f51a2be`.

Archive integrity was checked with gzip and the archived image config SHA-256 matches the built image content ID. Load this same archive in each environment; do not rebuild independently and assume mutable base tags produce identical bytes.

## Executed checks

- Pinned pnpm 9.15.9 frozen installation passed. Original lockfile security overrides retained; no lockfile change in final commits.
- Full frontend ESLint and typecheck passed. Initial full Vitest: 2222 passed, one obsolete source-string assertion failed. That assertion was replaced with six actual rendered-theme cases; 18 related layout tests passed. Other passed evidence was reused.
- Payment review found the native subscription tab exposed in store-only mode. Fixed in 64f459e05; 33 payment/panel tests passed, and independent focused review verified the fix.
- Backend unit tests passed. Deployment shell/Compose checks, acceptance layout guard and existing frontend audit-exception check passed.
- Root Docker build completed successfully, including i18n checks, vue-tsc, Vite/prerender and Go compilation. Build argument and image-label revision agree with Git main.
- Image CLI readback reports backend 0.2.6 with the complete source commit. LDXP CLI reports 0.2.6 and its packaged binary hash matches its manifest.
- An isolated container with a temporary dynamic localhost port served setup status, embedded HTML for home/purchase/redeem/register/tutorial/troubleshooting and its script assets. It used no live database or credentials and was removed. This proves packaged frontend startup only.
- Local HMR module at 4174 was read back and contains the channel-state and subscription-guard fixes.

## Not ready for production cutover

GitHub Actions could not start because the GitHub account is locked for a billing issue. No remote CI PASS is claimed.

Completed backend integration tests exited 2: a Redis testcontainer reaper-name collision, a cache cancellation timing failure and legacy identity fixture truncation blocked by the membership audit append-only trigger. The backend product source is unchanged from the base except VERSION; these failed checks are retained in backend-integration.log. They were not hidden or weakened.

Local running backend remains `sub2api-local:0.2.5-local-1b221364260b`; production remains `sub2api-local:0.2.5-8c9390cb6e77`. Neither was restarted or replaced. Code branches and the packaged artifact are aligned, but running dev/prod are not yet aligned.

No production configuration, purchase, redemption, inventory upload, stock toggle, database migration, credential reset or service interruption was performed in this release step. Real LDXP merchant configuration and purchase/restock acceptance remain outstanding. No human PASS or final production READY is declared.

The existing GoReleaser/tag path is not the artifact route used here and lacks the root image's packaged LDXP assertion; use the verified root-Dockerfile archive for any subsequent promotion.
