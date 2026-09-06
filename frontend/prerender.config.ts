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
  siteLogo: '__SUB2API_SITE_LOGO_URL__',
  siteSubtitle: '__SUB2API_SITE_SUBTITLE__',
  apiBaseUrl: '__SUB2API_API_BASE_URL__',
  docUrl: '__SUB2API_DOC_URL__',
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
    ['og:url', /<meta\s+property="og:url"[\s\S]*?\/?>/, `<meta property="og:url" content="${esc(url)}" />`]
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

  // The application shell deliberately starts with an empty #app. Public pages need
  // their own readable no-JavaScript body, so replace a legacy seed when present or
  // insert one into the empty application container after the homepage migration.
  if (/<main[\s\S]*?<\/main>/.test(html)) {
    html = html.replace(/<main[\s\S]*?<\/main>/, () => seed)
  } else if (/<div\s+id="app"\s*>[\s\S]*?<\/div>/.test(html)) {
    html = html.replace(/<div\s+id="app"\s*>[\s\S]*?<\/div>/, () => `<div id="app">${seed}</div>`)
  } else {
    throw new Error(
      `[prerender] ${page.route}: no legacy seed or empty #app container matched in index.html. ` +
      'Update prerender.config.ts instead of shipping pages without readable public content.'
    )
  }
  return html
}

type HomeLocale = 'zh' | 'en'
type HomeMode = 'default' | 'compact'

