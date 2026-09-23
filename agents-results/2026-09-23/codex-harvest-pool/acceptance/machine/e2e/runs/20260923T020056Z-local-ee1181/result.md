# Acceptance Run: 20260923T020056Z-local-ee1181

- Run ID: 20260923T020056Z-local-ee1181
- Task ID: codex-harvest-pool
- Lane: machine/e2e
- Status: PASS
- Acceptance contract: agents-results/2026-09-23/codex-harvest-pool/acceptance-contract.md
- Contract version: 1
- Contract SHA-256: 60c38dad882947017a8f894c00db505e8e78c7924172a13807926776d7fdc636
- Source identity: a1bbccf62e7d482c3da774ebf6af5b4e939144e1
- Runtime identity: vite-dev-127.0.0.1:4174
- Executor or reviewer: Claude Playwright Chromium
- Started at: 2026-09-23T02:00:56.908074Z
- Completed at: 2026-09-23T02:06:47Z
- Evidence directory: evidence/

## Scope

The new admin Codex tickets page (`/admin/codex-harvest`) rendered from the committed source by the Vite dev server on the fixed port 4174, in Playwright Chromium (headless shell 1228) at 1440x900 and 390x844, light theme, zh locale. Every `/api/` request was answered by deterministic fixtures from `agents-results/2026-09-23/codex-harvest-pool/e2e_capture.py`; no backend, database, upstream or credential was involved. Excludes API, persistence, permission and real harvesting behavior, which are covered by the unit and integration runs in local CI.

## Procedure

1. `python e2e_capture.py frontend <run>/evidence a1bbccf62e7d482c3da774ebf6af5b4e939144e1` with `PLAYWRIGHT_CHROMIUM` pointing to the cached headless shell; the script started Vite on 4174 and stopped it afterwards.
2. Seeded an admin session and the completed admin tour in localStorage, opened `/admin/codex-harvest`, waited for the harvest log section.
3. Read section titles, row counts, selected preset, document scroll width and whether the fixture ticket state or proxy password appear in the DOM (`evidence/browser-observations.json`).
4. Captured full-page screenshots for both viewports and the manual harvest dialog on desktop, then reviewed them.

## Requirement disposition

| Requirement | Result | Evidence | Notes |
| --- | --- | --- | --- |
| AC-03 | PASS | evidence/browser-observations.json; evidence/screenshot-manifest.json | Pool (3 managed proxies + 1 inactive member), speed presets with bounds, two accounts with ticket status, proxy results and four log events render; the manual dialog opens with both models selected. |
| AC-01 | PASS (display only) | evidence/browser-observations.json | Ticket state and proxy password from the fixtures are absent from the DOM at both viewports. |

## Findings

None on this page. Unmatched fixture calls from the shared layout (`/api/v1/admin/settings`, announcements, keys and similar) were answered with empty data and are listed in the observations.

## Evidence manifest

| Artifact | SHA-256 | Meaning |
| --- | --- | --- |
| evidence/browser-observations.json | 845ad850c18b0f6a97743c574d08230dba3c42d8f87f522fe961b976f2b3c13b | DOM facts per viewport |
| evidence/screenshot-manifest.json | 756a2ac2aebbba6d962f43d72127edd8ab6afa98ddd566788dfc0a8b637c15b0 | Screenshot hashes and review |
| evidence/screenshots/codex-harvest-desktop-1440x900.png | 976614dee95dd1eed64a08ff6e49e85ca43016c4e612054e22a4f2a680d1214c | codex-harvest-desktop 1440x900 |
| evidence/screenshots/codex-harvest-manual-dialog-1440x900.png | 8156835fa8420614620531535819686e5e34c5fd5543a7bcb264810447d8a32c | codex-harvest-manual-dialog 1440x900 |
| evidence/screenshots/codex-harvest-mobile-390x844.png | 66f9f085e0a706c1c26971154ff20ace9869587eeb7fe2c751eeeba6ecad26dd | codex-harvest-mobile 390x844 |

## Unverified items

Real backend responses, saving against a database, manual harvest against the real upstream, and the settings page link were not exercised in this browser run.

## Conclusion

PASS for rendering: the committed Codex tickets page presents every section and the manual harvest dialog at desktop and mobile widths without page-level overflow, and does not expose ticket state or proxy passwords from the provided data.
