# Acceptance Contract: N2

- Task ID: N2
- Contract kind: implementation
- Contract profile: acceptance-contract-kind-profiles@1
- Verification layer: machine
- Acceptance mode: Automatic
- Evidence target: test result
- Contract version: 1
- Contract status: DRAFT
- Test baseline: PLANNED
- Acceptance owner: product-owner
- Execution actor: orchestrator
- Approval evidence: TBD
- Request source: item N2
- SSOT node: N2
- SSOT path: .ssot/nodes/N2.json
- Readiness mode: FORMAL
- Decision refs: codex-migration.documentation-scope@1
- Assumption IDs: none
- Invalidation keys: task.n2
- AC budget: 4
- Baseline identity: ssot-input.json#items[N2]
- Product Context refs: none
- Role Context refs: none
- Resolved Surface Contract refs: none
- Screen Contract ref: none
- Visual Contract refs: none
- UI Change declaration: none
- Human acceptance workspace: none

## User and scenario

The user reaching the surface declared for N2 drives item N2 (unspecified dimension) through the interface declared for N2. Concrete seed references: tools/codex_migration_transaction.py, tools/test_codex_migration_transaction.py.

## Problem

Item N2 exists because the interface declared for N2 does not yet satisfy the acceptance seeds registered for it, leaving the surface declared for N2 incomplete. Concrete seed references: tools/codex_migration_transaction.py, tools/test_codex_migration_transaction.py.

## Expected outcome

After item N2 lands, the interface declared for N2 satisfies every acceptance seed below and the surface declared for N2 reflects that behavior. Concrete seed references: tools/codex_migration_transaction.py, tools/test_codex_migration_transaction.py.

## Non-goals

Item N2 covers only the interface declared for N2 and the surface declared for N2 as described by its acceptance seeds; behavior outside those seeds is out of scope. Concrete seed references: tools/codex_migration_transaction.py, tools/test_codex_migration_transaction.py.

## Normal path

```gherkin
Given a user reaches the surface declared for N2 for item N2
When the flow defined by the interface declared for N2 executes
Then every acceptance seed for item N2 holds  Concrete seed references: tools/codex_migration_transaction.py, tools/test_codex_migration_transaction.py.
```

## Exception paths

If the interface declared for N2 fails for item N2, the surface declared for N2 must surface the failure exactly as the acceptance seeds below specify; no exception handling beyond those seeds is in scope. Concrete seed references: tools/codex_migration_transaction.py, tools/test_codex_migration_transaction.py.

## Invariants

For item N2, the interface declared for N2 must continue to satisfy every acceptance seed below on every call; the surface declared for N2 must never show a state the seeds forbid. Concrete seed references: tools/codex_migration_transaction.py, tools/test_codex_migration_transaction.py.

## Data impact

Item N2 constrains any create, update, or delete reachable through the interface declared for N2; only the acceptance seeds below define what data changes are permitted for the surface declared for N2. Node-specific data assertions: tools/test_codex_migration_transaction.py 注入备份创建失败、读回摘要不符、磁盘不足、权限拒绝及客户端写入；任一故障均禁止进入首次数据修改，不能重建事后副本并称为原始备份。 | tools/codex_migration_transaction.py 只修改已选任务中已支持的 session_meta.payload.model_provider 和对应索引行；未修改消息与附件逐字节守恒，配置和认证文件不写入，无关任务行及其更新时间保持原值。 | tools/test_codex_migration_transaction.py 在每次文件替换和数据库提交前后强制中断再恢复；只有所有目标完成并验证才报告成功，中间状态要求恢复；重复执行不重复迁移，输入摘要变化阻止沿用旧计划。 | tools/codex_migration_transaction.py 回滚前验证当前状态仍匹配迁移后记录；有新增消息或索引变化时拒绝覆盖并输出可恢复的冲突报告，其他任务新增数据不得被整库恢复抹除。 Concrete seed references: tools/codex_migration_transaction.py, tools/test_codex_migration_transaction.py.

## Permissions

Item N2 is owned by product-owner; access to the interface declared for N2 and the surface declared for N2 follows the acceptance seeds below and no wider grant. Concrete seed references: tools/codex_migration_transaction.py, tools/test_codex_migration_transaction.py.

## Performance and reliability

No performance or reliability threshold beyond existing behavior is declared for item N2; correctness of the interface declared for N2 is governed entirely by the acceptance seeds below. Concrete seed references: tools/codex_migration_transaction.py, tools/test_codex_migration_transaction.py.

## Acceptance criteria

| ID | Class | Source requirement refs | Lane | Requirement | Verification layer | Mode | Blocking |
| --- | --- | --- | --- | --- | --- | --- | --- |
| AC-01 | behavior | none | machine/unit | tools/test_codex_migration_transaction.py 注入备份创建失败、读回摘要不符、磁盘不足、权限拒绝及客户端写入；任一故障均禁止进入首次数据修改，不能重建事后副本并称为原始备份。 | Unit | Automatic | Yes |
| AC-02 | behavior | none | machine/integration-contract | tools/codex_migration_transaction.py 只修改已选任务中已支持的 session_meta.payload.model_provider 和对应索引行；未修改消息与附件逐字节守恒，配置和认证文件不写入，无关任务行及其更新时间保持原值。 | Integration | Automatic | Yes |
| AC-03 | behavior | none | machine/non-functional | tools/test_codex_migration_transaction.py 在每次文件替换和数据库提交前后强制中断再恢复；只有所有目标完成并验证才报告成功，中间状态要求恢复；重复执行不重复迁移，输入摘要变化阻止沿用旧计划。 | Non-functional | Automatic | Yes |
| AC-04 | behavior | none | machine/unit | tools/codex_migration_transaction.py 回滚前验证当前状态仍匹配迁移后记录；有新增消息或索引变化时拒绝覆盖并输出可恢复的冲突报告，其他任务新增数据不得被整库恢复抹除。 | Unit | Automatic | Yes |

## Human acceptance

Item N2 is fully determined by its acceptance seeds; outcomes for the interface declared for N2 on the surface declared for N2 are machine-verifiable, so no human judgment step is declared.

## Protected acceptance tests

| Path | SHA-256 | Covers |
| --- | --- | --- |
| none | none | Behavior specification only for item N2; executable baseline not yet locked. Concrete seed references: tools/codex_migration_transaction.py, tools/test_codex_migration_transaction.py |

## Requirements-test traceability

| Requirement | Verification | Evidence target | Mode | Blocking |
| --- | --- | --- | --- | --- |
| AC-01 | Unit | tools/test_codex_migration_transaction.py | Automatic | Yes |
| AC-02 | Integration | tools/codex_migration_transaction.py | Automatic | Yes |
| AC-03 | Non-functional | tools/test_codex_migration_transaction.py | Automatic | Yes |
| AC-04 | Unit | tools/codex_migration_transaction.py | Automatic | Yes |

## Exploratory testing

Probe the surface declared for N2 for item N2 under retry, interruption, and boundary-value inputs against the interface declared for N2, beyond the deterministic acceptance seeds below.

## Production monitoring and rollback

Rollback for item N2 reverts the change to the interface declared for N2; no bespoke production metric is declared beyond the acceptance seeds below.

## Risks and open decisions

<!-- shared_acceptance_policy: SAP-MIGRATION-EVIDENCE@1 -->
Node-specific increment for item N2: review risks specific to the surface declared for N2 and record any open decision.
