# Acceptance Contract: ldxp-manual-recheck

- Task ID: ldxp-manual-recheck
- Contract version: 1
- Contract status: DRAFT
- Test baseline: PLANNED
- Acceptance owner: 使用者
- Approval evidence: 2026-09-16 当前任务授权本地立即检查功能及管理员可选 Chrome 扩展入口移除；本合同和清单尚待审阅
- Request source: 2026-09-16 当前用户任务
- SSOT node: none
- SSOT path: none
- Readiness mode: FORMAL
- Decision refs: none
- Assumption IDs: none
- Invalidation keys: ldxp.manual-recheck-request, ldxp.manual-recheck-result, ldxp.pause-preservation, ldxp.admin-extension-entry-removal, ldxp.product-mapping-disclosure
- AC budget: 5
- Baseline identity: 本任务本地工作区候选；所验证源码与运行身份由各实际机器执行结果记录
- Product Context refs: agents-results/2026-09-16/ldxp-manual-recheck/ui-context.md
- Role Context refs: agents-results/2026-09-16/ldxp-manual-recheck/ui-context.md
- Resolved Surface Contract refs: agents-results/2026-09-16/ldxp-manual-recheck/ui-context.md
- Screen Contract ref: agents-results/2026-09-16/ldxp-manual-recheck/ui-context.md
- Visual Contract refs: agents-results/2026-09-16/ldxp-manual-recheck/ui-context.md
- UI Change declaration: agents-results/2026-09-16/ldxp-manual-recheck/ui-change.json
- Human acceptance workspace: acceptance/human/2026-W38/2026-09-16-ldxp-manual-recheck

## User and scenario

本地业务管理员完成商户页面验证后，回到本地补货管理页请求立即检查，并观察等待及检查结果。

## Problem

完成验证后的管理员需要明确请求重新检查，区分请求已提交与检查已成功；同时移除管理员页面不再需要的可选 Chrome 扩展入口。

## Expected outcome

点击“我已完成验证，立即检查”后出现等待状态；结果返回后清楚显示成功、失败或仍需暂停。刷新仍可读取结果，失败不擅自恢复补货。管理员页面不显示可选扩展入口及安装下载引导；商品映射位于页面底部，默认折叠，点击后展开。

## Non-goals

不操作生产，不新增购买、退款、补货额度或商品，不重置现有数据。本合同仅覆盖本地交互与检查链路，不以文档或机器通过代替实际人工签署。

## Normal path

Given 本地管理员页面提示需完成商户验证
When 管理员完成验证并点击立即检查
Then 页面先显示等待，再显示实际检查结果，刷新仍可核对结果。

## Exception paths

请求失败、检查失败或本地补货端未处理时，页面不能宣称检查成功。失败保留暂停；人工暂停和不确定上传须继续受保护，不能因手动检查绕过。

## Invariants

请求受管理员权限与设备归属限制；待处理请求不得重复派发；检查结果来自实际执行。凭据、完整兑换内容和敏感原始响应不进入人工材料或普通证据。

## Data impact

新增或更新本地检查请求及结果状态，保留现有商品、库存与批次事实；不通过清除暂停或删除批次制造成功。

## Permissions

请求由获准的本地业务管理员触发；补货端仅处理自身请求。无本任务生产写入授权。

## Performance and reliability

界面等待中应避免重复提交；检查及结果回报故障必须可观察，不承诺固定外部响应时长；执行和结果重试不得造成重复上传。

## Acceptance criteria

| ID | Class | Lane | Requirement | Mode | Blocking |
| --- | --- | --- | --- | --- | --- |
| AC-01 | behavior | machine/integration-contract | 管理员请求立即检查后形成可查询的等待状态；重复点击或待处理请求不并发重复执行，仅授权管理员可请求且目标设备归属正确。 | Automatic | Yes |
| AC-02 | behavior | machine/integration-contract | 本地补货端领取请求后执行当前有效检查并回报结果；失败保留暂停，人工暂停及未决批次保护不能被绕过；结果上报失败可重试且不重复执行副作用。 | Automatic | Yes |
| AC-03 | behavior | machine/e2e | 按钮文案为“我已完成验证，立即检查”；等待、失败、成功或仍暂停状态与实际结果一致，刷新后能读取最新结果，不把请求已发送显示成检查成功。 | Automatic | Yes |
| AC-04 | behavior | machine/e2e | 管理员页面删除可选 Chrome 扩展入口、下载与安装引导；现有商品、库存和补货状态仍可查看，桌面与窄屏操作可达。 | Automatic | Yes |

