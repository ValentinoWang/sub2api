# Acceptance Contract: ldxp-browser-session

- Task ID: ldxp-browser-session
- Contract version: 1
- Contract status: DRAFT
- Test baseline: PLANNED
- Acceptance owner: 使用者
- Approval evidence: 2026-09-16 当前用户授权本地持久专用浏览器与同会话人工验证方案；合同及清单尚待审阅
- Request source: 2026-09-16 当前用户任务
- SSOT node: none
- SSOT path: none
- Readiness mode: FORMAL
- Decision refs: none
- Assumption IDs: none
- Invalidation keys: ldxp.dedicated-browser-session, ldxp.local-window-control, ldxp.same-session-verification, ldxp.pause-preservation
- AC budget: 4
- Baseline identity: 本任务本地实现候选；最终源码及运行身份由实际机器结果记录，本草稿不声明已实测通过
- Product Context refs: agents-results/2026-09-16/ldxp-browser-session/ui-context.md
- Role Context refs: agents-results/2026-09-16/ldxp-browser-session/ui-context.md
- Resolved Surface Contract refs: agents-results/2026-09-16/ldxp-browser-session/ui-context.md
- Screen Contract ref: agents-results/2026-09-16/ldxp-browser-session/ui-context.md
- Visual Contract refs: agents-results/2026-09-16/ldxp-browser-session/ui-context.md
- UI Change declaration: agents-results/2026-09-16/ldxp-browser-session/ui-change.json
- Human acceptance workspace: acceptance/human/2026-W38/2026-09-16-ldxp-browser-session

## User and scenario

业务管理员从本地补货管理页进入本机助手，打开补货专用窗口，亲自在该窗口登录商户并完成人工验证，再返回管理页请求检查。

## Problem

人工验证必须发生在实际补货使用的浏览器会话内，否则普通浏览器中验证成功不能证明执行器获得有效授权。

## Expected outcome

专用窗口承载人工登录、验证和后续补货。管理员能理解打开窗口、验证、返回检查与等待结果的步骤；只有授权及全部库存核对通过才恢复既有 999 库存目标。

## Non-goals

仅本地，不操作生产，不新增商品、购买或退款，不改变库存目标、价格及到账规则；不要求用户发送密码或卡密。本文不代替实际商户验证，也不创建人工通过或入队声明。

## Normal path

Given 本地补货需要人工验证
When 管理员打开补货专用窗口，亲自登录并完成验证，再回管理页点击立即检查
Then 在同一会话完成授权与全部库存核对后才恢复，界面显示实际结果。

## Exception paths

专用窗口未打开、登录或验证未完成、检查失败、库存未知及人工暂停时不得强制恢复。仅打开窗口或提交检查不代表成功；未决上传继续核对原批，不能重传。

## Invariants

人工验证和补货共用专用持久会话；普通浏览器登录结果不能替代。实际恢复必须满足授权及全部库存核对。真实凭据、卡密和敏感会话资料不进入普通报告或人工记录。

## Data impact

本地保存专用浏览器会话及检查状态，复用已有本地商品、库存和批次；不清理用户数据，不以重置暂停或删除未决批次处理错误。

## Permissions

本地业务管理员亲自完成商户登录及人工验证，助手只提供打开受控窗口的操作。既有授权商品与本地补货范围保持不变，无生产操作。

## Performance and reliability

不承诺固定外部验证时长；等待和失败须可见。持久会话不等于永久登录有效，恢复仍需当次授权与完整库存核对。

## Acceptance criteria

| ID | Class | Lane | Requirement | Mode | Blocking |
| --- | --- | --- | --- | --- | --- |
| AC-01 | behavior | machine/integration-contract | 本地助手 http://127.0.0.1:52401/ 的“打开补货专用窗口”只唤起 Playwright 持久专用浏览器的真实执行器窗口；人工登录、验证与后续补货共用该会话，不能以普通浏览器页面替代。 | Automatic | Yes |
| AC-02 | behavior | machine/integration-contract | 管理员立即检查请求触发同会话授权及全部已配置商品库存核对，全部通过后才恢复既有 999 库存目标；请求已发送、仅登录或部分商品成功不得标记恢复。 | Automatic | Yes |
| AC-03 | behavior | machine/integration-contract | 检查失败、库存未知、未决批次及人工暂停保持安全约束；不得强制恢复、重复上传或清空状态；本机助手仅服务本地受控窗口操作，不索取或暴露密码、卡密。 | Automatic | Yes |
| AC-04 | behavior | machine/e2e | 本地管理页可进入助手并返回点击“我已完成验证，立即检查”，准确呈现等待和实际结果，刷新不误报成功；商品映射保留页面底部默认折叠、点击展开。 | Automatic | Yes |

