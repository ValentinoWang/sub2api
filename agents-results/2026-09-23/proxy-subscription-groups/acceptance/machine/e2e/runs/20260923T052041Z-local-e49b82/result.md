# Acceptance Run: 20260923T052041Z-local-e49b82

- Run ID: 20260923T052041Z-local-e49b82
- Task ID: proxy-subscription-groups
- Lane: machine/e2e
- Status: PASS
- Acceptance contract: agents-results/2026-09-23/proxy-subscription-groups/acceptance-contract.md
- Contract version: 1
- Contract SHA-256: d2e49224ee44032e6486c261e4e06f1b806ffe813ea1b78b28aa999e72f3ae3f
- Source identity: ef67476430a6e89476f8cd81b4706ce4385c847b
- Runtime identity: vite-dev-127.0.0.1:4174
- Executor or reviewer: Claude Playwright Chromium
- Started at: 2026-09-23T05:20:41.389366Z
- Completed at: 2026-09-23T05:21:15Z
- Evidence directory: evidence/

## Scope

The proxy management “订阅与分组” dialog and the Codex tickets pool add-by-group section, rendered from the committed source by the Vite dev server on the fixed port 4174 in Playwright Chromium (headless shell 1228) at 1440x900 and 390x844. Every `/api/` request was answered by fixtures from `agents-results/2026-09-23/proxy-subscription-groups/e2e_capture.py` (a node-list subscription with info entries and a Mihomo YAML subscription with provider groups); no backend, subscription URL or credential was involved. Excludes fetching, parsing, refresh and persistence, covered by backend tests.

## Procedure

1. Ran `e2e_capture.py frontend <run>/evidence ef6747643…` with `PLAYWRIGHT_CHROMIUM` set; Vite started on 4174 and stopped afterwards.
2. Opened `/admin/proxies`, clicked “订阅与分组”, expanded “🎫 打票出口”, recorded DOM facts and captured the dialog.
3. Opened `/admin/codex-harvest`, recorded group chips and normalized names, captured the pool card.
4. Reviewed all four screenshots.

## Requirement disposition

| Requirement | Result | Evidence | Notes |
| --- | --- | --- | --- |
| AC-03 | PASS | evidence/browser-observations.json; evidence/screenshot-manifest.json | Dialog shows usage/expiry, info entries, refresh status, interval, provider/purpose/multiplier/region groups and expanded members; the no-URL hint appears for the YAML subscription; the pool shows 9 group chips (multiplier groups excluded) and normalized names. |

## Findings

None. On mobile the pool table scrolls inside its card; the page itself does not overflow.

## Evidence manifest

| Artifact | SHA-256 | Meaning |
| --- | --- | --- |
| evidence/browser-observations.json | f363ad834f75232e62c3b5ecbd43df6db38374a80ac19252cb4342b4102048a9 | DOM facts per page and viewport |
| evidence/screenshot-manifest.json | ce3267e673a042756a129a9578bfd466d406541900db3645cd5df3e8856e31ce | Screenshot hashes and review |
| evidence/screenshots/proxy-subscriptions-desktop-1440x900.png | f7cbc2a9a5072bc3befaf13573b86da5ad0b3d603bf341cbf0265459f48540d0 | proxy-subscriptions-desktop |
| evidence/screenshots/harvest-pool-groups-desktop-1440x900.png | 68a684cd71608de77050a27e36121b454950ee24e6296560138cff374598b922 | harvest-pool-groups-desktop |
| evidence/screenshots/proxy-subscriptions-mobile-390x844.png | ce133f7d1832b96479db5e35b8f7fbd1d2ab606fcdd15c838eb636c2e4afcb88 | proxy-subscriptions-mobile |
| evidence/screenshots/harvest-pool-groups-mobile-390x844.png | 8173bf68f37ef7239960dfd2f39ec29bb5c33815ae0875fff9cdfd68525afce6 | harvest-pool-groups-mobile |

## Unverified items

Real subscription fetches, refresh against a provider, Mihomo reload and saving the pool against a database were not exercised in this browser run.

## Conclusion

PASS for rendering: the committed subscription groups dialog and add-by-group pool present the intended information at desktop and mobile widths without exposing subscription URLs.
