import { describe, expect, it } from 'vitest'
import { buildPrerenderPages, renderHomePage } from '../../../prerender.config'
import { experiences, getExperienceById } from '@/content/experiences'
import router from '@/router'

const baseHTML = '<!doctype html><html lang="zh-CN"><head><title>Home</title><meta name="description" content="old" /><link rel="canonical" href="https://example.test/" /></head><body><div id="app"></div></body></html>'

describe('public experience publication', () => {
  const featuredExperience = getExperienceById('codex-cli')!
  const errorExperience = getExperienceById('gpt-6-astra-not-visible')!

  it('keeps the formal home first paint linked natively to the experience index and article', () => {
    const html = renderHomePage(baseHTML, 'zh', 'default')
    const featured = html.indexOf('data-home-experience-featured')
    const browseLink = html.indexOf('href="/experiences"', featured)
    const articleLink = html.indexOf(`href="${featuredExperience.route}"`, featured)
    const faq = html.indexOf('data-home-faq')

    expect(featured).toBeGreaterThan(-1)
    expect(browseLink).toBeGreaterThan(featured)
    expect(articleLink).toBeGreaterThan(featured)
    expect(faq).toBeGreaterThan(articleLink)
    expect(html).toContain(featuredExperience.title)
  })

  it('pre-renders a readable experience index with a native handoff to every published article', () => {
    const index = buildPrerenderPages().find((page) => page.route === '/experiences')

    expect(index).toBeDefined()
    expect(index?.body).toContain('<h1>AI 使用经验分享</h1>')
    expect(index?.body).toContain('收录面向真实使用场景的排障与实践经验。')
    for (const experience of experiences) {
      expect(index?.body).toContain(experience.title)
      expect(index?.body).toContain(`href="${experience.route}"`)
    }
  })

  it('pre-renders the article as readable public content while preserving its router destination', () => {
    const article = buildPrerenderPages().find((page) => page.route === errorExperience.route)

    expect(article).toBeDefined()
    expect(article?.title).toBe(errorExperience.title)
    expect(article?.body).toContain('<h2>情况说明</h2>')
    expect(article?.body).toContain('<h2>Codex 帮你处理</h2>')
    expect(article?.body).toContain('<h2>给人看的：原因、证据与经验</h2>')
    expect(article?.body).toContain('GPT-6-Astra')
  })

  it('keeps every static experience handoff resolvable by the public Vue router', () => {
    const indexRoute = router.resolve('/experiences')

    expect(indexRoute.name).toBe('Experiences')
    expect(indexRoute.meta.requiresAuth).toBe(false)
    for (const experience of experiences) {
      const articleRoute = router.resolve(experience.route)
      expect(articleRoute.name).toBe(experience.routeName)
      expect(articleRoute.meta.requiresAuth).toBe(false)
    }
  })

  it('pre-renders every experience route exactly once', () => {
    const pages = buildPrerenderPages()

    for (const experience of experiences) {
      expect(pages.filter((page) => page.route === experience.route)).toHaveLength(1)
    }
  })
})
