import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import DashboardView from '../DashboardView.vue'
import UserDashboardStats from '@/components/user/dashboard/UserDashboardStats.vue'
import UserDashboardCharts from '@/components/user/dashboard/UserDashboardCharts.vue'
import UserDashboardRecentUsage from '@/components/user/dashboard/UserDashboardRecentUsage.vue'

const api = vi.hoisted(() => ({
  getDashboardStats: vi.fn(),
  getDashboardTrend: vi.fn(),
  getDashboardModels: vi.fn(),
  getByDateRange: vi.fn(),
  refreshUser: vi.fn(),
  getMyPlatformQuotas: vi.fn()
}))
vi.mock('@/api/usage', () => ({ usageAPI: api }))
vi.mock('@/api/user', () => ({ getMyPlatformQuotas: api.getMyPlatformQuotas }))
vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ user: { balance: 20 }, isSimpleMode: false, refreshUser: api.refreshUser })
}))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key })
}))

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason: Error) => void
  const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej })
  return { promise, resolve, reject }
}

const mountDashboard = () => mount(DashboardView, {
  global: {
    stubs: {
      AppLayout: { template: '<main><slot /></main>' },
      UserDashboardCharts: true,
      UserDashboardRecentUsage: true,
      UserDashboardQuickActions: true,
      ExperienceCollection: true,
      Icon: true
    }
  }
})

beforeEach(() => {
  vi.resetAllMocks()
  vi.spyOn(console, 'error').mockImplementation(() => {})
  api.refreshUser.mockResolvedValue(undefined)
  api.getDashboardStats.mockResolvedValue({ today_requests: 42 })
  api.getDashboardTrend.mockResolvedValue({ trend: [] })
  api.getDashboardModels.mockResolvedValue({ models: [] })
  api.getByDateRange.mockResolvedValue({ items: [{ id: 1 }] })
  api.getMyPlatformQuotas.mockResolvedValue({ platform_quotas: [] })
})
afterEach(() => vi.restoreAllMocks())

describe('user dashboard loading layout', () => {
  it('mounts the dashboard during a slow successful initial load and replaces only placeholders', async () => {
    const request = deferred<{ today_requests: number }>()
    api.getDashboardStats.mockReturnValueOnce(request.promise)
    const wrapper = mountDashboard()
    const charts = wrapper.findComponent(UserDashboardCharts).vm
    const cards = wrapper.findAll('.dashboard-metric').map(card => card.element)
    expect(cards).toHaveLength(8)
    expect(wrapper.findAll('.dashboard-metrics-pending')).toHaveLength(2)
    expect(wrapper.find('.dashboard-metrics').attributes('aria-hidden')).toBe('true')
    expect(wrapper.find('.dashboard-activity').exists()).toBe(true)
    expect(wrapper.find('[data-testid="dashboard-experience-sharing"]').exists()).toBe(true)

    request.resolve({ today_requests: 42 })
    await flushPromises()
    expect(wrapper.findComponent(UserDashboardCharts).vm).toBe(charts)
    expect(wrapper.findAll('.dashboard-metric').map(card => card.element)).toEqual(cards)
    expect(wrapper.findAll('.dashboard-metrics-pending')).toHaveLength(0)
    expect(wrapper.find('.dashboard-metrics').attributes('aria-hidden')).toBe('false')
    expect(wrapper.text()).toContain('42')
    wrapper.unmount()
  })

  it('retains stats, mounted charts and recent rows during refresh and after refresh failure', async () => {
    const wrapper = mountDashboard()
    await flushPromises()
    const charts = wrapper.findComponent(UserDashboardCharts)
    const instance = charts.vm
    const request = deferred<never>()
    const recent = deferred<{ items: { id: number }[] }>()
    api.getDashboardStats.mockReturnValueOnce(request.promise)
    api.getByDateRange.mockReturnValueOnce(recent.promise)
    charts.vm.$emit('refresh')
    await flushPromises()
    expect(wrapper.findComponent(UserDashboardStats).props('stats')).toEqual({ today_requests: 42 })
    expect(wrapper.findComponent(UserDashboardCharts).vm).toBe(instance)
    expect(wrapper.findComponent(UserDashboardRecentUsage).props()).toMatchObject({ loading: false, data: [{ id: 1 }] })
    request.reject(new Error('unavailable'))
    recent.resolve({ items: [{ id: 2 }] })
    await flushPromises()
    expect(wrapper.find('[role="alert"]').text()).toContain('dashboard.refreshFailed')
    expect(wrapper.text()).toContain('42')
    expect(wrapper.findComponent(UserDashboardCharts).vm).toBe(instance)
    wrapper.unmount()
  })

  it('keeps the dashboard after initial failure and recovers using retry', async () => {
    api.getDashboardStats.mockRejectedValueOnce(new Error('unavailable'))
    const wrapper = mountDashboard()
    await flushPromises()
    expect(wrapper.find('[role="alert"]').text()).toContain('dashboard.loadFailed')
    expect(wrapper.findAll('.dashboard-metric')).toHaveLength(8)
    expect(wrapper.findComponent(UserDashboardCharts).exists()).toBe(true)
    const request = deferred<{ today_requests: number }>()
    api.getDashboardStats.mockReturnValueOnce(request.promise)
    await wrapper.find('[role="alert"] button').trigger('click')
    expect(wrapper.find('[role="alert"] button').attributes('disabled')).toBeDefined()
    request.resolve({ today_requests: 99 })
    await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('99')
    expect(wrapper.findAll('.dashboard-metrics-pending')).toHaveLength(0)
    wrapper.unmount()
  })
})
