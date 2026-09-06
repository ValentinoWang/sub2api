---
name: sub2api-upstream-release
description: Synchronize this Sub2API project with the latest GitHub upstream code, reconcile local changes on main, clean the release checkout and Codex runtime state, validate the result, and publish an immutable image to the remote server. Use when updating Sub2API from Wei-Shaw/sub2api, merging a new upstream release, updating the deployed 43.156.50.78 instance, or preparing a main-only release.
---

# Sub2API Upstream Release

Use the GitHub repository as the code baseline, the local `main` branch as the release source, and the remote server as a deployment target. Do not copy a production worktree back over the local repository and do not claim that a running container proves that the source is synchronized.

## Release Invariants

- Inspect before mutating: record remotes, current commit, tags, branch tracking, worktrees, dirty files, untracked files, and active build/deploy processes.
- Preserve unrelated user changes, `.artifacts/`, `agents-results/`, Codex memories, task transcripts, provider definitions, project trust entries, and runtime configuration. Never use `git reset --hard`, `git clean -fd`, or a broad recursive delete to make the tree look clean.
- Identify task-owned files before staging. Commit only the intended source, tests, documentation, and release metadata.
- The release checkout must have exactly one active release worktree on `main`. Reconcile or archive task-owned branches before publishing. Do not delete an unclassified user branch or worktree; branch deletion requires explicit authorization.
- Stop or finish task-owned Codex workers, builds, and deployment sessions before the final release check. Do not edit `~/.codex/config.toml`, `~/.codex/memories`, `~/.codex/sessions`, or Codex provider state as part of a Sub2API release.
- The published artifact must identify the final `main` commit and target platform. The server receives the artifact; it is not a second source of truth.
- The local machine is the release build worker. Build the immutable `linux/amd64` candidate locally, verify it, and transfer/load it on the server. The server MUST NOT compile the checkout, run BuildKit, depend on the local Docker socket, or make the replacement depend on the current Codex/SSH stream.
- The application version must be a project-controlled numeric `MAJOR.MINOR.PATCH` value. Never put channel, branch, feature, date, or commit labels in `VERSION`. When local committed code is ahead of the highest merged upstream release, advance the local project version before release instead of relabeling it as the upstream version.

## Synchronize The Code

1. Inspect local state with `git status --short`, `git branch -vv`, `git worktree list`, `git remote -v`, and `git log -1 --oneline`. Save a non-secret inventory under the ignored `.artifacts/` directory when evidence is useful.
2. Resolve the authoritative GitHub remote and fetch the requested tag or `main`. Confirm the release commit from GitHub's API or signed/tagged ref; do not infer it from a local stale remote-tracking branch.
3. Capture task-owned local changes as a reviewable patch or commit. Keep unrelated untracked directories in place and out of the patch.
4. Move the release checkout to `main`, merge or rebase the fetched upstream commit according to the repository's existing history, and reapply the task-owned changes. Resolve every conflict explicitly and inspect the resulting diff.
5. Confirm the final source with `git diff upstream-ref..main`, `git status`, and `git log`. If `main` contains local commits beyond the upstream tag, increment `backend/cmd/server/VERSION` to the next project-controlled numeric version; do not use a channel suffix or preserve an upstream version that no longer identifies the source. The tree may retain declared unrelated untracked files, but tracked release files must be committed and there must be no unresolved conflict markers.
6. Push `main` only when the user has requested a repository push and the final commit has passed validation. Read back the remote `main` SHA after pushing; a local push result alone is not synchronization evidence.

Do not interpret “唯一在 main 上” as permission to erase all personal branches. It means the release has one active checkout and one publishable branch: `main`. Preserve other branches unless their ownership and deletion have been confirmed.

## Clean The Runtime Before Publishing

Before building the candidate:

- Confirm `git worktree list` has one active release worktree for this project and it is checked out on `main`.
- Finish or stop task-owned Codex workers and local build processes. Leave unrelated projects and user sessions alone.
- Check that no stale generated binary, temporary archive, cross-compile output, or pending deployment session can be mistaken for the candidate.
- Inspect stale candidate image references and the exact Redis `update_check_cache` key. Delete only image tags with no container ancestors and task-owned failed Buildx history records; clear the Redis key immediately before activation. Never use a shared Docker cache prune as a version-cache reset.
- Keep Codex configuration and local memory independent from the selected Sub2API model provider. Runtime cleanup is process/worktree cleanup, not deletion of Codex state.
- Record the exact source SHA, image tag, platform, and archive checksum in `.artifacts/`.

The remote activation path is a separate, resumable phase. Upload or push the local candidate while the current service remains untouched; start a private green candidate; wait for readiness and authenticated `/responses` evidence; then drain blue and atomically switch the stable proxy. Run the cutover coordinator as a durable detached job with release-specific state/logs so an API or SSH disconnect cannot leave the deployment without a recoverable phase record. Keep blue available until the observation window completes and route back to it on any failed acceptance gate. The client must retain the stable `base_url`; it must not connect directly to the green port.

## Validate And Publish

Read [../sub2api-deployment/SKILL.md](../sub2api-deployment/SKILL.md) for the build, maintenance-notice, upload, cutover, and acceptance gates. Read [../sub2api-admin/SKILL.md](../sub2api-admin/SKILL.md) for authenticated model/account checks.

When the release request also requires local and server business data to be consistent, read [../sub2api-migration/SKILL.md](../sub2api-migration/SKILL.md) and its merge reference. The default is a server-preserving merge: retain server records, add local-only records, map relational IDs explicitly, compare file-backed runtime manifests, and keep deployment infrastructure separate from application data. Do not overwrite the server database or claim data parity from matching row counts alone.

At minimum, before remote cutover:

- Run the relevant backend tests, frontend build, and `git diff --check`.
- Build the candidate locally for `linux/amd64`; do not compile on the low-memory production host.
- Verify the candidate image version and source SHA, and record its SHA256.
- Inspect the current remote service before transfer. Upload/load the candidate without replacing the running container.
- Verify the candidate is a separate green slot with a real readiness gate; `/health` alone is insufficient. Do not call the release complete until the stable public endpoint has passed authenticated smoke checks after the proxy switch and the old slot remains available for rollback.
- If data synchronization is in scope, complete the migration inventory, dry-run mapping, and approved merge before cutover; keep the data evidence separate from code and image evidence.
- Publish the required maintenance notice before any restart or recreation, then cut over with `--no-build --no-deps`.
- Verify application, PostgreSQL, Redis, admin access, `/health`, public routes, authenticated `/v1/models`, and recent logs.
- After acceptance, remove only the temporary remote image archive and temporary release files requested by the user. Do not remove persistent database, Redis, proxy, TLS, or Compose configuration.

The final report must state the GitHub ref, local `main` SHA, pushed ref and read-back SHA (if pushed), image/archive digest, remote container identity, data merge policy and counts, checks performed, and any evidence that was not available, such as a real provider transaction, API credential, or non-migratable live session.
