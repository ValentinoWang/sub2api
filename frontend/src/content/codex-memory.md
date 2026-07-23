# Codex 本地记忆统一工具

Codex Memory CLI 用于把同一台电脑上由不同登录方式或 `model_provider` 使用的本地状态合并到统一的 `CODEX_HOME`。它支持 macOS、Windows 和 Linux，并适合由 Codex 直接调用。

站内公开文档入口为 `/docs/codex-memory`。可下载版本只以本项目 GitHub Releases 的发布清单、平台压缩包和 SHA-256 校验文件为准。

## 数据边界

工具只处理以下目录：

- `$CODEX_HOME/memories/`
- `$CODEX_HOME/sessions/`
- `$CODEX_HOME/archived_sessions/`

工具会解析并校验目标 `config.toml` 的 SHA-256，但不会通过合并命令改写它。以下内容永不参与合并：

- `auth.json`、OAuth 令牌、API Key 和系统钥匙串；
- Sub2API Redis 或 PostgreSQL 数据；
- 正在执行的流式响应、工具调用或 WebSocket 连接；
- 另一台电脑或服务端未明确导出的私有历史。

切换 provider 不会改变本地记忆的所有权。开始合并、恢复或切换 `CODEX_HOME` 前，必须先结束活动请求并退出 Codex；完成后重启 Codex。

## 合并规则

1. `plan` 只读扫描来源状态、校验 JSONL 和普通文件，并生成合并预览。
2. 仅当任务身份和 SHA-256 同时一致时去重。
3. 身份相同但内容不同的任务全部保留，并在文件名和 provenance 中标记来源。
4. `merge` 需要同时确认写入和“没有活动请求”，先创建本地备份，再写入目标。
5. 来源 `CODEX_HOME` 默认不修改、不删除；目标 `config.toml` 保持原样。
6. `restore` 在恢复前再创建一次安全备份，并校验原备份内的每个文件。

## 与 Sub2API 连续性的区别

本工具只统一用户电脑上的 Codex 持久状态。Sub2API 的 Responses 连续性负责服务端请求在上游账户切换时的短期或持久续接，两者没有数据迁移关系。

工具源码、测试、清单模式和发布脚本位于 `tools/codex-memory-unifier/`。通用文档与功能代码共同在本项目维护的 fork 中演进，原始 Sub2API 父仓库不是该功能的运行依赖。
