# Local Implementation Progress

Status: IMPLEMENTED for the local HMR candidate. This record is implementation evidence, not a production release or a replacement for the SSOT acceptance contracts.

## Delivered

- N1: the experience catalogue now has stable categories for connection and configuration, conversation continuity, models and usage, and troubleshooting. P1's Codex migration handoff carries only `from=p1` and the topic identifier, never a credential or endpoint value.
- N2: `/experiences` uses a responsive two-column card grid that becomes one column at the mobile breakpoint. Category selection is restored from and written to the route query.
- N3: Chinese homepage positioning is now `AI 使用经验分享与开发接入支持`; the two Chinese access modes use `本站托管接入` and `自有密钥接入支持`.
- N4: public pages use a full-width header with scrollable navigation and a route-aware wide content rail for `/experiences`. The shared public substrate has distinct light and dark surfaces without aurora decorations.
- N5: targeted Vitest coverage and frontend type checking passed. The running Vite HMR preview rendered `/experiences` and `/home` successfully.

## Verification

```text
pnpm --dir frontend exec vitest run src/views/public/__tests__/ExperiencesView.spec.ts src/components/keys/__tests__/UseKeyModal.spec.ts src/views/__tests__/HomeView.compact.spec.ts
36 tests passed

pnpm --dir frontend typecheck
passed
```

The preview remained local at `http://127.0.0.1:4174`; no server deployment or server-side write was performed.
