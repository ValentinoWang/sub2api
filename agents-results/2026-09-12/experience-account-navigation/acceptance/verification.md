# Local navigation verification

- Scope: authenticated experience index, articles and integration guides use AppLayout; anonymous and unrelated public pages retain PublicPageLayout. Article subroutes retain the experience sidebar selection.
- User requested enabling the existing payment switch. Local PostgreSQL setting payment_enabled was absent; upserted to true. Public GET through 4174 read back true. No production settings changed.
- Existing /purchase navigation restored; temporary /recharge implementation removed. Browser confirmed recharge/subscription and order sidebar entries, and /purchase shows the existing Liandong redemption card linking to /redeem. Direct online recharge still displays unavailable. No purchase or redemption submitted.
- Browser: authenticated desktop experience index retains account sidebar/header/background. At 390 x 844, cards stack and mobile menu exposes recharge/subscription. Viewport override reset after inspection.
- Targeted Vitest: 5 files, 59 tests passed (layout, sidebar, profile, router guards and feature access). vue-tsc, targeted ESLint and git diff --check passed.
- Figma: https://www.figma.com/design/bG5roZJVC4F4IQwaL8oeOk?node-id=15-14 ; editable desktop/mobile navigation reference; font and simplified controls prevent pixel-equivalence claims.
- Local HMR and browser evidence only; no production deployment or human acceptance claimed.
