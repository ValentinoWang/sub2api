import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { describe, expect, it } from 'vitest'
import AmountInput from '../AmountInput.vue'

const i18n = createI18n({
  legacy: false,
  locale: 'zh',
  messages: {
    zh: {
      payment: {
        customAmount: '自定义金额',
        enterAmount: '请输入金额',
        quickAmounts: '快捷金额',
      },
    },
  },
})

describe('AmountInput', () => {
  it('uses the selected payment currency symbol', () => {
    const wrapper = mount(AmountInput, {
      props: {
        amounts: [10, 20],
        currency: 'CNY',
        modelValue: null,
      },
      global: { plugins: [i18n] },
    })

    expect(wrapper.text()).toContain('¥10')
    expect(wrapper.text()).toContain('¥20')
    expect(wrapper.text()).not.toContain('$')
  })
})
