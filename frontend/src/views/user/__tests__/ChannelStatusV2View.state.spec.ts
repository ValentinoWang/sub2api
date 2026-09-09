import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { appStore, api } = vi.hoisted(() => ({
  appStore: {
    cachedPublicSettings: { channel_monitor_enabled: true },
    showError: vi.fn(),
  },
  api: {
    getDimensions: vi.fn(),
    getSnapshot: vi.fn(),
    getMatrix: vi.fn(),
    getModels: vi.fn(),
    getErrors: vi.fn(),
    getUsers: vi.fn(),
  },
}))

vi.mock('@/stores/app', () => ({ useAppStore: () => appStore }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ isAdmin: false }) }))
vi.mock('@/api/channelMonitorV2', () => api)
vi.mock('@/utils/featureFlags', () => ({
  isChannelMonitorThroughputHidden: () => false,
  isChannelMonitorUserRankingHidden: () => false,
}))
vi.mock('vue-router', () => ({
  useRoute: () => ({ query: {} }),
  useRouter: () => ({ replace: vi.fn() }),
}))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key, te: () => false, locale: { value: 'en' } }),
  }
})

import ChannelStatusV2View from '../ChannelStatusV2View.vue'

const coverage = {
  requested_start: '2026-09-07T00:00:00Z',
  requested_end: '2026-09-07T01:30:00Z',
  coverage_start: '2026-09-07T00:00:00Z',
  data_through: '2026-09-07T01:30:00Z',
  computed_at: '2026-09-07T01:30:00Z',
  aggregation_lag_seconds: 0,
  coverage_complete: true,
  bucket_seconds: 300,
}

function snapshot(requestCount: number) {
  return {
    config: { refresh_interval_seconds: 300 },
    coverage,
    metrics: {
      request_count: requestCount,
      error_rate: 0.02,
      cache_rate: 0.4,
      rpm: 2,
      tpm: 120,
      ttft: { sample_count: requestCount, p50_ms: 120, p95_ms: 200, avg_ms: 140 },
      duration: { sample_count: requestCount, p50_ms: 300, p95_ms: 450, avg_ms: 320 },
    },
    health: { overall: 'healthy', error_rate: 'healthy', ttft: 'healthy', cache: 'healthy', minimum_sample: 1 },
    trend: [],
  }
}

function installSuccessfulResponses(requestCount: number) {
  api.getDimensions.mockResolvedValue({ platforms: [], groups: [], models: [] })
  api.getSnapshot.mockResolvedValue(snapshot(requestCount))
  api.getMatrix.mockResolvedValue({ coverage, group_by: 'platform_group', items: [] })
  api.getModels.mockResolvedValue({ coverage, items: [] })
  api.getErrors.mockResolvedValue({ coverage, items: [] })
  api.getUsers.mockResolvedValue({ coverage, items: [] })
}

function mountView() {
  return mount(ChannelStatusV2View, {
    global: {
      stubs: {
        AppLayout: { template: '<main><slot /></main>' },
        Icon: { template: '<i />' },
        LoadingSpinner: { template: '<i />' },
        Select: { props: ['modelValue', 'options'], template: '<div />' },
        FilterMultiSelect: { template: '<div />' },
        MetricCell: { template: '<div data-testid="metric" />' },
        MonitorRankBadge: { template: '<span />' },
        MonitorTrendChart: { template: '<div />' },
        RelayPulseMatrix: { template: '<div />' },
      },
    },
  })
}

describe('ChannelStatusV2View monitor states', () => {
  beforeEach(() => {
    appStore.cachedPublicSettings = { channel_monitor_enabled: true }
    appStore.showError.mockReset()
    for (const fn of Object.values(api)) fn.mockReset()
  })

  it('shows feature-disabled and avoids all monitor requests when the public flag is false', async () => {
    appStore.cachedPublicSettings = { channel_monitor_enabled: false }
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-testid="channel-monitor-v2-state"]').attributes('data-state')).toBe('feature-disabled')
    expect(api.getSnapshot).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('shows initializing while the first aggregate request is outstanding', () => {
    api.getDimensions.mockReturnValue(new Promise(() => {}))
    api.getSnapshot.mockReturnValue(new Promise(() => {}))
    api.getMatrix.mockReturnValue(new Promise(() => {}))
    const wrapper = mountView()

    expect(wrapper.get('[data-testid="channel-monitor-v2-state"]').attributes('data-state')).toBe('initializing')
    wrapper.unmount()
  })

  it('shows request-failed for a first core request failure', async () => {
    const error = { code: 'NETWORK_UNAVAILABLE', message: 'backend offline' }
    api.getDimensions.mockRejectedValue(error)
    api.getSnapshot.mockRejectedValue(error)
    api.getMatrix.mockRejectedValue(error)
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-testid="channel-monitor-v2-state"]').attributes('data-state')).toBe('request-failed')
    expect(wrapper.text()).toContain('backend offline')
    wrapper.unmount()
  })

  it('shows no-request-data for a successful aggregate with zero request_count', async () => {
    installSuccessfulResponses(0)
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-testid="channel-monitor-v2-state"]').attributes('data-state')).toBe('no-request-data')
    expect(wrapper.find('[data-testid="metric"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('shows metrics only after a successful aggregate contains request data', async () => {
    installSuccessfulResponses(3)
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.find('[data-testid="channel-monitor-v2-state"]').exists()).toBe(false)
    expect(wrapper.findAll('[data-testid="metric"]')).not.toHaveLength(0)
    wrapper.unmount()
  })
})
