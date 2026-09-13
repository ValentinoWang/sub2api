<template>
  <section class="card mx-auto max-w-2xl overflow-hidden" data-testid="liandong-recharge">
    <div class="flex items-center justify-between gap-4 border-b border-gray-100 px-4 py-4 dark:border-dark-700 sm:px-5">
      <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('purchase.storeTitle') }}</h2>
      <p class="shrink-0 text-xs text-gray-500 dark:text-dark-400 sm:text-sm">
        {{ t('redeem.currentBalance') }}
        <span class="ml-1 font-semibold text-emerald-600 dark:text-emerald-400">${{ authStore.user?.balance?.toFixed(2) ?? '0.00' }}</span>
      </p>
    </div>
    <div class="divide-y divide-gray-100 px-4 dark:divide-dark-700 sm:px-5">
      <div class="flex gap-3 py-5 sm:gap-4">
        <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-primary-50 text-primary-600 dark:bg-primary-900/25 dark:text-primary-300" aria-hidden="true">
          <Icon name="creditCard" size="sm" />
        </div>
        <div class="min-w-0 flex-1">
          <p class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('purchase.buyStep') }}</p>
          <p class="mt-1 text-sm leading-5 text-gray-500 dark:text-dark-400">{{ t('purchase.buyDescription') }}</p>
          <div v-if="products.length" class="mt-4">
            <div class="grid grid-cols-2 gap-2 sm:grid-cols-3" role="group" :aria-label="t('purchase.selectAmount')" data-testid="liandong-products">
              <button v-for="product in products" :key="product.goods_id" type="button"
                class="rounded-xl border p-3 text-left transition-colors"
                :class="selectedProduct?.goods_id === product.goods_id ? 'border-primary-500 bg-primary-50 dark:bg-primary-900/20' : 'border-gray-200 hover:border-primary-300 dark:border-dark-600'"
                :aria-pressed="selectedProduct?.goods_id === product.goods_id"
                :data-testid="`liandong-product-${product.goods_id}`"
                @click="selectedGoodsId = product.goods_id">
                <span class="block text-sm font-semibold text-gray-900 dark:text-white">{{ t('purchase.productName', { amount: product.cny_amount }) }}</span>
                <span class="mt-1 block text-xs text-gray-500 dark:text-dark-400">{{ t('purchase.productCredit', { price: product.cny_amount.toFixed(2), credit: product.usd_credit.toFixed(2) }) }}</span>
              </button>
            </div>
            <a v-if="selectedProduct" :href="selectedProduct.external_url" target="_blank" rel="noopener noreferrer" class="btn btn-primary mt-3" data-testid="liandong-buy">
              {{ t('purchase.buySelected', { amount: selectedProduct.cny_amount }) }}
              <Icon name="externalLink" size="sm" />
            </a>
          </div>
          <p v-else-if="catalogLoading" class="mt-3 text-sm text-gray-500" role="status">{{ t('common.loading') }}</p>
          <a v-else-if="catalogUnavailable && shopUrl" :href="shopUrl" target="_blank" rel="noopener noreferrer" class="btn btn-primary mt-3" data-testid="liandong-buy">
            <span>{{ t('purchase.buyInStore') }}</span>
            <Icon name="externalLink" size="sm" />
          </a>
          <p v-else class="mt-2 text-sm text-gray-500 dark:text-dark-400" role="status">{{ t('purchase.storeUnavailable') }}</p>
        </div>
      </div>
      <form class="flex gap-3 py-5 sm:gap-4" @submit.prevent="redeem">
        <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-emerald-50 text-emerald-600 dark:bg-emerald-900/25 dark:text-emerald-300" aria-hidden="true">
          <Icon name="gift" size="sm" />
        </div>
        <div class="min-w-0 flex-1">
          <label for="purchase-redeem-code" class="block text-sm font-semibold text-gray-900 dark:text-white">{{ t('purchase.redeemStep') }}</label>
          <div class="mt-3 flex flex-col gap-2.5 sm:flex-row">
            <input id="purchase-redeem-code" v-model="code" type="text" autocomplete="off" autocapitalize="off" :spellcheck="false" :disabled="submitting" class="input min-w-0 flex-1" :placeholder="t('purchase.codePlaceholder')" />
            <button type="submit" class="btn btn-primary shrink-0 sm:min-w-28" :disabled="submitting || !code.trim()">
              {{ t(submitting ? 'redeem.redeeming' : 'purchase.confirmRedeem') }}
            </button>
          </div>
          <div v-if="result" class="mt-3 rounded-lg bg-emerald-50 p-3 text-sm text-emerald-800 dark:bg-emerald-900/20 dark:text-emerald-200" role="status">
            <p class="font-semibold">{{ t('redeem.redeemSuccess') }}</p>
            <p v-if="result.type === 'balance'">{{ t('purchase.credited', { amount: result.value.toFixed(2) }) }}</p>
            <p v-else>{{ result.message }}</p>
            <p v-if="result.new_balance !== undefined">{{ t('redeem.newBalance') }}: ${{ result.new_balance.toFixed(2) }}</p>
          </div>
          <p v-if="error" class="mt-3 text-sm text-red-600 dark:text-red-400" role="alert">{{ error }}</p>
          <p v-if="refreshWarning" class="mt-3 text-sm text-amber-700 dark:text-amber-300" role="status">{{ t('purchase.refreshWarning') }}</p>
        </div>
      </form>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import Icon from '@/components/icons/Icon.vue'
import { resolveLiandongProducts, resolveLiandongPurchaseUrl } from '@/components/payment/liandongPurchase'
import { getLiandongProducts } from '@/api/liandongBrowser'
import type { LiandongRechargeProduct } from '@/types'
import { useRedeemCode } from '@/components/payment/useRedeemCode'

const { t } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()
const { code, submitting, error, refreshWarning, result, redeem } = useRedeemCode(() => t('redeem.failedToRedeem'))
const catalog = ref<LiandongRechargeProduct[]>([])
const catalogLoading = ref(true)
const catalogUnavailable = ref(false)
const products = computed(() => appStore.cachedPublicSettings?.purchase_subscription_enabled ? resolveLiandongProducts(catalog.value) : [])
const selectedGoodsId = ref<number | null>(null)
const selectedProduct = computed(() => products.value.find(product => product.goods_id === selectedGoodsId.value) ?? products.value[0])
const shopUrl = computed(() => resolveLiandongPurchaseUrl(appStore.cachedPublicSettings))

onMounted(async () => {
  void appStore.fetchPublicSettings(true)
  try { catalog.value = await getLiandongProducts() }
  catch (error: unknown) {
    const status = (error as { status?: number }).status
    catalogUnavailable.value = status === 404
  } finally { catalogLoading.value = false }
})
</script>
