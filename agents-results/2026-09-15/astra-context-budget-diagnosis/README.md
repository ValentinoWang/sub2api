# Astra 上下文预算诊断

核验时间：2026-09-15 22:10 CST。性质：只读诊断记录，不是发布、人工验收或最大输入压测证据。

## 结论

当前任务选中 Astra，但报告 258,400 的原因是客户端使用内置 Astra 模型目录：`context_window=272000`，可用比例为 95%。当前 API Key 认证不满足这两个已安装 Codex 版本的远端目录刷新条件，Sub2API 已返回的 1,050,000 元数据没有进入客户端生效目录。

不是用户没有切模型；新建 Astra 任务也复现。不是当前生产模型目录把 Astra 宣告为 272K；使用现有认证读取实际配置入口，生产返回的是 1,050,000。这里不判断上游真实最大可接受输入，没有执行模型生成或大上下文请求。

## 本机与生产证据

- 用户配置：`/Users/vsiyo/.codex/config.toml`，默认 `gpt-6-astra`，provider `sub2api`，Base URL `https://www.ai.rest2build.lol`，`wire_api=responses`，`requires_openai_auth=true`。
- 用户配置未设置 `model_context_window`、`model_auto_compact_token_limit` 或 `model_catalog_json`。所查仓库及祖先项目配置、系统配置路径未发现对应文件。
- 当前认证文件只包含 `OPENAI_API_KEY`；未记录任何密钥值。本机当前没有 `models_cache.json`，因此不能归结为这个文件中的过期缓存。
- 桌面运行引擎：`/Applications/ChatGPT.app/Contents/Resources/codex`，版本 `0.154.0-alpha.6.2`。
- 全局 CLI：`/Users/vsiyo/Desktop/Opensource_Tool/npm-global/bin/codex`，版本 `0.154.0`。
- 桌面 app-server 启动参数没有窗口覆盖；只有已存在的 code-mode 和插件参数。

两份二进制各自执行 `debug models`（未加 `--bundled`）都返回：

```json
{"slug":"gpt-6-astra","context_window":272000,"max_context_window":872000,"effective_context_window_percent":95,"auto_compact_token_limit":null}
```

二进制内置目录中的 Astra 也对应上述默认窗口、最大窗口。可用预算为 `272000 * 95 / 100 = 258400`。这不是自动压缩阈值；所核对版本在未显式设置压缩阈值时按默认窗口的 90% 推导，即 244,800。

使用现有认证分别读取以下生产目录，均返回 HTTP 200：

- `https://www.ai.rest2build.lol/models?client_version=0.154.0-alpha.6.2`
- `https://www.ai.rest2build.lol/models?client_version=0.154.0`

两次响应的 ETag 相同：`6c269deb14c73c7f75b89c3407718aa3932c7e73618bd57d26f16d0b9852f7fd`。

| 来源 | 模型 | context_window | max_context_window | effective_context_window_percent |
| --- | --- | ---: | ---: | ---: |
| 生产模型目录 | gpt-6-astra | 1050000 | 1050000 | 95 |
| 生产模型目录 | gpt-5.6-terra | 272000 | 872000 | 95 |
| 桌面实际目录 | gpt-6-astra | 272000 | 872000 | 95 |
| 全局 CLI 实际目录 | gpt-6-astra | 272000 | 872000 | 95 |

任务记录的此前核验：`/Users/vsiyo/.codex/sessions/2026/09/15/rollout-2026-09-15T21-59-38-01a0a55d-bac4-7631-8f79-907303b6dc79.jsonl` 第 8 行为 Astra / xhigh；第 72 行报告 258400。另外 22:01 新建的 Astra 任务也报告相同窗口。

## 隔离诊断

