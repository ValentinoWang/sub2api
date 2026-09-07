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
- Decision refs: codex-migration.documentation-scope@1
- Assumption IDs: none
- Invalidation keys: task.n5
- AC budget: 3
- Baseline identity: ssot-input.json#items[N5]
- Product Context refs: none
- Role Context refs: none
- Resolved Surface Contract refs: none
- Screen Contract ref: none
- Visual Contract refs: none
- UI Change declaration: none
- Human acceptance workspace: none

## User and scenario

The user reaching the surface declared for N5 drives item N5 (unspecified dimension) through the interface declared for N5. Concrete seed references: frontend/prerender.config.ts, frontend/public/codex-session-migrate-manifest.json, frontend/src/components/keys/UseKeyModal.vue.

## Problem

Item N5 exists because the interface declared for N5 does not yet satisfy the acceptance seeds registered for it, leaving the surface declared for N5 incomplete. Concrete seed references: frontend/prerender.config.ts, frontend/public/codex-session-migrate-manifest.json, frontend/src/components/keys/UseKeyModal.vue.

## Expected outcome

After item N5 lands, the interface declared for N5 satisfies every acceptance seed below and the surface declared for N5 reflects that behavior. Concrete seed references: frontend/prerender.config.ts, frontend/public/codex-session-migrate-manifest.json, frontend/src/components/keys/UseKeyModal.vue.

## Non-goals

Item N5 covers only the interface declared for N5 and the surface declared for N5 as described by its acceptance seeds; behavior outside those seeds is out of scope. Concrete seed references: frontend/prerender.config.ts, frontend/public/codex-session-migrate-manifest.json, frontend/src/components/keys/UseKeyModal.vue.

## Normal path

```gherkin
Given a user reaches the surface declared for N5 for item N5
When the flow defined by the interface declared for N5 executes
Then every acceptance seed for item N5 holds  Concrete seed references: frontend/prerender.config.ts, frontend/public/codex-session-migrate-manifest.json, frontend/src/components/keys/UseKeyModal.vue.
```

## Exception paths

If the interface declared for N5 fails for item N5, the surface declared for N5 must surface the failure exactly as the acceptance seeds below specify; no exception handling beyond those seeds is in scope. Concrete seed references: frontend/prerender.config.ts, frontend/public/codex-session-migrate-manifest.json, frontend/src/components/keys/UseKeyModal.vue.

## Invariants

For item N5, the interface declared for N5 must continue to satisfy every acceptance seed below on every call; the surface declared for N5 must never show a state the seeds forbid. Concrete seed references: frontend/prerender.config.ts, frontend/public/codex-session-migrate-manifest.json, frontend/src/components/keys/UseKeyModal.vue.

## Data impact

Item N5 constrains any create, update, or delete reachable through the interface declared for N5; only the acceptance seeds below define what data changes are permitted for the surface declared for N5. Node-specific data assertions: frontend/src/components/keys/UseKeyModal.vue 各 Codex 接入场景提供迁移入口并使用与当前示例一致的目标 provider 和 Base URL；OpenAI 与 sub2api 可因场景不同共存，不自动重命名用户已有配置，地址和凭据不得进入链接参数。 | frontend/prerender.config.ts、经验网页及 frontend/public/codex-session-migrate-prompt.txt 的指令由同一内容定义产生；入口允许公开访问且后端模式可达，关闭 JavaScript 仍能看到完整症状、指令和真实下载链接。 | frontend/public/codex-session-migrate-manifest.json 与工具包实际版本、大小、摘要、系统支持矩阵一致；页面和包作为同一发布候选更新，404、HTML 回退、缺附件或缓存旧包均导致下载验收失败。 Concrete seed references: frontend/prerender.config.ts, frontend/public/codex-session-migrate-manifest.json, frontend/src/components/keys/UseKeyModal.vue.

## Permissions

Item N5 is owned by product-owner; access to the interface declared for N5 and the surface declared for N5 follows the acceptance seeds below and no wider grant. Concrete seed references: frontend/prerender.config.ts, frontend/public/codex-session-migrate-manifest.json, frontend/src/components/keys/UseKeyModal.vue.

## Performance and reliability

No performance or reliability threshold beyond existing behavior is declared for item N5; correctness of the interface declared for N5 is governed entirely by the acceptance seeds below. Concrete seed references: frontend/prerender.config.ts, frontend/public/codex-session-migrate-manifest.json, frontend/src/components/keys/UseKeyModal.vue.

## Acceptance criteria

| ID | Class | Source requirement refs | Lane | Requirement | Verification layer | Mode | Blocking |
| --- | --- | --- | --- | --- | --- | --- | --- |
| AC-01 | behavior | none | machine/unit | frontend/src/components/keys/UseKeyModal.vue 各 Codex 接入场景提供迁移入口并使用与当前示例一致的目标 provider 和 Base URL；OpenAI 与 sub2api 可因场景不同共存，不自动重命名用户已有配置，地址和凭据不得进入链接参数。 | Unit | Automatic | Yes |
| AC-02 | behavior | none | machine/integration-contract | frontend/prerender.config.ts、经验网页及 frontend/public/codex-session-migrate-prompt.txt 的指令由同一内容定义产生；入口允许公开访问且后端模式可达，关闭 JavaScript 仍能看到完整症状、指令和真实下载链接。 | Integration | Automatic | Yes |
| AC-03 | behavior | none | machine/e2e | frontend/public/codex-session-migrate-manifest.json 与工具包实际版本、大小、摘要、系统支持矩阵一致；页面和包作为同一发布候选更新，404、HTML 回退、缺附件或缓存旧包均导致下载验收失败。 | E2E | Automatic | Yes |

## Human acceptance

Item N5 is fully determined by its acceptance seeds; outcomes for the interface declared for N5 on the surface declared for N5 are machine-verifiable, so no human judgment step is declared.

## Protected acceptance tests

| Path | SHA-256 | Covers |
| --- | --- | --- |
| none | none | Behavior specification only for item N5; executable baseline not yet locked. Concrete seed references: frontend/prerender.config.ts, frontend/public/codex-session-migrate-manifest.json, frontend/src/components/keys/UseKeyModal.vue |

## Requirements-test traceability

| Requirement | Verification | Evidence target | Mode | Blocking |
| --- | --- | --- | --- | --- |
| AC-01 | Unit | frontend/src/components/keys/UseKeyModal.vue | Automatic | Yes |
| AC-02 | Integration | frontend/prerender.config.ts | Automatic | Yes |
| AC-03 | E2E | frontend/public/codex-session-migrate-manifest.json | Automatic | Yes |

## Exploratory testing

Probe the surface declared for N5 for item N5 under retry, interruption, and boundary-value inputs against the interface declared for N5, beyond the deterministic acceptance seeds below.

## Production monitoring and rollback

Rollback for item N5 reverts the change to the interface declared for N5; no bespoke production metric is declared beyond the acceptance seeds below.

## Risks and open decisions

<!-- shared_acceptance_policy: SAP-MIGRATION-EVIDENCE@1 -->
Node-specific increment for item N5: review risks specific to the surface declared for N5 and record any open decision.
