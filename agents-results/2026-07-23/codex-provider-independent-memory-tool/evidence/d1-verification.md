# D1 Verification Evidence

Date: 2026-07-23

## Passed locally

- CLI and release unit tests: 20 passed on the maintainer Mac.
- Public Docs, route guard, and release-manifest frontend tests: 43 passed.
- Vue type check: passed.
- Full frontend ESLint: passed.
- Vite production build: passed. Existing chunk-size and mixed dynamic/static import warnings remain non-blocking.
- Disposable D2 merge/restore scenario: passed.
- Two clean release builds for version `0.1.1` were byte-identical across all five outputs.
- Every macOS, Linux, and Windows archive was extracted locally and its shared Python core completed `--help`; both POSIX launchers passed `/bin/sh -n`, and the PowerShell launcher contract was verified.
- Workflow YAML and both JSON schemas parse successfully.
- `git diff --check`: passed before this evidence update.

## Deterministic candidate hashes

| File | SHA-256 |
|---|---|
| `codex-memory-checksums.txt` | `764cae49a8b9a7413119248758a45248b678105ffcbae4ddf58772ff39b73ef7` |
| `codex-memory-release-manifest.json` | `ef9ab2966272b5c9ec7d52f268f65bbdacc5d03f7e3a04fde3a01aadfbc9851f` |
| `codex-memory_0.1.1_linux.zip` | `d034b6242580a52936bc3ad967628cdb44f498d7810a08f61253c08da76423ba` |
| `codex-memory_0.1.1_macos.zip` | `d034b6242580a52936bc3ad967628cdb44f498d7810a08f61253c08da76423ba` |
| `codex-memory_0.1.1_windows.zip` | `52cbc41744c42761263c7be9a863e82de3b0f1e96c1b620bc549aa53a2f3489d` |

These are the local `0.1.1` release-candidate hashes. Publication acceptance must match them exactly.

## Publication boundary

The authoritative release gate runs on the maintainer Mac. GitHub Releases stores the immutable outputs only after the two local builds, manifest comparison, archive execution, launcher checks, and SHA-256 verification pass. The optional `codex-memory-v*` GitHub workflow may add hosted-platform runs and Artifact Attestations when available, but its availability does not define product completion.
