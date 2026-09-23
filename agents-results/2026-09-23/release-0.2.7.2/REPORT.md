# 0.2.7.2 生产发布记录

2026-09-23：生产 `sub2api` 已从 0.2.7.1 替换为 0.2.7.2（打票代理池、速度与限额、代理学习、打票流水与手动打票，见任务 `codex-harvest-pool`）。按要求不保留回滚镜像。健康、版本、后台、商城与补货配置、页面外壳和新打票接口核验通过；**真实模型请求未能核验**，原因是账号 11 使用的代理节点连接 chatgpt.com 超时（见“功能核验”）。完整本地 CI 未全绿，失败项与 0.2.7.1 相同且均为既有问题。技术核验不等于人工验收或开售决定。

## 发布来源

- 提交：`a1bbccf62e7d482c3da774ebf6af5b4e939144e1`（分支 `release/0.2.7.2-codex-harvest`，推送为 fork `main` 与 `dev`）
- 版本：`0.2.7.2`；基线为生产 0.2.7.1 提交 `f4a7a4920938420be39a8c753fb265ac5659b88f`
- 内容：参考 ranxi2001/sub2api v2.8.0 的采票管理与 Codex_degrade 的限额规则，打票代理改从代理管理选择；移除单一打票代理地址设置，不保留兼容路径。业务请求的转发、上游连接与 WebSocket 代码与 0.2.7.1 相同。

## 同一镜像

- 镜像：`sub2api-local:0.2.7.2-a1bbccf62e7d`，Image ID `sha256:0ca96e166491e5bb4611172f2eeb64dd1b06f865fb95b9b34fd9769a60109949`，`linux/amd64`，OCI revision/version 与提交、版本一致，二进制自报 `0.2.7.2`。
- 构建：Docker VM 内 `goproxy.cn` 无法解析，以 `--build-arg GOPROXY=https://proxy.golang.org,direct --build-arg GOSUMDB=sum.golang.org` 覆盖，源码为已提交快照（`acceptance/image-build.log`，本地保留）。
- 归档 SHA-256 `76637168328b8a54207c5befe06e4d8b7e3fc9c2ae04d010719b0ab8ff3c59af`，上传后校验一致，生产 `docker load` 后 Image ID 一致。

## 迁移

- 生产新增 `245_codex_harvest_proxy_learning.sql`（代理学习记录与学习代数，并删除停用的 `openai_codex_ticket_harvest_proxy_url` 设置）与 `246_codex_harvest_flow_events.sql`（打票流水）。替换脚本核对迁移前后清单与候选声明精确一致。

## 本地 CI（未全绿，均为既有问题）

- `codex-harvest-pool/acceptance/local-ci-01`（官方脚本）：前 8 个阶段通过；backend-unit 仅 `TestValidateCreateParams_CheckModeMatrix/quota_probe_requires_primary_model` 失败，该用例需解析 `api.kimi.com`，本机无法解析，本次未改动相关代码。
- `local-ci-02-rest`（同阶段命令，仅跳过上述 1 项）：backend-unit、backend-integration（`-p 1`）、frozen-install、browser-extension-tests、frontend-lint、frontend-typecheck、frontend-build、govulncheck、pnpm audit 通过；backend-lint 41 项全部位于成本中心代码（与 0.2.7.1 相同）；frontend-tests 2575/2578 通过，3 项为既有 `CostCenterPages.spec.ts` 失败。
- `focused-01`：打票服务、后台接口、仓储集成（迁移 245/246）和前端页面定向测试全部通过；`machine/e2e` 截图验收 PASS。

## 隔离迁移验证

- [prod-rollback-proof.json](acceptance/prod-rollback-proof.json)：空库内新版启动并执行 245/246，旧 0.2.7.1 镜像在已迁移库上可启动，临时资源清理通过。发布后旧镜像已删除，不作为回滚来源保留。

## 切换

- 基线 [prod-baseline-before.json](acceptance/prod-baseline-before.json)：0.2.7.1，打票关闭，商城与支付开启，key 3 active/group 3。
- 备份 `/home/ubuntu/sub2api-backups/20260923-0.2.7.2-a1bbccf6`（TOC 1374 项、数据目录 33 项）。
- 维护公告 27 → 只重建 `sub2api` 单个容器 → [prod-replace.json](acceptance/prod-replace.json)：`running / healthy`，环境、挂载与端口不变，迁移核对通过。
- 恢复公告 28，维护公告 27 已归档：[prod-recovery.json](acceptance/prod-recovery.json)，正文不声明功能核验通过。

## 功能核验（模型请求未通过）

- [prod-functional.json](acceptance/prod-functional.json)、[prod-functional-2.json](acceptance/prod-functional-2.json)：健康、版本 `0.2.7.2`、后台、商城地址、补货配置、公开目录 200、页面外壳通过；`gpt-5.6-luna` 经 key 3 返回 502。
- 原因：请求已选中账号 11（今日重新可用），经代理 65（苏菲新加坡 hy2 节点 `3f545f96`）连接 chatgpt.com；mihomo 记录 `dial ... chatgpt.com:443 error: context deadline exceeded` 与 `timeout: no recent network activity`。同一代理的 wget 探测时通时断，属代理节点问题；业务转发路径代码未改动。补充诊断中 `gpt-5.5` 同样 502。
- 新接口：`GET /api/v1/admin/codex-harvest` 与 `/nodes` 正常返回（打票关闭、代理池为空、学习与流水为空），设置接口不再含打票代理地址字段。

## 清理与保留

- 生产删除 `sub2api-local:0.2.7.1-f4a7a4920938` 与本次上传的归档包，只保留 `sub2api-local:0.2.7.2-a1bbccf62e7d`；本地删除 0.2.7.1 镜像与 0.2.7.2 归档包，保留 0.2.7.2 镜像。
- 保留生产全部数据库与数据目录备份。

## 待办

1. 为账号 11 换一个能稳定连通 chatgpt.com 的代理，或排查代理 65 节点，然后重跑 `verify_live.py`。
2. 在“打票管理”选入住宅代理并决定是否开启打票（开启后 `fail_closed` 会在无票时拦截门控模型）。
3. 修复成本中心 41 项 lint 与 3 项 `CostCenterPages` 前端测试；人工验收 H-01 待执行。

## 边界

本次未导入代理、未修改账号或商品、未开启打票。技术核验不等于人工验收、开售或启用真实供应商提交。
