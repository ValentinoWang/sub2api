# Sub2API Merge Reference

Use this reference after the inventory and before any write. It keeps application data separate from deployment state and makes the default conflict policy explicit.

## Merge matrix

| Data | PostgreSQL/API owner | Default merge behavior | Required evidence |
|---|---|---|---|
| Users | PostgreSQL | Keep server match; add local-only users | Stable identity map and counts |
| Provider accounts | Admin account data API | Keep server match; import local-only accounts | Import result and credential test |
| API keys | PostgreSQL/admin API | Keep server match; add local-only keys | Key-to-user map without printing secrets |
| Groups and memberships | PostgreSQL/admin API | Keep server definitions and add non-conflicting local groups/memberships | Membership ID map |
| Subscriptions and balances | PostgreSQL | Keep server financial state on conflict; add non-conflicting local records | User mapping and order/idempotency evidence |
| Settings | PostgreSQL/config | Keep live server settings; add missing non-conflicting keys | Key-level diff |
| Proxies and assignments | PostgreSQL plus host config | Keep server route and assignments unless explicitly mapped | Account-to-proxy report |
| Usage and audit history | PostgreSQL | Append only after ID mapping and deduplication | Row mapping and duplicate check |
| File-backed runtime state | Local/remote deploy data | Compare manifests and hashes; preserve server network files unless explicitly selected | File list, hashes, and ownership |
| Redis cache/stickiness | Redis | Optional and best-effort; never require exact key equality | Persistence mode, key counts, TTL caveat |

## Identity and conflict rules

- Prefer immutable provider/account identifiers, normalized user email, API-key owner, and durable order or idempotency identifiers. Never match records by row ID alone.
- Preserve target primary keys. Translate source foreign keys through the mapping before inserting dependent rows.
- Treat credentials, balances, subscriptions, and payment/order state as sensitive conflicts. Keep the server value by default and stop for an unresolved discrepancy.
- Deduplicate usage, audit, and event rows only with a documented stable identifier; otherwise leave the source row pending review.
- Do not use a full PostgreSQL restore for a merge. It replaces target-only business data and can invalidate encrypted credentials or target proxy records.

## Secret handling

Compare secret fingerprints, not values. The account data export is intentionally sensitive because it contains plaintext credentials during transfer. Use an authenticated HTTPS connection, restrictive temporary permissions, and immediate cleanup. Never put an export, JWT, API key, password, or encryption key in Git, logs, command arguments, or the final report.

## Redis boundary

Redis snapshots are only meaningful after the PostgreSQL identity map is stable. Check whether AOF or RDB is active; replacing only `dump.rdb` on an AOF deployment is unsafe. Expired keys and process-local response/connection bindings cannot be made exactly equal.
