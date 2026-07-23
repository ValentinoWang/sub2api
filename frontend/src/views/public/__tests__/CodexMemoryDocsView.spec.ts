import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import CodexMemoryDocsView from '@/views/public/CodexMemoryDocsView.vue'
import releaseManifest from '../../../../public/codex-memory-release-manifest.json'

vi.mock('@/i18n', () => ({ getLocale: () => 'zh' }))

describe('CodexMemoryDocsView', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
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
})
