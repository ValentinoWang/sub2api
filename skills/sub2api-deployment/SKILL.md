---
name: sub2api-deployment
description: Build, verify, troubleshoot, upload, notify users about maintenance, and safely cut over this Sub2API project's Docker image without compiling on the production server. Use for Sub2API releases, server updates, Docker image transfers, local Docker build failures, Go cache or registry mirror errors, production cutovers, deployment recovery, health or admin-login regressions, and any request to rebuild or deploy the 43.136.113.101 instance or another low-memory Sub2API host.
---

# Sub2API Deployment

Protect the running service before optimizing deployment speed. Never run `docker compose build`, `docker build`, `go build`, `pnpm build`, or BuildKit on a low-memory production host. Build `linux/amd64` on the Mac, verify it locally, upload an immutable image, and make cutover a separate decision.

If the release also requests local/server data consistency, read [../sub2api-migration/SKILL.md](../sub2api-migration/SKILL.md) before changing PostgreSQL, Redis, or account data. Its default is a server-preserving merge with explicit ID mapping; an image cutover alone does not synchronize business data.

For errors seen during local candidate builds, read [references/local-build-failures.md](references/local-build-failures.md) before changing scripts or retrying blindly.

## Hard Gates

1. Inspect the current server before any build or transfer. Require the application container, PostgreSQL, Redis, `/health`, public root, `/Api_subscribe`, and `/admin/ops` page to respond normally.
2. If the current service is unavailable, stop. Recover the existing container first. Do not build, load, prune, migrate, or switch images during an availability incident.
3. Require a clean tracked worktree and a committed SHA. Preserve unrelated untracked files.
4. Run the relevant Go tests and complete the embedded frontend/backend image build locally.
5. Verify the candidate platform is `linux/amd64`, run `/app/sub2api --version`, and record the image/archive SHA256.
6. Upload and load the candidate without changing the running container. Recheck current service health after `docker load`.
7. Before cutover, verify admin login using an authenticated probe or an actual login. Page-shell HTTP 200 alone is insufficient.
8. Publish an active popup maintenance announcement to all users before any command that can restart, recreate, or interrupt a production container. API success and a recorded announcement ID are mandatory; otherwise stop before cutover.
9. Cut over with `--no-build --no-deps`. Never use a mutable `latest` tag as the candidate identity.
10. Verify health, admin login, API-key authentication, `/responses` SSE completion, PostgreSQL migrations, and recent logs before declaring success.
11. Archive the maintenance announcement and publish a recovery popup after acceptance. If acceptance fails, update the original announcement to say maintenance is delayed; never silently leave users waiting.

For a data merge, add these gates before cutover: complete a non-secret inventory and dry-run mapping, resolve all identity conflicts, verify the approved backup policy, and confirm that account credentials will be imported through the authenticated account-data API. Do not restore a full database dump for a merge or copy encrypted account rows across different keys.

## Build And Upload

Run from the repository root:

```bash
skills/sub2api-deployment/scripts/build-candidate.sh
skills/sub2api-deployment/scripts/upload-candidate.sh \
  --target root@43.136.113.101 \
  --manifest .artifacts/sub2api-deployment/<commit>/candidate.env
```

`build-candidate.sh` runs tests, cross-builds the image, performs image checks, and creates a compressed archive. `upload-candidate.sh` verifies the old service before and after transfer, validates the archive checksum remotely, and runs `docker load`. It never recreates the application container.

`build-candidate.sh` intentionally separates the test container platform from the production image platform. On Apple Silicon, Go tests should run on the host platform, while the release image must still be `linux/amd64`. Do not "simplify" this into a single platform variable.

Password authentication may be supplied interactively or through an existing SSH agent. Never add a password, API key, JWT, OAuth token, or encryption key to these scripts, command history, manifests, or the repository.

## Cutover

Read the candidate image tag from `candidate.env`. On the server, verify that `deploy/docker-compose.server.yml` has no `build:` section and that the candidate image exists. Preserve the current container image ID only for the duration of cutover; remove any temporary rollback tag after acceptance if the user does not want retained rollback images.

Before cutover, publish the maintenance notice through the site's own notification module. Use an admin API key or admin JWT from the environment; never place it in arguments, source, state files, or logs.

```bash
export SUB2API_BASE_URL='http://43.136.113.101:8080'
export SUB2API_ADMIN_API_KEY='<admin api key>'
node skills/sub2api-deployment/scripts/notify-maintenance.js \
  --event start --release '<version-or-commit>' --eta-minutes 5
```

Do not continue unless the command returns `maintenance_announcement_id=<id>`. This requirement applies to planned Docker recreation, restart, host reboot, proxy restart, migration, or other work that may interrupt requests. A read-only inspection, image upload, or `docker load` does not require a notice while the running service remains unaffected.

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

After every acceptance check succeeds, close the notification lifecycle:

```bash
node skills/sub2api-deployment/scripts/notify-maintenance.js \
  --event complete --release '<version-or-commit>'
```

If cutover or acceptance fails, publish the failure state before recovery work:

```bash
node skills/sub2api-deployment/scripts/notify-maintenance.js \
  --event failed --release '<version-or-commit>' \
  --details '服务恢复时间另行通知。'
```

The script records only announcement IDs and timestamps under the ignored `.artifacts/` directory. It targets all users with `popup` mode. Never claim deployment completion while its state remains `start` or `failed`.

## Acceptance

Verify with fresh evidence:

- `docker inspect sub2api` reports `running` and `healthy` with the candidate image ID.
- `curl http://127.0.0.1:8080/health` and the public URL succeed with normal latency.
- `/admin/ops` loads and authenticated admin access succeeds.
- PostgreSQL and Redis remain healthy; required migration rows/tables exist.
- A real API key completes a streamed `/responses` request without 502/503 or SSE idle timeout.
- Recent application logs contain no panic, fatal migration, decryption, database, Redis, or continuity errors.
- Proxy/tunnel services and existing sites remain available.
- When data synchronization is in scope, target-only records remain, local-only records appear once, dependent foreign keys resolve through the mapping, and temporary exports/dumps are removed or explicitly retained.

If any acceptance check fails, stop traffic changes, capture logs and image IDs, and restore the prior application image when schema compatibility permits. Never respond to a failed cutover by rebuilding on the production server.

## Incident Recovery

When an accidental production build makes the host unavailable:

1. Cancel the build client and any active BuildKit job.
2. Restart the existing `sub2api` container by container ID/name, not through a Compose recreation that may select a newly built tag.
3. Check memory, swap, disk, Docker health, localhost `/health`, admin login, and public reachability.
4. Resume release work only after the old service is stable.

Keep Codex memories and transcripts independent from deployment. Do not edit `~/.codex/config.toml`, `~/.codex/memories`, `~/.codex/sessions`, or Redis as part of an image release.

## Local Build Failure Memory

If a candidate build is slow or fails before image export, check these recurring causes first:

- Docker registry mirror EOF while resolving base image metadata.
- Apple Silicon tag/platform drift after pulling `linux/amd64` base images.
- Corrupted Go module or build cache under the mounted cache directories.
- `go mod download` hanging because the test container lacks the intended `GOPROXY`.
- Stream timeout tests hanging because the SSE scanner is synchronously drained after closing the upstream body.

Use the reference file for concrete error strings, diagnosis commands, and durable fixes. Prefer updating the project skill or scripts when a new failure mode is confirmed.
