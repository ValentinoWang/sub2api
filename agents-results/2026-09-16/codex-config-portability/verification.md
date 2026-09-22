# Codex 配置示例的系统与环境适配

范围：使用密钥弹窗的 Codex 示例与本地 Vite 代理。未修改用户级 Codex 配置或生产部署。

- OpenAI 普通与 WebSocket 示例共用生成函数，模型和 review_model 默认使用 gpt-6-astra，并保留目录支持范围回退。
- 移除旧顶层 disable_response_storage、network_access，以及预先确认 Windows 引导的 windows_wsl_setup_acknowledged；不替用户选择沙箱权限或确认系统初始化。移除旧 responses_websockets_v2 功能标志，使用 provider 的 supports_websockets 区分 HTTP/SSE 与 WS。
- macOS/Linux 文件标签为 ~/.codex/config.toml、~/.codex/auth.json；Windows 原生标签使用 Windows 分隔符，修复原先混用正反斜杠的问题。WSL 使用 Linux 配置路径，页面分别说明。
- Codex 示例和模型目录请求共用 buildCodexBaseUrl。明确 API 地址优先，否则使用当前网页域名；补齐一次 /v1，保留已有路径前缀，不写死生产域名或本地端口。
- Vite 的 /v1 代理开启 WebSocket。配置文件仍需真实 API Key 或对应认证文件；匿名代理检查仅用于确认请求到达后端。

验证：34 项测试通过，包含多系统、多地址、HTTP/WS、两种认证方式的既有覆盖；ESLint 与 git diff --check 通过。HTTP 模型目录、Responses 请求和 WebSocket 握手分别经过直连 8080 与代理 4174，均到达后端并返回预期 401，未得到前端 HTML。未发起收费模型生成。

依据：OpenAI 官方配置参考将 windows_wsl_setup_acknowledged 定义为 Windows 引导状态；network_access 的有效沙箱配置位于 sandbox_workspace_write.network_access，不能用旧顶层字符串代替。goals 是跨平台功能，不属于系统专用设置。

本地 HMR 地址生成的配置依赖本地预览服务运行；复制完成的静态文件不会随网页域名自动改写。生产环境地址适配由生成测试覆盖，没有在生产修改配置或执行请求。
