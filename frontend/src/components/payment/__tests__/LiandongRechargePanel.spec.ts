import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import zh from '@/i18n/locales/zh'
import LiandongRechargePanel from '../LiandongRechargePanel.vue'
import type { LiandongRechargeProduct } from '@/types'

const mocks = vi.hoisted(() => ({
  getProducts: vi.fn(), redeem: vi.fn(), refreshUser: vi.fn(), refreshSubscriptions: vi.fn(), fetchSettings: vi.fn(),
  settings: { purchase_subscription_enabled: true, purchase_subscription_url: 'https://shop.example.com/recharge?goods=123', liandong_recharge_products: [] as LiandongRechargeProduct[] }
}))
vi.mock('@/api/liandongBrowser', () => ({ getLiandongProducts: mocks.getProducts }))
vi.mock('@/api/redeem', () => ({ redeemAPI: { redeem: mocks.redeem } }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ user: { balance: 3 }, refreshUser: mocks.refreshUser }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ cachedPublicSettings: mocks.settings, fetchPublicSettings: mocks.fetchSettings }) }))
vi.mock('@/stores/subscriptions', () => ({ useSubscriptionStore: () => ({ fetchActiveSubscriptions: mocks.refreshSubscriptions }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({
  t: (key: string, params: Record<string, string> = {}) => {
    const value = key.split('.').reduce<unknown>((node, part) => (node as Record<string, unknown>)?.[part], zh)
    return String(value ?? key).replace(/\{(\w+)\}/g, (_, name: string) => params[name] ?? name)
  }
}) }))

function render() {
  return mount(LiandongRechargePanel)
}

describe('Liandong purchase and same-page redemption', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.getProducts.mockRejectedValue({ status: 404 })
    mocks.settings.liandong_recharge_products = []
    mocks.settings.purchase_subscription_enabled = true
    mocks.settings.purchase_subscription_url = 'https://shop.example.com/recharge?goods=123'
    mocks.fetchSettings.mockResolvedValue(mocks.settings)
    mocks.refreshUser.mockResolvedValue(undefined)
    mocks.refreshSubscriptions.mockResolvedValue(undefined)
    mocks.redeem.mockResolvedValue({ type: 'balance', value: 13.25, new_balance: 16.25, message: 'OK' })
  })

  it('opens the configured buyer URL separately and keeps redemption on this page', async () => {
    const wrapper = render()
    await flushPromises()
    const buy = wrapper.get('[data-testid="liandong-buy"]')
    expect(buy.attributes('href')).toBe('https://shop.example.com/recharge?goods=123')
    expect(buy.attributes('target')).toBe('_blank')
    expect(buy.attributes('rel')).toBe('noopener noreferrer')
    expect(wrapper.find('form').exists()).toBe(true)
    expect(wrapper.find('a[href="/redeem"]').exists()).toBe(false)
    expect(mocks.redeem).not.toHaveBeenCalled()
    expect(mocks.fetchSettings).toHaveBeenCalledWith(true)
  })

  it('pairs each configured denomination with its own buyer URL and server credit', async () => {
    mocks.settings.liandong_recharge_products = [
      { goods_id: 101, cny_amount: 5, usd_credit: 5, external_url: 'https://shop.example.com/buy/101' },
      { goods_id: 202, cny_amount: 20, usd_credit: 20, external_url: 'https://shop.example.com/buy/202' },
    ]
    mocks.getProducts.mockResolvedValue(mocks.settings.liandong_recharge_products)
    const wrapper = render()
    await flushPromises()
    expect(wrapper.get('[data-testid="liandong-buy"]').attributes('href')).toBe('https://shop.example.com/buy/101')
    await wrapper.get('[data-testid="liandong-product-202"]').trigger('click')
    expect(wrapper.get('[data-testid="liandong-buy"]').attributes('href')).toBe('https://shop.example.com/buy/202')
    expect(wrapper.get('[data-testid="liandong-product-202"]').attributes('aria-pressed')).toBe('true')
    expect(wrapper.text()).toContain('20元额度')
    expect(wrapper.text()).toContain('支付 ¥20.00 · 到账 $20.00')
    expect(wrapper.find('a[href="https://shop.example.com/recharge?goods=123"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('10元额度')
  })

  it('does not fall back to the old link when the public catalog is empty', async () => {
    mocks.getProducts.mockResolvedValue([])
    const wrapper = render()
    await flushPromises()
    expect(wrapper.find('[data-testid="liandong-products"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="liandong-buy"]').exists()).toBe(false)
    expect(wrapper.find('form').exists()).toBe(true)
  })

  it('keeps redemption available without opening an old buyer link when the catalog fails', async () => {
    mocks.getProducts.mockRejectedValueOnce({ status: 503 })
    const wrapper = render()
    await flushPromises()
    expect(wrapper.find('[data-testid="liandong-buy"]').exists()).toBe(false)
    expect(wrapper.find('form').exists()).toBe(true)
  })

  it('rejects malformed catalog products without blocking redemption', async () => {
    mocks.settings.purchase_subscription_url = ''
    mocks.settings.liandong_recharge_products = [
      { goods_id: 101, cny_amount: 5, usd_credit: 5, external_url: 'javascript:alert(1)' },
      { goods_id: 102, cny_amount: -5, usd_credit: 5, external_url: 'https://shop.example.com/buy/102' },
    ]
    mocks.getProducts.mockResolvedValue(mocks.settings.liandong_recharge_products)
    const wrapper = render()
    await flushPromises()
    expect(wrapper.find('[data-testid="liandong-buy"]').exists()).toBe(false)
    expect(wrapper.find('form').exists()).toBe(true)
  })

  it('redeems the trimmed code and displays the amount returned by the server', async () => {
    const wrapper = render()
    await wrapper.get('input').setValue('  fixture-code  ')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(mocks.redeem).toHaveBeenCalledWith('fixture-code')
    expect(wrapper.text()).toContain('已到账 $13.25')
    expect(wrapper.text()).toContain('$16.25')
    expect((wrapper.get('input').element as HTMLInputElement).value).toBe('')
    expect(mocks.refreshUser).toHaveBeenCalledOnce()
  })

  it('allows only one in-flight redemption even if submit fires twice', async () => {
    let resolve!: (value: unknown) => void
    mocks.redeem.mockReturnValue(new Promise(done => { resolve = done }))
    const wrapper = render()
    await wrapper.get('input').setValue('fixture-code')
    await wrapper.get('form').trigger('submit')
    await wrapper.get('form').trigger('submit')
    expect(mocks.redeem).toHaveBeenCalledOnce()
    resolve({ type: 'balance', value: 1, new_balance: 4 })
    await flushPromises()
  })

  it('retains server-confirmed success when balance refresh fails', async () => {
    mocks.refreshUser.mockRejectedValueOnce(new Error('Network unavailable'))
    const wrapper = render()
    await wrapper.get('input').setValue('fixture-code')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.text()).toContain('已到账 $13.25')
    expect(wrapper.text()).toContain('无需再次兑换')
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
  })

  it('shows a rejected or already-used code without claiming money was credited', async () => {
    mocks.redeem.mockRejectedValueOnce({ message: '该兑换码已使用' })
    const wrapper = render()
    await wrapper.get('input').setValue('used-fixture')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('该兑换码已使用')
    expect(wrapper.text()).not.toContain('已到账')
    expect(mocks.refreshUser).not.toHaveBeenCalled()
  })

  it.each(['', 'javascript:alert(1)', 'https://user:secret@example.com/store'])('retains redemption without an unsafe or missing buyer link: %s', (url) => {
    mocks.settings.purchase_subscription_url = url
    const wrapper = render()
    expect(wrapper.find('[data-testid="liandong-buy"]').exists()).toBe(false)
    expect(wrapper.find('form').exists()).toBe(true)
  })

  it('honors the store entry switch', () => {
    mocks.settings.purchase_subscription_enabled = false
    expect(render().find('[data-testid="liandong-buy"]').exists()).toBe(false)
  })
})
