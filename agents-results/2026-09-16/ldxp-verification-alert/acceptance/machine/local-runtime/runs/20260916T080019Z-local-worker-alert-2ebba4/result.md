# Acceptance Run: 20260916T080019Z-local-worker-alert-2ebba4

- Run ID: 20260916T080019Z-local-worker-alert-2ebba4
- Task ID: ldxp-verification-alert
- Lane: machine/local-runtime
- Status: PASS
- Acceptance contract: agents-results/2026-09-16/ldxp-verification-alert/acceptance-contract.md
- Contract version: 1
- Contract SHA-256: e7e34bb5a753ab5b39bb0db01376782154c9cf2b5abb218a0a5ba4b1ee853005
- Source identity: local-verification-alert-candidate
- Runtime identity: local-worker-alert
- Executor or reviewer: Codex
- Started at: 2026-09-16T08:00:19.579699Z
- Completed at: 2026-09-16T08:33:58.455194+00:00
- Evidence directory: evidence/

## Scope

AC-03：本机执行器的真实验证阻塞，通过独立runtime接口传到管理员前端并呈现。不是商户验证通过或补货已恢复的验收。

## Procedure

从提交2478cc12a97cd3caa10aac7db51fc9b47295aae0导出源码，使用固定本机工具链构建前端和Linux/amd64 Go二进制，再以原运行镜像为固定基底生成0.2.4.6镜像。构建全过程监测空间，最低6.888GiB，高于3GiB要求。初次Docker构建和首轮native镜像封装失败记录保留。

Docker日志显示16:08磁盘写入错误及no-space事件，随后引擎及多个本机入口无响应。主机已有空间后正常重启Docker，未重置数据；服务恢复。保存受影响补货配置、设备和库存表备份，检查恢复目录，替换本机8080并验证健康。

脚本真实读取仍为WAF验证页，安全上报paused/non_json/browser_verification_required。管理员API读回runtime状态，同时原商户授权时间仍为14:10:24，证明该上报没有刷新商户授权。Chrome管理员页实见提醒、固定商户入口、最近检查和下次检查时间。

## Findings

浏览器登录成功不等于独立脚本通行；本次只解决状态传递。当前脚本仍在保护性暂停，等待只读复查通过，不执行验证码脚本或盲目补货。

## Evidence manifest

| Artifact | SHA-256 | Meaning |
| --- | --- | --- |
| evidence/admin-runtime-readback.json | 8ebbec0d066a8e25c172ef4df64dc0362251f9e3415ff9c6fa3699325d20a6ee | 脱敏运行证据 |
| evidence/browser-readback.json | 560f84090829674739310a639737268e9034feeaaa35700a2171c0e6e6351cbf | 脱敏运行证据 |
| evidence/build-summary.json | d700ff5dc8814543167809bd9428a6db34837cdc3ad6eac8832941b305f95e1a | 脱敏运行证据 |
| evidence/candidate.json | 8a20a20fa4b267d62f1413615a06b62a22233b1fc36f9c254a4a42e5e3ae586c | 脱敏运行证据 |
| evidence/local-backup.json | 72effa093424e1df7c6eb8e8b45a421576a9e18b5cae41767e3eb9d8fc600eb4 | 脱敏运行证据 |
| evidence/local-deployment.json | 7e0d95da7cd553395cc226f2b558405639a0c55d0aec7c7087730e1d5d7cdd21 | 脱敏运行证据 |
| evidence/restock-20260916T163036-17554ba3.json | 37db0e602dd54177714ba9a57020d8aced63dcc5c13acb36a1d5b429593eac50 | 脱敏运行证据 |

## Unverified items

未部署生产，未运行本轮整仓完整CI或正式人工签署。手机布局和全站通知不在本轮验证范围。上游滑块仍由用户在平台认可的流程完成。

## Conclusion

本地状态传递和真实前端提醒PASS；补货仍暂停，未伪造授权成功。Go编译及镜像构建最低空间满足用户要求。
