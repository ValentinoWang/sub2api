# Acceptance Run: 20260910T165151Z-static-dde0ff

- Run ID: 20260910T165151Z-static-dde0ff
- Task ID: windows-wsl-codex-frontend-tutorial
- Lane: machine/static
- Status: PASS
- Acceptance contract: agents-results/2026-09-10/windows-wsl-codex-frontend-tutorial/acceptance-contract.md
- Contract version: 1
- Contract SHA-256: 9221df47eac56d5f5b1e9839bbb4ab53e82dcbe3a20088186c6d05ed1d331db3
- Source identity: 20189b7348fa74020e066241890c240e0c28cad4+worktree-dde0ffb9e5fd
- Runtime identity: frontend local toolchain
- Executor or reviewer: Codex
- Started at: 2026-09-10T16:52:17.896719Z
- Completed at: 2026-09-10T16:53:11Z
- Evidence directory: evidence/

## Scope

The reader-facing tutorial copy, ten-step content structure, experience index entry, public route, prerender output, copy interactions, responsive CSS, TypeScript, lint, and frontend regression suite.

## Procedure

1. Ran the three focused tutorial, experience index, and publication test files.
2. Ran the complete frontend Vitest suite.
3. Ran Vue TypeScript checking and full frontend ESLint.
4. Ran the Git whitespace check.

## Requirement disposition

| Requirement | Result | Evidence | Notes |
| --- | --- | --- | --- |
| AC-01 | PASS | evidence/test-summary.json | The experience card, route, prerender output, and existing articles remain covered. |
| AC-02 | PASS | evidence/test-summary.json | Tests require the reader guide, ten steps, distinct input labels, responsibility labels, review flow, and practice task. |
| AC-03 | PASS | evidence/test-summary.json | Copy success/failure, TypeScript, full lint, and all frontend tests pass. |

## Findings

None. Existing fixture warnings remained non-blocking and the complete suite exited successfully.

## Evidence manifest

| Artifact | SHA-256 | Meaning |
| --- | --- | --- |
| evidence/test-summary.json | 24b49aa0c79f1c18439a744fec7d8d579d8e8922f7cc0180e4436bb1a4ba1e00 | Focused and full frontend validation summary |

## Unverified items

Real Windows 11 or WSL2 execution, third-party repository behavior, browser rendering, and beginner comprehension.

## Conclusion

AC-01, AC-02, and AC-03 pass for source identity `20189b7348fa74020e066241890c240e0c28cad4+worktree-dde0ffb9e5fd`.
