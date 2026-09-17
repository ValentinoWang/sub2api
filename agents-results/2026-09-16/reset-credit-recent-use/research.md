# 重置卡使用历史调研与只读探针

日期：2026-09-16（Asia/Shanghai）。对象为 OpenAI/Codex OAuth 账号的额度重置卡，不是本站余额充值码。

后续更新：14:09 的再次只读查询返回同一条北京时间 00:40:32 的 used 事件，用户确认与实际用卡吻合。累计探针为 11 次 GET、零 consume。最终功能、提醒规则与验证结论见 [功能汇总](feature-summary.md)；下文保留首轮调研与验证的阶段记录。

## 实际发现

官方上游接口 `GET https://chatgpt.com/backend-api/wham/rate-limit-reset-credits/history` 可返回账号级历史，结构为：

```json
{
  "as_of": "上游本次历史快照时间",
  "window_start": "历史覆盖起点",
  "events": [{"id": "仅内部去重使用", "kind": "used", "occurred_at": "实际使用时间"}],
  "next_cursor": null
}
```

真实样本中 `kind` 包含 `used` 和 `granted`。10 号本地账号的历史有 4 条事件，其中 1 条 used：`2026-09-15T16:40:32.907674Z`。查询时上游 `as_of=2026-09-15T17:36:34.759225Z`，对应最近 30 天窗口，`next_cursor=null`。该条用卡发生在查询前约 56 分钟，不属于最近 10 分钟。事件不提供使用者或渠道，不能断言由“其他人”或“其他中转站”操作。

查询的是上游账号历史，因此无需依赖本站本地消费日志；但各凭证权限、上游返回范围与历史功能状态必须分别判断，不把单个账号结果推广为所有账号一定可读。

## 探针过程与证据

- 仅选择已检查的本地 OpenAI OAuth 账号，使用已有 access_token 和 chatgpt_account_id。SQL 只读；认证值只在进程内传递，不写文件、不输出。TLS 使用 curl_cffi 的 Chrome 模拟，复用服务代码中官方请求头语义，20 秒超时，不跟随重定向。
- 10 号账号：现有卡列表和 usage 均为 200。列表字段包含 history_enabled、redeemed_at，但列表实际仅返回 available 卡。剩余数为 2。
- 6 号账号：卡列表 `history_enabled=true`，可用数 2；增加 `include_history=true` 后响应摘要完全相同，不能靠该参数获取历史。
- 同一样本 `total_earned_count=0` 而 `available_count=2`，两字段口径不能按“获得减剩余”等式推导累计使用次数；实现直接统计历史中的 used 事件。
- 6 号账号：独立 `/history` 接口为 200，确认为另一种 events 数据结构。
- 10 号账号：独立 `/history` 为 200，确认 used 事件和真实 occurred_at；只保留脱敏摘要、事件类型和时间，未保存事件 ID、profile_user_id、账号标识、图片或原始响应。
- 11 号账号：列表、history、usage 均为 401。未刷新其凭证或触发消费，作为查询不可用负例保留。
- 所有请求都是 GET，未调用 `/consume`，未购买卡、未触发生成、未重置账号额度。

具体响应见本目录 `probe-account-*.json`。脚本 `probe_upstream.py` 仅允许本次已检查账号；它是任务探针，不是面向其他环境的通用运维工具。

## 原实现缺口

- `openai_quota_reset_credits.go` 只解析可用卡，并过滤非 available 状态；只能得到剩余数和过期时间。
- `OpenAIQuotaResetCredit.redeemed_at` 来自一次消费的响应，不能查询其他入口的消费历史。
- 旧确认弹窗不先查历史；ResetQuota 直接消费。
- 管理端 QueryQuota / RefreshQuota 会调用 NotifyOpenAIAutoResetCredit，因此不能用现有管理查询接口作为保证零消费的探针入口。本次绕过该入口直查官方 GET。

## 实现约定

新增独立管理端 GET `/api/v1/admin/openai/accounts/:id/reset-credit-check`。只查询历史，不通知自动用卡任务、不写展示快照。正常业务运行仍可能通过现有 token provider 刷新认证；这与消耗重置卡不同。

- `recent`：从可信新鲜时间范围中看到距上游 as_of 不足 10 分钟的 used 事件。
- `clear`：完整遍历已返回分页，历史覆盖至少最近 10 分钟，时间有效且没有近期 used。
- `unknown`：认证、网络、格式、覆盖范围、过期快照或分页完整性不能满足判断要求。不可解释为“没用过”。

最多读取 5 页并共用 20 秒超时；页数不足以遍历、游标重复或未知事件类型会降为不可确认。按事件 ID 去重，但 ID 不进入前端响应；返回总使用次数仅指所查询的历史窗口，不是账号终身累计次数。

手动确认显示近期次数、最近使用时间与历史范围。提交时后端重新查询；近期记录改变或状态转为未知时要求新的确认，不自动重试消费。确认摘要不是权限令牌，仅绑定用户看到的观察状态，最终操作仍受管理员认证限制。

自动用卡在实际消费之前检查同一历史，recent 或 unknown 时不消费。现有自动用卡幂等与失败恢复逻辑保留。

跨站操作可能在检查与消费之间同时发生，上游没有提供“检查加消费”的原子接口；本功能是使用前提醒和复查，不宣称全渠道事务互斥。未通过真实消费制造 10 分钟内记录，该边界用确定性测试覆盖。

## 验证结果与运行状态

- 前端 `OpenAIQuotaResetCell.spark_shadow.spec.ts`：21 项通过，包括新增近期提醒、检查失败提示、确认变化不自动重试、切换账号丢弃旧检查，以及既有影子账号、过期卡、缓存与消费后恢复行为。
- 后端 service 与 admin handler 定向测试均通过：真实历史结构解析、窗口边界、未来/陈旧时间、缺字段、分页去重、确认摘要变化、自动用卡 recent/unknown 零消费、手动旧确认零消费、未知历史显式确认、纯查询不进入用卡工作流，以及既有自动幂等、消费后恢复测试。
- 后端命令：`GOGC=50 GOMAXPROCS=2 go test -p 1 -gcflags='github.com/Wei-Shaw/sub2api/internal/service=-l' -tags unit ./internal/service ./internal/handler/admin -run 'Test(AssessResetCreditHistory|ResetCreditHistory|CheckResetCreditHistory|OpenAIResetQuota|OpenAIResetCreditHistoryCheck|OpenAIRefreshQuota|OpenAIQuotaAutoReset)' -count=1`，两个包退出成功。
- 初次默认编译与机器上另一轮完整 CI 竞争，耗时较长；只终止了本次测试进程及其编译子进程，改为串行、较低并发并关闭 service 包内联的定向测试。没有终止另一轮 CI，没有跳过失败测试。上述结果不等于完整本地 CI 或生产编译验收。
- 前端定向 ESLint、gofmt 和 git diff --check 通过。本地 4174 HMR 返回的组件包含 preflight 和历史提醒。
- 真实探针共 10 次 GET，零 consume 调用、零收费生成、零令牌刷新；真实分页样本的 next_cursor 均为空，分页分支以模拟上游测试验证。
- 当前 8080 后端和生产服务均未重启、未替换或部署。这是代码与本地自动测试完成，不宣称新后端接口已在运行服务启用。实际 UI 联调需加载新后端后进行；未执行真实用卡验收。
