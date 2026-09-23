# 0.2.7.1 生产发布记录

2026-09-23：生产 `sub2api` 已从 0.2.5.1 替换为 0.2.7.1，旧镜像已删除。健康、版本、后台、商城与补货配置、页面外壳核验通过；**真实模型请求未能核验**，原因是管理员可用的上游账号在发布前已全部不可用（见“功能核验”）。完整本地 CI 未全绿，失败项如实列在下方。技术核验不等于人工验收或开售决定。

## 发布来源

- 提交：`f4a7a4920938420be39a8c753fb265ac5659b88f`（分支 `release/0.2.7.1-merge-main`，推送为 fork `main` 与 `dev`）
- 版本：`0.2.7.1`
- 组成：`dev` `1d4627191`（上游 0.2.7 集成与 Codex ticket 生命周期修复）合并 fork `main` `899fb6614`（生产 0.2.5.1 的成本中心、LDXP 补货与迁移 242–244、登录会话修复、重置额度检查）。
- 必须先合并 `main`：`dev` 缺少上述生产功能，直接发布会移除这些功能，且与生产已应用的 242–244 迁移不一致。合并提交 `2a9c1853d` 解决 20 个冲突；`b5d0edfe0` 用中央 manager 重新生成人工验收日志投影；`f4a7a4920` 修正合并后本地化超时文案的测试期望。

## 同一镜像

- 镜像：`sub2api-local:0.2.7.1-f4a7a4920938`
- Image ID：`sha256:3a497985ba29c1d22ec6dced35cef327101e6f6ddfbb7b088c2b7c25fd138b71`，`linux/amd64`，OCI 标签 revision/version 与提交、版本一致，二进制自报 `0.2.7.1`。
- 构建环境：Docker VM 内 `goproxy.cn` NXDOMAIN，构建时以 `--build-arg GOPROXY=https://proxy.golang.org,direct --build-arg GOSUMDB=sum.golang.org` 覆盖默认值，源码仍为已提交快照（日志 `acceptance/image-build-4.log`，本地保留）。
- 归档 SHA-256 `6a92ff165fac0f1d467a7554b039820e43ecc5415965a0417eb1743afb1feb26`；上传后生产校验一致，`docker load` 后 Image ID 与本地一致，生产机未重建镜像。

## 迁移

- 生产新增 1 个：`238b_content_moderation_engine_meta.sql`。替换脚本核对迁移前后清单与候选声明精确一致。

## 本地 CI（未全绿）

- `local-ci-01`：默认 Node 22，toolchain 阶段失败；改用 Node 24 工具链。
- `local-ci-02`：acceptance-layout 失败（手工合并的人工验收日志投影过期），以中央 manager 重新生成后提交 `b5d0edfe0`。
- `local-ci-03`（`b5d0edfe0`）：前 8 个阶段通过；backend-unit 失败 1 项 `TestValidateCreateParams_CheckModeMatrix/quota_probe_requires_primary_model`，该用例需解析 `api.kimi.com`，本机 DNS 无法解析，合并未改动相关代码。
- `local-ci-04-rest`（`b5d0edfe0`，同阶段命令，仅跳过上述 1 项）：backend-unit、backend-integration（`-p 1`）、frozen-install、browser-extension-tests、frontend-lint、frontend-typecheck、frontend-build、govulncheck 通过；
  - backend-lint 失败：41 项，全部位于 `main` 带入的成本中心代码（`cmd/cost-*`、`internal/costing`），合并前 `main` 已如此；
  - frontend-tests 失败 4/2571：1 项为合并引入的 referral 超时文案期望，已在 `f4a7a4920` 修正并定向复测 3 文件 43 项通过；3 项 `CostCenterPages.spec.ts` 在 0.2.5.1 发布时已记录为 `main` 既有失败。
- 结论：`f4a7a4920` 未获得一次完整通过的本地 CI；发布依据是上述逐项归因，而不是 CI 通过。

## 隔离回滚验证

