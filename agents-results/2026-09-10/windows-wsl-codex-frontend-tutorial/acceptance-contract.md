# Acceptance Contract: windows-wsl-codex-frontend-tutorial

- Task ID: windows-wsl-codex-frontend-tutorial
- Contract version: 1
- Contract status: DRAFT
- Test baseline: PLANNED
- Acceptance owner: Codex
- Approval evidence: 2026-09-10 用户请求中的十五项教程内容和验收要求
- Request source: 2026-09-10 用户要求新增 Windows 11 WSL Codex 前端开发实操教程并完成本地 HMR
- SSOT node: none
- SSOT path: none
- Readiness mode: FORMAL
- Decision refs: none
- Assumption IDs: none
- Invalidation keys: frontend.tutorial.windows-wsl, frontend.experiences.index, frontend.public-routes, frontend.prerender.experiences
- AC budget: 4
- Baseline identity: 20189b7348fa74020e066241890c240e0c28cad4 + task surface digest 7dbbdb86976a55b71e35b5e0feffeec3ec84d571881890209926f8561358a35d
- Product Context refs: user request
- Role Context refs: beginner Windows 11 and WSL2 frontend developer
- Resolved Surface Contract refs: frontend/src/views/public/WslCodexFrontendTutorialView.vue
- Screen Contract ref: desktop 1280x720 and mobile 390x844 browser states
- Visual Contract refs: user request sections 13 and 15; Figma file bG5roZJVC4F4IQwaL8oeOk page Experience - WSL Codex Tutorial
- UI Change declaration: agents-results/2026-09-10/windows-wsl-codex-frontend-tutorial/ui-change.json
- Human acceptance workspace: acceptance/human/2026-W37/2026-09-10-windows-wsl-codex-frontend-tutorial

## User and scenario

第一次在 Windows 11 的 WSL2 中使用 Codex CLI 的开发者，跟随页面在自己的真实前端仓库中完成调查、实现、测试、Review 和人工验收。

## Problem

现有经验体系缺少一篇从零开始的真实 WSL 操作教程。单纯介绍 Codex 功能或给出长篇 Markdown，不能帮助初学者区分 Shell 命令、Codex Prompt 及人和 Agent 的职责。

## Expected outcome

经验中心新增可直接访问的教程卡片和独立页面。页面用流程、Terminal、Prompt、Why、责任标签和最终练习组织内容；命令可复制；桌面和移动端均能使用。

## Non-goals

不修改后端业务、不发布生产、不替用户操作 Windows 或 WSL、不保证第三方仓库的安装和测试命令、不硬编码未来可能变化的登录界面。

## Normal path

用户从经验中心进入教程，依次确认 WSL、准备仓库、安装 Codex、调查代码库、建立 AGENTS.md、提交真实任务、让 Agent 启动和测试、执行 Review，最后完成人工验收与练习。

## Exception paths

复制权限不可用时提示手动选择文本；CLI 行为变化时引导用户核对 `--help`、当前斜杠菜单及官方 WSL 文档；项目缺少某种测试能力时由 Agent按仓库实际能力说明未执行项。

## Invariants

Shell 命令与 Codex Prompt 必须明显区分；人类验收不得被自动测试替代；教程不宣称已经操作用户的 WSL 或完成其项目任务。

## Data impact

仅新增公开前端内容、路由、预渲染页面、sitemap 入口、测试和验收文档，不写入业务数据库。

## Permissions

本地公开页面只读；复制按钮仅响应用户点击并写入当前浏览器剪贴板。

## Performance and reliability

新页面采用延迟路由加载。移动视口不得出现页面级横向溢出，代码块允许自身横向滚动。

## Acceptance criteria

| ID | Class | Lane | Requirement | Mode | Blocking |
| --- | --- | --- | --- | --- | --- |
| AC-01 | behavior | machine/static | 经验中心新增教程卡，公开路由、预渲染页面和 sitemap 均可解析且不破坏既有经验文章 | Automatic | Yes |
| AC-02 | behavior | machine/static | 页面完整呈现 10 个阶段、流程图、Terminal/Prompt/Why、AI/Human 标签、最终练习和官方 WSL 参考 | Automatic | Yes |
| AC-03 | behavior | machine/static | 复制成功与失败均有明确结果，TypeScript、ESLint 和聚焦测试通过 | Automatic | Yes |
| AC-04 | behavior | visual-fidelity | HMR 在桌面与 390x844 移动视口可访问，无页面级横向溢出或文字裁切，浅色层级清楚 | Automatic | Yes |

## Human acceptance

| ID | Summary | Checklist path | Required role | Blocking |
| --- | --- | --- | --- | --- |
| H-01 | 初学者是否能分清 Shell、Codex Prompt 及人和 Agent 的职责并跟随完成真实任务 | acceptance/human/2026-W37/2026-09-10-windows-wsl-codex-frontend-tutorial/checklist.md#h-01 | 产品负责人 | No |

## Protected acceptance tests

| Path | SHA-256 | Covers |
| --- | --- | --- |
| none | none | Executable baseline remains PLANNED until human approval |

## Requirements-test traceability

| Requirement | Verification | Evidence target | Mode | Blocking |
| --- | --- | --- | --- | --- |
| AC-01 | 经验目录及预渲染集成测试 | acceptance/machine/static/ | Automatic | Yes |
| AC-02 | 教程组件测试与浏览器 DOM 检查 | acceptance/machine/static/; acceptance/visual-fidelity/ | Automatic | Yes |
| AC-03 | 剪贴板组件测试、TypeScript、ESLint | acceptance/machine/static/ | Automatic | Yes |
| AC-04 | 固定 HMR 的桌面和移动浏览器检查 | acceptance/visual-fidelity/ | Automatic | Yes |
| H-01 | 产品负责人跟随页面判断教程是否清楚可用 | acceptance/human/2026-W37/2026-09-10-windows-wsl-codex-frontend-tutorial/checklist.md#h-01 | Human | No |

## Exploratory testing

从经验目录进入教程，复制第一条 Shell 命令，浏览首屏、长 Prompt 和最终练习；不执行页面中的安装命令，不登录或写入本地业务后端。

## Production monitoring and rollback

本任务只做本地 HMR，无生产监控和发布。回滚范围是本任务新增页面和入口，不涉及数据库。

## Risks and open decisions

浏览器中的本地 8080 保存了失效登录态时，全局应用初始化会产生 401 和预加载错误；它不来自教程组件。深色主题 CSS 已实现并通过静态检查，本轮没有可用的公开主题切换控件用于独立浏览器截图。最终阅读体验仍需人工验收。
