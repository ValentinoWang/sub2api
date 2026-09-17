<template>
  <section class="space-y-3" aria-labelledby="ledger-title">
<div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
      <div class="flex min-w-0 items-start gap-3">
        <span
          class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-primary-50 text-primary-600 ring-1 ring-inset ring-primary-100 dark:bg-primary-900/25 dark:text-primary-300 dark:ring-primary-800/50"
        >
          <Icon name="clipboard" size="md" />
        </span>
        <div class="min-w-0">
          <h2 id="ledger-title" class="text-base font-semibold text-gray-900 dark:text-white sm:text-lg">
            {{ tr('采购账本', 'Procurement ledger') }}
          </h2>
          <p class="ledger-desc">
            {{ tr('按付款与交付事实追加记录，用来核对同一账期内的实际成本。录入只写入账本，不会发生购买或充值。', 'Append payment and delivery facts to review actual cost in one period. Recording only writes to the ledger; it never purchases or recharges.') }}
          </p>
        </div>
      </div>
      <div class="flex shrink-0 items-center gap-2">
        <span
          class="badge gap-1.5"
          :class="connected ? 'badge-success' : 'badge-warning'"
          :title="health ? health.status : ''"
        >
          <span class="h-1.5 w-1.5 rounded-full" :class="connected ? 'bg-emerald-500' : 'bg-amber-500'"></span>
          {{ connected ? tr('已连接', 'Connected') : tr('未连接', 'Not connected') }}
        </span>
        <button type="button" class="btn btn-secondary btn-sm" :disabled="busy" @click="refresh">
          <Icon name="refresh" size="sm" class="ledger-refresh-icon" :class="busy ? 'animate-spin' : ''" />
          {{ tr('刷新', 'Refresh') }}
        </button>
      </div>
    </div>

    <div
      class="flex flex-col gap-2 rounded-xl border px-4 py-3 sm:flex-row sm:items-start sm:justify-between sm:gap-4"
      :class="connected
        ? 'border-primary-200 bg-primary-50/60 dark:border-primary-900/50 dark:bg-primary-950/20'
        : 'border-amber-200 bg-amber-50/70 dark:border-amber-900/50 dark:bg-amber-950/20'"
    >
      <p
        class="flex min-w-0 items-start gap-2 text-xs leading-5"
        :class="connected ? 'text-primary-900 dark:text-primary-200' : 'text-amber-900 dark:text-amber-200'"
        role="status"
      >
        <Icon :name="connected ? 'checkCircle' : 'exclamationTriangle'" size="sm" class="mt-px shrink-0" />
        <span class="min-w-0">
          <template v-if="!loaded">{{ tr('正在检查账本连接…', 'Checking ledger connection…') }}</template>
          <template v-else-if="connected">
            {{ tr('账本可以读取，查询与录入均可用。', 'The ledger is readable; querying and recording are available.') }}
          </template>
          <template v-else>
            {{ tr('账本未连接：接口读不到，查询与录入已停用。缺失的数据不会显示成 0。', 'Ledger not connected: the endpoint is unreadable, so querying and recording are disabled. Missing data is never shown as zero.') }}
          </template>
          <span v-if="loaded && !connected && events.length" class="mt-0.5 block">
            {{ tr('下面的记录来自上一次成功读取，可能已经过期。', 'Records below come from the last successful read and may be stale.') }}
          </span>
        </span>
      </p>
      <p
        v-if="checkedAt"
        class="shrink-0 text-[11px] leading-5 text-gray-500 dark:text-gray-400"
        :title="tr('本地时间', 'Local time')"
      >
        {{ tr('最近检查', 'Checked') }} {{ checkedAt }}
        <span v-if="health" class="font-mono">· {{ health.status }}</span>
      </p>
    </div>
  </section>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import { useCostLedger } from './useCostLedger'

const { tr, connected, health, busy, refresh, loaded, events, checkedAt, ensureLoaded } = useCostLedger()
onMounted(ensureLoaded)
</script>

<style scoped src="./ledger.css"></style>
