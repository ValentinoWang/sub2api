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
- Decision refs: rest2build.experience-center.local-hmr-scope@1
- Assumption IDs: none
- Invalidation keys: task.n4
- AC budget: 2
- Baseline identity: ssot-input.json#items[N4]
- Product Context refs: none
- Role Context refs: none
- Resolved Surface Contract refs: none
- Screen Contract ref: none
- Visual Contract refs: none
- UI Change declaration: none
- Human acceptance workspace: none

## User and scenario

The user reaching the surface declared for N4 drives item N4 (unspecified dimension) through the interface declared for N4. Concrete seed references: frontend/src/components/layout/PublicPageLayout.vue.

## Problem

Item N4 exists because the interface declared for N4 does not yet satisfy the acceptance seeds registered for it, leaving the surface declared for N4 incomplete. Concrete seed references: frontend/src/components/layout/PublicPageLayout.vue.

## Expected outcome

After item N4 lands, the interface declared for N4 satisfies every acceptance seed below and the surface declared for N4 reflects that behavior. Concrete seed references: frontend/src/components/layout/PublicPageLayout.vue.

## Non-goals

Item N4 covers only the interface declared for N4 and the surface declared for N4 as described by its acceptance seeds; behavior outside those seeds is out of scope. Concrete seed references: frontend/src/components/layout/PublicPageLayout.vue.

## Normal path

```gherkin
Given a user reaches the surface declared for N4 for item N4
When the flow defined by the interface declared for N4 executes
Then every acceptance seed for item N4 holds  Concrete seed references: frontend/src/components/layout/PublicPageLayout.vue.
```

## Exception paths

If the interface declared for N4 fails for item N4, the surface declared for N4 must surface the failure exactly as the acceptance seeds below specify; no exception handling beyond those seeds is in scope. Concrete seed references: frontend/src/components/layout/PublicPageLayout.vue.

## Invariants

For item N4, the interface declared for N4 must continue to satisfy every acceptance seed below on every call; the surface declared for N4 must never show a state the seeds forbid. Concrete seed references: frontend/src/components/layout/PublicPageLayout.vue.

## Data impact

Item N4 constrains any create, update, or delete reachable through the interface declared for N4; only the acceptance seeds below define what data changes are permitted for the surface declared for N4. Node-specific data assertions: frontend/src/components/layout/PublicPageLayout.vue 的公共页内容容器根据页面职责使用可扫描的全宽约束；经验中心不被 max-w-3xl 限成狭长单列。导航主要区域在可用宽度内两侧对齐，必要时采用明确的溢出策略或折叠菜单，不能只把标签堆在左侧或产生页面水平滚动。 | frontend/src/components/layout/PublicPageLayout.vue 的深色底层、导航表面、活动标签、卡片边界与正文有可辨别的明度层级；焦点、悬停和当前状态不依赖单一颜色。frontend/src/components/keys/UseKeyModal.vue 的横向标签在 390px 和桌面宽度下可阅读、可点击且不会遮挡操作区。 Concrete seed references: frontend/src/components/layout/PublicPageLayout.vue.

## Permissions

Item N4 is owned by product-owner; access to the interface declared for N4 and the surface declared for N4 follows the acceptance seeds below and no wider grant. Concrete seed references: frontend/src/components/layout/PublicPageLayout.vue.

## Performance and reliability

No performance or reliability threshold beyond existing behavior is declared for item N4; correctness of the interface declared for N4 is governed entirely by the acceptance seeds below. Concrete seed references: frontend/src/components/layout/PublicPageLayout.vue.

## Acceptance criteria

| ID | Class | Source requirement refs | Lane | Requirement | Verification layer | Mode | Blocking |
| --- | --- | --- | --- | --- | --- | --- | --- |
| AC-01 | behavior | none | machine/unit | frontend/src/components/layout/PublicPageLayout.vue 的公共页内容容器根据页面职责使用可扫描的全宽约束；经验中心不被 max-w-3xl 限成狭长单列。导航主要区域在可用宽度内两侧对齐，必要时采用明确的溢出策略或折叠菜单，不能只把标签堆在左侧或产生页面水平滚动。 | Unit | Automatic | Yes |
| AC-02 | behavior | none | machine/e2e | frontend/src/components/layout/PublicPageLayout.vue 的深色底层、导航表面、活动标签、卡片边界与正文有可辨别的明度层级；焦点、悬停和当前状态不依赖单一颜色。frontend/src/components/keys/UseKeyModal.vue 的横向标签在 390px 和桌面宽度下可阅读、可点击且不会遮挡操作区。 | E2E | Automatic | Yes |

## Human acceptance

Item N4 is fully determined by its acceptance seeds; outcomes for the interface declared for N4 on the surface declared for N4 are machine-verifiable, so no human judgment step is declared.

## Protected acceptance tests

| Path | SHA-256 | Covers |
| --- | --- | --- |
| none | none | Behavior specification only for item N4; executable baseline not yet locked. Concrete seed references: frontend/src/components/layout/PublicPageLayout.vue |

## Requirements-test traceability

| Requirement | Verification | Evidence target | Mode | Blocking |
| --- | --- | --- | --- | --- |
| AC-01 | Unit | frontend/src/components/layout/PublicPageLayout.vue | Automatic | Yes |
| AC-02 | E2E | frontend/src/components/layout/PublicPageLayout.vue | Automatic | Yes |

## Exploratory testing

Probe the surface declared for N4 for item N4 under retry, interruption, and boundary-value inputs against the interface declared for N4, beyond the deterministic acceptance seeds below.

## Production monitoring and rollback

Rollback for item N4 reverts the change to the interface declared for N4; no bespoke production metric is declared beyond the acceptance seeds below.

## Risks and open decisions

<!-- shared_acceptance_policy: SAP-EXPERIENCE-CENTER-LOCAL@1 -->
Node-specific increment for item N4: review risks specific to the surface declared for N4 and record any open decision.
