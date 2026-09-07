<template>
  <section class="dashboard-recent">
    <div class="dashboard-section-header">
      <h2 class="dashboard-section-heading text-gray-900 dark:text-white">{{ t('dashboard.recentUsage') }}</h2>
      <span class="text-xs text-gray-500 dark:text-gray-400">{{ t('dashboard.last7Days') }}</span>
    </div>
    <div>
      <div v-if="loading" class="flex items-center justify-center py-12">
        <LoadingSpinner size="lg" />
      </div>
      <div v-else-if="data.length === 0" class="py-8">
        <EmptyState :title="t('dashboard.noUsageRecords')" :description="t('dashboard.startUsingApi')" />
      </div>
      <div v-else>
        <div v-for="log in data" :key="log.id" class="dashboard-usage-row">
          <div class="flex min-w-0 items-center gap-3">
            <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-teal-100 dark:bg-teal-900/30">
              <Icon name="beaker" size="md" class="text-primary-600 dark:text-primary-400" />
            </div>
            <div class="min-w-0">
              <p class="break-words text-sm font-medium text-gray-900 [overflow-wrap:anywhere] dark:text-white">{{ log.model }}</p>
              <p class="text-xs text-gray-500 dark:text-dark-400">{{ formatDateTime(log.created_at) }}</p>
            </div>
          </div>
          <div class="dashboard-usage-cost">
            <p class="text-sm font-semibold">
              <span class="text-green-600 dark:text-green-400" :title="t('dashboard.actual')">${{ formatCost(log.actual_cost) }}</span>
              <span class="font-normal text-gray-400 dark:text-gray-500" :title="t('dashboard.standard')"> / ${{ formatCost(log.total_cost) }}</span>
            </p>
            <p class="text-xs text-gray-500 dark:text-dark-400">{{ (log.input_tokens + log.output_tokens).toLocaleString() }} tokens</p>
          </div>
        </div>

        <router-link to="/usage" class="flex items-center justify-center gap-2 py-3 text-sm font-medium text-primary-600 transition-colors hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300">
          {{ t('dashboard.viewAllUsage') }}
          <Icon name="arrowRight" size="sm" />
        </router-link>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'
import { formatDateTime } from '@/utils/format'
import type { UsageLog } from '@/types'

defineProps<{
  data: UsageLog[]
  loading: boolean
}>()
const { t } = useI18n()
const formatCost = (c: number) => c.toFixed(4)
</script>

<style scoped>
.dashboard-usage-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 12px;
  min-height: 80px;
  padding: 16px 8px;
  border-bottom: 1px solid var(--dashboard-border, #dfe7e5);
  font-variant-numeric: tabular-nums;
}

.dashboard-usage-cost {
  min-width: 0;
  padding-left: 48px;
  overflow-wrap: anywhere;
}

@media (min-width: 640px) {
  .dashboard-usage-row {
    grid-template-columns: minmax(0, 1fr) minmax(0, auto);
    align-items: center;
  }

  .dashboard-usage-cost {
    padding-left: 0;
    text-align: right;
  }
}
</style>
