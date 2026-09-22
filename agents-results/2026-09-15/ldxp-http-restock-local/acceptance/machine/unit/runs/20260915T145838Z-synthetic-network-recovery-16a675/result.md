# Acceptance Run: 20260915T145838Z-synthetic-network-recovery-16a675

- Run ID: 20260915T145838Z-synthetic-network-recovery-16a675
- Task ID: ldxp-http-restock-local
- Lane: machine/unit
- Status: PASS
- Acceptance contract: agents-results/2026-09-15/ldxp-http-restock-local/acceptance-contract.md
- Contract version: 1
- Contract SHA-256: f9c2eeddae8878127f2d9e3956ec9aeeb23e0bee9339261af9e512914eaa9d02
- Source identity: local-http-restock-network-recovery
- Runtime identity: synthetic-network-recovery
- Executor or reviewer: Codex
- Started at: 2026-09-15T14:58:38.461976Z
- Completed at: 2026-09-15T15:03:05.064067+00:00
- Evidence directory: evidence/

## Scope

本机 HTTP 补货脚本的合成回归，覆盖原有行为及网络中断恢复。未执行完整本地 CI。

## Procedure

执行 `/usr/bin/python3 -B -m unittest discover -s tools/quality/tests -p 'test_ldxp_http*.py' -v`，退出码 0，59 项通过（14 项登录探针、45 项补货）。测试屏蔽真实网络；源码摘要见 evidence/source-identity.json。

## Requirement disposition

| Requirement | Result | Evidence | Notes |
| --- | --- | --- | --- |
| AC-01 | PASS | evidence/http-tests.log | 全商品绑定、库存目标与单批上限 |
| AC-02 / AC-03 | PASS | evidence/http-tests.log | 合成协议与中断恢复；不代替真实商户验收 |
| AC-04 | PASS | evidence/http-tests.log | 仅网络故障自动预检恢复，其余暂停原因受保护 |

## Findings

实测网络读取超时暴露旧逻辑将 network_error 覆盖为 manual，之后每轮持续暂停。现保留根因；网络恢复必须先核对全部商品及原批次，未确认批次不得重传。check 失败不能把人工、授权或库存暂停降级成可自动恢复的网络故障。

## Evidence manifest

| Artifact | SHA-256 | Meaning |
| --- | --- | --- |
| evidence/http-tests.log | f1453359aee9d9a7e61a8e114bf3074731e439499873b4d74783ec3b1c966ce8 | 针对性测试证据 |
| evidence/source-identity.json | 511602a28070205544351da0af5faf8b457f1daffe856ad3050eda39e30a8c24 | 针对性测试证据 |

## Unverified items

不证明完整本地 CI、全天候可用、生产部署或人工签署。59 项合成测试不等于真实支付与结算测试。

## Conclusion

本次针对性回归通过，源码摘要与测试输出已绑定。
