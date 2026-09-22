import { CODEX_SESSION_MIGRATION } from '../constants/codexMigration'
import { PUBLIC_PAGES } from '../constants/brand'
import codexModelCatalogGuide from './codexModelCatalogGuide.json'

export type ExperienceCategory =
  | 'connectionConfiguration'
  | 'conversationContinuity'
  | 'modelsUsage'
  | 'troubleshooting'

export interface ExperienceCategoryDefinition {
  id: ExperienceCategory
  labelKey: string
}

export type ExperienceIcon = 'chat' | 'link' | 'sparkles' | 'sync' | 'terminal'

export interface ExperienceContent {
  id: string
  category: ExperienceCategory
  route: string
  routeName: string
  icon: ExperienceIcon
  title: string
  summary: string
  series: string
  subtitle: string
  applicableTo: string
  updatedAt: string
  prompt: string
}

export const experienceCategories: ExperienceCategoryDefinition[] = [
  { id: 'connectionConfiguration', labelKey: 'experiences.categories.connectionConfiguration' },
  { id: 'conversationContinuity', labelKey: 'experiences.categories.conversationContinuity' },
  { id: 'modelsUsage', labelKey: 'experiences.categories.modelsUsage' },
  { id: 'troubleshooting', labelKey: 'experiences.categories.troubleshooting' },
]

const gpt6AstraNotVisiblePrompt = `我使用 Sub2API 中转站接入 Codex，目标模型是 gpt-6-astra。现在可能遇到以下情况之一：桌面模型列表没有它；终端更新后桌面仍没变化；或者已经可选，但原任务还在使用旧模型。请先确认我实际遇到哪一种，再定位并处理，不要直接假定是客户端旧版本、缓存或站点故障。

我的身份是普通中转站使用者，只有自己的客户端、本站提供的接入地址和个人 API Key，没有站点服务器管理权限。请使用当前机器已有的授权配置；缺少必要信息时问我，不让我把完整 API Key、认证文件或私人对话贴到回复里。

第一步：检查实际使用环境。
确认系统、桌面应用来源与安装路径、实际运行的 Codex 引擎，以及终端 codex 的版本。区分桌面内置引擎与全局 CLI，不把终端升级等同于桌面升级。若当前环境无法访问我的桌面机器，只报告你能确认的内容，并给出需要在那台机器检查的最小项目。

第二步：核对实际连接和模型选择。
只读取必要的配置字段，说明 provider、Base URL、默认 model、命名配置及启动覆盖的关系，并检查当前任务或新任务实际选择的模型。Base URL 要与本站接入说明核对，不能自行猜测域名、补删 /v1 或改成 localhost。如果现有配置看起来走了本地代理，先确认是否为我有意设置。保留现有 provider、features、memories、profiles、MCP、项目 trust、认证和任务历史。

第三步：分开验证服务和客户端。
用我已有的认证访问实际配置入口的模型目录，判断 gpt-6-astra 是否对我可用；按该入口的协议与路径发请求，不重复拼接 /v1。若有必要，允许针对我正在使用的入口做一次极短的 Responses 生成请求，可能产生少量费用，不循环重试、不探测无关入口。记录状态、脱敏错误、请求标识和返回模型字段；目录中有模型不等于生成成功，返回模型名也不是独立的上游身份认证。
再检查当前版本可用的客户端模型目录或 model/list，区分“服务可调用”“客户端可见”“任务选中”。2026-09-06 的历史案例中，桌面内置 0.153.3 将 Astra 标记为 hide，0.153.4 恢复 list；这只是排查线索，需对照我的实际版本和当前官方说明，不能强行降级到历史版本。

第四步：只修复已定位的问题。
可以备份后修改与本问题直接相关、可恢复的本机客户端配置；连接目标有歧义时先问我。需要桌面升级时，优先使用官方更新入口或完整安装包，核对来源与签名，并在活跃任务和工具调用结束后安装、重启。不要在运行中替换 App，不要把全局 CLI 手工塞进签名应用。
缓存不是默认清理项：先确认该版本实际读取什么、缓存是否相关及如何恢复，只有明确相关的可再生对象才定点清理。不要删除整个 Codex 目录、memories、任务历史、认证或 Docker 数据。不要登录、重建、重启本站服务器，不更改站点账号、密钥或上游路由。遇到权限或安全策略阻塞时说明边界，不换命令绕过。

最后，请用普通使用者能看懂的语言分别报告：
1. 我的客户端实际版本与连接入口是否正确；
2. gpt-6-astra 是否在桌面可见；
3. 新任务或当前任务是否真的选中目标模型；
4. 最小请求是否成功，哪些检查没有执行；
5. 若未恢复，是我本机可继续处理，还是需要联系站点管理员。
不要把配置文件写好、安装包下载完成或模型目录有名字当作修复完成。如果必须由我结束当前任务或重启应用，先给出准备情况、恢复方式和重启后的检查项。`

