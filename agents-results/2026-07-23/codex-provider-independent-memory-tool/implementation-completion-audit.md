# Codex Provider-Independent Memory Tool Implementation Audit

Date: 2026-07-23

## Result

The implementation and existing production deployment are verified. The corrected v5 completion contract uses the maintainer Mac for all three platform artifacts; GitHub-hosted CI and Artifact Attestation are optional. Local `0.1.1` acceptance has passed, while publication and production manifest rollout are still pending.

| Node | Result | Evidence |
|---|---|---|
| A1 boundary audit | done | SSOT state-layer table, `source-notes.md`, continuity documentation |
| A2 contract freeze | done | `openproblem.md`, plan and release JSON schemas |
| B1 local unifier | done | `tools/codex-memory-unifier/`, 19 passing tests, D2 scenario |
| B2 release implementation | verified locally | `0.1.1` built twice on the Mac with byte-identical outputs; 20 tests, three archive executions, launcher contracts, manifest and SHA-256 passed |
| B3 public Docs | done and deployed | public routes, Home/Login/admin links, Markdown-backed page, 43 passing tests, visual evidence, production HTTP 200 |
| C-M1 integration | verified locally | one `0.1.1` release manifest contract and one fork-maintained documentation source; publication and production readback pending |
| D1 verification | done | `evidence/d1-verification.md` |
| D2 recovery exercise | done | `evidence/d2-acceptance-scenario.json`, `evidence/d2-scenario-matrix.md` |
| D3 audit | verified locally | final acceptance awaits the `0.1.1` Release and production readback |

## Safety Findings

- Only `memories/`, `sessions/`, and `archived_sessions/` are merged.
- Credentials, OAuth state, API keys, keychain data, Redis, and PostgreSQL are excluded.
- `config.toml` is parsed and hash-checked but is not rewritten or copied into backups.
- Source homes remain untouched. Merge and restore require explicit confirmation and a declaration that active requests have ended.
- Exact identity plus exact SHA-256 is the only deduplication rule. Different content is preserved with provenance.
- Path traversal, undeclared sources, symlinks, post-plan symlink swaps, malformed JSONL, insufficient space, and interrupted swaps are covered by tests.

## Corrected Release Gate

The product owner explicitly requires local builds. The authoritative v5 release gate therefore runs on the maintainer Mac and proves the three platform script archives through deterministic reconstruction, shared-core execution, launcher checks, manifest equality, and SHA-256 readback. The `0.1.1` candidate passed this gate. GitHub-hosted CI and Artifact Attestation are optional provenance only and do not affect completion.

The earlier `0.1.0` Release and production deployment remain valid historical evidence. The failed GitHub Actions run `30000043556` is retained only as evidence of the discarded hosted path; it is not a product blocker. Current state:

- local implementation and v5 release gate: complete;
- `0.1.1` public Release: pending;
- production manifest and online acceptance for `0.1.1`: pending;
- hosted CI and Artifact Attestation: optional, not applicable to completion;
- overall final acceptance: partial until publication and production readback.
