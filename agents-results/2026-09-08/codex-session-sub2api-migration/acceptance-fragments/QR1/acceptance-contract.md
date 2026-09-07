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
- Decision refs: codex-migration.documentation-scope@1
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

An authorized decision authority reviews the evidence and records the signed decision for item QR1. Concrete seed references: tools/build_codex_session_migrate.py, tools/test_codex_session_migrate.py.

## Problem

Item QR1 remains open because its required governance decision has not yet been recorded in the isolated decision record. Concrete seed references: tools/build_codex_session_migrate.py, tools/test_codex_session_migrate.py.

## Expected outcome

After item QR1 is accepted, the signed decision record exists, is attributable to the declared authority, and is bound to the seeds below. Concrete seed references: tools/build_codex_session_migrate.py, tools/test_codex_session_migrate.py.

## Non-goals

Item QR1 covers only the governance decision and its isolated record; implementation changes are owned by the downstream item named in the seeds. Concrete seed references: tools/build_codex_session_migrate.py, tools/test_codex_session_migrate.py.

## Normal path

```gherkin
Given the declared decision authority reviews item QR1 evidence
When the authority records the decision in the isolated decision record
Then every acceptance seed for item QR1 holds  Concrete seed references: tools/build_codex_session_migrate.py, tools/test_codex_session_migrate.py.
```

## Exception paths

If item QR1 lacks an authorized signed decision or its record is invalid, promotion must stop and the failure must be recorded. Concrete seed references: tools/build_codex_session_migrate.py, tools/test_codex_session_migrate.py.

## Invariants

Item QR1 must retain an immutable, attributable decision record; no implementation or runtime state may be inferred from an unsigned recommendation. Concrete seed references: tools/build_codex_session_migrate.py, tools/test_codex_session_migrate.py.

## Data impact

Item QR1 writes only its isolated decision record; it must not modify implementation files, generated nodes, or evidence collectors. Concrete seed references: tools/build_codex_session_migrate.py, tools/test_codex_session_migrate.py.

## Permissions

Only the declared decision authority may accept item QR1; the execution owner and downstream implementer cannot substitute for that authority. Concrete seed references: tools/build_codex_session_migrate.py, tools/test_codex_session_migrate.py.

## Performance and reliability

No performance or reliability threshold beyond existing behavior is declared for item QR1; correctness of the interface declared for QR1 is governed entirely by the acceptance seeds below. Concrete seed references: tools/build_codex_session_migrate.py, tools/test_codex_session_migrate.py.

## Acceptance criteria

| ID | Class | Source requirement refs | Lane | Requirement | Verification layer | Mode | Blocking |
| --- | --- | --- | --- | --- | --- | --- | --- |
| AC-01 | behavior | RQ-LOCAL | human | tools/test_codex_session_migrate.py 的跨系统夹具、故障注入及回滚检查全部通过且零跳过；真实已支持客户端重开迁移前的同一个任务，能引用迁移前测试事实并追加一轮交流。 | Human decision | Manual / Authority-attested | Yes |
| AC-02 | behavior | RQ-LOCAL | human | tools/build_codex_session_migrate.py 输出可解包的完整离线工具包、版本及摘要；备份不成立或输入漂移时工具退出为阻塞状态，原始配置、任务内容及无关数据库行均保持原值。 | Human decision | Manual / Authority-attested | Yes |

## Human acceptance

Decision item QR1 requires an authorized human decision against the acceptance seeds; record the signed decision before promotion.

## Protected acceptance tests

| Path | SHA-256 | Covers |
| --- | --- | --- |
| none | none | Behavior specification only for item QR1; executable baseline not yet locked. Concrete seed references: tools/build_codex_session_migrate.py, tools/test_codex_session_migrate.py |

## Requirements-test traceability

| Requirement | Verification | Evidence target | Mode | Blocking |
| --- | --- | --- | --- | --- |
| AC-01 | Human decision | tools/test_codex_session_migrate.py | Manual / Authority-attested | Yes |
| AC-02 | Human decision | tools/build_codex_session_migrate.py | Manual / Authority-attested | Yes |

## Exploratory testing

Review item QR1 for missing signatures, stale references, duplicate records, and attempts to promote an unsigned recommendation. Concrete seed references: tools/build_codex_session_migrate.py, tools/test_codex_session_migrate.py.

## Production monitoring and rollback

To roll back item QR1, invalidate its decision record and return the dependent implementation node to its pre-decision state. Concrete seed references: tools/build_codex_session_migrate.py, tools/test_codex_session_migrate.py.

## Risks and open decisions

<!-- shared_acceptance_policy: SAP-MIGRATION-EVIDENCE@1 -->
Node-specific increment for item QR1: review risks specific to RELEASE-R1 and record any open decision.
