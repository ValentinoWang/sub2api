---
name: sub2api-deployment
description: Build, verify, troubleshoot, upload, notify users about maintenance, and safely cut over this Sub2API project's Docker image without compiling on the production server. Use for Sub2API releases, server updates, Docker image transfers, local Docker build failures, Go cache or registry mirror errors, production cutovers, deployment recovery, health or admin-login regressions, and any request to rebuild or deploy the 43.136.113.101 instance or another low-memory Sub2API host.
---

# Sub2API Deployment

Protect the running service before optimizing deployment speed. Never run `docker compose build`, `docker build`, `go build`, `pnpm build`, or BuildKit on a low-memory production host. Build `linux/amd64` on the Mac, verify it locally, upload an immutable image, and make cutover a separate decision.

The production host is an image consumer, not a build worker. The release image MUST be built by the local Docker/BuildKit environment (or an explicitly approved dedicated builder), then transferred as an immutable archive or pulled by an immutable registry digest. Do not expose the local Docker socket to a remote container, make a remote build depend on the local API connection, or compile on the production host. The running service must remain untouched while the local build, archive transfer, and remote `docker load` complete.

If the release also requests local/server data consistency, read [../sub2api-migration/SKILL.md](../sub2api-migration/SKILL.md) before changing PostgreSQL, Redis, or account data. Its default is a server-preserving merge with explicit ID mapping; an image cutover alone does not synchronize business data.

For errors seen during local candidate builds, read [references/local-build-failures.md](references/local-build-failures.md) before changing scripts or retrying blindly.

## Release Version Contract

The running application version MUST match `^[0-9]+\.[0-9]+\.[0-9]+$`. It is a project release identity, never a channel, branch, feature, date, or commit label. Do not build versions such as `payment-chain-20260905`, `rest2build-*`, `feature-*`, or a bare SHA.

When committed local code contains changes beyond the highest synchronized upstream release, allocate the next project-controlled numeric version before building. For example, local commits on top of upstream `0.2.1` produce `0.2.2`; they must not be relabeled as upstream `0.2.1` and must not receive a channel suffix. Keep the same numeric version in `backend/cmd/server/VERSION` and Docker's `VERSION` build argument. Record source provenance only through the immutable image tag, `COMMIT`, `DATE`, and artifact manifest.

## Candidate Cache Cleanup

Before building, inventory Docker image references, Buildx history, and the application's exact update-check key. Delete only verified stale candidates:

```bash
docker ps -a --filter "ancestor=<old-image-tag>" --format '{{.ID}} {{.Names}} {{.Status}}'
docker image inspect <old-image-tag>
docker buildx history ls
docker exec sub2api-redis sh -lc 'unset REDISCLI_AUTH; redis-cli TTL update_check_cache'
```

An image is removable only when the ancestor query returns no container. Remove the exact old image tag and task-owned failed Buildx history records; preserve the running image, its rollback image, production base images, databases, Redis data, and unrelated project images. `docker buildx history rm` removes selected history records only. Do not use `docker system prune`, `docker builder prune`, or a shared-builder cache prune to clear a version issue. A broad BuildKit cache cleanup is allowed only on a dedicated release builder created for this project and only after the candidate archive is verified.

Clear the application update cache immediately before a new image is activated and verify the key is absent afterward:

```bash
docker exec sub2api-redis sh -lc 'unset REDISCLI_AUTH; redis-cli DEL update_check_cache; redis-cli TTL update_check_cache'
```

The browser's version state is in-memory, so reload the admin page after the replacement container is healthy. Cache invalidation cannot make a running old binary report a new version; it must be paired with a verified image activation.

## Hard Gates

1. Inspect the current server before any build or transfer. Require the application container, PostgreSQL, Redis, `/health`, public root, `/Api_subscribe`, and `/admin/ops` page to respond normally.
2. If the current service is unavailable, stop. Recover the existing container first. Do not build, load, prune, migrate, or switch images during an availability incident.
3. Require a clean tracked worktree, a committed SHA, and a `backend/cmd/server/VERSION` value matching the release-version contract. Preserve unrelated untracked files.
4. Run the relevant Go tests and complete the embedded frontend/backend image build locally. The Docker `VERSION` argument must exactly equal `backend/cmd/server/VERSION`.
5. Verify the candidate platform is `linux/amd64`, run `/app/sub2api --version`, assert that it reports the exact numeric release version, and record the image/archive SHA256.
6. Before activation, clear `update_check_cache` using the exact Redis command in Candidate Cache Cleanup. Record the `DEL` result and the postcondition `TTL = -2`.
7. Upload and load the candidate without changing the running container. Recheck current service health after `docker load`.
8. Before cutover, verify admin login using an authenticated probe or an actual login. Page-shell HTTP 200 alone is insufficient.
9. Publish an active popup maintenance announcement to all users before any command that can restart, recreate, or interrupt a production container. API success and a recorded announcement ID are mandatory; otherwise stop before cutover.
10. Cut over with `--no-build --no-deps`. Never use a mutable `latest` tag as the candidate identity.
11. Verify health, admin login, API-key authentication, `/responses` SSE completion, PostgreSQL migrations, and recent logs before declaring success.
12. Archive the maintenance announcement and publish a recovery popup after acceptance. If acceptance fails, update the original announcement to say maintenance is delayed; never silently leave users waiting.

