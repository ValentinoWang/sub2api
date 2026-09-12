import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import zh from '@/i18n/locales/zh'
import LiandongRechargePanel from '../LiandongRechargePanel.vue'

const mocks = vi.hoisted(() => ({
  redeem: vi.fn(), refreshUser: vi.fn(), refreshSubscriptions: vi.fn(), fetchSettings: vi.fn(),
  settings: { purchase_subscription_enabled: true, purchase_subscription_url: 'https://shop.example.com/recharge?goods=123' }
}))
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
