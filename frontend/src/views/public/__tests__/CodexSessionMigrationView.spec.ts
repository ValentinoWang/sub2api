import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, RouterLinkStub } from '@vue/test-utils'
import CodexSessionMigrationView from '../CodexSessionMigrationView.vue'
import { CODEX_SESSION_MIGRATION } from '@/constants/codexMigration'

const writeText = vi.fn()
vi.stubGlobal('navigator', { clipboard: { writeText } })

const PublicPageLayout = { template: '<div><slot /><footer><slot name="footer" /></footer></div>' }

describe('CodexSessionMigrationView', () => {
  it('keeps the public diagnosis, prompt and human explanation in order with real downloads', () => {
    const wrapper = mount(CodexSessionMigrationView, { global: { stubs: { PublicPageLayout, RouterLink: RouterLinkStub } } })
    expect(wrapper.find('h1').text()).toBe(CODEX_SESSION_MIGRATION.title)
    expect(wrapper.findAll('h2').map((heading) => heading.text()).slice(0, 3)).toEqual(['情况说明', 'Codex 帮你处理', '给人看的：原因、证据与经验'])
    expect(wrapper.html()).toContain(CODEX_SESSION_MIGRATION.packageDownload)
    expect(wrapper.html()).toContain(CODEX_SESSION_MIGRATION.manifestDownload)
  })

  it('copies the exact prompt shared by the download artifact', async () => {
    writeText.mockResolvedValue(undefined)
    const wrapper = mount(CodexSessionMigrationView, { global: { stubs: { PublicPageLayout, RouterLink: RouterLinkStub } } })
    await wrapper.get('button').trigger('click')
    await flushPromises()
    expect(writeText).toHaveBeenCalledWith(CODEX_SESSION_MIGRATION.prompt)
  })
})
