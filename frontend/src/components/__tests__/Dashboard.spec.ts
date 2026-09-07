/**
 * Dashboard 数据加载逻辑测试
 * 通过封装组件测试仪表板核心数据加载流程
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, shallowMount, flushPromises } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import { defineComponent, ref, onMounted, nextTick } from 'vue'
import UserDashboardCharts from '@/components/user/dashboard/UserDashboardCharts.vue'
import UserDashboardRecentUsage from '@/components/user/dashboard/UserDashboardRecentUsage.vue'
import UserDashboardQuickActions from '@/components/user/dashboard/UserDashboardQuickActions.vue'
import UserDashboardStats from '@/components/user/dashboard/UserDashboardStats.vue'
import type { UserDashboardStats as UserStats } from '@/api/usage'
import type { UsageLog } from '@/types'

const dashboardMocks = vi.hoisted(() => ({ push: vi.fn(), refreshAccess: vi.fn(), canUseBatchImage: false }))

vi.mock('vue-router', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-router')>(),
  useRouter: () => ({ push: dashboardMocks.push }),
}))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key }),
}))
vi.mock('@/composables/useBatchImageAccess', () => ({
  useBatchImageAccess: () => ({
    canUseBatchImage: dashboardMocks.canUseBatchImage,
    refreshBatchImageAccess: dashboardMocks.refreshAccess,
  }),
}))

// Mock API
const mockGetDashboardStats = vi.fn()

vi.mock('@/api', () => ({
  authAPI: {
    getCurrentUser: vi.fn().mockResolvedValue({
      data: { id: 1, username: 'test', email: 'test@example.com', role: 'user', balance: 100, concurrency: 5, status: 'active', allowed_groups: null, created_at: '', updated_at: '' },
    }),
    logout: vi.fn(),
    refreshToken: vi.fn(),
  },
  isTotp2FARequired: () => false,
}))

vi.mock('@/api/usage', () => ({
  usageAPI: {
    getDashboardStats: (...args: any[]) => mockGetDashboardStats(...args),
  },
}))

vi.mock('@/api/admin/system', () => ({
  checkUpdates: vi.fn(),
}))

vi.mock('@/api/auth', () => ({
  getPublicSettings: vi.fn().mockResolvedValue({}),
}))

interface DashboardStats {
  balance: number
  api_key_count: number
  active_api_key_count: number
  today_requests: number
  today_cost: number
  today_tokens: number
  total_tokens: number
}

/**
 * 简化的 Dashboard 测试组件
 */
const DashboardTestComponent = defineComponent({
  setup() {
    const stats = ref<DashboardStats | null>(null)
    const loading = ref(false)
    const error = ref('')

    const loadStats = async () => {
      loading.value = true
      error.value = ''
      try {
        stats.value = await mockGetDashboardStats()
      } catch (e: any) {
        error.value = e.message || '加载失败'
      } finally {
        loading.value = false
      }
    }

    onMounted(loadStats)

    return { stats, loading, error, loadStats }
  },
  template: `
    <div>
      <div v-if="loading" class="loading">加载中...</div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="stats" class="stats">
        <span class="balance">{{ stats.balance }}</span>
        <span class="api-keys">{{ stats.api_key_count }}</span>
        <span class="today-requests">{{ stats.today_requests }}</span>
        <span class="today-cost">{{ stats.today_cost }}</span>
      </div>
      <button class="refresh" @click="loadStats">刷新</button>
    </div>
  `,
})

describe('Dashboard 数据加载', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  const fakeStats: DashboardStats = {
    balance: 100.5,
    api_key_count: 3,
    active_api_key_count: 2,
    today_requests: 150,
    today_cost: 2.5,
    today_tokens: 50000,
    total_tokens: 1000000,
  }

  it('挂载后自动加载数据', async () => {
    mockGetDashboardStats.mockResolvedValue(fakeStats)

    const wrapper = mount(DashboardTestComponent)
    await flushPromises()

    expect(mockGetDashboardStats).toHaveBeenCalledTimes(1)
    expect(wrapper.find('.balance').text()).toBe('100.5')
    expect(wrapper.find('.api-keys').text()).toBe('3')
    expect(wrapper.find('.today-requests').text()).toBe('150')
    expect(wrapper.find('.today-cost').text()).toBe('2.5')
  })

  it('加载中显示 loading 状态', async () => {
    let resolveStats: (v: any) => void
    mockGetDashboardStats.mockImplementation(
      () => new Promise((resolve) => { resolveStats = resolve })
    )

    const wrapper = mount(DashboardTestComponent)
    await nextTick()

    expect(wrapper.find('.loading').exists()).toBe(true)

    resolveStats!(fakeStats)
    await flushPromises()

    expect(wrapper.find('.loading').exists()).toBe(false)
    expect(wrapper.find('.stats').exists()).toBe(true)
  })

  it('加载失败时显示错误信息', async () => {
    mockGetDashboardStats.mockRejectedValue(new Error('Network error'))

    const wrapper = mount(DashboardTestComponent)
    await flushPromises()

    expect(wrapper.find('.error').text()).toBe('Network error')
    expect(wrapper.find('.stats').exists()).toBe(false)
  })

  it('点击刷新按钮重新加载数据', async () => {
    mockGetDashboardStats.mockResolvedValue(fakeStats)

    const wrapper = mount(DashboardTestComponent)
    await flushPromises()

    expect(mockGetDashboardStats).toHaveBeenCalledTimes(1)

    // 更新数据
    const updatedStats = { ...fakeStats, today_requests: 200 }
    mockGetDashboardStats.mockResolvedValue(updatedStats)

    await wrapper.find('.refresh').trigger('click')
    await flushPromises()

    expect(mockGetDashboardStats).toHaveBeenCalledTimes(2)
    expect(wrapper.find('.today-requests').text()).toBe('200')
  })

  it('数据为空时不显示统计信息', async () => {
    mockGetDashboardStats.mockResolvedValue(null)

    const wrapper = mount(DashboardTestComponent)
    await flushPromises()

    expect(wrapper.find('.stats').exists()).toBe(false)
  })
})

