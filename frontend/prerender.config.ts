/**
 * Build-time prerendering of the public marketing pages.
 *
 * The SPA stays the source of truth; this only writes `dist/<route>/index.html` files whose
 * <head> carries a page-specific title / description / canonical and whose #app seed contains
 * the page's real copy, so crawlers that do not execute JavaScript (and LLM web-fetch tools)
 * read the same facts the Vue app renders. The backend serves these files for their routes
 * with the usual settings injection; the Vue app then mounts and replaces the seed.
 */
import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import type { Plugin, ResolvedConfig } from 'vite'
import zh from './src/i18n/locales/zh/landing'
import { BRAND_DOMAIN, PUBLIC_PAGES } from './src/constants/brand'

type Section = { h: string; p?: string; items?: string[] }

interface PrerenderPage {
  route: string
  title: string
  description: string
  body: string
}

/**
 * Values the backend substitutes at serve time. Anything an administrator can rename must go
 * through a placeholder: baking it in here would make the crawler-facing copy disagree with what
 * the Vue app renders, which matters most on the store-verification page.
 */
export const PRERENDER_PLACEHOLDERS = {
  siteName: '__SUB2API_SITE_NAME__',
  storeName: '__SUB2API_XIANYU_STORE_NAME__'
} as const

function esc(value: unknown): string {
  return String(value ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
}

function sectionsHTML(sections: Section[] | undefined): string {
  if (!Array.isArray(sections)) return ''
  return sections
    .map((s) => {
      const items = s.items?.length ? `<ul>${s.items.map((i) => `<li>${esc(i)}</li>`).join('')}</ul>` : ''
      const p = s.p ? `<p>${esc(s.p)}</p>` : ''
      return `<section><h2>${esc(s.h)}</h2>${p}${items}</section>`
    })
    .join('')
}

function guideHTML(page: { intro: string; byok?: string; managedIntro?: string; steps: Section[]; troubleshooting: { h: string; items: string[] } }): string {
  const byok = page.byok ? `<section><h2>BYOK</h2><p>${esc(page.byok)}</p></section>` : ''
  const managed = page.managedIntro ? `<p>${esc(page.managedIntro)}</p>` : ''
  const steps = page.steps.map((s) => `<h3>${esc(s.h)}</h3><p>${esc(s.p)}</p>`).join('')
  const ts = `<section><h2>${esc(page.troubleshooting.h)}</h2><ul>${page.troubleshooting.items.map((i) => `<li>${esc(i)}</li>`).join('')}</ul></section>`
  return `<p>${esc(page.intro)}</p>${byok}<section>${managed}${steps}</section>${ts}`
}

function errorExperienceHTML(): string {
  const prompt = `我使用 Sub2API 中转站接入 Codex，目标模型是 gpt-6-astra。现在可能遇到以下情况之一：桌面模型列表没有它；终端更新后桌面仍没变化；或者已经可选，但原任务还在使用旧模型。请先确认我实际遇到哪一种，再定位并处理，不要直接假定是客户端旧版本、缓存或站点故障。

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
  return `<p>错误经验 / ERR-001 · 适用：Codex 桌面使用者 · 更新：2026-09-06</p>
    <section><h2>情况说明</h2><p>你已经按照本站接入说明配置了 Codex，站点也已提供 GPT-6-Astra，但桌面模型选择器里仍然只有旧模型。中转站能否调用模型、桌面应用是否展示模型、当前任务实际选择哪个模型，是不同环节。</p><ul><li>桌面模型列表没有 GPT-6-Astra。</li><li>终端已更新，桌面仍显示旧列表。</li><li>模型可选，但原任务还在使用旧模型。</li><li>不确定实际连接的是本站、旧地址，还是本机代理。</li></ul></section>
    <section><h2>Codex 帮你处理</h2><pre style="white-space:pre-wrap">${esc(prompt)}</pre></section>
    <section><h2>给人看的：原因、证据与经验</h2><h3>支持、可见与选中不是同一个开关</h3><p>先分开检查中转站与个人访问权限、客户端模型目录，以及默认配置与当前任务选择。API 成功不必然使桌面出现新模型；桌面没有新模型也不能单独证明中转站不支持它。</p><h3>终端升级不代表桌面升级</h3><p>终端命令与桌面应用可各自携带 Codex。历史案例中，桌面内置 0.153.3 将 Astra 标记为 hide，0.153.4 恢复 list；这只是带日期的排查线索，应以当前实际运行版本和官方说明为准。</p><h3>先判断缓存是否真的相关</h3><p>不要删除整个 Codex 目录、memories、任务历史、认证或 Docker 数据。仅在确认存在、被读取且过时后，定点清理可再生的模型元数据缓存。</p><h3>恢复验收</h3><p>重新打开应用后，应分别确认实际版本、模型是否可见、任务是否明确选择目标模型，以及使用个人入口完成的最小请求是否成功。</p></section>
    <section><h2>rest2build</h2><p><strong>歇一会儿，让 AI 接着干。</strong></p><p>rest 是你的，build 交给 AI。</p><p>rest2build 提供面向 Codex、Claude Code 等工具的 AI 模型接入服务。同时围绕公益 Skills、AI 使用经验分享与 Harness 工程，持续开展内容与实践。</p><p><a href="https://ai.rest2build.lol/">ai.rest2build.lol</a></p></section>`
}

export function buildPrerenderPages(): PrerenderPage[] {
  const m = zh.marketing
  const p = m.pages
  const pages: PrerenderPage[] = [
    { route: PUBLIC_PAGES.publicBenefit, title: p.publicBenefit.title, description: p.publicBenefit.subtitle, body: sectionsHTML(p.publicBenefit.sections as Section[]) },
    { route: PUBLIC_PAGES.business, title: p.business.title, description: p.business.subtitle, body: sectionsHTML(p.business.sections as Section[]) },
    { route: PUBLIC_PAGES.security, title: p.security.title, description: p.security.subtitle, body: sectionsHTML(p.security.sections as Section[]) },
    {
      route: PUBLIC_PAGES.verify,
      title: p.verify.title,
      description: p.verify.subtitle,
      body: `<p><strong>${esc(p.verify.platform)} · ${esc(p.verify.storeLabel)}：${PRERENDER_PLACEHOLDERS.storeName}</strong></p><ul>${p.verify.tips.map((t) => `<li>${esc(t)}</li>`).join('')}</ul>`
    },
    { route: PUBLIC_PAGES.codex, title: p.codex.title, description: p.codex.subtitle, body: guideHTML(p.codex as never) },
    { route: PUBLIC_PAGES.claudeCode, title: p.claudeCode.title, description: p.claudeCode.subtitle, body: guideHTML(p.claudeCode as never) },
    { route: PUBLIC_PAGES.openaiCompat, title: p.openaiCompat.title, description: p.openaiCompat.subtitle, body: guideHTML(p.openaiCompat as never) },
    { route: PUBLIC_PAGES.benchmarks, title: p.benchmarks.title, description: p.benchmarks.subtitle, body: `<p>${esc(p.benchmarks.intro)}</p><p>${esc(p.benchmarks.method)}</p>` },
    { route: PUBLIC_PAGES.status, title: zh.statusPage.title, description: zh.statusPage.subtitle, body: `<p>${esc(zh.statusPage.method)}</p>` },
    { route: PUBLIC_PAGES.share, title: zh.share.title, description: zh.share.subtitle, body: `<p>${esc(zh.share.hint)}</p>` },
    {
      route: PUBLIC_PAGES.errorExperience,
      title: 'GPT-6 已接入，为什么 Codex 仍然看不见？',
      description: 'Codex 桌面中 GPT-6-Astra 不可见时，区分中转站支持、客户端模型目录与当前任务选择的公开排障经验。',
      body: errorExperienceHTML()
    }
  ]
  return pages
}

function renderPage(baseHTML: string, page: PrerenderPage): string {
  const url = `https://${BRAND_DOMAIN}${page.route}`
  const title = `${page.title}｜${PRERENDER_PLACEHOLDERS.siteName}`
  const faq = zh.marketing.faq.items.map((f) => `<p><strong>${esc(f.q)}</strong> ${esc(f.a)}</p>`).join('')
  const seed = `<main style="max-width: 720px; margin: 48px auto; padding: 0 20px; font-family: system-ui, sans-serif; line-height: 1.6">
        <p>${esc(zh.marketing.nonOfficialShort)}</p>
        <h1>${esc(page.title)}</h1>
        <p>${esc(page.description)}</p>
        ${page.body}
        <p>${esc(zh.marketing.disclaimer)}</p>
        <h2>${esc(zh.marketing.faq.title)}</h2>
        ${faq}
        <p><a href="/home">${esc(zh.marketing.pages.common.backHome)}</a> · <a href="/login">${esc(zh.marketing.pages.common.login)}</a></p>
      </main>`
  // Each tag must actually be found: a silently unmatched pattern would leave every prerendered
  // page carrying the home page's title, description and canonical, which is worse than not
  // prerendering at all. The replacement is passed as a function so "$&" and friends in the copy
  // are inserted literally instead of being read as replacement patterns.
  const substitutions: Array<[string, RegExp, string]> = [
    ['title', /<title>[^<]*<\/title>/, `<title>${esc(title)}</title>`],
    ['meta description', /<meta\s+name="description"[\s\S]*?\/?>/, `<meta name="description" content="${esc(page.description)}" />`],
    ['canonical', /<link\s+rel="canonical"[\s\S]*?\/?>/, `<link rel="canonical" href="${esc(url)}" />`],
    ['og:title', /<meta\s+property="og:title"[\s\S]*?\/?>/, `<meta property="og:title" content="${esc(title)}" />`],
    ['og:description', /<meta\s+property="og:description"[\s\S]*?\/?>/, `<meta property="og:description" content="${esc(page.description)}" />`],
    ['og:url', /<meta\s+property="og:url"[\s\S]*?\/?>/, `<meta property="og:url" content="${esc(url)}" />`],
    ['crawler seed', /<main[\s\S]*?<\/main>/, seed]
  ]

  let html = baseHTML
  for (const [label, pattern, replacement] of substitutions) {
    if (!pattern.test(html)) {
      throw new Error(
        `[prerender] ${page.route}: no ${label} tag matched in index.html. ` +
          'The template changed shape; update prerender.config.ts instead of shipping pages with the wrong metadata.'
      )
    }
    html = html.replace(pattern, () => replacement)
  }
  return html
}

/** Vite plugin: after the client bundle is written, emit one static HTML per public route. */
export function prerenderPublicPages(): Plugin {
  let config: ResolvedConfig
  return {
    name: 'sub2api-prerender-public-pages',
    apply: 'build',
    configResolved(resolved) {
      config = resolved
    },
    closeBundle() {
      const outDir = config.build.outDir
      const indexPath = join(outDir, 'index.html')
      if (!existsSync(indexPath)) return
      const baseHTML = readFileSync(indexPath, 'utf8')
      let count = 0
      for (const page of buildPrerenderPages()) {
        const dir = join(outDir, page.route.replace(/^\//, ''))
        mkdirSync(dir, { recursive: true })
        writeFileSync(join(dir, 'index.html'), renderPage(baseHTML, page), 'utf8')
        count++
      }
      config.logger.info(`[prerender] wrote ${count} public pages under ${outDir}`)
    }
  }
}