## Human acceptance

当前实际商户登录及人工验证尚待人员执行，全部待验收。合同与绑定为 DRAFT，不生成 handoff 或人工 PASS。

| ID | Summary | Checklist path | Required role | Blocking |
| --- | --- | --- | --- | --- |
| H-01 | 打开专用窗口、亲自验证并返回检查的流程清楚 | acceptance/human/2026-W38/2026-09-16-ldxp-browser-session/checklist.md#h-01 | 业务管理员 | Yes |
| H-02 | 失败、未知或人工暂停不会被误报恢复 | acceptance/human/2026-W38/2026-09-16-ldxp-browser-session/checklist.md#h-02 | 业务管理员 | Yes |
| H-03 | 刷新结果与底部商品映射仍可正常查看 | acceptance/human/2026-W38/2026-09-16-ldxp-browser-session/checklist.md#h-03 | 业务管理员 | Yes |

## Protected acceptance tests

none；本草稿不追溯锁定测试。保留同会话、权限、完整核对、暂停及未决上传的有效安全断言。

## Requirements-test traceability

| Requirement | Verification | Evidence target | Mode | Blocking |
| --- | --- | --- | --- | --- |
| AC-01 | 本地助手 http://127.0.0.1:52401/ 的“打开补货专用窗口”只唤起 Playwright 持久专用浏览器的真实执行器窗口；人工登录、验证与后续补货共用该会话，不能以普通浏览器页面替代。 | acceptance/machine/integration-contract/ | Automatic | Yes |
| AC-02 | 管理员立即检查请求触发同会话授权及全部已配置商品库存核对，全部通过后才恢复既有 999 库存目标；请求已发送、仅登录或部分商品成功不得标记恢复。 | acceptance/machine/integration-contract/ | Automatic | Yes |
| AC-03 | 检查失败、库存未知、未决批次及人工暂停保持安全约束；不得强制恢复、重复上传或清空状态；本机助手仅服务本地受控窗口操作，不索取或暴露密码、卡密。 | acceptance/machine/integration-contract/ | Automatic | Yes |
| AC-04 | 本地管理页可进入助手并返回点击“我已完成验证，立即检查”，准确呈现等待和实际结果，刷新不误报成功；商品映射保留页面底部默认折叠、点击展开。 | acceptance/machine/e2e/ | Automatic | Yes |
| H-01 | 打开专用窗口、亲自验证并返回检查的流程清楚的实际观察与双人签署 | acceptance/human/2026-W38/2026-09-16-ldxp-browser-session/checklist.md#h-01 | Human | Yes |
| H-02 | 失败、未知或人工暂停不会被误报恢复的实际观察与双人签署 | acceptance/human/2026-W38/2026-09-16-ldxp-browser-session/checklist.md#h-02 | Human | Yes |
| H-03 | 刷新结果与底部商品映射仍可正常查看的实际观察与双人签署 | acceptance/human/2026-W38/2026-09-16-ldxp-browser-session/checklist.md#h-03 | Human | Yes |

## Exploratory testing

观察管理员能否明确区分专用窗口与普通浏览器，能否顺利返回管理页，是否把请求已发误认为已经恢复。

## Production monitoring and rollback

本任务不执行生产操作。本地出现错误恢复、会话不一致或库存不明时暂停相关补货并保留证据及原批状态；不能删除商品、库存或已发权益。

## Risks and open decisions

本草稿未执行实际商户登录及人工验证，模拟与组件测试不能替代此项。机器结果以主任务实测为准；缺少可用专用窗口或安全失败场景时对应人工步骤阻塞，状态不升级。
