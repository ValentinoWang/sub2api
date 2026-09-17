# Acceptance Contract: local-8080-version-sync

- Task ID: local-8080-version-sync
- Contract version: 1
- Contract status: DRAFT
- Test baseline: PLANNED
- Acceptance owner: Codex
- Approval evidence: 本次用户请求授权本地更新；人工清单待审阅
- Request source: 2026-09-10 用户要求同步本地 8080 到当前版本并清理缓存
- SSOT node: none
- SSOT path: none
- Readiness mode: FORMAL
- Decision refs: none
- Assumption IDs: none
- Invalidation keys: local-8080.runtime
- AC budget: 2
- Baseline identity: 20189b7348fa74020e066241890c240e0c28cad4 + local build input digest 1b221364260bb98954cef10d9c289487f4707f3314a1925146b044ba2d7f1045
- Product Context refs: none
- Role Context refs: none
- Resolved Surface Contract refs: none
- Screen Contract ref: none
- Visual Contract refs: none
- UI Change declaration: none
- Human acceptance workspace: acceptance/human/2026-W37/2026-09-10-local-8080-version-sync

## User and scenario

使用者在本机 8080 查看当前已开发的页面。本任务安装既有源码，不继续修改界面实现。

## Problem

本机仍运行旧应用，无法看到当前页面和后端能力。

## Expected outcome

本地页面提供当前构建内容，已有业务数据保持可用，旧页面缓存不阻止读取新资源。

## Non-goals

不更新远端生产、不推送 Git、不实现模型实测展示提案、不修改订阅及订单状态。

## Normal path

备份和预检后构建当前工作区，替换本地应用，检查页面、资源、迁移和数据，再定向清理可重建缓存。

## Exception paths

构建失败保留旧服务。切换失败保留证据并恢复旧镜像；数据库恢复须考虑切换后的写入，不能直接覆盖。浏览器无法定向清理时明确记录实际清理范围。

## Invariants

保留数据库、Redis、代理服务及数据卷；保持数据库凭据、JWT 与加密配置；不清空 Redis，不删除账号和业务数据。

## Data impact

自动执行当前代码尚未应用的数据库迁移。备份保存在仓库外私有目录，机器证据仅记录统计和摘要。允许删除可重建前端缓存。

## Permissions

用户已授权本地应用更新和缓存清理。此授权不涵盖生产业务写入。

## Performance and reliability

切换后连续五次本地健康请求成功，关键公开页面和新引用资源可读取；不得出现启动失败或迁移校验错误。

## Acceptance criteria

| ID | Class | Lane | Requirement | Mode | Blocking |
| --- | --- | --- | --- | --- | --- |
| AC-01 | behavior | machine/local-runtime | 当前本地页面及其引用资源可读取，服务重复访问成功；实际缓存处理有可核对记录 | Automatic | Yes |
| AC-02 | behavior | machine/local-runtime | 数据迁移成功，用户、账号、密钥、分组、代理与订单的数量和选定身份字段保持一致 | Automatic | Yes |

容器身份、构建版本与哈希仅为本地部署来源记录，不冒充线上发布证据。

## Human acceptance

| ID | Summary | Checklist path | Required role | Blocking |
| --- | --- | --- | --- | --- |
| H-01 | 当前页面是否符合使用者之前确认的内容与阅读体验 | acceptance/human/2026-W37/2026-09-10-local-8080-version-sync/checklist.md#h-01 | 产品负责人 | No |

人工复核不阻止完成本地安装；不得由机器结果代签人工通过。

## Protected acceptance tests

none；本次为本地运维验证，无新增业务测试基线。

## Requirements-test traceability

| Requirement | Verification | Evidence target | Mode | Blocking |
| --- | --- | --- | --- | --- |
| AC-01 | 本地健康、静态页面和浏览器检查 | acceptance/machine/local-runtime/ | Automatic | Yes |
| AC-02 | 更新前后数据库摘要与迁移记录比较 | acceptance/machine/local-runtime/ | Automatic | Yes |
| H-01 | 阅读页面并判断是否符合预期 | acceptance/human/2026-W37/2026-09-10-local-8080-version-sync/checklist.md#h-01 | Human | No |

## Exploratory testing

查看首页、经验中心、公益申请、企业服务和登录入口；不提交真实业务操作。

## Production monitoring and rollback

仅针对本地安装。旧镜像及数据库备份保留，启动错误或数据损失触发停止切换和恢复调查。远端生产不在本任务内。

## Risks and open decisions

完整工具包要求 linux/amd64，本机通过 Docker 架构兼容运行；不把本地健康结果等同于付费模型请求成功。人工界面复核仍待使用者执行。
