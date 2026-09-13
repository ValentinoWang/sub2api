import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ChromeRestockPanel from '../ChromeRestockPanel.vue'
import type { BrowserRestockStatus } from '@/api/liandongBrowser'
const mocks = vi.hoisted(() => ({ getStatus: vi.fn(), saveConfig: vi.fn(), createDevice: vi.fn(), revokeDevice: vi.fn(), resume: vi.fn() }))
vi.mock('@/api/liandongBrowser', () => ({ liandongBrowserAPI: mocks }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
const initial = (): BrowserRestockStatus => ({ enabled: false, paused_reason: '', products: [], devices: [], batches: [] })
async function render() { const wrapper = mount(ChromeRestockPanel); await flushPromises(); return wrapper }
beforeEach(() => { vi.clearAllMocks(); mocks.getStatus.mockResolvedValue(initial()); mocks.saveConfig.mockResolvedValue(initial()) })

describe('Chrome restocking management', () => {
  it('reports an unavailable endpoint without presenting a usable enable action', async () => {
    mocks.getStatus.mockRejectedValueOnce(new Error('Unavailable'))
    const wrapper = await render()
    expect(wrapper.get('[role="alert"]').text()).toContain('loadError')
    expect(wrapper.find('[data-testid="chrome-toggle"]').exists()).toBe(false)
  })
  it('creates a limited device credential once and hides it on request', async () => {
    mocks.getStatus.mockResolvedValue({ ...initial(), products: [{ goods_id: 101, cny_amount: 5, usd_credit: 5, external_url: 'https://wzyp.cn/item/101', target_stock: 20, batch_size: 10, enabled: true }] })
    mocks.createDevice.mockResolvedValue({ device: { id: 'device-1', name: 'Office Chrome', revoked: false }, device_key: 'fixture-device-secret' })
    const wrapper = await render()
    await wrapper.get('input[aria-label="ldxpToolkit.browser.deviceName"]').setValue('Office Chrome')
    await wrapper.get('[data-testid="chrome-create-device"]').trigger('submit')
    await flushPromises()
    expect(mocks.createDevice).toHaveBeenCalledWith('Office Chrome', undefined)
    expect((wrapper.get('textarea').element as HTMLTextAreaElement).value).toBe('fixture-device-secret')
    expect(wrapper.get('[data-testid="chrome-create-device"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="chrome-device-key"] button').trigger('click')
    expect(wrapper.find('textarea').exists()).toBe(false)
    expect(localStorage.getItem('fixture-device-secret')).toBeNull()
  })
  it('limits a new device to the explicitly selected product', async () => {
    mocks.getStatus.mockResolvedValue({ ...initial(), products: [{ goods_id: 101, cny_amount: 5, usd_credit: 5, external_url: 'https://wzyp.cn/item/101', target_stock: 20, batch_size: 10, enabled: true }] })
    mocks.createDevice.mockResolvedValue({ device: { id: 'device-1', name: 'Office', goods_ids: [101], revoked: false }, device_key: 'fixture-scoped-key' })
    const wrapper = await render()
    await wrapper.get('input[aria-label="ldxpToolkit.browser.deviceName"]').setValue('Office')
    await wrapper.get('select').setValue('101')
    await wrapper.get('[data-testid="chrome-create-device"]').trigger('submit')
    await flushPromises()
    expect(mocks.createDevice).toHaveBeenCalledWith('Office', [101])
  })
  it('only submits fixed 1:1 credit mappings and preserves the enabled switch', async () => {
    mocks.getStatus.mockResolvedValue({ ...initial(), products: [{ goods_id: 101, cny_amount: 5, usd_credit: 5, external_url: 'https://wzyp.cn/item/101', target_stock: 20, batch_size: 10, enabled: true, current_stock: 4, identity_verified: true }] })
    const wrapper = await render()
    const fields = wrapper.get('[data-testid="chrome-product"]').findAll('input')
    await fields[1].setValue(20)
    await wrapper.get('[data-testid="chrome-save"]').trigger('submit')
    await flushPromises()
    expect(mocks.saveConfig).toHaveBeenCalledWith({ enabled: false, products: [{ goods_id: 101, cny_amount: 20, usd_credit: 20, external_url: 'https://wzyp.cn/item/101', target_stock: 20, batch_size: 10, enabled: true }] })
  })
  it('allows stopping with unsaved edits without publishing those edits', async () => {
    const product = { goods_id: 101, cny_amount: 5, usd_credit: 5, external_url: 'https://wzyp.cn/item/101', target_stock: 20, batch_size: 10, enabled: true }
    mocks.getStatus.mockResolvedValue({ ...initial(), enabled: true, products: [product] })
    mocks.saveConfig.mockResolvedValue({ ...initial(), products: [product] })
    const wrapper = await render()
    await wrapper.get('[data-testid="chrome-product"]').findAll('input')[1].setValue(20)
    await wrapper.get('[data-testid="chrome-toggle"]').trigger('click')
    await flushPromises()
    expect(mocks.saveConfig).toHaveBeenCalledWith({ enabled: false, products: [product] })
    expect((wrapper.get('[data-testid="chrome-product"]').findAll('input')[1].element as HTMLInputElement).value).toBe('20')
  })
  it('does not claim success for rejected enable requests', async () => {
    mocks.saveConfig.mockRejectedValueOnce(new Error('Inventory not verified'))
    const wrapper = await render()
    await wrapper.get('[data-testid="chrome-toggle"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('operationError')
    expect(wrapper.text()).toContain('ldxpToolkit.browser.disabled')
  })
  it('labels a previously authorized browser offline after two minutes without a heartbeat', async () => {
    mocks.getStatus.mockResolvedValue({ ...initial(), devices: [{ id: 'device-1', name: 'Office Chrome', revoked: false, last_seen_at: new Date(Date.now() - 121_000).toISOString(), authorization_verified_at: new Date().toISOString() }] })
    const wrapper = await render()
    expect(wrapper.text()).toContain('ldxpToolkit.browser.offline')
    expect(wrapper.text()).not.toContain('ldxpToolkit.browser.authorized')
    wrapper.unmount()
  })
  it('keeps failed authorization visibly paused ahead of the offline label', async () => {
    mocks.getStatus.mockResolvedValue({ ...initial(), devices: [{ id: 'device-1', name: 'Office Chrome', revoked: false, paused_reason: 'authorization_failed', last_seen_at: new Date(Date.now() - 121_000).toISOString() }] })
    const wrapper = await render()
    expect(wrapper.text()).toContain('ldxpToolkit.browser.paused')
    expect(wrapper.text()).not.toContain('ldxpToolkit.browser.offline')
    wrapper.unmount()
  })
  it('distinguishes pending batch outcomes from stock identity without revealing codes', async () => {
    mocks.getStatus.mockResolvedValue({ ...initial(), batches: [
      { batch_id: 'claimed-fixture', goods_id: 101, status: 'claimed', code_count: 10, created_at: '2026-01-01', codes: ['MUST-NOT-RENDER'] },
      { batch_id: 'uncertain-fixture', goods_id: 102, status: 'uncertain', code_count: 5, created_at: '2026-01-01' },
      { batch_id: 'verified-fixture', goods_id: 103, status: 'verified', code_count: 20, created_at: '2026-01-01' },
    ] })
    const wrapper = await render()
    const batches = wrapper.get('[data-testid="chrome-pending-batches"]')
    expect(batches.text()).toContain('ldxpToolkit.browser.batchClaimed')
    expect(batches.text()).toContain('ldxpToolkit.browser.batchUncertain')
    expect(batches.text()).not.toContain('MUST-NOT-RENDER')
    expect(batches.text()).not.toContain('verified-fixture')
    expect(batches.findAll('.badge')).toHaveLength(2)
    wrapper.unmount()
  })
  it('revokes a device and clears the one-time credential display', async () => {
    mocks.getStatus.mockResolvedValue({ ...initial(), devices: [{ id: 'device-1', name: 'Office Chrome', revoked: false }] })
    mocks.revokeDevice.mockResolvedValue({ revoked: true })
    const wrapper = await render()
    const button = wrapper.findAll('button').find(button => button.text() === 'ldxpToolkit.browser.revoke')!
    await button.trigger('click'); await flushPromises()
    expect(mocks.revokeDevice).toHaveBeenCalledWith('device-1')
    expect(wrapper.text()).toContain('ldxpToolkit.browser.revoked')
  })
})
