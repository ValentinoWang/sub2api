import { describe, expect, it } from 'vitest'
import { renderHomePage } from '../../prerender.config'
import { getExperienceById } from '@/content/experiences'

const baseHTML = `<!doctype html><html lang="zh-CN"><head><title>Home</title><meta name="description" content="old" /><link rel="canonical" href="https://example.test/" /></head><body><div id="app"></div></body></html>`

describe('home static prerender', () => {
  it('places the featured experience and its native index link before the FAQ', () => {
    const featuredExperience = getExperienceById('codex-cli')
    const html = renderHomePage(baseHTML, 'zh', 'default')
    const experience = html.indexOf('data-home-experience-featured')
    const experiencesLink = html.indexOf('href="/experiences"')
    const faq = html.indexOf('data-home-faq')

    expect(experience).toBeGreaterThan(-1)
    expect(featuredExperience).toBeDefined()
    expect(experiencesLink).toBeGreaterThan(experience)
    expect(faq).toBeGreaterThan(experiencesLink)
    expect(html).toContain(featuredExperience!.title)
    expect(html).toContain(`href="${featuredExperience!.route}"`)
  })

  it('keeps compact output aligned with the compact Vue home branch', () => {
    const html = renderHomePage(baseHTML, 'en', 'compact')

    expect(html).toContain('data-home-prerender="compact"')
    expect(html).not.toContain('data-home-experience-featured')
    expect(html).not.toContain('data-home-faq')
  })
})
