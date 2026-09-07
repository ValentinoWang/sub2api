### Release R2 node contract summary

- Title: 站内经验、P1 接入与公开下载闭环
- User value: 用户从本站接入教程或经验分享进入同一案例，下载对应系统的工具或复制独立 Codex 指令，修复后返回接入流程。
- Independent failure: 站点发布失败可撤回该候选下载和页面；已验证的本机工具仍有效，不触碰账号、支付、凭据或服务端业务数据库。

| Task ID | Semantic key | Work kind | Write authority | Side effect class | Hard dependencies | Soft dependencies | Decision refs | Execution contract ref | Acceptance contract ref |
|---|---|---|---|---|---|---|---|---|---|
| N4 | requirement.n4 | implementation | implementation | reversible | F; D1; N1 | none | codex-migration.documentation-scope@1 | nodes/N4.json | acceptance-fragments/N4/acceptance-contract.md |
| N5 | requirement.n5 | implementation | implementation | reversible | F; D1; N3; N4 | none | codex-migration.documentation-scope@1 | nodes/N5.json | acceptance-fragments/N5/acceptance-contract.md |
| N6 | requirement.n6 | implementation | implementation | reversible | F; D1; N5 | none | codex-migration.documentation-scope@1 | nodes/N6.json | acceptance-fragments/N6/acceptance-contract.md |
| QR2 | acceptance.release.r2 | release-decision | shared-generated | none | N4; N5; N6 | none | codex-migration.documentation-scope@1 | nodes/QR2.json | acceptance-fragments/QR2/acceptance-contract.md |

### Release R2 status ledger

| Task ID | Stage | Versions | Execution state | Attempt | Execution owner | Acceptance mode | Acceptance authorities | Quorum | Minimum trust | Guard ID | Blocking reason | Evidence | Unlocks |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| N4 | R2 | codex-migration.documentation-scope@1 | PLANNED | none | implementation-owner | independent | acceptance-owner:product-owner | 1 | repository-bound | none | none | acceptance-fragments/N4/acceptance-contract.md | N5; QR2 |
| N5 | R2 | codex-migration.documentation-scope@1 | PLANNED | none | implementation-owner | independent | acceptance-owner:product-owner | 1 | repository-bound | none | none | acceptance-fragments/N5/acceptance-contract.md | N6; QR2 |
| N6 | R2 | codex-migration.documentation-scope@1 | PLANNED | none | implementation-owner | independent | acceptance-owner:product-owner | 1 | repository-bound | none | none | acceptance-fragments/N6/acceptance-contract.md | QR2 |
| QR2 | R2 | codex-migration.documentation-scope@1 | PLANNED | none | 发布决策负责人 | external | release-owner:product-owner | 1 | repository-bound | none | none | acceptance-fragments/QR2/acceptance-contract.md | none |

### Release R2 deliverables

| Deliverable ID | Owning node |
|---|---|
| none | none |
