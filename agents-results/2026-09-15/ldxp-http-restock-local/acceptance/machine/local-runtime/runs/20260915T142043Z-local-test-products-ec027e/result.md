# Acceptance Run: 20260915T142043Z-local-test-products-ec027e

- Run ID: 20260915T142043Z-local-test-products-ec027e
- Task ID: ldxp-http-restock-local
- Lane: machine/local-runtime
- Status: PASS
- Acceptance contract: agents-results/2026-09-15/ldxp-http-restock-local/acceptance-contract.md
- Contract version: 1
- Contract SHA-256: f9c2eeddae8878127f2d9e3956ec9aeeb23e0bee9339261af9e512914eaa9d02
- Source identity: local-working-tree
- Runtime identity: localhost8080-merchant-http
- Executor or reviewer: Codex
- Started at: 2026-09-15T14:20:43.873702Z
- Completed at: 2026-09-15T14:26:54.344544+00:00
- Evidence directory: evidence/

## Scope

本机五个已配置测试商品的小批量真实补货、完整库存逐码核对、达到目标后不补货、进程中断恢复，以及999目标的定时维护启动。实付购买、兑换与生产不在已验证范围。

## Procedure

先读取当前配置，自动创建本地专用设备。将目标临时设为4，逐商品核对、补货并重复执行。随后将目标设为5，在第一个商品上传成功后故意退出进程；新进程先核对原批次再继续其他商品。恢复默认999目标，注册macOS每60秒调度任务。发现心跳断线状态时序缺陷后修复，停机超过两分钟再启动，确认恢复后的多轮实际补货结果。

## Requirement disposition

| Requirement | Result | Evidence | Notes |
| --- | --- | --- | --- |
| AC-05 | PASS | evidence/summary.json | 五商品全部完成真实上传与读回；999为持续维护目标，尚未全部补满 |
| AC-03 | PASS | evidence/intentional-upload-interruption.json | 测试注入进程退出；新进程未重传原批次，恢复回执由summary精确引用 |

## Findings

首次定时执行失败的原因是/config读取后，heartbeat才新写入disconnected状态。通过新心跳返回状态恢复，并增加回归。日志与原失败回执保留。当前同一源码的最新成功运行回执记录worker/probe文件SHA-256；更早回执属于开发期间实际操作，不伪称全部来自最终源码。

## Evidence manifest

