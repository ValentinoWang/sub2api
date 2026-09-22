# Read-only schema drift audit

## Decision-ready result

Append a new `240_canonical_legacy_schema_alignment.sql` migration after refund migration 239. Preserve the production DeepMath WS trigger behavior (`oauth`: store recovery `false`; `apikey`: `true`; both use `ctx_pool`) and add `groups.models_list_config JSONB NOT NULL DEFAULT '{}'::jsonb` only where absent. Do not update existing account rows or merge/delete legacy group configuration. The current application reads `model_allowlist`; legacy `models_list_config` is retained compatibility data.

This recommendation preserves production behavior and real data while bringing the two logical schemas into agreement. It does not claim existing account configuration values are equal, nor should accounts, credentials, balances, or inventory be copied to manufacture parity.

No application source, migration source, database state, or migration history was modified during this audit. No tests were run. This file is the sole audit artifact written by this lane.

## Evidence inspected

- `acceptance/dev-schema-before.sql` and `acceptance/prod-schema-before.sql` are the supplied sanitized schema snapshots.
- `acceptance/pre-cutover-migrations.json` contains 292 development and 294 production applied records. All shared filenames have identical recorded hashes.
- A local read-only `docker exec sub2api-postgres postgres --version` returned PostgreSQL 18.4. The root agent independently supplied production 18.6. The sanitized dumps omit their version headers. Both therefore use major 18; differing constraint renderings must not be dismissed solely as a major-version difference.
- `backend/migrations/235_group_model_allowlist.sql` renames the old column when only it exists. `236_group_model_allowlist_repair.sql` explicitly preserves the old column if both exist, filling an empty new column from old data during that historical migration. The proposed alignment must not repeat that data backfill arbitrarily.
- `backend/ent/schema/group.go` declares the current `model_allowlist` field. Runtime source searches found no current data access to `models_list_config` outside migration repair coverage.
- `backend/internal/service/account.go` reads `extra.openai_ws_allow_store_recovery`. `openai_ws_forwarder_payload.go` allows recovery when either the account flag or global gateway flag is true; account `false` does not override global `AllowStoreRecovery=true`. The default in `config.go` is false. A runtime policy comparison must examine the effective global setting separately without exposing credentials.

## Exact drift classification

| Difference | Classification | Handling |
|---|---|---|
| Bodies of `enforce_deepmath_openai_ws_policy_on_account()` and `enforce_deepmath_openai_ws_policy_on_membership()` | Semantic: development permits OAuth recovery; production disables it | Pin the production bodies into new migration 240 with `CREATE OR REPLACE FUNCTION`; require both canonical definitions afterward |
| Production-only `groups.models_list_config` | Real extra retained column, `jsonb`, non-null, default `{}` | Add the same unused compatibility column to development if absent; preserve production contents and existing `model_allowlist` |
| `codex_continuity_turns.client_window_id` physical position versus the created/committed timestamps | Physical column order only; name/type/default/nullability match | Compare columns by schema/table/name, keeping type/default/nullability and constraints exact |
| 25 CHECK expression renderings | Equivalent constant `varchar[]` to `text[]` cast forms | Narrowly canonicalize this specific literal-array cast equivalence; preserve all constraint names, literal values, operators, validation state, and every other expression token |
| Other whitespace in the two WS functions | Formatting mixed with a real policy difference | Do not normalize away function semantics; migration 240 pins identical function source |

Both dumps contain 108 named CHECK constraints with the same names. Twenty-five differ textually; every one became identical after converting only the observed literal-varchar-array-to-text cast representation. No CHECK was omitted. All 335 indexes match byte-for-byte, including definitions; no index exists on only one side.

As a diagnostic cross-check, after normalizing the narrowly identified cast representation and sorting complete column definitions within their table, and isolating the two already-identified function bodies for their separate semantic review, the complete remaining dump difference is only production's additional `models_list_config` column. This diagnostic isolation is not permission for a parity gate to omit those functions or CHECKs.

## Proposed migration 240, exact scope

