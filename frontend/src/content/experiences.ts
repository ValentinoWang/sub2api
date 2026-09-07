export type ExperienceCategory =
  | 'toolUse'
  | 'integrationTroubleshooting'
  | 'publicBenefitSkills'
  | 'harnessEngineering'

export interface ExperienceCategoryDefinition {
  id: ExperienceCategory
  labelKey: string
}

export interface ExperienceContent {
  id: string
  category: ExperienceCategory
  route: string
  title: string
  summary: string
  series: string
  subtitle: string
  applicableTo: string
  updatedAt: string
  prompt: string
}

export const experienceCategories: ExperienceCategoryDefinition[] = [
  { id: 'toolUse', labelKey: 'experiences.categories.toolUse' },
  { id: 'integrationTroubleshooting', labelKey: 'experiences.categories.integrationTroubleshooting' },
  { id: 'publicBenefitSkills', labelKey: 'experiences.categories.publicBenefitSkills' },
  { id: 'harnessEngineering', labelKey: 'experiences.categories.harnessEngineering' },
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

export const experiences: ExperienceContent[] = [
  {
    id: 'gpt-6-astra-not-visible',
    category: 'integrationTroubleshooting',
    route: '/error-experiences/gpt-6-astra-not-visible',
    title: 'GPT-6 已接入，为什么 Codex 仍然看不见？',
    summary: '把服务可调用、客户端可见和任务选中分开检查，避免把模型目录、桌面版本或旧任务状态误判为同一个问题。',
    series: 'Codex 使用错误说明',
    subtitle: '解决你使用 Codex 或 Claude Code 的最后一公里',
    applicableTo: 'Codex 桌面使用者 · 历史案例环境：macOS',
    updatedAt: '2026-09-06',
    prompt: gpt6AstraNotVisiblePrompt,
  },
]

export function getExperienceById(id: string): ExperienceContent | undefined {
  return experiences.find((experience) => experience.id === id)
}
