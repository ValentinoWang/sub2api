import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import LiandongToolkitView from '../LiandongToolkitView.vue'
const api = vi.hoisted(() => ({ getStatus: vi.fn(), saveConfig: vi.fn(), createDevice: vi.fn(), revokeDevice: vi.fn(), resume: vi.fn() }))
vi.mock('@/api/liandongBrowser', () => ({ liandongBrowserAPI: api }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<main><slot /></main>' } }))
vi.mock('vue-i18n', async () => {
  const { default: messages } = await import('@/i18n/locales/zh/misc')
  return { useI18n: () => ({ t: (key: string) => key.split('.').reduce<unknown>((value, part) => value && typeof value === 'object' ? (value as Record<string, unknown>)[part] : undefined, messages) ?? key }) }
})
function render() { return mount(LiandongToolkitView) }
beforeEach(() => { vi.clearAllMocks(); api.getStatus.mockResolvedValue({ enabled: false, paused_reason: '', products: [], devices: [], batches: [] }) })
describe('Liandong single restocking screen', () => {
  it('loads only browser configuration and exposes one mapping/device workflow', async () => {
    const wrapper = render(); await flushPromises()
    expect(api.getStatus).toHaveBeenCalledTimes(1)
    expect(wrapper.find('h1').exists()).toBe(false)
    expect(wrapper.findAll('[data-testid="chrome-restock"]')).toHaveLength(1)
    expect(wrapper.text()).toContain('默认每种额度 999 张')
    expect(wrapper.text()).not.toMatch(/安装 \/ 修复|Merchant-Token|预览与执行|预览补货|同步远程商品|默认 50000/)
    expect(wrapper.findAll('input[type="password"]')).toHaveLength(0)
    expect(api.saveConfig).not.toHaveBeenCalled()
    wrapper.unmount()
  })
  it('shows unavailable browser backend without a legacy fallback or false enabled state', async () => {
    api.getStatus.mockRejectedValueOnce(new Error('unavailable'))
    const wrapper = render(); await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('补货状态暂不可用')
    expect(wrapper.find('[data-testid="chrome-toggle"]').exists()).toBe(false)
    expect(api.saveConfig).not.toHaveBeenCalled()
    wrapper.unmount()
  })
})
