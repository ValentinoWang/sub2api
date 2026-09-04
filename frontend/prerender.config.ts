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
import { BRAND_DOMAIN, PUBLIC_PAGES, XIANYU_STORE_NAME } from './src/constants/brand'

type Section = { h: string; p?: string; items?: string[] }

interface PrerenderPage {
  route: string
  title: string
  description: string
  body: string
}

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
      body: `<p><strong>${esc(p.verify.platform)} · ${esc(p.verify.storeLabel)}：${esc(XIANYU_STORE_NAME)}</strong></p><ul>${p.verify.tips.map((t) => `<li>${esc(t)}</li>`).join('')}</ul>`
    },
    { route: PUBLIC_PAGES.codex, title: p.codex.title, description: p.codex.subtitle, body: guideHTML(p.codex as never) },
    { route: PUBLIC_PAGES.claudeCode, title: p.claudeCode.title, description: p.claudeCode.subtitle, body: guideHTML(p.claudeCode as never) },
    { route: PUBLIC_PAGES.openaiCompat, title: p.openaiCompat.title, description: p.openaiCompat.subtitle, body: guideHTML(p.openaiCompat as never) },
    { route: PUBLIC_PAGES.benchmarks, title: p.benchmarks.title, description: p.benchmarks.subtitle, body: `<p>${esc(p.benchmarks.intro)}</p><p>${esc(p.benchmarks.method)}</p>` },
    { route: PUBLIC_PAGES.status, title: zh.statusPage.title, description: zh.statusPage.subtitle, body: `<p>${esc(zh.statusPage.method)}</p>` },
    { route: PUBLIC_PAGES.share, title: zh.share.title, description: zh.share.subtitle, body: `<p>${esc(zh.share.hint)}</p>` }
  ]
  return pages
}

function renderPage(baseHTML: string, page: PrerenderPage, siteName: string): string {
  const url = `https://${BRAND_DOMAIN}${page.route}`
  const title = `${page.title}｜${siteName}`
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
  let html = baseHTML
  html = html.replace(/<title>[^<]*<\/title>/, `<title>${esc(title)}</title>`)
  html = html.replace(/<meta\s+name="description"\s+content="[^"]*"\s*\/?>/, `<meta name="description" content="${esc(page.description)}" />`)
  html = html.replace(/<link rel="canonical" href="[^"]*" \/>/, `<link rel="canonical" href="${esc(url)}" />`)
  html = html.replace(/<meta property="og:title" content="[^"]*" \/>/, `<meta property="og:title" content="${esc(title)}" />`)
  html = html.replace(/<meta property="og:description" content="[^"]*" \/>/, `<meta property="og:description" content="${esc(page.description)}" />`)
  html = html.replace(/<meta property="og:url" content="[^"]*" \/>/, `<meta property="og:url" content="${esc(url)}" />`)
  // Replace the seed block inside #app (everything between the first <main and </main>).
  html = html.replace(/<main[\s\S]*?<\/main>/, seed)
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
        writeFileSync(join(dir, 'index.html'), renderPage(baseHTML, page, 'rest2build'), 'utf8')
        count++
      }
      config.logger.info(`[prerender] wrote ${count} public pages under ${outDir}`)
    }
  }
}
