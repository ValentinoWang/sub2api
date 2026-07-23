# 现状依据

## 1. 结论

本文件记录规划时采用的现状依据及实施后的复核结果，不是新的产品事实来源。

- Codex 的持久记忆位于用户电脑的 `~/.codex/memories/`。
- Codex 的活动任务和归档任务位于 `~/.codex/sessions/` 与 `~/.codex/archived_sessions/`。
- 选择不同的 `model_provider` 不应改变上述本地状态的归属。
- Sub2API Redis 只承担临时路由亲和与连接租约，不是 Codex 持久记忆库。
- 认证文件、OAuth 令牌和 API 密钥不属于可统一的记忆数据。
- 正在进行的流式响应、工具调用和 WebSocket 连接不能迁移。

## 2. 已有服务端能力

当前仓库已经存在服务端请求连续性纵切：

| 能力 | 权威路径 | 当前含义 |
|---|---|---|
| 连续性业务逻辑 | `backend/internal/service/openai_continuity.go` | 在请求具备明确任务身份且命中灰度条件时，恢复已完成请求的输入序列 |
| 连续性持久化 | `backend/internal/repository/openai_continuity_repo.go` | 按 Sub2API 用户、API 密钥和任务身份隔离读取与提交 |
| 数据结构 | `backend/migrations/183_codex_continuity.sql` | PostgreSQL 保存加密后的已完成轮次和校验摘要 |
| 配置 | `backend/internal/config/config.go`、`deploy/config.example.yaml` | 默认关闭，可按 API 密钥灰度，设置保留期和最大持久化大小 |
| 现有说明 | `docs/CODEX_OAUTH_MEMORY_CONTINUITY_MACRO_CN.md` | 说明服务端连续性边界及后续本地导入方向 |

已有能力只处理经过 Sub2API 的新请求连续性。它不读取用户电脑上的 Codex 记忆目录，也不等于本 SSOT 所规划的本地记忆统一工具。

## 3. 初始缺口与当前结果

1. 初始没有面向外部用户的 Codex 记忆统一工具下载入口；当前 fork 已实现 `/docs` 与 `/docs/codex-memory`，正式可下载状态仍取决于 Release 和部署。
2. 当前 `/Api_subscribe` 由 `frontend/src/router/index.ts` 定义为管理员路由，并加载 `frontend/src/views/admin/AccountSubscriptionsView.vue`，不能作为未登录用户的公开文档页。
3. 当前 fork 文档已形成从下载、校验、审计、快照、合并到恢复的完整外部用户流程。
4. 本地历史任务导入服务端、用户自助导出与删除、大型图片对象化仍未实现；这些不属于第一版本地工具。
5. `/docs`、`/docs/codex-memory`、三平台脚本构建器和发布流水线已实现；正式 Release 与站点部署仍需完成。

## 3.1 已确认的产品决定

2026-07-23，产品负责人明确：

- 外部文档必须是用户打开 Sub2API 网站时可见的站内公开文档。公开文档中心使用 `/docs`，Codex 记忆统一工具详情页使用 `/docs/codex-memory`，并由未登录首屏或全局导航提供入口。Obsidian 与手机提醒仅用于内部开发审计，不属于外部用户文档或运行依赖。
- 第一版同时支持 macOS、Windows、Linux，并以便于 Codex 调用的脚本/命令行工具交付。
- 第一版只统一同一台电脑的本地状态，不包含服务端历史导入。
- 工具在提醒、预检、快照和明确确认后自动设置统一 `CODEX_HOME` 并迁移全部可迁移状态；凭据始终排除，`config.toml` 只做结构化必要修改。
- 发布权威为 GitHub Releases 的不可变制品、校验文件和单一发布清单，Sub2API Docs 读取该清单。
- 差异任务都保留并标注来源；只有任务身份和内容摘要均一致时去重；不自动覆盖。

当前第一版本地工具已无待拍板产品问题。Codex Memory 功能代码和通用文档源共同保存在本项目团队维护的 Sub2API fork 中，网站渲染文档并叠加部署实例信息；原始父仓库不是当前依赖。多账户合并前自动创建本地备份，默认不覆盖或删除来源数据，只生成合并结果，并提供恢复命令。服务端托管的数据治理问题移入范围外未来备注，不阻塞当前工具；原 OP-08 继续保持既有默认关闭/按 API 密钥灰度。

## 4. 证据范围

初始规划基于 2026-07-23 的工作树进行只读核对。后续实施证据以本 bundle 的 D1、D2 和完成审计为准；生产运行状态和正式 Release 必须单独以线上验证证明。
