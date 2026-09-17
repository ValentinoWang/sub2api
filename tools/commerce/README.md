# LDXP 本地工具

## 专用浏览器自动补货

[`ldxp_browser_restock.py`](ldxp_browser_restock.py) 使用 Playwright Python 控制一个专用 Chrome 窗口。人工登录、真人验证和补货请求使用同一浏览器会话；执行器不会导入日常 Chrome 的 Cookie，也不将验证后的 Cookie 搬回独立 HTTP 客户端。商品身份、本站兑换码批次、逐码核对和失败恢复继续复用 [`ldxp_http_restock.py`](ldxp_http_restock.py) 中的业务规则。

当前运行范围固定为本机 `127.0.0.1:8080` 的独立测试商品。项目配置为 [`ldxp-restock.local.json`](ldxp-restock.local.json)：所有已绑定额度各保持999张，每档每轮最多补20张，正常维护间隔60秒。执行器每5秒领取管理员的立即检查请求；请求只是核验意图，不能直接声明通过。人工暂停、停用、库存差异与未决批次仍受保护。

日常操作：

1. 打开本地管理员的“联动小铺补货”页面。
2. 遇到登录或验证提示，点击“打开补货专用浏览器”，在本机助手中点击“打开补货专用窗口”。助手固定地址为 `http://127.0.0.1:52401/`，只绑定本机；它不收集账号密码或兑换码。
3. 在弹出的专用商户窗口里完成登录或真人验证。
4. 返回管理员页点击“我已完成验证，立即检查”。通过商户身份、全部绑定商品与完整库存核对后才恢复。失败时保留实际原因，不能通过反复点按钮解除保护。
5. 保持专用窗口开启。窗口关闭或登录再次失效时停止写库存，通过助手重新打开窗口。浏览器重启保留平台允许持久化的会话；会话型Cookie及平台授权仍可能失效，失效后需重新验证。

本地依赖固定在项目专用虚拟环境，使用已安装的Google Chrome：

```bash
/usr/bin/python3 -m venv ~/.local/share/sub2api/ldxp-browser-venv
~/.local/share/sub2api/ldxp-browser-venv/bin/python -m pip install --no-cache-dir -r tools/commerce/requirements-browser.txt
~/.local/share/sub2api/ldxp-browser-venv/bin/python -B tools/commerce/ldxp_browser_restock.py install-service
```

常驻任务名为 `lol.rest2build.ldxp-browser-local`，安装时验证并备份原补货任务，再替换自动启动入口，防止两种执行器竞争。同一专用目录只允许一个浏览器拥有者，现有调度锁和工作锁继续防止批次并发。电脑休眠、退出macOS用户会话或关机时不运行；生产服务器部署不在这次本机配置范围内。

浏览器目录为 `~/.local/share/sub2api/ldxp-browser/profile/`；本站设备凭据、库存状态、待处理批次及运行记录位于 `~/.local/share/sub2api/ldxp-http-restock/`。私有目录权限700，文件按受限权限保存。商户凭据仅在商户页面内使用，本站设备凭据只发给本机后端。日志保留固定错误、时间和摘要，不包含Cookie、Token、密码或完整兑换码。

补货前本站生成缺少的兑换码并登记原批次，上传后必须完整读回核对。上传响应丢失、网页验证或超时会保留原批次；后续只在原有恢复规则及去重能力证明允许时处理原批次，不生成替代码掩盖异常。未售码必须仍可兑换；已售码仅作为原批次交付证明。库存核对通过不代替购买、发货、兑换和退款验收。

