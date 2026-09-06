import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import ErrorExperienceView from '../ErrorExperienceView.vue'

const writeText = vi.fn()

vi.stubGlobal('navigator', { clipboard: { writeText } })

const PublicPageLayout = {
  template: '<div><slot /><footer><slot name="footer" /></footer></div>'
}

describe('ErrorExperienceView', () => {
  it('renders one complete experience with the prompt-first order and company footer', () => {
    const wrapper = mount(ErrorExperienceView, {
      global: { stubs: { PublicPageLayout } }
    })

    expect(wrapper.find('h1').text()).toBe('GPT-6 已接入，为什么 Codex 仍然看不见？')
    expect(wrapper.findAll('h2').map((heading) => heading.text()).slice(0, 3)).toEqual([
      '情况说明',
      'Codex 帮你处理',
      '给人看的：原因、证据与经验'
    ])
    expect(wrapper.find('.brand-tagline').text()).toBe('歇一会儿，让 AI 接着干。')
    expect(wrapper.find('.brand-service').text()).toContain('面向 Codex、Claude Code 等工具的 AI 模型接入服务')
  })

  it('copies the full Codex prompt', async () => {
    writeText.mockResolvedValue(undefined)
    const wrapper = mount(ErrorExperienceView, {
      global: { stubs: { PublicPageLayout } }
    })

    const prompt = (wrapper.find('textarea').element as HTMLTextAreaElement).value
    await wrapper.get('button').trigger('click')
    await flushPromises()

    expect(writeText).toHaveBeenCalledWith(prompt)
    expect(wrapper.find('[role="status"]').text()).toBe('提示词已复制')
  })

  it('selects the prompt for manual copying when clipboard access is unavailable', async () => {
    writeText.mockRejectedValueOnce(new Error('clipboard denied'))
    const select = vi.spyOn(HTMLTextAreaElement.prototype, 'select')
    const wrapper = mount(ErrorExperienceView, {
      global: { stubs: { PublicPageLayout } }
    })

    await wrapper.get('button').trigger('click')
    await flushPromises()

    expect(select).toHaveBeenCalledOnce()
    expect(wrapper.find('[role="status"]').text()).toBe('自动复制未成功，提示词已选中，可手动复制')
    select.mockRestore()
  })
})
