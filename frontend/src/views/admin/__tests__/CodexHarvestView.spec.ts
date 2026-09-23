import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import CodexHarvestView from '../CodexHarvestView.vue'
import type { CodexHarvestSnapshot, CodexHarvestSpeed } from '@/api/admin/codexHarvest'

const { getSnapshot, updateControls, listNodes, resetNodes, startManual, getAllWithCount, testProxy, showError, showSuccess } = vi.hoisted(() => ({
  getSnapshot: vi.fn(),
  updateControls: vi.fn(),
  listNodes: vi.fn(),
  resetNodes: vi.fn(),
  startManual: vi.fn(),
  getAllWithCount: vi.fn(),
  testProxy: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    codexHarvest: { getSnapshot, updateControls, listNodes, resetNodes, startManual },
    proxies: { getAllWithCount, testProxy }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError, showSuccess })
}))

vi.mock('vue-i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-i18n')>()),
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) => (params ? `${key}:${JSON.stringify(params)}` : key),
    te: () => true
  })
}))

const presets: Record<string, CodexHarvestSpeed> = {
  slow: { round_interval_seconds: 45, probe_interval_seconds: 5, attempt_timeout_seconds: 25, cooldown_seconds: 120, max_requests_per_round: 4, max_proxy_attempts: 2, max_requests_per_account_hour: 30 },
  standard: { round_interval_seconds: 20, probe_interval_seconds: 2, attempt_timeout_seconds: 25, cooldown_seconds: 60, max_requests_per_round: 8, max_proxy_attempts: 3, max_requests_per_account_hour: 60 },
  fast: { round_interval_seconds: 10, probe_interval_seconds: 1, attempt_timeout_seconds: 20, cooldown_seconds: 30, max_requests_per_round: 16, max_proxy_attempts: 3, max_requests_per_account_hour: 90 },
  burst: { round_interval_seconds: 5, probe_interval_seconds: 0, attempt_timeout_seconds: 15, cooldown_seconds: 10, max_requests_per_round: 30, max_proxy_attempts: 5, max_requests_per_account_hour: 180 }
}

const bound = { min: 0, max: 600 }

function snapshot(overrides: Partial<CodexHarvestSnapshot> = {}): CodexHarvestSnapshot {
  return {
    generated_at: '2026-09-23T03:00:00Z',
    enabled: true,
    fail_closed: true,
    models: ['gpt-6-astra', 'gpt-5.6-sol'],
    target_length: 292,
    controls: { version: 1, preset: 'standard', speed: { ...presets.standard }, proxy_ids: [9] },
    configured: true,
    presets,
    preset_order: ['slow', 'standard', 'fast', 'burst'],
    bounds: {
      round_interval_seconds: bound,
      probe_interval_seconds: bound,
      attempt_timeout_seconds: bound,
      cooldown_seconds: bound,
      max_requests_per_round: bound,
      max_proxy_attempts: bound,
      max_requests_per_account_hour: bound
    },
    pool: [{ proxy_id: 9, name: 'old-node', usable: false, reason: 'inactive' }],
    runtime: { running: false, requests_used: 0, request_budget: 8, idle_reason: 'empty_pool' },
    accounts: [
      {
        id: 41,
        name: 'pro-1',
        status: 'active',
        schedulable: true,
        hour_used: 3,
        hour_limit: 60,
        tickets: [
          { model: 'gpt-6-astra', ready: true, blocked: false, length: 292, remaining_seconds: 120, proxy_name: 'res-1' },
          { model: 'gpt-5.6-sol', ready: false, blocked: true, remaining_seconds: 0 }
        ]
      }
    ],
    events: [
      { id: 'e1', at: '2026-09-23T02:59:00Z', stage: 'probe', kind: 'probe_miss', account_name: 'pro-1', model: 'gpt-6-astra', proxy_name: 'res-1', result: 'invalid_state', detail: 'first' },
      { id: 'e2', at: '2026-09-23T02:59:30Z', stage: 'ticket', kind: 'accept', account_name: 'pro-1', model: 'gpt-6-astra', proxy_name: 'res-1', result: 'stored', accepted: true, manual: true, detail: 'second' }
    ],
    ...overrides
  }
}

function mountView() {
  return mount(CodexHarvestView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        BaseDialog: { props: ['show', 'title'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' },
        ConfirmDialog: {
          props: ['show'],
          emits: ['confirm', 'cancel'],
          template: '<div v-if="show" data-testid="confirm"><button data-testid="confirm-yes" @click="$emit(\'confirm\')" /></div>'
        },
        Pagination: true,
        Icon: true,
        RouterLink: { props: ['to'], template: '<a :href="to"><slot /></a>' }
      }
    }
  })
}

