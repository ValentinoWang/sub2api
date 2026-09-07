# Acceptance Contract: QR1

- Task ID: QR1
- Contract kind: release-decision
- Contract profile: acceptance-contract-kind-profiles@1
- Verification layer: Human decision
- Acceptance mode: Manual / Authority-attested
- Evidence target: signed decision record
- Contract version: 1
- Contract status: DRAFT
- Test baseline: PLANNED
- Acceptance owner: product-owner
- Execution actor: human
- Approval evidence: TBD
- Request source: release:R1#gate:QR1
- SSOT node: QR1
- SSOT path: .ssot/nodes/QR1.json
- Readiness mode: FORMAL
- Decision refs: rest2build.experience-center.local-hmr-scope@1
- Assumption IDs: none
- Invalidation keys: task.qr1
- AC budget: 2
- Baseline identity: ssot-input.json#items[QR1]
- Product Context refs: none
- Role Context refs: none
- Resolved Governance Contract refs: none
- Screen Contract ref: none
- Visual Contract refs: none
- UI Change declaration: none
- Human acceptance workspace: none

## User and scenario

An authorized decision authority reviews the evidence and records the signed decision for item QR1. Concrete seed references: frontend/package.json, frontend/src/views/public/__tests__/ExperiencesView.spec.ts.

## Problem

Item QR1 remains open because its required governance decision has not yet been recorded in the isolated decision record. Concrete seed references: frontend/package.json, frontend/src/views/public/__tests__/ExperiencesView.spec.ts.

## Expected outcome

After item QR1 is accepted, the signed decision record exists, is attributable to the declared authority, and is bound to the seeds below. Concrete seed references: frontend/package.json, frontend/src/views/public/__tests__/ExperiencesView.spec.ts.

## Non-goals

Item QR1 covers only the governance decision and its isolated record; implementation changes are owned by the downstream item named in the seeds. Concrete seed references: frontend/package.json, frontend/src/views/public/__tests__/ExperiencesView.spec.ts.

## Normal path

```gherkin
Given the declared decision authority reviews item QR1 evidence
When the authority records the decision in the isolated decision record
Then every acceptance seed for item QR1 holds  Concrete seed references: frontend/package.json, frontend/src/views/public/__tests__/ExperiencesView.spec.ts.
```

## Exception paths

If item QR1 lacks an authorized signed decision or its record is invalid, promotion must stop and the failure must be recorded. Concrete seed references: frontend/package.json, frontend/src/views/public/__tests__/ExperiencesView.spec.ts.

## Invariants

Item QR1 must retain an immutable, attributable decision record; no implementation or runtime state may be inferred from an unsigned recommendation. Concrete seed references: frontend/package.json, frontend/src/views/public/__tests__/ExperiencesView.spec.ts.

## Data impact

Item QR1 writes only its isolated decision record; it must not modify implementation files, generated nodes, or evidence collectors. Concrete seed references: frontend/package.json, frontend/src/views/public/__tests__/ExperiencesView.spec.ts.

## Permissions

Only the declared decision authority may accept item QR1; the execution owner and downstream implementer cannot substitute for that authority. Concrete seed references: frontend/package.json, frontend/src/views/public/__tests__/ExperiencesView.spec.ts.

## Performance and reliability

No performance or reliability threshold beyond existing behavior is declared for item QR1; correctness of the interface declared for QR1 is governed entirely by the acceptance seeds below. Concrete seed references: frontend/package.json, frontend/src/views/public/__tests__/ExperiencesView.spec.ts.

## Acceptance criteria

| ID | Class | Source requirement refs | Lane | Requirement | Verification layer | Mode | Blocking |
| --- | --- | --- | --- | --- | --- | --- | --- |
| AC-01 | behavior | RQ-LOCAL-HMR | human | 使用 frontend/package.json 所定义的 Vite HMR，并以 VITE_DEV_PROXY_TARGET=https://ai.rest2build.lol 和 VITE_DEV_PORT=4174 启动；在 /experiences、/home 和登录后 P1 截取桌面及 390px 宽度的深浅主题证据，不执行任何会改变远端业务状态的动作。 | Human decision | Manual / Authority-attested | Yes |
| AC-02 | behavior | RQ-LOCAL-HMR | human | frontend/src/views/public/__tests__/ExperiencesView.spec.ts、frontend/src/views/__tests__/HomeView.compact.spec.ts 与 frontend/src/components/keys/__tests__/UseKeyModal.spec.ts 覆盖主题分类、卡片链接、中文本地化和导航可访问状态；视觉检查断言无水平滚动、无文字遮挡、无控制台错误，且所有流程只访问 http://127.0.0.1:4174。 | Human decision | Manual / Authority-attested | Yes |

## Human acceptance

Decision item QR1 requires an authorized human decision against the acceptance seeds; record the signed decision before promotion.

## Protected acceptance tests

| Path | SHA-256 | Covers |
| --- | --- | --- |
| none | none | Behavior specification only for item QR1; executable baseline not yet locked. Concrete seed references: frontend/package.json, frontend/src/views/public/__tests__/ExperiencesView.spec.ts |

## Requirements-test traceability

| Requirement | Verification | Evidence target | Mode | Blocking |
| --- | --- | --- | --- | --- |
| AC-01 | Human decision | frontend/package.json | Manual / Authority-attested | Yes |
| AC-02 | Human decision | frontend/src/views/public/__tests__/ExperiencesView.spec.ts | Manual / Authority-attested | Yes |

## Exploratory testing

Review item QR1 for missing signatures, stale references, duplicate records, and attempts to promote an unsigned recommendation. Concrete seed references: frontend/package.json, frontend/src/views/public/__tests__/ExperiencesView.spec.ts.

## Production monitoring and rollback

To roll back item QR1, invalidate its decision record and return the dependent implementation node to its pre-decision state. Concrete seed references: frontend/package.json, frontend/src/views/public/__tests__/ExperiencesView.spec.ts.

## Risks and open decisions

<!-- shared_acceptance_policy: SAP-EXPERIENCE-CENTER-LOCAL@1 -->
Node-specific increment for item QR1: review risks specific to RELEASE-R1 and record any open decision.
