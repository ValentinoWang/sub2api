---
name: sub2api-deployment
description: Build, verify, upload, and safely cut over this Sub2API project's Docker image without compiling on the production server. Use for Sub2API releases, server updates, Docker image transfers, production cutovers, deployment recovery, health or admin-login regressions, and any request to rebuild or deploy the 43.136.113.101 instance or another low-memory Sub2API host.
---

# Sub2API Deployment

Protect the running service before optimizing deployment speed. Never run `docker compose build`, `docker build`, `go build`, `pnpm build`, or BuildKit on a low-memory production host. Build `linux/amd64` on the Mac, verify it locally, upload an immutable image, and make cutover a separate decision.

## Hard Gates

1. Inspect the current server before any build or transfer. Require the application container, PostgreSQL, Redis, `/health`, public root, `/Api_subscribe`, and `/admin/ops` page to respond normally.
2. If the current service is unavailable, stop. Recover the existing container first. Do not build, load, prune, migrate, or switch images during an availability incident.
3. Require a clean tracked worktree and a committed SHA. Preserve unrelated untracked files.
4. Run the relevant Go tests and complete the embedded frontend/backend image build locally.
5. Verify the candidate platform is `linux/amd64`, run `/app/sub2api --version`, and record the image/archive SHA256.
6. Upload and load the candidate without changing the running container. Recheck current service health after `docker load`.
7. Before cutover, verify admin login using an authenticated probe or an actual login. Page-shell HTTP 200 alone is insufficient.
8. Cut over with `--no-build --no-deps`. Never use a mutable `latest` tag as the candidate identity.
9. Verify health, admin login, API-key authentication, `/responses` SSE completion, PostgreSQL migrations, and recent logs before declaring success.

## Build And Upload

Run from the repository root:

```bash
skills/sub2api-deployment/scripts/build-candidate.sh
skills/sub2api-deployment/scripts/upload-candidate.sh \
  --target root@43.136.113.101 \
  --manifest .artifacts/sub2api-deployment/<commit>/candidate.env
```

`build-candidate.sh` runs tests, cross-builds the image, performs image checks, and creates a compressed archive. `upload-candidate.sh` verifies the old service before and after transfer, validates the archive checksum remotely, and runs `docker load`. It never recreates the application container.

Password authentication may be supplied interactively or through an existing SSH agent. Never add a password, API key, JWT, OAuth token, or encryption key to these scripts, command history, manifests, or the repository.

## Cutover

Read the candidate image tag from `candidate.env`. On the server, verify that `deploy/docker-compose.server.yml` has no `build:` section and that the candidate image exists. Preserve the current container image ID only for the duration of cutover; remove any temporary rollback tag after acceptance if the user does not want retained rollback images.

Use the server's existing Compose files and environment:

```bash
cd /srv/sub2api/current
SUB2API_IMAGE='sub2api-local:<commit>' \
  docker compose \
  -f deploy/docker-compose.local.yml \
  -f deploy/docker-compose.server.yml \
  up -d --no-build --no-deps --force-recreate sub2api
```

Do not run this command merely because upload succeeded. Require all pre-cutover gates first.

## Acceptance

Verify with fresh evidence:

- `docker inspect sub2api` reports `running` and `healthy` with the candidate image ID.
- `curl http://127.0.0.1:8080/health` and the public URL succeed with normal latency.
- `/admin/ops` loads and authenticated admin access succeeds.
- PostgreSQL and Redis remain healthy; required migration rows/tables exist.
- A real API key completes a streamed `/responses` request without 502/503 or SSE idle timeout.
- Recent application logs contain no panic, fatal migration, decryption, database, Redis, or continuity errors.
- Proxy/tunnel services and existing sites remain available.

If any acceptance check fails, stop traffic changes, capture logs and image IDs, and restore the prior application image when schema compatibility permits. Never respond to a failed cutover by rebuilding on the production server.

## Incident Recovery

When an accidental production build makes the host unavailable:

1. Cancel the build client and any active BuildKit job.
2. Restart the existing `sub2api` container by container ID/name, not through a Compose recreation that may select a newly built tag.
3. Check memory, swap, disk, Docker health, localhost `/health`, admin login, and public reachability.
4. Resume release work only after the old service is stable.

Keep Codex memories and transcripts independent from deployment. Do not edit `~/.codex/config.toml`, `~/.codex/memories`, `~/.codex/sessions`, or Redis as part of an image release.
