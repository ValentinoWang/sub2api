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
- Decision refs: rest2build.experience-center.local-hmr-scope@1
- Assumption IDs: none
- Invalidation keys: task.n2
- AC budget: 2
- Baseline identity: ssot-input.json#items[N2]
- Product Context refs: none
- Role Context refs: none
- Resolved Surface Contract refs: none
- Screen Contract ref: none
- Visual Contract refs: none
- UI Change declaration: none
- Human acceptance workspace: none

## User and scenario

The user reaching the surface declared for N2 drives item N2 (unspecified dimension) through the interface declared for N2. Concrete seed references: frontend/src/views/public/ExperiencesView.vue, frontend/src/views/public/__tests__/ExperiencesView.spec.ts.

## Problem

Item N2 exists because the interface declared for N2 does not yet satisfy the acceptance seeds registered for it, leaving the surface declared for N2 incomplete. Concrete seed references: frontend/src/views/public/ExperiencesView.vue, frontend/src/views/public/__tests__/ExperiencesView.spec.ts.

## Expected outcome

After item N2 lands, the interface declared for N2 satisfies every acceptance seed below and the surface declared for N2 reflects that behavior. Concrete seed references: frontend/src/views/public/ExperiencesView.vue, frontend/src/views/public/__tests__/ExperiencesView.spec.ts.

## Non-goals

Item N2 covers only the interface declared for N2 and the surface declared for N2 as described by its acceptance seeds; behavior outside those seeds is out of scope. Concrete seed references: frontend/src/views/public/ExperiencesView.vue, frontend/src/views/public/__tests__/ExperiencesView.spec.ts.

## Normal path

```gherkin
Given a user reaches the surface declared for N2 for item N2
When the flow defined by the interface declared for N2 executes
Then every acceptance seed for item N2 holds  Concrete seed references: frontend/src/views/public/ExperiencesView.vue, frontend/src/views/public/__tests__/ExperiencesView.spec.ts.
```

## Exception paths

If the interface declared for N2 fails for item N2, the surface declared for N2 must surface the failure exactly as the acceptance seeds below specify; no exception handling beyond those seeds is in scope. Concrete seed references: frontend/src/views/public/ExperiencesView.vue, frontend/src/views/public/__tests__/ExperiencesView.spec.ts.

## Invariants

For item N2, the interface declared for N2 must continue to satisfy every acceptance seed below on every call; the surface declared for N2 must never show a state the seeds forbid. Concrete seed references: frontend/src/views/public/ExperiencesView.vue, frontend/src/views/public/__tests__/ExperiencesView.spec.ts.

## Data impact

Item N2 constrains any create, update, or delete reachable through the interface declared for N2; only the acceptance seeds below define what data changes are permitted for the surface declared for N2. Node-specific data assertions: frontend/src/views/public/ExperiencesView.vue 在桌面宽度下使用至少两列的稳定网格，frontend/src/components/experiences/ExperienceCard.vue 的卡片等高或用明确的栅格轨道约束；依次呈现主题、标题、摘要、适用对象和可见行动。卡片、筛选和空状态不嵌套成多层装饰卡，圆角不超过 8px。 | frontend/src/views/public/__tests__/ExperiencesView.spec.ts 在 390px 宽度下断言网格降为一列，筛选控件可换行且触达面积不小于 36px；最长中文标题、主题和行动文字不溢出、不覆盖相邻内容，键盘焦点和当前筛选状态可见。frontend/src/components/experiences/ExperienceCollection.vue 使用同一信息层级和卡片 token。 Concrete seed references: frontend/src/views/public/ExperiencesView.vue, frontend/src/views/public/__tests__/ExperiencesView.spec.ts.

## Permissions

Item N2 is owned by product-owner; access to the interface declared for N2 and the surface declared for N2 follows the acceptance seeds below and no wider grant. Concrete seed references: frontend/src/views/public/ExperiencesView.vue, frontend/src/views/public/__tests__/ExperiencesView.spec.ts.

## Performance and reliability

No performance or reliability threshold beyond existing behavior is declared for item N2; correctness of the interface declared for N2 is governed entirely by the acceptance seeds below. Concrete seed references: frontend/src/views/public/ExperiencesView.vue, frontend/src/views/public/__tests__/ExperiencesView.spec.ts.

## Acceptance criteria

| ID | Class | Source requirement refs | Lane | Requirement | Verification layer | Mode | Blocking |
| --- | --- | --- | --- | --- | --- | --- | --- |
| AC-01 | behavior | none | machine/unit | frontend/src/views/public/ExperiencesView.vue 在桌面宽度下使用至少两列的稳定网格，frontend/src/components/experiences/ExperienceCard.vue 的卡片等高或用明确的栅格轨道约束；依次呈现主题、标题、摘要、适用对象和可见行动。卡片、筛选和空状态不嵌套成多层装饰卡，圆角不超过 8px。 | Unit | Automatic | Yes |
| AC-02 | behavior | none | machine/e2e | frontend/src/views/public/__tests__/ExperiencesView.spec.ts 在 390px 宽度下断言网格降为一列，筛选控件可换行且触达面积不小于 36px；最长中文标题、主题和行动文字不溢出、不覆盖相邻内容，键盘焦点和当前筛选状态可见。frontend/src/components/experiences/ExperienceCollection.vue 使用同一信息层级和卡片 token。 | E2E | Automatic | Yes |

## Human acceptance

Item N2 is fully determined by its acceptance seeds; outcomes for the interface declared for N2 on the surface declared for N2 are machine-verifiable, so no human judgment step is declared.

## Protected acceptance tests

| Path | SHA-256 | Covers |
| --- | --- | --- |
| none | none | Behavior specification only for item N2; executable baseline not yet locked. Concrete seed references: frontend/src/views/public/ExperiencesView.vue, frontend/src/views/public/__tests__/ExperiencesView.spec.ts |

## Requirements-test traceability

| Requirement | Verification | Evidence target | Mode | Blocking |
| --- | --- | --- | --- | --- |
| AC-01 | Unit | frontend/src/views/public/ExperiencesView.vue | Automatic | Yes |
| AC-02 | E2E | frontend/src/views/public/__tests__/ExperiencesView.spec.ts | Automatic | Yes |

## Exploratory testing

Probe the surface declared for N2 for item N2 under retry, interruption, and boundary-value inputs against the interface declared for N2, beyond the deterministic acceptance seeds below.

## Production monitoring and rollback

Rollback for item N2 reverts the change to the interface declared for N2; no bespoke production metric is declared beyond the acceptance seeds below.

## Risks and open decisions

<!-- shared_acceptance_policy: SAP-EXPERIENCE-CENTER-LOCAL@1 -->
Node-specific increment for item N2: review risks specific to the surface declared for N2 and record any open decision.
