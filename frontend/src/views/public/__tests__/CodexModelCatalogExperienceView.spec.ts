import { flushPromises, mount, RouterLinkStub } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import CodexModelCatalogExperienceView from '../CodexModelCatalogExperienceView.vue'

const render = () => mount(CodexModelCatalogExperienceView, {
  global: { stubs: {
    PublicPageLayout: { template: '<div><slot /><slot name="footer" /></div>' },
    Rest2BuildBrandFooter: true, RouterLink: RouterLinkStub,
  } },
})

describe('Codex model catalog guide', () => {
  afterEach(() => vi.restoreAllMocks())

  it('shows the problem before the solution and the real script link', () => {
    const wrapper = render()
    expect(wrapper.findAll('.content-section h2').map(h => h.text())).toEqual(['问题说明', '解决方案', '原因、验证与注意事项'])
    expect(wrapper.get('a[href="/downloads/sync-codex-model-catalog.py"]').exists()).toBe(true)
    expect(wrapper.get('.prompt-download a').attributes('href')).toBe('/downloads/sync-codex-model-catalog.py')
    expect(wrapper.text()).toContain('首个 [section] 之前')
    expect(wrapper.text()).not.toContain('/Users/vsiyo')
  })

  it('copies the same complete prompt as the published article', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText } })
    const wrapper = render()
    await wrapper.get('.copy-button').trigger('click')
    await flushPromises()
    const displayed = wrapper.get<HTMLTextAreaElement>('.prompt-field').element.value
    expect(writeText).toHaveBeenCalledWith(displayed)
    expect(displayed).toContain(`${window.location.origin}/downloads/sync-codex-model-catalog.py`)
    expect(displayed).toContain(`${window.location.origin}/error-experiences/codex-model-catalog-context-window`)
    expect(wrapper.get('.copy-status').text()).toBe('提示词已复制')
  })

  it('selects the prompt and reports failure when copying is unavailable', async () => {
    Object.defineProperty(navigator, 'clipboard', { configurable: true, value: undefined })
    const wrapper = render()
    const field = wrapper.get<HTMLTextAreaElement>('.prompt-field').element
    const select = vi.spyOn(field, 'select')
    await wrapper.get('.copy-button').trigger('click')
    await flushPromises()
    expect(select).toHaveBeenCalledOnce()
    expect(field.value).toContain(`${window.location.origin}/downloads/sync-codex-model-catalog.py`)
    expect(wrapper.get('.copy-status').text()).toContain('可手动复制')
  })
})
