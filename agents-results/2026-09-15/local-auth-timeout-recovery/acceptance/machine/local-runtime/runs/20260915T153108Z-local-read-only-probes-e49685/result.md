# Acceptance Run: 20260915T153108Z-local-read-only-probes-e49685

- Run ID: 20260915T153108Z-local-read-only-probes-e49685
- Task ID: local-auth-timeout-recovery
- Lane: machine/local-runtime
- Status: PARTIAL
- Acceptance contract: agents-results/2026-09-15/local-auth-timeout-recovery/acceptance-contract.md
- Contract version: 1
- Contract SHA-256: b92427a7bdd45e8d06900a69899aaeea75a8f2b5391b14fd8b0a21b8e0ab8538
- Source identity: local-auth-timeout-candidate
- Runtime identity: local-read-only-probes
- Executor or reviewer: Codex
- Started at: 2026-09-15T15:31:08.631657Z
- Completed at: 2026-09-15T15:45:16.521824+00:00
- Evidence directory: evidence/

## Scope

AC-05：当前本地只读探针、HMR代码读回和历史调查；同时记录未确认补货状态。

## Procedure

查看历史应用、数据库、Docker及电源日志；没有修改系统设置、终止会话或注入真实数据库/Redis故障。

本轮初检8080可用、4174无监听，恢复固定端口Vite后读取client.ts确认REQUEST_TIMEOUT与AUTH_REFRESH_UNAVAILABLE代码已提供。随后32个低频检查全部符合预期，最慢数据库只读约3.03秒；不涉及用户密码或真实兑换。

读取补货状态并对商品874783做库存核对：404张已有未售码均匹配，未决20张不在未售库存、batch_resolved=false，保持uncertain暂停。没有强行恢复或重复上传。

## Findings

历史22:54多个模块约40秒后失败，但缺当时等待链；无对应窗口inventory请求或可证实睡眠/容器重启/OOM。当前探针不再出现40秒超时。前端已由4174提供；8080仍为原0.2.4.4后端，代码修复未替换到运行容器。

补货仍存在未确认批次，不能声明持续补齐999已恢复。

## Evidence manifest

| Artifact | SHA-256 | Meaning |
| --- | --- | --- |
| evidence/current-local-probes.json | ac82ad5dff74e5a73dd0653c34ca0d6713c14ab6de62cb656ae4293eb3acb8c0 | 实测证据 |
| evidence/historical-investigation.json | a244ed0f6cb6353b3ca34caa0c1076968183a676255864af27e6c56734be47e2 | 实测证据 |
| evidence/hmr-readback.json | 2a6d019bd75a2e3dfb7eec1c3b47fbcaa3de6beeb19691f2b3c5949d0e66b971 | 实测证据 |
| evidence/live-with-restock.json | a1365ca3fe4ef0b1d2c16e8a75467c330dab2a4a35a3f7f4d1d38fd49d0b5e06 | 实测证据 |
| evidence/restock-20260915T233515-776e7083.json | c3704ac0f2f8feb967454be7b8cdc9d380b54df5a8c6862aa84e13f609c1375d | 实测证据 |
| evidence/restock-pause-readback.json | 794f13cf026925acec068afffb1cd6240d42d13b173525d8f3b9b8d29c310b9c | 实测证据 |

## Unverified items

历史40秒整体延迟的来源仍未证明。没有执行完整本地CI、替换8080容器或部署生产。针对性测试不等于全部repository包或人工/生产验收。

## Conclusion

当前连通及前端更新读回通过；历史根因、后端修复在原容器的效果和未决补货结果仍未完成，因此运行结论为PARTIAL。
