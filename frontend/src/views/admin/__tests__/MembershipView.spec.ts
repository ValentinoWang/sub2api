import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const { getAdminOverview, checkAvailability, importCDKs, createValidationOrder, verifyProduct, saveCoupon, reviewOrder, updateProduct, showError, showSuccess } = vi.hoisted(() => ({
  getAdminOverview: vi.fn(), checkAvailability: vi.fn(), importCDKs: vi.fn(), createValidationOrder: vi.fn(), verifyProduct: vi.fn(), saveCoupon: vi.fn(), reviewOrder: vi.fn(), updateProduct: vi.fn(), showError: vi.fn(), showSuccess: vi.fn(),
}))
const run = vi.hoisted(() => vi.fn((fn: () => unknown) => fn()))
vi.mock('@/api/membership', () => ({ membershipAPI: { getAdminOverview, checkAvailability, importCDKs, createValidationOrder, verifyProduct, saveCoupon, reviewOrder, updateProduct } }))
vi.mock('@/stores', () => ({ useAppStore: () => ({ showError, showSuccess }) }))
vi.mock('@/composables/useStepUp', () => ({ useStepUp: () => ({ visible: { value: false }, run }), isStepUpCancelled: () => false, isStepUpBlocked: () => false, stepUpBlockReason: () => '' }))
vi.mock('vue-i18n', async (importOriginal) => ({ ...(await importOriginal<typeof import('vue-i18n')>()), useI18n: () => ({ t: (key: string) => key }) }))

import MembershipView from '../MembershipView.vue'

const overview = { runtime_ready: false, payments_enabled: false, consent_version: 'membership-2026-09-07-v1', products: [{ sku: 'chatgpt_plus', name: 'Codex/GPT Plus', price_minor: 10000, currency: 'CNY', period_days: 30, credential_mode: 'account_id', for_sale: false, available: 0, eta_minutes: 30, channel: 'gpt', paused: true, verified_at: null, inventory_checked_at: null, poll_seconds: 5, wait_seconds: 600 }], orders: [{ id: 'order-1', sku: 'chatgpt_plus', name: 'Codex/GPT Plus', kind: 'customer', payment_state: 'paid', fulfillment_state: 'review_required', price_minor: 10000, discount_minor: 0, period_days: 30, target_masked: 'acct...1234', credential_mode: 'account_id', input_required: false, error_code: 'REVIEW_REQUIRED', refund_requested_at: null, created_at: '2026-09-07T00:00:00Z', updated_at: '2026-09-07T00:00:00Z', events: [] }], stats: { total: 1, success: 0, review: 1, queued: 0, mean_seconds: 0 }, ledger: { payment: 0, fee: 0, supplier_cost: 0, refund: 0, profit: 0 } }

describe('admin MembershipView', () => {
  beforeEach(() => {
    getAdminOverview.mockReset().mockResolvedValue(overview)
    checkAvailability.mockReset().mockResolvedValue(undefined)
    importCDKs.mockReset().mockResolvedValue({ imported: 2 })
    createValidationOrder.mockReset().mockResolvedValue({ id: 'validation-1' })
    verifyProduct.mockReset().mockResolvedValue(undefined)
    saveCoupon.mockReset().mockResolvedValue(undefined)
    reviewOrder.mockReset().mockResolvedValue(undefined)
    updateProduct.mockReset().mockResolvedValue(undefined)
    run.mockClear(); showError.mockReset(); showSuccess.mockReset()
  })

  it('keeps product verification and availability visibly gated when runtime is unavailable', async () => {
    const wrapper = mount(MembershipView, { global: { stubs: { AppLayout: { template: '<div><slot /></div>' }, Icon: true, BaseDialog: { template: '<div><slot /></div>' }, TotpStepUpDialog: true } } })
    await flushPromises()
    expect(wrapper.get('[data-test="membership-product-table"]').text()).toContain('Codex/GPT Plus')
    expect(wrapper.text()).toContain('membership.unavailable')
    await wrapper.get('[data-test="availability-chatgpt_plus"]').trigger('click')
    await flushPromises()
    expect(run).toHaveBeenCalled()
    expect(checkAvailability).toHaveBeenCalledWith('chatgpt_plus')
  })

  it('imports CDKs only through the protected action and clears the in-memory paste field', async () => {
    const wrapper = mount(MembershipView, { global: { stubs: { AppLayout: { template: '<div><slot /></div>' }, Icon: true, BaseDialog: true, TotpStepUpDialog: true } } })
    await flushPromises()
    await wrapper.get('[data-test="stock-sku"]').setValue('chatgpt_plus')
    await wrapper.get('[data-test="stock-codes"]').setValue('code-one\ncode-two')
    await wrapper.get('[data-test="stock-cost"]').setValue(88)
    await wrapper.get('[data-test="import-cdks"]').trigger('submit')
    await flushPromises()
    expect(importCDKs).toHaveBeenCalledWith('chatgpt_plus', ['code-one', 'code-two'], 88)
    expect((wrapper.get('[data-test="stock-codes"]').element as HTMLTextAreaElement).value).toBe('')
    expect(run).toHaveBeenCalled()
  })
})
