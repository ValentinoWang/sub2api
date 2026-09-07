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
- Decision refs: rest2build.experience-center.local-hmr-scope@1
- Assumption IDs: none
- Invalidation keys: task.n3
- AC budget: 2
- Baseline identity: ssot-input.json#items[N3]
- Product Context refs: none
- Role Context refs: none
- Resolved Surface Contract refs: none
- Screen Contract ref: none
- Visual Contract refs: none
- UI Change declaration: none
- Human acceptance workspace: none

## User and scenario

The user reaching the surface declared for N3 drives item N3 (unspecified dimension) through the interface declared for N3. Concrete seed references: frontend/src/i18n/locales/zh/landing.ts.

## Problem

Item N3 exists because the interface declared for N3 does not yet satisfy the acceptance seeds registered for it, leaving the surface declared for N3 incomplete. Concrete seed references: frontend/src/i18n/locales/zh/landing.ts.

## Expected outcome

After item N3 lands, the interface declared for N3 satisfies every acceptance seed below and the surface declared for N3 reflects that behavior. Concrete seed references: frontend/src/i18n/locales/zh/landing.ts.

## Non-goals

Item N3 covers only the interface declared for N3 and the surface declared for N3 as described by its acceptance seeds; behavior outside those seeds is out of scope. Concrete seed references: frontend/src/i18n/locales/zh/landing.ts.

## Normal path

```gherkin
Given a user reaches the surface declared for N3 for item N3
When the flow defined by the interface declared for N3 executes
Then every acceptance seed for item N3 holds  Concrete seed references: frontend/src/i18n/locales/zh/landing.ts.
```

## Exception paths

If the interface declared for N3 fails for item N3, the surface declared for N3 must surface the failure exactly as the acceptance seeds below specify; no exception handling beyond those seeds is in scope. Concrete seed references: frontend/src/i18n/locales/zh/landing.ts.

## Invariants

For item N3, the interface declared for N3 must continue to satisfy every acceptance seed below on every call; the surface declared for N3 must never show a state the seeds forbid. Concrete seed references: frontend/src/i18n/locales/zh/landing.ts.

## Data impact

Item N3 constrains any create, update, or delete reachable through the interface declared for N3; only the acceptance seeds below define what data changes are permitted for the surface declared for N3. Node-specific data assertions: frontend/src/i18n/locales/zh/landing.ts 的中文首页定位不再使用“多模型 API 公益体验与开发接入支持”，改为以经验分享和开发接入为中心的准确文案；frontend/src/views/HomeView.vue 保留已批准的 rest2build 名称、口号和对 Codex、Claude Code 接入服务的描述，不扩大未证实服务能力。 | frontend/src/i18n/locales/zh/landing.ts 的接入模式标题以中文表达，例如“本站托管接入”或等义已确认术语；禁止剩余的 Managed Gateway、BYOK 等孤立英文标题。frontend/src/i18n/locales/en/landing.ts 保持完整英文表达，不以中文资源替代；新增本地化键必须在两种 locale 中存在。 Concrete seed references: frontend/src/i18n/locales/zh/landing.ts.

## Permissions

Item N3 is owned by product-owner; access to the interface declared for N3 and the surface declared for N3 follows the acceptance seeds below and no wider grant. Concrete seed references: frontend/src/i18n/locales/zh/landing.ts.

## Performance and reliability

No performance or reliability threshold beyond existing behavior is declared for item N3; correctness of the interface declared for N3 is governed entirely by the acceptance seeds below. Concrete seed references: frontend/src/i18n/locales/zh/landing.ts.

## Acceptance criteria

| ID | Class | Source requirement refs | Lane | Requirement | Verification layer | Mode | Blocking |
| --- | --- | --- | --- | --- | --- | --- | --- |
| AC-01 | behavior | none | machine/unit | frontend/src/i18n/locales/zh/landing.ts 的中文首页定位不再使用“多模型 API 公益体验与开发接入支持”，改为以经验分享和开发接入为中心的准确文案；frontend/src/views/HomeView.vue 保留已批准的 rest2build 名称、口号和对 Codex、Claude Code 接入服务的描述，不扩大未证实服务能力。 | Unit | Automatic | Yes |
| AC-02 | behavior | none | machine/integration-contract | frontend/src/i18n/locales/zh/landing.ts 的接入模式标题以中文表达，例如“本站托管接入”或等义已确认术语；禁止剩余的 Managed Gateway、BYOK 等孤立英文标题。frontend/src/i18n/locales/en/landing.ts 保持完整英文表达，不以中文资源替代；新增本地化键必须在两种 locale 中存在。 | Integration | Automatic | Yes |

## Human acceptance

Item N3 is fully determined by its acceptance seeds; outcomes for the interface declared for N3 on the surface declared for N3 are machine-verifiable, so no human judgment step is declared.

## Protected acceptance tests

| Path | SHA-256 | Covers |
| --- | --- | --- |
| none | none | Behavior specification only for item N3; executable baseline not yet locked. Concrete seed references: frontend/src/i18n/locales/zh/landing.ts |

## Requirements-test traceability

| Requirement | Verification | Evidence target | Mode | Blocking |
| --- | --- | --- | --- | --- |
| AC-01 | Unit | frontend/src/i18n/locales/zh/landing.ts | Automatic | Yes |
| AC-02 | Integration | frontend/src/i18n/locales/zh/landing.ts | Automatic | Yes |

## Exploratory testing

Probe the surface declared for N3 for item N3 under retry, interruption, and boundary-value inputs against the interface declared for N3, beyond the deterministic acceptance seeds below.

## Production monitoring and rollback

Rollback for item N3 reverts the change to the interface declared for N3; no bespoke production metric is declared beyond the acceptance seeds below.

## Risks and open decisions

<!-- shared_acceptance_policy: SAP-EXPERIENCE-CENTER-LOCAL@1 -->
Node-specific increment for item N3: review risks specific to the surface declared for N3 and record any open decision.
