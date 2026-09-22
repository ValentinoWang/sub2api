# Acceptance Contract: ldxp-verification-alert

- Task ID: ldxp-verification-alert
- Contract version: 1
- Contract status: DRAFT
- Test baseline: PLANNED
- Acceptance owner: 使用者
- Approval evidence: 用户要求小铺验证阻塞在管理员前端可见；未签署发布验收
- Request source: 本轮用户指令
- Readiness mode: FORMAL
- Decision refs: none
- Assumption IDs: none
- Invalidation keys: ldxp.runtime-observation, ldxp.verification-alert
- AC budget: 3
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

管理员希望小铺要求滑块或网页验证时直接在本地补货页面收到明确提示，能打开真实商户页面处理。

## Problem

执行器暂停时不再有商户成功心跳，前端只显示没有近期心跳，无法区分执行器仍在但等待验证。

## Expected outcome

脚本通过独立设备状态接口报告暂停与验证需要，前端显示原因、最近检查、下次检查和固定商户入口；真实恢复后清除提醒。

## Non-goals

不绕过或自动解决验证码，不复制浏览器通行凭证，不部署生产，不把浏览器已登录当作脚本授权通过。

## Normal path

脚本核对完成后上报running；遇验证页停止写入并上报paused和验证标志。管理员页面按既有15秒轮询呈现。

## Exception paths

接口失败不影响现有安全暂停；未知/过期上报不伪造在线；撤销设备无法写；非法字段与秘密内容拒绝。

## Invariants

运行状态是观察信息，不更新商户authorization_verified_at、不清paused_reason、不改变余额、批次、开关或库存准入。

## Data impact

新增设备运行状态表；写入固定枚举、可选时间与服务端记录时间，不写入响应正文、Cookie、Token或兑换码。

## Permissions

复用现有设备认证、限流和管理员状态读取权限。当前验证仅本地8080及4174。

## Performance and reliability

沿用60秒调度和15秒前端刷新。上游异常退避期间仍上报执行器状态；时间过期明确提示。

## Acceptance criteria

| ID | Class | Source requirement refs | Lane | Requirement | Verification layer | Mode | Blocking |
| --- | --- | --- | --- | --- | --- | --- | --- |
| AC-01 | behavior | none | machine/unit | 安全识别验证页通知信号，暂停期间上报，前端显示入口与时间且不伪造恢复 | Unit | Automatic | Yes |
| AC-02 | behavior | none | machine/integration-contract | 认证设备的runtime持久读回，字段/权限/限流严格，状态不影响任何授权和补货权限 | Integration | Automatic | Yes |
| AC-03 | behavior | none | machine/local-runtime | 本机脚本真实验证阻塞经后端传递至管理员界面 | Runtime | Automatic | Yes |

## Human acceptance

none。本次不要求再次购买，不声明生产或正式人工验收。

## Protected acceptance tests

保留已有补货和认证安全断言；新增运行观察测试。

## Requirements-test traceability

| Requirement | Verification | Evidence target | Mode | Blocking |
| --- | --- | --- | --- | --- |
| AC-01 | Python与Vue回归 | acceptance/machine/unit/ | Automatic | Yes |
| AC-02 | 隔离PG与实际路由 | acceptance/machine/integration-contract/ | Automatic | Yes |
| AC-03 | 本机API与浏览器 | acceptance/machine/local-runtime/ | Automatic | Yes |

## Exploratory testing

查看过期、验证需要、一般HTML异常、已恢复、已撤销设备和接口不可用状态。

## Production monitoring and rollback

生产不动。本机更新前保存旧镜像与受限备份；新表不改变既有授权字段；出现异常保留暂停和原批次。

## Risks and open decisions

浏览器通过验证不代表脚本会话获得同样放行；只能以真实读取为准。一般HTML异常不能自动说成验证码，只有已核验的响应标记提示验证需要。
