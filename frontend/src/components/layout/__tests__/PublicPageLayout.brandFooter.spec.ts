import { describe, expect, it, vi } from 'vitest'
import { mount, RouterLinkStub } from '@vue/test-utils'

import PublicPageLayout from '../PublicPageLayout.vue'

vi.mock('vue-router', () => ({
  useRoute: () => ({ path: '/public-benefit' })
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
        LocaleSwitcher: { template: '<div />' }
      }
    }
  })
}

describe('PublicPageLayout brand footer', () => {
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
