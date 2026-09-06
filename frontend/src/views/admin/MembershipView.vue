<template>
  <AppLayout>
    <main class="mx-auto w-full max-w-7xl space-y-6 px-3 py-5 sm:px-5">
      <header class="flex flex-wrap items-end justify-between gap-3 border-b border-gray-200 pb-4 dark:border-dark-700">
        <div class="min-w-0">
          <h1 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('adminMembership.title') }}</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('adminMembership.safeData') }}</p>
        </div>
        <button class="btn btn-secondary btn-sm" :disabled="loading" data-test="refresh-overview" @click="load"><Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />{{ t('common.refresh') }}</button>
      </header>

      <section class="grid gap-3 sm:grid-cols-2 xl:grid-cols-5" aria-label="Membership runtime summary">
        <div class="card border border-gray-200 p-4 dark:border-dark-700"><p class="text-xs text-gray-500">{{ t('adminMembership.runtime') }}</p><p class="mt-1 font-semibold" :class="overview?.runtime_ready ? 'text-green-600' : 'text-amber-600'">{{ overview?.runtime_ready ? t('common.available') : t('membership.unavailable') }}</p></div>
        <div class="card border border-gray-200 p-4 dark:border-dark-700"><p class="text-xs text-gray-500">{{ t('adminMembership.products') }}</p><p class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">{{ overview?.products.length ?? 0 }}</p></div>
        <div class="card border border-gray-200 p-4 dark:border-dark-700"><p class="text-xs text-gray-500">{{ t('adminMembership.orders') }}</p><p class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">{{ overview?.stats.total ?? 0 }}</p></div>
        <div class="card border border-gray-200 p-4 dark:border-dark-700"><p class="text-xs text-gray-500">{{ t('membership.status') }}</p><p class="mt-1 text-xl font-semibold text-amber-600">{{ overview?.stats.review ?? 0 }}</p></div>
        <div class="card border border-gray-200 p-4 dark:border-dark-700"><p class="text-xs text-gray-500">{{ t('adminMembership.coupon') }}</p><p class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">{{ money(overview?.ledger.profit ?? 0) }}</p></div>
      </section>

      <section class="card overflow-hidden border border-gray-200 dark:border-dark-700" aria-labelledby="membership-products-title">
        <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-200 p-4 dark:border-dark-700"><h2 id="membership-products-title" class="font-semibold text-gray-900 dark:text-white">{{ t('adminMembership.products') }}</h2><span class="text-xs text-gray-500">{{ overview?.payments_enabled ? t('common.enabled') : t('common.disabled') }}</span></div>
        <div class="overflow-x-auto">
          <table class="w-full min-w-[850px] text-left text-sm" data-test="membership-product-table">
            <thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-800/60"><tr><th class="px-4 py-3">SKU</th><th class="px-4 py-3">{{ t('membership.catalog') }}</th><th class="px-4 py-3">{{ t('membership.status') }}</th><th class="px-4 py-3">{{ t('adminMembership.availability') }}</th><th class="px-4 py-3">{{ t('common.actions') }}</th></tr></thead>
            <tbody>
              <tr v-for="product in overview?.products" :key="product.sku" class="border-t border-gray-100 align-top dark:border-dark-700">
                <td class="px-4 py-4 font-mono text-xs text-gray-700 dark:text-gray-300">{{ product.sku }}</td>
                <td class="px-4 py-4"><p class="font-medium text-gray-900 dark:text-white">{{ product.name }}</p><p class="mt-1 text-xs text-gray-500">{{ product.channel }} · {{ product.credential_mode }} · {{ product.available }} {{ t('common.available') }}</p></td>
                <td class="px-4 py-4"><span class="badge" :class="product.for_sale ? 'badge-success' : 'badge-warning'">{{ product.for_sale ? t('common.available') : t('membership.unavailable') }}</span><p class="mt-1 text-xs text-gray-500">{{ product.verified_at ? date(product.verified_at) : '-' }}</p></td>
                <td class="px-4 py-4"><p>{{ product.inventory_checked_at ? date(product.inventory_checked_at) : '-' }}</p><p class="mt-1 text-xs text-gray-500">{{ product.poll_seconds }}s / {{ product.wait_seconds }}s</p></td>
                <td class="px-4 py-4"><div class="flex flex-wrap gap-2"><button class="btn btn-secondary btn-sm" :data-test="`availability-${product.sku}`" @click="runSensitive(() => membershipAPI.checkAvailability(product.sku))">{{ t('adminMembership.availability') }}</button><button class="btn btn-secondary btn-sm" :data-test="`edit-${product.sku}`" @click="openProduct(product)">{{ t('common.edit') }}</button></div></td>
              </tr>
              <tr v-if="!overview?.products.length"><td colspan="5" class="px-4 py-10 text-center text-gray-500">{{ loading ? t('common.loading') : '-' }}</td></tr>
            </tbody>
          </table>
        </div>
      </section>

      <div class="grid gap-6 xl:grid-cols-2">
        <section class="card border border-gray-200 p-5 dark:border-dark-700" aria-labelledby="membership-stock-title">
          <h2 id="membership-stock-title" class="font-semibold text-gray-900 dark:text-white">{{ t('adminMembership.stock') }}</h2>
          <form class="mt-4 space-y-3" @submit.prevent="importStock">
            <select v-model="stockSKU" class="input w-full" data-test="stock-sku"><option disabled value="">SKU</option><option v-for="item in overview?.products" :key="item.sku" :value="item.sku">{{ item.name }}</option></select>
            <textarea v-model="stockCodes" class="input min-h-28 w-full font-mono text-xs" autocomplete="off" placeholder="One code per line" data-test="stock-codes" />
            <input v-model.number="stockCost" class="input w-full" type="number" min="0" step="1" data-test="stock-cost" />
            <button class="btn btn-primary btn-sm" :disabled="!stockSKU || !stockCodes.trim() || stockBusy" data-test="import-cdks">{{ stockBusy ? t('common.processing') : t('common.import') }}</button>
          </form>
        </section>
        <section class="card border border-gray-200 p-5 dark:border-dark-700" aria-labelledby="membership-validation-title">
          <h2 id="membership-validation-title" class="font-semibold text-gray-900 dark:text-white">{{ t('adminMembership.validation') }}</h2>
          <form class="mt-4 space-y-3" @submit.prevent="createValidation"><select v-model="validationSKU" class="input w-full" data-test="validation-sku"><option disabled value="">SKU</option><option v-for="item in overview?.products" :key="item.sku" :value="item.sku">{{ item.name }}</option></select><button class="btn btn-secondary btn-sm" :disabled="!validationSKU" data-test="create-validation">{{ t('common.create') }}</button></form>
          <form class="mt-4 grid gap-2 sm:grid-cols-3" @submit.prevent="verifyProduct"><input v-model="verificationSKU" class="input min-w-0" placeholder="SKU" data-test="verify-sku" /><input v-model="verificationRun" class="input min-w-0" placeholder="Validation order ID" data-test="verify-run" /><input v-model="verificationEvidence" class="input min-w-0" placeholder="Evidence ref" data-test="verify-evidence" /><button class="btn btn-primary btn-sm sm:col-span-3" :disabled="!verificationSKU || !verificationRun || !verificationEvidence" data-test="verify-product">{{ t('common.verify') }}</button></form>
        </section>
      </div>

      <div class="grid gap-6 xl:grid-cols-2">
        <section class="card border border-gray-200 p-5 dark:border-dark-700" aria-labelledby="membership-coupon-title"><h2 id="membership-coupon-title" class="font-semibold text-gray-900 dark:text-white">{{ t('adminMembership.coupon') }}</h2><form class="mt-4 grid gap-3 sm:grid-cols-2" @submit.prevent="saveCoupon"><input v-model="coupon.code" class="input" placeholder="Coupon code" data-test="coupon-code" /><input v-model.number="coupon.discount_minor" class="input" type="number" min="1" placeholder="Discount minor" data-test="coupon-discount" /><input v-model.number="coupon.max_uses" class="input" type="number" min="1" placeholder="Max uses" data-test="coupon-max-uses" /><input v-model="coupon.expires_at" class="input" type="datetime-local" data-test="coupon-expires" /><button class="btn btn-primary btn-sm sm:col-span-2" :disabled="!coupon.code || coupon.discount_minor <= 0 || coupon.max_uses <= 0 || !coupon.expires_at" data-test="save-coupon">{{ t('common.save') }}</button></form></section>
        <section class="card border border-gray-200 p-5 dark:border-dark-700" aria-labelledby="membership-review-title"><h2 id="membership-review-title" class="font-semibold text-gray-900 dark:text-white">{{ t('adminMembership.review') }}</h2><form class="mt-4 grid gap-3 sm:grid-cols-2" @submit.prevent="review"><input v-model="reviewID" class="input" placeholder="Order ID" data-test="review-order" /><select v-model="reviewAction" class="input" data-test="review-action"><option value="query">query</option><option value="confirm_success">confirm success</option><option value="confirm_not_submitted">confirm not submitted</option><option value="retry">retry</option><option value="cancel">cancel</option></select><input v-model="reviewEvidence" class="input sm:col-span-2" placeholder="Evidence ref" data-test="review-evidence" /><button class="btn btn-secondary btn-sm sm:col-span-2" :disabled="!reviewID || !reviewEvidence" data-test="review-order-submit">{{ t('adminMembership.review') }}</button></form></section>
      </div>

      <section class="card overflow-hidden border border-gray-200 dark:border-dark-700" aria-labelledby="membership-orders-title"><div class="border-b border-gray-200 p-4 dark:border-dark-700"><h2 id="membership-orders-title" class="font-semibold text-gray-900 dark:text-white">{{ t('adminMembership.orders') }}</h2></div><div class="overflow-x-auto"><table class="w-full min-w-[760px] text-left text-sm" data-test="membership-orders-table"><thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-800/60"><tr><th class="px-4 py-3">{{ t('membership.detail') }}</th><th class="px-4 py-3">{{ t('membership.status') }}</th><th class="px-4 py-3">{{ t('membership.timeline') }}</th><th class="px-4 py-3">{{ t('common.updatedAt') }}</th></tr></thead><tbody><tr v-for="order in overview?.orders" :key="order.id" class="border-t border-gray-100 dark:border-dark-700"><td class="px-4 py-3"><p class="font-medium text-gray-900 dark:text-white">{{ order.name }}</p><p class="max-w-64 truncate font-mono text-xs text-gray-500">{{ order.id }}</p></td><td class="px-4 py-3"><span class="badge" :class="stateClass(order.fulfillment_state)">{{ order.fulfillment_state }}</span><p v-if="order.error_code" class="mt-1 text-xs text-amber-700">{{ order.error_code }}</p></td><td class="px-4 py-3 text-xs text-gray-600 dark:text-gray-300">{{ order.events.length }} events</td><td class="px-4 py-3 text-xs text-gray-500">{{ date(order.updated_at) }}</td></tr></tbody></table></div></section>

      <BaseDialog :show="productDialog" :title="t('common.edit')" @close="productDialog = false"><form class="space-y-3" @submit.prevent="saveProduct"><label class="block text-sm"><span>Price (minor)</span><input v-model.number="productForm.price_minor" class="input mt-1 w-full" type="number" min="0" /></label><label class="flex gap-2 text-sm"><input v-model="productForm.for_sale" type="checkbox" />{{ t('common.enabled') }}</label><label class="flex gap-2 text-sm"><input v-model="productForm.paused" type="checkbox" />Paused</label><div class="flex justify-end gap-2"><button type="button" class="btn btn-secondary btn-sm" @click="productDialog = false">{{ t('common.cancel') }}</button><button class="btn btn-primary btn-sm" data-test="save-product">{{ t('common.save') }}</button></div></form></BaseDialog>
      <TotpStepUpDialog :controller="stepUp" />
    </main>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import TotpStepUpDialog from '@/components/auth/TotpStepUpDialog.vue'
