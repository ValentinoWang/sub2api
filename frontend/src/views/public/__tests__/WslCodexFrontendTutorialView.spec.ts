import { createPinia, setActivePinia } from 'pinia'
import { mount, RouterLinkStub } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import WslCodexFrontendTutorialView from '../WslCodexFrontendTutorialView.vue'

const PublicPageLayout = { template: '<main><slot /><footer><slot name="footer" /></footer></main>' }

function mountTutorial() {
  const pinia = createPinia()
  setActivePinia(pinia)
  return mount(WslCodexFrontendTutorialView, {
    global: {
      plugins: [pinia],
      stubs: {
        PublicPageLayout,
        Rest2BuildBrandFooter: { template: '<div />' },
        RouterLink: RouterLinkStub,
      },
    },
  })
}

describe('WslCodexFrontendTutorialView', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    Object.defineProperty(window, 'isSecureContext', { configurable: true, value: true })
  })

  it('renders the complete beginner workflow with distinct terminal and Codex prompt cards', () => {
    const wrapper = mountTutorial()

    expect(wrapper.get('h1').text()).toContain('第一次真实前端开发')
    expect(wrapper.get('.article-guide').text()).toContain('这不是一张需要背下来的命令清单')
    expect(wrapper.findAll('.step')).toHaveLength(10)
    expect(wrapper.findAll('.input-terminal').length).toBeGreaterThan(1)
    expect(wrapper.findAll('.input-prompt').length).toBeGreaterThan(1)
    expect(wrapper.get('.input-terminal .input-label').text()).toContain('WSL 终端命令')
    expect(wrapper.get('.input-prompt .input-label').text()).toContain('发给 Codex')
    expect(wrapper.text()).toContain('不要把 PowerShell、CMD 和 WSL Shell 混为一谈')
    expect(wrapper.text()).toContain('先调查，再动手')
    expect(wrapper.text()).toContain('/review')
    expect(wrapper.text()).toContain('验收通过')
    expect(wrapper.get('a[href="https://learn.chatgpt.com/docs/windows/wsl"]').attributes('target')).toBe('_blank')
  })

  it('copies a command and reports success', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText } })
    const wrapper = mountTutorial()

    await wrapper.get('.input-terminal .copy-button').trigger('click')
    await Promise.resolve()

    expect(writeText).toHaveBeenCalledWith('echo $WSL_DISTRO_NAME\npwd\nuname -a')
    expect(wrapper.get('[role="status"]').text()).toContain('内容已复制')
  })

  it('shows a manual-copy instruction when clipboard access fails', async () => {
    Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText: vi.fn().mockRejectedValue(new Error('denied')) } })
    document.execCommand = vi.fn().mockReturnValue(false)
    const wrapper = mountTutorial()

    await wrapper.get('.input-terminal .copy-button').trigger('click')
    await Promise.resolve()

    expect(wrapper.get('[role="status"]').text()).toContain('复制失败')
  })
})
