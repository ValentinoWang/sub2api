# Acceptance Contract: ldxp-http-restock-local

- Task ID: ldxp-http-restock-local
- Contract version: 1
- Contract status: DRAFT
- Test baseline: PLANNED
- Acceptance owner: 使用者
- Approval evidence: 2026-09-15 用户要求在本机实现链动小铺纯 HTTP 补货闭环；本合同和人工清单尚待审阅
- Request source: 2026-09-15 用户请求
- SSOT node: none
- SSOT path: none
- Readiness mode: FORMAL
- Decision refs: none
- Assumption IDs: none
- Invalidation keys: ldxp.http-restock-binding, ldxp.http-restock-upload-verification, ldxp.http-restock-recovery, ldxp.http-restock-maintenance
- AC budget: 5
- Baseline identity: 2026-09-15 本机 HTTP 补货请求；执行时另行记录所验证源码与本地运行身份
- Product Context refs: none
- Role Context refs: none
- Resolved Surface Contract refs: none
- Screen Contract ref: none
- Visual Contract refs: none
- UI Change declaration: none
- Human acceptance workspace: acceptance/human/2026-W38/2026-09-15-ldxp-http-restock-local

## User and scenario

业务管理员复用本机 Sub2API 已配置的五个链动小铺测试商品与批次，通过本机 HTTP 工具维护卡密库存。测试购买者从对应商品购买并在本机 Sub2API 兑换，分别核对每个商品的到账权益。

## Problem

需要把已配置商品的库存发现、补货上传、上传确认和中断恢复连成可持续运行的本机流程。上传响应丢失不能被误判为未上传，也不能因此重复发出同一批兑换码。

## Expected outcome

工具自动绑定全部已配置测试商品，默认目标库存为 999，按缺口和小批量上限补货；真实上传后逐码读回核验，确认成功后才向本机批次记录报告上传完成。丢响应或重启后的未决批次先核对已有库存，禁止重新上传同批码；运行期间每 60 秒维护一次。五个商品分别保留验证记录，不能用其中一个商品替代其余商品。

## Non-goals

不改变远端 Sub2API 生产环境，不重建本机后端，不清空现有账号、商品或批次数据，不新增商品或调整商品价格与到账权益，不验收退款、提现、结算及原生支付接入。本合同不把长期常驻、人工购买完成或外部服务持续可用视为已经实现的事实。

## Normal path

```gherkin
Given 本机已配置五个获准测试的商品，管理员具备有效的链动小铺访问权限
When HTTP 工具读取库存并为缺货商品执行小批量补货
Then 每个批次只有在商户库存逐码核对通过后才被确认为上传完成，后续维护继续按该商品目标库存执行
```

## Exception paths

- 未登录、权限不足或登录失效时停止相关商户写入，给出可理解的失败原因，不能把掉线当作零库存。
- 上传超时、丢失响应、返回异常或工具重启后，已有未决批次只能读回核对，不能自动重传；确认不足时保持未决或阻塞状态。
- 商品映射缺失、读取不完整、逐码缺失或发现不一致时，不能提前标记上传成功，也不能为同一未决缺口不断产生新批次。
- 库存达到目标时不补货；不足时按实际缺口和批量上限生成本轮需要的数量。

## Invariants

Sub2API 仍是兑换权益的唯一账本；一张兑换码只能成功兑换一次。商品身份、批次归属和码集合保持一致；未完成逐码确认的上传不得被声明成功。商户访问凭据、完整兑换码和用户敏感资料不得进入共享报告或人工验收材料。本机读写目标不能悄然切换到远端生产环境。

## Data impact

会更新本机已配置商品的绑定与库存目标、创建或恢复本机补货批次，并将获准的小批量兑换码写入商户测试商品库存。人工购买与兑换会产生对应测试订单和余额变动。上传前记录恢复所需状态，保留未决状态供核对；不得以删除批次或清空库存作为重试方式。本任务不自动清理已售出或已兑换数据。

## Permissions

本机商品和批次操作使用既有管理权限；商户库存操作限用户授权的测试商品。测试购买者只使用自己的测试账号购买和兑换。工具使用既有授权方式获取所需访问权限，验收材料不要求使用者提交或公开凭据。

## Performance and reliability

维护间隔默认为 60 秒；每轮受限于库存缺口及配置的小批量上限。网络请求必须有超时边界；超时或重启后保留可核对状态。恢复逻辑不得依靠内存中的一次成功响应判断是否可以再次上传。持续运行的观察时长与实际完成轮次在机器证据中如实记录，有限窗口不能证明永久可用。

## Acceptance criteria

