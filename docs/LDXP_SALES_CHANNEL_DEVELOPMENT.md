# 联动小铺充值与 Chrome 补货

当前保留一条运营流程：在本站配对商品并创建设备凭据，在 Chrome 插件统一设置每种额度的库存，启动自动补货。完整操作说明维护在 [插件 README](../tools/ldxp-browser-extension/README.md)，两站地址统一维护在插件 `config.js`。

## 业务范围

- 联动小铺负责商品展示、付款和发码；本站负责生成可兑换的码和唯一的权益账本。
- 固定规则为 ¥5 商品兑换 $5 额度，其他面额也按 ¥1 对应 $1；小铺手续费另外结算。
- 本地和生产运行相同代码，但使用不同商品 ID、设备凭据、兑换码、订单和余额。同一小铺商品不能混入两个站点的码。
- 用户从本站充值页选择商品，在小铺付款后回本站兑换。在线支付、余额充值与兑换继续使用原业务能力。
- 小铺是销售渠道。网页支付页和查询结果不足以证明原生支付 provider 的签名回调、退款或结算合同。

## 当前入口与代码

| 用途 | 位置 |
| --- | --- |
| 管理员商品与设备配置 | `/admin/tools/ldxp`；`frontend/src/components/admin/liandong/ChromeRestockPanel.vue` |
| 插件唯一操作页 | `tools/ldxp-browser-extension/options.html`；点击扩展图标直接打开 |
| 双站点配置与默认库存 | `tools/ldxp-browser-extension/config.js` |
| 配对、启停及浏览器定时检查 | `tools/ldxp-browser-extension/worker.js` |
| 库存核验和上传批次状态 | `tools/ldxp-browser-extension/core.js` |
| 小铺网页登录适配 | `tools/ldxp-browser-extension/merchant-adapter.js` |
| 设备授权、商品映射、持久批次 | `backend/internal/service/liandong_browser.go` |
| 设备范围内修改库存目标 | `backend/internal/service/liandong_browser_stock.go` |
| 未兑换权益退款 | `backend/internal/service/liandong_refund.go` |
| 每日结算账单核对 | `tools/commerce/reconcile_ldxp_settlement.py` |

旧工具安装器、独立 CLI、服务器工具任务页面及其 API 客户端已删除；`/admin/tools/ldxp/installation`、旧 `/config`、`/goods`、`/jobs/*` 等工具接口不再注册。历史批次、退款、配置核对和支付管理接口仍依赖的服务代码与数据库迁移保留，不删除真实库存、订单或历史记录。

## 自动补货规则

每种额度默认保持 **999 张未售库存**，插件读取商品后默认全选。点击「保存并自动补货」会完成双站点配对、保存库存目标、逐码核验和启动，无需逐个商品检查、开始。

每轮新增量为 `min(单批数量, max(0, 目标库存 - 已核验未售库存))`。目标范围 1–999，单批范围 1–20。首次补足需多轮；每分钟检查一次，达到目标只检查，售出后补缺口。

同一绑定重复提交复用原库存记录和未完成批次。上传前保存批次状态，上传后重新全量逐码核验；结果不明只核对原批次，不盲目重传或生成新批。单站失败显示实际暂停原因，不能把另一站成功当作双站成功。

关闭 Chrome、休眠、离线或商户登录失效都会停止补货。恢复连接或登录后，在插件点击「保存并自动补货」重新核验。浏览器方案不能保证关机期间持续供货。

## 接口与权限

管理员接口根路径 `/api/v1/admin/tools/ldxp/browser`，保留管理员认证、审计、限流和合规门禁：

- `GET /status`、`PUT /config`：库存、商品映射和管理员总开关。
- `POST /devices`、`DELETE /devices/:id`：创建设备、撤销设备。
- `POST /resume`：满足核验条件后解除服务端暂停。

设备接口根路径 `/api/v1/ldxp/device`，使用独立、限定商品的设备凭据和限流：

- `GET /config`、`POST /stock-target`：读取授权商品、修改其库存目标。
- `POST /inventory`、`POST /claim`：逐码核对、领取差额补货批次。
- `POST /batches/:id/start`、`POST /batches/:id/result`：持久化上传边界和结果。
- `POST /heartbeat`、`POST /resume`：授权状态与核验后的恢复。

`GET /api/v1/ldxp/products` 只返回可公开购买的商品。设备不能修改商品身份、到账金额、批次数量或管理员总开关。库存目标降低时如有未核实批次则拒绝。两站目标分别提交，界面必须准确报告部分成功。

商户登录仅在小铺标签页内使用，不上传给本站或存入插件配置；本站补货设备凭据仅存 Chrome 扩展私有存储，不写源码、ZIP、日志或证据。旧库存仅发送 SHA-256 摘要。

## 必须保留的验收

- 商品和站点身份匹配；未知码或库存分页不完整时禁止补货。
- 重复执行不重复生成或上传；上传响应丢失仍保留原批次。
- 998 张补 1 张，999 张不补，售出 2 张补 2 张。
- 兑换幂等；退款只处理未兑换权益。
- 未售库存与本地批次差异可见；已售未兑换不能误判为丢码。
- 日结账单逐项核对总额、手续费和净额。
- 登录失效、撤销设备、休眠和暂停全部不会继续创建新批。

本地测试、模拟商户浏览器验证、真实购买兑换和生产发布分别记录。源码或 ZIP 更新不代表正在运行的后端和真实 Chrome 已更新。生产发布前单独通知并确认；真实商品持续补货须在相应环境完成验证后启用。

机器证据使用 `agents-results/YYYY-MM-DD/<task>/acceptance/`，人工验收遵循 [Acceptance 目录说明](../acceptance/README.md)。历史人工记录保持只读。
