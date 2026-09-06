<template>
  <AppLayout>
    <main class="mx-auto w-full max-w-6xl space-y-6 px-3 py-5 sm:px-5">
      <header class="flex flex-wrap items-end justify-between gap-3 border-b border-gray-200 pb-4 dark:border-dark-700">
        <div class="min-w-0">
          <h1 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('membership.title') }}</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('membership.consentPurpose') }}</p>
        </div>
        <button class="btn btn-secondary btn-sm" :disabled="loading" data-test="refresh-memberships" @click="load">
          <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
          {{ t('common.refresh') }}
        </button>
      </header>

      <section aria-labelledby="membership-catalog-title">
        <div class="mb-3 flex items-center justify-between gap-3">
          <h2 id="membership-catalog-title" class="text-base font-semibold text-gray-900 dark:text-white">{{ t('membership.catalog') }}</h2>
          <span class="text-xs text-gray-500 dark:text-gray-400">{{ products.length }}</span>
        </div>
        <div v-if="loading" class="py-10 text-center text-sm text-gray-500">{{ t('common.loading') }}</div>
        <div v-else class="grid gap-4 md:grid-cols-2">
          <article v-for="product in products" :key="product.sku" class="card flex min-w-0 flex-col border border-gray-200 p-5 dark:border-dark-700" :data-test="`product-${product.sku}`">
            <div class="flex flex-wrap items-start justify-between gap-3">
              <div class="min-w-0">
                <h3 class="break-words text-base font-semibold text-gray-900 dark:text-white">{{ product.name }}</h3>
                <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ product.period_days }} {{ t('membership.days') }} · {{ product.eta_minutes }} min</p>
              </div>
              <span class="badge shrink-0" :class="product.for_sale ? 'badge-success' : 'badge-warning'">{{ product.for_sale ? t('common.available') : t('membership.unavailable') }}</span>
            </div>
            <div class="mt-4 flex flex-wrap items-center justify-between gap-3 border-t border-gray-100 pt-4 dark:border-dark-700">
              <div>
                <p class="text-lg font-semibold text-gray-900 dark:text-white">{{ money(product.price_minor, product.currency) }}</p>
                <p class="text-xs text-gray-500 dark:text-gray-400">{{ product.available > 0 ? `${product.available} ${t('common.available')}` : t('membership.outOfStock') }}</p>
              </div>
              <button class="btn btn-primary btn-sm" :disabled="!product.for_sale || creatingSKU === product.sku" :data-test="`create-${product.sku}`" @click="createOrder(product)">
                {{ creatingSKU === product.sku ? t('common.processing') : t('membership.continue') }}
              </button>
            </div>
          </article>
        </div>
      </section>

      <section aria-labelledby="membership-orders-title">
        <div class="mb-3 flex items-center justify-between gap-3">
          <h2 id="membership-orders-title" class="text-base font-semibold text-gray-900 dark:text-white">{{ t('membership.orders') }}</h2>
          <span class="text-xs text-gray-500 dark:text-gray-400">{{ orders.length }}</span>
        </div>
        <p v-if="!orders.length && !loading" class="border border-dashed border-gray-300 px-4 py-8 text-center text-sm text-gray-500 dark:border-dark-600">{{ t('membership.noOrders') }}</p>
        <div v-else class="space-y-3">
          <article v-for="order in orders" :key="order.id" class="card min-w-0 border border-gray-200 p-4 dark:border-dark-700" :data-test="`order-${order.id}`">
            <div class="flex flex-wrap items-start justify-between gap-3">
              <div class="min-w-0">
                <div class="flex flex-wrap items-center gap-2">
                  <h3 class="font-medium text-gray-900 dark:text-white">{{ order.name }}</h3>
                  <span class="badge" :class="stateClass(order.fulfillment_state)">{{ stateLabel(order.fulfillment_state) }}</span>
                </div>
                <p class="mt-1 break-all text-xs text-gray-500 dark:text-gray-400">{{ t('membership.detail') }} · {{ order.target_masked || '-' }} · {{ date(order.updated_at) }}</p>
              </div>
              <button class="btn btn-secondary btn-sm" :data-test="`detail-${order.id}`" @click="toggleDetail(order)">{{ selectedOrder?.id === order.id ? t('common.close') : t('membership.detail') }}</button>
            </div>

            <div v-if="selectedOrder?.id === order.id" class="mt-4 space-y-4 border-t border-gray-100 pt-4 dark:border-dark-700" data-test="membership-order-detail">
              <div v-if="order.input_required" class="rounded border border-amber-200 bg-amber-50 p-4 dark:border-amber-900/50 dark:bg-amber-950/20">
                <h4 class="font-medium text-amber-900 dark:text-amber-100">{{ t('membership.completeDetails') }}</h4>
                <form class="mt-3 space-y-3" @submit.prevent="submitCredential(order)">
                  <label class="block text-sm font-medium text-gray-700 dark:text-gray-300" :for="`credential-${order.id}`">{{ order.credential_mode === 'session' ? t('membership.session') : t('membership.accountId') }}</label>
                  <textarea v-if="order.credential_mode === 'session'" :id="`credential-${order.id}`" v-model="credentialValue" class="input min-h-28 w-full font-mono text-xs" autocomplete="off" spellcheck="false" data-test="session-input" />
                  <input v-else :id="`credential-${order.id}`" v-model="credentialValue" class="input w-full" autocomplete="off" data-test="account-id-input" />
                  <label v-if="order.credential_mode === 'session'" class="block text-sm font-medium text-gray-700 dark:text-gray-300" :for="`account-${order.id}`">{{ t('membership.accountId') }}</label>
                  <input v-if="order.credential_mode === 'session'" :id="`account-${order.id}`" v-model="accountID" class="input w-full" autocomplete="off" data-test="session-account-input" />
                  <ul class="space-y-1 text-xs text-gray-600 dark:text-gray-300">
                    <li>{{ t('membership.consentPurpose') }}</li><li>{{ t('membership.consentPartner') }}</li><li>{{ t('membership.consentRetention') }}</li><li>{{ t('membership.consentRisk') }}</li>
                  </ul>
                  <label class="flex items-start gap-2 text-sm text-gray-700 dark:text-gray-300"><input v-model="consent" type="checkbox" class="mt-1" data-test="credential-consent" /><span>{{ t('membership.consent') }}</span></label>
                  <button class="btn btn-primary btn-sm" :disabled="!credentialValue.trim() || !consent || submittingCredential" data-test="submit-credential">{{ submittingCredential ? t('common.processing') : t('common.submit') }}</button>
                </form>
              </div>

              <div v-if="canPay(order)" class="flex flex-wrap items-center justify-between gap-3 rounded border border-blue-200 bg-blue-50 p-3 dark:border-blue-900/50 dark:bg-blue-950/20">
                <span class="text-sm text-blue-900 dark:text-blue-100">{{ money(order.price_minor, 'CNY') }}</span>
                <button class="btn btn-primary btn-sm" :disabled="payingOrderID === order.id" data-test="membership-payment" @click="pay(order)">{{ payingOrderID === order.id ? t('common.processing') : t('membership.payment') }}</button>
              </div>
              <div v-if="canContinuePayment(order)" class="flex flex-wrap items-center justify-between gap-3 rounded border border-blue-200 bg-blue-50 p-3 dark:border-blue-900/50 dark:bg-blue-950/20">
                <span class="text-sm text-blue-900 dark:text-blue-100">{{ money(order.price_minor, 'CNY') }}</span>
                <button class="btn btn-primary btn-sm" :disabled="payingOrderID === order.id" data-test="resume-membership-payment" @click="resumePayment(order)">{{ payingOrderID === order.id ? t('common.processing') : t('membership.continuePayment') }}</button>
              </div>
              <a v-if="paymentRedirectURL && payingOrderID === order.id" :href="paymentRedirectURL" target="_blank" rel="noopener noreferrer" class="btn btn-secondary btn-sm" data-test="continue-membership-payment">{{ t('membership.continuePayment') }}</a>
              <div v-if="order.payment_state === 'paid' && !order.refund_requested_at && order.fulfillment_state !== 'succeeded'" class="flex flex-wrap items-center gap-2">
                <select v-model="refundReason" class="input max-w-52 text-sm" data-test="refund-reason"><option value="not_delivered">not delivered</option><option value="wrong_plan">wrong plan</option><option value="cancel_request">cancel request</option></select>
                <button class="btn btn-secondary btn-sm" data-test="request-refund" @click="refund(order)">{{ t('membership.requestRefund') }}</button>
              </div>

              <div>
                <h4 class="text-sm font-medium text-gray-900 dark:text-white">{{ t('membership.timeline') }}</h4>
                <ol class="mt-2 space-y-2 border-l border-gray-200 pl-4 dark:border-dark-600">
                  <li v-for="event in order.events" :key="`${event.created_at}-${event.action}`" class="text-sm text-gray-600 dark:text-gray-300"><span class="font-medium">{{ stateLabel(event.state) }}</span><span v-if="event.error_code" class="ml-2 text-amber-700 dark:text-amber-300">{{ event.error_code }}</span><span class="ml-2 text-xs text-gray-500">{{ date(event.created_at) }}</span></li>
                </ol>
              </div>
            </div>
          </article>
        </div>
      </section>
    </main>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores'
