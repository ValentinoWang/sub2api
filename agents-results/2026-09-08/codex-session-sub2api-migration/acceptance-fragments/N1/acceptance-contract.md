# Acceptance Contract: N1

- Task ID: N1
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
- Request source: item N1
- SSOT node: N1
- SSOT path: .ssot/nodes/N1.json
- Readiness mode: FORMAL
- Decision refs: codex-migration.documentation-scope@1
- Assumption IDs: none
- Invalidation keys: task.n1
- AC budget: 3
- Baseline identity: ssot-input.json#items[N1]
- Product Context refs: none
- Role Context refs: none
- Resolved Surface Contract refs: none
- Screen Contract ref: none
- Visual Contract refs: none
- UI Change declaration: none
- Human acceptance workspace: none

## User and scenario

The user reaching the surface declared for N1 drives item N1 (unspecified dimension) through the interface declared for N1. Concrete seed references: tools/codex_migration_inspect.py, tools/test_codex_migration_inspect.py.

## Problem

Item N1 exists because the interface declared for N1 does not yet satisfy the acceptance seeds registered for it, leaving the surface declared for N1 incomplete. Concrete seed references: tools/codex_migration_inspect.py, tools/test_codex_migration_inspect.py.

## Expected outcome

After item N1 lands, the interface declared for N1 satisfies every acceptance seed below and the surface declared for N1 reflects that behavior. Concrete seed references: tools/codex_migration_inspect.py, tools/test_codex_migration_inspect.py.

## Non-goals

Item N1 covers only the interface declared for N1 and the surface declared for N1 as described by its acceptance seeds; behavior outside those seeds is out of scope. Concrete seed references: tools/codex_migration_inspect.py, tools/test_codex_migration_inspect.py.

## Normal path

```gherkin
Given a user reaches the surface declared for N1 for item N1
When the flow defined by the interface declared for N1 executes
Then every acceptance seed for item N1 holds  Concrete seed references: tools/codex_migration_inspect.py, tools/test_codex_migration_inspect.py.
```

## Exception paths

If the interface declared for N1 fails for item N1, the surface declared for N1 must surface the failure exactly as the acceptance seeds below specify; no exception handling beyond those seeds is in scope. Concrete seed references: tools/codex_migration_inspect.py, tools/test_codex_migration_inspect.py.

## Invariants

For item N1, the interface declared for N1 must continue to satisfy every acceptance seed below on every call; the surface declared for N1 must never show a state the seeds forbid. Concrete seed references: tools/codex_migration_inspect.py, tools/test_codex_migration_inspect.py.

## Data impact

Item N1 constrains any create, update, or delete reachable through the interface declared for N1; only the acceptance seeds below define what data changes are permitted for the surface declared for N1. Node-specific data assertions: tools/test_codex_migration_inspect.py 覆盖官方 openai、第三方自定义名称、缺失 provider、仅认证切换无需迁移及目标已经匹配；使用 TOML 和 JSON 解析器区分名称大小写，未声明目标或多入口冲突时只输出诊断。 | tools/codex_migration_inspect.py 扫描 CODEX_HOME、sessions 与 archived_sessions，并识别索引真实结构；默认诊断不会改动任何被检文件、SQLite 及其日志或共享内存，计划显示任务数量、范围、源到目标映射及阻塞项。 | tools/test_codex_migration_inspect.py 对自定义数据目录、旧任务近期续写、重复片段、索引与元数据冲突、损坏 JSONL 和未知数据库结构分别断言；禁止按文件修改时间推导全部迁移对象或假定文件数等于任务数。 Concrete seed references: tools/codex_migration_inspect.py, tools/test_codex_migration_inspect.py.

## Permissions

Item N1 is owned by product-owner; access to the interface declared for N1 and the surface declared for N1 follows the acceptance seeds below and no wider grant. Concrete seed references: tools/codex_migration_inspect.py, tools/test_codex_migration_inspect.py.

## Performance and reliability

No performance or reliability threshold beyond existing behavior is declared for item N1; correctness of the interface declared for N1 is governed entirely by the acceptance seeds below. Concrete seed references: tools/codex_migration_inspect.py, tools/test_codex_migration_inspect.py.

## Acceptance criteria

| ID | Class | Source requirement refs | Lane | Requirement | Verification layer | Mode | Blocking |
| --- | --- | --- | --- | --- | --- | --- | --- |
| AC-01 | behavior | none | machine/unit | tools/test_codex_migration_inspect.py 覆盖官方 openai、第三方自定义名称、缺失 provider、仅认证切换无需迁移及目标已经匹配；使用 TOML 和 JSON 解析器区分名称大小写，未声明目标或多入口冲突时只输出诊断。 | Unit | Automatic | Yes |
| AC-02 | behavior | none | machine/integration-contract | tools/codex_migration_inspect.py 扫描 CODEX_HOME、sessions 与 archived_sessions，并识别索引真实结构；默认诊断不会改动任何被检文件、SQLite 及其日志或共享内存，计划显示任务数量、范围、源到目标映射及阻塞项。 | Integration | Automatic | Yes |
| AC-03 | behavior | none | machine/unit | tools/test_codex_migration_inspect.py 对自定义数据目录、旧任务近期续写、重复片段、索引与元数据冲突、损坏 JSONL 和未知数据库结构分别断言；禁止按文件修改时间推导全部迁移对象或假定文件数等于任务数。 | Unit | Automatic | Yes |

## Human acceptance

Item N1 is fully determined by its acceptance seeds; outcomes for the interface declared for N1 on the surface declared for N1 are machine-verifiable, so no human judgment step is declared.

## Protected acceptance tests

| Path | SHA-256 | Covers |
| --- | --- | --- |
| none | none | Behavior specification only for item N1; executable baseline not yet locked. Concrete seed references: tools/codex_migration_inspect.py, tools/test_codex_migration_inspect.py |

## Requirements-test traceability

| Requirement | Verification | Evidence target | Mode | Blocking |
| --- | --- | --- | --- | --- |
| AC-01 | Unit | tools/test_codex_migration_inspect.py | Automatic | Yes |
| AC-02 | Integration | tools/codex_migration_inspect.py | Automatic | Yes |
| AC-03 | Unit | tools/test_codex_migration_inspect.py | Automatic | Yes |

## Exploratory testing

Probe the surface declared for N1 for item N1 under retry, interruption, and boundary-value inputs against the interface declared for N1, beyond the deterministic acceptance seeds below.

## Production monitoring and rollback

Rollback for item N1 reverts the change to the interface declared for N1; no bespoke production metric is declared beyond the acceptance seeds below.

## Risks and open decisions

<!-- shared_acceptance_policy: SAP-MIGRATION-EVIDENCE@1 -->
Node-specific increment for item N1: review risks specific to the surface declared for N1 and record any open decision.