| ID | Class | Source requirement refs | Lane | Requirement | Verification layer | Mode | Blocking |
| --- | --- | --- | --- | --- | --- | --- | --- |
| AC-01 | behavior | none | machine/unit | 自动绑定全部本机已配置测试商品；默认目标为 999；按目标缺口与批量上限补货，达到目标不上传，不添加未配置商品 | Unit | Automatic | Yes |
| AC-02 | behavior | none | machine/integration-contract | HTTP 上传后逐码读回核验商品与码集合；只有完整确认才回报批次上传完成，缺码、错配和异常响应不能误报成功 | Integration | Automatic | Yes |
| AC-03 | behavior | none | machine/integration-contract | 上传已生效但响应丢失、确认失败或工具重启时，未决批次只进行读回核对，不重传同批码或为该未决缺口重复生成批次 | Integration | Automatic | Yes |
| AC-04 | behavior | none | machine/unit | 运行期间默认每 60 秒维护；请求超时、读取失败或登录失效不得当作零库存继续写入，报告不泄露访问凭据或完整兑换码 | Unit | Automatic | Yes |
| AC-05 | behavior | none | machine/local-runtime | 五个既有测试商品分别完成受控小批量真实 HTTP 上传、逐码库存读回及本机批次确认，并记录多轮维护和恢复观察的实际边界 | Runtime | Automatic | Yes |

## Human acceptance

详细业务步骤与实际签署结果位于下列人工工作区。当前仅准备草稿，尚未执行人工验收，不存在人工通过结论。

| ID | Summary | Checklist path | Required role | Blocking |
| --- | --- | --- | --- | --- |
| H-01 | 管理员理解并核对五个测试商品的权益、库存和补货状态 | acceptance/human/2026-W38/2026-09-15-ldxp-http-restock-local/checklist.md#h-01 | 业务管理员 | Yes |
| H-02 | 购买者逐商品完成购买、收码、本机兑换和重复兑换核对 | acceptance/human/2026-W38/2026-09-15-ldxp-http-restock-local/checklist.md#h-02 | 测试购买者 | Yes |
| H-03 | 管理员在掉线及恢复后理解提示并确认库存状态可继续核对 | acceptance/human/2026-W38/2026-09-15-ldxp-http-restock-local/checklist.md#h-03 | 业务管理员 | Yes |

## Protected acceptance tests

none；合同为草稿，测试基线为 PLANNED。测试路径与不可变执行证据在实际验证后记录，不预先宣称已通过。

## Requirements-test traceability

| Requirement | Verification | Evidence target | Mode | Blocking |
| --- | --- | --- | --- | --- |
| AC-01 | 全商品绑定、库存边界及批量上限自动检查 | acceptance/machine/unit/ | Automatic | Yes |
| AC-02 | 上传与逐码读回正常、缺失及错配用例 | acceptance/machine/integration-contract/ | Automatic | Yes |
| AC-03 | 丢响应、未决批次与重启恢复用例 | acceptance/machine/integration-contract/ | Automatic | Yes |
| AC-04 | 维护间隔、访问失效、超时及敏感输出用例 | acceptance/machine/unit/ | Automatic | Yes |
| AC-05 | 五个商品分别保留真实上传与读回、批次确认和维护观察 | acceptance/machine/local-runtime/ | Automatic | Yes |
| H-01 | 逐商品库存与权益业务核对 | acceptance/human/2026-W38/2026-09-15-ldxp-http-restock-local/checklist.md#h-01 | Human | Yes |
| H-02 | 逐商品购买和兑换业务闭环 | acceptance/human/2026-W38/2026-09-15-ldxp-http-restock-local/checklist.md#h-02 | Human | Yes |
| H-03 | 掉线提示和恢复后库存核对 | acceptance/human/2026-W38/2026-09-15-ldxp-http-restock-local/checklist.md#h-03 | Human | Yes |

## Exploratory testing

观察商品名称和额度单位是否易混淆；关注购买与补货交错时库存说明是否清楚，以及短暂掉线后使用者能否理解继续操作的条件。人工只记录可观察业务结果，精确批次一致性由机器证据负责。

## Production monitoring and rollback

本任务不执行生产部署。发现未授权商品被写入、库存持续差异或登录失效时停止本机补货，保留已有批次并先核对商户库存；不删除已上传兑换码、不自动重传未决批次、不回退已消费权益。后续处理由业务负责人基于实际库存与订单决定。

## Risks and open decisions

商户会话有效性、库存分页完整性、实际购买与发码均需当前运行证据。目标 999 是默认维护配置，不代表已上传 999 张或库存已达标。真实购买及各商品兑换尚待指定人员执行；机器、持续运行观察和人工签署均以各自实际结果为准。合同和清单仍为草稿，不能用于生成机器全绿或人工通过声明。
