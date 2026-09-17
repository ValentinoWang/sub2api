# Acceptance Run: 20260910T162756Z-local-7dbbdb

- Run ID: 20260910T162756Z-local-7dbbdb
- Task ID: windows-wsl-codex-frontend-tutorial
- Lane: machine/e2e
- Status: PASS
- Acceptance contract: agents-results/2026-09-10/windows-wsl-codex-frontend-tutorial/acceptance-contract.md
- Contract version: 1
- Contract SHA-256: 9221df47eac56d5f5b1e9839bbb4ab53e82dcbe3a20088186c6d05ed1d331db3
- Source identity: 20189b7348fa74020e066241890c240e0c28cad4+worktree-7dbbdb86976a
- Runtime identity: vite-hmr-127.0.0.1:4174
- Executor or reviewer: Codex Playwright Chromium
- Started at: 2026-09-10T16:27:56.792725Z
- Completed at: 2026-09-10T16:30:50Z
- Evidence directory: evidence/

## Scope

The public tutorial route at the fixed local HMR runtime, its desktop and mobile first viewport, light-theme hierarchy, responsive layout, and visible tutorial identity. This run does not exercise Windows, WSL2, authentication, the installation script, or backend writes.

## Procedure

1. Loaded the route from the fixed Vite HMR server and waited for `#tutorial-title`.
2. Forced the existing `theme=light` preference through an isolated Playwright storage state.
3. Captured Chromium screenshots at 1280x720 and 390x844.
4. Reviewed both captures for page identity, information hierarchy, clipping, overlap, and responsive single-column behavior.
5. Reused the browser DOM measurements recorded by the visual-fidelity run for the full-page overflow assertion.

## Requirement disposition

| Requirement | Result | Evidence | Notes |
| --- | --- | --- | --- |
| AC-02 | PASS | evidence/screenshot-manifest.json | The tutorial title, responsibility model, goals, and workflow are visibly rendered. |
| AC-04 | PASS | evidence/screenshot-manifest.json; ../../../../visual-fidelity/runs/20260910T161231Z-visual-7dbbdb/evidence/browser-observations.json | Both declared viewports render coherently; the mobile DOM measurement reports no page-level horizontal overflow. |

## Findings

The page itself rendered without clipping or overlap in both captures. The site navigation keeps its existing compact mobile behavior. This task did not modify that shared navigation.

## Evidence manifest

| Artifact | SHA-256 | Meaning |
| --- | --- | --- |
| evidence/screenshots/tutorial-desktop-1280x720.png | 612a34778cd37802c98f2aea3462f5dd41a573535fc3f18e0b9bb86f0c2696d2 | Reviewed light-theme desktop viewport |
| evidence/screenshots/tutorial-mobile-390x844.png | 9f018acb60c2109dcfff19d7d6f4c434d62a4808fd3752185eaed4eebd5de1b4 | Reviewed light-theme mobile viewport |
| evidence/light-theme-storage.json | 46c4a038f934e27a2f87e5b74a583424c11870e8e7f54f48658d59e1a5ebaaea | Isolated theme preference used for deterministic capture |

## Unverified items

Full execution on Windows 11 and WSL2, clipboard behavior in browsers other than the inspected Chromium runtime, dark-theme pixel review, and beginner comprehension remain outside this run.

## Conclusion

AC-02 and AC-04 pass for the declared local HMR source and runtime. Human H-01 remains pending.
