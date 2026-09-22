# Acceptance Run: 20260915T142043Z-synthetic-3b83ba

- Run ID: 20260915T142043Z-synthetic-3b83ba
- Task ID: ldxp-http-restock-local
- Lane: machine/unit
- Status: PASS
- Acceptance contract: agents-results/2026-09-15/ldxp-http-restock-local/acceptance-contract.md
- Contract version: 1
- Contract SHA-256: f9c2eeddae8878127f2d9e3956ec9aeeb23e0bee9339261af9e512914eaa9d02
- Source identity: local-working-tree
- Runtime identity: synthetic
- Executor or reviewer: Codex
- Started at: 2026-09-15T14:20:43.557519Z
- Completed at: 2026-09-15T14:21:41.352899+00:00
- Evidence directory: evidence/

## Scope

仅验证本地 HTTP 登录、库存读取、补货脚本和 macOS 启动项生成；所有商户、本站和系统调度操作均使用模拟边界。

## Procedure

执行 `/usr/bin/python3 -B -m unittest discover -s tools/quality/tests -p 'test_ldxp_http_*.py' -v`，49 项全部通过。没有运行完整本地 CI 或修改应用容器。

## Requirement disposition

| Requirement | Result | Evidence | Notes |
| --- | --- | --- | --- |
| AC-01 | PASS | evidence/http-tests.log | 全部商品绑定、目标999配置、按缺口补货和目标满时不生成码 |
| AC-02 | PASS | evidence/http-tests.log | 哈希、商品和匹配数量校验，部分成功不能视为完整上传 |
| AC-03 | PASS | evidence/http-tests.log | 响应丢失、进程重启、原批次核对和禁止不确定批次重传 |
| AC-04 | PASS | evidence/http-tests.log | 固定本地站点、会话失效暂停、提示去重、定时任务及单进程互斥 |

## Findings

真实定时首跑发现心跳才更新 disconnected 的时序问题。增加对应测试并修复为完整库存核对后依据新心跳状态恢复。上传后增加匹配数量与实际库存总量的一致性校验。macOS 调度安装拒绝覆盖未知的同名任务。

## Evidence manifest

| Artifact | SHA-256 | Meaning |
| --- | --- | --- |
| evidence/http-tests.log | 090ed6196a6519c1fe764f5a847b4ee332d4bce89734bf0e49ab7a6dee52a76b | 49 个针对性测试结果 |
| evidence/source-identity.json | a392ba11080111a7a4a9645feb146e2c4923d236a60b8783d061165054dcfc41 | 当前工作区源码与解释器身份 |

## Unverified items

本轮模拟测试不证明真实购买、兑换、退款、结算、生产可用或人类验收通过。实际商户写入见独立 local-runtime 记录。

## Conclusion

当前源码的49项针对性测试通过；不是完整CI、发布或人工签署结论。
