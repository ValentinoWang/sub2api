# Acceptance Run: 20260915T144041Z-test-listing-2b27b5

- Run ID: 20260915T144041Z-test-listing-2b27b5
- Task ID: ldxp-http-restock-local
- Lane: machine/local-runtime
- Status: PASS
- Acceptance contract: agents-results/2026-09-15/ldxp-http-restock-local/acceptance-contract.md
- Contract version: 1
- Contract SHA-256: f9c2eeddae8878127f2d9e3956ec9aeeb23e0bee9339261af9e512914eaa9d02
- Source identity: merchant-index-ac52079d-observed-protocol
- Runtime identity: test-listing
- Executor or reviewer: Codex
- Started at: 2026-09-15T14:40:41.198365Z
- Completed at: 2026-09-15T14:40:41.216069+00:00
- Evidence directory: evidence/

## Scope

仅修复五元测试商品未上架导致无法购买的问题；不创建订单或变更其他商品。

## Procedure

从当前商户前端脚本核对 statusUpdate 请求为 id 和 status，并确认上架值为1。精确核对商品874783的测试名称与5元价格后将状态0改为1。读回全部商品的名称、金额与状态，确认其他商品未变化。匿名调用公开 goodsInfo、getUserChannel 和 getGoodsPrice，核对短链接e8yrh4对应测试商品、已上架、存在支付渠道、单张报价5元。

## Requirement disposition

| Requirement | Result | Evidence | Notes |
| --- | --- | --- | --- |
| H-02 prerequisite | PASS | evidence/public-purchase-ready.json | 仅购买入口预检；不等于人工购买或兑换通过 |

## Findings

前一次交付购买链接时仅验证库存，没有验证商品上架状态。已更正当前商品并在工具说明补充上架、公开详情、渠道和报价检查。

## Evidence manifest

| Artifact | SHA-256 | Meaning |
| --- | --- | --- |
| evidence/merchant-status.json | fa2a1426cf50e9308d57f68cfe54c84172a59a7be2b57ef77d3987df7a6f4520 | 商户状态或公开购买预检 |
| evidence/public-purchase-ready.json | e594eee77f4faf47984eb11073515a40a662edcb161bbd2261a8b122275338dd | 商户状态或公开购买预检 |

## Unverified items

本次未进行浏览器渲染验收、订单创建、付款或兑换。待用户完成测试购买后继续。

## Conclusion

五元测试商品已上架，公开购买前置接口核对通过。其他商品的名称、价格、上架状态不变。
