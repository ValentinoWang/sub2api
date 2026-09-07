# Acceptance Contract: N4

- Task ID: N4
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
- Request source: item N4
- SSOT node: N4
- SSOT path: .ssot/nodes/N4.json
- Readiness mode: FORMAL
- Decision refs: codex-migration.documentation-scope@1
- Assumption IDs: none
- Invalidation keys: task.n4
- AC budget: 3
- Baseline identity: ssot-input.json#items[N4]
- Product Context refs: none
- Role Context refs: none
- Resolved Surface Contract refs: none
- Screen Contract ref: none
- Visual Contract refs: none
- UI Change declaration: none
- Human acceptance workspace: none

## User and scenario

The user reaching the surface declared for N4 drives item N4 (unspecified dimension) through the interface declared for N4. Concrete seed references: docs/error-experiences/2026-09-08-codex-session-migration.md, frontend/src/content/experiences.ts, frontend/src/views/public/CodexSessionMigrationView.vue.

## Problem

Item N4 exists because the interface declared for N4 does not yet satisfy the acceptance seeds registered for it, leaving the surface declared for N4 incomplete. Concrete seed references: docs/error-experiences/2026-09-08-codex-session-migration.md, frontend/src/content/experiences.ts, frontend/src/views/public/CodexSessionMigrationView.vue.

## Expected outcome

After item N4 lands, the interface declared for N4 satisfies every acceptance seed below and the surface declared for N4 reflects that behavior. Concrete seed references: docs/error-experiences/2026-09-08-codex-session-migration.md, frontend/src/content/experiences.ts, frontend/src/views/public/CodexSessionMigrationView.vue.

## Non-goals

Item N4 covers only the interface declared for N4 and the surface declared for N4 as described by its acceptance seeds; behavior outside those seeds is out of scope. Concrete seed references: docs/error-experiences/2026-09-08-codex-session-migration.md, frontend/src/content/experiences.ts, frontend/src/views/public/CodexSessionMigrationView.vue.

## Normal path

```gherkin
Given a user reaches the surface declared for N4 for item N4
When the flow defined by the interface declared for N4 executes
Then every acceptance seed for item N4 holds  Concrete seed references: docs/error-experiences/2026-09-08-codex-session-migration.md, frontend/src/content/experiences.ts, frontend/src/views/public/CodexSessionMigrationView.vue.
```

## Exception paths

If the interface declared for N4 fails for item N4, the surface declared for N4 must surface the failure exactly as the acceptance seeds below specify; no exception handling beyond those seeds is in scope. Concrete seed references: docs/error-experiences/2026-09-08-codex-session-migration.md, frontend/src/content/experiences.ts, frontend/src/views/public/CodexSessionMigrationView.vue.

## Invariants

For item N4, the interface declared for N4 must continue to satisfy every acceptance seed below on every call; the surface declared for N4 must never show a state the seeds forbid. Concrete seed references: docs/error-experiences/2026-09-08-codex-session-migration.md, frontend/src/content/experiences.ts, frontend/src/views/public/CodexSessionMigrationView.vue.

## Data impact

Item N4 constrains any create, update, or delete reachable through the interface declared for N4; only the acceptance seeds below define what data changes are permitted for the surface declared for N4. Node-specific data assertions: frontend/src/views/public/CodexSessionMigrationView.vue 按情况说明、Codex 帮你处理、给人看的顺序展示同一个案例；包含系统适用范围、工具下载、指令复制及复制失败反馈，沿用 rest2build 已批准品牌收尾。 | frontend/src/content/experiences.ts 中迁移案例的指令可脱离文章执行，要求先诊断再审阅目标并验证备份；不得写死用户路径、旧 provider 名、认证模式或本机地址，明确无法在当前应用关闭后继续的步骤由离线脚本完成。 | docs/error-experiences/2026-09-08-codex-session-migration.md、静态预览和网页的指令与支持边界一致；公开内容不包含所附截图的真实密钥或私人会话，只说明本地历史接续，不能承诺网页记忆迁移或无损迁移正在进行的工具调用。 Concrete seed references: docs/error-experiences/2026-09-08-codex-session-migration.md, frontend/src/content/experiences.ts, frontend/src/views/public/CodexSessionMigrationView.vue.

## Permissions

Item N4 is owned by product-owner; access to the interface declared for N4 and the surface declared for N4 follows the acceptance seeds below and no wider grant. Concrete seed references: docs/error-experiences/2026-09-08-codex-session-migration.md, frontend/src/content/experiences.ts, frontend/src/views/public/CodexSessionMigrationView.vue.

## Performance and reliability

No performance or reliability threshold beyond existing behavior is declared for item N4; correctness of the interface declared for N4 is governed entirely by the acceptance seeds below. Concrete seed references: docs/error-experiences/2026-09-08-codex-session-migration.md, frontend/src/content/experiences.ts, frontend/src/views/public/CodexSessionMigrationView.vue.

## Acceptance criteria

| ID | Class | Source requirement refs | Lane | Requirement | Verification layer | Mode | Blocking |
| --- | --- | --- | --- | --- | --- | --- | --- |
| AC-01 | behavior | none | machine/unit | frontend/src/views/public/CodexSessionMigrationView.vue 按情况说明、Codex 帮你处理、给人看的顺序展示同一个案例；包含系统适用范围、工具下载、指令复制及复制失败反馈，沿用 rest2build 已批准品牌收尾。 | Unit | Automatic | Yes |
| AC-02 | behavior | none | machine/e2e | frontend/src/content/experiences.ts 中迁移案例的指令可脱离文章执行，要求先诊断再审阅目标并验证备份；不得写死用户路径、旧 provider 名、认证模式或本机地址，明确无法在当前应用关闭后继续的步骤由离线脚本完成。 | E2E | Automatic | Yes |
| AC-03 | behavior | none | machine/unit | docs/error-experiences/2026-09-08-codex-session-migration.md、静态预览和网页的指令与支持边界一致；公开内容不包含所附截图的真实密钥或私人会话，只说明本地历史接续，不能承诺网页记忆迁移或无损迁移正在进行的工具调用。 | Unit | Automatic | Yes |

## Human acceptance

Item N4 is fully determined by its acceptance seeds; outcomes for the interface declared for N4 on the surface declared for N4 are machine-verifiable, so no human judgment step is declared.

## Protected acceptance tests

| Path | SHA-256 | Covers |
| --- | --- | --- |
| none | none | Behavior specification only for item N4; executable baseline not yet locked. Concrete seed references: docs/error-experiences/2026-09-08-codex-session-migration.md, frontend/src/content/experiences.ts, frontend/src/views/public/CodexSessionMigrationView.vue |

## Requirements-test traceability

| Requirement | Verification | Evidence target | Mode | Blocking |
| --- | --- | --- | --- | --- |
| AC-01 | Unit | frontend/src/views/public/CodexSessionMigrationView.vue | Automatic | Yes |
| AC-02 | E2E | frontend/src/content/experiences.ts | Automatic | Yes |
| AC-03 | Unit | docs/error-experiences/2026-09-08-codex-session-migration.md | Automatic | Yes |

## Exploratory testing

Probe the surface declared for N4 for item N4 under retry, interruption, and boundary-value inputs against the interface declared for N4, beyond the deterministic acceptance seeds below.

## Production monitoring and rollback

Rollback for item N4 reverts the change to the interface declared for N4; no bespoke production metric is declared beyond the acceptance seeds below.

## Risks and open decisions

<!-- shared_acceptance_policy: SAP-MIGRATION-EVIDENCE@1 -->
Node-specific increment for item N4: review risks specific to the surface declared for N4 and record any open decision.
