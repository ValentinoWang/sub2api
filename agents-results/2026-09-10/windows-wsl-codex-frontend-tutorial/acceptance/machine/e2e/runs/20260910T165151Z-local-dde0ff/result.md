# Acceptance Run: 20260910T165151Z-local-dde0ff

- Run ID: 20260910T165151Z-local-dde0ff
- Task ID: windows-wsl-codex-frontend-tutorial
- Lane: machine/e2e
- Status: PASS
- Acceptance contract: agents-results/2026-09-10/windows-wsl-codex-frontend-tutorial/acceptance-contract.md
- Contract version: 1
- Contract SHA-256: 9221df47eac56d5f5b1e9839bbb4ab53e82dcbe3a20088186c6d05ed1d331db3
- Source identity: 20189b7348fa74020e066241890c240e0c28cad4+worktree-dde0ffb9e5fd
- Runtime identity: vite-hmr-127.0.0.1:4174
- Executor or reviewer: Codex Playwright Chromium
- Started at: 2026-09-10T16:52:17.964098Z
- Completed at: 2026-09-10T16:53:11Z
- Evidence directory: evidence/

## Scope

The rewritten public experience page at the fixed HMR runtime, including its reader-facing first viewport, role labels, desktop and mobile layout, content counts, and horizontal overflow behavior.

## Procedure

1. Loaded the public tutorial route from `127.0.0.1:4174` in Playwright Chromium.
2. Forced the existing light theme preference for deterministic screenshots.
3. Read the title, guide, content counts, and document scroll width from the rendered DOM.
4. Captured and reviewed 1280x720 and 390x844 screenshots.

## Requirement disposition

| Requirement | Result | Evidence | Notes |
| --- | --- | --- | --- |
| AC-02 | PASS | evidence/browser-observations.json; evidence/screenshot-manifest.json | The page reads as a beginner experience article and clearly distinguishes reader and Codex actions. |
| AC-04 | PASS | evidence/browser-observations.json; evidence/screenshot-manifest.json | Both viewports render without page-level horizontal overflow, clipping, or incoherent overlap. |

## Findings

None on the tutorial surface. The compact shared navigation retains its existing behavior outside this page's ownership.

## Evidence manifest

| Artifact | SHA-256 | Meaning |
| --- | --- | --- |
| evidence/browser-observations.json | 84bca9b42a6ae32c09a5d1a87cb80507793c0f0590bdad58ef45bc02e54458be | Route, content, role, and viewport measurements |
| evidence/screenshot-manifest.json | 9650b007341c0e6822cf9c81048b0c9e0f803346d457b24a75823296c321df92 | Reviewed screenshot identities and hashes |
| evidence/screenshots/tutorial-desktop-1280x720.png | 098aa96a180046bdcd896db5d0619b058c52716dc3bd6f7bf3c6300c71f050fd | Desktop first viewport |
| evidence/screenshots/tutorial-mobile-390x844.png | b274e83af7fc2f385b3b8857f53959662eeb8752c38b624b838199c0d01338d7 | Mobile first viewport |

## Unverified items

Real Windows 11 and WSL2 execution, clipboard behavior in other browsers, dark-theme pixel review, and beginner comprehension.

## Conclusion

AC-02 and AC-04 pass for the rewritten local HMR source and runtime. Human H-01 remains pending.