import { formatDateTimeToMinute } from '@/utils/format'
import { membershipAPI, MEMBERSHIP_CONSENT_VERSION, type MembershipFulfillmentState, type MembershipOrder, type MembershipProduct } from '@/api/membership'

const { t } = useI18n()
const appStore = useAppStore()
const products = ref<MembershipProduct[]>([])
const orders = ref<MembershipOrder[]>([])
const selectedOrder = ref<MembershipOrder | null>(null)
const loading = ref(false)
const creatingSKU = ref('')
const credentialValue = ref('')
const accountID = ref('')
const consent = ref(false)
const submittingCredential = ref(false)
const payingOrderID = ref('')
const paymentRedirectURL = ref('')
const refundReason = ref<'not_delivered' | 'wrong_plan' | 'cancel_request'>('not_delivered')

/** Customer errors are intentionally code-only. Never surface Axios/server message/detail text here. */
function message(error: unknown): string {
  const source = typeof error === 'object' && error !== null ? error as Record<string, unknown> : {}
  const code = [source.code, source.reason].find((value): value is string => typeof value === 'string') || ''
  if (code === 'CREDENTIAL_EXPIRED') return t('membership.errors.credentialExpired')
  if (code === 'REVIEW_REQUIRED' || code === 'MEMBERSHIP_CONFLICT') return t('membership.errors.reviewRequired')
  return t('membership.errors.unavailable')
}
function money(minor: number, currency: string): string { return new Intl.NumberFormat(undefined, { style: 'currency', currency }).format(minor / 100) }
function date(value: string): string { return formatDateTimeToMinute(value) || '-' }
function stateLabel(state: MembershipFulfillmentState): string { return t(`membership.states.${state}`) }
function stateClass(state: MembershipFulfillmentState): string { return state === 'succeeded' ? 'badge-success' : state === 'review_required' || state === 'failed' ? 'badge-warning' : state === 'canceled' ? 'badge-danger' : 'badge-primary' }
function canPay(order: MembershipOrder): boolean { return order.payment_state === 'created' && !order.input_required && order.fulfillment_state !== 'canceled' }
function canContinuePayment(order: MembershipOrder): boolean { return order.payment_state === 'pending' && order.fulfillment_state !== 'canceled' }