For a data merge, add these gates before cutover: complete a non-secret inventory and dry-run mapping, resolve all identity conflicts, verify the approved backup policy, and confirm that account credentials will be imported through the authenticated account-data API. Do not restore a full database dump for a merge or copy encrypted account rows across different keys.

## Build And Upload

Run from a clean committed checkout at the repository root. This repository does not currently include the older `build-candidate.sh` or `upload-candidate.sh` helpers; use this explicit build sequence rather than referencing absent scripts:

```bash
release_version="$(tr -d '\\r\\n' < backend/cmd/server/VERSION)"
source_sha="$(git rev-parse --short=12 HEAD)"
build_date="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
image_tag="sub2api-local:${release_version}-${source_sha}"

test "$release_version" != "$(printf '%s' "$release_version" | sed -E 's/^[0-9]+\\.[0-9]+\\.[0-9]+$//')"

docker buildx build --platform linux/amd64 --load \
  --tag "$image_tag" \
  --build-arg "VERSION=$release_version" \
  --build-arg "COMMIT=$source_sha" \
  --build-arg "DATE=$build_date" \
  .

docker image inspect "$image_tag" --format '{{.Os}}/{{.Architecture}}'
docker run --rm --platform linux/amd64 --entrypoint /app/sub2api "$image_tag" --version
```

Save the source SHA, numeric version, image tag, image ID, platform, and archive checksum under `.artifacts/` before any upload. Upload/load verification never recreates the application container.

The build sequence intentionally separates the test platform from the production image platform. On Apple Silicon, Go tests should run on the host platform, while the release image must still be `linux/amd64`. Do not "simplify" this into a single platform variable.

Password authentication may be supplied interactively or through an existing SSH agent. Never add a password, API key, JWT, OAuth token, or encryption key to these scripts, command history, manifests, or the repository.

## Local Build, Remote Load, And Resumable Cutover

Use the following deployment boundary for the final replacement workflow:

1. Build and test the candidate on the local machine with `--platform linux/amd64`. The candidate tag, numeric application version, source commit, image ID, and archive digest are the local release manifest.
2. Transfer the archive or push the immutable digest while the current remote container continues serving traffic. `docker load` or an immutable `docker pull` MUST NOT recreate, restart, or replace the current container.
3. Start the candidate as a separate `green` slot on a private port or internal network. It MUST pass a real readiness check, database/Redis checks, authenticated admin/API-key checks, and a representative `/responses` SSE completion before it is added to the public upstream set. `/health` alone is liveness evidence, not readiness evidence.
4. Keep the client `base_url` on the stable domain or local failover relay. Never point Codex at the candidate port. If the current client control connection is fragile, run the release coordinator as a detached, durable job and persist its phase and log under a release-specific state directory. A foreground SSH or Codex stream MUST NOT be the only copy of the cutover state.
5. Before traffic changes, publish the required maintenance notice. Mark `blue` as draining so it rejects new requests but keeps existing HTTP/SSE work alive until completion or the configured grace deadline. WebSocket sessions need explicit tracking and a bounded close policy; Go HTTP shutdown cannot transparently move a hijacked WebSocket to another process.
6. Immediately before activation, clear `update_check_cache`, verify `TTL = -2`, atomically reload the stable proxy to the green slot, and run the public authenticated smoke checks. Keep blue and its image available for the rollback window.
7. If any post-switch gate fails, reload the proxy to blue, record the failure, and only then clean up the candidate. Do not rebuild on the remote host as a reaction to a failed cutover.

The coordinator SHOULD persist explicit states such as `preflight`, `building_local`, `candidate_uploaded`, `green_ready`, `draining_blue`, `switched`, `observing`, `completed`, and `rolled_back`. A lost API/SSH connection must allow the job to finish or roll back independently and allow a later operator to read the last durable state.

This workflow protects new requests and lets existing streams drain; it does not promise that a TCP, SSE, or WebSocket connection that already died will be resumed byte-for-byte. For that case, require a stable request ID, idempotent retry semantics, and durable response/event state. The current completed-turn continuity ledger can rebuild a later request context, but it is not an in-flight event log or a WebSocket session migration protocol.

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
