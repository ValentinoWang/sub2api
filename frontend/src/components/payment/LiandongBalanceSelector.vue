<template>
  <section class="card p-6" aria-labelledby="liandong-balance-title">
    <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
      <div>
        <h2 id="liandong-balance-title" class="text-base font-semibold text-gray-900 dark:text-white">Sub2API 额度</h2>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          汇率按照
          <a
            href="https://open.er-api.com/v6/latest/USD"
            class="font-medium underline underline-offset-2 hover:text-primary-600 dark:hover:text-primary-400"
            target="_blank"
            rel="noopener noreferrer"
          >open.er-api 实时接口</a>
          计算
        </p>
      </div>
      <RouterLink class="btn btn-secondary px-3 py-2 text-sm" to="/redeem">兑换卡密</RouterLink>
    </div>

    <div class="grid grid-cols-2 gap-3 sm:grid-cols-3">
      <a
        v-for="product in products"
        :key="product.cnyAmount"
        :href="product.productUrl || undefined"
        :aria-disabled="!product.productUrl"
        :tabindex="product.productUrl ? 0 : -1"
        class="flex min-h-20 items-center justify-between rounded-lg border border-gray-200 px-4 py-3 transition-colors dark:border-dark-600"
        :class="product.productUrl
          ? 'bg-white hover:border-primary-400 hover:bg-primary-50 dark:bg-dark-800 dark:hover:border-primary-500 dark:hover:bg-dark-700'
          : 'cursor-not-allowed bg-gray-50 opacity-50 dark:bg-dark-800'"
        target="_blank"
        rel="noopener noreferrer"
      >
        <span>
          <span class="block text-lg font-bold text-gray-900 dark:text-white">¥{{ product.cnyAmount }}</span>
          <span class="block text-xs text-gray-500 dark:text-gray-400">到账 ${{ product.usdCredit.toFixed(2) }}</span>
        </span>
        <Icon name="externalLink" size="sm" class="text-gray-400" />
      </a>
    </div>
  </section>
</template>

<script setup lang="ts">
import Icon from '@/components/icons/Icon.vue'
import type { LiandongBalanceProduct } from './liandongBalanceProducts'

defineProps<{
  products: readonly LiandongBalanceProduct[]
}>()
</script>
