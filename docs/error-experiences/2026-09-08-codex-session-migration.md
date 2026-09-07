# ERR-002 · 切换接入后，为什么 Codex 旧对话无法继续？

记录时间：2026-09-08（Asia/Shanghai）
适用：Codex 桌面端与命令行使用者；已支持的本地 JSONL 会话记录和 SQLite 索引。

## 情况说明

切换官方登录、其他中转站或本地转发后，旧任务仍可能保存已删除的 provider 名称。重新打开或继续它时，Codex 会提示 `Model provider ... not found`，即使当前 `config.toml` 已经可以连接新入口。

这不是“网页记忆丢失”的证明。需要分开看本机任务记录、Codex memories、ChatGPT 网页历史、服务端 Redis 连接粘性，以及正在进行的连接或工具调用。离线工具只处理已支持的本机任务关联，不上传私人历史，也不修改配置或认证。

## Codex 帮你处理

下载站内提示词或将下列要求交给 Codex：先识别有效数据目录与实际配置，做只读诊断，给出准确任务范围和源到目标映射；只有计划已审阅、目标无歧义、所有使用同一目录的 Codex 已关闭，并且全部备份已读回验证时，才允许从外部终端执行离线工具。

[下载离线工具包](/codex-session-migrate-1.0.0.zip) · [下载提示词](/codex-session-migrate-prompt.txt) · [查看校验清单](/codex-session-migrate-manifest.json)

工具命令为 `inspect`、`plan`、`apply`、`verify`、`recover`、`rollback`。默认是只读诊断；`plan` 需要显式选择任务或全量历史；`apply` 需要确认计划摘要和客户端已关闭。备份失败、配置冲突、未知格式、输入漂移或新消息出现时停止写入。

## 给人看的：原因、证据与经验

provider 名称、Base URL 和 API Key/登录凭据是不同概念。教程中看到 `OpenAI` 或 `sub2api` 并不代表所有用户的有效配置都应改成同一个名称。工具从实际生效的配置解析目标；自定义 provider 名称按大小写匹配，内置 `openai` 另行识别。

JSONL 与 SQLite 不是一个跨文件原子事务。工具在原文件同一文件系统内替换会话记录，在每个数据库中使用事务更新已选行，并写入持久阶段日志。中断后只能根据日志、前态和后态恢复；任何不认识的新变化都会拒绝覆盖。回滚仅恢复选中任务及其关联行，不能用整库副本抹掉之后新增的任务数据。

结构验证成功不表示使用者已经真正找回对话接续。完成后需重新打开同一个旧任务，引用迁移前已存的测试事实并追加一轮交流。新建任务、读取 SQLite 或模型目录出现目标名称都不能替代这个检查。

## 适用边界

首版要求 Python 3.11 或更高版本，支持含 `session_meta.payload.model_provider` 的 JSONL 元数据，及具有已登记会话 ID 与 provider 列的 SQLite 索引。历史缺失、损坏、多个有效数据根、未知 schema 或目标未定义时，工具保留在只读诊断阶段。

不要把本工具用于清空整个 `.codex` 目录、删除 memories、重写认证、迁移 ChatGPT 网页历史，或接续正在进行的连接和工具调用。

---

**rest2build**

**歇一会儿，让 AI 接着干。**

rest 是你的，build 交给 AI。

rest2build 提供面向 Codex、Claude Code 等工具的 AI 模型接入服务。同时围绕公益 Skills、AI 使用经验分享与 Harness 工程，持续开展内容与实践。