- [prod-rollback-proof.json](acceptance/prod-rollback-proof.json)：空库内新版启动、迁移，再以旧 0.2.5.1 镜像在已迁移库上启动，全部通过，临时资源清理通过。

## 切换

- 发布前基线：[prod-baseline-before.json](acceptance/prod-baseline-before.json)（0.2.5.1，商城与支付开启，补货开启、产品 0，key 3 active/group 3）。
- 备份：`/home/ubuntu/sub2api-backups/20260923-0.2.7.1-f4a7a492`（数据库 198 MB、TOC 1374 项、数据目录 34 项、compose 与 `.env`），02:55。
- 维护公告 25（02:56:02）→ 只重建 `sub2api` 单个容器 → [prod-replace.json](acceptance/prod-replace.json)：`running / healthy`，环境变量、挂载与端口不变，迁移核对通过。
- 恢复公告 26（03:13:29），维护公告 25 已归档：[prod-recovery.json](acceptance/prod-recovery.json)。因功能核验未通过，恢复公告正文不再声明“功能核验通过”，状态记为 `RECOVERED_WITHOUT_FUNCTIONAL_ACCEPTANCE`。

## 功能核验（模型请求未通过）

- [prod-functional.json](acceptance/prod-functional.json)：健康、版本 `0.2.7.1`、后台权限、商城地址、补货配置、公开目录 200、`/purchase` 与 `/redeem` 页面外壳通过；`gpt-5.6-luna` 经 key 3 返回 503 `no available OpenAI accounts supporting model: gpt-5.6-luna (pool=0)`。
- 数据库时间戳证明为发布前既有状态：
  - group 3 仅有账号 10 与 11：10 自 09-21 18:38 为 `error`（token 刷新不可重试失败）；11 自 09-17 23:44 起 `schedulable=false`。
  - 生产自 09-18 03:23 起没有任何使用记录。
- 补充核验（admin 其他 key 仅在 group 1）：
  - [prod-gateway-supplementary.json](acceptance/prod-gateway-supplementary.json)：原始 API 请求被 group 1 的“仅允许 Claude Code 客户端”规则拒绝，符合配置。
  - [prod-gateway-claude-code.json](acceptance/prod-gateway-claude-code.json)：真实 Claude Code CLI 请求返回 503 `no available accounts`。账号 12 access token 已于 09-21 12:18 过期；经代理 65 刷新 token 的 POST 持续 EOF（旧版 02:53:59、新版 03:06:40 各一次）；同代理 GET 该端点返回 405，代理本身可达。
- 结论：新版网关按账号状态正确拒绝，模型链路需重新授权账号后才能核验；回滚不会恢复模型可用性。

## 清理与保留

- 生产删除 17 个旧镜像标签（`sub2api-local:*` 0.1.183–0.2.5.1 及 `sub2api-rollback:20260904-0314`），仅保留 `sub2api-local:0.2.7.1-f4a7a4920938`；磁盘占用 16 GB → 13 GB。清单见 [prod-image-cleanup.txt](acceptance/prod-image-cleanup.txt)。
- 保留：生产全部备份目录。
- 同日按要求删除 0.2.5.1 与 0.2.7.1 的归档 tar.gz（生产发布目录与本地 `commerce-release` 各两份）。0.2.5.1 此后只剩本地 Docker 镜像 `sub2api-local:0.2.5.1-66162d981c7b`（`sha256:16e6e530…`）；需要回滚时须先从该镜像重新导出并上传，再按 `rollback_app.py` 流程执行。
- 本次未更新本地 8080 环境。

## 待办

1. 重新授权账号 10、12，确认账号 11 是否应恢复调度，然后重跑 `verify_live.py`。
2. 修复成本中心 41 项 lint 与 3 项 `CostCenterPages` 前端测试，使完整本地 CI 通过。

## 边界

本次未导入库存、未生成兑换码、未修改账号或商品。技术核验不等于人工验收、开售或启用真实供应商提交。
