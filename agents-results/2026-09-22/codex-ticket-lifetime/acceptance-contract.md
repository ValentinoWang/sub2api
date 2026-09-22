# Acceptance Contract: codex-ticket-lifetime

- Task ID: codex-ticket-lifetime
- Contract version: 1
- Contract status: DRAFT
- Test baseline: PLANNED
- Acceptance owner: 产品负责人
- Approval evidence: TBD
- Request source: 2026-09-22 用户要求默认采用 150 秒刷新、210 秒停止注入、240 秒过期，并参考 ranxi2001/sub2api v2.8.0
- SSOT node: none
- SSOT path: none
- Readiness mode: FORMAL
- Decision refs: none
- Assumption IDs: none
- Invalidation keys: codex-ticket-lifetime.age
- AC budget: 2
- Baseline identity: 758cc26bf2704afe0e491940e2f0cf2b7bcb0862
- Product Context refs: none
- Role Context refs: none
- Resolved Surface Contract refs: none
- Screen Contract ref: none
- Visual Contract refs: none
- UI Change declaration: none
- Human acceptance workspace: acceptance/human/2026-W39/2026-09-22-codex-ticket-lifetime

## User and scenario

启用票据功能的管理员需要保守的本地票龄策略。功能默认按新规则运行，不增加模式名称或切换项。

## Problem

原实现将过期与注入合并判断，并默认保留一小时。旧持久化记录也可能继续使用一小时过期时间。

## Expected outcome

以首次本地捕获时间计龄，默认 150 秒进入刷新窗口、210 秒停止注入、240 秒过期。允许更短的既有 TTL 配置，不允许配置延长硬上限；旧记录、内存状态、账号门控和管理端状态遵循相同边界。重复收到同一个已知票据不得延长寿命。

## Non-goals

不迁移 Cookie、固定出口、Mihomo 或整个参考仓库；不发布、不操作生产数据、不进行真实上游打票，不声明票据长度对应模型质量。

## Normal path

后台在刷新窗口内准备替换票；有效且小于 210 秒的票可注入，替换失败时仅在剩余注入窗口继续使用；新票独立计时。

## Exception paths

无票、缺失捕获时间、未来捕获时间、长度不符、注入截止或过期均不可注入。保持缺票拦截开关既有语义。探测失败不延长票龄。已有较短过期时间保持有效。

## Invariants

按账号、模型隔离；不在请求现场打票；关闭功能保持原路径。保留测试和既有源码；不记录真实票据和凭据。

## Data impact

票据仍保存在既有账号元数据中，不新增迁移。本次只修改源码及任务文档；执行测试使用模拟或临时隔离数据。

## Permissions

依据用户明确实现请求修改本地源码；人工签署单独进行，不用自动测试替代。

## Performance and reliability

继续使用现有后台轮询与去重，150 秒表示进入刷新窗口，实际尝试由下一轮调度执行。注入截止在每次现有注入路径调用时检查，不承诺撤销已发出的请求。

## Acceptance criteria

| ID | Class | Source requirement refs | Lane | Requirement | Verification layer | Mode | Blocking |
| --- | --- | --- | --- | --- | --- | --- | --- |
| AC-01 | behavior | 用户生命周期要求 | machine/unit | 149/150、209/210、239/240 边界、旧记录硬上限、刷新失败、重复票不续期以及门控和管理状态一致 | Unit | Automatic | Yes |
| AC-02 | behavior | 用户默认行为要求及项目 CI 规范 | machine/integration-contract | 配置默认 240/90，不增加模式；相关回归和完整本地 CI 通过，保留失败和未执行阶段 | Integration | Automatic | Yes |

## Human acceptance

| ID | Summary | Checklist path | Required role | Blocking |
| --- | --- | --- | --- | --- |
| H-01 | 管理员能区分票据可用和等待补票状态 | acceptance/human/2026-W39/2026-09-22-codex-ticket-lifetime/checklist.md#h-01 | 产品负责人 | Yes |

## Protected acceptance tests

| Path | SHA-256 | Covers |
| --- | --- | --- |
| none | none | 未锁定人工验收基线；保留有效测试断言 |

## Requirements-test traceability

| Requirement | Verification | Evidence target | Mode | Blocking |
| --- | --- | --- | --- | --- |
| AC-01 | 后端定向测试 | acceptance/focused-01/summary.json | Automatic | Yes |
| AC-02 | 完整本地 CI 与设置页测试 | acceptance/local-ci-01/summary.json | Automatic | Yes |
| H-01 | 管理员查看专用演示账号状态 | acceptance/human/2026-W39/2026-09-22-codex-ticket-lifetime/checklist.md#h-01 | Human | Yes |

## Exploratory testing

真实上游票据的寿命、出口绑定、会话连续性和 Cookie 行为需要单独验证；本次源码测试不提供该证据。

## Production monitoring and rollback

本任务不发布生产。后续发布需独立记录候选提交、观察证据和回滚来源。

## Risks and open decisions

本机可用磁盘空间不足可能阻碍完整 CI；失败和未运行检查如实记录。人工验收保持待验收，不构造批准或签字。
