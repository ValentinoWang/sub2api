import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, describe, expect, it, vi } from 'vitest'
import CodexMemoryDocsView from '@/views/public/CodexMemoryDocsView.vue'
import releaseManifest from '../../../../public/codex-memory-release-manifest.json'

vi.mock('@/i18n', () => ({
  getLocale: () => 'zh',
  i18n: { global: { t: (key: string) => key } },
}))

describe('CodexMemoryDocsView', () => {
  setActivePinia(createPinia())

  afterEach(() => {
    vi.unstubAllGlobals()
    delete (document as typeof document & { execCommand?: unknown }).execCommand
  })

  it('renders the fork-maintained Markdown source and an explicit unpublished state', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 404 }))
    const wrapper = mount(CodexMemoryDocsView, {
      global: {
        stubs: {
          RouterLink: { template: '<a><slot /></a>' },
          Icon: true,
          CommandBlock: true,
        },
      },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('当前部署尚未提供经过校验的 GitHub Release 清单')
    expect(wrapper.text()).toContain('原始 Sub2API 父仓库不是该功能的运行依赖')
    expect(wrapper.text()).not.toContain('Obsidian')
  })

  it('renders downloads from the generated release manifest', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: async () => releaseManifest }))
    const wrapper = mount(CodexMemoryDocsView, {
      global: {
        stubs: {
          RouterLink: { template: '<a><slot /></a>' },
          Icon: true,
          CommandBlock: true,
        },
      },
    })
    await flushPromises()

    expect(wrapper.text()).toContain(`v${releaseManifest.version}`)
    expect(wrapper.text()).toContain(releaseManifest.assets[0].filename)
    expect(wrapper.findAll(`a[href*="${releaseManifest.tag}"]`)).toHaveLength(4)
    expect(wrapper.text()).not.toContain('当前部署尚未提供经过校验')
  })

  it('puts one provider-independent Codex instruction before the manual details', async () => {
    Object.defineProperty(window, 'isSecureContext', { value: false, configurable: true })
    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText: vi.fn() },
      configurable: true,
    })
    const execCommand = vi.fn().mockReturnValue(true)
    Object.defineProperty(document, 'execCommand', { value: execCommand, configurable: true })
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: async () => releaseManifest }))
    const wrapper = mount(CodexMemoryDocsView, {
      global: {
        stubs: {
          RouterLink: { template: '<a><slot /></a>' },
          Icon: true,
        },
      },
    })
    await flushPromises()

    const firstCommand = wrapper.findComponent({ name: 'CommandBlock' })
    expect(firstCommand.props('title')).toBe('复制整段指令')
    expect(firstCommand.props('command')).toContain('/codex-memory-release-manifest.json')
    expect(firstCommand.props('command')).toContain('只合并 memories、sessions 和 archived_sessions')
    expect(firstCommand.props('command')).toContain('不要改写 config.toml')
    expect(wrapper.find('details').attributes('open')).toBeUndefined()

    await firstCommand.find('button').trigger('click')
    expect(execCommand).toHaveBeenCalledWith('copy')
    expect(firstCommand.find('button').attributes('title')).toBe('已复制')
  })
})
