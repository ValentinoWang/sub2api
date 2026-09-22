# Acceptance Run: 20260915T165501Z-merchant-test-products-f9d68b

- Run ID: 20260915T165501Z-merchant-test-products-f9d68b
- Task ID: ldxp-restock-recovery
- Lane: machine/local-runtime
- Status: PASS
- Acceptance contract: agents-results/2026-09-15/ldxp-restock-recovery/acceptance-contract.md
- Contract version: 1
- Contract SHA-256: 37c0fa2ebb63d8865bd988d25799d63113db8ea609387448c67d52ba7905b2f7
- Source identity: be8771d92b66f8742fa90036d8507b822306e3eb
- Runtime identity: merchant-test-products
- Executor or reviewer: Codex
- Started at: 2026-09-15T16:55:01.809556Z
- Completed at: 2026-09-15T18:22:58.541396+00:00
- Evidence directory: evidence/

## Scope

AC-01和AC-05：商户测试商品去重、原批次受控恢复、新版本真实部分上传中断的自动恢复、后续定时维护及五档999目标停止新增。仅本机后端与独立测试商品。

## Procedure

- 实测已有未售、已有已售及并发相同新码的远端去重，记录真实结果与精确库存回读；测试商品恢复原状态。
- 先对历史20张未决批次做权威数据库只读权益检查，再以已验证去重能力重传原码，404→424并确认原批次。此步骤标记operator recovery，不当作新协议自动执行。
- 从明确Git快照构建0.2.4.5，备份本地数据库约204MB并检查1358项恢复目录，仅替换8080；242迁移表、健康及配置正常。
- 100元测试商品20张批次只上传1张后进程退出75。LaunchAgent自动核对原批次，只补19张，数据库记录原20张唯一代码的完整交付证明；没有生成替代批次。
- 后续网络读取超时自行恢复并继续正常补货；五档均达到999、逐码匹配，后续周期uploaded=0。
- 全量CI基线26e666ca通过19个阶段；最终分支4aa71a仅脚本/回归增量，170项Python与原样源码24项Go独立验证补齐差异。运行镜像be8771d92保留其真实来源；4aa71a只新增测试，不影响运行实现。

## Findings

真实结果未知恢复不再依赖人工解锁：脚本保留原码和持久次数，在权益与去重准入后有限补缺，并以未售/已售证明确认。有限远端去重测试不是永久服务端合同，登录/商品身份变化、去重能力过期、库存差异或重试耗尽仍会暂停。

## Evidence manifest

