# 错误经验

解决你使用codex或者claudecode的最后一公里

面向中转站使用者的客户端接入与排障帮助。以完整问题登记可复用的错误经验模块，每个模块对应一个编号、一张卡片和一份案例文档，说明如何识别问题、交给 Codex 处理、自查恢复结果及联系管理员。

当前登记 6 个模块。每个模块先说明问题，再给出解决方案和可直接交给 Codex 的提示词。

经验首页最上方提供统一排障指令：用户复制后发给能读取网页并操作本机的 Codex，可选补充一句症状。指令直接引用同站 `/experience-reference.md` 完整经验文档，包含经验栏目所有条目的适用范围、更新时间、公开完整正文与案例提示词；一次读取后完成相关诊断、修复与验证，不让用户逐篇查找。文末标记 `EXPERIENCE_DOCUMENT_END` 可帮助检查读取是否完整。`/experience-reference.json` 提供同源结构化内容作为备用。缺少症状时先询问，不批量套用全部修复。

完整文档和参考包由 `frontend/prerender.config.ts` 的 `experienceReference` 插件生成：HMR 直接提供 Markdown 和 JSON，生产构建输出相同路径的静态资源。条目来自 `frontend/src/content/experiences.ts`，全文来自对应公开预渲染页面，新增经验自动纳入；缺失正文会阻止生成。统一指令来自 `frontend/src/content/experienceHelp.ts`，链接按页面当前域名解析，不包含用户凭证。复制按钮只复制文本，不在网站执行本机修改。

- [HTML 卡片预览](index.html)
- [ERR-006 · 已切换新模型，为什么 Codex 上下文仍然偏小？](2026-09-15-codex-model-catalog-context-window.md) · [HTML 预览](2026-09-15-codex-model-catalog-context-window.html)
- [模型目录显示规则：别名、入口绑定与恢复](codex-catalog-picker-rules.md)
- [ERR-001 · 中转站已有新模型，为什么 Codex 看不到？](2026-09-06-codex-astra-visibility.md)
- [ERR-002 · 切换接入后，为什么 Codex 旧对话无法继续？](2026-09-08-codex-session-migration.md)
- [ERR-003 · Windows 11 + WSL2 里 Codex 装好了却不能正常使用，怎么排查？](2026-09-12-windows-wsl2-codex-environment.md) · [HTML 预览](2026-09-12-windows-wsl2-codex-environment.html)
- [ERR-004 · Claude Code 每次执行都要确认，怎样默认跳过？](2026-09-13-claude-code-bypass-permissions.md) · [HTML 预览](2026-09-13-claude-code-bypass-permissions.html)
- [ERR-005 · 中转站已有新模型，为什么 Claude Code 看不到？](2026-09-13-claude-code-fable-5-1-not-visible.md) · [HTML 预览](2026-09-13-claude-code-fable-5-1-not-visible.html)

本目录是项目经验文档与独立静态预览，不是生产发布或人工验收记录。

新增案例应保留编号、文章更新时间、适用客户端、历史证据来源与未验证边界。正文不是内部开发日记：不展示聊天引语、私人验收反馈、对话时间线、作者的内部配置与操作流水账。不要保存密钥、账户标识或私有对话原文。

写作顺序固定为：**问题说明 → 解决方案（含可直接发送给 Codex 的提示词）→ 原因、验证与注意事项**。完整复盘结构置于第三部分，不得让读者先理解长文才能使用提示词。

教程依赖脚本、工具包或校验清单时，下载链接必须同时出现在处理区和完整提示词中，不能只写“使用本文脚本”。站内资源在内容源与页面链接中使用根相对路径；网页中的提示词通过 `formatTutorialPrompt` 根据当前站点解析为完整资源地址并附教程来源，复制按钮与文本框保持一致。不得写死本地端口或生产域名，不使用模型 API Base URL 代替教程来源，也不向公开资源下载请求附带 API Key。修改后检查本地、生产地址解析、资源文件存在及下载提示词与源文件一致。

写作 Skill：[sub2api-error-experience](../../skills/sub2api-error-experience/SKILL.md)，可通过 `$sub2api-error-experience` 调用。项目自动发现入口为 `.agents/skills/sub2api-error-experience`。

对外品牌使用 `rest2build`。公司标语与传播口号已由用户于 2026-09-06 定稿：“歇一会儿，让 AI 接着干。”与“rest 是你的，build 交给 AI。”。帮助页最底部用全宽独立品牌主视觉突出展示，搭配现有 logo 和站点链接 `https://ai.rest2build.lol/`；这项约定不自动授权修改登录页或部署。

完整业务介绍原文：“rest2build 提供面向 Codex、Claude Code 等工具的 AI 模型接入服务。同时围绕公益 Skills、AI 使用经验分享与 Harness 工程，持续开展内容与实践。”。保留为可见文字，明确 GEO 所需的品牌、工具、服务与内容方向关联，不改写为“未来还将”，也不扩写具体功能完成或效果保证。
