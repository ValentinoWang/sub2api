import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, RouterLinkStub } from '@vue/test-utils'

import PublicPageLayout from '../PublicPageLayout.vue'

const auth = vi.hoisted(() => ({ isAuthenticated: false }))
const route = vi.hoisted(() => ({ path: '/public-benefit' }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => auth }))

vi.mock('vue-router', () => ({
  useRoute: () => route
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    cachedPublicSettings: {},
    siteName: 'rest2build',
    siteLogo: ''
  })
}))

vi.mock('vue-i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-i18n')>()),
  useI18n: () => ({ t: (key: string) => key })
}))

function mountLayout(slots?: Record<string, string>) {
  return mount(PublicPageLayout, {
    slots,
    global: {
      stubs: {
        RouterLink: RouterLinkStub,
        AppLayout: { template: '<div data-testid="app-layout"><slot /></div>' },
        LocaleSwitcher: { template: '<div />' }
      }
    }
  })
}

describe('PublicPageLayout brand footer', () => {
  beforeEach(() => {
    auth.isAuthenticated = false
    route.path = '/public-benefit'
  })

  it.each(['/experiences', '/experiences/windows-11-wsl-codex-frontend', '/error-experiences/windows-11-wsl2-codex-environment', '/codex-cli', '/claude-code', '/openai-compatible-api'])('keeps signed-in readers in the account shell at %s', (path) => {
    auth.isAuthenticated = true
    route.path = path
    const wrapper = mountLayout({ default: '<article>Experience content</article>' })
    expect(wrapper.find('[data-testid="app-layout"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('Experience content')
    expect(wrapper.find('.pub-header').exists()).toBe(false)
    expect(wrapper.find('.brand-statement').exists()).toBe(false)
  })

  it('retains the public shell for unrelated public pages when signed in', () => {
    auth.isAuthenticated = true
    expect(mountLayout().find('.pub-header').exists()).toBe(true)
  })

  it('retains public access to experiences when signed out', () => {
    route.path = '/experiences'
    expect(mountLayout().find('.pub-header').exists()).toBe(true)
  })

  it('uses one rest2build brand statement as its default footer', () => {
    const wrapper = mountLayout()

    expect(wrapper.findAll('footer .brand-statement')).toHaveLength(1)
    expect(wrapper.get('footer .brand-wordmark').text()).toBe('rest2build')
    expect(wrapper.get('footer .brand-tagline').text()).toBe('歇一会儿，让 AI 接着干。')
  })

  it('uses a supplied footer instead of appending the default brand statement', () => {
    const wrapper = mountLayout({ footer: '<div data-test="custom-footer">custom footer</div>' })

    expect(wrapper.get('footer [data-test="custom-footer"]').text()).toBe('custom footer')
    expect(wrapper.find('footer .brand-statement').exists()).toBe(false)
  })
})