1. 在动态 loopback 端口启动临时模型目录服务，响应完整内置目录，但将 Astra 的两个窗口字段设为 1,050,000。控制 GET 成功到达一次。
2. 仅对诊断子进程覆盖 provider Base URL，执行桌面引擎 `debug models`。进程正常退出；临时服务收到的 Codex 请求数为 **0**，结果仍是 272000 / 872000 / 95。临时服务随后关闭；没有保存或打印认证请求头。
3. 再次读取生产目录，将完整 JSON 通过 stdin 传给同一桌面引擎，诊断进程参数为 `-c model_catalog_json="/dev/stdin" debug models`。进程正常退出，Astra 返回 **1050000 / 1050000 / 95**。

上述操作没有更改用户配置、模型缓存或任何运行任务，没有调用 Responses 生成，没有重启服务。显式目录检查只证明客户端能加载元数据，不证明实际接受百万上下文。

## 源码闭环

核对 `openai/codex` 的已安装 CLI 对应标签 `rust-v0.154.0`：

- [models-manager/src/manager.rs](https://github.com/openai/codex/blob/rust-v0.154.0/codex-rs/models-manager/src/manager.rs)：先载入内置目录；`should_refresh_models` 仅在 `uses_codex_backend()` 或 `has_command_auth()` 成立时允许网络刷新；不成立则尝试缓存并返回。
- [model-provider/src/models_endpoint.rs](https://github.com/openai/codex/blob/rust-v0.154.0/codex-rs/model-provider/src/models_endpoint.rs)：上述条件来自实际认证模式和 provider 的命令认证配置。
- [protocol/src/auth.rs](https://github.com/openai/codex/blob/rust-v0.154.0/codex-rs/protocol/src/auth.rs)：`ApiKey` 的 `uses_codex_backend()` 为 false。`requires_openai_auth=true` 不等于 ChatGPT 认证。
- [model-provider-info/src/lib.rs](https://github.com/openai/codex/blob/rust-v0.154.0/codex-rs/model-provider-info/src/lib.rs)：`has_command_auth` 检查 `self.auth.is_some()`。现有 provider 未配置命令认证。
- [cli/src/main.rs](https://github.com/openai/codex/blob/rust-v0.154.0/codex-rs/cli/src/main.rs)：普通 `debug models` 使用 `OnlineIfUncached` 刷新策略，不是主动强制使用内置目录。
- [protocol/src/openai_models.rs](https://github.com/openai/codex/blob/rust-v0.154.0/codex-rs/protocol/src/openai_models.rs)：可用窗口按 context_window × effective_context_window_percent 计算。
- [models-manager/src/model_info.rs](https://github.com/openai/codex/blob/rust-v0.154.0/codex-rs/models-manager/src/model_info.rs)：单独覆盖 `model_context_window` 仍会受到目录 `max_context_window` 限制。

该标签源码用于解释实现；桌面 alpha 版本的行为由实际二进制目录与隔离诊断独立确认。

本地 Sub2API 源码 `backend/internal/service/openai_codex_models_service.go` 为 Astra 设置了 1,050,000，与本次生产目录读回吻合。`backend/internal/server/routes/gateway.go` 根据 `client_version` 查询参数进入 Codex 完整目录处理，不应以不带该参数的普通模型 ID 列表替代核验。

## 修复方向与边界

应解决客户端在 API Key 模式下采用正确模型能力的问题，可以使用已验证可解析的显式模型目录机制，或修正/更新客户端远端目录刷新策略。仅修改网关相同字段、清除不存在的缓存、重复切模型或新建任务，都没有消除本次根因。

不应只强写 `model_context_window=1050000`：当前内置最大字段仍是 872000，且真实上游接受范围、最大输入与输出预留需要分别核对。根据已核对官方 API 文档，Astra 总上下文为 1,050,000、最大输入 922,000；读取这些元数据不等于已经验证当前接入通道的长输入能力。显式目录按 95% 推导的 997,500 也不应被当成已经验证可发送的输入长度。

本次仅完成诊断，没有实施修复。
