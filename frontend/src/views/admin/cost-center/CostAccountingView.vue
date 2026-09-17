<template>
  <div class="min-w-0 space-y-5">
    <CostLedgerStatus />
    <CostNativeTools />
    <CostTrafficSamples />
<div class="card p-4 sm:p-5 md:p-6">
      <div class="min-w-0">
        <h3 class="ledger-title">{{ tr('账期汇总', 'Period summary') }}</h3>
        <p class="ledger-desc">
          {{ tr('在同一口径下统计已记录的费用与交付，只覆盖账本里已有的记录。', 'Totals recorded costs and deliveries under one scope, covering recorded entries only.') }}
        </p>
      </div>

      <form class="mt-4 space-y-3" @submit.prevent="loadSummary">
        <div class="grid grid-cols-1 gap-x-4 gap-y-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-5">
          <div>
            <label class="ledger-label" for="ledger-start">{{ tr('账期起（UTC，含当日）', 'Start (UTC, inclusive)') }}</label>
            <input id="ledger-start" v-model="filters.start" class="input" type="date" required />
                  <CostInputHint term="time" />
          </div>
          <div>
            <label class="ledger-label" for="ledger-end">{{ tr('账期止（UTC，不含当日）', 'End (UTC, exclusive)') }}</label>
            <input id="ledger-end" v-model="filters.end" class="input" type="date" required />
                  <CostInputHint term="service_period" />
          </div>
          <div>
            <label class="ledger-label" for="ledger-currency">{{ tr('币种', 'Currency') }}</label>
            <select id="ledger-currency" v-model="filters.currency" class="input">
              <option v-for="c in currencies" :key="c">{{ c }}</option>
            </select>
                  <CostInputHint term="currency" />
          </div>
          <div>
            <label class="ledger-label" for="ledger-model">{{ tr('模型 / 工作负载', 'Model / workload') }}</label>
            <input
              id="ledger-model"
              v-model="filters.model"
              class="input"
              required
              maxlength="100"
              aria-describedby="ledger-scope-hint"
            />
                  <CostInputHint term="model" />
          </div>
          <div>
            <label class="ledger-label" for="ledger-unit">{{ tr('交付量单位', 'Delivery unit') }}</label>
            <input
              id="ledger-unit"
              v-model="filters.unit"
              class="input"
              required
              maxlength="100"
              aria-describedby="ledger-scope-hint"
            />
                  <CostInputHint term="unit" />
          </div>
        </div>
        <p id="ledger-scope-hint" class="ledger-hint">
          {{ tr('模型与单位要和交付记录里的写法完全一致，否则交付不会计入分母。', 'Model and unit must match the delivery records exactly, otherwise deliveries are excluded from the denominator.') }}
        </p>
        <div class="flex flex-wrap items-center gap-x-3 gap-y-2">
          <button class="btn btn-primary btn-sm" :disabled="busy || !connected">{{ tr('查询账期', 'Query period') }}</button>
          <span v-if="!connected" class="text-xs text-gray-500 dark:text-gray-400">
            {{ tr('账本连接后才能查询。', 'Available once the ledger is connected.') }}
          </span>
        </div>
      </form>

      <p v-if="queryError" role="alert" class="ledger-alert-error mt-4">
        <Icon name="exclamationCircle" size="sm" class="mt-px shrink-0" />
        <span>{{ queryError }}</span>
      </p>

      <div v-if="summary" class="mt-5 space-y-3">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <p class="min-w-0 text-xs text-gray-500 dark:text-gray-400">
            <time :datetime="summary.start" :title="summary.start">{{ utcLabel(summary.start) }}</time>
            →
            <time :datetime="summary.end" :title="summary.end">{{ utcLabel(summary.end) }}</time>
            · {{ summary.currency }}
            <template v-if="summaryScope"> · {{ summaryScope.model }} · {{ summaryScope.unit }}</template>
            <template v-if="summary.as_of">
              ·
              {{ tr('查询时点', 'As of') }}
              <time :datetime="summary.as_of" :title="summary.as_of">{{ utcLabel(summary.as_of) }}</time>
            </template>
          </p>
          <span class="badge badge-gray shrink-0" :title="summary.status">{{ summaryStatus(summary.status) }}</span>
        </div>

        <div class="table-container">
          <table class="table min-w-[900px]">
            <thead>
              <tr>
                <th scope="col">{{ tr('归属', 'Tier') }}</th>
                <th v-for="column in moneyColumns" :key="column.key" scope="col" class="text-right">
                  {{ tr(column.zh, column.en) }}
                  <span class="block text-[11px] font-normal text-gray-400 dark:text-dark-400">{{ summary.currency }}</span>
                </th>
                <th scope="col" class="text-right">
                  {{ tr('有效交付', 'Delivered') }}
                  <span v-if="summaryScope" class="block text-[11px] font-normal text-gray-400 dark:text-dark-400">
                    {{ summaryScope.unit }}
                  </span>
                </th>
                <th scope="col" class="text-right">
                  {{ tr('范围内单位成本', 'Scope unit cost') }}
                  <span v-if="summaryScope" class="block text-[11px] font-normal text-gray-400 dark:text-dark-400">
                    {{ summary.currency }} / {{ summaryScope.unit }}
                  </span>
                </th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in summary.rows" :key="row.tier">
                <td class="font-medium text-gray-900 dark:text-white">{{ tierName(row.tier) }}</td>
                <td class="text-right tabular-nums">{{ row.cash_paid }}</td>
                <td class="text-right tabular-nums">{{ row.period_expense }}</td>
                <td class="text-right tabular-nums">{{ row.recognized_expense }}</td>
                <td class="text-right tabular-nums">{{ row.remaining_service_value }}</td>
                <td class="text-right tabular-nums">{{ row.delivered }}</td>
                <td class="text-right tabular-nums">
                  <span v-if="row.recorded_scope_unit_cost !== null">{{ row.recorded_scope_unit_cost }}</span>
                  <span
                    v-else
                    class="text-gray-400 dark:text-dark-400"
                    :title="tr('没有可用分母，未计算；不代表 0。', 'No usable denominator, not computed; this is not zero.')"
                  >—</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <ul
          v-if="summary.warnings.length"
          class="space-y-1.5 rounded-xl border border-amber-200 bg-amber-50/70 p-3 dark:border-amber-900/50 dark:bg-amber-950/20"
        >
          <li
            v-for="code in summary.warnings"
            :key="code"
            class="flex items-start gap-2 text-xs leading-5 text-amber-900 dark:text-amber-200"
          >
            <Icon name="exclamationTriangle" size="sm" class="mt-px shrink-0" />
            <span :title="code">{{ warning(code) }}</span>
          </li>
        </ul>

        <p class="ledger-hint">
          {{ tr('已发生费用算到查询时点为止，整期费用还包含尚未发生的服务时间。单位成本只覆盖已记录范围，不是已对账的经营底价。', 'Recognized expense stops at the query time; full-period expense also covers scheduled service. Unit cost covers the recorded scope only, not a reconciled business cost floor.') }}
        </p>
      </div>

      <div v-else class="ledger-empty mt-5">
        <span class="ledger-empty-icon"><Icon name="chartBar" size="lg" /></span>
        <p class="text-sm font-medium text-gray-900 dark:text-white">
          {{ connected ? tr('还没有查询账期', 'No period queried yet') : tr('账本未连接，暂时不能汇总', 'Not connected, summary unavailable') }}
        </p>
        <p class="max-w-md text-xs leading-5 text-gray-500 dark:text-gray-400">
          {{ connected
            ? tr('填好账期、币种、模型与单位后点“查询账期”，这里会显示各档的费用与交付。', 'Fill in the period, currency, model and unit, then query — per-tier costs and deliveries appear here.')
            : tr('接口恢复后点右上角“刷新”，再重新查询。', 'Use Refresh at the top once the endpoint is reachable, then query again.') }}
        </p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import CostInputHint from './CostInputHint.vue'
import Icon from '@/components/icons/Icon.vue'
import CostNativeTools from './CostNativeTools.vue'
import CostTrafficSamples from './CostTrafficSamples.vue'
import CostLedgerStatus from './CostLedgerStatus.vue'
import { useCostLedger } from './useCostLedger'

const {
  tr, filters, connected, busy, loadSummary, summary, summaryScope,
  queryError, utcLabel, summaryStatus, moneyColumns, tierName, warning, currencies,
} = useCostLedger()
</script>

<style scoped src="./ledger.css"></style>
