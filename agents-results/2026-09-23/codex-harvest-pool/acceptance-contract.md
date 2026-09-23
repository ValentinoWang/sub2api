# Acceptance Contract: codex-harvest-pool

- Task ID: codex-harvest-pool
- Contract version: 1
- Contract status: DRAFT
- Test baseline: PLANNED
- Acceptance owner: 产品负责人
- Approval evidence: TBD
- Request source: 2026-09-23 用户要求参考 ranxi2001/sub2api v2.8.0 与 Codex_degrade 的打票代理管理，合并为 0.2.7.2 生产版本，不做回滚与兼容版本
- SSOT node: none
- SSOT path: none
- Readiness mode: FORMAL
- Decision refs: none
- Assumption IDs: none
- Invalidation keys: codex-harvest-pool.controls, codex-harvest-pool.pool
- AC budget: 4
- Baseline identity: 4afaae5ae
- Product Context refs: agents-results/2026-09-23/codex-harvest-pool/ui-context.md
- Role Context refs: agents-results/2026-09-23/codex-harvest-pool/ui-context.md
- Resolved Surface Contract refs: agents-results/2026-09-23/codex-harvest-pool/ui-context.md
- Screen Contract ref: agents-results/2026-09-23/codex-harvest-pool/ui-context.md
- Visual Contract refs: agents-results/2026-09-23/codex-harvest-pool/ui-context.md
- UI Change declaration: agents-results/2026-09-23/codex-harvest-pool/ui-change.json
- Human acceptance workspace: acceptance/human/2026-W39/2026-09-23-codex-harvest-pool

## User and scenario

管理员在代理管理中导入节点后，把其中一组代理设为打票代理池，选择速度预设，并在一个页面查看各账号票据、代理成绩和打票流水，必要时对单个账号手动打票。

## Problem

打票只能使用一个手填的代理地址，填写后无法在后台清空；后台按账号和模型全部并发打票，没有请求预算、失败冷却和按账号限额，也看不到哪个代理出过票。

## Expected outcome

- 打票代理池只来自代理管理中的代理，按账号、模型记录每个代理的成功、未命中、网络错误和账号错误，并按近期成绩选择代理；同一账号优先使用上次出票的代理。
- 每轮请求预算、请求间隔、单次超时、失败冷却、每账号每轮换代理次数与每账号每小时请求上限可由预设或手工设置；换代理不恢复额度；429 按 Retry-After 冷却，401/403/429 本轮停止该账号。
- 采票响应只有在 HTTP 200、长度与前缀合格且流式响应以 response.completed 结束时才入库。
- 打票流水保留最近 200 条并在重启后恢复；管理员可对单个账号发起有上限的手动打票。
- 移除单一打票代理地址设置及 yaml 字段，不保留兼容路径；业务请求出口、会话与身份字段保持现状。

## Non-goals

不移植 v2.8.0 的自管 Mihomo 进程、Selector 定向切换、业务出口与会话绑定、身份字段改写、Cookie 注入和备用票；不改票龄规则（150/210/240 秒）；不声明票据长度对应模型质量；不在生产导入代理或改动账号。

## Normal path

后台每轮按“可调度账号优先、游标轮转”遍历需要补票的账号与模型；对每一对在池中按成绩选代理，受预算、间隔和每小时上限约束发出采票请求；合格即入库并记录流水与代理成绩；不合格则换代理直到达到换代理次数，最后进入冷却。

## Exception paths

未开启打票、代理池为空或池中代理全部停用、过期或删除时不发请求并在页面说明原因；预算或每小时上限用尽时本轮跳过；网络错误、上游错误、非 completed 响应、长度或前缀不符、取消均不入库；学习记录写入失败不影响已合格门票。

## Invariants

按账号、模型隔离；业务请求不打票、不换出口；fail_closed 语义不变；关闭打票保持原转发路径；页面与接口不显示票据原文和代理密码；保留全部既有测试断言的意图并按新模型改写。

## Data impact

新增迁移 245（代理学习记录与学习代数）与 246（打票流水，保留 200 条）；删除已停用的设置键 openai_codex_ticket_harvest_proxy_url。门票仍存账号元数据，增加出票代理 ID 与名称用于展示和粘滞选择。

## Permissions

新增接口仅管理员可用；手动打票需要管理员显式发起并受每小时上限约束。

## Performance and reliability

打票串行执行并受每轮预算约束，不再按账号与模型全量并发；每小时上限只在内存中计数，重启后重新计数；流水写库超时 1.5 秒且失败不阻塞打票。

## Acceptance criteria

| ID | Class | Source requirement refs | Lane | Requirement | Verification layer | Mode | Blocking |
| --- | --- | --- | --- | --- | --- | --- | --- |
| AC-01 | behavior | 用户代理池与限额要求 | machine/unit | 代理池解析、预设与边界校验、排序、粘滞、预算、每小时上限、冷却、Retry-After、响应校验与入库条件 | Unit | Automatic | Yes |
| AC-02 | behavior | 用户流水与学习要求 | machine/integration-contract | 迁移 245/246、学习记录代数与清理、流水持久化与恢复、后台接口权限与参数校验 | Integration | Automatic | Yes |
| AC-03 | behavior | 用户页面要求 | machine/unit | 打票管理页面加载、保存、手动打票与设置页移除代理地址 | Unit | Automatic | Yes |
| AC-04 | behavior | 项目 CI 规范 | machine/integration-contract | 完整本地 CI 结果如实记录，失败与未执行阶段保留 | Integration | Automatic | Yes |

## Human acceptance

| ID | Summary | Checklist path | Required role | Blocking |
| --- | --- | --- | --- | --- |
| H-01 | 管理员能设置打票代理池与速度并看懂打票结果 | acceptance/human/2026-W39/2026-09-23-codex-harvest-pool/checklist.md#h-01 | 产品负责人 | Yes |

## Protected acceptance tests

| Path | SHA-256 | Covers |
| --- | --- | --- |
| none | none | 未锁定人工验收基线；保留有效测试断言 |

## Requirements-test traceability

| Requirement | Verification | Evidence target | Mode | Blocking |
| --- | --- | --- | --- | --- |
| AC-01 | 后端定向测试 | acceptance/focused-01/summary.json | Automatic | Yes |
| AC-02 | 仓储集成测试与接口测试 | acceptance/local-ci-01/summary.json | Automatic | Yes |
| AC-03 | 前端页面测试 | acceptance/local-ci-01/summary.json | Automatic | Yes |
| AC-04 | 完整本地 CI | acceptance/local-ci-01/summary.json | Automatic | Yes |
| H-01 | 管理员在演示环境设置代理池并查看结果 | acceptance/human/2026-W39/2026-09-23-codex-harvest-pool/checklist.md#h-01 | Human | Yes |

## Exploratory testing

真实上游能否用所选代理稳定拿到合格票据、票据是否与出口 IP 绑定，本次源码测试不提供证据。

## Production monitoring and rollback

按用户要求发布 0.2.7.2 生产版本，不保留回滚镜像，迁移前备份数据库；发布后观察打票流水、代理成绩和账号票据状态。

## Risks and open decisions

生产当前没有可用的 OpenAI OAuth 账号与住宅代理，打票功能保持关闭时无法在生产验证出票；人工验收保持待验收，不构造批准或签字。