| Artifact | SHA-256 | Meaning |
| --- | --- | --- |
| evidence/before-local-config.json | 61af52c49b1999c52f91a8854375b9e1db232b4d3bec34c46aa56f3f8813b436 | 实际运行记录；阶段与范围见报告 |
| evidence/intentional-upload-interruption.json | 7942d838066aedd437df42907657426bcce8b841ff01186bc095605854903d9c | 实际运行记录；阶段与范围见报告 |
| evidence/open-source-references.json | 74332f2952c7c75e21940f8c8efd437d3744cf64c06284d8d5366619719d9ce0 | 实际运行记录；阶段与范围见报告 |
| evidence/restart-window.json | 042caad8eb9d7025d5409ed84611fe90d97aba1c5df901d33391a81f99522189 | 实际运行记录；阶段与范围见报告 |
| evidence/restock-20260915T220709-b541adc0.json | 7f821d010e84265870c631edc8f72d4d2d68c2cc37a203570c2e8aa223587fda | 实际运行记录；阶段与范围见报告 |
| evidence/restock-20260915T220713-a262b311.json | f269ad876b3716a3f27e3695f4a439d4fb115dbbcb1ec7ef624628f02c797f87 | 实际运行记录；阶段与范围见报告 |
| evidence/restock-20260915T221040-389a437d.json | 8d44a83560204baa19dedf51802594d5be0ea59526887dc5611ef38318dcff78 | 实际运行记录；阶段与范围见报告 |
| evidence/restock-20260915T221048-7108c869.json | 4bf2cab4bdfbd817132e82db125adf760d84b2f73ec0d52fd5791f9f14145d55 | 实际运行记录；阶段与范围见报告 |
| evidence/restock-20260915T221116-f566b32e.json | 0a9740055c6560b1826717d1b30afdd1b600c174c6b04c4d0653f17b3e11376d | 实际运行记录；阶段与范围见报告 |
| evidence/restock-20260915T221216-1a685e2b.json | d79128fabceb1ec1afacfeb6bc5a4072d701ac7af925d4a15d069e4af88de2a0 | 实际运行记录；阶段与范围见报告 |
| evidence/restock-20260915T221232-0e8f454a.json | de9e37363dc86b7c25c8d60a181d1cdaaf7dee3826fc3ae09a7058e0a8482091 | 实际运行记录；阶段与范围见报告 |
| evidence/restock-20260915T221536-7cd50d44.json | 49ca5462e7fdfa7c04f78c640da0b1907cdeb10b6ed1b5afde89b82a2e25eb6d | 实际运行记录；阶段与范围见报告 |
| evidence/restock-20260915T221542-36566582.json | 4487e6fc336660d669a1e9b65f690efb082f8d288aaa66504c822d8e5c48a072 | 实际运行记录；阶段与范围见报告 |
| evidence/restock-20260915T221744-81ed719f.json | f032af9b0371f01a411287b329a7827025c0c1f852a5e678be3e47643aaf2588 | 实际运行记录；阶段与范围见报告 |
| evidence/restock-20260915T221745-fa97df02.json | 7ee0f29b4c61d690714c833d544af6a9c36f9ad3071591c2986af9bf57935a49 | 实际运行记录；阶段与范围见报告 |
| evidence/restock-20260915T221756-87e5b6c1.json | b8668ade469271c6dc346ddbc8f2891bdb9c1581f9b6a211ff3bc5a6450e05ae | 实际运行记录；阶段与范围见报告 |
| evidence/restock-20260915T221847-36d3eafa.json | e0062adb77c6503bc1524c68184ee4fe2b07f0a5884fd7b7a2d48c5335c99b16 | 实际运行记录；阶段与范围见报告 |
| evidence/restock-20260915T221914-51d45942.json | 339b03de254a8758dce06142978c5fc1362ec48135a2d85a2b6109c4b5eaa5ac | 实际运行记录；阶段与范围见报告 |
| evidence/restock-20260915T222152-3045e25a.json | a611696beb1f987d56b8d89ea8f9f757add6e4d8f8aac1842843167e5412f076 | 实际运行记录；阶段与范围见报告 |
| evidence/restock-20260915T222200-44321deb.json | 58265030932a370a24229a79879de6e905a0e754792a89d53390a67d77ffb98f | 实际运行记录；阶段与范围见报告 |
| evidence/restock-20260915T222310-861afe83.json | 7b5b10d81d075ec2944657427d240a17d607193605fac87f8f9a8198d2587a20 | 实际运行记录；阶段与范围见报告 |
| evidence/restock-20260915T222431-d9c00fdd.json | 3c73498a9a3c8e23ade2b8f6d081ab1011f2d644163014929c3d171778981632 | 实际运行记录；阶段与范围见报告 |
| evidence/scheduled-first-run-failure.json | 090e8488234d90dcd5532dbf0c7f30e769b14890db49f178c5e72e582a75ee97 | 实际运行记录；阶段与范围见报告 |
| evidence/scheduled-runtime.json | 4e51ddd96ed26eeab149f5df6c8369e4fbdc45885e93433cc4dc1c9923348b7a | 实际运行记录；阶段与范围见报告 |
| evidence/summary.json | 542bba433e6d3b594559ca1e47c12b48c17d62dc16440a3846facd244babfdd0 | 实际运行记录；阶段与范围见报告 |

## Unverified items

真实付款、发货和兑换等待用户测试订单。各商品人工验收未签署；没有生产写入、完整CI、发布或长期24小时观察。999为配置目标而非本轮已达数量。

## Conclusion

本地库存补货与中断恢复技术闭环通过，定时维护正在运行。详见任务REPORT.md和本run的summary；不代表完整购买交易闭环或人类验收通过。
