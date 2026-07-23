# Production And Publication Acceptance

Date: 2026-07-23

## Historical `0.1.0` GitHub Release

- Release: `https://github.com/ValentinoWang/sub2api/releases/tag/codex-memory-v0.1.0`
- State: published, not draft, not prerelease.
- Assets: macOS, Linux, Windows, checksums, and release manifest.
- GitHub reported asset digests match the locally verified release inputs.
- All four public download URLs returned HTTP 200 from the production host.

GitHub Actions run `30000043556` did not provide hosted-platform acceptance. Attempts 1, 2, and 3 all ended with the Ubuntu, macOS, and Windows jobs failing before running any steps; the release job was skipped. The attempt-3 Check annotation states exactly: `The job was not started because your account is locked due to a billing issue.` This is retained only as historical evidence of the discarded hosted path. The authoritative release gate now runs on the maintainer Mac; hosted CI and Artifact Attestation are optional and do not block acceptance.

## Current `0.1.1` GitHub Release

- Release: `https://github.com/ValentinoWang/sub2api/releases/tag/codex-memory-v0.1.1`
- State: published, not draft, not prerelease.
- Release target: `693ed02487c125f851503184c89d4b5693cb5374`; its tree matches local source commit `bfb8526aed6690de32cfaad18a5517cafff72d75`.
- macOS and Linux ZIP SHA-256: `d034b6242580a52936bc3ad967628cdb44f498d7810a08f61253c08da76423ba`.
- Windows ZIP SHA-256: `52cbc41744c42761263c7be9a863e82de3b0f1e96c1b620bc549aa53a2f3489d`.
- Checksums SHA-256: `764cae49a8b9a7413119248758a45248b678105ffcbae4ddf58772ff39b73ef7`.
- Release manifest SHA-256: `ef9ab2966272b5c9ec7d52f268f65bbdacc5d03f7e3a04fde3a01aadfbc9851f`.
- All five assets were downloaded again after publication; every digest matched the Mac-local candidate. The four public URLs referenced by the production manifest returned HTTP 200.

## Production Candidate

- Host: `43.136.113.101`
- Source commit: `bfb8526aed6690de32cfaad18a5517cafff72d75`
- Image: `sub2api-local:bfb8526aed66`
- Version: `Sub2API 0.1.160`
- Platform: `linux/amd64`
- Image ID: `sha256:52201ac4f3c5d4ea688fad5fe7c654886312a96a287f960e49af8089336b9682`
- Uploaded archive SHA-256: `1c78a740dc8a5ab885ca5d93433dc31e0e95b88087e4847d1bbba76c94c85600`

The candidate was built and verified on the Mac, uploaded without changing the running service, and switched with `docker compose up -d --no-build --no-deps --force-recreate sub2api`. The remote image ID matched the local candidate before and after cutover.

## Online Acceptance

| Gate | Result |
|---|---|
| Container | `running/healthy`, exact candidate image ID |
| PostgreSQL | healthy; 227 migrations; latest `183_codex_continuity.sql` |
| Redis | healthy |
| `/health` | HTTP 200 on localhost and public IPv4 |
| `/`, `/docs`, `/docs/codex-memory` | HTTP 200 |
| `/codex-memory-release-manifest.json` | HTTP 200; version `0.1.1`; valid JSON; four release download URLs |
| `/Api_subscribe`, `/admin/ops` | HTTP 200 |
| Admin authentication | login HTTP 200; JWT-protected ops overview HTTP 200 |
| `/responses` after cutover | HTTP 200; two `response.completed` markers; zero error events; 70,088 SSE bytes |
| Recent critical log scan | no panic, fatal, migration, database, Redis, decryption, or continuity error |
| Existing sites | `https://cve.8689888.xyz` and `http://43.136.113.101` returned HTTP 200 |

The old image `sub2api-local:ae2705973300` (`sha256:c718ce21e12cdaccf3cc0e2618ba58de2b0d273b3c2cd1a99e1e7b7814606855`) and the remote transfer archive were deleted after acceptance, as requested.

## Completion Boundary

The `0.1.1` three-platform assets passed the Mac-local double-build, archive execution, launcher-contract, manifest, and SHA-256 gates. The public Release, production manifest, authenticated administrator path, database and Redis health, migrations, real streamed `/responses` completion, logs, and existing sites all passed online readback. Missing hosted jobs or Artifact Attestation are not completion blockers.

During the previous snapshot refresh, the whole-collection `--audit-archive` result fluctuated with unrelated iCloud state. This bundle's own snapshot and hash check passed; no unrelated snapshot was deleted or rewritten.
