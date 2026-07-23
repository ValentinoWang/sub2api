# Codex Memory Unifier

This CLI keeps local Codex memories and task transcripts independent from the selected `model_provider`. It merges only:

- `$CODEX_HOME/memories/`
- `$CODEX_HOME/sessions/`
- `$CODEX_HOME/archived_sessions/`

It never merges `auth.json`, OAuth tokens, API keys, keychain data, or Sub2API Redis/PostgreSQL state. The target `config.toml` is parsed and hash-checked but never rewritten by the merge.

Python 3.11 or newer is required. End active streaming responses and tool calls before applying a merge or restore.

Verify the downloaded ZIP against `codex-memory-checksums.txt` before extracting it:

```bash
shasum -a 256 -c codex-memory-checksums.txt
```

The release gate runs on the maintainer's Mac: it builds all three platform archives twice, requires byte-identical output, verifies the manifest and SHA-256 file, executes the shared Python entry point from every extracted archive, and validates both launcher formats. GitHub-hosted CI or Artifact Attestation may be added as optional provenance, but neither is required to use or verify the release. The deliverables are Python scripts rather than native macOS binaries, so Apple code signing and notarization do not apply.

## Plan

```bash
./codex-memory plan \
  --source "$HOME/.codex-account-a" \
  --source "$HOME/.codex-account-b" \
  --target "$HOME/.codex" \
  --output "$HOME/codex-memory-plan.json"
```

Review the JSON summary and destinations. Exact identity plus SHA-256 matches are deduplicated. Different content is preserved with a source-labelled filename.

## Merge

```bash
./codex-memory merge \
  --plan "$HOME/codex-memory-plan.json" \
  --confirm \
  --confirm-no-active-requests \
  --activate
```

The command creates a local backup before writing, leaves source homes untouched, writes provenance to `$CODEX_HOME/.codex-memory-unifier/provenance.json`, and configures the user-scoped `CODEX_HOME`. Restart Codex after activation.

## Restore

Use the backup path returned by `merge`:

```bash
./codex-memory restore \
  --backup "/path/to/.codex-memory-backups/backup-id" \
  --target "$HOME/.codex" \
  --confirm \
  --confirm-no-active-requests
```

Restore creates another safety backup before changing the target. Backups and original source homes are never automatically deleted.

To upgrade, verify and extract the newer ZIP over the tool installation directory. To uninstall, remove that directory. Activation files are reported by `activate`; remove those files and unset the user-level `CODEX_HOME` only when intentionally returning to the platform default.