// This stylesheet is deliberately independent from HomeView's scoped CSS. It loads before the
// app bundle, keeping the static document useful when JavaScript is unavailable or delayed.
const HOME_STATIC_CSS = `:root{color-scheme:light;font-family:Inter,ui-sans-serif,system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif}html.dark{color-scheme:dark}*{box-sizing:border-box}body{margin:0}.home-static{--bg:#f6f9fc;--ink:#102033;--muted:#536477;--glass:rgba(255,255,255,.76);--border:rgba(15,23,42,.1);min-height:100vh;overflow:hidden;color:var(--ink);background:radial-gradient(1100px 560px at 50% -10%,rgba(20,184,166,.16),transparent 62%),#f6f9fc}.dark .home-static{--bg:#050b14;--ink:#f8fafc;--muted:#aab7c7;--glass:rgba(10,18,32,.7);--border:rgba(148,163,184,.16);background:radial-gradient(1100px 560px at 50% -10%,rgba(20,184,166,.2),transparent 62%),#050b14}.home-static a{color:inherit;text-decoration:none}.home-static-grid{position:absolute;inset:0;pointer-events:none;opacity:.8;background-image:linear-gradient(rgba(100,116,139,.1) 1px,transparent 1px),linear-gradient(90deg,rgba(100,116,139,.1) 1px,transparent 1px);background-size:56px 56px;mask-image:radial-gradient(ellipse 80% 55% at 50% 0,#000 30%,transparent 100%)}.home-static-nav,.home-static-main,.home-static-footer{position:relative;z-index:1;max-width:72rem;margin:auto;padding-left:1.25rem;padding-right:1.25rem}.home-static-nav{padding-top:1rem}.home-static-nav-inner{display:flex;align-items:center;justify-content:space-between;gap:1rem;padding:.65rem .8rem;border:1px solid var(--border);border-radius:1rem;background:var(--glass);backdrop-filter:blur(18px);box-shadow:0 16px 36px -24px rgba(2,6,23,.6)}.home-static-brand{display:flex;align-items:center;gap:.7rem;min-width:0;font-weight:800}.home-static-brand img{width:2.25rem;height:2.25rem;flex:none;border-radius:.7rem;object-fit:contain}.home-static-brand span{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.home-static-actions,.home-static-ctas{display:flex;flex-wrap:wrap;align-items:center;gap:.65rem}.home-static-button,.home-static-doc{display:inline-flex;align-items:center;justify-content:center;min-height:2.5rem;padding:.5rem .9rem;border-radius:.65rem;font-size:.875rem;font-weight:700}.home-static-button{color:#fff;background:linear-gradient(135deg,#14b8a6,#0891b2 62%,#4f46e5);box-shadow:0 12px 28px -14px rgba(20,184,166,.9)}.home-static-doc{border:1px solid var(--border);background:var(--glass)}.home-static-main{padding-top:4rem;padding-bottom:3rem}.home-static-hero{display:grid;grid-template-columns:minmax(0,1fr) minmax(18rem,.9fr);align-items:center;gap:3rem;min-height:29rem}.home-static-kicker{display:inline-flex;gap:.5rem;align-items:center;padding:.42rem .75rem;border:1px solid rgba(20,184,166,.38);border-radius:999px;color:#0f766e;background:rgba(20,184,166,.1);font:700 .72rem ui-monospace,SFMono-Regular,Menlo,monospace;letter-spacing:.06em}.dark .home-static-kicker{color:#99f6e4}.home-static-dot{width:.45rem;height:.45rem;border-radius:99px;background:#14b8a6;box-shadow:0 0 0 .22rem rgba(20,184,166,.14)}.home-static-title{margin:1.25rem 0 .8rem;overflow-wrap:anywhere;font-size:clamp(2.8rem,7vw,5.6rem);line-height:1;font-weight:850}.home-static-tagline{margin:0;font-size:clamp(1.35rem,3vw,2rem);font-weight:800}.home-static-copy{max-width:40rem;color:var(--muted);line-height:1.65}.home-static-address,.home-static-panel,.home-static-card{border:1px solid var(--border);border-radius:1rem;background:var(--glass);backdrop-filter:blur(14px)}.home-static-address{margin-top:1.5rem;padding:1rem}.home-static-address-head{display:flex;justify-content:space-between;gap:1rem;margin-bottom:.55rem;font-size:.8rem;font-weight:750}.home-static-address code{display:block;overflow:hidden;padding:.72rem;border:1px solid rgba(20,184,166,.3);border-radius:.65rem;color:#0f766e;background:rgba(20,184,166,.09);text-overflow:ellipsis;white-space:nowrap}.dark .home-static-address code{color:#5eead4}.home-static-panel{padding:1.3rem}.home-static-panel-head{display:flex;justify-content:space-between;color:var(--muted);font:.75rem ui-monospace,SFMono-Regular,Menlo,monospace}.home-static-live{color:#10b981;font-weight:800}.home-static-route{display:grid;grid-template-columns:1fr auto 1fr;align-items:center;gap:.65rem;margin:2rem 0}.home-static-node{padding:1rem .6rem;border:1px solid var(--border);border-radius:.85rem;text-align:center;font-weight:750}.home-static-node strong{display:block;font-size:1.1rem}.home-static-line{height:2px;min-width:2rem;background:linear-gradient(90deg,#14b8a6,#6366f1)}.home-static-rtt{display:flex;justify-content:space-between;padding:.9rem;border-radius:.75rem;background:rgba(2,6,23,.07)}.dark .home-static-rtt{background:rgba(255,255,255,.06)}.home-static-rtt b{font-size:1.4rem}.home-static-section{margin-top:3rem}.home-static-section h2{font-size:1.45rem}.home-static-cards{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:.75rem}.home-static-card{padding:1rem}.home-static-card b{display:block;margin-bottom:.55rem}.home-static-card p{margin:0;color:var(--muted);font-size:.875rem;line-height:1.55}.home-static-footer{padding-bottom:2rem;color:var(--muted);font-size:.84rem}.home-static-links{display:flex;flex-wrap:wrap;gap:.75rem;margin-top:.75rem}.home-static-links a{text-decoration:underline;text-underline-offset:3px}.home-static-compact .home-static-main{display:grid;min-height:calc(100vh - 5.5rem);place-items:center;padding-top:2rem}.home-static-compact .home-static-hero{display:block;min-height:0;text-align:center}.home-static-compact .home-static-copy{margin-right:auto;margin-left:auto}.home-static-compact .home-static-ctas{justify-content:center}.home-static-compact .home-static-address,.home-static-compact .home-static-panel,.home-static-compact .home-static-section{display:none}.home-static-compact .home-static-kicker{margin:auto}@media(max-width:760px){.home-static-nav,.home-static-main,.home-static-footer{padding-left:.8rem;padding-right:.8rem}.home-static-main{padding-top:2.5rem}.home-static-hero{grid-template-columns:1fr;gap:1.5rem;min-height:0;text-align:center}.home-static-copy{margin-right:auto;margin-left:auto}.home-static-ctas{justify-content:center}.home-static-address{text-align:left}.home-static-cards{grid-template-columns:repeat(2,minmax(0,1fr))}.home-static-brand span{max-width:9rem}.home-static-doc{display:none}}@media(max-width:390px){.home-static-cards{grid-template-columns:1fr}.home-static-button{padding:.45rem .65rem}}`

