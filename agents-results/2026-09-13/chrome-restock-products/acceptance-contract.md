# Acceptance Contract: chrome-restock-products

- Task ID: chrome-restock-products
- Contract version: 1
- Contract status: DRAFT
- Test baseline: PLANNED
- Acceptance owner: 使用者
- Approval evidence: 当前任务已授权实现与并行工作；本合同与人工清单为待审阅草稿，不构成人工签署或新增运营授权
- Request source: 2026-09-13 当前用户任务
- SSOT node: none
- SSOT path: none
- Readiness mode: FORMAL
- Decision refs: none
- Assumption IDs: none
- Invalidation keys: commerce.amount-mapping, commerce.environment-isolation, chrome.device-scope, chrome.inventory-identity, chrome.batch-recovery, chrome.manual-resume, purchase.compatibility
- AC budget: 9
- Baseline identity: 本任务当前实现候选；最终源码与运行身份由对应机器结果绑定，本草稿不声明测试已通过
- Product Context refs: agents-results/2026-09-13/chrome-restock-products/ui-context.md
- Role Context refs: agents-results/2026-09-13/chrome-restock-products/ui-context.md
- Resolved Surface Contract refs: agents-results/2026-09-13/chrome-restock-products/ui-context.md
- Screen Contract ref: agents-results/2026-09-13/chrome-restock-products/ui-context.md
- Visual Contract refs: agents-results/2026-09-13/chrome-restock-products/ui-context.md
- UI Change declaration: agents-results/2026-09-13/chrome-restock-products/ui-change.json
- Human acceptance workspace: acceptance/human/2026-W37/2026-09-13-chrome-restock-products

## User and scenario

购买者从额度购买页选面额、购买对应小铺商品，再返回购买页兑换。业务管理员在 Chrome 已登录商户的条件下管理各授权商品的少量补货和异常核对。测试与正式环境分别执行。

## Problem

多面额必须避免商品跳转、到账金额与码库混淆；商户登录失效或上传结果不明时，自动补货不能盲目重发，亦不能因补货暂停而误关销售入口。

## Expected outcome

人民币 5、10、20、50、100 元分别购买美元余额 5、10、20、50、100 元，商品显示“X元额度”。购买后返回 purchase 兑换且只到账一次。自动补货凭逐码核验推进，同批重试不重发，暂停原因和恢复动作可理解。

## Non-goals

不新增支付渠道，不验收退款、提现或会员商品，不把历史 5 元真实购买认定为本任务签署。本文不增加真实交易、上架或补货授权，也不创建 SSOT、openproblem 或 Obsidian 快照。

## Normal path

Given 测试与正式商品及码库独立且逐个面额已准备
When 管理员在已登录商户的 Chrome 核验授权商品并手动开始获准补货，购买者购买对应面额后返回购买页兑换
Then 补货完成须有逐码核验确认，购买者只得到该面额的一次美元余额，另一环境不受影响。

## Exception paths

空库存、未知库存、上传结果不明、登录失效和离线要明确区分；查询失败不能显示为零或成功。中断后保留原批，不重发或新领；登录恢复后仍须核验与手动恢复。重复兑换、错误环境和无权限设备拒绝产生权益或补货。

## Invariants

测试和生产不共用商品或码库；人民币售价与美元余额分别标币种。销售启停与补货启停独立。merchantToken 只用于商户页，本站仅使用 restricted 设备凭据；原码、登录材料和设备凭据不进入普通证据或人工步骤。数量相同不是逐码核验成功。

## Data impact

商品映射、设备授权、补货批次与兑换记录须持久保存并有环境归属。不得删除批次或重置码库来解决不确定结果。实际交易和商户库存写入仅在当前授权内执行；故障实验使用隔离数据。

## Permissions

购买者只操作获准账户和商品；管理员配置授权商品、撤销设备并处理暂停。扩展仅用受限设备身份访问本站，不能替代管理员权限。人工通过不自动授权开售、支付或额外真实库存写入。

## Performance and reliability

不新增延迟或吞吐承诺。要求单 worker 互斥、持久批次、上传前写入不确定状态、完成后逐码核验及故障后人工恢复；不得无限重试外部写入。

## Acceptance criteria

