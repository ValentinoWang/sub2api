# D1 Verification Evidence

Date: 2026-07-23

## Passed locally

- CLI and release unit tests: 19 passed.
- Public Docs, route guard, and release-manifest frontend tests: 43 passed.
- Vue type check: passed.
- Full frontend ESLint: passed.
- Vite production build: passed. Existing chunk-size and mixed dynamic/static import warnings remain non-blocking.
- Disposable D2 merge/restore scenario: passed.
- Two clean release builds for version `0.1.0` were byte-identical.
- Workflow YAML and both JSON schemas parse successfully.
- `git diff --check`: passed before this evidence update.

## Deterministic candidate hashes

| File | SHA-256 |
|---|---|
| `codex-memory-checksums.txt` | `03cc2c6917d77644b086eca0e4bc6d77413e79982fbf5b766c35329971845200` |
| `codex-memory-release-manifest.json` | `5dba29e1840e6bdfe87b8db691f7c393976b2a408e0a5facf5e8817d55c8a947` |
| `codex-memory_0.1.0_linux.zip` | `63d9d7707e7572875cd505d1ed044081a2999c85113bd13f2ab3c549049387e1` |
| `codex-memory_0.1.0_macos.zip` | `63d9d7707e7572875cd505d1ed044081a2999c85113bd13f2ab3c549049387e1` |
| `codex-memory_0.1.0_windows.zip` | `b9177f9139dadcf33703a6f1f93ddc6f5a8152911ec86d91e9076d548b6ecd1a` |

These are disposable local candidate hashes, not published release identifiers.

## Publication boundary

The dedicated `codex-memory-v*` release workflow defines Ubuntu, macOS, and Windows tests, gates artifact construction on that matrix, verifies the committed website manifest byte-for-byte, creates GitHub Artifact Attestations, and attaches the generated assets to a GitHub Release. At the time of this evidence snapshot, no hosted workflow run, GitHub Release, or attestation had yet been created; later publication evidence must be recorded separately.
