import { flushPromises, mount, RouterLinkStub } from '@vue/test-utils'
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
  it('renders guide entries, keeps their public routes, and persists a topic filter in the route', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/experiences', component: ExperiencesView }]
    })
    await router.push('/experiences?source=home')
    await router.isReady()

    const wrapper = mount(ExperiencesView, {
      global: {
        plugins: [router],
        stubs: { PublicPageLayout, RouterLink: RouterLinkStub }
      }
    })

    expect(wrapper.text()).toContain('GPT-6 已接入，为什么 Codex 仍然看不见？')
    expect(wrapper.findAll('.experience-card')).toHaveLength(7)
    expect(wrapper.findAll('.experience-filter')).toHaveLength(5)
    expect(wrapper.get('[data-experience-id="codex-cli"]').find('.experience-card-topic').text()).toBe('接入主题')
    expect(wrapper.get('[data-experience-id="codex-cli"]').find('.experience-card-audience-label').text()).toBe('experiences.appliesTo')

    const routes = wrapper.findAllComponents(RouterLinkStub).map((link) => link.props('to'))
    expect(routes).toEqual(expect.arrayContaining([
      '/codex-cli',
      '/claude-code',
      '/openai-compatible-api',
      '/experiences/windows-11-wsl-codex-frontend',
    ]))

    await wrapper.get('[data-category-filter="connectionConfiguration"]').trigger('click')
    await flushPromises()
    expect(wrapper.findAll('.experience-card')).toHaveLength(4)
    expect(router.currentRoute.value.query.category).toBe('connectionConfiguration')
    expect(router.currentRoute.value.query.source).toBe('home')

    await wrapper.get('[data-category-filter="all"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.query.category).toBeUndefined()
    expect(router.currentRoute.value.query.source).toBe('home')

    await wrapper.get('[data-category-filter="troubleshooting"]').trigger('click')
    await flushPromises()
    expect(wrapper.findAll('.experience-card')).toHaveLength(1)
    expect(wrapper.text()).toContain('Windows 11 + WSL2 里 Codex 装好了却不能正常使用')
    expect(wrapper.text()).not.toContain('第一次真实前端开发')
    expect(router.currentRoute.value.query.category).toBe('troubleshooting')
  })

  it('reads the category filter from the URL on initial load', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/experiences', component: ExperiencesView }]
    })
    await router.push('/experiences?category=conversationContinuity')
    await router.isReady()

    const wrapper = mount(ExperiencesView, {
      global: {
        plugins: [router],
        stubs: { PublicPageLayout, RouterLink: RouterLinkStub }
      }
    })

    expect(wrapper.findAll('.experience-card')).toHaveLength(1)
    expect(wrapper.get('[data-experience-id="codex-session-migration"]').exists()).toBe(true)
    expect(wrapper.get('[data-category-filter="conversationContinuity"]').classes()).toContain('is-active')
  })
})