const HOME_COPY: Record<HomeLocale, { kicker: string; tagline: string; subtitle: string; login: string; docs: string; address: string; rtt: string; cards: Array<[string, string]>; footer: string }> = {
  zh: { kicker: 'AI API 中转站', tagline: '人去 rest，AI 去 build。', subtitle: '一个入口接入多个 AI 模型；保留你自己的工具和工作流。', login: '登录', docs: '查看文档', address: 'API 接入地址', rtt: '本站 RTT', cards: [['实时状态', 'RTT 会在客户端测量完成后更新。'], ['统一入口', '一个 Base URL，兼容常用客户端。'], ['多模型接入', '在一个工作流中保持灵活选择。'], ['用量可见', '按实际使用量管理配额与账单。']], footer: 'rest 是你的，build 交给 AI。' },
  en: { kicker: 'AI API RELAY', tagline: 'You rest. AI builds.', subtitle: 'One entry point for multiple AI models while your tools and workflow stay yours.', login: 'Login', docs: 'View docs', address: 'API base URL', rtt: 'Site RTT', cards: [['Live status', 'RTT updates after the client-side measurement completes.'], ['One endpoint', 'One Base URL for compatible clients.'], ['Model access', 'Keep a flexible choice inside one workflow.'], ['Usage visibility', 'Manage quotas and billing by actual use.']], footer: 'rest is yours. Let AI handle the build.' }
}

function homeTemplate(locale: HomeLocale, mode: HomeMode, includeAlternate = true): string {
  const copy = HOME_COPY[locale]
  const cards = copy.cards.map(([title, description]) => `<article class="home-static-card"><b>${esc(title)}</b><p>${esc(description)}</p></article>`).join('')
  const sections = mode === 'compact' ? '' : `<section class="home-static-section"><h2>${esc(copy.kicker)}</h2><div class="home-static-cards">${cards}</div></section>`
  const documentMarkup = `<!-- SUB2API_HOME_TEMPLATE -->\n<div id="app"><main data-home-prerender="${mode}" class="home-static home-static-${mode}" data-home-locale="${locale}"><div class="home-static-grid" aria-hidden="true"></div><header class="home-static-nav"><nav class="home-static-nav-inner" aria-label="Primary"><a class="home-static-brand" href="/home"><img src="${PRERENDER_PLACEHOLDERS.siteLogo}" width="36" height="36" alt="${PRERENDER_PLACEHOLDERS.siteName}" /><span>${PRERENDER_PLACEHOLDERS.siteName}</span></a><div class="home-static-actions"><a class="home-static-doc" href="${PRERENDER_PLACEHOLDERS.docUrl}" target="_blank" rel="noopener noreferrer">${esc(copy.docs)}</a><a class="home-static-button" href="/login" data-home-primary-cta>${esc(copy.login)}</a></div></nav></header><div class="home-static-main"><section class="home-static-hero" data-home-hero><div><p class="home-static-kicker"><i class="home-static-dot"></i>${esc(copy.kicker)}</p><h1 class="home-static-title">${PRERENDER_PLACEHOLDERS.siteName}</h1><p class="home-static-tagline">${esc(copy.tagline)}</p><p class="home-static-copy">${esc(copy.subtitle)} ${PRERENDER_PLACEHOLDERS.siteSubtitle}</p><div class="home-static-ctas"><a class="home-static-button" href="/login" data-home-primary-cta>${esc(copy.login)}</a><a class="home-static-doc" href="${PRERENDER_PLACEHOLDERS.docUrl}" target="_blank" rel="noopener noreferrer">${esc(copy.docs)}</a></div><div class="home-static-address"><div class="home-static-address-head"><span>${esc(copy.address)}</span><span>${PRERENDER_PLACEHOLDERS.siteSubtitle}</span></div><code>${PRERENDER_PLACEHOLDERS.apiBaseUrl}</code></div></div><aside class="home-static-panel" aria-label="API relay status"><div class="home-static-panel-head"><span>${BRAND_DOMAIN}</span><span class="home-static-live">ONLINE</span></div><div class="home-static-route"><div class="home-static-node"><strong>you</strong>rest</div><div class="home-static-line"></div><div class="home-static-node"><strong>AI</strong>build</div></div><div class="home-static-rtt"><span>${esc(copy.rtt)}</span><b>-- ms</b></div></aside></section>${sections}</div><footer class="home-static-footer">${esc(copy.footer)}<nav class="home-static-links" aria-label="Public links"><a href="/codex-cli">Codex CLI</a><a href="/claude-code">Claude Code</a><a href="/openai-compatible-api">OpenAI API</a><a href="/model-plaza">Models</a><a href="/security">Security</a></nav></footer></main></div>`
  if (!includeAlternate) return documentMarkup
  const alternateLocale: HomeLocale = locale === 'zh' ? 'en' : 'zh'
  const alternateMain = homeTemplate(alternateLocale, mode, false).match(/<main[\s\S]*<\/main>/)?.[0]
  if (!alternateMain) throw new Error('[prerender] failed to build the alternate home language template')
  const localeSwap = `<template id="sub2api-home-alternate">${alternateMain}</template><script nonce="__SUB2API_CSP_NONCE__">;(()=>{try{const s=localStorage.getItem('sub2api_locale'),l=s==='zh'||s==='en'?s:(navigator.language||'').toLowerCase().startsWith('zh')?'zh':'en';if(l!=='${locale}'){const t=document.getElementById('sub2api-home-alternate'),a=document.getElementById('app');if(t instanceof HTMLTemplateElement&&a)a.replaceChildren(t.content.cloneNode(true))}}catch{}})()</script>`
  return `${documentMarkup}${localeSwap}`
}

