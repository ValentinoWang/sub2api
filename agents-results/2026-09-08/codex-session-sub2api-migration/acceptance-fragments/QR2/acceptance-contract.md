# Acceptance Contract: QR2

- Task ID: QR2
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
- Request source: release:R2#gate:QR2
- SSOT node: QR2
- SSOT path: .ssot/nodes/QR2.json
- Readiness mode: FORMAL
- Decision refs: codex-migration.documentation-scope@1
- Assumption IDs: none
- Invalidation keys: task.qr2
- AC budget: 2
- Baseline identity: ssot-input.json#items[QR2]
- Product Context refs: none
- Role Context refs: none
- Resolved Governance Contract refs: none
- Screen Contract ref: none
- Visual Contract refs: none
- UI Change declaration: none
- Human acceptance workspace: none

## User and scenario

An authorized decision authority reviews the evidence and records the signed decision for item QR2. Concrete seed references: /error-experiences/codex-session-migration, docs/human-acceptance/CODEX_SESSION_MIGRATION.md.

## Problem

Item QR2 remains open because its required governance decision has not yet been recorded in the isolated decision record. Concrete seed references: /error-experiences/codex-session-migration, docs/human-acceptance/CODEX_SESSION_MIGRATION.md.

## Expected outcome

After item QR2 is accepted, the signed decision record exists, is attributable to the declared authority, and is bound to the seeds below. Concrete seed references: /error-experiences/codex-session-migration, docs/human-acceptance/CODEX_SESSION_MIGRATION.md.

## Non-goals

Item QR2 covers only the governance decision and its isolated record; implementation changes are owned by the downstream item named in the seeds. Concrete seed references: /error-experiences/codex-session-migration, docs/human-acceptance/CODEX_SESSION_MIGRATION.md.

## Normal path

```gherkin
Given the declared decision authority reviews item QR2 evidence
When the authority records the decision in the isolated decision record
Then every acceptance seed for item QR2 holds  Concrete seed references: /error-experiences/codex-session-migration, docs/human-acceptance/CODEX_SESSION_MIGRATION.md.
```

## Exception paths

If item QR2 lacks an authorized signed decision or its record is invalid, promotion must stop and the failure must be recorded. Concrete seed references: /error-experiences/codex-session-migration, docs/human-acceptance/CODEX_SESSION_MIGRATION.md.

## Invariants

Item QR2 must retain an immutable, attributable decision record; no implementation or runtime state may be inferred from an unsigned recommendation. Concrete seed references: /error-experiences/codex-session-migration, docs/human-acceptance/CODEX_SESSION_MIGRATION.md.

## Data impact

Item QR2 writes only its isolated decision record; it must not modify implementation files, generated nodes, or evidence collectors. Concrete seed references: /error-experiences/codex-session-migration, docs/human-acceptance/CODEX_SESSION_MIGRATION.md.

## Permissions

Only the declared decision authority may accept item QR2; the execution owner and downstream implementer cannot substitute for that authority. Concrete seed references: /error-experiences/codex-session-migration, docs/human-acceptance/CODEX_SESSION_MIGRATION.md.

## Performance and reliability

No performance or reliability threshold beyond existing behavior is declared for item QR2; correctness of the interface declared for QR2 is governed entirely by the acceptance seeds below. Concrete seed references: /error-experiences/codex-session-migration, docs/human-acceptance/CODEX_SESSION_MIGRATION.md.

## Acceptance criteria

| ID | Class | Source requirement refs | Lane | Requirement | Verification layer | Mode | Blocking |
| --- | --- | --- | --- | --- | --- | --- | --- |
| AC-01 | behavior | RQ-SITE | human | /experiences 能进入 /error-experiences/codex-session-migration，P1 返回路径保持原接入场景；生产下载的文件与页面展示版本和摘要一致，失败下载不得返回伪装成脚本的前端 HTML。 | Human decision | Manual / Authority-attested | Yes |
| AC-02 | behavior | RQ-SITE | human | docs/human-acceptance/CODEX_SESSION_MIGRATION.md 对 macOS、Linux、Windows 分别记录真实下载、运行、旧任务接续结果及验收人和复核人；自动测试不能代替签字，尚无记录时状态保持待验收。 | Human decision | Manual / Authority-attested | Yes |

## Human acceptance

Decision item QR2 requires an authorized human decision against the acceptance seeds; record the signed decision before promotion.

## Protected acceptance tests

| Path | SHA-256 | Covers |
| --- | --- | --- |
| none | none | Behavior specification only for item QR2; executable baseline not yet locked. Concrete seed references: /error-experiences/codex-session-migration, docs/human-acceptance/CODEX_SESSION_MIGRATION.md |

## Requirements-test traceability

| Requirement | Verification | Evidence target | Mode | Blocking |
| --- | --- | --- | --- | --- |
| AC-01 | Human decision | /error-experiences/codex-session-migration | Manual / Authority-attested | Yes |
| AC-02 | Human decision | docs/human-acceptance/CODEX_SESSION_MIGRATION.md | Manual / Authority-attested | Yes |

## Exploratory testing

Review item QR2 for missing signatures, stale references, duplicate records, and attempts to promote an unsigned recommendation. Concrete seed references: /error-experiences/codex-session-migration, docs/human-acceptance/CODEX_SESSION_MIGRATION.md.

## Production monitoring and rollback

To roll back item QR2, invalidate its decision record and return the dependent implementation node to its pre-decision state. Concrete seed references: /error-experiences/codex-session-migration, docs/human-acceptance/CODEX_SESSION_MIGRATION.md.

## Risks and open decisions

<!-- shared_acceptance_policy: SAP-MIGRATION-EVIDENCE@1 -->
Node-specific increment for item QR2: review risks specific to RELEASE-R2 and record any open decision.
