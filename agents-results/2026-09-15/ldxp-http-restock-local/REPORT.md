# 本机 HTTP 自动补货验证

**后续状态更新：** 原未决20张已恢复，新版0.2.4.5已在本机通过自动部分上传恢复验证；五档库存现均为999、无未决批次，定时任务达到目标后新增为0。最新证据见[补货恢复报告](../ldxp-restock-recovery/REPORT.md)。

**5 元本地商品已完成真实购买、发货、兑换、重复拒绝和售出后补货。** 短暂网络故障导致的永久暂停已修复，定时维护已恢复。当前自动选择本地站点全部五个测试商品，目标为每档 999 张，每 60 秒调度一次，每档每轮最多补 20 张。首次填充正在进行，不能把目标数当成已完成库存数。

## 当前运行结果

截至 2026-09-15T14:59:54+00:00 的完整自动周期：

| 测试商品 ID | 小铺未售库存 | 本站可兑换码匹配数 | 目标 |
|---|---:|---:|---:|
| 874783 | 284 | 284 | 999 |
| 875537 | 285 | 285 | 999 |
| 875562 | 265 | 265 | 999 |
| 875570 | 265 | 265 | 999 |
| 875577 | 265 | 265 | 999 |

五个商品分别是 ¥5→$5、¥10→$10、¥20→$20、¥50→$50、¥100→$100。脚本仅访问本地 Sub2API 和这些测试商品的卡密；没有写正式商品库存或访问生产 Sub2API。数量是快照，后续定时运行会继续变化。

## 本次真实购买与手续费

[本次验证记录](acceptance/machine/local-runtime/runs/20260915T145401Z-local-test-purchase-26b761/result.md)：订单 LD260915G5W4S3 为 5 元测试商品，已付款并发货一张。发货码与小铺已售库存及本地 $5 码逐字匹配；用户在 Chrome 的 `localhost:8080/purchase` 兑换成功，后台读回已使用、账户余额 $10。代理使用同一浏览器账号重复提交同码，返回 `redeem code already used`，余额仍为 $10。

该商品已改为买家承担手续费。新订单公开报价为商品 ¥5、手续费 ¥0.15、合计 ¥5.15，到账仍为 $5。已付款订单实付 ¥5，其 ¥0.15 手续费仍由卖家承担；没有改写旧订单、店铺默认或其他商品。

5 元档售出后有 244 张未售库存，恢复补货的首轮补至 264 张，随后自动周期继续补至 284 张，全部逐码匹配。首次补齐999尚未完成，所以此次证明售出后继续补货，尚未真实验证从999售出一张再补回一张。

## 已完成的真实验证

- [初次小批量补货](acceptance/machine/local-runtime/runs/20260915T142043Z-local-test-products-ec027e/evidence/restock-20260915T221048-7108c869.json)：五档补到各 4 张，共生成并上传 13 张，全部逐码匹配本站未用额度。
- [重复运行](acceptance/machine/local-runtime/runs/20260915T142043Z-local-test-products-ec027e/evidence/restock-20260915T221116-f566b32e.json)：各档仍为 4 张，新增兑换码和上传数均为 0。并发启动也被单进程锁拒绝。
- [中断注入](acceptance/machine/local-runtime/runs/20260915T142043Z-local-test-products-ec027e/evidence/intentional-upload-interruption.json)：5 元档成功上传 1 张后，故意退出进程，未执行读回确认。
- [新进程恢复](acceptance/machine/local-runtime/runs/20260915T142043Z-local-test-products-ec027e/evidence/restock-20260915T221232-0e8f454a.json)：先确认原批次已在小铺库存，5 元档没有再次上传；其余四档各补 1 张到测试目标 5 张。该中断是测试故障注入，上传和库存读回是真实操作。
- [定时任务读回](acceptance/machine/local-runtime/runs/20260915T142043Z-local-test-products-ec027e/evidence/summary.json)：任务已加载，重启后累计触发 4 次，最近退出码为 0，最近完整周期库存全部逐码匹配。
- [49 项针对性测试](acceptance/machine/unit/runs/20260915T142043Z-synthetic-3b83ba/result.md)：包含登录失效暂停、部分上传、错误面额、批次哈希不符、响应丢失、重启恢复、同机互斥、通知去重和未知启动项保护。

