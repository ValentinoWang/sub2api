# Acceptance Run: 20260915T153107Z-isolated-timeout-reproduction-6e321a

- Run ID: 20260915T153107Z-isolated-timeout-reproduction-6e321a
- Task ID: local-auth-timeout-recovery
- Lane: machine/unit
- Status: PASS
- Acceptance contract: agents-results/2026-09-15/local-auth-timeout-recovery/acceptance-contract.md
- Contract version: 1
- Contract SHA-256: b92427a7bdd45e8d06900a69899aaeea75a8f2b5391b14fd8b0a21b8e0ab8538
- Source identity: local-auth-timeout-candidate
- Runtime identity: isolated-timeout-reproduction
- Executor or reviewer: Codex
- Started at: 2026-09-15T15:31:07.277272Z
- Completed at: 2026-09-15T15:45:16.512840+00:00
- Evidence directory: evidence/

## Scope

AC-01至AC-04的隔离错误与等待回归，仅覆盖明确修复。

## Procedure

前端真实loopback HTTP慢响应/连接失败/刷新503、429与401场景，先5条预期失败、1条身份失效正常通过；修复后加入刷新自身超时场景，共55项客户端、刷新与auth store测试通过。修改文件ESLint和全树vue-tsc退出0。

JWT原分类通过Go overlay复现依赖错误被误判401，修复后13个顶层测试通过。Activity服务取消等待、短预算及真实Gin链先3条失败，修复后4个服务测试通过，Gin阻塞场景包含在JWT测试中。

Redis真实实现文件与隔离net.Pipe伪RESP协议先复现100ms context仍等2秒，修复后约149ms返回，2项测试通过。完整repository包尝试因缺缓存测试依赖setup失败，保留red-package.log；随后精确实现文件测试通过。

## Findings

修复分类、会话保留、Redis截止时间、辅助写入等待四个范围。明确身份失效仍拒绝；未知结果的兑换POST没有自动重发。原始等待问题与修复都有红绿日志和源码摘要，未将历史事故来源归为某一缺陷。

## Evidence manifest

| Artifact | SHA-256 | Meaning |
| --- | --- | --- |
| evidence/activity/green-command.json | 7311dba823f0140bde839c2c18a58e58290dddeb34899d28a199af761f1be4bc | 实测证据 |
| evidence/activity/green.log | 79faec98e16e981648bff0e0e4ebc05ed0c3cb165b25bf329d8f4b7a200e6aa0 | 实测证据 |
| evidence/activity/red-command.json | ebf4b7e7993a1455bdba46c6cb06442da4a6235ddaaf25bf25147e7cb78efe08 | 实测证据 |
| evidence/activity/red.log | 2ec817af452d04452b39baa8d31aec95b5c66d3bd938abe902a2271c18002027 | 实测证据 |
| evidence/frontend-final.log | d2589efe0f470b6da30c4361cad5979103029d9387270c487fb589d2ceaa8184 | 实测证据 |
| evidence/frontend-green.log | f1a647f3f29f3ebda1d63f293eacd5b538c8be761db87949ac7591f1ef4abe66 | 实测证据 |
| evidence/frontend-red.log | 8ecc791be09c5a14f07fe70e3c441f3ef8e5c1294bf03a35ed47c766a55c6de6 | 实测证据 |
| evidence/frontend-static.json | bc8f2bcd3f66a58eba423c21d07a65d78c69bb007bb67aea8185bc7575020bc4 | 实测证据 |
| evidence/jwt/green-command.json | e57e8d74271804b426bd79575f558fa2a9e035f0a819f6f941b438151e357bf1 | 实测证据 |
| evidence/jwt/green.log | bdad0073f91cece6c7a8d73ea6b85ef03c73e723d994d62d0d0c02c70e228d5a | 实测证据 |
| evidence/jwt/pre-fix-jwt-auth.go.txt | 045d6afe27fccab3ee59d94041eb9cbaea0d458e02bef96a260fb402b2cf41ac | 实测证据 |
| evidence/jwt/red-command.json | a36f6cd4cbd49bb2cc6a95897fa730949fc786785b64ca27a620408fbeacfb52 | 实测证据 |
| evidence/jwt/red-overlay.json | 4a282846212c6abb8f2c99b7245001fe692e52fdbf43692a07818389c10bccfc | 实测证据 |
| evidence/jwt/red.log | 977f257aa30e0184c30df421388359eb47daacba9351dbc0895a0a983352d175 | 实测证据 |
| evidence/jwt-activity-verification.json | eda7fb601fddb145a3ecc7bdfbc5fcc244bbfc19d21f461e48b4d01d5e3c6ceb | 实测证据 |
| evidence/redis-context/diff-check.exitcode | 9a271f2a916b0b6ee6cecb2426f0b3206ef074578be55d9bc94f6f3fe3ab86aa | 实测证据 |
| evidence/redis-context/diff-check.log | e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 | 实测证据 |
| evidence/redis-context/green.exitcode | 9a271f2a916b0b6ee6cecb2426f0b3206ef074578be55d9bc94f6f3fe3ab86aa | 实测证据 |
| evidence/redis-context/green.log | e4c47f234758a4523908fff1c8d12e05f51a907961906523111566a2cf917bd9 | 实测证据 |
| evidence/redis-context/red-package.exitcode | 4355a46b19d348dc2f57c046f8ef63d4538ebb936000f3c9ee954a27460dd865 | 实测证据 |
| evidence/redis-context/red-package.log | ca7b6b833835bac4597204270e426151e73dd5920b40129005d7ee314b23a30a | 实测证据 |
| evidence/redis-context/red.exitcode | 4355a46b19d348dc2f57c046f8ef63d4538ebb936000f3c9ee954a27460dd865 | 实测证据 |
| evidence/redis-context/red.log | 77ffbf5e4fd128a7e8f79f6822a0e26961c21f238e723a29f2710a430f0f9131 | 实测证据 |
| evidence/redis-context/summary.json | 8e87c966644acdc653163f4103e9cb9abfef0069c3a87a510f7e6e7868e75e0e | 实测证据 |
| evidence/source-identity.json | d7efbfbda9eea04f9893ec9e92beeed654673b73a2da388d5cdf425b3a9612d3 | 实测证据 |

## Unverified items

历史40秒整体延迟的来源仍未证明。没有执行完整本地CI、替换8080容器或部署生产。针对性测试不等于全部repository包或人工/生产验收。

## Conclusion

已验证的9个源码/测试文件由source-identity.json绑定；74项顶层相关测试通过（55前端、13JWT、4activity、2Redis），合成与真实传输夹具不等于生产验证。
