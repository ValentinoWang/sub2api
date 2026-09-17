# 链动小铺补货恢复：GitHub 公开源码核查

结论：本轮找到可参考的已售卡密核对实现，未找到能直接证明“上传响应丢失后可安全重发，且已售卡密也不会重复入库”的现成完整方案。AliasHub 最有参考价值的是已售证据读取与本地关联；其上传重试本身仍存在未决结果边界。以下是静态源码核查，不是链动服务端合同或真实商户验收。

| 项目与固定提交 | 许可证 | 实际提供的能力 | 跨重试、已售后重复入库结论 |
|---|---|---|---|
| [1120393079/aliashub @ d45f51c](https://github.com/1120393079/aliashub/tree/d45f51c96495d2150c02e5ac7971d38ba3c05a50) | [AGPL-3.0](https://github.com/1120393079/aliashub/blob/d45f51c96495d2150c02e5ac7971d38ba3c05a50/LICENSE) | 商户卡密上传、已售卡密轮询、可选订单详情回填、本地唯一账本 | 不能证明；通用异常重试可能重发上传，账本晚于远端响应写入 |
| [qion888/ldsub2api @ 21435fa](https://github.com/qion888/ldsub2api/tree/21435faf3ada8889ad0f1c17c4d5447c7645139b) | [Apache-2.0](https://github.com/qion888/ldsub2api/blob/21435faf3ada8889ad0f1c17c4d5447c7645139b/LICENSE) | 买家查单、卡密找回、导入 Sub2API 的持久重试记录 | 范围是买家账号导入，不是商户卡密入库恢复 |
| [miku1130/ldxp-merchant-toolkit @ 90bdecc](https://github.com/miku1130/ldxp-merchant-toolkit/tree/90bdecc5c19f97ccf43a91e2617e8b56dc7919ca) | [MIT](https://github.com/miku1130/ldxp-merchant-toolkit/blob/90bdecc5c19f97ccf43a91e2617e8b56dc7919ca/LICENSE) | 货源搜索对接、商品批量编辑、库存展示 | 未实现卡密上传恢复或已售卡密去重 |
| [miao8818/ldxp-consumption-assistant @ c821b69](https://github.com/miao8818/ldxp-consumption-assistant/tree/c821b6992c2533a28b562a010c58cfaa770f3b86) | [MIT](https://github.com/miao8818/ldxp-consumption-assistant/blob/c821b6992c2533a28b562a010c58cfaa770f3b86/LICENSE) | 买家订单列表统计、按订单号合并、CSV 导出 | 不能代替商户侧逐卡身份核对 |
| [Samge0/wzyp-view @ 0c91b5b](https://github.com/Samge0/wzyp-view/tree/0c91b5bbd2c7bf94f5fb1f131cc91e95d0d92ad3) | [MIT](https://github.com/Samge0/wzyp-view/blob/0c91b5bbd2c7bf94f5fb1f131cc91e95d0d92ad3/LICENSE) | 公开店铺商品数据洞察与价格库存采集 | 未找到商户补货恢复能力 |
| [hotcrush/Aether_api @ 77280a5](https://github.com/hotcrush/Aether_api/tree/77280a5984d20f77619335af85684f77eabbb49e) | 未发现根 LICENSE；GitHub license 为 null | 公开市场监控与补库事件通知 | “补货”指观察库存增加，不是向商户库存上传卡密 |
| [iKeilo/LdxpM @ 2fe31f0](https://github.com/iKeilo/LdxpM/tree/2fe31f0e5a04aeb227950a9f206ce8f1c816bdda) | 未发现根 LICENSE；GitHub license 为 null | 公开库存变化和补货提醒 | 未实现商户卡密上传 |
| [yusheng266186-beep/liandong-ai-radar @ d0b1c88](https://github.com/yusheng266186-beep/liandong-ai-radar/tree/d0b1c8831e042156c64c0f375ed5997f61a57946) | [MIT](https://github.com/yusheng266186-beep/liandong-ai-radar/blob/d0b1c8831e042156c64c0f375ed5997f61a57946/LICENSE) | 公开商品价格、库存雷达 | README 和目录显示为观察端；未证明商户入库恢复 |

`codingwa/toolbox-release` 的默认分支提交接口返回 HTTP 409，未取得可核查源代码，不能作为现成实现背书。检索覆盖项目元数据、README 关键词和上表固定提交的相关源码，不能据此断言整个 GitHub 不存在其他实现。公开源码只保存在本研究目录，未执行、安装或并入产品代码。

## AliasHub 可参考的具体实现

1. **已售卡密不是库存减少的推测。** [app.py:2947](https://github.com/1120393079/aliashub/blob/d45f51c96495d2150c02e5ac7971d38ba3c05a50/mail-pickup/app.py#L2947) 的 `_fetch_inventory_with_browser` 请求 `POST /merchantApi/goodsCardStorage/list`，参数为 `goods_id`、`current`、`pageSize=100`、`keywords=""`、`status="1"`、`first=""`。代码把 `status=1` 解释为已售。每商品最多 100 页，遇到短页结束；这只能作为客户端使用方式，状态含义与分页完整性仍应对当前商户协议核实。
2. **逐卡关联销售证据。** [app.py:1255](https://github.com/1120393079/aliashub/blob/d45f51c96495d2150c02e5ac7971d38ba3c05a50/mail-pickup/app.py#L1255) 读取 `secret`、卡密 ID、`trade_no` 和销售时间；[app.py:1140](https://github.com/1120393079/aliashub/blob/d45f51c96495d2150c02e5ac7971d38ba3c05a50/mail-pickup/app.py#L1140) 计算 SHA-256 并匹配本地邮箱或外部卡密记录，写入销售账本，把本地对象标记为 `sold`。`pickup_sales` 的唯一键是 `(provider, ldxp_card_id)`，不是“所有历史卡密内容在远端唯一”的证明。
3. **可选订单详情补全。** [app.py:2982](https://github.com/1120393079/aliashub/blob/d45f51c96495d2150c02e5ac7971d38ba3c05a50/mail-pickup/app.py#L2982) 使用 `POST /merchantApi/Order/orderInfo`、`{"trade_no": ...}`；[app.py:3058](https://github.com/1120393079/aliashub/blob/d45f51c96495d2150c02e5ac7971d38ba3c05a50/mail-pickup/app.py#L3058) 只在有订单号、不是重复销售记录，并启用相应配置或提供测试替身时回填。不能说它默认每次都核实订单详情。
4. **轮询的是销售状态。** [app.py:3088](https://github.com/1120393079/aliashub/blob/d45f51c96495d2150c02e5ac7971d38ba3c05a50/mail-pickup/app.py#L3088) 的后台线程执行 `sync_now()`，并非按目标库存自动生成与上传新批次。上货入口由管理员发起。

## AliasHub 没有解决的上传边界

- [app.py:2784](https://github.com/1120393079/aliashub/blob/d45f51c96495d2150c02e5ac7971d38ba3c05a50/mail-pickup/app.py#L2784) 向 `POST /merchantApi/GoodsCardStorage/add` 提交 `{goods_id, content, first:0, remove_repeat:1}`。没有服务端源码、服务端唯一约束、响应丢失重放测试或已售后重复上传测试来说明 `remove_repeat` 的去重集合和保留时长。这个参数的存在不能当作幂等键。
- [app.py:2854](https://github.com/1120393079/aliashub/blob/d45f51c96495d2150c02e5ac7971d38ba3c05a50/mail-pickup/app.py#L2854) 先远端上传，拿到响应后才 `record_ldxp_uploads()`。本地候选排除“已售／已有上传账本”只保护已记录结果；远端成功而响应丢失时，本地没有提前持久化的 `pending/uncertain` 批次阻挡下一次提交。
- [app.py:2380](https://github.com/1120393079/aliashub/blob/d45f51c96495d2150c02e5ac7971d38ba3c05a50/mail-pickup/app.py#L2380) 的非 CDP 路径在异常后重跑整个 `operation(page)`，每条代理路线默认尝试 3 次，上传调用也使用这个包装器。30 秒 fetch 取消会变为 `LDXP_REQUEST_TIMEOUT`；它没有在重试上传前先核对该批卡密的未售与已售集合。
- 外部卡密入口 [app.py:2892](https://github.com/1120393079/aliashub/blob/d45f51c96495d2150c02e5ac7971d38ba3c05a50/mail-pickup/app.py#L2892) 只检查本次参数里 `external_id` 不重复，未在发请求前排除本地已经 `listed/sold` 的外部卡密。其 SQLite 唯一约束发生在远端写入之后；它不能阻止第二次远端副作用。
- 因此存在未被该实现排除的路径：第一次上传成功 → 响应丢失 → 部分卡密已售 → 包装器或人工重试再上传。**是否真的发生重复入库仍取决于远端 `remove_repeat` 是否覆盖已售历史；公开项目没有证明这一点。**
- 已售列表最多读取 100 页，满页到达上限时直接返回收集结果，没有显式“完整扫描已证明”标志。并发销售、分页漂移、历史删除、延迟可见仍需恢复逻辑单独处理。

## 其他项目的准确适用范围

- [ldsub2api/client.py:562](https://github.com/qion888/ldsub2api/blob/21435faf3ada8889ad0f1c17c4d5447c7645139b/backend/order_query/client.py#L562) 使用买家侧 `/shopApi/Order/list` 和 `/shopApi/Order/info`。其 [card_import_history.py:469](https://github.com/qion888/ldsub2api/blob/21435faf3ada8889ad0f1c17c4d5447c7645139b/backend/sub2api/card_import_history.py#L469) 为导入 Sub2API 账号保存 `pending/failed` 重试上下文；这不是小铺商户入库结果核对，不能直接迁移为卖家去重证明。
- [merchant-toolkit:23](https://github.com/miku1130/ldxp-merchant-toolkit/blob/90bdecc5c19f97ccf43a91e2617e8b56dc7919ca/ldxp-merchant-toolkit.user.js#L23) 的接口表包含货源对接和商品编辑，没有 `GoodsCardStorage/add`。批量编辑不等于卡密自动补货。
- [consumption-assistant/page-bridge.js:38](https://github.com/miao8818/ldxp-consumption-assistant/blob/c821b6992c2533a28b562a010c58cfaa770f3b86/src/page-bridge.js#L38) 观察买家 `/shopApi/Order/list`，按 `trade_no` 归并订单。适合消费统计，不能证明某一批次的每张卡已被商户接收。
- [Aether_api/README.md:328](https://github.com/hotcrush/Aether_api/blob/77280a5984d20f77619335af85684f77eabbb49e/README.md#L328) 的补货是事件通知；[engine.rs:1183](https://github.com/hotcrush/Aether_api/blob/77280a5984d20f77619335af85684f77eabbb49e/src-tauri/src/market/engine.rs#L1183) 从前后库存差产生信号。没有上传事务。

## 对当前批次恢复的直接建议

本节是本次审查的独立建议，不冒充以上项目已经实现的行为，也不替代当前 20 张批次的真实读回。

1. 继续保留当前批次和每张卡的原始摘要，结果未知时只做核对；不要重新生成另一批来覆盖未决状态。
2. 按商品读取未售与已售卡密，分别保存来源、远端卡密 ID、订单号、观察时间及分页完整性，再以原始提交内容的统一摘要核对本批次。已售意味着该卡曾经成功交给小铺；“未售库存少于本批数量”本身不是上传失败。
3. 每张批次卡都在“未售或已售”证据中出现时，可确认该批已被远端接收；未售库存计数只用于下一批目标库存计算。远端已售不等于用户已经在 Sub2API 兑换，不据此改写本地兑换状态。
4. 任一卡缺失、列表不完整、身份字段不可读取或同一摘要映射到异常多张远端卡时，保持未决并补查订单；“本次没有查到”不自动等于“从未上传”。读操作可以重试，已进入远端写入区间的上传不能仅凭网络错误重放。
5. 自动重发缺失项前，需要当前远端合同或隔离商品验证证明覆盖范围：同批内容内去重、同商品跨请求去重、已售后重放、响应丢失后重放，以及并发重复提交。`remove_repeat=1` 可保留，但在这些边界验证前不能成为解除未决状态的唯一依据。

研究材料位于 `sources/<owner>__<repo>/<commit>/`。`github-search-results.json` 保存精简检索结果；`*-index.json` 保存固定提交和目录；`source-manifest.json` 记录每个公开源码文件的固定 URL、字节数和 SHA-256。未访问商户接口，未读取本地会话或凭据。
