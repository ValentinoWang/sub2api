# Acceptance Run: 20260916T092550Z-local-9ba5e1

- Run ID: 20260916T092550Z-local-9ba5e1
- Task ID: ldxp-manual-recheck
- Lane: machine/local-runtime
- Status: PASS
- Acceptance contract: agents-results/2026-09-16/ldxp-manual-recheck/acceptance-contract.md
- Contract version: 1
- Contract SHA-256: 5c08569af30310bec8ac38378c195105a82f6ac274ce826e13636819410202f1
- Source identity: f0e321a7cb350f809e5f7657a3edc3b433177a40
- Runtime identity: sub2api-local-0.2.4.8
- Executor or reviewer: Codex
- Started at: 2026-09-16T09:25:50.649654Z
- Completed at: 2026-09-16T09:30:35.629648+00:00
- Evidence directory: evidence/

## Scope

本机立即检查请求的真实执行与失败回传，管理员可见结果、移除可选扩展入口、商品映射底部默认折叠，以及仅本机版本更新。PASS限于此运行范围，不代表外部小铺授权成功或整份合同的人工验收。

## Procedure

真实Chrome管理员页点击“我已完成验证，立即检查”。请求由常驻脚本领取，约1秒后返回failed/non_json/resumed=false；原先下一次检查时间尚未到，因此验证了立即请求确实跳过等待。运行记录uploaded=0，未上传卡密。结果持久保存在后端，刷新后仍显示未通过原因。

Chrome实见商品映射在设备列表下方默认收起，展开后显示五档既有商品，刷新后重新折叠。可选扩展安装下载入口不存在。该轮截图与真实请求发生在0.2.4.7；随后发现新状态晚于页面30秒时钟而误报离线，加入稳定红例并修复，25项前端测试通过。后续0.2.4.8只改变该前端判断及版本；对应后端和脚本哈希未变。最终镜像与健康、持久请求已读回。

## Requirement disposition

| Requirement | Result | Evidence | Notes |
| --- | --- | --- | --- |
| AC-01 | PASS | evidence/backend-verification.json | 51个路由/数据库场景含22个新场景，重复请求及权限通过 |
| AC-02 | PASS | evidence/python-verification.json | 115项脚本测试，18项新测试；真实失败保持暂停且无上传 |
| AC-03 | PASS | evidence/real-recheck-result.json | Chrome真实点击及刷新后的失败结果；成功恢复仅合成验证 |
| AC-04 | PARTIAL | evidence/desktop-recheck.png | 桌面扩展入口移除，窄屏未实测 |
| AC-05 | PASS | evidence/mapping-folded.png | 桌面底部折叠、展开和刷新恢复已观察；窄屏未实测 |

## Findings

修复结果ACK丢失后会话丢失导致改写既有结果，以及非法请求ID抛出未捕获异常。实际页面还复现了显示时钟落后导致短暂误报离线，已由独立红绿用例覆盖。商户浏览器通过验证并不自动授权独立HTTP脚本；当前脚本仍遭网页验证拦截。

首轮离线依赖缺失、第二轮因磁盘保护停止在前端阶段，失败记录保留。清理12小时未使用且无Go编译进程时的3.947GiB可再生成Go缓存后，两轮成功构建Go阶段最低空间分别6.292和7.180GiB。没有删除业务数据、Docker卷或运行服务。最终替换首次30秒健康等待到期，随后健康和真实接口均恢复，未重复重启。

## Evidence manifest

| Artifact | SHA-256 | Meaning |
| --- | --- | --- |
| evidence/candidate.json | 7e83230e1c69546a15629b33884a01f3744485cbd1d2e908ba0b331af82a57cf | 本任务实测证据 |
| evidence/backend-verification.json | 1fd224495513556e316e15bb1ccd571346d3e5a27232e885253e3664f5ad69c3 | 本任务实测证据 |
| evidence/python-verification.json | 6d99ee23409006ac04ada4c1841580bb2c22aa68e59496662acdbd4acf2b8995 | 本任务实测证据 |
| evidence/frontend-green.log | d1f05362de8587feac1805340d3c584598d8844c4f66a4df6c8314ef535d2ab2 | 本任务实测证据 |
| evidence/frontend-clock-red.log | d194b34fb88faa9a6d8ff70289ffb251f1d4c8d39ffb4789d7327a55831f6246 | 本任务实测证据 |
| evidence/build-summary.json | 59312174f0ec41fd6684aa7f6358c84b1219e902a67955acc80a8d43c743bd6f | 本任务实测证据 |
| evidence/real-recheck-result.json | cdc996429eabcb160f6333da8d8c58da7eedb16900406b3ba94d402117106b34 | 本任务实测证据 |
| evidence/worker-recheck-receipt.json | 417cd36bf564a9913bf4bfe8a9aefbc92f78c0f53d8caccf5b3ec54c07fbb702 | 本任务实测证据 |
| evidence/final-health.json | bf0ac1b0eee3525d16b7e1112c6ecda232815e9349daba886e53169161fa8276 | 本任务实测证据 |
| evidence/final-readback.json | e37d9d1e0566017550719a26ea72627ba82a2abce4431bffb81f40923562632a | 本任务实测证据 |
| evidence/desktop-recheck.png | 0c7988903fd4003ce620524b440e2753b9b29a183ab2ecfdf33ed6bfe9394901 | 本任务实测证据 |
| evidence/mapping-folded.png | 2825018e9592b9d5f4208eed603beb041835505d2f6d6df2af41b70bb6fcf719 | 本任务实测证据 |
| evidence/reloaded-state.txt | add15ce143a1bbb5f2dc89cd8b3ff20b73181940eee6e4257a5d10dce64fab06 | 本任务实测证据 |
| evidence/service-check.json | b3d5c4f9acabfa7cd4c6b84218e7e71e2cf3bd6628c39052eaab3bf344fadd39 | 本任务实测证据 |

## Unverified items

未执行本轮完整整仓CI、生产部署、实际人工签署或窄屏人工观察。商户侧解除验证后的真实成功恢复仍未发生，不能将115项合成测试或浏览器登录当作该外部事实。并行任务使用Chrome导致最终0.2.4.8未再取得可靠截图，保留此前真实桌面截图与最终接口读回的边界。

## Conclusion

本机立即检查、真实失败回传、暂停保护及桌面折叠功能通过上述范围验证。独立脚本仍暂停；正式人工验收保持待验收。
