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
- Decision refs: rest2build.experience-center.local-hmr-scope@1
- Assumption IDs: none
- Invalidation keys: task.n1
- AC budget: 2
- Baseline identity: ssot-input.json#items[N1]
- Product Context refs: none
- Role Context refs: none
- Resolved Surface Contract refs: none
- Screen Contract ref: none
- Visual Contract refs: none
- UI Change declaration: none
- Human acceptance workspace: none

## User and scenario

The user reaching the surface declared for N1 drives item N1 (unspecified dimension) through the interface declared for N1. Concrete seed references: frontend/src/components/keys/UseKeyModal.vue, frontend/src/content/experiences.ts.

## Problem

Item N1 exists because the interface declared for N1 does not yet satisfy the acceptance seeds registered for it, leaving the surface declared for N1 incomplete. Concrete seed references: frontend/src/components/keys/UseKeyModal.vue, frontend/src/content/experiences.ts.

## Expected outcome

After item N1 lands, the interface declared for N1 satisfies every acceptance seed below and the surface declared for N1 reflects that behavior. Concrete seed references: frontend/src/components/keys/UseKeyModal.vue, frontend/src/content/experiences.ts.

## Non-goals

Item N1 covers only the interface declared for N1 and the surface declared for N1 as described by its acceptance seeds; behavior outside those seeds is out of scope. Concrete seed references: frontend/src/components/keys/UseKeyModal.vue, frontend/src/content/experiences.ts.

## Normal path

```gherkin
Given a user reaches the surface declared for N1 for item N1
When the flow defined by the interface declared for N1 executes
Then every acceptance seed for item N1 holds  Concrete seed references: frontend/src/components/keys/UseKeyModal.vue, frontend/src/content/experiences.ts.
```

## Exception paths

If the interface declared for N1 fails for item N1, the surface declared for N1 must surface the failure exactly as the acceptance seeds below specify; no exception handling beyond those seeds is in scope. Concrete seed references: frontend/src/components/keys/UseKeyModal.vue, frontend/src/content/experiences.ts.

## Invariants

For item N1, the interface declared for N1 must continue to satisfy every acceptance seed below on every call; the surface declared for N1 must never show a state the seeds forbid. Concrete seed references: frontend/src/components/keys/UseKeyModal.vue, frontend/src/content/experiences.ts.

## Data impact

Item N1 constrains any create, update, or delete reachable through the interface declared for N1; only the acceptance seeds below define what data changes are permitted for the surface declared for N1. Node-specific data assertions: frontend/src/content/experiences.ts 以稳定主题标识组织经验；至少区分接入配置、对话接续、模型与用量、故障排查四类。筛选状态可由 URL 查询参数恢复，主题内卡片仍链接到现有详情路由，不能因重排而产生死链。 | frontend/src/components/keys/UseKeyModal.vue 的每个接入环境或问题说明都只链接到相关经验主题或详情，并传入不含服务密钥、账号、Base URL 或其他敏感值的上下文。详情路由可返回原 P1 场景或回到 /experiences，缺少上下文时安全回退。 Concrete seed references: frontend/src/components/keys/UseKeyModal.vue, frontend/src/content/experiences.ts.

## Permissions

Item N1 is owned by product-owner; access to the interface declared for N1 and the surface declared for N1 follows the acceptance seeds below and no wider grant. Concrete seed references: frontend/src/components/keys/UseKeyModal.vue, frontend/src/content/experiences.ts.

## Performance and reliability

No performance or reliability threshold beyond existing behavior is declared for item N1; correctness of the interface declared for N1 is governed entirely by the acceptance seeds below. Concrete seed references: frontend/src/components/keys/UseKeyModal.vue, frontend/src/content/experiences.ts.

## Acceptance criteria

| ID | Class | Source requirement refs | Lane | Requirement | Verification layer | Mode | Blocking |
| --- | --- | --- | --- | --- | --- | --- | --- |
| AC-01 | behavior | none | machine/unit | frontend/src/content/experiences.ts 以稳定主题标识组织经验；至少区分接入配置、对话接续、模型与用量、故障排查四类。筛选状态可由 URL 查询参数恢复，主题内卡片仍链接到现有详情路由，不能因重排而产生死链。 | Unit | Automatic | Yes |
| AC-02 | behavior | none | machine/integration-contract | frontend/src/components/keys/UseKeyModal.vue 的每个接入环境或问题说明都只链接到相关经验主题或详情，并传入不含服务密钥、账号、Base URL 或其他敏感值的上下文。详情路由可返回原 P1 场景或回到 /experiences，缺少上下文时安全回退。 | Integration | Automatic | Yes |

## Human acceptance

Item N1 is fully determined by its acceptance seeds; outcomes for the interface declared for N1 on the surface declared for N1 are machine-verifiable, so no human judgment step is declared.

## Protected acceptance tests

| Path | SHA-256 | Covers |
| --- | --- | --- |
| none | none | Behavior specification only for item N1; executable baseline not yet locked. Concrete seed references: frontend/src/components/keys/UseKeyModal.vue, frontend/src/content/experiences.ts |

## Requirements-test traceability

| Requirement | Verification | Evidence target | Mode | Blocking |
| --- | --- | --- | --- | --- |
| AC-01 | Unit | frontend/src/content/experiences.ts | Automatic | Yes |
| AC-02 | Integration | frontend/src/components/keys/UseKeyModal.vue | Automatic | Yes |

## Exploratory testing

Probe the surface declared for N1 for item N1 under retry, interruption, and boundary-value inputs against the interface declared for N1, beyond the deterministic acceptance seeds below.

## Production monitoring and rollback

Rollback for item N1 reverts the change to the interface declared for N1; no bespoke production metric is declared beyond the acceptance seeds below.

## Risks and open decisions

<!-- shared_acceptance_policy: SAP-EXPERIENCE-CENTER-LOCAL@1 -->
Node-specific increment for item N1: review risks specific to the surface declared for N1 and record any open decision.
