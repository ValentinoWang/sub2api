# Acceptance Run: 20260915T145401Z-local-test-purchase-26b761

- Run ID: 20260915T145401Z-local-test-purchase-26b761
- Task ID: ldxp-http-restock-local
- Lane: machine/local-runtime
- Status: PARTIAL
- Acceptance contract: agents-results/2026-09-15/ldxp-http-restock-local/acceptance-contract.md
- Contract version: 1
- Contract SHA-256: f9c2eeddae8878127f2d9e3956ec9aeeb23e0bee9339261af9e512914eaa9d02
- Source identity: merchant-observed-protocol-fee-and-paid-order
- Runtime identity: local-test-purchase
- Executor or reviewer: Codex
- Started at: 2026-09-15T14:54:01.828385Z
- Completed at: 2026-09-15T15:03:05.065527+00:00
- Evidence directory: evidence/

## Scope

当前 5 元测试商品的买家手续费报价、真实订单发货、本地兑换与重复兑换，以及全部五商品的暂停恢复和售出后库存补货。不操作生产 Sub2API。

## Procedure

1. 精确修改商品 874783 的 fee_payer 为 1，保留其他配置；通过匿名购买接口取得 ¥5 + ¥0.15 = ¥5.15 报价。
2. 从商户订单列表定位最近一天唯一完成订单 LD260915G5W4S3，按订单详情核对商品 key e8yrh4。该旧订单实付 ¥5，手续费 ¥0.15 由卖家承担，不追改旧交易。
3. 订单发货 1 张，与商品已售卡 ID 33669 逐字一致，再与本地码 ID 5 精确匹配，权益为 $5，初始 unused。
4. 用户在 Chrome 的 localhost:8080/purchase 完成兑换，并回复“已到账 $5.00”；后台读回 used、到账后余额 $10。代理通过同一浏览器账号重复提交同码，页面返回 redeem code already used，后台余额仍为 $10。
5. 执行 check，五档未售库存全部匹配且没有新上传；原10元档未确认批次通过读回确认已上传，不重复发码。确认所有商品无未决批次后执行 resume，5元档从售后244张补到264张，五档本轮共补100张。
6. 读取后续 LaunchAgent 自动周期，最近成功退出码0，继续上传并逐码匹配，目标仍为每档999张。

## Requirement disposition

| Requirement | Result | Evidence | Notes |
| --- | --- | --- | --- |
| AC-03 | PASS | evidence/restock-20260915T225606-1f904674.json | 以本目录实际 check 回执为准；原批次核实，无盲重传 |
| AC-05 | PASS | evidence/scheduled-cycle.json | 五档真实上传、核对及恢复后定时周期 |
| H-02 observation | PARTIAL | evidence/purchase.json; evidence/redemption.json; evidence/duplicate-redemption.json | 仅5元档真实交易观察，无全SKU签署 |
| buyer fee | PASS | evidence/fee-update.json; evidence/new-order-quote.json | 新订单报价5.15；未再创建付费订单 |

## Findings

旧脚本一次网络超时后永久暂停已修复；先读回原批次再恢复，没有覆盖旧状态或补造订单/兑换码。此次售出发生在尚未首次填满999张期间，证明售出后核对和小批量补货；尚未证明从999售出一张后精确补回一张的真实场景。

用户在4174内置浏览器另遇 Network error；当前公开接口和无凭据登录POST代理探针正常，Chrome 8080真实兑换已完成。其失败与后端历史40秒超时尚未建立请求级关联，不能宣称该登录问题根因已修复。

## Evidence manifest

| Artifact | SHA-256 | Meaning |
| --- | --- | --- |
| evidence/duplicate-redemption.json | 571bd25342fa3182dcdc77bf3a4d4e526e27b8e0deb63bc51a94a11b80929432 | 脱敏运行证据 |
| evidence/fee-update.json | e303f7ef01e61372c23c9692f41ec4a57ca3b5f6f05907c4b297427081ad7232 | 脱敏运行证据 |
| evidence/new-order-quote.json | cab9f7e67872e198954713e02f9ce2a92e1ff4804ea0f14998be5ee09880e83b | 脱敏运行证据 |
| evidence/purchase.json | 0542770bf85fabcd337c2558aa6a4e3847e2a0ec4aeacdf88e06f91b362e23de | 脱敏运行证据 |
| evidence/redemption.json | 2e69de9d0929d66a4e2c5dab09b28b9ce511bcadf1e007ab501658709d3d861e | 脱敏运行证据 |
| evidence/restock-20260915T225606-1f904674.json | b61959ce8b04161af87bd050e1f127f45a105ebce04f4dc0202b60468dd9a49b | 脱敏运行证据 |
| evidence/restock-20260915T225805-3b7bc4db.json | 5833b3466bdd635eb7dd9bfc65729574062aba9c14fd8c8a7eaa7ac54c52c811 | 脱敏运行证据 |
| evidence/scheduled-cycle.json | 2bb285de13e5806b74043ce63598f8c65b8c1ac3d6438115a2198d3c4b8347cb | 脱敏运行证据 |
| evidence/scheduled-readback.json | 68d79868658eac5b7edd6bf11be5f9560f00bde18825ba89271baa99eba80e27 | 脱敏运行证据 |

## Unverified items

其余四档真实购买兑换、目标999首次补满、长期会话与24小时运行、真实退款结算、完整本地CI及生产部署未验收。人工合同仍为草稿，未签署PASS。

## Conclusion

5元本地商品的购买、发货、到账、重复拒绝与售后补货已取得真实证据；新订单买家手续费报价正确。整体多SKU人工验收保持未完成。
