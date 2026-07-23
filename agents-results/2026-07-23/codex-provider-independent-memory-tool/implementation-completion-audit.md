# Codex Provider-Independent Memory Tool Implementation Audit

Date: 2026-07-23

## Result

The implementation, public GitHub Release, and production deployment are complete and directly verified. Overall completion is not 100% because GitHub hosted CI did not execute and no Artifact Attestation was generated.

| Node | Result | Evidence |
|---|---|---|
| A1 boundary audit | done | SSOT state-layer table, `source-notes.md`, continuity documentation |
| A2 contract freeze | done | `openproblem.md`, plan and release JSON schemas |
| B1 local unifier | done | `tools/codex-memory-unifier/`, 19 passing tests, D2 scenario |
| B2 release implementation | partial | published Release and verified assets exist; hosted Ubuntu/macOS/Windows jobs failed before all steps due to a GitHub billing lock, so no attestation exists |
| B3 public Docs | done and deployed | public routes, Home/Login/admin links, Markdown-backed page, 43 passing tests, visual evidence, production HTTP 200 |
| C-M1 integration | verified and deployed | one generated release manifest contract, one fork-maintained documentation source, production manifest and downloads verified |
| D1 verification | done | `evidence/d1-verification.md` |
| D2 recovery exercise | done | `evidence/d2-acceptance-scenario.json`, `evidence/d2-scenario-matrix.md` |
| D3 audit | partial | this report; final acceptance remains blocked by B2 hosted CI and attestation |

## Safety Findings

- Only `memories/`, `sessions/`, and `archived_sessions/` are merged.
- Credentials, OAuth state, API keys, keychain data, Redis, and PostgreSQL are excluded.
- `config.toml` is parsed and hash-checked but is not rewritten or copied into backups.
- Source homes remain untouched. Merge and restore require explicit confirmation and a declaration that active requests have ended.
- Exact identity plus exact SHA-256 is the only deduplication rule. Different content is preserved with provenance.
- Path traversal, undeclared sources, symlinks, post-plan symlink swaps, malformed JSONL, insufficient space, and interrupted swaps are covered by tests.

## Publication And Production Evidence

The public Release exists at `https://github.com/ValentinoWang/sub2api/releases/tag/codex-memory-v0.1.0`, and its macOS, Linux, Windows, checksum, and manifest assets are downloadable. Production `43.136.113.101` runs the exact locally built `linux/amd64` image for commit `ae2705973300327ada0b7590826f6b7b66aa5143`. Public Docs, administrator authentication, PostgreSQL, Redis, migrations, and a real streamed `/responses` completion passed after cutover. Full evidence is in `evidence/production-publication-acceptance.md`.

GitHub Actions run `30000043556` is a real failed run, not acceptance evidence: attempts 1, 2, and 3 all ended with all three hosted operating-system jobs failing before executing a step, and the release job was skipped. The attempt-3 Check annotation confirms that the account is locked due to a billing issue. Therefore:

- local implementation: complete;
- public Release and download availability: complete;
- production deployment and online acceptance: complete;
- hosted Ubuntu/macOS/Windows CI: blocked externally;
- GitHub Artifact Attestation: missing because the gated release job never ran;
- overall final acceptance: partial, not 100%.
