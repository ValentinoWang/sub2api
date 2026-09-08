import { ref } from 'vue'
import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import AuthLayout from '../AuthLayout.vue'

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    cachedPublicSettings: {},
    fetchPublicSettings: vi.fn(),
    publicSettingsLoaded: true,
    siteLogo: '',
    siteName: 'rest2build'
  })
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key })
}))

vi.mock('@/composables/useLatencyProbe', () => ({
  useLatencyProbe: () => ({ latencyMs: ref(null), state: ref('idle') })
}))

function mountLayout(slots?: Record<string, string>) {
  return mount(AuthLayout, {
    slots,
    global: {
      stubs: {
        BrandWordmark: { template: '<span><slot /></span>' },
        RelayStationVisual: { template: '<div />' }
      }
    }
  })
}

describe('AuthLayout after content', () => {
  it('uses the full-width after-content section without leaving the legacy copyright block above it', () => {
    const wrapper = mountLayout({
      default: '<div>sign in</div>',
      afterContent: '<section data-test="experience-sharing">experience sharing</section>'
    })

    expect(wrapper.get('[data-test="experience-sharing"]').text()).toBe('experience sharing')
    expect(wrapper.find('.auth-copyright').exists()).toBe(false)
    expect(wrapper.find('.auth-after-content').exists()).toBe(true)
  })

  it('keeps the conventional copyright block for authentication pages without extended content', () => {
    const wrapper = mountLayout({ default: '<div>sign in</div>' })

    expect(wrapper.find('.auth-copyright').exists()).toBe(true)
    expect(wrapper.find('.auth-after-content').exists()).toBe(false)
  })
})
