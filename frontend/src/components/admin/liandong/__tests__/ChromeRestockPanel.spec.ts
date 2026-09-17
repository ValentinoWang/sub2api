import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import ChromeRestockPanel from '../ChromeRestockPanel.vue'
import type { BrowserRestockStatus } from '@/api/liandongBrowser'
const mocks = vi.hoisted(() => ({ getStatus: vi.fn(), saveConfig: vi.fn(), createDevice: vi.fn(), revokeDevice: vi.fn(), resume: vi.fn(), requestRecheck: vi.fn() }))
vi.mock('@/api/liandongBrowser', () => ({ liandongBrowserAPI: mocks }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
const initial = (): BrowserRestockStatus => ({ enabled: false, paused_reason: '', products: [], devices: [], batches: [] })
async function render() { const wrapper = mount(ChromeRestockPanel); await flushPromises(); return wrapper }
beforeEach(() => { vi.clearAllMocks(); mocks.getStatus.mockResolvedValue(initial()); mocks.saveConfig.mockResolvedValue(initial()) })

afterEach(() => { vi.unstubAllGlobals() })

describe('Chrome restocking management', () => {
  it.each(['localhost', '127.0.0.1'])('opens the dedicated browser helper on local host %s', async (hostname) => {
    vi.stubGlobal('location', new URL(`http://${hostname}:4174/`))
    mocks.getStatus.mockResolvedValue({ ...initial(), recheck_supported: true, devices: [{ id: 'browser', name: '本机 HTTP 补货脚本', revoked: false,
      runtime: { execution_mode: 'browser', state: 'paused', reason: 'verification_required', browser_verification_required: true, reported_at: new Date().toISOString() },
    }] })
    const wrapper = await render()
    try {
      const link = wrapper.get('[data-testid="open-merchant-verification"]')
      expect(wrapper.get('[data-testid="restock-execution-mode"]').text()).toBe('ldxpToolkit.browser.dedicatedBrowserExecutor')
      expect(wrapper.get('[data-testid="restock-runtime-alert"]').text()).toContain('本机 HTTP 补货脚本')
      expect(link.attributes('href')).toBe('http://127.0.0.1:52401/')
      expect(link.text()).toBe('ldxpToolkit.browser.openDedicatedBrowser')
      expect(link.attributes('rel')).toBe('noopener noreferrer')
      expect(wrapper.get('[data-testid="restock-runtime-alert"]').text()).toContain('dedicatedBrowserVerificationHelp')
      expect(wrapper.get('[data-testid="request-merchant-recheck"]').exists()).toBe(true)
    } finally { wrapper.unmount() }
  })
  it.each(['ai.rest2build.lol', 'localhost.example.com', '192.168.1.2'])('does not point a remote page at the current computer: %s', async (hostname) => {
    vi.stubGlobal('location', new URL(`https://${hostname}/`))
    mocks.getStatus.mockResolvedValue({ ...initial(), recheck_supported: true, devices: [{ id: 'browser', name: 'Browser worker', revoked: false,
      runtime: { execution_mode: 'browser', state: 'paused', reason: 'login_required', browser_verification_required: false, reported_at: new Date().toISOString() },
    }] })
    const wrapper = await render()
    try {
      expect(wrapper.find('[data-testid="open-merchant-verification"]').exists()).toBe(false)
      expect(wrapper.get('[data-testid="restock-runtime-alert"]').text()).toContain('dedicatedBrowserRemoteHelp')
      expect(wrapper.get('[data-testid="request-merchant-recheck"]').exists()).toBe(true)
    } finally { wrapper.unmount() }
  })
  it.each(['verification_required', 'non_json', 'login_required', 'login_rejected'])('uses same-browser feedback after a browser-mode recheck fails: %s', async (reason) => {
    mocks.getStatus.mockResolvedValue({ ...initial(), devices: [{ id: 'browser', name: 'Browser worker', revoked: false,
      runtime: { execution_mode: 'browser', state: 'paused', reason, browser_verification_required: reason === 'non_json', reported_at: new Date().toISOString() },
      recheck: { id: 'check-1', device_id: 'browser', state: 'failed', reason, resumed: false, requested_at: new Date().toISOString() },
    }] })
    const wrapper = await render()
    try {
      expect(wrapper.get('[data-testid="merchant-recheck-result"]').text()).toBe('ldxpToolkit.browser.dedicatedBrowserStillBlocked')
      expect(wrapper.text()).not.toContain('ldxpToolkit.browser.recheckStillBlocked')
    } finally { wrapper.unmount() }
  })
  it('keeps the merchant verification link for explicit HTTP mode on remote pages', async () => {
    vi.stubGlobal('location', new URL('https://ai.rest2build.lol/'))
    mocks.getStatus.mockResolvedValue({ ...initial(), devices: [{ id: 'http', name: 'HTTP worker', revoked: false,
      runtime: { execution_mode: 'http', state: 'paused', reason: 'verification_required', browser_verification_required: true, reported_at: new Date().toISOString() },
    }] })
    const wrapper = await render()
    try {
      expect(wrapper.get('[data-testid="open-merchant-verification"]').attributes('href')).toBe('https://www.ldxp.cn/merchant/')
      expect(wrapper.get('[data-testid="open-merchant-verification"]').text()).toBe('ldxpToolkit.browser.openMerchantVerification')
    } finally { wrapper.unmount() }
  })
  it('accepts a fresh report received between display clock ticks', async () => {
    vi.useFakeTimers()
    const device = { id: 'script', name: 'Worker', revoked: false,
      runtime: { state: 'running' as const, reason: '', browser_verification_required: false, reported_at: new Date().toISOString() },
    }
    mocks.getStatus.mockResolvedValue({ ...initial(), devices: [device] })
    const wrapper = await render()
    try {
      await vi.advanceTimersByTimeAsync(5_000)
      mocks.getStatus.mockResolvedValue({ ...initial(), devices: [{ ...device,
        runtime: { ...device.runtime, reported_at: new Date().toISOString() },
      }] })
      await wrapper.findAll('button').find(button => button.text() === 'common.refresh')!.trigger('click')
      await flushPromises()
      expect(wrapper.get('[data-testid="restock-connection-status"]').text()).toContain('connected')
    } finally {
      wrapper.unmount()
      vi.useRealTimers()
    }
  })
  it('places product mappings at the bottom and keeps them collapsed initially', async () => {
    const wrapper = await render()
    const mappings = wrapper.get('[data-testid="restock-product-mappings"]')
    expect(mappings.element.tagName).toBe('DETAILS')
    expect((mappings.element as HTMLDetailsElement).open).toBe(false)
    expect(mappings.get('summary').text()).toContain('ldxpToolkit.browser.products')
    expect(wrapper.element.lastElementChild).toBe(mappings.element)
    wrapper.unmount()
  })
  it('polls a pending request and preserves its result after the alert clears', async () => {
    vi.useFakeTimers()
    const now = new Date().toISOString()
    const device = { id: 'script', name: 'Worker', revoked: false,
      runtime: { state: 'paused' as const, reason: 'non_json', browser_verification_required: true, reported_at: now },
      recheck: { id: 'request-1', device_id: 'script', state: 'checking' as const, reason: '', resumed: false, requested_at: now },
    }
    mocks.getStatus.mockResolvedValue({ ...initial(), recheck_supported: true, devices: [device] })
    const wrapper = await render()
    try {
      expect(wrapper.get('[data-testid="request-merchant-recheck"]').attributes('disabled')).toBeDefined()
      mocks.getStatus.mockResolvedValue({ ...initial(), devices: [{ ...device,
        runtime: { state: 'running', reason: '', browser_verification_required: false, reported_at: now },
        recheck: { ...device.recheck, state: 'passed', resumed: true },
      }] })
      await vi.advanceTimersByTimeAsync(2_000)
      expect(mocks.getStatus).toHaveBeenCalledTimes(2)
      expect(wrapper.find('[data-testid="restock-runtime-alert"]').exists()).toBe(false)
      expect(wrapper.get('[data-testid="merchant-recheck-result"]').text()).toContain('recheckPassed')
      expect(mocks.resume).not.toHaveBeenCalled()
    } finally {
      wrapper.unmount()
      vi.useRealTimers()
    }
  })
  it('requests an immediate check without declaring browser verification successful', async () => {
    const now = new Date().toISOString()
    const device = { id: 'script', name: 'Local worker', revoked: false,
      runtime: { state: 'paused' as const, reason: 'non_json', browser_verification_required: true, reported_at: now },
    }
    mocks.getStatus.mockResolvedValue({ ...initial(), recheck_supported: true, enabled: true, devices: [device] })
    mocks.requestRecheck.mockResolvedValue({ id: 'request-1', device_id: 'script', state: 'queued', reason: '', resumed: false, requested_at: now })
    const wrapper = await render()
    expect(wrapper.find('a[download]').exists()).toBe(false)
    expect(wrapper.find('details').text()).not.toContain('extensionOption')
    await wrapper.get('[data-testid="request-merchant-recheck"]').trigger('click')
    await flushPromises()
    expect(mocks.requestRecheck).toHaveBeenCalledWith('script')
    expect(mocks.resume).not.toHaveBeenCalled()
    expect(wrapper.get('[data-testid="request-merchant-recheck"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-testid="merchant-recheck-result"]').text()).toContain('recheckQueued')
    mocks.getStatus.mockResolvedValue({ ...initial(), recheck_supported: true, enabled: true, devices: [{ ...device,
      recheck: { id: 'request-1', device_id: 'script', state: 'failed', reason: 'non_json', resumed: false, requested_at: now },
    }] })
    await wrapper.findAll('button').find(button => button.text() === 'common.refresh')!.trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="merchant-recheck-result"]').text()).toContain('recheckStillBlocked')
    expect(wrapper.get('[data-testid="restock-runtime-alert"]').text()).toContain('verificationNeeded')
    expect(wrapper.get('[data-testid="request-merchant-recheck"]').attributes('disabled')).toBeUndefined()
    wrapper.unmount()
  })
  it('does not expose an action before the backend supports rechecks', async () => {
    mocks.getStatus.mockResolvedValue({ ...initial(), devices: [{ id: 'script', name: 'Worker', revoked: false,
      runtime: { state: 'paused', reason: 'non_json', browser_verification_required: true, reported_at: new Date().toISOString() },
    }] })
    const wrapper = await render()
    expect(wrapper.find('[data-testid="request-merchant-recheck"]').exists()).toBe(false)
    wrapper.unmount()
  })
  it('surfaces browser verification while the worker reports without merchant authorization', async () => {
    const recent = new Date().toISOString()
    const device = { id: 'script', name: 'HTTP worker', revoked: false,
      last_seen_at: new Date(Date.now() - 600_000).toISOString(),
      runtime: { state: 'paused' as const, reason: 'non_json', browser_verification_required: true,
        reported_at: recent, checked_at: recent, next_check_at: new Date(Date.now() + 60_000).toISOString() },
    }
    mocks.getStatus.mockResolvedValue({ ...initial(), enabled: true, devices: [device] })
    const wrapper = await render()
    expect(wrapper.get('[data-testid="restock-runtime-alert"]').text()).toContain('verificationNeeded')
    expect(wrapper.get('[data-testid="restock-runtime-alert"]').text()).toContain('nextCheckAt')
    expect(wrapper.get('[data-testid="restock-connection-status"]').text()).toContain('workerPaused')
    const link = wrapper.get('[data-testid="open-merchant-verification"]')
    expect(link.attributes('href')).toBe('https://www.ldxp.cn/merchant/')
    expect(link.attributes('rel')).toContain('noopener')
    expect(mocks.resume).not.toHaveBeenCalled()
    mocks.getStatus.mockResolvedValue({ ...initial(), enabled: true, devices: [{ ...device,
      runtime: { state: 'running', reason: '', browser_verification_required: false, reported_at: recent },
    }] })
    await wrapper.findAll('button').find(button => button.text() === 'common.refresh')!.trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="restock-runtime-alert"]').exists()).toBe(false)
    expect(mocks.resume).not.toHaveBeenCalled()
    wrapper.unmount()
  })
  it('marks an old verification report stale without claiming the worker is online', async () => {
    mocks.getStatus.mockResolvedValue({ ...initial(), enabled: true, devices: [{ id: 'script', name: 'HTTP worker', revoked: false,
      runtime: { state: 'paused', reason: 'non_json', browser_verification_required: true, reported_at: new Date(Date.now() - 600_000).toISOString() },
    }] })
    const wrapper = await render()
    expect(wrapper.get('[data-testid="restock-runtime-alert"]').text()).toContain('oldRuntimeReport')
    expect(wrapper.get('[data-testid="restock-connection-status"]').text()).toContain('noConnection')
    wrapper.unmount()
  })
  it('does not label generic HTML as a confirmed slider or show a revoked device alert', async () => {
    const runtime = { state: 'paused' as const, reason: 'non_json', browser_verification_required: false, reported_at: new Date().toISOString() }
    mocks.getStatus.mockResolvedValue({ ...initial(), devices: [
      { id: 'script', name: 'HTTP worker', revoked: false, runtime },
      { id: 'old', name: 'Old worker', revoked: true, runtime: { ...runtime, browser_verification_required: true } },
    ] })
    const wrapper = await render()
    expect(wrapper.findAll('[data-testid="restock-runtime-alert"]')).toHaveLength(1)
    expect(wrapper.get('[data-testid="restock-runtime-alert"]').text()).toContain('upstreamPage')
    expect(wrapper.get('[data-testid="restock-runtime-alert"]').text()).not.toContain('verificationNeeded')
    wrapper.unmount()
  })
  it('shows the enabled switch separately from an offline worker and stale inventory', async () => {
    mocks.getStatus.mockResolvedValue({ ...initial(), enabled: true,
      devices: [{ id: 'script', name: 'HTTP worker', revoked: false, last_seen_at: new Date(Date.now() - 130_000).toISOString(), authorization_verified_at: new Date().toISOString() }],
      products: [{ goods_id: 101, cny_amount: 5, usd_credit: 5, external_url: 'https://wzyp.cn/item/101', target_stock: 999, batch_size: 20, enabled: true, current_stock: 999, identity_verified: true, inventory_at: new Date(Date.now() - 130_000).toISOString() }],
    })
    const wrapper = await render()
    expect(wrapper.get('[data-testid="restock-enabled-status"]').text()).toContain('browser.enabled')
    expect(wrapper.get('[data-testid="restock-connection-status"]').text()).toContain('noConnection')
    expect(wrapper.get('[data-testid="restock-stock-status"]').text()).toBe('0 / 1')
    wrapper.unmount()
  })
  it('polls real status without overwriting unsaved product edits and stops polling on unmount', async () => {
    vi.useFakeTimers()
    const recent = new Date().toISOString()
    mocks.getStatus.mockResolvedValue({ ...initial(), enabled: true,
      devices: [{ id: 'script', name: 'HTTP worker', revoked: false, last_seen_at: recent, authorization_verified_at: recent }],
      products: [{ goods_id: 101, cny_amount: 5, usd_credit: 5, external_url: 'https://wzyp.cn/item/101', target_stock: 999, batch_size: 20, enabled: true, current_stock: 999, identity_verified: true, inventory_at: recent }],
    })
    const wrapper = mount(ChromeRestockPanel)
    try {
      await flushPromises()
      expect(wrapper.get('[data-testid="restock-connection-status"]').text()).toContain('connected')
      expect(wrapper.get('[data-testid="restock-stock-status"]').text()).toBe('1 / 1')
      await wrapper.get('[data-testid="chrome-product"]').findAll('input')[1].setValue(20)
      await vi.advanceTimersByTimeAsync(15_000)
      expect(mocks.getStatus).toHaveBeenCalledTimes(2)
      expect((wrapper.get('[data-testid="chrome-product"]').findAll('input')[1].element as HTMLInputElement).value).toBe('20')
      mocks.getStatus.mockRejectedValueOnce(new Error('offline'))
      await vi.advanceTimersByTimeAsync(15_000)
      expect(wrapper.get('[data-testid="restock-connection-status"]').text()).toContain('statusUnknown')
      expect(wrapper.get('[data-testid="restock-stock-status"]').text()).toBe('— / 1')
      wrapper.unmount()
      const calls = mocks.getStatus.mock.calls.length
      await vi.advanceTimersByTimeAsync(30_000)
      expect(mocks.getStatus).toHaveBeenCalledTimes(calls)
    } finally {
      wrapper.unmount()
      vi.useRealTimers()
    }
  })
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
