import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import SubscriptionsView from '../SubscriptionsView.vue'

const { getMySubscriptions, liandongProducts } = vi.hoisted(() => ({
  getMySubscriptions: vi.fn(),
  liandongProducts: [] as Array<{ cnyAmount: number; usdCredit: number; productUrl: string }>,
}))

vi.mock('@/api/subscriptions', () => ({
  default: { getMySubscriptions },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    cachedPublicSettings: null,
    showError: vi.fn(),
  }),
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => ({
        'userSubscriptions.noActiveSubscriptions': '暂无有效订阅',
        'userSubscriptions.noActiveSubscriptionsDesc': '请联系管理员获取订阅。',
        'userSubscriptions.liandongPurchaseDesc': '卡密将在付款后由链动小铺发放。',
        'userSubscriptions.liandongPurchaseTitle': '选择金额，前往链动小铺购买',
        'userSubscriptions.redeemPurchasedCode': '已有卡密，去兑换',
        'userSubscriptions.failedToLoad': '加载订阅失败',
      }[key] ?? key),
    }),
  }
})

vi.mock('@/components/payment/liandongBalanceProducts', () => ({
  LIANDONG_BALANCE_PRODUCTS: liandongProducts,
}))

function mountView() {
  return mount(SubscriptionsView, {
    global: {
      stubs: {
        AppLayout: { template: '<main><slot /></main>' },
        Icon: { template: '<span data-icon />' },
        RouterLink: {
          props: ['to'],
          template: '<a :href="to"><slot /></a>',
        },
      },
    },
  })
}

describe('SubscriptionsView Liandong purchase entry', () => {
  beforeEach(() => {
    getMySubscriptions.mockReset()
    getMySubscriptions.mockResolvedValue([])
    liandongProducts.splice(0, liandongProducts.length,
      { cnyAmount: 1, usdCredit: 0.14, productUrl: 'https://pay.ldxp.cn/item/one' },
      { cnyAmount: 20, usdCredit: 2.78, productUrl: 'https://pay.ldxp.cn/item/twenty' },
      { cnyAmount: 500, usdCredit: 69.44, productUrl: '' },
    )
  })

  it('shows configured Liandong products and the redemption route in the empty state', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('选择金额，前往链动小铺购买')
    const productLinks = wrapper.findAll('a[target="_blank"]')
    expect(productLinks).toHaveLength(2)
    expect(productLinks[0].attributes('href')).toBe('https://pay.ldxp.cn/item/one')
    expect(productLinks[0].attributes('rel')).toBe('noopener noreferrer')
    expect(productLinks[1].attributes('href')).toBe('https://pay.ldxp.cn/item/twenty')
    expect(wrapper.find('a[href="/redeem"]').exists()).toBe(true)
  })

  it('falls back to the administrator message when no product URL is configured', async () => {
    liandongProducts.splice(0, liandongProducts.length)
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('请联系管理员获取订阅。')
    expect(wrapper.findAll('a[target="_blank"]')).toHaveLength(0)
    expect(wrapper.find('a[href="/redeem"]').exists()).toBe(false)
  })
})
