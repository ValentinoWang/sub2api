# Production And Publication Acceptance

Date: 2026-07-23

## GitHub Release

- Release: `https://github.com/ValentinoWang/sub2api/releases/tag/codex-memory-v0.1.0`
- State: published, not draft, not prerelease.
- Assets: macOS, Linux, Windows, checksums, and release manifest.
- GitHub reported asset digests match the locally verified release inputs.
- All four public download URLs returned HTTP 200 from the production host.

GitHub Actions run `30000043556` did not provide hosted-platform acceptance. The Ubuntu, macOS, and Windows jobs failed before running any steps because the GitHub account was locked for a billing issue; the release job was skipped. Therefore no GitHub Artifact Attestation exists, and B2 remains `VERIFIED` rather than `ACCEPTED`.

## Production Candidate

- Host: `43.136.113.101`
- Source commit: `ae2705973300327ada0b7590826f6b7b66aa5143`
- Image: `sub2api-local:ae2705973300`
- Version: `Sub2API 0.1.160`
- Platform: `linux/amd64`
- Image ID: `sha256:c718ce21e12cdaccf3cc0e2618ba58de2b0d273b3c2cd1a99e1e7b7814606855`
- Uploaded archive SHA-256: `5b25ef3767e33f84c1d3d199927e214c7c972d2c6d0e1e343dd5915a1c08f0c5`

The candidate was built and verified on the Mac, uploaded without changing the running service, and switched with `docker compose up -d --no-build --no-deps --force-recreate sub2api`. The remote image ID matched the local candidate before and after cutover.

## Online Acceptance

| Gate | Result |
|---|---|
| Container | `running/healthy`, exact candidate image ID |
| PostgreSQL | healthy; 227 migrations; latest `183_codex_continuity.sql` |
| Redis | healthy |
| `/health` | HTTP 200 on localhost and public IPv4 |
| `/`, `/docs`, `/docs/codex-memory` | HTTP 200 |
| `/codex-memory-release-manifest.json` | HTTP 200; valid JSON; four release download URLs |
| `/Api_subscribe`, `/admin/ops` | HTTP 200 |
| Admin authentication | login HTTP 200; JWT-protected ops overview HTTP 200 |
| `/responses` before cutover | HTTP 200; `response.completed`; no error event |
| `/responses` after cutover | HTTP 200; `response.completed`; no error event |
| Recent critical log scan | no panic, fatal, migration, database, Redis, decryption, or continuity error |
| Existing sites | `https://cve.8689888.xyz` and `http://43.136.113.101` returned HTTP 200 |

The old image `sub2api-local:3f9535ab5450` and the remote transfer archive were deleted after acceptance, as requested. The production filesystem retained approximately 30 GiB free.

## Completion Boundary

Production publication and runtime acceptance are complete. Overall SSOT completion is still blocked by the missing hosted Ubuntu/macOS/Windows run and missing GitHub Artifact Attestation. The unrelated historical Obsidian archive drift also prevents a clean whole-collection `--audit-archive` result, although this bundle's own snapshot and hash check pass.
