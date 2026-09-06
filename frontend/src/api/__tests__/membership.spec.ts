import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post, put } = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), put: vi.fn() }))
vi.mock('../client', () => ({ apiClient: { get, post, put } }))

import {
  createMembershipOrder,
  createMembershipPayment,
  createMembershipPaymentRedirectTicket,
  getMembershipAdminOverview,
  listMembershipProducts,
  submitMembershipCredential,
  toMembershipOrder,
} from '../membership'

describe('membership API', () => {
  beforeEach(() => {
    get.mockReset().mockResolvedValue({ data: [] })
    post.mockReset().mockResolvedValue({ data: {} })
    put.mockReset().mockResolvedValue({ data: {} })
  })

  it('uses dedicated customer paths and includes a generated idempotency key', async () => {
    await listMembershipProducts()
    expect(get).toHaveBeenCalledWith('/membership/products')

    await createMembershipOrder({ sku: 'chatgpt_plus' })
    expect(post).toHaveBeenCalledWith('/membership/orders', expect.objectContaining({ sku: 'chatgpt_plus', idempotency_key: expect.any(String) }))

    await submitMembershipCredential('order/1', {
      mode: 'account_id', value: 'account-1234', consent: true, consent_version: 'membership-2026-09-07-v1',
    })
    expect(put).toHaveBeenCalledWith('/membership/orders/order%2F1/credential', expect.objectContaining({ value: 'account-1234' }))

    await createMembershipPayment('order/1', 'alipay')
    expect(post).toHaveBeenCalledWith('/membership/orders/order%2F1/payment', expect.objectContaining({
      payment_type: 'alipay', return_url: `${window.location.origin}/payment/result`,
    }))

    post.mockResolvedValueOnce({ data: {
      redirect_url: '/api/v1/membership/payment/redirect?ticket=abcdefghijklmnopqrstuvwxyzABCDEFGHIJK123456',
      order_id: 42,
    } })
    await expect(createMembershipPaymentRedirectTicket('order/1')).resolves.toEqual({
      redirect_url: '/api/v1/membership/payment/redirect?ticket=abcdefghijklmnopqrstuvwxyzABCDEFGHIJK123456', order_id: 42,
    })
    expect(post).toHaveBeenCalledWith('/membership/orders/order%2F1/payment/redirect-ticket')
    expect(post).not.toHaveBeenCalledWith('/payment/orders', expect.anything())
  })

  it('returns only the same-site membership payment redirect URL', async () => {
    post.mockResolvedValueOnce({ data: {
      redirect_url: '/api/v1/membership/payment/redirect?ticket=abcdefghijklmnopqrstuvwxyzABCDEFGHIJK123456',
      pay_url: 'https://pay.example.test/checkout',
      qr_code: 'http://localhost:5173/qr',
      order_id: 42,
    } })
    await expect(createMembershipPayment('order-1')).resolves.toEqual({
      redirect_url: '/api/v1/membership/payment/redirect?ticket=abcdefghijklmnopqrstuvwxyzABCDEFGHIJK123456',
      order_id: 42,
    })

    post.mockResolvedValueOnce({ data: {
      redirect_url: 'https://pay.example.test/checkout?provider_trace=private',
      pay_url: 'javascript:alert("private")',
      qr_code: 'weixin://wxpay/bizpayurl?pr=private',
      order_id: 43,
    } })
    await expect(createMembershipPayment('order-2')).resolves.toEqual({ redirect_url: '', order_id: 43 })

    post.mockResolvedValueOnce({ data: {
      redirect_url: '/api/v1/membership/orders/order-2/payment/redirect?payment_order_id=43',
      order_id: 43,
    } })
    await expect(createMembershipPayment('order-3')).resolves.toEqual({ redirect_url: '', order_id: 43 })
  })

  it('whitelists the order display DTO and converts unknown raw errors to a safe review code', () => {
    const order = toMembershipOrder({
      id: 'order-1', sku: 'chatgpt_plus', name: 'Codex/GPT Plus', kind: 'customer',
      payment_state: 'paid', fulfillment_state: 'processing', price_minor: 1000, discount_minor: 0,
      period_days: 30, target_masked: 'acct...1234', credential_mode: 'account_id', input_required: false,
      error_code: 'raw_provider_stacktrace.example', created_at: '2026-09-07T00:00:00Z', updated_at: '2026-09-07T00:00:00Z',
      upstream_url: 'https://supplier.example/private', cdk: 'SECRET-CDK', task_id: 'internal-task-1',
      events: [{ action: 'submitted', state: 'processing', error_code: 'untrusted', created_at: '2026-09-07T00:01:00Z', raw_error: 'secret' }],
    })

    expect(order.error_code).toBe('REVIEW_REQUIRED')
    expect(order.events[0].error_code).toBe('REVIEW_REQUIRED')
    expect(order).not.toHaveProperty('upstream_url')
    expect(order).not.toHaveProperty('cdk')
    expect(order).not.toHaveProperty('task_id')
  })

  it('maps the admin overview without retaining unapproved product or order fields', async () => {
    get.mockResolvedValueOnce({ data: {
      runtime_ready: true, payments_enabled: true, consent_version: 'membership-2026-09-07-v1',
      products: [{ product: { sku: 'chatgpt_plus', name: 'Codex/GPT Plus', price_minor: 100, currency: 'CNY', period_days: 30, credential_mode: 'account_id', for_sale: false, available: 0, eta_minutes: 30, secret_url: 'https://supplier.example' }, channel: 'gpt', paused: true }],
      orders: [{ id: 'order-1', sku: 'chatgpt_plus', name: 'Codex/GPT Plus' }], stats: { total: 1 }, ledger: { payment: 100 },
    } })

    const overview = await getMembershipAdminOverview()
    expect(get).toHaveBeenCalledWith('/admin/membership')
    expect(overview.products[0]).not.toHaveProperty('secret_url')
    expect(overview.orders[0]).not.toHaveProperty('task_id')
    expect(overview.ledger.profit).toBe(0)
  })
})
