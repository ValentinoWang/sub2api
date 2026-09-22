# Acceptance Run: 20260915T165501Z-original-batch-recovery-054747

- Run ID: 20260915T165501Z-original-batch-recovery-054747
- Task ID: ldxp-restock-recovery
- Lane: machine/unit
- Status: PASS
- Acceptance contract: agents-results/2026-09-15/ldxp-restock-recovery/acceptance-contract.md
- Contract version: 1
- Contract SHA-256: 37c0fa2ebb63d8865bd988d25799d63113db8ea609387448c67d52ba7905b2f7
- Source identity: be8771d92b66f8742fa90036d8507b822306e3eb
- Runtime identity: original-batch-recovery
- Executor or reviewer: Codex
- Started at: 2026-09-15T16:55:01.080808Z
- Completed at: 2026-09-15T17:05:32.369701+00:00
- Evidence directory: evidence/

## Scope

原默认59项保护、新恢复26项以及供应商能力与暂停边界。

## Procedure

按evidence中的命令和红绿日志执行。失败记录保留，源码摘要绑定；没有以删除旧测试或放松有效断言换取通过。

## Findings

独立复核发现并修正跨商品准入、人工暂停、心跳授权暂停及旧批次重试预算残留。已售只作交付证明，未售库存单独核对。

## Evidence manifest

| Artifact | SHA-256 | Meaning |
| --- | --- | --- |
| evidence/candidate-source.json | 37d6c07d98c5f7d11c042a92c997f37c8aa3b9e683ea6976fdd7d8319d188cc0 | 脱敏验证证据 |
| evidence/python-tests/all-http-final-v2.log | 68cb547dc58cad2ff846e4d76b90b6ee319c892a63cdb01327c63cc0df41de73 | 脱敏验证证据 |
| evidence/python-tests/all-http-final-v3.log | 3a3d55f5c3db77502ce2a2e74ba2bb0e259d0eab7f0cc7cae4acc720dbb519b0 | 脱敏验证证据 |
| evidence/python-tests/all-http-final.log | 48a8d50cb34b07ece0a309561d2fe66fe2f13b998e7eec597646fd937693f721 | 脱敏验证证据 |
| evidence/python-tests/all-http-large-page.log | 8da6eea957a278c9dbda016e5625c3f269a02d17edaf1814b4a415adecc7bbda | 脱敏验证证据 |
| evidence/python-tests/all-http-tests.log | 365ec1b4d136b6904b7e8d2de88ed92f7c047402340322d6059bbd067aaa4eb5 | 脱敏验证证据 |
| evidence/python-tests/legacy-default-recovery-disabled.json | 98a5f164ed1a66e5c3dab5fb83ba7374ea709d364b635de81e1c569238695ba5 | 脱敏验证证据 |
| evidence/python-tests/legacy-default-recovery-disabled.log | 5965bec43e3fa77258292b74d28113c1857bea456bf2201178ab717412eadba3 | 脱敏验证证据 |
| evidence/python-tests/legacy-http-probe.json | 19ecfc02ed5085f130d82218d551fb0687a2712d1797fecdee9a622c32610506 | 脱敏验证证据 |
| evidence/python-tests/legacy-http-probe.log | cd96a25556fdfd4d2f3d569ed603e46f73a12538a3b5197eb9597457f7684219 | 脱敏验证证据 |
| evidence/python-tests/protective-heartbeat-red.log | dc744a29588e5bc0408760d8b5a1912a95275ec21f3ee0233968715056fc417e | 脱敏验证证据 |
| evidence/python-tests/recovery-green.json | 72f3f8ebe331d26336272854e1dc89c5f560ea06351883d5700e8741477f05e0 | 脱敏验证证据 |
| evidence/python-tests/recovery-green.log | 6d64fdd09fcb90fc8cdfeff46800f37fc3a554fc80bf9e1e8eafd93435b6f82a | 脱敏验证证据 |
| evidence/python-tests/recovery-red.json | 76a8a8714a40412ffe671dc0cc9919b5a1ed48a77a149f3a787211e61d85b55d | 脱敏验证证据 |
| evidence/python-tests/recovery-red.log | 8807a41a5550c29b95202b253e988bb8c895b48fb1d53074fb52f5781d4d20af | 脱敏验证证据 |
| evidence/python-tests/recovery-review-2.json | 0a7d6a7d93dfca55755070be7b1fe6664cc0dc65b5453848f9a7a25dcdbc6e1f | 脱敏验证证据 |
| evidence/python-tests/recovery-review-2.log | e0e62ff76c873a66bebd38da432171b491b2aaccda44d95d8c75516504597fcc | 脱敏验证证据 |
| evidence/python-tests/recovery-test-summary.json | 7d165a060ce490b974a964c179fcf5e61a19f91da425d8ee4d181c21b1a6f263 | 脱敏验证证据 |
| evidence/toolchain/README.md | a34786afcce9612037a4309ec55dbfe7a7f79cb8fc574b6ad8d895a347b0b08f | 脱敏验证证据 |
| evidence/toolchain/golangci-lint-2.13.0-checksums.txt | 63229ded20eb772b3e6e68e73d6def65015df874912a939fbd4100efe79b3a3f | 脱敏验证证据 |
| evidence/toolchain/python-followup.log | 8f75ceb9203719450637718fb7a55886b6333f241ed41ece326a9c0f47f4823e | 脱敏验证证据 |
| evidence/toolchain/toolchain.json | a9d4868a7eb6b68560232e6626e6438375a003bc06d5b049bf6b59698779c479 | 脱敏验证证据 |
| evidence/toolchain/versions.log | f84720614400b399fd0f7a4a3bf4bce74da9e2ffdbb3deac654bc142c8084a7e | 脱敏验证证据 |

## Unverified items

该记录不证明真实未决批次已恢复、生产发布或永久无人值守。完整CI、镜像和真实运行分别记录。

## Conclusion

85项Python恢复相关测试通过；最终候选完整质量脚本170项通过，完整CI另行记录。