export const wsl2CodexEnvironmentPrompt = `我在 Windows 11 + WSL2 中使用 Codex CLI，现在遇到以下一种或多种情况：终端里找不到 codex；运行到的版本或安装位置不对；同一条命令在 PowerShell 和 Ubuntu 中结果不同；项目位于 /mnt/c 后速度慢、权限或软链接异常；Windows 能访问网络，但 WSL 中 Codex 登录或请求失败。我的目标是先判断问题属于 Windows、WSL2、Codex 安装、工作目录还是网络配置，再做最小修复，让 Codex 能从 WSL 中稳定启动并完成一次最小交互。

请把我当作普通使用者。我可以操作自己的 Windows、WSL 发行版和 Codex 客户端，但没有中转站服务器、Docker 或管理员后台权限。不要要求我发送完整 API Key、认证文件或私人对话；命令输出中如果出现用户名、目录、令牌或代理凭证，请先提醒我脱敏。

请先只读诊断，并明确告诉我每条命令应该在“Windows PowerShell”还是“WSL 的 Ubuntu Shell”中执行，不要把两种终端的命令混在同一个代码块里。

第一步，在 Windows PowerShell 中检查 WSL2 本身：
- 运行 wsl --status 和 wsl -l -v，确认目标发行版存在、状态正常且 VERSION 为 2；
- 如果 WSL 未安装、发行版无法启动或版本不是 2，先说明现状和影响，再给出微软当前官方文档对应的最小处理步骤；
- 不要删除、注销或重装发行版，不要执行 wsl --unregister，除非我另行明确授权并已有可验证备份。

第二步，在 WSL 的 Ubuntu Shell 中确认当前环境：
- 检查 echo $WSL_DISTRO_NAME、pwd、uname -a 和 printf '%s\\n' "$HOME"；
- 判断我是否误在 PowerShell/CMD 中执行 Linux 命令，或者虽然打开了 WSL，却仍在 /mnt/c/... 的 Windows 挂载目录工作；
- 如果工作目录在 /mnt/c，只说明它可能带来的文件 I/O、权限、大小写和软链接差异。先检查仓库是否有未提交内容和大文件，再给出迁移到 ~/code/... 的安全计划，不直接移动、覆盖或删除原目录。

第三步，确认 WSL 实际运行哪一个 Codex：
- 运行 command -v codex、type -a codex 和 codex --version，并检查解析出的路径属于 WSL 还是 Windows 挂载路径；
- 只在确认 WSL 中缺少 Codex 或当前安装损坏后，才参考 Codex 当前官方安装说明进行安装或修复；不要同时混用多个安装方式，也不要把 Windows 侧可执行文件手工复制进 WSL；
- 保留现有 Codex 配置、登录状态、memories、任务历史、skills、MCP 和项目 trust。不要通过删除整个 ~/.codex 来解决 command not found 或网络问题。

第四步，核对配置和网络边界：
- 确认当前进程读取的是 WSL 用户目录下的配置，不把 Windows 与 WSL 的 HOME、PATH、代理变量或认证状态当作自动共享；
- 只显示 Base URL 的协议、主机和必要路径，Key 只报告“已配置/未配置”，不要打印值；
- 分别检查 DNS、TLS、代理变量和目标入口的可达性。Windows 浏览器可访问不等于 WSL 一定继承相同代理；收到 401/403 说明已经到达某个 HTTP 服务，但仍需核对地址、认证和权限；超时、DNS 或证书错误应保留脱敏错误正文；
- 不要登录、重启或修改中转站服务器，不要更改账号、密钥、上游路由或计费配置。任何可能产生费用的模型请求先征得我同意，只做一次简短验证，不循环重试。

定位后，只执行与根因直接相关、可恢复的本机修复。修复完成后请分别报告：
1. WSL 发行版和版本是否正确；
2. 当前 Shell、HOME 和工作目录属于哪一侧；
3. command -v codex 的实际路径与版本；
4. 配置和网络检查通过到哪一层；
5. Codex 是否能从 WSL 启动，以及最小交互是否实际完成；
6. 哪些项目没有验证，是否需要我操作或联系站点管理员。

不要把“安装命令执行完”“路径里出现 codex”或“HTTP 有响应”单独当作恢复成功。遇到缺少权限、工具或信息时，说明具体阻塞和下一步，不虚构已修复。`