| AC-05 | behavior | machine/e2e | 商品映射位于管理员页面底部，初次进入与刷新后默认折叠；点击展开可查看并使用映射，折叠不改变已保存商品或补货状态。 | Automatic | Yes |

## Human acceptance

当前全部待验收；清单、合同和绑定为草稿，不生成 handoff 或签署结果。

| ID | Summary | Checklist path | Required role | Blocking |
| --- | --- | --- | --- | --- |
| H-01 | 立即检查请求、等待与刷新后的结果可理解 | acceptance/human/2026-W38/2026-09-16-ldxp-manual-recheck/checklist.md#h-01 | 业务管理员 | Yes |
| H-02 | 检查失败仍暂停且下一步提示清楚 | acceptance/human/2026-W38/2026-09-16-ldxp-manual-recheck/checklist.md#h-02 | 业务管理员 | Yes |
| H-03 | 管理员页面不再出现可选 Chrome 扩展入口 | acceptance/human/2026-W38/2026-09-16-ldxp-manual-recheck/checklist.md#h-03 | 业务管理员 | Yes |

| H-04 | 底部商品映射默认折叠且能点击展开 | acceptance/human/2026-W38/2026-09-16-ldxp-manual-recheck/checklist.md#h-04 | 业务管理员 | Yes |

## Protected acceptance tests

none；草稿不追溯声明测试已锁定。保留有效权限、暂停和幂等断言，不以跳过或弱化测试换取通过。

## Requirements-test traceability

| Requirement | Verification | Evidence target | Mode | Blocking |
| --- | --- | --- | --- | --- |
| AC-01 | 管理员请求立即检查后形成可查询的等待状态；重复点击或待处理请求不并发重复执行，仅授权管理员可请求且目标设备归属正确。 | acceptance/machine/integration-contract/ | Automatic | Yes |
| AC-02 | 本地补货端领取请求后执行当前有效检查并回报结果；失败保留暂停，人工暂停及未决批次保护不能被绕过；结果上报失败可重试且不重复执行副作用。 | acceptance/machine/integration-contract/ | Automatic | Yes |
| AC-03 | 按钮文案为“我已完成验证，立即检查”；等待、失败、成功或仍暂停状态与实际结果一致，刷新后能读取最新结果，不把请求已发送显示成检查成功。 | acceptance/machine/e2e/ | Automatic | Yes |
| AC-04 | 管理员页面删除可选 Chrome 扩展入口、下载与安装引导；现有商品、库存和补货状态仍可查看，桌面与窄屏操作可达。 | acceptance/machine/e2e/ | Automatic | Yes |
| H-01 | 立即检查请求、等待与刷新后的结果可理解的实际观察与双人签署 | acceptance/human/2026-W38/2026-09-16-ldxp-manual-recheck/checklist.md#h-01 | Human | Yes |
| H-02 | 检查失败仍暂停且下一步提示清楚的实际观察与双人签署 | acceptance/human/2026-W38/2026-09-16-ldxp-manual-recheck/checklist.md#h-02 | Human | Yes |
| H-03 | 管理员页面不再出现可选 Chrome 扩展入口的实际观察与双人签署 | acceptance/human/2026-W38/2026-09-16-ldxp-manual-recheck/checklist.md#h-03 | Human | Yes |

| AC-05 | 底部顺序、默认折叠、展开与刷新断言 | acceptance/machine/e2e/ | Automatic | Yes |
| H-04 | 底部商品映射的实际展开观察与双人签署 | acceptance/human/2026-W38/2026-09-16-ldxp-manual-recheck/checklist.md#h-04 | Human | Yes |

## Exploratory testing

观察等待与暂停是否易混淆，长提示、刷新及窄屏是否妨碍管理员判断下一步。

## Production monitoring and rollback

不执行生产操作。本地发现结果误报或暂停保护失效即停止相关检查试验，保留现状和证据后修正；不删除库存、商品或批次。

## Risks and open decisions

本地机器、浏览器及人工观察各自记录实际范围，未执行不能视为通过。缺少安全的等待、失败或成功场景时，对应人工步骤阻塞。合同审批及人工签署尚未完成。
