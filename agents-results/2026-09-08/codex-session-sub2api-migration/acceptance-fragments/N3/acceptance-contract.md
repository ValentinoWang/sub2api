# Acceptance Contract: N3

- Task ID: N3
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
- Request source: item N3
- SSOT node: N3
- SSOT path: .ssot/nodes/N3.json
- Readiness mode: FORMAL
- Decision refs: codex-migration.documentation-scope@1
- Assumption IDs: none
- Invalidation keys: task.n3
- AC budget: 3
- Baseline identity: ssot-input.json#items[N3]
- Product Context refs: none
- Role Context refs: none
- Resolved Surface Contract refs: none
- Screen Contract ref: none
- Visual Contract refs: none
- UI Change declaration: none
- Human acceptance workspace: none

## User and scenario

The user reaching the surface declared for N3 drives item N3 (unspecified dimension) through the interface declared for N3. Concrete seed references: .github/workflows/codex-session-migration.yml, tools/codex_session_migrate.py, tools/test_codex_session_migrate.py.

## Problem

Item N3 exists because the interface declared for N3 does not yet satisfy the acceptance seeds registered for it, leaving the surface declared for N3 incomplete. Concrete seed references: .github/workflows/codex-session-migration.yml, tools/codex_session_migrate.py, tools/test_codex_session_migrate.py.

## Expected outcome

After item N3 lands, the interface declared for N3 satisfies every acceptance seed below and the surface declared for N3 reflects that behavior. Concrete seed references: .github/workflows/codex-session-migration.yml, tools/codex_session_migrate.py, tools/test_codex_session_migrate.py.

## Non-goals

Item N3 covers only the interface declared for N3 and the surface declared for N3 as described by its acceptance seeds; behavior outside those seeds is out of scope. Concrete seed references: .github/workflows/codex-session-migration.yml, tools/codex_session_migrate.py, tools/test_codex_session_migrate.py.

## Normal path

```gherkin
Given a user reaches the surface declared for N3 for item N3
When the flow defined by the interface declared for N3 executes
Then every acceptance seed for item N3 holds  Concrete seed references: .github/workflows/codex-session-migration.yml, tools/codex_session_migrate.py, tools/test_codex_session_migrate.py.
```

## Exception paths

If the interface declared for N3 fails for item N3, the surface declared for N3 must surface the failure exactly as the acceptance seeds below specify; no exception handling beyond those seeds is in scope. Concrete seed references: .github/workflows/codex-session-migration.yml, tools/codex_session_migrate.py, tools/test_codex_session_migrate.py.

## Invariants

For item N3, the interface declared for N3 must continue to satisfy every acceptance seed below on every call; the surface declared for N3 must never show a state the seeds forbid. Concrete seed references: .github/workflows/codex-session-migration.yml, tools/codex_session_migrate.py, tools/test_codex_session_migrate.py.

## Data impact

Item N3 constrains any create, update, or delete reachable through the interface declared for N3; only the acceptance seeds below define what data changes are permitted for the surface declared for N3. Node-specific data assertions: tools/codex_session_migrate.py 提供 inspect、plan、apply、verify、recover、rollback，默认只读；在 macOS、Linux、Windows 使用相同核心，缺 Python 3.11 或数据格式不支持时明确退出，不安装运行时或放宽系统策略。 | tools/test_codex_session_migrate.py 断言脱敏输出不含密钥、认证内容和私人消息，远程下载内容不直接通过管道执行；500 个任务共 1 GiB 的基准扫描峰值额外内存不超过 256 MiB，附件不入模型、不上传。 | .github/workflows/codex-session-migration.yml 在三个系统以隔离目录执行零跳过回归；额外真实客户端记录必须证明重启后能列出、打开、继续同一个旧任务，不以数据库读取或新建任务成功替代。 Concrete seed references: .github/workflows/codex-session-migration.yml, tools/codex_session_migrate.py, tools/test_codex_session_migrate.py.

## Permissions

Item N3 is owned by product-owner; access to the interface declared for N3 and the surface declared for N3 follows the acceptance seeds below and no wider grant. Concrete seed references: .github/workflows/codex-session-migration.yml, tools/codex_session_migrate.py, tools/test_codex_session_migrate.py.

## Performance and reliability

No performance or reliability threshold beyond existing behavior is declared for item N3; correctness of the interface declared for N3 is governed entirely by the acceptance seeds below. Concrete seed references: .github/workflows/codex-session-migration.yml, tools/codex_session_migrate.py, tools/test_codex_session_migrate.py.

## Acceptance criteria

| ID | Class | Source requirement refs | Lane | Requirement | Verification layer | Mode | Blocking |
| --- | --- | --- | --- | --- | --- | --- | --- |
| AC-01 | behavior | none | machine/unit | tools/codex_session_migrate.py 提供 inspect、plan、apply、verify、recover、rollback，默认只读；在 macOS、Linux、Windows 使用相同核心，缺 Python 3.11 或数据格式不支持时明确退出，不安装运行时或放宽系统策略。 | Unit | Automatic | Yes |
| AC-02 | behavior | none | machine/local-runtime | tools/test_codex_session_migrate.py 断言脱敏输出不含密钥、认证内容和私人消息，远程下载内容不直接通过管道执行；500 个任务共 1 GiB 的基准扫描峰值额外内存不超过 256 MiB，附件不入模型、不上传。 | Local runtime | Automatic | Yes |
| AC-03 | behavior | none | machine/non-functional | .github/workflows/codex-session-migration.yml 在三个系统以隔离目录执行零跳过回归；额外真实客户端记录必须证明重启后能列出、打开、继续同一个旧任务，不以数据库读取或新建任务成功替代。 | Non-functional | Automatic | Yes |

## Human acceptance

Item N3 is fully determined by its acceptance seeds; outcomes for the interface declared for N3 on the surface declared for N3 are machine-verifiable, so no human judgment step is declared.

## Protected acceptance tests

| Path | SHA-256 | Covers |
| --- | --- | --- |
| none | none | Behavior specification only for item N3; executable baseline not yet locked. Concrete seed references: .github/workflows/codex-session-migration.yml, tools/codex_session_migrate.py, tools/test_codex_session_migrate.py |

## Requirements-test traceability

| Requirement | Verification | Evidence target | Mode | Blocking |
| --- | --- | --- | --- | --- |
| AC-01 | Unit | tools/codex_session_migrate.py | Automatic | Yes |
| AC-02 | Local runtime | tools/test_codex_session_migrate.py | Automatic | Yes |
| AC-03 | Non-functional | .github/workflows/codex-session-migration.yml | Automatic | Yes |

## Exploratory testing

Probe the surface declared for N3 for item N3 under retry, interruption, and boundary-value inputs against the interface declared for N3, beyond the deterministic acceptance seeds below.

## Production monitoring and rollback

Rollback for item N3 reverts the change to the interface declared for N3; no bespoke production metric is declared beyond the acceptance seeds below.

## Risks and open decisions

<!-- shared_acceptance_policy: SAP-MIGRATION-EVIDENCE@1 -->
Node-specific increment for item N3: review risks specific to the surface declared for N3 and record any open decision.