describe('Dashboard actual component interactions', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    dashboardMocks.canUseBatchImage = false
  })

  it('keeps refresh named and prevents refresh while charts are loading', async () => {
    const wrapper = shallowMount(UserDashboardCharts, {
      props: { loading: false, startDate: '2026-09-01', endDate: '2026-09-07', granularity: 'day', trend: [], models: [] },
    })
    const refresh = wrapper.get('button[aria-label="common.refresh"]')
    await refresh.trigger('click')
    expect(wrapper.emitted('refresh')).toHaveLength(1)
    await wrapper.setProps({ loading: true })
    expect(refresh.attributes('disabled')).toBeDefined()
    await refresh.trigger('click')
    expect(wrapper.emitted('refresh')).toHaveLength(1)
  })

  it('continues forwarding date and granularity changes from the controls', () => {
    const wrapper = shallowMount(UserDashboardCharts, {
      props: { loading: false, startDate: '2026-09-01', endDate: '2026-09-07', granularity: 'day', trend: [], models: [] },
    })
    wrapper.findComponent({ name: 'DateRangePicker' }).vm.$emit('update:startDate', '2026-09-03')
    wrapper.findComponent({ name: 'DateRangePicker' }).vm.$emit('change', { startDate: '2026-09-03', endDate: '2026-09-07' })
    wrapper.findComponent({ name: 'Select' }).vm.$emit('update:model-value', 'hour')
    wrapper.findComponent({ name: 'Select' }).vm.$emit('change')
    expect(wrapper.emitted('update:startDate')).toEqual([['2026-09-03']])
    expect(wrapper.emitted('dateRangeChange')).toHaveLength(1)
    expect(wrapper.emitted('update:granularity')).toEqual([['hour']])
    expect(wrapper.emitted('granularityChange')).toHaveLength(1)
  })

  it('preserves quick action destinations and restricted image access', async () => {
    const wrapper = shallowMount(UserDashboardQuickActions)
    expect(dashboardMocks.refreshAccess).toHaveBeenCalledOnce()
    const buttons = wrapper.findAll('button')
    expect(buttons).toHaveLength(3)
    for (const button of buttons) await button.trigger('click')
    expect(dashboardMocks.push.mock.calls).toEqual([['/keys'], ['/usage'], ['/redeem']])
    wrapper.unmount()
    dashboardMocks.canUseBatchImage = true
    const enabled = shallowMount(UserDashboardQuickActions)
    const imageAction = enabled.findAll('button').find((button) => button.text().includes('dashboard.batchImageAgent'))
    expect(imageAction).toBeDefined()
    await imageAction!.trigger('click')
    expect(dashboardMocks.push).toHaveBeenLastCalledWith('/batch-image')
  })

  it('preserves complete model names, actual and standard prices in usage rows', () => {
    const model = 'provider-a/very-long-model-identifier-for-compact-mobile-layout'
    const wrapper = shallowMount(UserDashboardRecentUsage, {
      props: {
        loading: false,
        data: [{ id: 1, model, actual_cost: 0.125, total_cost: 0.25, input_tokens: 100, output_tokens: 200, created_at: '2026-09-07T08:00:00Z' } as UsageLog],
      },
      global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } },
    })
    expect(wrapper.text()).toContain(model)
    expect(wrapper.text()).toContain('$0.1250')
    expect(wrapper.text()).toContain('$0.2500')
    expect(wrapper.text()).toContain('300 tokens')
    expect(wrapper.findComponent({ name: 'EmptyState' }).exists()).toBe(false)
  })

  it('keeps usage loading and empty states distinct', async () => {
    const wrapper = shallowMount(UserDashboardRecentUsage, {
      props: { data: [], loading: true },
      global: { stubs: { RouterLink: true } },
    })
    expect(wrapper.findComponent({ name: 'LoadingSpinner' }).exists()).toBe(true)
    expect(wrapper.findComponent({ name: 'EmptyState' }).exists()).toBe(false)
    await wrapper.setProps({ loading: false })
    expect(wrapper.findComponent({ name: 'LoadingSpinner' }).exists()).toBe(false)
    expect(wrapper.findComponent({ name: 'EmptyState' }).props('title')).toBe('dashboard.noUsageRecords')
  })

  it('keeps balances and platform quotas hidden in simple mode', async () => {
    const wrapper = shallowMount(UserDashboardStats, {
      props: { stats: { total_api_keys: 2, today_requests: 15 } as UserStats, balance: 1234.5, isSimple: false },
    })
    expect(wrapper.text()).toContain('$1,234.50')
    expect(wrapper.text()).toContain('dashboard.apiKeys')
    await wrapper.setProps({ isSimple: true })
    expect(wrapper.text()).not.toContain('dashboard.balance')
    expect(wrapper.text()).not.toContain('dashboard.platformBreakdown')
    expect(wrapper.text()).toContain('dashboard.todayRequests')
  })
})
