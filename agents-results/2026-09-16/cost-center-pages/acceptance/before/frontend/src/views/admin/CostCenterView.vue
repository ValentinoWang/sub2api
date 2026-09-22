<template>
  <AppLayout>
    <div class="w-full space-y-6">
      <!-- 页头 -->
      <header class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div class="flex min-w-0 items-start gap-3">
          <span
            class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-primary-50 text-primary-600 ring-1 ring-inset ring-primary-100 dark:bg-primary-900/25 dark:text-primary-300 dark:ring-primary-800/50"
          >
            <Icon name="calculator" size="md" />
          </span>
          <div class="min-w-0">
            <h1 class="text-xl font-semibold tracking-tight text-gray-900 dark:text-white sm:text-2xl">
              {{ tr('成本工作台', 'Cost workbench') }}
            </h1>
            <p class="mt-1 text-sm leading-6 text-gray-500 dark:text-gray-400">
              {{ tr('在同一工作负载、同一账期、同一币种下记录采购支出并核对单位成本。', 'Record procurement spending and review unit cost within one workload, billing period and currency.') }}
            </p>
          </div>
        </div>
        <p
          class="flex items-start gap-2 rounded-xl border border-gray-200 bg-white px-3 py-2.5 text-xs leading-5 text-gray-500 dark:border-dark-700 dark:bg-dark-800/50 dark:text-gray-400 lg:max-w-md"
        >
          <Icon name="infoCircle" size="sm" class="mt-px shrink-0 text-gray-400 dark:text-dark-400" />
          <span>{{ tr('本页只做记录与核对：不会执行购买、充值、额度重置或售价调整。', 'This page only records and reviews: no purchase, recharge, quota reset or price change happens here.') }}</span>
        </p>
      </header>

      <!-- 订阅参考资料 -->
      <section class="card p-4 sm:p-5 md:p-6" aria-labelledby="cc-catalog-title">
        <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div class="min-w-0">
            <h2 id="cc-catalog-title" class="cc-title">{{ tr('订阅参考资料', 'Subscription reference') }}</h2>
            <p class="cc-desc">
              {{ tr('公开的参考月费与购买状态，只用于对照，不是采购发票，也不是实测容量。', 'Published reference fees and purchase status, for comparison only — not invoices and not measured capacity.') }}
            </p>
          </div>
          <span
            v-if="catalog"
            class="badge badge-gray shrink-0 self-start font-mono text-[11px]"
            :title="tr('参考资料版本', 'Reference version')"
          >
            {{ catalog.version }}
          </span>
        </div>

        <div v-if="catalog" class="mt-4 space-y-4">
          <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
            <article
              v-for="plan in catalog.plans"
              :key="plan.id"
              class="rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-600 dark:bg-dark-800/40"
            >
              <div class="flex items-start justify-between gap-2">
                <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ plan.label }}</h3>
                <span class="text-xs text-gray-500 dark:text-gray-400">
                  {{ tr('宣称倍数', 'Advertised') }} {{ plan.advertised_usage_multiplier }}×
                </span>
              </div>
              <p class="mt-2 text-lg font-semibold tabular-nums text-gray-900 dark:text-white">
                USD {{ plan.reference_monthly_usd }}
                <span class="ml-1 text-xs font-normal text-gray-500 dark:text-gray-400">
                  {{ tr('/ 月（参考）', '/ mo (reference)') }}
                </span>
              </p>
              <div class="mt-3 flex flex-wrap gap-1.5">
                <span class="badge" :class="planStatusTone(plan.new_purchase_status)" :title="plan.new_purchase_status">
                  {{ tr('新购', 'New') }} · {{ planStatus(plan.new_purchase_status) }}
                </span>
                <span class="badge" :class="planStatusTone(plan.renewal_status)" :title="plan.renewal_status">
                  {{ tr('续费', 'Renewal') }} · {{ planStatus(plan.renewal_status) }}
                </span>
              </div>
              <p class="mt-3 text-xs text-gray-500 dark:text-gray-400">
                {{ tr('实测周容量', 'Measured weekly capacity') }}：{{ plan.measured_weekly_capacity ?? tr('未测得', 'not measured') }}
              </p>
            </article>
          </div>
          <p
            class="flex flex-wrap items-center gap-x-4 gap-y-1 border-t border-gray-100 pt-3 text-xs text-gray-500 dark:border-dark-700 dark:text-gray-400"
          >
            <span>
              {{ tr('核查时间', 'Checked') }}：
              <time :datetime="catalog.checked_at" :title="catalog.checked_at">{{ utcLabel(catalog.checked_at) }}</time>
            </span>
            <span>
              {{ tr('有效至', 'Valid until') }}：
              <time :datetime="catalog.valid_until" :title="catalog.valid_until">{{ utcLabel(catalog.valid_until) }}</time>
            </span>
            <span>{{ tr('超过有效期后，新购情景需要重新核查。', 'Recheck new-purchase facts after the validity window.') }}</span>
          </p>
        </div>

        <p v-else-if="catalogLoading" class="mt-4 text-sm text-gray-500 dark:text-gray-400">
          {{ tr('正在读取参考资料…', 'Loading reference…') }}
        </p>

        <div
          v-else
          class="mt-4 flex flex-col items-start gap-3 rounded-xl border border-dashed border-gray-300 bg-gray-50/60 p-5 text-left dark:border-dark-600 dark:bg-dark-900/30 sm:flex-row sm:items-center"
        >
          <span
            class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-white text-gray-400 ring-1 ring-inset ring-gray-200 dark:bg-dark-800 dark:text-dark-400 dark:ring-dark-600"
          >
            <Icon name="document" size="md" />
          </span>
          <div class="min-w-0 flex-1">
            <p class="text-sm font-medium text-gray-900 dark:text-white">
              {{ tr('参考资料暂时读不到', 'Reference is not readable') }}
            </p>
            <p class="mt-1 text-xs leading-5 text-gray-500 dark:text-gray-400">
              {{ catalogError }}
            </p>
          </div>
          <button type="button" class="btn btn-secondary btn-sm shrink-0" :disabled="catalogLoading" @click="loadCatalog">
            <Icon name="refresh" size="sm" />
            {{ tr('重新读取', 'Retry') }}
          </button>
        </div>
      </section>

      <!-- 采购账本 -->
      <CostLedgerPanel />

      <!-- 三档情景比较 -->
      <section class="card overflow-hidden" aria-labelledby="cc-compare-title">
        <div
          class="flex flex-col gap-3 border-b border-gray-100 p-4 dark:border-dark-700 sm:flex-row sm:items-start sm:justify-between sm:p-5 md:p-6"
        >
          <div class="min-w-0">
            <h2 id="cc-compare-title" class="cc-title">{{ tr('三档情景比较', 'Three-tier scenarios') }}</h2>
            <p class="cc-desc">
              {{ tr('用各自独立的假设或实测数据推演 Plus、Pro 5x、Pro 20x 的单位成本。只做推演，不写入账本。', 'Compare unit cost across Plus, Pro 5x and Pro 20x from independent inputs. Scenario only — nothing is written to the ledger.') }}
            </p>
          </div>
          <button
            type="button"
            class="btn btn-secondary btn-sm shrink-0 self-start"
            :aria-expanded="compareOpen"
            aria-controls="cc-compare-body"
            @click="compareOpen = !compareOpen"
          >
            <Icon name="chevronRight" size="sm" class="cc-chevron" :class="compareOpen ? 'rotate-90' : ''" />
            {{ compareOpen ? tr('收起', 'Collapse') : tr('展开', 'Expand') }}
          </button>
        </div>

        <div v-show="compareOpen" id="cc-compare-body" class="space-y-6 p-4 sm:p-5 md:p-6">
          <form class="space-y-6" @submit.prevent="calculate">
            <fieldset class="min-w-0">
              <legend class="text-sm font-semibold text-gray-900 dark:text-white">
                {{ tr('口径与账期', 'Scope and period') }}
              </legend>
              <p class="cc-desc">
                {{ tr('三档共用同一口径；任何一项改动后都需要重新计算。', 'All tiers share this scope. Recalculate after any change.') }}
              </p>
              <div class="mt-3 grid grid-cols-1 gap-x-4 gap-y-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
                <div>
                  <label class="cc-label" for="cc-purpose">{{ tr('用途', 'Purpose') }}</label>
                  <select id="cc-purpose" v-model="input.purpose" class="input">
                    <option value="existing">{{ tr('已有账号成本', 'Existing accounts') }}</option>
                    <option value="new_purchase">{{ tr('新购可得性情景', 'New-purchase scenario') }}</option>
                  </select>
                </div>
                <div>
                  <label class="cc-label" for="cc-currency">{{ tr('币种', 'Currency') }}</label>
                  <select id="cc-currency" v-model="input.currency" class="input">
                    <option v-for="c in currencies" :key="c">{{ c }}</option>
                  </select>
                </div>
                <div>
                  <label class="cc-label" for="cc-demand">{{ tr('本期合格交付目标', 'Qualified period demand') }}</label>
                  <input
                    id="cc-demand"
                    v-model="input.demand"
                    class="input tabular-nums"
                    inputmode="decimal"
                    required
                    pattern="[0-9]+([.][0-9]+)?"
                    aria-describedby="cc-demand-hint"
                  />
                  <p id="cc-demand-hint" class="cc-hint">{{ tr('只统计达标的交付量。', 'Counts qualified delivery only.') }}</p>
                </div>
                <div>
                  <label class="cc-label" for="cc-model">{{ tr('模型 / 工作负载版本', 'Model / workload version') }}</label>
                  <input id="cc-model" v-model="input.model" class="input" required maxlength="100" />
                </div>
                <div>
                  <label class="cc-label" for="cc-unit">{{ tr('计量单位', 'Unit') }}</label>
                  <input
                    id="cc-unit"
                    v-model="input.unit"
                    class="input"
                    required
                    maxlength="100"
                    aria-describedby="cc-unit-hint"
                  />
                  <p id="cc-unit-hint" class="cc-hint">{{ tr('三档必须使用同一单位。', 'All tiers must share one unit.') }}</p>
                </div>
                <div>
                  <label class="cc-label" for="cc-asof">{{ tr('情景时点（UTC）', 'Scenario time (UTC)') }}</label>
                  <input id="cc-asof" v-model="asOf" class="input" type="datetime-local" required />
                </div>
                <div>
                  <label class="cc-label" for="cc-start">{{ tr('账期起（UTC，含当日）', 'Start (UTC, inclusive)') }}</label>
                  <input id="cc-start" v-model="periodStart" class="input" type="date" required />
                </div>
                <div>
                  <label class="cc-label" for="cc-end">{{ tr('账期止（UTC，不含当日）', 'End (UTC, exclusive)') }}</label>
                  <input id="cc-end" v-model="periodEnd" class="input" type="date" required />
                </div>
              </div>
            </fieldset>

            <fieldset class="min-w-0">
              <legend class="text-sm font-semibold text-gray-900 dark:text-white">{{ tr('各档输入', 'Tier inputs') }}</legend>
              <p class="cc-desc">
                {{ tr('费用与容量来自各档自己的记录或假设，不要用参考月费乘以宣称倍数推算。', 'Use each tier own records or assumptions — do not multiply a reference fee by an advertised multiplier.') }}
              </p>
              <div class="mt-3 grid grid-cols-1 gap-4 xl:grid-cols-3">
                <div
                  v-for="row in input.rows"
                  :key="row.tier"
                  class="rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-600 dark:bg-dark-800/40"
                >
                  <div class="flex items-start justify-between gap-2">
                    <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ names[row.tier] }}</h3>
                    <span class="text-xs tabular-nums text-gray-500 dark:text-gray-400">
                      <template v-if="referenceFee(row.tier)">
                        USD {{ referenceFee(row.tier) }} {{ tr('/ 月（参考）', '/ mo (ref)') }}
                      </template>
                      <template v-else>{{ tr('参考月费未读取', 'reference unavailable') }}</template>
                    </span>
                  </div>
                  <p
                    v-if="row.tier === 'pro20x'"
                    class="mt-2 rounded-lg bg-amber-50 px-2.5 py-1.5 text-xs leading-5 text-amber-800 dark:bg-amber-900/20 dark:text-amber-200"
                  >
                    {{ tr('存量续费与新购状态不同，采购前需要重新核验。', 'Renewal and new-purchase status differ; recheck before buying.') }}
                  </p>
                  <div class="mt-3 space-y-3">
                    <div>
                      <label class="cc-label" :for="`cc-${row.tier}-cost`">
                        {{ tr('本期实际 / 假设总费用', 'Actual / assumed total cost') }}
                      </label>
                      <input
                        :id="`cc-${row.tier}-cost`"
                        v-model="row.period_cost"
                        class="input tabular-nums"
                        inputmode="decimal"
                      />
                    </div>
                    <div>
                      <label class="cc-label" :for="`cc-${row.tier}-capacity`">
                        {{ tr('独立测得 / 假设期内容量', 'Independent period capacity') }}
                      </label>
                      <input
                        :id="`cc-${row.tier}-capacity`"
                        v-model="row.period_capacity"
                        class="input tabular-nums"
                        inputmode="decimal"
                      />
                    </div>
                    <div>
                      <label class="cc-label" :for="`cc-${row.tier}-useful`">{{ tr('有用比例 0–1', 'Useful fraction 0–1') }}</label>
                      <input
                        :id="`cc-${row.tier}-useful`"
                        v-model="row.useful_fraction"
                        class="input tabular-nums"
                        inputmode="decimal"
                      />
                    </div>
                    <div>
                      <label class="cc-label" :for="`cc-${row.tier}-evidence`">{{ tr('数据类型', 'Evidence') }}</label>
                      <select :id="`cc-${row.tier}-evidence`" v-model="row.evidence" class="input">
                        <option value="assumption">{{ tr('假设', 'Assumption') }}</option>
                        <option value="observed">{{ tr('自身观察', 'Own observation') }}</option>
                        <option value="synthetic">{{ tr('合成测试', 'Synthetic') }}</option>
                      </select>
                    </div>
                  </div>
                  <div class="mt-3 flex items-start gap-2 border-t border-gray-100 pt-3 dark:border-dark-700">
                    <input
                      :id="`cc-${row.tier}-verified`"
                      v-model="row.workload_verified"
                      type="checkbox"
                      class="mt-0.5 h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500 dark:border-dark-500 dark:bg-dark-800"
                    />
                    <label :for="`cc-${row.tier}-verified`" class="text-xs leading-5 text-gray-600 dark:text-gray-300">
                      {{ tr('已确认该档支持同一工作负载', 'Same workload verified for this tier') }}
                    </label>
                  </div>
                </div>
              </div>
            </fieldset>

            <div class="flex flex-wrap items-center gap-x-3 gap-y-2 border-t border-gray-100 pt-4 dark:border-dark-700">
              <button type="submit" class="btn btn-primary btn-md" :disabled="busy">
                {{ busy ? tr('计算中…', 'Calculating…') : tr('计算比较', 'Compare tiers') }}
              </button>
              <span class="text-xs text-gray-500 dark:text-gray-400">
                {{ tr('只计算并展示结果，不写入账本、不触发采购。', 'Calculates and shows results only; no ledger write, no purchase.') }}
              </span>
            </div>
          </form>

          <p
            v-if="compareError"
            role="alert"
            class="flex items-start gap-2 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm leading-6 text-red-700 dark:border-red-900/50 dark:bg-red-950/30 dark:text-red-300"
          >
            <Icon name="exclamationCircle" size="sm" class="mt-1 shrink-0" />
            <span>{{ compareError }}</span>
          </p>

          <section
            v-if="result"
            aria-live="polite"
            class="space-y-3 rounded-xl border border-gray-200 bg-gray-50/70 p-4 dark:border-dark-600 dark:bg-dark-900/30"
          >
            <div class="flex flex-wrap items-center justify-between gap-2">
              <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ tr('比较结果', 'Result') }}</h3>
              <span class="badge badge-gray" :title="result.status">{{ resultStatus(result.status) }}</span>
            </div>
            <div class="flex flex-wrap items-center gap-2 text-sm text-gray-600 dark:text-gray-300">
              <span>{{ tr('先满足需求，再比成本：', 'Meet demand first, then compare cost:') }}</span>
              <template v-if="result.lowest_cost_feasible_tiers.length">
                <span v-for="tier in result.lowest_cost_feasible_tiers" :key="tier" class="badge badge-primary">
                  {{ names[tier] }}
                </span>
              </template>
              <span v-else class="text-amber-700 dark:text-amber-300">
                {{ tr('没有档位同时满足需求与全部前提。', 'No tier meets demand and every precondition.') }}
              </span>
            </div>
            <div class="table-container bg-white dark:bg-dark-800">
              <table class="table min-w-[760px]">
                <thead>
                  <tr>
                    <th scope="col">{{ tr('档位', 'Tier') }}</th>
                    <th scope="col" class="text-right">
                      {{ tr('有效容量', 'Useful capacity') }}
                      <span class="block text-[11px] font-normal text-gray-400 dark:text-dark-400">{{ result.unit }}</span>
                    </th>
                    <th scope="col" class="text-right">
                      {{ tr('未满足需求', 'Unmet demand') }}
                      <span class="block text-[11px] font-normal text-gray-400 dark:text-dark-400">{{ result.unit }}</span>
                    </th>
                    <th scope="col" class="text-right">
                      {{ tr('单位成本', 'Unit cost') }}
                      <span class="block text-[11px] font-normal text-gray-400 dark:text-dark-400">
                        {{ result.currency }} / {{ result.unit }}
                      </span>
                    </th>
                    <th scope="col">{{ tr('限制', 'Constraints') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="row in result.rows" :key="row.tier">
                    <td>
                      <span class="font-medium text-gray-900 dark:text-white">{{ names[row.tier] }}</span>
                      <span class="mt-0.5 block text-[11px] text-gray-500 dark:text-gray-400">
                        {{ evidenceLabel(row.evidence) }}
                      </span>
                    </td>
                    <td class="text-right tabular-nums">
                      <span v-if="row.effective_capacity !== null">{{ row.effective_capacity }}</span>
                      <span v-else class="text-gray-400 dark:text-dark-400" :title="tr('缺少输入，未计算', 'Not computed, input missing')">—</span>
                    </td>
                    <td class="text-right tabular-nums">
                      <span v-if="row.unmet !== null">{{ row.unmet }}</span>
                      <span v-else class="text-gray-400 dark:text-dark-400" :title="tr('缺少输入，未计算', 'Not computed, input missing')">—</span>
                    </td>
                    <td class="text-right tabular-nums">
                      <span v-if="row.unit_cost !== null">{{ row.unit_cost }}</span>
                      <span v-else class="text-gray-400 dark:text-dark-400" :title="tr('缺少输入，未计算', 'Not computed, input missing')">—</span>
                    </td>
                    <td>
                      <div v-if="row.reasons.length" class="flex flex-wrap gap-1">
                        <span v-for="code in row.reasons" :key="code" class="badge badge-warning" :title="code">
                          {{ reason(code) }}
                        </span>
                      </div>
                      <span v-else class="text-gray-500 dark:text-gray-400">
                        {{ tr('满足所填条件', 'Meets declared inputs') }}
                      </span>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
            <div class="border-t border-gray-200 pt-3 dark:border-dark-600">
              <p class="text-xs font-medium text-gray-600 dark:text-gray-300">{{ tr('结果边界', 'Result boundaries') }}</p>
              <ul class="mt-1.5 space-y-1 text-xs leading-5 text-gray-500 dark:text-gray-400">
                <li class="flex gap-1.5">
                  <span aria-hidden="true">·</span>
                  <span>{{ tr('不能据此判断短时窗口、并发、延迟和实际授权是否可用，也不能据此直接开售。', 'This does not establish short-window, concurrency, latency or authorization feasibility, and does not authorize selling.') }}</span>
                </li>
                <li v-for="line in result.boundaries" :key="line" class="flex gap-1.5">
                  <span aria-hidden="true">·</span>
                  <span>{{ line }}</span>
                </li>
              </ul>
            </div>
          </section>
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import CostLedgerPanel from './components/CostLedgerPanel.vue'
import { compareCostTiers, getCostCatalog } from '@/api/admin/cost-center'
import type {
  CostCatalog,
  CostComparisonInput,
  CostComparisonResult,
  CostEvidence,
  CostTier
} from '@/api/admin/cost-center'
const { locale } = useI18n()
const zh = computed(() => locale.value.startsWith('zh'))
const tr = (cn: string, en: string) => (zh.value ? cn : en)
const names: Record<CostTier, string> = { plus: 'Plus', pro5x: 'Pro 5x', pro20x: 'Pro 20x' }
const currencies = ['USD', 'CNY', 'HKD', 'SGD', 'EUR'] as const
const today = new Date().toISOString().slice(0, 10)
const asOf = ref(new Date().toISOString().slice(0, 16))
const periodStart = ref(today.slice(0, 8) + '01')
const next = new Date(today + 'T00:00:00Z'); next.setUTCMonth(next.getUTCMonth() + 1, 1)
const periodEnd = ref(next.toISOString().slice(0, 10))
const input = reactive<CostComparisonInput>({ currency: 'USD', unit: '', model: '', period_start: '', period_end: '', as_of: '', purpose: 'existing', demand: '', rows: (['plus','pro5x','pro20x'] as CostTier[]).map(tier => ({ tier, period_cost: null, period_capacity: null, useful_fraction: null, workload_verified: false, evidence: 'assumption' as const })) })
const catalog = ref<CostCatalog | null>(null)
const result = ref<CostComparisonResult | null>(null)
const busy = ref(false)
const catalogLoading = ref(true)
const catalogError = ref('')
const compareError = ref('')
const compareOpen = ref(false)
const reasons: Record<string, string> = { MISSING_COST_OR_CAPACITY: '费用或容量缺失', WORKLOAD_NOT_VERIFIED: '未确认工作负载可用', NEW_PURCHASE_PAUSED: '参考资料显示暂停新购', ACQUISITION_FACTS_REQUIRE_REFRESH: '采购资料已过期，需复核', INSUFFICIENT_USEFUL_CAPACITY: '有效容量不足', NO_DEMAND: '没有需求' }
function reason(code: string): string { return zh.value ? reasons[code] ?? code : code }
const planStatusText: Record<string, [string, string]> = {
  available_reference: ['参考资料显示可购', 'Available (reference)'],
  paused: ['暂停新购', 'Paused'],
  existing_subscribers_only: ['仅限存量续费', 'Existing subscribers only']
}
function planStatus(code: string): string { const text = planStatusText[code]; return text ? tr(text[0], text[1]) : code }
function planStatusTone(code: string): string { return code === 'paused' || code === 'existing_subscribers_only' ? 'badge-warning' : 'badge-gray' }
const evidenceText: Record<CostEvidence, [string, string]> = {
  assumption: ['假设', 'Assumption'],
  observed: ['自身观察', 'Own observation'],
  synthetic: ['合成测试', 'Synthetic']
}
function evidenceLabel(code: CostEvidence): string { const text = evidenceText[code]; return text ? tr(text[0], text[1]) : code }
function resultStatus(code: string): string { return code === 'CONDITIONAL_COMPARISON' ? tr('有条件比较', 'Conditional comparison') : code }
const planMap = computed(() => new Map((catalog.value?.plans ?? []).map(plan => [plan.id, plan] as const)))
function referenceFee(tier: CostTier): string | null { return planMap.value.get(tier)?.reference_monthly_usd ?? null }
// UTC 展示，原始值保留在 title/datetime 中。
function utcLabel(value: string): string {
  const ms = Date.parse(value)
  if (!Number.isFinite(ms)) return value
  const iso = new Date(ms).toISOString()
  return `${iso.slice(0, 10)} ${iso.slice(11, 16)} UTC`
}
async function loadCatalog(): Promise<void> {
  catalogLoading.value = true; catalogError.value = ''
  try { catalog.value = await getCostCatalog() }
  catch { catalog.value = null; catalogError.value = tr('原生成本接口尚未接入或当前不可达。页面不会用默认值代替真实资料。', 'The native cost endpoint is not wired or not reachable. Defaults are never shown in place of real facts.') }
  finally { catalogLoading.value = false }
}
onMounted(loadCatalog)
async function calculate() {
  busy.value = true; compareError.value = ''; result.value = null
  try {
    const rows = input.rows.map(r => ({ ...r, period_cost: r.period_cost?.trim() || null, period_capacity: r.period_capacity?.trim() || null, useful_fraction: r.useful_fraction?.trim() || null }))
    result.value = await compareCostTiers({ ...input, rows, period_start: periodStart.value + 'T00:00:00Z', period_end: periodEnd.value + 'T00:00:00Z', as_of: asOf.value + ':00Z' })
  } catch { compareError.value = tr('没有算出结果。请核对输入内容和接口是否可用，然后重试。', 'No result produced. Check the inputs and endpoint availability, then retry.') }
  finally { busy.value = false }
}
</script>

<style scoped>
.cc-title {
  @apply text-base font-semibold text-gray-900 dark:text-white;
}

.cc-desc {
  @apply mt-1 text-xs leading-5 text-gray-500 dark:text-gray-400 sm:text-sm sm:leading-6;
}

.cc-label {
  @apply mb-1.5 block text-xs font-medium text-gray-600 dark:text-gray-400;
}

.cc-hint {
  @apply mt-1 text-xs leading-5 text-gray-500 dark:text-dark-400;
}

.cc-chevron {
  @apply transition-transform duration-200;
}

@media (prefers-reduced-motion: reduce) {
  .cc-chevron {
    transition: none;
  }
}
</style>
