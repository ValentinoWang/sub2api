import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import GettingStartedView from '@/views/user/GettingStartedView.vue'

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

describe('GettingStartedView', () => {
  it('links every onboarding step to a real product destination', () => {
    const wrapper = mount(GettingStartedView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
          RouterLink: {
            props: ['to'],
            template: '<a :href="to"><slot /></a>'
          },
          Icon: true
        }
      }
    })

    const destinations = wrapper.findAll('a').map((link) => link.attributes('href'))
    expect(destinations).toEqual(expect.arrayContaining([
      '/purchase',
      '/redeem',
      '/keys',
      '/usage',
      '/subscriptions',
      '/docs/codex-memory'
    ]))
    expect(wrapper.get('[data-testid="getting-started-steps"]')).toBeTruthy()
    expect(wrapper.get('[data-testid="codex-setup-methods"]').text()).toContain('gettingStarted.connect.ccswitchTitle')
    expect(wrapper.get('[data-testid="codex-setup-methods"]').text()).toContain('gettingStarted.connect.manualTitle')
  })
})
