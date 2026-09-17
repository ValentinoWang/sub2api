# Canonical schema alignment implementation

Migration `240_canonical_legacy_schema_alignment.sql` adds the retained legacy group column only if absent and installs the exact captured production DeepMath WS functions and trigger scopes. OAuth store recovery stays false and API-key recovery true under that policy. The migration performs no account-data backfill, group-config merge, credential change, or migration-ledger rewrite.

The new integration regression fixture file contains the two actual pre-alignment function definitions from the sanitized development and production dumps. The regression test restores each prior column shape in a rollback-only transaction, seeds conflicting current/legacy group configurations and sentinel account data, then verifies:

- Both prior schemas converge to identical function definitions, triggers, and column metadata.
- Applying migration 240 twice leaves all account rows, existing group values, migration history, and scheduler outbox unchanged.
- Production legacy-column values remain intact; development receives the empty default only for the newly added compatibility column.
- Subsequent account updates and new memberships enforce OAuth=false/API-key=true, preserve unrelated JSON values, set `ctx_pool`, and emit the existing membership scheduling event.

Two migration-236 tests now explicitly reconstruct their historical input column shape inside their existing rollback-only transaction because the complete current schema includes the retained legacy column. Every pre-existing assertion remains intact.

Validation completed with exit 0:

```text
GOMAXPROCS=2 go test -tags integration -p 1 ./internal/repository -run 'TestMigration(236|240)' -count=1 -v
ok github.com/Wei-Shaw/sub2api/internal/repository 6.561s
```

The command ran from `backend/`. Complete output is in `schema-alignment-integration.log`. The migration-240 development and production subtests and all three migration-236 tests passed. The two temporary PostgreSQL/Redis containers identified in that log were automatically removed before an explicit cleanup attempt; a subsequent container inventory confirmed both absent. No retained development/production container was changed. `git diff --check` passed.

The later live catalog snapshots independently corroborate the bounded scope: `acceptance/dev-logical-schema-before.json` and `acceptance/prod-logical-schema-before.json` differ only in columns and functions. They contain 1,616 versus 1,617 columns; only the two named DeepMath function definitions differ among 80 functions. Every other captured section matches, including all 363 constraints, 482 catalog indexes, and 13 triggers. These catalog counts include object classes beyond the explicit CREATE INDEX/CHECK statements counted in the earlier sanitized dump audit.

`known-retired-migrations.json` supplies the nine exact historical filename/checksum pairs to the collector lane. Required current migrations must be verified against the candidate release's trimmed-content checksums; recognized retired records remain separately visible history, and unknown extras must fail parity. Neither migration 240 nor its tests fabricate applied records.

This is focused local machine evidence. The combined candidate's full local CI and actual environment application remain root-owned gates.
