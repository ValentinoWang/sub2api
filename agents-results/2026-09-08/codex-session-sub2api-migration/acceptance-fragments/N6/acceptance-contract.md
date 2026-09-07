# Acceptance Contract: N6

- Task ID: N6
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
- Request source: item N6
- SSOT node: N6
- SSOT path: .ssot/nodes/N6.json
- Readiness mode: FORMAL
- Decision refs: codex-migration.documentation-scope@1
- Assumption IDs: none
- Invalidation keys: task.n6
- AC budget: 3
- Baseline identity: ssot-input.json#items[N6]
- Product Context refs: none
- Role Context refs: none
- Resolved Surface Contract refs: none
- Screen Contract ref: none
- Visual Contract refs: none
- UI Change declaration: none
- Human acceptance workspace: none

## User and scenario

The user reaching the surface declared for N6 drives item N6 (unspecified dimension) through the interface declared for N6. Concrete seed references: backend/internal/web/codex_migration_download_test.go, docs/CODEX_SESSION_MIGRATION_RELEASE.md, docs/human-acceptance/CODEX_SESSION_MIGRATION.md.

## Problem

Item N6 exists because the interface declared for N6 does not yet satisfy the acceptance seeds registered for it, leaving the surface declared for N6 incomplete. Concrete seed references: backend/internal/web/codex_migration_download_test.go, docs/CODEX_SESSION_MIGRATION_RELEASE.md, docs/human-acceptance/CODEX_SESSION_MIGRATION.md.

## Expected outcome

After item N6 lands, the interface declared for N6 satisfies every acceptance seed below and the surface declared for N6 reflects that behavior. Concrete seed references: backend/internal/web/codex_migration_download_test.go, docs/CODEX_SESSION_MIGRATION_RELEASE.md, docs/human-acceptance/CODEX_SESSION_MIGRATION.md.

## Non-goals

Item N6 covers only the interface declared for N6 and the surface declared for N6 as described by its acceptance seeds; behavior outside those seeds is out of scope. Concrete seed references: backend/internal/web/codex_migration_download_test.go, docs/CODEX_SESSION_MIGRATION_RELEASE.md, docs/human-acceptance/CODEX_SESSION_MIGRATION.md.

## Normal path

```gherkin
Given a user reaches the surface declared for N6 for item N6
When the flow defined by the interface declared for N6 executes
Then every acceptance seed for item N6 holds  Concrete seed references: backend/internal/web/codex_migration_download_test.go, docs/CODEX_SESSION_MIGRATION_RELEASE.md, docs/human-acceptance/CODEX_SESSION_MIGRATION.md.
```

## Exception paths

If the interface declared for N6 fails for item N6, the surface declared for N6 must surface the failure exactly as the acceptance seeds below specify; no exception handling beyond those seeds is in scope. Concrete seed references: backend/internal/web/codex_migration_download_test.go, docs/CODEX_SESSION_MIGRATION_RELEASE.md, docs/human-acceptance/CODEX_SESSION_MIGRATION.md.

## Invariants

For item N6, the interface declared for N6 must continue to satisfy every acceptance seed below on every call; the surface declared for N6 must never show a state the seeds forbid. Concrete seed references: backend/internal/web/codex_migration_download_test.go, docs/CODEX_SESSION_MIGRATION_RELEASE.md, docs/human-acceptance/CODEX_SESSION_MIGRATION.md.

## Data impact

Item N6 constrains any create, update, or delete reachable through the interface declared for N6; only the acceptance seeds below define what data changes are permitted for the surface declared for N6. Node-specific data assertions: backend/internal/web/codex_migration_download_test.go 验证真实嵌入产物能提供可解包下载及匹配清单，未知下载路径返回明确失败；公开站点按相同路径回读可见入口、正文和下载内容，不能用 Vite 预览代替。 | docs/human-acceptance/CODEX_SESSION_MIGRATION.md 只包含用户步骤、可见结果、通过或阻塞条件及验收人复核人签字；技术版本、命令、日志和发布回滚证据只进入 docs/CODEX_SESSION_MIGRATION_RELEASE.md。 | docs/CODEX_SESSION_MIGRATION_RELEASE.md 区分本机结构校验、真实客户端接续与线上分发；缺任一系统真实客户端或签字证据时明确待验收，不以其他系统通过替代，也不要求读者拥有服务器管理权限。 Concrete seed references: backend/internal/web/codex_migration_download_test.go, docs/CODEX_SESSION_MIGRATION_RELEASE.md, docs/human-acceptance/CODEX_SESSION_MIGRATION.md.

