# 本地目录与公开说明交付

日期：2026-09-15。范围：本机目录文件、公开经验内容和 HMR 预览；未部署生产、未提交或推送。

## 本机目录

- 文件：`/Users/vsiyo/.codex/model-catalogs/sub2api.json`，0600 权限。
- 来源：当前用户配置的 `https://www.ai.rest2build.lol/models?client_version=0.154.0`，使用对应现有凭证；未打印凭证。
- 获取时间：2026-09-15T14:23:13Z。
- 共 19 个模型；完整模型指令和其他能力字段均保留，没有仅手写 Astra 字段。
- Astra：context_window 1050000，max_context_window 1050000，effective_context_window_percent 95，auto_compact_token_limit null。
- SHA-256：`83ee91dbd9f3f227cbd6712321117cfadfb72f82db6905be88f9e16ced68874b`。
- 当前桌面引擎 `/Applications/ChatGPT.app/Contents/Resources/codex` 已通过候选文件显式加载验证；未发送生成请求。
- 初次交付只生成目录，未修改 `config.toml`，也未重启当前任务。这不构成配置启用验收。后续已完成本机配置启用与真实短请求，见 [配置生效验证](config-activation.md)。

后续已写入的配置行（顶层、首个 section 前）：

```toml
model_catalog_json = "/Users/vsiyo/.codex/model-catalogs/sub2api.json"
```

## 网站内容与更新入口

- 新增 ERR-006，标题“已切换新模型，为什么 Codex 上下文仍然偏小？”。
- HMR 地址：`http://127.0.0.1:4174/error-experiences/codex-model-catalog-context-window`。
- 经验列表归类为“模型与用量”，已在 Vue 路由、公开预渲染目录和独立 HTML 索引登记。
- 公共内容源：`frontend/src/content/codexModelCatalogGuide.json`；Vue 与预渲染共用。
- 文档生成：`node tools/quality/render_codex_catalog_guide.mjs`；一致性检查增加 `--check`。
- 下载脚本：`frontend/public/downloads/sync-codex-model-catalog.py`。仅支持普通 API Key，Python 3.11+；不写用户配置、不生成模型输出、不自动刷新。
- 公开说明解释配置层、静态快照更新、不同 provider 隔离、备份/失败保留、重新加载、预算验证及回滚。
- 个人完整模型目录只放本机，没有放入公开站点或仓库。

## 已执行验证

- Python 同步脚本测试：7 项通过，覆盖路径前缀、跨域重定向拦截、无效目录、客户端拒绝、加载值不一致、完整字段与备份、默认比例。
- 前端相关测试：3 文件 / 14 项通过，包括经验列表、公开路由/内容登记和提示词复制成功/失败反馈。
- 定向 ESLint 通过；Vite 的 vue-tsc 检查报告 0 errors；独立 `pnpm typecheck`（固定 pnpm 9.15.9）退出码 0。
- Markdown 与 HTML 生成一致性检查通过；静态 HTML 的标题顺序、提示词一致性、本地链接和无私人路径检查通过。
- 实际下载响应的 SHA-256 与脚本源文件相同：`05d42fabc834379948c44c928e415b8a92519f056cfd68fc72901ed7cdf423e9`。
- Codex 内置浏览器：列表中的“模型与用量”筛选包含 3 张卡片，新文章可打开、复制提示词成功；桌面和 390px 手机视口无页面横向溢出，文章无控制台 error。
- 独立 HTML 的 file URL 被浏览器 URL 安全策略拦截，因此没有浏览器直接打开该文件；采用非执行的 HTML 结构与链接检查。网站 HMR 页面已实际渲染验证。

没有执行完整本地 CI、生产构建、Go 编译、部署或长输入压测。上述结果不等于人工验收或上游百万上下文可用。