async function load(): Promise<void> {
  loading.value = true
  try { [products.value, orders.value] = await Promise.all([membershipAPI.listProducts(), membershipAPI.listOrders()]) }
  catch (error) { appStore.showError(message(error)) }
  finally { loading.value = false }
}

async function createOrder(product: MembershipProduct): Promise<void> {
  creatingSKU.value = product.sku
  try {
    const created = await membershipAPI.createOrder({ sku: product.sku })
    const order = await membershipAPI.getOrder(created.id)
    orders.value = [order, ...orders.value.filter((item) => item.id !== order.id)]
    selectedOrder.value = order
  } catch (error) { appStore.showError(message(error)) }
  finally { creatingSKU.value = '' }
}

async function toggleDetail(order: MembershipOrder): Promise<void> {
  if (selectedOrder.value?.id === order.id) { selectedOrder.value = null; return }
  try { selectedOrder.value = await membershipAPI.getOrder(order.id) }
  catch (error) { appStore.showError(message(error)) }
}

async function submitCredential(order: MembershipOrder): Promise<void> {
  submittingCredential.value = true
  try {
    await membershipAPI.submitCredential(order.id, { mode: order.credential_mode, value: credentialValue.value, account_id: accountID.value || undefined, consent: true, consent_version: MEMBERSHIP_CONSENT_VERSION })
    credentialValue.value = ''
    accountID.value = ''
    consent.value = false
    const refreshed = await membershipAPI.getOrder(order.id)
    selectedOrder.value = refreshed
    orders.value = orders.value.map((item) => item.id === refreshed.id ? refreshed : item)
  } catch (error) { appStore.showError(message(error)) }
  finally { submittingCredential.value = false }
}

async function pay(order: MembershipOrder): Promise<void> {
  payingOrderID.value = order.id
  paymentRedirectURL.value = ''
  try {
    const payment = await membershipAPI.createPayment(order.id)
    paymentRedirectURL.value = payment.redirect_url
    appStore.showSuccess(t('membership.paymentCreated'))
  } catch (error) { appStore.showError(message(error)) }
  finally { if (!paymentRedirectURL.value) payingOrderID.value = '' }
}

async function resumePayment(order: MembershipOrder): Promise<void> {
  payingOrderID.value = order.id
  paymentRedirectURL.value = ''
  try {
    const payment = await membershipAPI.createPaymentRedirectTicket(order.id)
    paymentRedirectURL.value = payment.redirect_url
  } catch (error) { appStore.showError(message(error)) }
  finally { if (!paymentRedirectURL.value) payingOrderID.value = '' }
}

async function refund(order: MembershipOrder): Promise<void> {
  try { await membershipAPI.requestRefund(order.id, refundReason.value); await toggleDetail(order); appStore.showSuccess(t('common.success')) }
  catch (error) { appStore.showError(message(error)) }
}

onMounted(load)
</script>
