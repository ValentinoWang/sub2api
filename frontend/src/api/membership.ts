import { apiClient } from './client'

export const MEMBERSHIP_CONSENT_VERSION = 'membership-2026-09-07-v1'

export type MembershipCredentialMode = 'account_id' | 'session'
export type MembershipPaymentState = 'created' | 'pending' | 'paid' | 'refund_pending' | 'refunded' | 'closed'
export type MembershipFulfillmentState =
  | 'awaiting_input'
  | 'queued'
  | 'submitted'
  | 'processing'
  | 'review_required'
  | 'succeeded'
  | 'failed'
  | 'canceled'
export type MembershipSafeErrorCode =
  | ''
  | 'CREDENTIAL_EXPIRED'
  | 'ACCOUNT_NOT_ELIGIBLE'
  | 'OUT_OF_STOCK'
  | 'PROCESSING'
  | 'REVIEW_REQUIRED'
  | 'FULFILLMENT_FAILED'
  | 'NOT_SUPPORTED'
  | 'CHANNEL_UNAVAILABLE'
  | 'PAGE_CHANGED'
  | 'LOGIN_REQUIRED'
  | 'CAPTCHA_REQUIRED'
  | 'PRODUCT_MISMATCH'

export interface MembershipProduct {
  sku: string
  name: string
  price_minor: number
  currency: string
  period_days: number
  credential_mode: MembershipCredentialMode
  for_sale: boolean
  available: number
  eta_minutes: number
}

export interface MembershipOrderEvent {
  action: string
  state: MembershipFulfillmentState
  error_code: MembershipSafeErrorCode
  created_at: string
}

export interface MembershipOrder {
  id: string
  sku: string
  name: string
  kind: 'customer' | 'validation'
  payment_state: MembershipPaymentState
  fulfillment_state: MembershipFulfillmentState
  price_minor: number
  discount_minor: number
  period_days: number
  target_masked: string
  credential_mode: MembershipCredentialMode
  input_required: boolean
  error_code: MembershipSafeErrorCode
  refund_requested_at: string | null
  created_at: string
  updated_at: string
  events: MembershipOrderEvent[]
}

export interface MembershipAdminProduct extends MembershipProduct {
  channel: 'gpt' | 'gptpro'
  paused: boolean
  verified_at: string | null
  inventory_checked_at: string | null
  poll_seconds: number
  wait_seconds: number
}

export interface MembershipAdminOverview {
  products: MembershipAdminProduct[]
  orders: MembershipOrder[]
  stats: {
    total: number
    success: number
    review: number
    queued: number
    mean_seconds: number
  }
  ledger: Record<'payment' | 'fee' | 'supplier_cost' | 'refund' | 'profit', number>
  runtime_ready: boolean
  payments_enabled: boolean
  consent_version: string
}

export interface CreateMembershipOrderInput {
  sku: string
  coupon?: string
  idempotency_key: string
}

export interface CredentialSubmission {
  mode: MembershipCredentialMode
  value: string
  account_id?: string
  consent_version: typeof MEMBERSHIP_CONSENT_VERSION
  consent: true
}

export interface MembershipProductUpdate {
  price_minor: number
  period_days: number
  channel: 'gpt' | 'gptpro'
  credential_mode: MembershipCredentialMode
  for_sale: boolean
  paused: boolean
  poll_seconds: number
  wait_seconds: number
  eta_minutes: number
}

const safeErrors = new Set<MembershipSafeErrorCode>([
  '', 'CREDENTIAL_EXPIRED', 'ACCOUNT_NOT_ELIGIBLE', 'OUT_OF_STOCK', 'PROCESSING',
  'REVIEW_REQUIRED', 'FULFILLMENT_FAILED', 'NOT_SUPPORTED', 'CHANNEL_UNAVAILABLE',
  'PAGE_CHANGED', 'LOGIN_REQUIRED', 'CAPTCHA_REQUIRED', 'PRODUCT_MISMATCH',
])

function record(value: unknown): Record<string, unknown> {
  return typeof value === 'object' && value !== null ? value as Record<string, unknown> : {}
}

function text(value: unknown): string {
  return typeof value === 'string' ? value : ''
}

function number(value: unknown): number {
  return typeof value === 'number' && Number.isFinite(value) ? value : 0
}

function bool(value: unknown): boolean {
  return value === true
}

/** Membership checkout never accepts an upstream provider target in the browser. */
function paymentRedirectURL(value: unknown): string {
  const raw = text(value).trim()
  if (!raw || typeof window === 'undefined') return ''
  try {
    const parsed = new URL(raw, window.location.origin)
    const ticket = parsed.searchParams.get('ticket')
    if (
      parsed.origin !== window.location.origin ||
      parsed.pathname !== '/api/v1/membership/payment/redirect' ||
      !/^[A-Za-z0-9_-]{43}$/.test(ticket || '') ||
      parsed.searchParams.size !== 1 ||
      parsed.hash
    ) return ''
    return `${parsed.pathname}${parsed.search}`
  } catch {
    return ''
  }
}