describe('打票管理页面', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getSnapshot.mockResolvedValue(snapshot())
    getAllWithCount.mockResolvedValue([
      { id: 1, name: 'res-1', protocol: 'http', host: 'mihomo', port: 20001, status: 'active', ip_address: '203.0.113.7', country: 'SG', latency_ms: 88 },
      { id: 2, name: 'res-2', protocol: 'socks5h', host: 'res.example', port: 1080, status: 'active' }
    ])
    listNodes.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, pages: 0 })
    updateControls.mockImplementation(async (controls) => {
      getSnapshot.mockResolvedValue(snapshot({ controls }))
      return controls
    })
    startManual.mockResolvedValue({ account_id: 41, running: true, attempts: 0, models: [], harvested: [], started_at: '' })
    resetNodes.mockResolvedValue(undefined)
    testProxy.mockResolvedValue({ success: true, message: 'ok', ip_address: '198.51.100.4', country: 'JP', city: 'Tokyo' })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('shows the pool, idle reason, account tickets and newest events first', async () => {
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.get('[data-testid="harvest-idle"]').text()).toBe('admin.codexHarvest.idle.empty_pool')
    expect(wrapper.get('[data-testid="pool-proxy-1"]').text()).toContain('203.0.113.7 · SG')
    expect(wrapper.get('[data-testid="pool-proxy-2"]').text()).toContain('admin.codexHarvest.pool.unknownExit')
    expect(wrapper.get('[data-testid="pool-unusable-9"]').text()).toContain('admin.codexHarvest.pool.reasons.inactive')
    const account = wrapper.get('[data-testid="harvest-account-41"]').text()
    expect(account).toContain('admin.codexHarvest.accounts.ready:{"seconds":120}')
    expect(account).toContain('admin.codexHarvest.accounts.blocked')
    expect(account).toContain('3/60')
    const events = wrapper.findAll('[data-testid="harvest-event"]')
    expect(events[0].text()).toContain('second')
    expect(events[0].text()).toContain('admin.codexHarvest.events.manual')
    expect(events[1].text()).toContain('first')
  })

  it('saves the selected pool with a preset, and marks edited speeds as custom', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="pool-proxy-1"] input[type="checkbox"]').setValue(true)
    await wrapper.get('[data-testid="pool-unusable-9"] input[type="checkbox"]').setValue(false)
    await wrapper.get('[data-testid="preset-fast"]').trigger('click')
    await wrapper.get('[data-testid="harvest-save"]').trigger('click')
    await flushPromises()
    expect(updateControls).toHaveBeenCalledWith({ version: 1, preset: 'fast', speed: presets.fast, proxy_ids: [1] })
    expect(showSuccess).toHaveBeenCalled()

    await wrapper.get('[data-testid="speed-cooldown_seconds"]').setValue('31')
    expect(wrapper.get('[data-testid="preset-custom"]').attributes('aria-selected')).toBe('true')
    await wrapper.get('[data-testid="speed-cooldown_seconds"]').setValue('30')
    expect(wrapper.get('[data-testid="preset-fast"]').attributes('aria-selected')).toBe('true')
  })

  it('reports a rejected save without dropping the draft', async () => {
    updateControls.mockRejectedValueOnce(new Error('harvest proxy pool contains unknown proxies'))
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="pool-proxy-2"] input[type="checkbox"]').setValue(true)
    await wrapper.get('[data-testid="harvest-save"]').trigger('click')
    await flushPromises()
    expect(showError).toHaveBeenCalled()
    expect((wrapper.get('[data-testid="pool-proxy-2"] input[type="checkbox"]').element as HTMLInputElement).checked).toBe(true)
  })

  it('starts a bounded manual harvest for one account', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="manual-41"]').trigger('click')
    await wrapper.get('[data-testid="manual-attempts"]').setValue('3')
    await wrapper.get('[data-testid="manual-start"]').trigger('click')
    await flushPromises()
    expect(startManual).toHaveBeenCalledWith(41, { models: ['gpt-6-astra', 'gpt-5.6-sol'], max_attempts: 3, interval_seconds: 5 })
    expect(wrapper.find('[data-testid="manual-dialog"]').exists()).toBe(false)
  })

  it('disables manual harvest while harvesting is off', async () => {
    getSnapshot.mockResolvedValue(snapshot({ enabled: false }))
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.get('[data-testid="manual-41"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-testid="harvest-idle"]').text()).toBe('admin.codexHarvest.idle.disabled')
  })

  it('checks a proxy exit and resets all learning after confirmation', async () => {
    listNodes.mockResolvedValue({
      items: [{ id: 5, proxy_id: 1, proxy_name: 'res-1', account_id: 41, model: 'gpt-6-astra', successes: 2, misses: 1, network_errors: 0, account_errors: 0, consecutive_failures: 0, last_success: null, cooldown_until: null, latency_ms: 300, last_result: 'success', updated_at: '' }],
      total: 1, page: 1, page_size: 20, pages: 1
    })
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="pool-proxy-2"] button').trigger('click')
    await flushPromises()
    expect(testProxy).toHaveBeenCalledWith(2)
    expect(wrapper.get('[data-testid="pool-proxy-2"]').text()).toContain('198.51.100.4 · JP Tokyo')
    expect(wrapper.get('[data-testid="node-5"]').text()).toContain('2 / 1 / 0 / 0')
    await wrapper.get('[data-testid="nodes-reset-all"]').trigger('click')
    await wrapper.get('[data-testid="confirm-yes"]').trigger('click')
    await flushPromises()
    expect(resetNodes).toHaveBeenCalledWith(0)
  })

  it('polls the snapshot while mounted and stops after unmount', async () => {
    vi.useFakeTimers()
    const wrapper = mountView()
    await flushPromises()
    const initial = getSnapshot.mock.calls.length
    await vi.advanceTimersByTimeAsync(5000)
    expect(getSnapshot.mock.calls.length).toBe(initial + 1)
    wrapper.unmount()
    await vi.advanceTimersByTimeAsync(10000)
    expect(getSnapshot.mock.calls.length).toBe(initial + 1)
  })
})