底层库：[Playwright Python](https://github.com/microsoft/playwright-python)，Apache-2.0。持久浏览器及人工接管架构参考[公开源码核查报告](../../agents-results/2026-09-16/ldxp-browser-library-review/REPORT.md)；没有复制AliasHub的AGPL业务代码。

验证命令：

```bash
/usr/bin/python3 -B -m unittest discover -s tools/quality/tests -p 'test_ldxp*.py' -v
~/.local/share/sub2api/ldxp-browser-venv/bin/python -B tools/quality/test_ldxp_browser_live.py
```

第二组使用真实Chromium和完全拦截的合成商户页面，验证同一会话接管、重启持久化、逐码核对及不重复上传。它不联系真实小铺，不能替代真实授权证据。

## 商户登录态与库存脚本验证

[`ldxp_http_probe.py`](ldxp_http_probe.py) 只依赖 Python 3 标准库，通过 HTTPS 读取联动小铺商户商品和库存。登录一次后，独立脚本进程可以复用保存在本机的登录态，无需 Chrome 扩展参与请求。

首次登录或登录失效时，从仓库根目录执行：

```bash
python3 -B tools/commerce/ldxp_http_probe.py serve \
  --evidence agents-results/2026-09-15/ldxp-http-local-validation/acceptance
```

终端会给出本次启动的 `http://127.0.0.1:<端口>/`。在浏览器打开这个地址，输入商户账号和密码，点击“登录并验证”。端口自动分配，不占用本站的 4174 或 8080；关闭该进程后，这个登录地址不再有效。账号密码只用于此次登录，不保存到文件，也不需要填写到聊天或项目配置中。

本机已保存有效登录态时，直接执行以下命令。每次运行都会新建一份不含凭据和兑换码的 JSON 结果：

```bash
python3 -B tools/commerce/ldxp_http_probe.py verify \
  --evidence agents-results/2026-09-15/ldxp-http-local-validation/acceptance
```

- 商户登录态默认保存在 `~/.local/share/sub2api/ldxp-http-probe/session.json`，包括 Merchant Token 和 Cookie，不包括账号密码；目录权限为 700，文件权限为 600。
- 本地管理凭据复用 `~/.local/share/sub2api/commerce-release/dev-admin.key`，只请求固定的 `http://127.0.0.1:8080`。本机已有该文件，无需重新创建。上述文件都留在仓库外，不加入版本控制。
- 商品映射读取本站已有的本地配置，检查相应商品是否存在于商户目录。每个映射商品的未售库存扫描两次，在内存中对卡密摘要集合进行比较，只输出数量和一致性结果。它不会新建映射，也不会将商户卡密与本站码库逐码比对。
- `READ_ONLY_VERIFIED` 表示商户登录、目录、已有映射商品及两次库存读取通过；`HTTP_ONLY` 表示商户 HTTP 可用但缺少完整本地映射；`FAILED` 表示请求或核对失败。只有第一种状态的命令退出码为 0。
- 登录失效时返回 `login_required` 并结束此次验证，需要重新运行 `serve` 登录。它没有后台重试、定时调度或外部通知功能。

这个工具是只读验证入口，未实现生成码、上传、退款、兑换、自动保持 999 张库存或生产站运行。商户域名固定为 `https://www.ldxp.cn`，请求仅开放登录与读取所需的接口，拒绝重定向。已验证的会话复用不代表长期登录有效期已经验证。

运行针对性测试：

```bash
python3 -B -m unittest discover -s tools/quality/tests -p 'test_ldxp_http_probe.py' -v
```

真实本机验证结果见 [2026-09-15 验证报告](../../agents-results/2026-09-15/ldxp-http-local-validation/REPORT.md)。

## 日结账单核对

用本地命令逐笔比较**商户实际导出的结算账单**和**本地权威销售账本导出**，查看销售总额、手续费、退款和净额的差异。工具只读这两份文件，生成新的 JSON、CSV 和摘要，不连接商户 API，不修改数据库、余额、订单或退款状态。

## 准备两份独立 CSV

两份导出应覆盖同一商户、币种、结算范围和记账规则。工具匹配 `结算日期 + 外部订单号`，不是下单日期。商户延迟结算时，本地账本也必须提供对应的结算／会计日期；不能把下单日期改名后假装已匹配。工具不推断账期、不自动转换时区，也不把退款挪回原销售日。

| CSV 字段 | 含义 |
|---|---|
| `external_order_no` | 两份账本都能追溯的商户外部订单号，以文本保留 |
| `settlement_date` | 本行实际结算／会计日期，默认 `YYYY-MM-DD` |
| `gross_cny` | 买家实际支付总额，人民币，非负 |
| `fee_cny` | 手续费：收取为正，退回手续费为负 |
| `refund_cny` | 本结算日退给买家的金额，人民币，非负 |
| `net_cny` | 本行净额，可为负；必须等于 `gross_cny - fee_cny - refund_cny` |
| `currency` | 可选；如导出中存在这一列，其值必须为 `CNY` |

金额最多两位小数。工具直接转换成整数分，不使用浮点数，不四舍五入，也不换汇。金额中的货币符号、千位分隔符、科学计数法和超过两位的小数会产生无效行。不能把 Sub2API 的美元余额到账额当成人民币交易金额。

例如面值 ¥5 的商品，买家实付 ¥5.15、手续费 ¥0.15，则本行应为 `gross_cny=5.15, fee_cny=0.15, refund_cny=0, net_cny=5.00`。这不意味着订单创建日就是结算日。以后全额退款并返还手续费的独立结算行可以是 `gross_cny=0, fee_cny=-0.15, refund_cny=5.15, net_cny=-5.00`；应使用实际退款记账日期。

本工具不从兑换码批次、库存快照或权益到账记录猜测销售订单。现有 LDXP 本地库存／兑换信息不能自动代替具备外部订单号和人民币金额的权威销售账本。缺少真实导出或本地账本时，真实日结验收仍未完成。

同一个订单在同一天只能有一行。若商户导出按销售、手续费、退款分成多笔事件，应先按照经核对的会计规则制作有原始流水引用的每日订单汇总，再输入工具；工具不会擅自选一行、去重或合并重复键。原始文件应保留，汇总不能补造不存在的销售数据。

## 运行

用户终端命令，从仓库根目录执行：

```bash
python3 tools/commerce/reconcile_ldxp_settlement.py \
  --statement /绝对路径/商户结算账单.csv \
  --ledger /绝对路径/本地销售账本.csv \
  --output-dir /绝对路径/新的核对结果目录
```

默认读取 UTF-8 CSV，兼容 UTF-8 BOM。实际导出是其他编码时，可显式传入 `--statement-encoding gb18030` 或 `--ledger-encoding gb18030`。日期格式可分别通过 `--statement-date-format '%Y/%m/%d'`、`--ledger-date-format '%Y-%m-%d'` 指定；必须指向正确的结算日期字段。

如果商户使用中文列名，创建一份列名映射 JSON；以下只是格式示例，列名必须以真实导出为准：

```json
{
  "external_order_no": "订单号",
  "settlement_date": "结算日期",
  "gross_cny": "买家实付",
  "fee_cny": "手续费",
  "refund_cny": "退款",
  "net_cny": "净额"
}
```

追加 `--statement-map /绝对路径/商户列名映射.json`。本地导出也可使用独立的 `--ledger-map`。列名映射只选择字段，不改变金额符号、不换算币种、不填补缺失字段。两份映射必须各自列全六个必填字段，不能把同一列重复映射为多个金额。若币种列不是 `currency`，应显式添加 `"currency": "实际币种列名"`，避免遗漏币种校验。

## 查看结果

- `reconciliation.json`：完整的逐订单差异原因、双方行号和金额、每日及总计差额、输入文件 SHA-256、实际列名映射和日期格式。JSON 的 `*_fen` 金额对象使用整数分。
- `orders.csv`：每个订单双方原始金额行及匹配状态；金额显示人民币两位小数。订单号统一加文本前缀，防止表格软件执行公式或吞掉前导零；JSON 保留原始订单号。
- `daily.csv`：逐结算日的商户合计、本地合计和差额。差额定义为商户账单减本地账本。
- `report.md`：核对状态、问题数和总额摘要。

同日重复订单、重复行金额冲突、缺少对方订单、结算日期不同、金额不符和净额公式不符都会使结果失败。即使总计恰好相同，只要逐笔不同，也不会通过。两份空表不能通过。

合计保留所有可解析输入行，包括重复行，反映导出表面总数；无效行无法计入合计，会使 `totals_complete=false`。重复行不会被悄悄去重，也不会任选一条计算逐订单差额。存在问题时，应先查看 JSON 中的 `issues`，不能把摘要合计当成已经确认的结算额。

退出码：`0` 表示两份输入逐笔匹配；`1` 表示已生成带差异的报告；`2` 表示文件、结构或输出目录错误。输出目录必须不存在，以避免覆盖旧证据；目录和报告使用限制访问权限。工具不在终端打印订单号、兑换码或账单行，不把无关导出列复制到报告。

匹配结果只证明这两份文件按上述规则一致，不证明商户已付款、退款已到账或输入来源真实。实际结算验收仍需真实导出和负责人的核对记录。

## 开发验证

用户终端命令：

```bash
python3 -m unittest discover -s tools/quality/tests -p 'test_liandong_settlement.py' -v
```

测试使用临时目录和合成数据，覆盖 ¥5.15／¥0.15／¥5.00、跨日退款、手续费退回、精确分值、重复／冲突／缺单、相同总计但订单不符、中文列名、无效币种和金额、输出保护以及敏感无关列排除。该文件同时进入现有本地 CI 的 `tools/quality/tests` 自动发现范围；不需要修改 CI 入口。测试通过不等于已核对真实商户账单。
