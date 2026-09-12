import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, RouterLinkStub } from '@vue/test-utils'
import WslCodexTroubleshootingView from '../WslCodexTroubleshootingView.vue'
import { wsl2CodexEnvironmentPrompt } from '@/content/experiences'

const writeText = vi.fn()
vi.stubGlobal('navigator', { clipboard: { writeText } })

const PublicPageLayout = { template: '<div><slot /><footer><slot name="footer" /></footer></div>' }

function mountExperience() {
  return mount(WslCodexTroubleshootingView, {
    global: { stubs: { PublicPageLayout, RouterLink: RouterLinkStub } }
  })
}

describe('WslCodexTroubleshootingView', () => {
  it('keeps environment troubleshooting separate from the frontend tutorial', () => {
    const wrapper = mountExperience()

    expect(wrapper.findAll('h2').map((heading) => heading.text()).slice(0, 3)).toEqual([
      '情况说明',
      'Codex 帮你处理',
      '给人看的：原因、证据与经验'
    ])
    expect(wrapper.text()).toContain('PowerShell 和 WSL Shell 各自检查什么？')
    expect(wrapper.text()).toContain('真实 Windows / WSL2 实操待验证')
    expect(wrapper.text()).not.toContain('让 Codex 为你的项目增加 About 页面')
    expect(wrapper.find('.brand-tagline').text()).toBe('歇一会儿，让 AI 接着干。')
  })

  it('copies the complete standalone troubleshooting prompt', async () => {
    writeText.mockResolvedValue(undefined)
    const wrapper = mountExperience()

    expect((wrapper.get('textarea').element as HTMLTextAreaElement).value).toBe(wsl2CodexEnvironmentPrompt)
    await wrapper.get('.copy-button').trigger('click')
    await flushPromises()

    expect(writeText).toHaveBeenCalledWith(wsl2CodexEnvironmentPrompt)
    expect(wrapper.get('[role="status"]').text()).toBe('提示词已复制')
  })

  it('selects the prompt when clipboard access fails', async () => {
    writeText.mockRejectedValueOnce(new Error('clipboard denied'))
    const select = vi.spyOn(HTMLTextAreaElement.prototype, 'select')
    const wrapper = mountExperience()

    await wrapper.get('.copy-button').trigger('click')
    await flushPromises()

    expect(select).toHaveBeenCalledOnce()
    expect(wrapper.get('[role="status"]').text()).toBe('自动复制未成功，提示词已选中，可手动复制')
    select.mockRestore()
  })
})
