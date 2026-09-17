# 链动小铺补货：开源浏览器会话方案核查

核查日期：2026-09-16。仅进行公开GitHub检索、源码与官方文档核对，没有安装项目、修改补货执行器或操作生产，也没有尝试自动解验证码。

## 建议

采用Playwright Python驱动专用持久浏览器，并把人工验证窗口与实际商户操作绑定到同一浏览器实例。现有Sub2API商品身份、确定性兑换码批次、库存摘要核对、未决批次恢复和999张目标继续作为业务约束。本地先用可见窗口；迁移服务器时可用noVNC让管理员远程操作实际执行器的窗口。

需要改变的是商户访问方式和验证入口。当前Python urllib客户端只读取自己的session.json；打开普通Chrome页面不更新该会话。新版立即复查队列能要求执行器检查，但本身没有解决两套会话分离。

## 查到的项目与适用边界

| 项目 | 查证能力 | 在本项目中的用途 | 边界 |
| --- | --- | --- | --- |
| [microsoft/playwright-python](https://github.com/microsoft/playwright-python) | Python控制Chromium/Firefox/WebKit，macOS与Linux均支持；Apache-2.0 | 首选浏览器执行库 | 本身不提供Sub2API补货规则，不保证平台永不再次验证 |
| [1120393079/aliashub](https://github.com/1120393079/aliashub/blob/main/mail-pickup/app.py) | LdxpSyncBridge包含商品/库存/卡密上传接口；连接已有浏览器、持久目录、页面内请求；发现滑块时提示人工验证 | 与本场景直接相关的架构和协议参考 | 项目为AGPL-3.0-only，整包含大量其他业务；本轮未运行其程序，源码不是商户真实成功证据 |
| [novnc/noVNC](https://github.com/novnc/noVNC) | 网页VNC客户端，支持桌面和移动浏览器；核心MPL-2.0 | 服务器阶段提供实际浏览器窗口的人工作业入口 | 还需显示环境、VNC服务与受保护访问；不负责补货或判断验证通过 |
| [easychen/CookieCloud](https://github.com/easychen/CookieCloud) | 同步Cookie和LocalStorage | 普通会话资料同步工具 | 不迁移完整浏览器运行环境，不足以证明独立HTTP脚本能通过当前验证，因此不作为这次主方案 |
| [g1879/DrissionPage](https://github.com/g1879/DrissionPage) | Python浏览器自动化，可重复使用已打开的浏览器 | 可选浏览器执行库 | 当前无需同时引入第二套浏览器控制框架；未核对完整许可或实测 |
| [qion888/ldsub2api](https://github.com/qion888/ldsub2api) | 店铺监控、买家支付链接、账号找回和导入，文档含专用验证浏览器目录 | 功能与界面参考 | 主要面向购买监控及账号导入；未证明商户补货适配可用，不引入其自动验证路径 |
| [miku1130/ldxp-merchant-toolkit](https://github.com/miku1130/ldxp-merchant-toolkit) | 小铺商户后台的用户脚本，查询筛选及批量编辑，MIT | 站内商户操作参考 | 不提供我们需要的完整999张库存与本站兑换码闭环 |

## 最直接的源码证据

AliasHub的mail-pickup/app.py中，LdxpSyncBridge从2239行定义；2303行的_fetch_json_with_page在浏览器页面内执行请求；2347行附近的_with_cdp_page连接既有浏览器并在出现滑块时要求人工验证；持久浏览器分支通过launch_persistent_context保存专用配置。这证明其架构与本次问题相关，但不能据此声称我们的商户已通过验证。

Playwright官方[BrowserType文档](https://github.com/microsoft/playwright/blob/main/docs/src/api/class-browsertype.md#async-method-browsertypelaunchpersistentcontext)明确说明持久目录保存Cookie和LocalStorage；同一目录不能被多个浏览器实例同时使用；日常Chrome默认用户目录不受支持，应创建专用目录。

官方[APIRequestContext文档](https://github.com/microsoft/playwright/blob/main/docs/src/api/class-apirequestcontext.md)说明context.request和page.request可共享Cookie。共享Cookie不等于所有请求都变成网页操作；本案应验证真正通过商户页面执行的正常工作流程，而非仅给现有HTTP请求换一个库名。

## 最小落地流程

1. 专用浏览器以持久目录启动，由单一执行器持有；用户登录和人工验证就在该窗口中进行。
2. 管理员“打开小铺完成验证”指向这个实际窗口；本地唤起窗口，服务器阶段进入受保护远程窗口。
3. 遇到登录、验证或未知网页时停止库存写入。用户完成后，“立即检查”驱动同一实例检查商户身份及全部绑定商品的完整库存。
4. 通过现有逐码核对和批次门禁后恢复补货；原上传不确定时读取原批次，不能再造一批码掩盖问题。
5. 验收人工验证前后、执行器重启保留会话、再次失效、完整库存差异，以及真实售出一张后补回999且无重复。

本地应先验证这一闭环，再单独计划生产部署。不会用关闭防护、自动解滑块或轮换代理代替正常授权。

## 证据范围

GitHub仓库搜索成功；仓库元数据API返回限额403，因此未用星数、更新时间或Release推断维护状态。公开raw源码和官方文档读取成功，URL与SHA-256保存在source-manifest.json。本轮只证明有可复用的实现和文档依据，不证明实际小铺授权或补货已恢复。
