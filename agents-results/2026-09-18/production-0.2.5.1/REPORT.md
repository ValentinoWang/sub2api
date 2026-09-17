# 0.2.5.1 上游合并与本地/生产发布记录

2026-09-18：上游 `Wei-Shaw/sub2api` main（v0.2.5+53）已合并进 fork，本地与生产均已替换为同一镜像并通过技术核验。真实购买、兑换、库存与人工验收仍未进行。

## 发布来源

- 提交：`66162d981c7bccb8f8125eb11c6aa51261d3aeca`（main，已推送 fork）
- 版本：`0.2.5.1`
- 合并基线：本地 `25d1cc149`，上游 `efe9aab1e`；冲突 24 个文件，解决说明见合并提交 `39a82f881`。

## 同一镜像

- 镜像：`sub2api-local:0.2.5.1-66162d981c7b`
- Image ID：`sha256:16e6e530f0229eee399da65b8b44d2a92d48a7fdccd96a837cf8c674cd20e253`
- 平台：`linux/amd64`，由已提交源码快照构建，二进制自报版本 `0.2.5.1`。
- 归档 SHA-256 `8e8898a9cbed74e0b89e5f0dd637239b47401d243569f9f17af81382edc9344f`，上传后载入生产机 Image ID 一致，生产机未重建镜像。

## 迁移

- 生产新增 5 个：`238_opencode_go_platform`、`238_purge_unlimited_user_platform_quotas`、`242/243/244_liandong_browser_*`。
- 本地新增 2 个 238（242–244 此前已应用）。
- 两个 238 文件号低于生产已应用的 241；迁移执行按文件名遍历未应用项，替换后迁移清单与候选声明精确一致。

## 验证结果

- 合并后检查：后端 `go build`、`go vet`、handler/server/service/cmd 单元测试通过；前端 vue-tsc、eslint 通过，2521/2524 测试通过，3 项 `CostCenterPages.spec.ts` 失败在合并前的 main 上同样存在。
- 发布脚本离线测试 28 项通过。
- [本地隔离回滚验证](acceptance/dev-rollback-proof.json)、[生产隔离回滚验证](acceptance/prod-rollback-proof.json)：空库内新版启动、迁移、管理 API 与回滚到旧镜像全部通过，临时资源清理通过。
- [本地功能核验](acceptance/dev-functional-2.json)、[生产功能核验](acceptance/prod-functional.json)：健康、版本、后台权限、商城配置、补货配置、页面外壳与真实 `gpt-5.6-luna` 流式 `response.completed` 均通过。首次本地核验 [dev-functional.json](acceptance/dev-functional.json) 在模型请求阶段失败，手工复测同一请求成功，判为上游瞬时失败，复跑通过；失败记录保留。

## 切换

- 本地：维护公告 5，备份 `/Users/vsiyo/.local/share/sub2api/backups/20260918-0.2.5.1-66162d98-dev`，恢复公告 6。
- 生产：维护公告 23，备份 `/home/ubuntu/sub2api-backups/20260918-0.2.5.1-66162d98`，恢复公告 24。
- 两侧均只重建 `sub2api` 单个容器，运行时环境、挂载与端口不变，切换后 `running / healthy`。

## 清理与保留

- 删除本地旧镜像 `sub2api-local:0.2.4.9-d2966e82874a`（无容器引用），清理本地 `update_check_cache`。
- 保留生产回滚镜像 `sub2api-local:0.2.4.4-507a820c6792`、两侧最新备份与全部证据。

## 边界

生产补货 enabled=true、产品数 0，公开目录行为与发布前一致；本次未导入库存、未生成兑换码、未启用真实商户提交。技术核验通过不等于人工验收或开售决定。
