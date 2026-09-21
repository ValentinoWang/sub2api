# Acceptance Contract: upstream-ticket-capability

- Task ID: upstream-ticket-capability
- Contract version: 2
- Contract status: DRAFT
- Test baseline: PLANNED
- Acceptance owner: 产品负责人
- Approval evidence: TBD
- Request source: 2026-09-21 用户要求升级源码并提供打票方案，后续排除 VPN/VPS/ISP 订阅改造，追加会话恢复经验及 CC Switch 默认模型 gpt-6-astra
- SSOT node: none
- SSOT path: none
- Readiness mode: FORMAL
- Decision refs: none
- Assumption IDs: none
- Invalidation keys: upstream-ticket-capability.source, upstream-ticket-capability.design, upstream-ticket-capability.ccswitch
- AC budget: 3
- Baseline identity: 5d839653d2ed02965a43964f8402401c5ea69a8c
- Product Context refs: none
- Role Context refs: none
- Resolved Surface Contract refs: none
- Screen Contract ref: none
- Visual Contract refs: none
- UI Change declaration: none
- Human acceptance workspace: acceptance/human/2026-W39/2026-09-21-upstream-ticket-capability

## User and scenario

管理员需要当前源码包含固定的上游更新，并能评审仅聚焦打票能力的修改方案；用户补充要求 CC Switch 导入默认模型改为 gpt-6-astra。相关方案为同目录 ssot-development.md，尚未形成批准的版本化打票实施决定。本合同覆盖源码集成、方案交付与 CC Switch 导出设置，不声明实际部署或新的打票实现完成。

## Problem

当前源码落后于上游；现有打票只检查响应头，缺少完整协议验证、持久预算及关联失效策略。

## Expected outcome

固定上游提交被纳入候选；完整本地 CI 留存。方案区分当前事实、提议与真实上游验证边界，并具有可执行的后续验收场景。CC Switch 导入使用 gpt-6-astra、当前站点地址和选中密钥，推理与用量路径正确。

## Non-goals

真实部署、运行数据变更、真实打票、VPN/VPS/ISP 订阅改造、支付启用及打票功能实施不在本轮完成声明中。

## Normal path

读取两个项目当前实现，固定升级来源，在隔离工作区解决合并冲突，以本地 CI 验证候选；将方案与未决问题保存并核验 Obsidian 副本。

## Exception paths

任一必需检查失败时保留失败与未执行阶段；不提升失败候选。真实账号和协议行为没有现场验证时，不宣称上游闭环或人工通过。

## Invariants

保留定制行为、测试和真实运行数据。原票和凭据不进入文档。没有人工签署时保持未验收状态。

## Data impact

仅修改源码和方案文档；不在自用数据库上运行破坏性测试。SSOT 副本只包含声明的两份文档与生成清单。

## Permissions

源码升级及方案编写来自当前用户请求；不扩大为部署、支付、真实供应商提交或他人通知授权。

## Performance and reliability

本次以完整本地 CI 和重点合并回归为证据；方案中的采集预算、退避与并发数字是后续实现提议，尚未做容量承诺。

## Acceptance criteria

| ID | Class | Source requirement refs | Lane | Requirement | Verification layer | Mode | Blocking |
| --- | --- | --- | --- | --- | --- | --- | --- |
| AC-01 | behavior | 当前用户升级请求 | machine/integration-contract | 固定目标 7c700729c 被纳入，定制回归与完整本地 CI 通过；失败历史保留 | Integration | Automatic | Yes |
| AC-02 | behavior | 当前用户打票方案与范围澄清 | machine/static | 方案仅聚焦打票；源文件、快照哈希、完整归档审计及链接核验通过 | Static | Automatic | Yes |
| AC-03 | behavior | 用户追加 CC Switch 默认设置并选择 gpt-6-astra | machine/unit | 导入模型、选中密钥、站点显示名和路径正确；不请求自动激活；其他客户端选择保留 | Unit | Automatic | Yes |

## Human acceptance

| ID | Summary | Checklist path | Required role | Blocking |
| --- | --- | --- | --- | --- |
| H-01 | 方案中的成功、失败与待验证边界清楚，满足预期改造范围 | acceptance/human/2026-W39/2026-09-21-upstream-ticket-capability/checklist.md#h-01 | 产品负责人 | Yes |
| H-02 | CC Switch 导入确认页显示预期模型及站点 | acceptance/human/2026-W39/2026-09-21-upstream-ticket-capability/checklist.md#h-02 | 产品负责人 | Yes |

## Protected acceptance tests

| Path | SHA-256 | Covers |
| --- | --- | --- |
| none | none | 未声明新的受保护执行基线；保留已有有效断言 |

## Requirements-test traceability

| Requirement | Verification | Evidence target | Mode | Blocking |
| --- | --- | --- | --- | --- |
| AC-01 | 完整本地 CI、合并重点测试、提交祖先关系 | acceptance/local-ci-04/summary.json；后续修复使用新目录 | Automatic | Yes |
| AC-02 | 快照检查、全归档审计、文件链接检查 | acceptance/snapshot-check.json；acceptance/archive-audit.json | Automatic | Yes |
| AC-03 | 默认导入设置回归与完整本地 CI | acceptance/ccswitch-local-ci-01/summary.json | Automatic | Yes |
| H-01 | 产品负责人阅读方案并记录意见 | acceptance/human/2026-W39/2026-09-21-upstream-ticket-capability/checklist.md#h-01 | Human | Yes |
| H-02 | 产品负责人查看导入确认页 | acceptance/human/2026-W39/2026-09-21-upstream-ticket-capability/checklist.md#h-02 | Human | Yes |

## Exploratory testing

后续真实账号闭环按方案逐账号、逐模型、逐传输方式执行，本轮不替代。

## Production monitoring and rollback

本轮无发布。后续运行切换必须另行准备监测与回滚记录，不能用本合同推导部署授权。

## Risks and open decisions

打票改造为提议，等待评审；真实票据语义、成本规模和迁移闭环见 openproblem.md。合同保留 DRAFT，不伪造批准或人工 PASS。
