### Release R1 node contract summary

- Title: 本机 Codex 历史接续工具
- User value: 用户可离线诊断接入变更造成的旧任务关联问题，审阅计划后完成可恢复迁移，并在真实客户端继续原任务。
- Independent failure: 任何备份或计划一致性错误均阻止写入；跨 JSONL 与 SQLite 的中断进入明确恢复状态，不依赖经验页是否在线。

| Task ID | Semantic key | Work kind | Write authority | Side effect class | Hard dependencies | Soft dependencies | Decision refs | Execution contract ref | Acceptance contract ref |
|---|---|---|---|---|---|---|---|---|---|
| F | source.identity-baseline | contract-foundation | evidence-only | none | none | none | none | nodes/F.json | none |
| D1 | codex-migration.documentation-scope | decision-acceptance | isolated-record | none | F | none | none | nodes/D1.json | none |
| N1 | requirement.n1 | implementation | implementation | reversible | F; D1 | none | codex-migration.documentation-scope@1 | nodes/N1.json | acceptance-fragments/N1/acceptance-contract.md |
| N2 | requirement.n2 | implementation | implementation | reversible | F; D1; N1 | none | codex-migration.documentation-scope@1 | nodes/N2.json | acceptance-fragments/N2/acceptance-contract.md |
| N3 | requirement.n3 | implementation | implementation | reversible | F; D1; N2 | none | codex-migration.documentation-scope@1 | nodes/N3.json | acceptance-fragments/N3/acceptance-contract.md |
| QR1 | acceptance.release.r1 | release-decision | shared-generated | none | N1; N2; N3 | none | codex-migration.documentation-scope@1 | nodes/QR1.json | acceptance-fragments/QR1/acceptance-contract.md |

### Release R1 status ledger

| Task ID | Stage | Versions | Execution state | Attempt | Execution owner | Acceptance mode | Acceptance authorities | Quorum | Minimum trust | Guard ID | Blocking reason | Evidence | Unlocks |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| F | R1 | none | ACCEPTED | none | 编排器 | none | none | none | none | none | none | none | D1; N1; N2; N3; N4; N5; N6 |
| D1 | R1 | none | ACCEPTED | none | 决策负责人 | none | none | none | none | none | none | none | N1; N2; N3; N4; N5; N6 |
| N1 | R1 | codex-migration.documentation-scope@1 | PLANNED | none | implementation-owner | independent | acceptance-owner:product-owner | 1 | repository-bound | none | none | acceptance-fragments/N1/acceptance-contract.md | N2; N4; QR1 |
| N2 | R1 | codex-migration.documentation-scope@1 | PLANNED | none | implementation-owner | independent | acceptance-owner:product-owner | 1 | repository-bound | none | none | acceptance-fragments/N2/acceptance-contract.md | N3; QR1 |
| N3 | R1 | codex-migration.documentation-scope@1 | PLANNED | none | implementation-owner | independent | acceptance-owner:product-owner | 1 | repository-bound | none | none | acceptance-fragments/N3/acceptance-contract.md | N5; QR1 |
| QR1 | R1 | codex-migration.documentation-scope@1 | PLANNED | none | 发布决策负责人 | external | release-owner:product-owner | 1 | repository-bound | none | none | acceptance-fragments/QR1/acceptance-contract.md | none |

### Release R1 deliverables

| Deliverable ID | Owning node |
|---|---|
| none | none |