export const claudeCodeBypassPermissionsPrompt = `请帮我检查并配置这台电脑上的 Claude Code，让它启动后默认不再逐项弹出工具权限确认。

先确认操作系统、Claude Code 实际版本、启动命令，以及当前生效的用户级配置文件位置。只读取完成任务所需的字段；API Key、认证令牌和完整认证文件不得输出。若配置里已有自定义 API 地址、模型、环境变量、Hooks、MCP 或其他权限规则，必须原样保留。

在确认配置格式受当前 Claude Code 版本支持后，只做与本问题相关的最小修改：将 permissions.defaultMode 设为 bypassPermissions，并将 skipDangerousModePermissionPrompt 设为 true。修改前先建立可恢复备份；不要用一份简化示例覆盖整个配置文件，也不要删除其他字段。

请明确提醒我：该模式允许 Claude Code 在没有逐项确认的情况下执行文件修改和命令，风险高于普通确认模式，只适合我信任的本机项目与指令。不要因此扩大任务范围，不要修改系统权限、服务器、账号、密钥或项目数据。

修改后验证 JSON 或配置语法，并启动一个新的 Claude Code 会话确认默认权限模式。不要仅凭文件中出现字段就宣布成功。不要执行破坏性测试，也不要发送会产生费用的模型请求；如果必须由我关闭旧会话或重启终端，请说明原因和最短步骤。

同时告诉我单次恢复确认模式的入口：claude --permission-mode manual。最后只报告脱敏结果、修改文件、验证结论、风险和恢复方式；若当前版本不支持这些字段，保留原配置并报告准确错误，不要猜测其他开关。`

export const claudeCodeFableNotVisiblePrompt = `我通过一个兼容 Anthropic 接口的自定义中转站使用 Claude Code。中转站已经提供最新模型，但 Claude Code 的 /model 列表里看不到。下面以 Fable 5.1（模型 ID：claude-fable-5-1）为例，请在我的电脑上定位并修复客户端模型发现问题。

先确认 Claude Code 实际版本、启动方式和用户级配置文件位置。只检查必要配置；API Key、认证令牌、完整 Base URL 查询参数和认证文件不得输出。保留现有自定义 API 地址、认证、模型、权限、Hooks、MCP 与其他环境变量，不要用示例配置覆盖整个文件。

请把以下四层分开验证：
1. 当前 Claude Code 是否认识 claude-fable-5-1 这个模型 ID；
2. 当前实际连接的中转站 /v1/models 是否向我的凭证返回该模型；
3. Claude Code 是否启用了网关模型发现；
4. 新会话是否在 /model 中显示并能选中 Fable 5.1。

如果前两层通过但模型列表仍缺失，请在用户级 settings.json 的 env 中加入 CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY=1。修改前先建立可恢复备份，只增补该字段，不删除或打印现有敏感值。随后结束旧会话、启动新会话并重新打开 /model 验证；旧进程不一定会热加载环境配置。

默认只验证目录、列表和选中状态，不发送真实模型生成请求。请明确说明“模型可见”不等于“实际请求成功”。若我另行同意做最小请求，只执行一次短请求并报告脱敏状态，不循环重试。

最后分别报告：客户端版本、模型目录是否含 claude-fable-5-1、发现开关是否生效、/model 是否显示、是否实际选中、是否执行过生成请求。若仍未显示，保留客户端错误和脱敏目录响应，判断应继续检查本机版本/配置还是联系中转站管理员。`