1. In one normal transactional migration, execute `ALTER TABLE groups ADD COLUMN IF NOT EXISTS models_list_config JSONB NOT NULL DEFAULT '{}'::jsonb;`. Validate the resulting column shape; unexpected existing type/default/nullability should fail the canonical schema check rather than silently overwriting data. Keep `model_allowlist` and both existing column values unchanged.
2. Use `CREATE OR REPLACE FUNCTION` for the two exact production function definitions in the supplied production schema. Preserve the existing DeepMath membership predicate, active-group condition, OpenAI account/type scope, `ctx_pool` keys, JSON key-specific updates, `updated_at`, and `scheduler_outbox` event behavior. Change no production branch semantics.
3. Ensure their existing triggers have the exact current scopes: `accounts_enforce_deepmath_openai_ws_policy` is `BEFORE UPDATE OF platform, type, extra` on `accounts`; `account_groups_enforce_deepmath_openai_ws_policy` is `AFTER INSERT OR UPDATE OF account_id, group_id` on `account_groups`. Transactional `DROP TRIGGER IF EXISTS` plus `CREATE TRIGGER` gives new and historical installations the same definitions without firing them on existing rows.
4. Include no `UPDATE accounts`, group-config backfill, account membership change, credential mutation, ledger rewrite, or creation of fake historical records. Replacing functions changes future enforcement; it does not rewrite existing `extra` values. Any existing flag differences should be reported as account configuration, not silently corrected as schema work.
5. Let the normal runner apply and record migration 240 using its real checksum. Historical 194/195 filenames remain historical only. After implementation, use isolated migration tests for fresh/legacy schema shapes and preservation of existing account/group values; a new test account's subsequent update can verify the pinned trigger behavior. Those tests were not executed by this read-only lane.

## Migration ledger parity

The current tree at audit time contains 286 SQL migration files. Both snapshots have already applied 285 of them with zero checksum mismatches; only new refund migration 239 is pending. The correct checksum convention is the runner's SHA-256 of `strings.TrimSpace(content)`, not raw file-byte SHA-256. Migration 240 will become another real pending file once implemented.

Seven retired records are shared by both databases: `231_codex_continuity.sql`, `232_codex_continuity_client_window.sql`, `233_user_lifecycle_emails.sql`, `234_user_first_topup_bonus.sql`, `235_liandong_sales_channel.sql`, `236_membership_fulfillment.sql`, and `237_membership_coupon_late_payment.sql`. The consolidated 238 schema does not erase those historical facts.

Production additionally retains:

| Historical filename | Recorded checksum |
|---|---|
| `194_deepmath_openai_ws_policy.sql` | `2617b3736727c66bff37736cb54ee64d73bb1f1912a52831cc4dc52081fa6b1c` |
| `195_deepmath_stored_responses_apikey_policy.sql` | `eb6329ba91f938671e1abb350422521ba399c5e3d14c9fdd94d348d47b4e1ffe` |

A release parity collector should pin the required current migration filenames and runner-compatible checksums from the candidate Git snapshot, verify every required file is applied on both databases, and separately report recognized retired filename/checksum pairs from the captured ledger. Preserve all historical rows. Missing required files, changed checksums, unexpected retired hashes, and unknown extra records block parity. Do not require raw applied-record counts to match and do not fake development application of production-only retired files.

## Logical schema acceptance representation

Retain raw schema dumps as evidence, then compare a structured catalog snapshot. Key columns by table and column name rather than ordinal position. Keep SQL type/length, nullability, default/generated/identity expressions, collation, named constraints and validation state, primary/unique/foreign keys, referential actions, index method/predicate/expressions, trigger timing/events/conditions, and complete function definitions visible to comparison. Do not broadly discard CHECK text, functions, legacy columns, or unknown objects.

For the observed enum-like CHECKs only, the two renderings `ANY ((ARRAY['x'::varchar, ...])::text[])` and `ANY (ARRAY[('x'::varchar)::text, ...])` are equivalent here because all elements are explicit string literals with no length coercion. A canonicalizer may map these to the same ordered literal-text array while retaining surrounding operators and expressions. Nonliteral arrays, length-limited casts, changed literals, or other unrecognized forms must remain unequal. Pinning the normalization to this observed class prevents schema drift from disappearing behind a generic text scrubber.