| Artifact | SHA-256 | Meaning |
| --- | --- | --- |
| evidence/automatic-recovery-cycle.json | aa1320243d1f606fad64c3e3c6fe051b940e6dea70874237131750e7cc60004d | 脱敏运行证据 |
| evidence/automatic-recovery-db-proof.json | abcc3c25c2862b5472a52e397a356f1264cc9a2d1cf3b85b0dd8c3ff27b01de1 | 脱敏运行证据 |
| evidence/ci-source-delta.json | f1b2b8018cfac9ac991984d2a98daa1c051aea294d73756b65b54a27082314b0 | 脱敏运行证据 |
| evidence/ci-summary.json | 2f0d98c88cd12f1900b9bf7a566dff2db3ab048bb6aff40c1805277998102488 | 脱敏运行证据 |
| evidence/final-stock-scheduler.json | f2e01f8617d63065650d7938cbd09aced5c22743238060de64ddc92bb07b55b5 | 脱敏运行证据 |
| evidence/image-test-only-delta.json | b597601a3777af4834f27d1bce995fe655af4caab771ae21ad6076741a9211f6 | 脱敏运行证据 |
| evidence/local-backup.json | 62ab4c9fe7ef5954b43488676d93e8f00c4f393d4db0cff0c21abf5d0fa81653 | 脱敏运行证据 |
| evidence/local-container-replacement.json | 3fd7c95caf30dd94617e7760e89c3f9f578d00964b39d9b85fd77af2b0ec5e11 | 脱敏运行证据 |
| evidence/local-health-migration.json | d55ef098594fca2f7a1d19ac4b60b8ebeecd0c89f6252621424fff5acc221b6b | 脱敏运行证据 |
| evidence/operator-original-batch-recovery.json | 2c25e6dfe66f8eb122b0b812372a208df29f1be86b1c941173af26c38204a101 | 脱敏运行证据 |
| evidence/partial-upload-interruption.json | 978a99337da27811b823120e562ec6886b7924304463120bcfdbd1629aa8ff4d | 脱敏运行证据 |
| evidence/post-recovery-scheduler.json | ef72fda1a9d4541d50e5a19277d84df80cb47ed56e111dbab763daca5a7192bf | 脱敏运行证据 |
| evidence/remote-dedup-concurrent.json | 0706b3952e92e8952d3451b463d5741bed16f0960f50f3dcb1790b1c3fbe3285 | 脱敏运行证据 |
| evidence/remote-dedup-sold.json | 67f557a01a578201d65965562a19c41dbcac42dbe6a207817882320c0cb53882 | 脱敏运行证据 |
| evidence/remote-dedup-unsold.json | d8623d6924b48c4dbf2c6cc3b3f0646c8ad9204f9160eb707849e5f3cb725a78 | 脱敏运行证据 |
| evidence/restock-20260916T001841-29809ea2.json | d81d8a7fc230b9f66209ce7f73cb1bc24d593f5cf74951ee15cee9cd30f1e02c | 脱敏运行证据 |
| evidence/restock-20260916T011733-59e127a5.json | 4ddb4cc657155bcc9444558f6f7cb42e4d2b54d03ea19aa3708d1b09a6f4b7b6 | 脱敏运行证据 |
| evidence/restock-20260916T013916-4cbeb674.json | 483d90d5ca675f3980788e079817641ff7d7409b4bc6f0b77bc60e7c2ba8959d | 脱敏运行证据 |
| evidence/restock-20260916T014056-8c4e35e4.json | 4cd95cbd2c1b74bd7ea6b95fd58634365d72f7a4bb900ec2fb8d01509379eb4c | 脱敏运行证据 |
| evidence/restock-20260916T014305-25ea9769.json | aa1320243d1f606fad64c3e3c6fe051b940e6dea70874237131750e7cc60004d | 脱敏运行证据 |
| evidence/restock-20260916T014520-5a114fd2.json | 17e1816df8b4affd0e8a42bbc3dee389ac405cc95b5629ad0557804f109dc8fd | 脱敏运行证据 |
| evidence/restock-20260916T014754-9b7a2b4c.json | fda729db8f593db54dec0f6694b7d0264ae90b563b924de6ee59a9541a004775 | 脱敏运行证据 |
| evidence/restock-20260916T014924-caea5b5a.json | e29c277c23f67385c815506b19cf4ae24ddbff3d7d0f3582dded0a9ed21f2ea8 | 脱敏运行证据 |
| evidence/runtime-image-build.log | 32b1244cc54f019dc81d1c7fa7aa33e67894898aebca7b7b7bd332c36dc2689d | 脱敏运行证据 |
| evidence/target-full-no-new-codes.json | d962a3d6f227f4c3b786d4250b436d644a04e0f96185c35458b429977fb76152 | 脱敏运行证据 |

## Unverified items

没有生产部署、main合并、远端推送或正式人工签署。尚未证明服务器全天候运行、无限期会话有效、所有SKU真实付费购买或退款日结对平。本地目标维持与故障恢复PASS不等于这些范围通过。

## Conclusion

本地恢复最后一公里已完成：当前无未决批次，五档各999张匹配，定时维护正常且达到目标不再新增。完整CI加最终差异验证通过，源码/镜像/运行证据分别可追溯。