function renderHomePage(baseHTML: string, locale: HomeLocale, mode: HomeMode): string {
  const container = /<div\s+id="app"\s*>[\s\S]*?<\/div>/
  if (!container.test(baseHTML)) throw new Error('[prerender] index.html has no #app container for the home template')
  const copy = HOME_COPY[locale]
  const init = `<script nonce="__SUB2API_CSP_NONCE__">;(()=>{try{document.documentElement.classList.toggle('dark',localStorage.getItem('theme')!=='light');const s=localStorage.getItem('sub2api_locale'),l=s==='zh'||s==='en'?s:(navigator.language||'').toLowerCase().startsWith('zh')?'zh':'en';document.documentElement.setAttribute('data-home-locale',l);document.documentElement.setAttribute('lang',l)}catch{document.documentElement.classList.add('dark');document.documentElement.setAttribute('data-home-locale','${locale}');document.documentElement.setAttribute('lang','${locale}')}})()</script>`
  return baseHTML
    .replace(/<html\s+lang="[^"]*">/, `<html lang="${locale === 'zh' ? 'zh-CN' : 'en'}">`)
    .replace(/<title>[^<]*<\/title>/, `<title>${PRERENDER_PLACEHOLDERS.siteName} - AI API Gateway</title>`)
    .replace(
      /<meta\s+name="description"[\s\S]*?\/?>/,
      `<meta name="description" content="${esc(copy.subtitle)}" />`
    )
    .replace(/<link\s+rel="canonical"[\s\S]*?\/?>/, `<link rel="canonical" href="https://${BRAND_DOMAIN}/home" />`)
    .replace('</head>', `<link rel="stylesheet" href="/home/static.css" />${init}</head>`)
    .replace(container, () => homeTemplate(locale, mode))
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
      if (!existsSync(indexPath)) {
        throw new Error('[prerender] Vite did not emit index.html; cannot generate required home templates.')
      }
      const baseHTML = readFileSync(indexPath, 'utf8')
      let count = 0
      for (const page of buildPrerenderPages()) {
        const dir = join(outDir, page.route.replace(/^\//, ''))
        mkdirSync(dir, { recursive: true })
        writeFileSync(join(dir, 'index.html'), renderPage(baseHTML, page), 'utf8')
        count++
      }
      const homeDir = join(outDir, 'home')
      mkdirSync(homeDir, { recursive: true })
      writeFileSync(join(homeDir, 'static.css'), HOME_STATIC_CSS, 'utf8')
      for (const locale of ['zh', 'en'] as const) {
        for (const mode of ['default', 'compact'] as const) {
          writeFileSync(join(homeDir, `${mode}.${locale}.html`), renderHomePage(baseHTML, locale, mode), 'utf8')
        }
      }
      writeFileSync(join(homeDir, 'index.html'), renderHomePage(baseHTML, 'zh', 'default'), 'utf8')
      config.logger.info(`[prerender] wrote ${count} public pages and four home templates under ${outDir}`)
    }
  }
}
