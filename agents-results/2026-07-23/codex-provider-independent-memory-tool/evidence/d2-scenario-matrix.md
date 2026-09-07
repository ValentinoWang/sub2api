# D2 Scenario Matrix

| Scenario | Evidence | Result |
|---|---|---|
| OAuth-labelled source to unified target | `d2-acceptance-scenario.json`, `test_merge_preserves_config_and_creates_recoverable_backup` | passed |
| Sub2API-labelled source to unified target | `d2-acceptance-scenario.json` | passed |
| Custom provider reuse of the same target | Provider is treated as a label only; target state remains provider-independent | passed by contract |
| Exact duplicate | `test_exact_identity_and_digest_are_deduplicated` | passed |
| Same identity, different content | `test_different_content_with_same_identity_is_preserved` | passed |
| Invalid JSONL | `test_invalid_jsonl_is_rejected` | passed |
| Path traversal and undeclared source | Two tampered-plan tests | passed |
| Symlink and post-plan symlink swap | Two symlink tests | passed |
| Insufficient disk space | `test_insufficient_space_is_rejected_before_backup` | passed |
| Interrupted directory swap | `test_interrupted_directory_swap_restores_original_state` | passed |
| Large task record | `test_large_task_record_is_merged_and_verified` | passed |
| Config preservation | Unit test plus `config_unchanged` acceptance field | passed |
| Backup excludes config and is inventory-scoped | `backup_excludes_config` and `backup_inventory_scoped` acceptance fields | passed |
| Credential exclusion | Rejection test plus unchanged credential sentinels | passed |
| Restore | Acceptance scenario and recoverable-backup test | passed |

The scenario uses disposable directories only. It does not read or write the user's real `~/.codex`, credentials, Redis, PostgreSQL, or any remote Sub2API instance.
