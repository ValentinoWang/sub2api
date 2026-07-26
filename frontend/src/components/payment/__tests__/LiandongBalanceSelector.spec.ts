import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import LiandongBalanceSelector from '../LiandongBalanceSelector.vue'
import type { LiandongBalanceProduct } from '../liandongBalanceProducts'

const products: readonly LiandongBalanceProduct[] = [
  { cnyAmount: 1, usdCredit: 0.14, productUrl: 'https://www.ldxp.cn/goods/101' },
  { cnyAmount: 5, usdCredit: 0.69, productUrl: 'https://www.ldxp.cn/goods/105' },
  { cnyAmount: 20, usdCredit: 2.78, productUrl: 'https://www.ldxp.cn/goods/120' },
  { cnyAmount: 50, usdCredit: 6.94, productUrl: 'https://www.ldxp.cn/goods/150' },
  { cnyAmount: 100, usdCredit: 13.89, productUrl: 'https://www.ldxp.cn/goods/1100' },
  { cnyAmount: 500, usdCredit: 69.44, productUrl: '' },
]

describe('LiandongBalanceSelector', () => {
  it('shows every RMB tier and its fixed USD credit', () => {
    const wrapper = mount(LiandongBalanceSelector, {
      props: { products },
      global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } },
    })

    expect(wrapper.text()).toContain('¥1')
    expect(wrapper.text()).toContain('到账 $0.14')
    expect(wrapper.text()).toContain('¥500')
    expect(wrapper.text()).toContain('到账 $69.44')
    expect(wrapper.text()).not.toContain('1 USD = ¥7.2')
    const rateSourceLink = wrapper.find('a[href="https://open.er-api.com/v6/latest/USD"]')
    expect(rateSourceLink.text()).toBe('open.er-api 实时接口')
    expect(rateSourceLink.attributes('target')).toBe('_blank')
    expect(rateSourceLink.attributes('rel')).toBe('noopener noreferrer')
    expect(wrapper.findAll('a')).toHaveLength(8)
  })

  it('uses the exact configured product URL and disables missing mappings', () => {
    const wrapper = mount(LiandongBalanceSelector, {
      props: { products },
      global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } },
    })
    const productLinks = wrapper.findAll('a').slice(2)

    expect(productLinks[0].attributes('href')).toBe('https://www.ldxp.cn/goods/101')
    expect(productLinks[0].attributes('target')).toBe('_blank')
    expect(productLinks[5].attributes('href')).toBeUndefined()
    expect(productLinks[5].attributes('aria-disabled')).toBe('true')
  })
})
