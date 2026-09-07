# Acceptance Contract: N5

- Task ID: N5
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
- Request source: item N5
- SSOT node: N5
- SSOT path: .ssot/nodes/N5.json
- Readiness mode: FORMAL
- Decision refs: rest2build.experience-center.local-hmr-scope@1
- Assumption IDs: none
- Invalidation keys: task.n5
- AC budget: 2
- Baseline identity: ssot-input.json#items[N5]
- Product Context refs: none
- Role Context refs: none
- Resolved Surface Contract refs: none
- Screen Contract ref: none
- Visual Contract refs: none
- UI Change declaration: none
- Human acceptance workspace: none

## User and scenario

The user reaching the surface declared for N5 drives item N5 (unspecified dimension) through the interface declared for N5. Concrete seed references: frontend/package.json, frontend/src/views/public/__tests__/ExperiencesView.spec.ts.

## Problem

Item N5 exists because the interface declared for N5 does not yet satisfy the acceptance seeds registered for it, leaving the surface declared for N5 incomplete. Concrete seed references: frontend/package.json, frontend/src/views/public/__tests__/ExperiencesView.spec.ts.

## Expected outcome

After item N5 lands, the interface declared for N5 satisfies every acceptance seed below and the surface declared for N5 reflects that behavior. Concrete seed references: frontend/package.json, frontend/src/views/public/__tests__/ExperiencesView.spec.ts.

## Non-goals

Item N5 covers only the interface declared for N5 and the surface declared for N5 as described by its acceptance seeds; behavior outside those seeds is out of scope. Concrete seed references: frontend/package.json, frontend/src/views/public/__tests__/ExperiencesView.spec.ts.

## Normal path

```gherkin
Given a user reaches the surface declared for N5 for item N5
When the flow defined by the interface declared for N5 executes
Then every acceptance seed for item N5 holds  Concrete seed references: frontend/package.json, frontend/src/views/public/__tests__/ExperiencesView.spec.ts.
```

## Exception paths

If the interface declared for N5 fails for item N5, the surface declared for N5 must surface the failure exactly as the acceptance seeds below specify; no exception handling beyond those seeds is in scope. Concrete seed references: frontend/package.json, frontend/src/views/public/__tests__/ExperiencesView.spec.ts.

## Invariants

For item N5, the interface declared for N5 must continue to satisfy every acceptance seed below on every call; the surface declared for N5 must never show a state the seeds forbid. Concrete seed references: frontend/package.json, frontend/src/views/public/__tests__/ExperiencesView.spec.ts.

## Data impact

Item N5 constrains any create, update, or delete reachable through the interface declared for N5; only the acceptance seeds below define what data changes are permitted for the surface declared for N5. Node-specific data assertions: frontend/src/views/public/__tests__/ExperiencesView.spec.ts、frontend/src/views/__tests__/HomeView.compact.spec.ts 和 frontend/src/components/keys/__tests__/UseKeyModal.spec.ts 验证全部经验主题都有可达卡片、P1 经验入口不传递敏感值、中文资源不含被禁止的 Managed Gateway 标题、首页定位改为经验分享、公共导航和当前筛选的 aria 状态成立；在既有 Vitest 结构中运行，不复制实现细节为无意义快照。 | frontend/package.json 的指定 Vite HMR 命令在 /experiences、/home 及授权后的 P1 以桌面和 390px 宽度检查深浅主题。记录访问地址、视口、主题、可见结果和控制台状态；验证过程中不登录远端、不创建或修改业务数据，未获得后续授权前不运行生产构建、Go、Docker 或部署。 Concrete seed references: frontend/package.json, frontend/src/views/public/__tests__/ExperiencesView.spec.ts.

## Permissions

Item N5 is owned by product-owner; access to the interface declared for N5 and the surface declared for N5 follows the acceptance seeds below and no wider grant. Concrete seed references: frontend/package.json, frontend/src/views/public/__tests__/ExperiencesView.spec.ts.

## Performance and reliability

No performance or reliability threshold beyond existing behavior is declared for item N5; correctness of the interface declared for N5 is governed entirely by the acceptance seeds below. Concrete seed references: frontend/package.json, frontend/src/views/public/__tests__/ExperiencesView.spec.ts.

## Acceptance criteria

| ID | Class | Source requirement refs | Lane | Requirement | Verification layer | Mode | Blocking |
| --- | --- | --- | --- | --- | --- | --- | --- |
| AC-01 | behavior | none | machine/unit | frontend/src/views/public/__tests__/ExperiencesView.spec.ts、frontend/src/views/__tests__/HomeView.compact.spec.ts 和 frontend/src/components/keys/__tests__/UseKeyModal.spec.ts 验证全部经验主题都有可达卡片、P1 经验入口不传递敏感值、中文资源不含被禁止的 Managed Gateway 标题、首页定位改为经验分享、公共导航和当前筛选的 aria 状态成立；在既有 Vitest 结构中运行，不复制实现细节为无意义快照。 | Unit | Automatic | Yes |
| AC-02 | behavior | none | machine/e2e | frontend/package.json 的指定 Vite HMR 命令在 /experiences、/home 及授权后的 P1 以桌面和 390px 宽度检查深浅主题。记录访问地址、视口、主题、可见结果和控制台状态；验证过程中不登录远端、不创建或修改业务数据，未获得后续授权前不运行生产构建、Go、Docker 或部署。 | E2E | Automatic | Yes |

## Human acceptance

Item N5 is fully determined by its acceptance seeds; outcomes for the interface declared for N5 on the surface declared for N5 are machine-verifiable, so no human judgment step is declared.

## Protected acceptance tests

| Path | SHA-256 | Covers |
| --- | --- | --- |
| none | none | Behavior specification only for item N5; executable baseline not yet locked. Concrete seed references: frontend/package.json, frontend/src/views/public/__tests__/ExperiencesView.spec.ts |

## Requirements-test traceability

| Requirement | Verification | Evidence target | Mode | Blocking |
| --- | --- | --- | --- | --- |
| AC-01 | Unit | frontend/src/views/public/__tests__/ExperiencesView.spec.ts | Automatic | Yes |
| AC-02 | E2E | frontend/package.json | Automatic | Yes |

## Exploratory testing

Probe the surface declared for N5 for item N5 under retry, interruption, and boundary-value inputs against the interface declared for N5, beyond the deterministic acceptance seeds below.

## Production monitoring and rollback

Rollback for item N5 reverts the change to the interface declared for N5; no bespoke production metric is declared beyond the acceptance seeds below.

## Risks and open decisions

<!-- shared_acceptance_policy: SAP-EXPERIENCE-CENTER-LOCAL@1 -->
Node-specific increment for item N5: review risks specific to the surface declared for N5 and record any open decision.
