import { flushPromises, mount, RouterLinkStub } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ClaudeCodeExperienceView from '../ClaudeCodeExperienceView.vue'

const PublicPageLayout = { template: '<div><slot /><slot name="footer" /></div>' }

describe('ClaudeCodeExperienceView', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it.each([
    ['claude-code-bypass-permissions', 'ERR-004', 'claude --permission-mode manual'],
    ['claude-code-fable-5-1-not-visible', 'ERR-005', 'CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY=1'],
  ] as const)('shows the problem before the solution for %s', (experienceId, caseNumber, expectedText) => {
    const wrapper = mount(ClaudeCodeExperienceView, {
      props: { experienceId, caseNumber },
      global: { stubs: { PublicPageLayout, Rest2BuildBrandFooter: true, RouterLink: RouterLinkStub, Icon: true } },
    })

    const sections = wrapper.findAll('.content-section h2').map((heading) => heading.text())
    expect(sections.slice(0, 3)).toEqual(['问题说明', '解决方案', '原因、验证与注意事项'])
    expect(wrapper.text()).toContain(expectedText)
    expect(wrapper.text()).not.toContain('产品 0.2.4')
    expect(wrapper.text()).not.toContain('技术 v0.2.5')
  })

  it('reports clipboard success', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText } })
    const wrapper = mount(ClaudeCodeExperienceView, {
      props: { experienceId: 'claude-code-fable-5-1-not-visible', caseNumber: 'ERR-005' },
      global: { stubs: { PublicPageLayout, Rest2BuildBrandFooter: true, RouterLink: RouterLinkStub, Icon: true } },
    })

    await wrapper.get('.copy-button').trigger('click')
    await flushPromises()

    expect(writeText).toHaveBeenCalledOnce()
    expect(wrapper.get('.copy-status').text()).toBe('提示词已复制')
  })

  it('selects the prompt and reports the clipboard fallback', async () => {
    Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText: vi.fn().mockRejectedValue(new Error('denied')) } })
    const wrapper = mount(ClaudeCodeExperienceView, {
      props: { experienceId: 'claude-code-bypass-permissions', caseNumber: 'ERR-004' },
      global: { stubs: { PublicPageLayout, Rest2BuildBrandFooter: true, RouterLink: RouterLinkStub, Icon: true } },
    })
    const field = wrapper.get<HTMLTextAreaElement>('.prompt-field')
    const select = vi.spyOn(field.element, 'select')

    await wrapper.get('.copy-button').trigger('click')
    await flushPromises()

    expect(select).toHaveBeenCalledOnce()
    expect(wrapper.get('.copy-status').text()).toContain('可手动复制')
  })
})
