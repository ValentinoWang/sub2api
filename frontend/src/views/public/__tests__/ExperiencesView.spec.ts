import { mount, RouterLinkStub } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import ExperiencesView from '../ExperiencesView.vue'

const PublicPageLayout = {
  template: '<div><slot /><footer><slot name="footer" /></footer></div>'
}

vi.mock('vue-i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-i18n')>()),
  useI18n: () => ({ t: (key: string) => key })
}))

describe('ExperiencesView', () => {
  it('renders the initial experience and filters it by category', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/experiences', component: ExperiencesView }]
    })
    await router.push('/experiences')
    await router.isReady()

    const wrapper = mount(ExperiencesView, {
      global: {
        plugins: [router],
        stubs: { PublicPageLayout, RouterLink: RouterLinkStub }
      }
    })

    expect(wrapper.text()).toContain('GPT-6 已接入，为什么 Codex 仍然看不见？')
    expect(wrapper.findAll('.experience-filter')).toHaveLength(5)

    await wrapper.findAll('.experience-filter')[2].trigger('click')
    expect(wrapper.findAll('.experience-card')).toHaveLength(1)
  })
})