首次定时执行暴露了后端心跳才将过期设备标记为 disconnected 的时序问题。修复后，脚本依据新心跳状态，在完整库存核对通过后恢复设备。停止调度超过两分钟后重新启动，已验证可以恢复并继续补货。原失败回执仍保留。

后续一次网络超时又暴露暂停原因被覆盖成 `manual` 的缺陷。修复后仅 `network_error` 会自动进入全商品库存和原批次预检，其他暂停原因保持不变。旧状态先执行 `check`，确认原10元档批次已上传且五档没有未决批次，再执行 `resume`；没有重复上传原批次。[新回归记录](acceptance/machine/unit/runs/20260915T145838Z-synthetic-network-recovery-16a675/result.md)共59项通过，包含新进程恢复、全商品先核对、人工暂停优先和失败核对不能解除保护性暂停。

## 使用与配置

配置位于 [ldxp-restock.local.json](../../../tools/commerce/ldxp-restock.local.json)，默认选中本站全部已启用商品，目标 999、单批 20、间隔 60 秒。完整命令见 [工具说明](../../../tools/commerce/README.md)。本机绑定和定时任务已完成，用户不需要再填设备 key。

```bash
# 查看当前各档库存和暂停原因
/usr/bin/python3 -B tools/commerce/ldxp_http_restock.py status

# 暂停后续补货
/usr/bin/python3 -B tools/commerce/ldxp_http_restock.py pause

# 登录或库存问题处理好后，重新核对并恢复
/usr/bin/python3 -B tools/commerce/ldxp_http_restock.py resume
```

登录失效会暂停，保存可查看的原因，并尝试发送 macOS 通知。重新授权使用已有本机登录工具；账号密码不保存，商户会话和本站设备凭据分别保存在仓库外受限文件。启动项名为 `lol.rest2build.ldxp-http-local`，无需 Chrome 保持开启；电脑休眠、关机或退出 macOS 用户会话期间无法补货。

本机 Homebrew Python 的 `plistlib` 导入暴露了已有 `pyexpat` 系统库符号问题；实际运行使用标准库完整的 macOS Python 3.9.6。未修改全局 Python 安装，也没有添加第三方依赖。

## GitHub 参考

- [AliasHub / Mail Pickup](https://github.com/1120393079/aliashub/blob/main/mail-pickup/app.py)，AGPL-3.0-only：核对商品/库存读取与上传参数。其上传通过持久化 Playwright 页面，本工具独立实现 HTTP 传输，权益与幂等继续由现有 Sub2API 批次负责。
- [source-browser](https://github.com/maile456/source-browser/blob/main/server.js)：参考公开登录和会话协议；未声明许可证，没有复制其源码。
- [CookieCloud](https://github.com/easychen/CookieCloud)，GPL-3.0：能同步 Cookie 和 LocalStorage，但当前会话复用已可直接工作，没有安装该项目。

## 验证边界

自动补货、逐码库存核对、重复执行不重复发码与中断恢复已经有真实本地证据。目标 999 的首次填充尚未完成；长期登录有效期和全天候服务器运行尚未验收。

5 元档的真实购买、付款、发货、本地兑换、重复提交与售后补货已有本次证据，实际支付由用户完成。其余四档尚未做真实购买兑换，不能用5元档替代。人工工作区保持待验收，合同为草稿，未签署 PASS。

用户在4174内置浏览器登录时另遇 `Network error`。当前4174公开接口与无凭据登录POST代理检查正常，空JSON登录请求54毫秒返回预期参数校验400；Chrome 8080已完成真实兑换。22:54前后后端多个业务与后台任务同时变慢，`auth/me` 和 `redeem` 曾约40秒后超时，同一观察窗口没有库存核对请求，不能归因为补货锁。23:03快照显示7个业务数据库连接全部空闲，活动事务、锁等待和空闲事务均为0；后续库存核对持续200。历史慢查询和等待日志未开启，尚未与内置浏览器失败建立请求级关联，不能宣称已定位或修复登录报错根因。未为此改配置、重启或终止连接。

没有运行完整本地 CI、提交或推送代码、重建应用镜像或部署生产。本报告只给出本次本地脚本验证结论。
