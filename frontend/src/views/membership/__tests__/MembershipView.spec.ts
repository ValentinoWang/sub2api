import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const { listProducts, listOrders, getOrder, createOrder, submitCredential, createPayment, createPaymentRedirectTicket, requestRefund, showError, showSuccess } = vi.hoisted(() => ({
  listProducts: vi.fn(), listOrders: vi.fn(), getOrder: vi.fn(), createOrder: vi.fn(), submitCredential: vi.fn(), createPayment: vi.fn(), createPaymentRedirectTicket: vi.fn(), requestRefund: vi.fn(), showError: vi.fn(), showSuccess: vi.fn(),
}))

vi.mock('@/api/membership', () => ({
  MEMBERSHIP_CONSENT_VERSION: 'membership-2026-09-07-v1',
  membershipAPI: { listProducts, listOrders, getOrder, createOrder, submitCredential, createPayment, createPaymentRedirectTicket, requestRefund },
}))
vi.mock('@/stores', () => ({ useAppStore: () => ({ showError, showSuccess }) }))
vi.mock('vue-i18n', async (importOriginal) => ({ ...(await importOriginal<typeof import('vue-i18n')>()), useI18n: () => ({ t: (key: string) => key }) }))

import MembershipView from '../MembershipView.vue'

const product = { sku: 'chatgpt_pro_20x', name: 'Codex/GPT Pro20x', price_minor: 20000, currency: 'CNY', period_days: 30, credential_mode: 'session', for_sale: false, available: 0, eta_minutes: 240 }
const order = { id: 'order-1', sku: 'chatgpt_pro_20x', name: 'Codex/GPT Pro20x', kind: 'customer', payment_state: 'created', fulfillment_state: 'awaiting_input', price_minor: 20000, discount_minor: 0, period_days: 30, target_masked: '', credential_mode: 'session', input_required: true, error_code: '', refund_requested_at: null, created_at: '2026-09-07T00:00:00Z', updated_at: '2026-09-07T00:00:00Z', events: [] }

function mountView() {
  return mount(MembershipView, { global: { stubs: { AppLayout: { template: '<div><slot /></div>' }, Icon: true } } })
}

describe('MembershipView', () => {
  beforeEach(() => {
    localStorage.clear()
    listProducts.mockReset().mockResolvedValue([product])
    listOrders.mockReset().mockResolvedValue([order])
    getOrder.mockReset().mockResolvedValue(order)
    createOrder.mockReset().mockResolvedValue({ id: 'order-1' })
    submitCredential.mockReset().mockResolvedValue({ accepted: true, expires_in_seconds: 1800 })
    createPayment.mockReset().mockResolvedValue({ redirect_url: '/api/v1/membership/payment/redirect?ticket=abcdefghijklmnopqrstuvwxyzABCDEFGHIJK123456', order_id: 1 })
    createPaymentRedirectTicket.mockReset().mockResolvedValue({ redirect_url: '/api/v1/membership/payment/redirect?ticket=abcdefghijklmnopqrstuvwxyzABCDEFGHIJK123456', order_id: 1 })
    requestRefund.mockReset().mockResolvedValue(undefined)
    showError.mockReset(); showSuccess.mockReset()
  })

  it('makes initially unavailable products explicit and prevents checkout', async () => {
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.get('[data-test="product-chatgpt_pro_20x"]').text()).toContain('membership.unavailable')
    expect(wrapper.get('[data-test="create-chatgpt_pro_20x"]').attributes('disabled')).toBeDefined()
  })

  it('maps a raw upstream Axios error to a local safe message without rendering its details', async () => {
    const upstreamMessage = 'https://supplier.internal.example rejected CDK and task_id=private-task-9'
    listProducts.mockRejectedValueOnce({ code: 'UPSTREAM_STACKTRACE', message: upstreamMessage, detail: upstreamMessage })
    const wrapper = mountView()
    await flushPromises()

    expect(showError).toHaveBeenCalledWith('membership.errors.unavailable')
    expect(showError).not.toHaveBeenCalledWith(upstreamMessage)
    expect(wrapper.text()).not.toContain('supplier.internal.example')
    expect(wrapper.text()).not.toContain('private-task-9')
  })

  it('keeps a submitted session only in component state and clears inputs after successful submission', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-test="detail-order-1"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="session-input"]').setValue('{"accessToken":"session-secret"}')
    await wrapper.get('[data-test="session-account-input"]').setValue('account-1234')
    await wrapper.get('[data-test="credential-consent"]').setValue(true)
    await wrapper.get('[data-test="submit-credential"]').trigger('submit')
    await flushPromises()

    expect(submitCredential).toHaveBeenCalledWith('order-1', expect.objectContaining({ value: '{"accessToken":"session-secret"}', account_id: 'account-1234' }))
    expect((wrapper.get('[data-test="session-input"]').element as HTMLTextAreaElement).value).toBe('')
    expect(localStorage.getItem('session-secret')).toBeNull()
    expect(JSON.stringify(localStorage)).not.toContain('session-secret')
  })

  it('shows only the same-site payment redirect without rendering the provider URL', async () => {
    const payableOrder = { ...order, input_required: false }
    const providerURL = 'https://pay.example.test/checkout?provider_trace=private-trace-id'
    const redirectURL = '/api/v1/membership/payment/redirect?ticket=abcdefghijklmnopqrstuvwxyzABCDEFGHIJK123456'
    listOrders.mockResolvedValueOnce([payableOrder])
    getOrder.mockResolvedValueOnce(payableOrder)
    createPayment.mockResolvedValueOnce({ redirect_url: redirectURL, order_id: 1, pay_url: providerURL, qr_code: providerURL })
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-test="detail-order-1"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="membership-payment"]').trigger('click')
    await flushPromises()

    const action = wrapper.get('[data-test="continue-membership-payment"]')
    expect(action.text()).toBe('membership.continuePayment')
    expect(action.attributes('href')).toBe(redirectURL)
    expect(wrapper.text()).not.toContain(providerURL)
    expect(wrapper.text()).not.toContain('private-trace-id')
  })

  it('reissues a same-site ticket after page refresh for a pending payment without local storage', async () => {
    const pendingOrder = { ...order, input_required: false, payment_state: 'pending' }
    const redirectURL = '/api/v1/membership/payment/redirect?ticket=abcdefghijklmnopqrstuvwxyzABCDEFGHIJK123456'
    listOrders.mockResolvedValueOnce([pendingOrder])
    getOrder.mockResolvedValueOnce(pendingOrder)
    createPaymentRedirectTicket.mockResolvedValueOnce({ redirect_url: redirectURL, order_id: 1 })
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-test="detail-order-1"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="resume-membership-payment"]').trigger('click')
    await flushPromises()

    expect(createPaymentRedirectTicket).toHaveBeenCalledWith('order-1')
    expect(wrapper.get('[data-test="continue-membership-payment"]').attributes('href')).toBe(redirectURL)
    expect(JSON.stringify(localStorage)).not.toContain('ticket=')
  })
})