function safeMode(value: unknown): MembershipCredentialMode {
  return value === 'session' ? 'session' : 'account_id'
}

function safePaymentState(value: unknown): MembershipPaymentState {
  return ['created', 'pending', 'paid', 'refund_pending', 'refunded', 'closed'].includes(text(value))
    ? text(value) as MembershipPaymentState
    : 'created'
}

function safeFulfillmentState(value: unknown): MembershipFulfillmentState {
  return ['awaiting_input', 'queued', 'submitted', 'processing', 'review_required', 'succeeded', 'failed', 'canceled'].includes(text(value))
    ? text(value) as MembershipFulfillmentState
    : 'awaiting_input'
}

function safeError(value: unknown): MembershipSafeErrorCode {
  const code = text(value) as MembershipSafeErrorCode
  return safeErrors.has(code) ? code : 'REVIEW_REQUIRED'
}

/** Pick the display contract deliberately; secrets and upstream implementation fields never escape here. */
export function toMembershipProduct(value: unknown): MembershipProduct {
  const source = record(value)
  return {
    sku: text(source.sku), name: text(source.name), price_minor: number(source.price_minor),
    currency: text(source.currency) || 'CNY', period_days: number(source.period_days),
    credential_mode: safeMode(source.credential_mode), for_sale: bool(source.for_sale),
    available: number(source.available), eta_minutes: number(source.eta_minutes),
  }
}

export function toMembershipOrder(value: unknown): MembershipOrder {
  const source = record(value)
  const events = Array.isArray(source.events) ? source.events.map((event): MembershipOrderEvent => {
    const entry = record(event)
    return { action: text(entry.action), state: safeFulfillmentState(entry.state), error_code: safeError(entry.error_code), created_at: text(entry.created_at) }
  }) : []
  return {
    id: text(source.id), sku: text(source.sku), name: text(source.name),
    kind: source.kind === 'validation' ? 'validation' : 'customer', payment_state: safePaymentState(source.payment_state),
    fulfillment_state: safeFulfillmentState(source.fulfillment_state), price_minor: number(source.price_minor),
    discount_minor: number(source.discount_minor), period_days: number(source.period_days), target_masked: text(source.target_masked),
    credential_mode: safeMode(source.credential_mode), input_required: bool(source.input_required),
    error_code: safeError(source.error_code), refund_requested_at: text(source.refund_requested_at) || null,
    created_at: text(source.created_at), updated_at: text(source.updated_at), events,
  }
}

function toAdminProduct(value: unknown): MembershipAdminProduct {
  const source = record(value)
  const product = toMembershipProduct(source.product ?? source)
  return {
    ...product, channel: source.channel === 'gptpro' ? 'gptpro' : 'gpt', paused: bool(source.paused),
    verified_at: text(source.verified_at) || null, inventory_checked_at: text(source.inventory_checked_at) || null,
    poll_seconds: number(source.poll_seconds), wait_seconds: number(source.wait_seconds),
  }
}

function idempotencyKey(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') return crypto.randomUUID()
  return `membership-${Date.now()}-${Math.random().toString(36).slice(2)}`
}

export async function listMembershipProducts(): Promise<MembershipProduct[]> {
  const { data } = await apiClient.get<unknown>('/membership/products')
  return Array.isArray(data) ? data.map(toMembershipProduct) : []
}

export async function createMembershipOrder(input: Omit<CreateMembershipOrderInput, 'idempotency_key'> & { idempotency_key?: string }): Promise<{ id: string }> {
  const { data } = await apiClient.post<{ id?: unknown }>('/membership/orders', { ...input, idempotency_key: input.idempotency_key || idempotencyKey() })
  return { id: text(data?.id) }
}

export async function listMembershipOrders(): Promise<MembershipOrder[]> {
  const { data } = await apiClient.get<unknown>('/membership/orders')
  return Array.isArray(data) ? data.map(toMembershipOrder) : []
}

export async function getMembershipOrder(id: string): Promise<MembershipOrder> {
  const { data } = await apiClient.get<unknown>(`/membership/orders/${encodeURIComponent(id)}`)
  return toMembershipOrder(data)
}

export async function submitMembershipCredential(id: string, input: CredentialSubmission): Promise<{ accepted: boolean; expires_in_seconds: number }> {
  const { data } = await apiClient.put<{ accepted?: unknown; expires_in_seconds?: unknown }>(`/membership/orders/${encodeURIComponent(id)}/credential`, input)
  return { accepted: bool(data?.accepted), expires_in_seconds: number(data?.expires_in_seconds) }
}