import { useAppStore } from '@/stores'
import { formatDateTimeToMinute } from '@/utils/format'
import { isStepUpBlocked, isStepUpCancelled, stepUpBlockReason, useStepUp } from '@/composables/useStepUp'
import { membershipAPI, type MembershipAdminOverview, type MembershipAdminProduct, type MembershipFulfillmentState, type MembershipProductUpdate } from '@/api/membership'

const { t } = useI18n()
const appStore = useAppStore()
const stepUp = useStepUp()
const overview = ref<MembershipAdminOverview | null>(null)
const loading = ref(false)
const stockSKU = ref('')
const stockCodes = ref('')
const stockCost = ref(0)
const stockBusy = ref(false)
const validationSKU = ref('')
const verificationSKU = ref('')
const verificationRun = ref('')
const verificationEvidence = ref('')
const reviewID = ref('')
const reviewAction = ref<'query' | 'confirm_success' | 'confirm_not_submitted' | 'retry' | 'cancel'>('query')
const reviewEvidence = ref('')
const productDialog = ref(false)
const editingSKU = ref('')
const productForm = ref<MembershipProductUpdate>({ price_minor: 0, period_days: 30, channel: 'gpt', credential_mode: 'account_id', for_sale: false, paused: true, poll_seconds: 5, wait_seconds: 600, eta_minutes: 30 })
const coupon = ref({ code: '', discount_minor: 0, max_uses: 1, expires_at: '' })