## Permissions

Item N6 is owned by product-owner; access to the interface declared for N6 and the surface declared for N6 follows the acceptance seeds below and no wider grant. Concrete seed references: backend/internal/web/codex_migration_download_test.go, docs/CODEX_SESSION_MIGRATION_RELEASE.md, docs/human-acceptance/CODEX_SESSION_MIGRATION.md.

## Performance and reliability

No performance or reliability threshold beyond existing behavior is declared for item N6; correctness of the interface declared for N6 is governed entirely by the acceptance seeds below. Concrete seed references: backend/internal/web/codex_migration_download_test.go, docs/CODEX_SESSION_MIGRATION_RELEASE.md, docs/human-acceptance/CODEX_SESSION_MIGRATION.md.

## Acceptance criteria

| ID | Class | Source requirement refs | Lane | Requirement | Verification layer | Mode | Blocking |
| --- | --- | --- | --- | --- | --- | --- | --- |
| AC-01 | behavior | none | machine/integration-contract | backend/internal/web/codex_migration_download_test.go 验证真实嵌入产物能提供可解包下载及匹配清单，未知下载路径返回明确失败；公开站点按相同路径回读可见入口、正文和下载内容，不能用 Vite 预览代替。 | Integration | Automatic | Yes |
| AC-02 | behavior | none | production | docs/human-acceptance/CODEX_SESSION_MIGRATION.md 只包含用户步骤、可见结果、通过或阻塞条件及验收人复核人签字；技术版本、命令、日志和发布回滚证据只进入 docs/CODEX_SESSION_MIGRATION_RELEASE.md。 | Production | Automatic | Yes |
| AC-03 | behavior | none | human | docs/CODEX_SESSION_MIGRATION_RELEASE.md 区分本机结构校验、真实客户端接续与线上分发；缺任一系统真实客户端或签字证据时明确待验收，不以其他系统通过替代，也不要求读者拥有服务器管理权限。 | Human | Manual / Authority-attested | Yes |

## Human acceptance

Item N6 is fully determined by its acceptance seeds; outcomes for the interface declared for N6 on the surface declared for N6 are machine-verifiable, so no human judgment step is declared.

## Protected acceptance tests

| Path | SHA-256 | Covers |
| --- | --- | --- |
| none | none | Behavior specification only for item N6; executable baseline not yet locked. Concrete seed references: backend/internal/web/codex_migration_download_test.go, docs/CODEX_SESSION_MIGRATION_RELEASE.md, docs/human-acceptance/CODEX_SESSION_MIGRATION.md |

## Requirements-test traceability

| Requirement | Verification | Evidence target | Mode | Blocking |
| --- | --- | --- | --- | --- |
| AC-01 | Integration | backend/internal/web/codex_migration_download_test.go | Automatic | Yes |
| AC-02 | Production | docs/human-acceptance/CODEX_SESSION_MIGRATION.md | Automatic | Yes |
| AC-03 | Human | docs/CODEX_SESSION_MIGRATION_RELEASE.md | Manual / Authority-attested | Yes |

## Exploratory testing

Probe the surface declared for N6 for item N6 under retry, interruption, and boundary-value inputs against the interface declared for N6, beyond the deterministic acceptance seeds below.

## Production monitoring and rollback

Rollback for item N6 reverts the change to the interface declared for N6; no bespoke production metric is declared beyond the acceptance seeds below.

## Risks and open decisions

<!-- shared_acceptance_policy: SAP-MIGRATION-EVIDENCE@1 -->
Node-specific increment for item N6: review risks specific to the surface declared for N6 and record any open decision.
