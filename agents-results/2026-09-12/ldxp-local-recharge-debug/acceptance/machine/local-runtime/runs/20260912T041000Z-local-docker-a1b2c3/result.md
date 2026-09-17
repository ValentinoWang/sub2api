# Acceptance Run: 20260912T041000Z-local-docker-a1b2c3

- Run ID: 20260912T041000Z-local-docker-a1b2c3
- Task ID: ldxp-local-recharge-debug
- Lane: machine/local-runtime
- Status: PASS
- Acceptance contract: agents-results/2026-09-12/ldxp-local-recharge-debug/acceptance-contract.md
- Contract version: 1
- Contract SHA-256: 64eedd4024fbc416593118302b6708006bf305055b0536b15126ad45e939ad11
- Source identity: 20189b7348fa74020e066241890c240e0c28cad4+sub2api-local-1b221364260b
- Runtime identity: sub2api-local:0.2.5-local-1b221364260b
- Executor or reviewer: codex
- Started at: 2026-09-12T07:39:12.966422Z
- Completed at: 2026-09-12T07:58:38Z
- Evidence directory: evidence/

## Scope

本机 Docker 部署、前端开发入口、LDXP 管理路由的认证保护，以及 LDXP 服务、管理 handler、路由和迁移的专项自动化行为。

## Procedure

检查本机服务健康、LDXP 管理状态接口的未认证访问和前端入口；随后串行执行 LDXP 专项 Go 测试。首次合并测试因系统临时目录空间耗尽而中止；清理可再生 Go 构建缓存后，使用 `-vet=off` 串行完成目标行为测试。完整 vet 与全库测试不属于本次运行的结论。

## Requirement disposition

| Requirement | Result | Evidence | Notes |
| --- | --- | --- | --- |
| AC-01 | PASS | evidence/runtime-check.json | 后端健康、前端入口可读，LDXP 管理状态接口未认证访问返回 401。 |
| AC-02 | PASS | evidence/liandong-service-tests.txt, evidence/liandong-admin-handler-tests.txt, evidence/liandong-route-tests.txt | LDXP 服务、管理 handler 和路由专项测试通过。 |
| AC-03 | PASS | evidence/liandong-service-tests.txt | 批次持久化与恢复行为由服务专项测试覆盖并通过。 |
| AC-04 | PASS | evidence/liandong-migration-tests.txt | LDXP 迁移结构和约束专项测试通过。 |

## Findings

系统临时构建目录曾耗尽空间，导致一次带 vet 的合并测试无法启动。已仅清理可再生 Go 构建缓存并改为串行目标行为测试。该限制不影响正在运行的本地服务，但全量 vet 与全库测试仍未在本次运行中重做。

## Evidence manifest

| Artifact | SHA-256 | Meaning |
| --- | --- | --- |
| evidence/runtime-check.json | e079a9f8116fc1b0891f678682a19d91c6a667e07dfae85cd170712e1d86842a | 本机健康、前端入口、未认证保护和容器健康摘要。 |
| evidence/liandong-service-tests.txt | 267b05baf5e75c4dbab8352766bc3e0846fbfa601bb9cd02bbafcbeeb002c2b0 | LDXP 服务专项测试结果。 |
| evidence/liandong-admin-handler-tests.txt | 2b6c3ab576d9fe867252878b7a2a82da367c7e1e73e53075e963657921f2439c | LDXP 管理 handler 专项测试结果。 |
| evidence/liandong-route-tests.txt | 0b487d4eeba8640c693330e2570f700c68be56f7c3ea957c9b30260685ef6c91 | LDXP 路由和权限专项测试结果。 |
| evidence/liandong-migration-tests.txt | e6061d36481e8c03870fbef4a1e73a5b1e40b51a1ff12c71cfc220efd7667053 | LDXP 迁移专项测试结果。 |
| evidence/liandong-targeted-tests.txt | 61c7f46f1a558d1289d40e793e5fedfd5f6aed10085c2c5b0376643aa6bdedc0 | 首次合并运行因临时目录空间不足中止的原始输出。 |

## Unverified items

本次没有创建实际兑换码、上传库存、购买商品、兑换余额或检查真实外部发码。完整 Go vet 和全库测试未在这次运行中重做；本机结果不证明远端生产状态。

## Conclusion

本机 LDXP 调试入口已可用，服务健康、未认证保护及专项自动化行为通过。真实购买和兑换闭环仍在人工验收队列中，不能由本运行代替。
