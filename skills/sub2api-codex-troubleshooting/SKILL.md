---
name: sub2api-codex-troubleshooting
description: Diagnose Codex CLI or desktop configuration failures when Codex uses Sub2API or another OpenAI-compatible provider, including "Model provider ... not found", conversations that cannot continue, missing local transcripts, gateway availability failures, and local/remote state mismatches. Use when changing config.toml, model_providers, CODEX_HOME, API-key authentication, custom Responses gateways, or investigating 502/503/SSE disconnects. For image releases and production cutovers, route to the sub2api-deployment skill.
---

# Sub2api Codex Troubleshooting

Treat a Codex configuration load failure as a runtime initialization problem first. Do not conclude that conversation transcripts were deleted until the effective configuration, `CODEX_HOME`, SQLite state path, and transcript files have been checked.

For Docker builds, image upload, server updates, or production availability incidents, use `../sub2api-deployment/SKILL.md`. Never compile this project on a low-memory production server while it is serving traffic.

## Provider Configuration

`model_provider` is an exact, case-sensitive provider ID. It must match a table under `[model_providers.<id>]`.

Valid Sub2API configuration:

```toml
model_provider = "sub2api"

[model_providers.sub2api]
name = "Sub2API"
base_url = "http://localhost:8080"
wire_api = "responses"
requires_openai_auth = true
```

This is also valid, but the identifiers must remain `custom` in both places:

```toml
model_provider = "custom"

[model_providers.custom]
name = "Sub2API"
base_url = "http://localhost:8080"
wire_api = "responses"
requires_openai_auth = true
```

Never use `model_provider = "sub2api"` while defining only `[model_providers.custom]`. The desktop app reports this as `Model provider \`sub2api\` not found` and refuses to continue the conversation. `disable_response_storage` and API quota state do not cause this provider lookup error.

When the provider name is correct, save the file and restart or reopen the Codex conversation so the configuration is loaded again. Use the remote server's reachable `base_url`; do not copy a localhost URL to a different machine.

## Transcript Diagnosis

Check the error message before changing authentication or deleting state.

1. If it says `Model provider ... not found`, fix the provider ID/table mismatch first. The UI may make existing conversations look unavailable because the runtime cannot initialize; this is not proof of transcript deletion.
2. Check the effective state root. Codex stores state under `CODEX_HOME` (default `~/.codex`) and SQLite state under `CODEX_SQLITE_HOME` or the configured SQLite directory. Check shell, profile, project, and managed configuration layers for overrides.
3. Check local artifacts without printing secrets:

   - `$CODEX_HOME/sessions/**/*.jsonl`: interactive session transcripts.
   - `$CODEX_HOME/archived_sessions/`: archived transcripts.
   - `$CODEX_HOME/history.jsonl`: prompt/history index data when enabled.
   - `$CODEX_HOME/sqlite/` or `$CODEX_SQLITE_HOME`: thread and app-server indexes.
   - `$CODEX_HOME/auth.json` or the OS keychain: credentials only, not transcripts.

4. If the transcript file exists but the UI cannot load it, repair configuration and index/host visibility before attempting recovery. Never delete `sessions`, `archived_sessions`, or SQLite files during diagnosis.
5. After fixing the provider, locate a known session ID and validate with `codex resume <session_id>` or the desktop thread picker. Confirm that a new request reaches the gateway before declaring recovery complete.

## Incident Pattern

The observed failure pattern was:

```text
model_provider = "sub2api"
# only [model_providers.custom] exists
```

Every machine using that configuration showed the same "config.toml cannot be loaded; conversation cannot continue" banner. This is explained by the copied configuration error, not by simultaneous deletion of local transcripts. Existing local transcript files remained on disk and became usable again once the provider declaration was consistent.

Separate the following layers:

- Codex transcript persistence: local `CODEX_HOME` files and SQLite indexes.
- ChatGPT cloud tasks/memories: account and product-surface data; API-key mode does not automatically provide the same cloud surface.
- Sub2API routing: account selection, sticky-session hashes, failover, and upstream Responses forwarding. It does not act as the Codex desktop history store. In this project, `sessionHash` is for routing, not transcript persistence.

## Recovery Checklist

- Preserve the original `CODEX_HOME` and SQLite directory.
- Make the top-level `model_provider` exactly match `[model_providers.<id>]`.
- Verify `wire_api = "responses"`, the reachable gateway URL, and authentication requirements.
- Restart Codex or reopen the conversation after saving configuration.
- Resume one known session before testing new conversations.
- On remote hosts, repeat the checks against that host's own `CODEX_HOME`; local and remote state are not interchangeable.
- Redact API keys, OAuth tokens, cookies, and full request bodies from logs and reports.
