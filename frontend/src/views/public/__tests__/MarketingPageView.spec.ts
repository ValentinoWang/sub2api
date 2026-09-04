import { describe, expect, it, vi } from 'vitest'
import { mount, RouterLinkStub } from '@vue/test-utils'

import zh from '@/i18n/locales/zh'
import MarketingPageView from '../MarketingPageView.vue'
import { XIANYU_STORE_NAME } from '@/constants/brand'

const { routeMeta, appStore } = vi.hoisted(() => ({
  routeMeta: { marketingPage: 'publicBenefit' } as Record<string, unknown>,
  appStore: {
    cachedPublicSettings: { contact_info: 'tg: @rest2build' } as Record<string, unknown>,
    siteName: 'rest2build',
    siteLogo: '',
    contactInfo: ''
  }
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ path: '/x', meta: routeMeta })
}))
vi.mock('@/stores', () => ({ useAppStore: () => appStore }))

// The test build of vue-i18n has no message compiler, so resolve keys straight from the zh resources.
function resolve(key: string): unknown {
  return key.split('.').reduce<unknown>((node, part) => (node && typeof node === 'object' ? (node as Record<string, unknown>)[part] : undefined), zh)
}
vi.mock('vue-i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-i18n')>()),
  useI18n: () => ({
    t: (key: string) => {
      const value = resolve(key)
      return typeof value === 'string' ? value : key
    },
    tm: (key: string) => resolve(key) ?? [],
    rt: (value: unknown) => (typeof value === 'string' ? value : '')
  })
}))

function mountPage(marketingPage: string) {
  routeMeta.marketingPage = marketingPage
  return mount(MarketingPageView, {
    global: {
      stubs: {
        RouterLink: RouterLinkStub,
        LocaleSwitcher: { template: '<div />' }
      }
    }
  })
}

describe('MarketingPageView', () => {
  it.each([
    ['publicBenefit', zh.marketing.pages.publicBenefit.title],
    ['business', zh.marketing.pages.business.title],
    ['security', zh.marketing.pages.security.title],
    ['codex', zh.marketing.pages.codex.title],
    ['claudeCode', zh.marketing.pages.claudeCode.title],
    ['openaiCompat', zh.marketing.pages.openaiCompat.title],
    ['benchmarks', zh.marketing.pages.benchmarks.title]
  ])('renders the %s page title and disclaimer', (key, title) => {
    const wrapper = mountPage(key)
    expect(wrapper.get('h1').text()).toBe(title)
    expect(wrapper.text()).toContain(zh.marketing.disclaimer)
  })

  it('renders every section of a static page', () => {
    const wrapper = mountPage('publicBenefit')
    for (const section of zh.marketing.pages.publicBenefit.sections) {
      expect(wrapper.text()).toContain(section.h)
    }
  })

  it('shows the single Xianyu store name and contact on the verify page', () => {
    const wrapper = mountPage('verify')
    expect(wrapper.text()).toContain(XIANYU_STORE_NAME)
    expect(wrapper.text()).toContain('tg: @rest2build')
  })

  it('renders config snippets that point at the base URL on guide pages', () => {
    const wrapper = mountPage('codex')
    expect(wrapper.text()).toContain('wire_api = "responses"')
    expect(wrapper.text()).toContain('/v1"')
    const claude = mountPage('claudeCode')
    expect(claude.text()).toContain('ANTHROPIC_BASE_URL')
  })
})
