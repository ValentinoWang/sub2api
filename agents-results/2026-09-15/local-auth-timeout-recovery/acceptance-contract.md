# Acceptance Contract: local-auth-timeout-recovery

- Task ID: local-auth-timeout-recovery
- Contract version: 1
- Contract status: DRAFT
- Test baseline: PLANNED
- Acceptance owner: 使用者
- Approval evidence: 用户本轮要求复现并尝试修复；未签署发布验收
- Request source: 本轮用户指令
- Readiness mode: FORMAL
- Decision refs: none
- Assumption IDs: none
- Invalidation keys: auth.transport, auth.dependency, redis.deadline, user.activity
- AC budget: 5
- Baseline identity: 当前工作区；执行记录绑定源码摘要
- Product Context refs: none
- Role Context refs: none
- Resolved Surface Contract refs: none
- Screen Contract ref: none
- Visual Contract refs: none
- UI Change declaration: none
- SSOT path: none
- SSOT node: none
- Human acceptance workspace: none

## User and scenario

用户要求复现本机登录和兑换曾遇到的超时，并尝试修复。通过隔离HTTP/Redis及阻塞仓库夹具复现错误处理和等待问题；本地真实服务仅做低频只读和空登录请求检查，不阻塞真实数据库、不重复真实充值、不清库、不操作生产。

## Problem

历史多个模块曾约40秒后失败，缺少当时等待链证据。现有代码把超时和部分依赖错误归为网络或身份失效，需要隔离复现并修复可证实缺陷。

## Expected outcome

错误分类正确，临时依赖失败不清有效会话，辅助操作等待有界，原本有效的认证和兑换安全控制保持成立。

## Non-goals

不宣称重现历史全局停顿来源，不增加自动支付或兑换重试，不修改生产或用户权益，不执行发布。

## Normal path

有效认证继续进入业务处理，Redis正常读写，前端收到成功响应保持原行为。

## Exception paths

超时、依赖不可用、明确认证失效、用户取消分别测试；失败证据保留，不删除失败用例。

## Invariants

真实失效会话仍拒绝；临时故障不伪装用户不存在；任何测试不泄露真实凭据，不重复兑付。

## Data impact

只修改源码与隔离测试。运行诊断不改变真实用户、订单或余额。

## Permissions

沿用用户授权的本地开发和只读检查。真实故障注入只在隔离夹具执行。

## Performance and reliability

使用真实延迟HTTP和伪Redis socket验证截止时间，时间阈值允许调度误差并显著小于未修复的等待上限。

## Acceptance criteria

| ID | Class | Source requirement refs | Lane | Requirement | Verification layer | Mode | Blocking |
| --- | --- | --- | --- | --- | --- | --- | --- |
| AC-01 | behavior | none | machine/unit | HTTP响应超时与无法连接分别报告；不自动重发兑换请求；临时刷新登录失败保留凭据，明确失效仍退出 | Unit | Automatic | Yes |
| AC-02 | behavior | none | machine/unit | JWT依赖失败返回503；真实不存在或被撤销仍拒绝 | Unit | Automatic | Yes |
| AC-03 | behavior | none | machine/unit | Redis遵守已有context截止时间 | Unit | Automatic | Yes |
| AC-04 | behavior | none | machine/unit | 活跃时间辅助更新等待有界；取消等待者不取消另一正常请求 | Unit | Automatic | Yes |
| AC-05 | behavior | none | machine/local-runtime | 历史调查、当前只读探针和实际生效范围可核对 | Runtime | Automatic | Yes |

## Human acceptance

none。此次为开发排障和自动回归，不新增付费购买或人工签署，也不据此宣布全站或生产验收通过。

## Protected acceptance tests

none。此次补充回归，保留有效的既有安全断言。

## Requirements-test traceability

| Requirement | Verification | Evidence target | Mode | Blocking |
| --- | --- | --- | --- | --- |
| AC-01 | 前端client和tokenRefresh回归 | acceptance/machine/unit/ | Automatic | Yes |
| AC-02 | JWT中间件依赖异常回归 | acceptance/machine/unit/ | Automatic | Yes |
| AC-03 | 隔离Redis socket deadline | acceptance/machine/unit/ | Automatic | Yes |
| AC-04 | UserService和JWT阻塞夹具 | acceptance/machine/unit/ | Automatic | Yes |
| AC-05 | 当前只读运行检查 | acceptance/machine/local-runtime/ | Automatic | Yes |

## Exploratory testing

检查当前4174代理、8080后端和自动补货并行时的低频响应；不做真实压测。

## Production monitoring and rollback

本任务不部署生产或替换本地后端容器。源码可按具体文件回退；不通过数据修改处理故障。

## Risks and open decisions

隔离故障注入可证明实现缺陷和修复效果，但不能单独证明历史22:54全局延迟的来源。完整本地CI和后端容器替换另行记录，不用局部测试代替发布结论。
