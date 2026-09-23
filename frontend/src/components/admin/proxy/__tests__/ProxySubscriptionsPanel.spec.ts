import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import ProxySubscriptionsPanel from '../ProxySubscriptionsPanel.vue'
import type { ProxySubscription } from '@/api/admin/proxies'

const { listSubscriptions, refreshSubscription, updateSubscription, showError, showSuccess } = vi.hoisted(() => ({
  listSubscriptions: vi.fn(),
  refreshSubscription: vi.fn(),
  updateSubscription: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: { proxies: { listSubscriptions, refreshSubscription, updateSubscription } }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError, showSuccess })
}))

vi.mock('vue-i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-i18n')>()),
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) => (params ? `${key}:${JSON.stringify(params)}` : key)
  })
}))

function subscription(overrides: Partial<ProxySubscription> = {}): ProxySubscription {
  return {
    id: 'sub1',
    name: '苏菲',
    format: 'uri-list',
    updated_at: '2026-09-02T18:38:15Z',
    node_count: 2,
    has_url: true,
    refresh_interval_minutes: 60,
    last_refresh_at: '2026-09-23T02:00:00Z',
    last_refresh_status: 'ok',
    next_refresh_at: '2026-09-23T03:00:00Z',
    usage: { upload: 1024, download: 1024, total: 1073741824, expire_at: '2026-10-14T00:00:00Z' },
    info: ['剩余流量：138.17 GB'],
    groups: [
      { name: '🎫 打票出口', source: 'derived', kind: 'purpose', proxy_ids: [8] },
      { name: '💼 业务出口', source: 'derived', kind: 'purpose', proxy_ids: [65] },
      { name: '🇭🇰 香港 · 住宅', source: 'derived', kind: 'region', proxy_ids: [8] }
    ],
    nodes: [
      { proxy_id: 4, name: '套餐到期：2026-09-14', info: true, meta: { residential: false, multiplier: '1×', route: '直连', protocol: 'vless', display_name: '🌐未知-套餐到期-机房-Vless' } },
      { proxy_id: 8, name: '优秀|【3x】中转|香港家宽🇭🇰', info: false, meta: { region: 'HK', country: '香港', flag: '🇭🇰', residential: true, multiplier: '3×', route: '中转', protocol: 'vless', display_name: '🇭🇰香港-中转-香港家宽-住宅IP' } },
      { proxy_id: 65, name: '【2】新加坡高速节点🇸🇬hy2', info: false, meta: { region: 'SG', country: '新加坡', flag: '🇸🇬', residential: false, multiplier: '1×', route: '直连', protocol: 'hysteria2', display_name: '🇸🇬新加坡-【2】新加坡高速节点-机房-Hysteria2' } }
    ],
    ...overrides
  }
}

describe('订阅与分组面板', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    listSubscriptions.mockResolvedValue([subscription()])
    refreshSubscription.mockResolvedValue({ node_count: 2, created: 0, reused: 2, deactivated: 1 })
    updateSubscription.mockResolvedValue(undefined)
  })

  it('shows usage, info entries and Codex_degrade-style groups with members', async () => {
    const wrapper = mount(ProxySubscriptionsPanel)
    await flushPromises()
    const card = wrapper.get('[data-testid="subscription-sub1"]')
    expect(card.text()).toContain('admin.proxies.subscriptions.formats.uri-list')
    expect(card.text()).toContain('剩余流量：138.17 GB · 套餐到期：2026-09-14')
    expect(card.text()).toContain('🎫 打票出口 · 1')
    expect(wrapper.find('[data-testid="group-members-sub1-🎫 打票出口"]').exists()).toBe(false)
    await wrapper.get('[data-testid="group-sub1-🎫 打票出口"]').trigger('click')
    const members = wrapper.get('[data-testid="group-members-sub1-🎫 打票出口"]')
    expect(members.text()).toContain('🇭🇰香港-中转-香港家宽-住宅IP')
    expect(members.text()).toContain('3× · 中转 · #8')
    expect(members.text()).not.toContain('套餐到期')
  })

  it('refreshes and changes the interval, then reloads', async () => {
    const wrapper = mount(ProxySubscriptionsPanel)
    await flushPromises()
    await wrapper.get('[data-testid="subscription-refresh-sub1"]').trigger('click')
    await flushPromises()
    expect(refreshSubscription).toHaveBeenCalledWith('sub1')
    expect(showSuccess).toHaveBeenCalled()
    expect(wrapper.emitted('refreshed')).toHaveLength(1)
    await wrapper.get('[data-testid="subscription-interval-sub1"]').setValue('360')
    await flushPromises()
    expect(updateSubscription).toHaveBeenCalledWith('sub1', 360)
    expect(listSubscriptions).toHaveBeenCalledTimes(3)
  })

  it('explains why refresh is unavailable without a stored URL', async () => {
    listSubscriptions.mockResolvedValue([subscription({ has_url: false, refresh_interval_minutes: 0, next_refresh_at: undefined })])
    const wrapper = mount(ProxySubscriptionsPanel)
    await flushPromises()
    expect(wrapper.find('[data-testid="subscription-no-url"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="subscription-refresh-sub1"]').attributes('disabled')).toBeDefined()
  })

  it('reports a failed refresh', async () => {
    refreshSubscription.mockRejectedValueOnce(new Error('subscription server is unreachable'))
    const wrapper = mount(ProxySubscriptionsPanel)
    await flushPromises()
    await wrapper.get('[data-testid="subscription-refresh-sub1"]').trigger('click')
    await flushPromises()
    expect(showError).toHaveBeenCalled()
    expect(wrapper.emitted('refreshed')).toBeUndefined()
  })
})