export const experiences: ExperienceContent[] = [
  {
    id: 'codex-model-catalog-context-window',
    category: 'modelsUsage',
    route: PUBLIC_PAGES.codexModelCatalog,
    routeName: 'CodexModelCatalogExperience',
    icon: 'sync',
    title: codexModelCatalogGuide.title,
    summary: codexModelCatalogGuide.summary,
    series: 'Codex 使用错误说明',
    subtitle: '模型选对以后，还要让客户端加载正确的能力目录',
    applicableTo: codexModelCatalogGuide.applicableTo,
    updatedAt: codexModelCatalogGuide.updatedAt,
    prompt: codexModelCatalogGuide.prompt,
  },
  {
    id: 'windows-11-wsl-codex-frontend',
    category: 'connectionConfiguration',
    route: PUBLIC_PAGES.wslCodexTutorial,
    routeName: 'WslCodexFrontendTutorial',
    icon: 'terminal',
    title: 'Windows 11 + WSL2 使用 Codex：第一次真实前端开发',
    summary: '一篇可以照着操作的入门经验：在自己的仓库里，让 Codex 读代码、改页面、启动测试和自查，最后由你验收。',
    series: '新手实操经验',
    subtitle: '不用背命令，学会把开发执行交给 Codex',
    applicableTo: '第一次在 Windows 11 + WSL2 中使用 Codex CLI 的前端开发者',
    updatedAt: '2026-09-10',
    prompt: '',
  },
  {
    id: 'codex-cli',
    category: 'connectionConfiguration',
    route: PUBLIC_PAGES.codex,
    routeName: 'GuideCodex',
    icon: 'terminal',
    title: 'Codex CLI 接入与配置',
    summary: '从 config.toml 到最小请求，分清官方账号与自定义 provider 的配置边界。',
    series: '接入主题',
    subtitle: '面向 Codex CLI 的接入、配置与常见错误排查',
    applicableTo: 'Codex CLI 使用者 · 自有官方账号或本站服务密钥',
    updatedAt: '2026-09-08',
    prompt: '',
  },
  {
    id: 'claude-code',
    category: 'connectionConfiguration',
    route: PUBLIC_PAGES.claudeCode,
    routeName: 'GuideClaudeCode',
    icon: 'chat',
    title: 'Claude Code 接入与配置',
    summary: '通过环境变量设定 API 地址与凭证，并明确切换回官方方式的恢复路径。',
    series: '接入主题',
    subtitle: '面向 Claude Code 的自有账号与 API Key 接入指南',
    applicableTo: 'Claude Code 使用者 · 自有账号、Anthropic API Key 或本站服务密钥',
    updatedAt: '2026-09-08',
    prompt: '',
  },
  {
    id: 'openai-compatible-api',
    category: 'connectionConfiguration',
    route: PUBLIC_PAGES.openaiCompat,
    routeName: 'GuideOpenAICompat',
    icon: 'link',
    title: 'OpenAI 兼容接口与迁移测试',
    summary: '替换 SDK 的 base_url 和密钥，完成最小请求、流式响应与错误处理的迁移检查。',
    series: '接入主题',
    subtitle: '面向 OpenAI 兼容接口的 SDK 迁移与验证',
    applicableTo: 'OpenAI 或 Anthropic SDK 接入者 · 迁移测试场景',
    updatedAt: '2026-09-08',
    prompt: '',
  },
  {
    id: 'codex-session-migration',
    category: 'conversationContinuity',
    route: CODEX_SESSION_MIGRATION.route,
    routeName: 'CodexSessionMigration',
    icon: 'sync',
    title: CODEX_SESSION_MIGRATION.title,
    summary: '先只读定位旧任务的 provider 关联，再用经验证备份、计划摘要和恢复日志完成可审阅的本机历史迁移。',
    series: 'Codex 使用错误说明',
    subtitle: '解决你使用 Codex 或 Claude Code 的最后一公里',
    applicableTo: 'Codex 桌面端与命令行使用者 · macOS、Linux、Windows',
    updatedAt: '2026-09-08',
    prompt: CODEX_SESSION_MIGRATION.prompt,
  },
  {
    id: 'windows-11-wsl2-codex-environment',
    category: 'troubleshooting',
    route: PUBLIC_PAGES.wslCodexTroubleshooting,
    routeName: 'WslCodexTroubleshooting',
    icon: 'terminal',
    title: 'Windows 11 + WSL2 里 Codex 装好了却不能正常使用，怎么排查？',
    summary: '分清 PowerShell 与 WSL，逐层检查发行版、Codex 路径、工作目录和网络，不再靠反复重装碰运气。',
    series: 'Codex 使用错误说明',
    subtitle: '先确定命令到底在哪一层运行，再处理安装、路径与连接问题',
    applicableTo: 'Codex CLI 使用者 · Windows 11 + WSL2',
    updatedAt: '2026-09-12',
    prompt: wsl2CodexEnvironmentPrompt,
  },
  {
    id: 'gpt-6-astra-not-visible',
    category: 'modelsUsage',
    route: '/error-experiences/gpt-6-astra-not-visible',
    routeName: 'ErrorExperienceGpt6AstraNotVisible',
    icon: 'sparkles',
    title: '中转站已有新模型，为什么 Codex 看不到？',
    summary: '把服务可调用、客户端可见和任务选中分开检查，避免把模型目录、桌面版本或旧任务状态误判为同一个问题。',
    series: 'Codex 使用错误说明',
    subtitle: '解决你使用 Codex 或 Claude Code 的最后一公里',
    applicableTo: 'Codex 桌面使用者 · 历史案例环境：macOS',
    updatedAt: '2026-09-06',
    prompt: gpt6AstraNotVisiblePrompt,
  },
  {
    id: 'claude-code-bypass-permissions',
    category: 'connectionConfiguration',
    route: PUBLIC_PAGES.claudeCodeBypassPermissions,
    routeName: 'ClaudeCodeBypassPermissions',
    icon: 'terminal',
    title: 'Claude Code 每次执行都要确认，怎样默认跳过？',
    summary: '保留现有接入配置，只调整用户级权限模式，并说明高风险边界与单次恢复确认方式。',
    series: 'Claude Code 使用经验',
    subtitle: '减少重复确认之前，先把权限范围和恢复入口说清楚',
    applicableTo: 'Claude Code 使用者 · 信任的本机开发环境',
    updatedAt: '2026-09-13',
    prompt: claudeCodeBypassPermissionsPrompt,
  },
  {
    id: 'claude-code-fable-5-1-not-visible',
    category: 'modelsUsage',
    route: PUBLIC_PAGES.claudeCodeFableNotVisible,
    routeName: 'ClaudeCodeFableNotVisible',
    icon: 'sparkles',
    title: '中转站已有新模型，为什么 Claude Code 看不到？',
    summary: '以 Fable 5.1 为例，分开检查模型 ID、网关目录与客户端发现开关，让 /model 读取中转站提供的模型列表。',
    series: 'Claude Code 使用经验',
    subtitle: '以 Fable 5.1 为例：目录里有模型，还需要让客户端主动发现并刷新列表',
    applicableTo: 'Claude Code 使用者 · 自定义 Anthropic 兼容网关',
    updatedAt: '2026-09-13',
    prompt: claudeCodeFableNotVisiblePrompt,
  },
]

export function getExperienceById(id: string): ExperienceContent | undefined {
  return experiences.find((experience) => experience.id === id)
}