| ID | Class | Lane | Requirement | Mode | Blocking |
| --- | --- | --- | --- | --- | --- |
| AC-01 | behavior | machine/integration-contract | 五面额 5、10、20、50、100 的人民币售价分别对应等额美元余额，商品名为“X元额度”；测试与生产商品和码库独立，错误归属不可兑换 | Automatic | Yes |
| AC-02 | behavior | machine/non-functional | 设备凭据仅允许授权商品的补货操作，权限拒绝与撤销立即生效；商户登录材料不离开商户标签页，不发送给 Sub2API 或写入扩展持久存储 | Automatic | Yes |
| AC-03 | behavior | machine/integration-contract | 完整库存双扫描及逐码 SHA-256 核验，不能仅以数量相等放行；重复摘要、缺项、部分上传和响应不明均拒绝继续追加 | Automatic | Yes |
| AC-04 | behavior | machine/integration-contract | 开始上传前持久化同批身份；超时、重启和响应丢失后只核验原批，不重传、不另领新批；已确认完成的原批可幂等恢复 | Automatic | Yes |
| AC-05 | behavior | machine/integration-contract | 离线、登录失效、浏览器重启或不确定结果后暂停；重新登录或单独检查不会恢复自动补货，核验后仍须手动开始；销售入口和自动补货启停独立 | Automatic | Yes |
| AC-06 | behavior | machine/e2e | 前台各面额跳转对应外部商品并返回 purchase 兑换；金额准确且重复兑换不再次到账；兑换中心与原生在线支付保留既有入口和行为 | Automatic | Yes |
| AC-07 | behavior | visual-fidelity | 购买页面桌面/手机及 Chrome 管理页在正常、空、加载、错误、暂停和未保存状态均可读可操作；Figma 同步范围与未核对状态逐项记录 | Automatic | Yes |
| AC-08 | process-provenance | machine/static | 完整本地 CI 按明确候选身份执行全部必需阶段，保留失败与未执行事实；通过不得代替真实 Chrome 或人工签署 | Automatic | Yes |
| AC-09 | behavior | external-sandbox | 真实 Chrome 使用已登录商户完成授权商品的库存核验与获准少量补货，保存脱敏结果；测试与正式环境、各面额分开记录，不能由模拟测试或历史 5 元购买代替 | Automatic | Yes |

## Human acceptance

五面额在测试与正式环境各自签署，当前全部待验收。机器、真实 Chrome 验证仍由各执行结果报告，本草稿不声明 PASS，也不生成 handoff。

| ID | Summary | Checklist path | Required role | Blocking |
| --- | --- | --- | --- | --- |
| H-01 | 5元额度购买、发货、返回兑换与重复确认 | acceptance/human/2026-W37/2026-09-13-chrome-restock-products/checklist.md#h-01 | 额度商品验收购买者 | Yes |
| H-02 | 10元额度购买、发货、返回兑换与重复确认 | acceptance/human/2026-W37/2026-09-13-chrome-restock-products/checklist.md#h-02 | 额度商品验收购买者 | Yes |
| H-03 | 20元额度购买、发货、返回兑换与重复确认 | acceptance/human/2026-W37/2026-09-13-chrome-restock-products/checklist.md#h-03 | 额度商品验收购买者 | Yes |
| H-04 | 50元额度购买、发货、返回兑换与重复确认 | acceptance/human/2026-W37/2026-09-13-chrome-restock-products/checklist.md#h-04 | 额度商品验收购买者 | Yes |
| H-05 | 100元额度购买、发货、返回兑换与重复确认 | acceptance/human/2026-W37/2026-09-13-chrome-restock-products/checklist.md#h-05 | 额度商品验收购买者 | Yes |
| H-06 | 管理员能完成少量补货并理解库存与商品归属 | acceptance/human/2026-W37/2026-09-13-chrome-restock-products/checklist.md#h-06 | 业务管理员 | Yes |
| H-07 | 中断与结果不明时能识别暂停，核对后手动恢复 | acceptance/human/2026-W37/2026-09-13-chrome-restock-products/checklist.md#h-07 | 业务管理员 | Yes |
| H-08 | 销售入口独立、兑换中心与在线支付兼容且窄屏可用 | acceptance/human/2026-W37/2026-09-13-chrome-restock-products/checklist.md#h-08 | 业务管理员、额度商品验收购买者 | Yes |

## Protected acceptance tests

none；现有与新增测试由实际源码追溯，本草稿不追溯声明测试已批准锁定，不允许弱化或跳过测试换取通过。

## Requirements-test traceability

