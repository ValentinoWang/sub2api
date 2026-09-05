---
name: sub2api-migration
description: Merge and verify Sub2API business data between a local deployment and a server, including users, provider accounts, API keys, groups, subscriptions, proxies, settings, and usage history. Use when local and remote Sub2API data must be made consistent without silently losing either side.
---

# Sub2API Data Migration

Use this skill when the request is to make a local and server Sub2API deployment consistent. Treat the application data and the server's deployment infrastructure as separate state. The target server keeps its Compose files, environment values, reverse proxy, TLS, tunnel, firewall, and proxy route unless the user explicitly asks to replace them.

## Default decision

Use a merge unless the user explicitly says that one side should overwrite the other. The default conflict policy is:

- Keep the server's existing user, account, API-key, group, subscription, setting, proxy, order, and usage record when the same logical record exists on both sides.
- Add local records that do not exist on the server.
- Never delete a server record merely because it is absent locally.
- Record every unresolved identity or field conflict and stop before applying it.

Confirm the source and target, the backup policy, and the conflict policy when the request changes these defaults. A read-only inventory and dry-run diff may proceed before that decision.

After a successful merge, treat the server as the runtime authority. Refresh the local database and file-backed state from the recorded server snapshot before the next release, or explicitly record a later local change as a new promotion candidate. “Consistent” means the approved PostgreSQL records and file manifests converge; it does not mean every Redis TTL or active connection is identical.

## Data planes

1. Inventory PostgreSQL, Redis, application/schema versions, table counts, stable identifiers, proxy assignments, and encryption-key fingerprints on both sides. Do not print passwords, tokens, OAuth credentials, API keys, or key material.
2. PostgreSQL contains users, provider accounts, encrypted credentials, API keys, groups, subscriptions, proxy records, settings, orders, audit records, and usage history. A relational merge must build explicit source-to-target ID maps before copying join or usage rows.
3. Provider account credentials must use the authenticated account data export/import API (`GET /api/v1/admin/accounts/data` and `POST /api/v1/admin/accounts/data`). Keep the export in a mode-`600` temporary file or memory, never display it, and delete it after verification. Do not copy encrypted rows directly when the target encryption key differs.
4. Compare `JWT_SECRET` and `TOTP_ENCRYPTION_KEY` by fingerprint. A full authoritative restore requires the target application to use the source keys; a merge must preserve the target keys and re-encrypt imported credentials through the API.
5. Merge API keys, groups, subscriptions, settings, proxy assignments, orders, and usage logs separately when the account API does not include them. Match users and accounts by stable identity, preserve target IDs, and deduplicate records using their durable order, idempotency, or event identifiers where available.
6. Compare file-backed runtime state separately, including `deploy/data/config.yaml`, model-pricing manifests, and Mihomo subscription/config files. Preserve the server's network and proxy configuration unless explicitly mapped; record hashes for any file that is intentionally synchronized.
7. Redis is a separate, optional continuity plane. Check persistence mode before considering a snapshot. Only migrate compatible persistent keys after PostgreSQL IDs match; do not promise equality for expiring cache entries, live WebSocket connections, process-local response bindings, or active tool turns.
8. Codex memories and task transcripts remain on the Codex client host. They are not Sub2API data and must not be copied, deleted, or used as a migration source.

## Execution

1. Capture a non-secret inventory and a dry-run merge report for both deployments.
2. Resolve identity mappings and list target-only, source-only, and conflicting records. Do not write while mappings are incomplete.
3. Follow the confirmed backup policy. Migration-only dumps, plaintext account exports, secret files, and temporary scripts are removed after validation unless retention was explicitly requested.
4. Publish the required maintenance notice before stopping the application, PostgreSQL, or Redis. Uploading an image and running `docker load` while the current application remains running does not require an outage notice.
5. Merge account credentials through the admin API, then apply relational records in dependency order: users, groups and providers, accounts, API keys and memberships, subscriptions, proxies and assignments, settings, orders, usage, and audit records.
6. Keep target-specific infrastructure and reapply any target proxy or tunnel rows that are intentionally outside the source data. Recreate only the application services needed for changed environment or schema state.
7. If Redis migration is selected, stop writers, use a version-compatible snapshot method for the active persistence mode, restore ownership and permissions, and verify `PING`, key counts, and relevant sticky-session keys before starting the application.

## Acceptance

Require fresh evidence for both sides of the merge:

- Expected users, provider accounts, API keys, groups, subscriptions, settings, proxies, orders, and usage records exist according to the merge report.
- Every imported encrypted account can be read or tested through the admin API without a decryption error.
- Target-only records remain present, and local-only records appear exactly once.
- IDs in memberships, subscriptions, proxy assignments, orders, and usage rows point to the mapped target records.
- Application, PostgreSQL, Redis, `/health`, public routes, and authenticated admin access are healthy.
- Recent logs contain no migration, decryption, database, Redis, or continuity errors.
- All temporary dumps, exports, archives, and secret files are removed or listed as intentionally retained.

Report what was merged, preserved, skipped, deleted, and not migratable. Do not call the deployments identical when Redis TTL state, live sessions, process-local state, or unverified credentials remain different.
