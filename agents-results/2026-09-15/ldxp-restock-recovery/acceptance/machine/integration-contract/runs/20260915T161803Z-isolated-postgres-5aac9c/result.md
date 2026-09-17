# Acceptance Run: 20260915T161803Z-isolated-postgres-5aac9c

- Run ID: 20260915T161803Z-isolated-postgres-5aac9c
- Task ID: ldxp-restock-recovery
- Lane: machine/integration-contract
- Status: PASS
- Acceptance contract: agents-results/2026-09-15/ldxp-restock-recovery/acceptance-contract.md
- Contract version: 1
- Contract SHA-256: 37c0fa2ebb63d8865bd988d25799d63113db8ea609387448c67d52ba7905b2f7
- Source identity: ldxp-delivery-proof-candidate
- Runtime identity: isolated-postgres
- Executor or reviewer: Codex
- Started at: 2026-09-15T16:18:03.832458Z
- Completed at: 2026-09-15T17:05:32.442112+00:00
- Evidence directory: evidence/

## Scope

临时PostgreSQL验证已售交付证明、原码权益、所有权、同事务审计和retry_eligible。

## Procedure

按evidence中的命令和红绿日志执行。失败记录保留，源码摘要绑定；没有以删除旧测试或放松有效断言换取通过。

## Findings

独立复核发现并修正跨商品准入、人工暂停、心跳授权暂停及旧批次重试预算残留。已售只作交付证明，未售库存单独核对。

## Evidence manifest

| Artifact | SHA-256 | Meaning |
| --- | --- | --- |
| evidence/commands.json | f53221320cf3dbb8afa3035ef89c2a7f26fcbba086657c2a9924ce9e91d1a31c | 脱敏验证证据 |
| evidence/green.log | d9a06dc30aabb0436f61611a561c73dfa7cfef31a3c70c4b395561ed3e2017d4 | 脱敏验证证据 |
| evidence/red.log | 5160a2a2454df2111bb6bf77623556fb15873ec0baa5d7325a5c85314b106c8d | 脱敏验证证据 |
| evidence/verification.json | 62ed04849d1f989b9efe9ce97f2aef8d84d3615d860727bdcb5008379dbd031a | 脱敏验证证据 |

## Unverified items

该记录不证明真实未决批次已恢复、生产发布或永久无人值守。完整CI、镜像和真实运行分别记录。

## Conclusion

24个新场景及原PersistentSafety通过；迁移242在临时数据库验证，未触碰原8080。