function errorMessage(error: unknown): string { return typeof error === 'object' && error !== null && 'message' in error ? String((error as { message?: unknown }).message || t('common.unknownError')) : t('common.unknownError') }
function date(value: string | null): string { return value ? formatDateTimeToMinute(value) || '-' : '-' }
function money(minor: number): string { return new Intl.NumberFormat(undefined, { style: 'currency', currency: 'CNY' }).format(minor / 100) }
function stateClass(state: MembershipFulfillmentState): string { return state === 'succeeded' ? 'badge-success' : state === 'review_required' || state === 'failed' ? 'badge-warning' : state === 'canceled' ? 'badge-danger' : 'badge-primary' }
function idempotencyKey(): string { return typeof crypto !== 'undefined' && crypto.randomUUID ? crypto.randomUUID() : `validation-${Date.now()}-${Math.random().toString(36).slice(2)}` }

function reportError(error: unknown): void {
  if (isStepUpCancelled(error)) return
  if (isStepUpBlocked(error)) { appStore.showError(stepUpBlockReason(error) === 'STEP_UP_ADMIN_API_KEY_FORBIDDEN' ? t('stepUp.adminApiKeyForbidden') : t('stepUp.notEnabled')); return }
  appStore.showError(errorMessage(error))
}
async function load(): Promise<void> { loading.value = true; try { overview.value = await membershipAPI.getAdminOverview() } catch (error) { reportError(error) } finally { loading.value = false } }
async function runSensitive(action: () => Promise<void>): Promise<void> { try { await stepUp.run(action); await load(); appStore.showSuccess(t('common.success')) } catch (error) { reportError(error) } }
function openProduct(product: MembershipAdminProduct): void { editingSKU.value = product.sku; productForm.value = { price_minor: product.price_minor, period_days: product.period_days, channel: product.channel, credential_mode: product.credential_mode, for_sale: product.for_sale, paused: product.paused, poll_seconds: product.poll_seconds, wait_seconds: product.wait_seconds, eta_minutes: product.eta_minutes }; productDialog.value = true }
async function saveProduct(): Promise<void> { await runSensitive(async () => { await membershipAPI.updateProduct(editingSKU.value, productForm.value); productDialog.value = false }) }
async function importStock(): Promise<void> { stockBusy.value = true; const codes = stockCodes.value.split(/\r?\n/).map((line) => line.trim()).filter(Boolean); try { await stepUp.run(() => membershipAPI.importCDKs(stockSKU.value, codes, stockCost.value)); stockCodes.value = ''; stockCost.value = 0; await load(); appStore.showSuccess(t('common.success')) } catch (error) { reportError(error) } finally { stockBusy.value = false } }
async function createValidation(): Promise<void> { try { const result = await stepUp.run(() => membershipAPI.createValidationOrder({ sku: validationSKU.value, idempotency_key: idempotencyKey() })); verificationSKU.value = validationSKU.value; verificationRun.value = result.id; appStore.showSuccess(t('common.success')); await load() } catch (error) { reportError(error) } }
async function verifyProduct(): Promise<void> { await runSensitive(() => membershipAPI.verifyProduct(verificationSKU.value, verificationRun.value, verificationEvidence.value)) }
async function saveCoupon(): Promise<void> { await runSensitive(async () => { await membershipAPI.saveCoupon({ ...coupon.value, expires_at: new Date(coupon.value.expires_at).toISOString() }); coupon.value = { code: '', discount_minor: 0, max_uses: 1, expires_at: '' } }) }
async function review(): Promise<void> { await runSensitive(() => membershipAPI.reviewOrder(reviewID.value, reviewAction.value, reviewEvidence.value)) }
onMounted(load)
</script>