| Requirement | Verification | Evidence target | Mode | Blocking |
| --- | --- | --- | --- | --- |
| AC-01 | 五面额 5、10、20、50、100 的人民币售价分别对应等额美元余额，商品名为“X元额度”；测试与生产商品和码库独立，错误归属不可兑换 | backend/internal/service/liandong_browser_test.go | Automatic | Yes |
| AC-02 | 设备凭据仅允许授权商品的补货操作，权限拒绝与撤销立即生效；商户登录材料不离开商户标签页，不发送给 Sub2API 或写入扩展持久存储 | tools/ldxp-browser-extension/test/backend.test.js、tools/ldxp-browser-extension/test/merchant.test.js | Automatic | Yes |
| AC-03 | 完整库存双扫描及逐码 SHA-256 核验，不能仅以数量相等放行；重复摘要、缺项、部分上传和响应不明均拒绝继续追加 | tools/ldxp-browser-extension/test/merchant.test.js、backend/internal/service/liandong_browser_integration_test.go | Automatic | Yes |
| AC-04 | 开始上传前持久化同批身份；超时、重启和响应丢失后只核验原批，不重传、不另领新批；已确认完成的原批可幂等恢复 | tools/ldxp-browser-extension/test/core.test.js、tools/ldxp-browser-extension/test/worker.test.js、backend/internal/service/liandong_browser_integration_test.go | Automatic | Yes |
| AC-05 | 离线、登录失效、浏览器重启或不确定结果后暂停；重新登录或单独检查不会恢复自动补货，核验后仍须手动开始；销售入口和自动补货启停独立 | tools/ldxp-browser-extension/test/worker.test.js、backend/internal/service/liandong_browser_test.go | Automatic | Yes |
| AC-06 | 前台各面额跳转对应外部商品并返回 purchase 兑换；金额准确且重复兑换不再次到账；兑换中心与原生在线支付保留既有入口和行为 | frontend/src/components/payment/__tests__/LiandongRechargePanel.spec.ts、acceptance/machine/e2e/ | Automatic | Yes |
| AC-07 | 购买页面桌面/手机及 Chrome 管理页在正常、空、加载、错误、暂停和未保存状态均可读可操作；Figma 同步范围与未核对状态逐项记录 | acceptance/visual-fidelity/ | Automatic | Yes |
| AC-08 | 完整本地 CI 按明确候选身份执行全部必需阶段，保留失败与未执行事实；通过不得代替真实 Chrome 或人工签署 | tools/quality/run_local_ci.sh、acceptance/ | Automatic | Yes |
| AC-09 | 真实 Chrome 使用已登录商户完成授权商品的库存核验与获准少量补货，保存脱敏结果；测试与正式环境、各面额分开记录，不能由模拟测试或历史 5 元购买代替 | acceptance/external-sandbox/、acceptance/production/ | Automatic | Yes |
| H-01 | 5元额度购买、发货、返回兑换与重复确认的实际观察及双人签署 | acceptance/human/2026-W37/2026-09-13-chrome-restock-products/checklist.md#h-01 | Human | Yes |
| H-02 | 10元额度购买、发货、返回兑换与重复确认的实际观察及双人签署 | acceptance/human/2026-W37/2026-09-13-chrome-restock-products/checklist.md#h-02 | Human | Yes |
| H-03 | 20元额度购买、发货、返回兑换与重复确认的实际观察及双人签署 | acceptance/human/2026-W37/2026-09-13-chrome-restock-products/checklist.md#h-03 | Human | Yes |
| H-04 | 50元额度购买、发货、返回兑换与重复确认的实际观察及双人签署 | acceptance/human/2026-W37/2026-09-13-chrome-restock-products/checklist.md#h-04 | Human | Yes |
| H-05 | 100元额度购买、发货、返回兑换与重复确认的实际观察及双人签署 | acceptance/human/2026-W37/2026-09-13-chrome-restock-products/checklist.md#h-05 | Human | Yes |
| H-06 | 管理员能完成少量补货并理解库存与商品归属的实际观察及双人签署 | acceptance/human/2026-W37/2026-09-13-chrome-restock-products/checklist.md#h-06 | Human | Yes |
| H-07 | 中断与结果不明时能识别暂停，核对后手动恢复的实际观察及双人签署 | acceptance/human/2026-W37/2026-09-13-chrome-restock-products/checklist.md#h-07 | Human | Yes |
| H-08 | 销售入口独立、兑换中心与在线支付兼容且窄屏可用的实际观察及双人签署 | acceptance/human/2026-W37/2026-09-13-chrome-restock-products/checklist.md#h-08 | Human | Yes |

## Exploratory testing

分别检查五种面额是否一眼可区分，手机与桌面能否顺利选择、外跳和返回；管理员能否区分售卖与自动补货、数量已知与身份已核验、检查与恢复。探索记录不替代逐项签署。

## Production monitoring and rollback

运行身份、构建、发布、健康与真实 Chrome 技术证据只存任务机器证据根。发现错发、重复到账、跨环境混用或无法核验的上传即停止相关补货并核对原批；暂停补货本身不改变销售入口。回退实现不能删除既有权益或批次事实。

## Risks and open decisions

合同与人工清单尚待审阅；完整 CI、真实 Chrome 和业务验收的结论以当前执行结果为准。Figma 已同步购买页桌面/手机及 Chrome 管理空状态，其余交互状态尚未核对；不由此宣称视觉通过。缺少可用商品、获准余额、验收账户、商户入口或安全故障场景时，对应项保持阻塞。最终必需机器证据与审批齐备后才允许中央工具入队。