export async function requestMembershipRefund(id: string, reason: 'not_delivered' | 'wrong_plan' | 'cancel_request'): Promise<void> {
  await apiClient.post(`/membership/orders/${encodeURIComponent(id)}/refund`, { reason })
}

export async function createMembershipPayment(id: string, paymentType: 'wxpay' | 'alipay' = 'wxpay'): Promise<{ redirect_url: string; order_id: number }> {
  const { data } = await apiClient.post<{ redirect_url?: unknown; order_id?: unknown }>(`/membership/orders/${encodeURIComponent(id)}/payment`, {
    payment_type: paymentType,
    return_url: typeof window === 'undefined' ? '' : `${window.location.origin}/payment/result`,
  })
  return { redirect_url: paymentRedirectURL(data?.redirect_url), order_id: number(data?.order_id) }
}

/** Recover a short-lived same-site payment URL for an existing pending order. */
export async function createMembershipPaymentRedirectTicket(id: string): Promise<{ redirect_url: string; order_id: number }> {
  const { data } = await apiClient.post<{ redirect_url?: unknown; order_id?: unknown }>(`/membership/orders/${encodeURIComponent(id)}/payment/redirect-ticket`)
  return { redirect_url: paymentRedirectURL(data?.redirect_url), order_id: number(data?.order_id) }
}

export async function getMembershipAdminOverview(): Promise<MembershipAdminOverview> {
  const { data } = await apiClient.get<unknown>('/admin/membership')
  const source = record(data)
  const stats = record(source.stats)
  const ledger = record(source.ledger)
  return {
    products: Array.isArray(source.products) ? source.products.map(toAdminProduct) : [],
    orders: Array.isArray(source.orders) ? source.orders.map(toMembershipOrder) : [],
    stats: { total: number(stats.total), success: number(stats.success), review: number(stats.review), queued: number(stats.queued), mean_seconds: number(stats.mean_seconds) },
    ledger: { payment: number(ledger.payment), fee: number(ledger.fee), supplier_cost: number(ledger.supplier_cost), refund: number(ledger.refund), profit: number(ledger.profit) },
    runtime_ready: bool(source.runtime_ready), payments_enabled: bool(source.payments_enabled), consent_version: text(source.consent_version),
  }
}

export async function updateMembershipProduct(sku: string, input: MembershipProductUpdate): Promise<void> {
  await apiClient.put(`/admin/membership/products/${encodeURIComponent(sku)}`, input)
}
export async function checkMembershipAvailability(sku: string): Promise<void> {
  await apiClient.post(`/admin/membership/products/${encodeURIComponent(sku)}/availability`)
}
export async function createMembershipValidationOrder(input: CreateMembershipOrderInput): Promise<{ id: string }> {
  const { data } = await apiClient.post<{ id?: unknown }>('/admin/membership/validation-orders', input)
  return { id: text(data?.id) }
}
export async function verifyMembershipProduct(sku: string, runID: string, evidenceRef: string): Promise<void> {
  await apiClient.post(`/admin/membership/products/${encodeURIComponent(sku)}/verify`, { run_id: runID, evidence_ref: evidenceRef })
}
export async function importMembershipCDKs(sku: string, codes: string[], costMinor: number): Promise<{ imported: number }> {
  const { data } = await apiClient.post<{ imported?: unknown }>('/admin/membership/cdks/import', { sku, codes, cost_minor: costMinor })
  return { imported: number(data?.imported) }
}
export async function reviewMembershipOrder(id: string, action: 'query' | 'confirm_success' | 'confirm_not_submitted' | 'retry' | 'cancel', evidenceRef: string): Promise<void> {
  await apiClient.post(`/admin/membership/orders/${encodeURIComponent(id)}/review`, { action, evidence_ref: evidenceRef })
}
export async function saveMembershipCoupon(input: { code: string; discount_minor: number; max_uses: number; expires_at: string }): Promise<void> {
  await apiClient.post('/admin/membership/coupons', input)
}

export const membershipAPI = {
  listProducts: listMembershipProducts, createOrder: createMembershipOrder, listOrders: listMembershipOrders,
  getOrder: getMembershipOrder, submitCredential: submitMembershipCredential, requestRefund: requestMembershipRefund,
  createPayment: createMembershipPayment, createPaymentRedirectTicket: createMembershipPaymentRedirectTicket, getAdminOverview: getMembershipAdminOverview, updateProduct: updateMembershipProduct,
  checkAvailability: checkMembershipAvailability, createValidationOrder: createMembershipValidationOrder,
  verifyProduct: verifyMembershipProduct, importCDKs: importMembershipCDKs, reviewOrder: reviewMembershipOrder,
  saveCoupon: saveMembershipCoupon,
}
