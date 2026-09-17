<template>
  <section class="card space-y-5 p-4 sm:p-5">
    <div>
      <h3 class="text-base font-semibold">{{ tr('预充值与账号对账', 'Prepaid credit and account reconciliation') }}</h3>
      <p class="mt-1 text-xs leading-5 text-gray-500">{{ tr('录入付款和核销事实，或读取网关内的账号用量。读取用量不会购买、充值或重置额度。', 'Record payment and consumption facts, or read gateway usage. Reading does not buy, recharge, or reset quota.') }}</p>
    </div>
    <label class="block text-sm">
      {{ tr('办理事项', 'Operation') }}
      <select v-model="operation" class="input mt-1" @change="clearFeedback">
        <option value="credit_lot">{{ tr('录入预充值批次', 'Record prepaid lot') }}</option>
        <option value="credit_use">{{ tr('按先入先出核销额度', 'Consume credit using FIFO') }}</option>
        <option value="account_interval">{{ tr('绑定账号与档位期间', 'Bind an account tier interval') }}</option>
        <option value="reconcile">{{ tr('读取账号用量', 'Read account usage') }}</option>
        <option value="analysis">{{ tr('导入采样与推演资料', 'Import samples and scenarios') }}</option>
      </select>
    </label>
    <form v-if="operation === 'credit_lot' || operation === 'credit_use'" class="space-y-4" @submit.prevent="saveCredit">
      <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        <label v-for="field in creditFields" :key="field.key" class="text-xs">{{ tr(field.zh, field.en) }}<input v-model="credit[field.key]" class="input mt-1" required :type="field.type || 'text'" /><span class="mt-1 block text-xs leading-5 text-gray-500">{{ creditHint(field.key) }}</span></label>
        <label class="text-xs">{{ tr('币种', 'Currency') }}<select v-model="credit.currency" class="input mt-1"><option v-for="c in currencies" :key="c">{{ c }}</option></select></label>
        <label v-if="operation === 'credit_use'" class="text-xs">{{ tr('档位', 'Tier') }}<select v-model="creditTier" class="input mt-1"><option v-for="t in tiers" :key="t" :value="t">{{ tierName(t) }}</option></select></label>
      </div>
      <p class="text-xs leading-5 text-gray-500">{{ tr('时间使用 UTC。预充值不是期间订阅费；核销只用未过期批次的实际现金价格。赠送额度的现金金额填 0。', 'Times use UTC. Prepayment is separate from subscriptions; FIFO uses actual prices of unexpired lots. Enter zero cash for gifted credits.') }}</p>
      <button class="btn btn-primary btn-sm" :disabled="working || !connected">{{ tr('保存记账事实', 'Save ledger facts') }}</button>
    </form>
    <form v-else-if="operation === 'account_interval'" class="space-y-4" @submit.prevent="saveBinding">
      <button type="button" class="btn btn-secondary btn-sm" :disabled="working" @click="loadAccounts">{{ tr('读取可绑定账号', 'Read available accounts') }}</button>
      <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        <label class="text-xs">{{ tr('明确选择账号', 'Choose an account') }}<select v-model.number="binding.account_id" class="input mt-1" required><option :value="0" disabled>{{ tr('请选择', 'Select') }}</option><option v-for="a in accounts" :key="a.id" :value="a.id">#{{ a.id }} · {{ a.platform }} · {{ a.status }}</option></select></label>
        <label class="text-xs">{{ tr('资产引用', 'Asset reference') }}<input v-model="binding.asset" class="input mt-1" required /></label>
        <label class="text-xs">{{ tr('档位', 'Tier') }}<select v-model="binding.tier" class="input mt-1"><option v-for="t in tiers" :key="t" :value="t">{{ tierName(t) }}</option></select></label>
        <label class="text-xs">{{ tr('生效时间（UTC）', 'Effective from (UTC)') }}<input v-model="bindingStart" class="input mt-1" type="datetime-local" required /></label>
        <label class="text-xs">{{ tr('结束时间（UTC，不含）', 'Until (UTC, exclusive)') }}<input v-model="bindingEnd" class="input mt-1" type="datetime-local" required /></label>
        <label class="text-xs">{{ tr('凭据引用', 'Evidence reference') }}<input v-model="binding.evidence_ref" class="input mt-1" required /></label>
      </div>
      <p class="text-xs text-gray-500">{{ tr('只记录归属，不修改账号套餐。升级前后分别记录，期间不能重叠。', 'Records attribution without changing the subscription. Record separate non-overlapping intervals before and after an upgrade.') }}</p>
      <button class="btn btn-primary btn-sm" :disabled="working || !connected || binding.account_id <= 0">{{ tr('保存归属期间', 'Save account interval') }}</button>
    </form>
    <form v-else-if="operation === 'reconcile'" class="space-y-4" @submit.prevent="readUsage">
      <div class="grid gap-3 sm:grid-cols-3">
        <label class="text-xs">{{ tr('开始日期（UTC）', 'Start (UTC)') }}<input v-model="usageStart" type="date" class="input mt-1" required /></label>
        <label class="text-xs">{{ tr('结束日期（UTC，不含）', 'End (UTC, exclusive)') }}<input v-model="usageEnd" type="date" class="input mt-1" required /></label>
        <label class="text-xs">{{ tr('模型', 'Model') }}<input v-model="usageModel" class="input mt-1" placeholder="留空读取全部模型" /></label>
      </div>
      <button class="btn btn-primary btn-sm" :disabled="working">{{ tr('读取并核对来源', 'Read and verify source') }}</button>
      <p class="text-xs text-gray-500">{{ tr('系统按来源账号直接读取用量，不需要先猜档位。未确认服务期间的记录会标为待核对；来源读取只读，核对快照保存到成本库，不生成采购费用。', 'Read usage by source account without guessing a tier. Unknown tier intervals remain unclassified. Source access is read-only; a verified snapshot is stored separately from purchases.') }}</p>
      <p v-if="reconciliation?.source_latest_usage_at" class="text-xs text-gray-500">{{ tr('来源最新用量时间', 'Latest source usage') }}：{{ new Date(reconciliation.source_latest_usage_at).toLocaleString() }} · {{ tr('网关以外用量和历史保留范围尚未验证', 'Direct upstream usage and retention coverage are unverified') }}</p>
      <div v-if="reconciliation" class="table-container">
        <table class="table min-w-[620px]"><thead><tr><th>{{ tr('资产 / 档位', 'Asset / tier') }}</th><th>{{ tr('请求数', 'Requests') }}</th><th>{{ tr('输入量', 'Input tokens') }}</th><th>{{ tr('输出量', 'Output tokens') }}</th><th>{{ tr('参考计费金额', 'Reference charge') }}</th><th>{{ tr('与交付记录核对', 'Reconciliation') }}</th></tr></thead><tbody><tr v-for="row in reconciliation.rows" :key="row.asset + row.start"><td>{{ row.asset }} · {{ row.tier === 'unassigned' ? tr('档位待核对', 'Tier unconfirmed') : tierName(row.tier) }}</td><td>{{ row.requests }}</td><td>{{ row.input_tokens }}</td><td>{{ row.output_tokens }}</td><td>{{ row.reference_cost }}</td><td>{{ reconciliationStatus(row.reconciliation_status) }}<span v-if="row.request_difference !== null"> · {{ row.request_difference }}</span></td></tr></tbody></table>
        <p v-if="!reconciliation.rows.length" class="p-3 text-sm">{{ tr('所选期间没有读取到用量记录。', 'No usage records in this period.') }}</p>
      </div>
    </form>
    <form v-else class="space-y-4" @submit.prevent="runAnalysis">
      <label class="block text-xs">{{ tr('资料类型', 'Data type') }}<select v-model="analysisKind" class="input mt-1" @change="clearAnalysis"><option value="traffic">{{ tr('网卡流量与费率', 'Network traffic and tariff') }}</option><option value="replay">{{ tr('双限额窗口与需求事件', 'Quota windows and demand') }}</option><option value="forecast">{{ tr('完整观察与预测条件', 'Observed exposure and forecast inputs') }}</option></select></label>
      <label class="block text-xs">{{ tr('选择资料文件（JSON）', 'Select data file (JSON)') }}<input :key="analysisKind" type="file" accept="application/json,.json" class="input mt-1" required @change="readAnalysisFile" /></label>
      <p class="text-xs leading-5 text-gray-500">{{ tr('按导入资料计算，不补默认观测。流量需要同一网卡的两次 vnStat 累计快照与明确费率；回放需要完整窗口和事件；预测需要自己的完整曝光期及逐次净增。', 'Computes from supplied data without filling missing observations. Traffic needs two vnStat snapshots and an explicit tariff; replay needs complete windows and events; forecasting needs complete own exposure and measured gains.') }}</p>
      <div class="flex flex-wrap gap-2"><button class="btn btn-primary btn-sm" :disabled="working || !analysisInput">{{ tr('计算并检查资料', 'Validate and calculate') }}</button><button v-if="analysisResult" type="button" class="btn btn-secondary btn-sm" :disabled="working || !connected" @click="saveAnalysis">{{ tr('保存这份资料', 'Save these inputs') }}</button></div>
      <dl v-if="analysisResult" class="grid gap-3 rounded-xl bg-gray-50 p-4 text-sm dark:bg-dark-900 sm:grid-cols-2">
        <div v-for="row in analysisDisplay" :key="row.label"><dt class="text-xs text-gray-500">{{ row.label }}</dt><dd class="mt-1 break-all tabular-nums">{{ row.value }}</dd></div>
      </dl>
    </form>
    <p v-if="error" role="alert" class="text-sm text-red-600">{{ error }}</p>
    <p v-if="message" role="status" class="text-sm text-emerald-600">{{ message }}</p>
    <div v-if="summary?.prepaid" class="space-y-3 border-t border-gray-200 pt-4 dark:border-dark-700">
      <h4 class="font-medium">{{ tr('当前查询账期的预充值', 'Prepaid credit in the queried period') }}</h4>
      <p class="text-sm">{{ tr('充值现金', 'Prepaid cash') }} {{ summary.prepaid.cash_paid }} {{ summary.currency }}</p>
      <div class="table-container"><table class="table min-w-[660px]"><thead><tr><th>{{ tr('额度池 / 批次', 'Pool / lot') }}</th><th>{{ tr('可用额度', 'Available credit') }}</th><th>{{ tr('剩余现金价值', 'Remaining cash value') }}</th><th>{{ tr('过期额度价值', 'Expired value') }}</th></tr></thead><tbody><tr v-for="b in summary.prepaid.balances" :key="b.event_id"><td>{{ b.pool }} · {{ b.event_id }}</td><td>{{ b.remaining_quantity }} {{ b.unit }}</td><td>{{ b.remaining_value }}</td><td>{{ b.expired_value }}</td></tr></tbody></table></div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { costHint } from './costFieldHelp'
import { useCostLedger } from './useCostLedger'
import { analyzeNativeCost, appendNativeCost, getCostSourceAccounts, syncCostSource } from '@/api/admin/cost-center-native'
import type { AccountIntervalInput, AnalysisKind, NativeCommand, Reconciliation, SourceAccount } from '@/api/admin/cost-center-native'
import type { CostTier } from '@/api/admin/cost-center'
const { tr, tiers, tierName, currencies, connected, refresh, summary } = useCostLedger()
const operation = ref('credit_lot'), working = ref(false), error = ref(''), message = ref('')
const credit = reactive<Record<string, string>>({ reference: '', pool: '', unit: '', currency: 'USD', amount: '', quantity: '', paid_at: '', expires_at: '', occurred_at: '', model: '' })
const creditTier = ref<CostTier>('plus')
const creditFields = computed(() => [
  { key: 'reference', zh: '业务编号', en: 'Business reference' }, { key: 'pool', zh: '额度池', en: 'Credit pool' }, { key: 'unit', zh: '额度单位', en: 'Credit unit' }, { key: 'quantity', zh: '额度数量', en: 'Credit quantity' },
  ...(operation.value === 'credit_lot' ? [{ key: 'amount', zh: '实际现金金额', en: 'Actual cash paid' }, { key: 'paid_at', zh: '付款时间（UTC）', en: 'Paid at (UTC)', type: 'datetime-local' }, { key: 'expires_at', zh: '到期时间（UTC）', en: 'Expires at (UTC)', type: 'datetime-local' }] : [{ key: 'model', zh: '模型', en: 'Model' }, { key: 'occurred_at', zh: '消耗时间（UTC）', en: 'Consumed at (UTC)', type: 'datetime-local' }])
])
function creditHint(key: string) { const map: Record<string, string> = { quantity: 'credit_quantity', paid_at: 'time', expires_at: 'credit_quantity', occurred_at: 'time' }; return costHint(map[key] ?? key) }
const binding = reactive<AccountIntervalInput>({ asset: '', account_id: 0, tier: 'plus', start: '', end: '', evidence_ref: '' })
const bindingStart = ref(''), bindingEnd = ref(''), accounts = ref<SourceAccount[]>([])
const currentDay = new Date().toISOString().slice(0, 10)
const usageStart = ref(currentDay.slice(0, 8) + '01'), usageEnd = ref(new Date(Date.now() + 86400000).toISOString().slice(0,10)), usageModel = ref(''), reconciliation = ref<Reconciliation | null>(null)
const analysisKind = ref<AnalysisKind>('traffic'), analysisInput = ref<Record<string, unknown> | null>(null), analysisResult = ref<Record<string, unknown> | null>(null)
const keys = new Map<string, string>()
const utc = (value: string) => value.length === 16 ? value + ':00Z' : value + 'Z'
function clearFeedback() { error.value = ''; message.value = '' }
function clearAnalysis() { clearFeedback(); analysisInput.value = null; analysisResult.value = null }
async function act(fn: () => Promise<void>) { working.value = true; clearFeedback(); try { await fn() } catch { error.value = tr('操作未完成。请核对资料、时间范围、剩余额度和连接后重试。缺失数据不会用 0 代替。', 'Not completed. Check inputs, dates, remaining credit and connectivity, then retry. Missing values are never replaced by zero.') } finally { working.value = false } }
async function save(command: NativeCommand) { const payload = JSON.stringify(command); let key = keys.get(payload); if (!key) { key = crypto.randomUUID(); keys.set(payload, key) } const result = await appendNativeCost(command, key); message.value = result.replayed ? tr('已读回原记录，没有重复记账。', 'Original record returned without duplicate booking.') : tr('资料已保存。', 'Records saved.'); summary.value = null; await refresh() }
async function saveCredit() { await act(async () => { const common = { reference: credit.reference, pool: credit.pool, unit: credit.unit, currency: credit.currency, quantity: credit.quantity, evidence: 'manual' }; await save(operation.value === 'credit_lot' ? { kind: 'credit_lot', credit_lot: { ...common, amount: credit.amount, paid_at: utc(credit.paid_at), expires_at: utc(credit.expires_at) } } : { kind: 'credit_use', credit_use: { ...common, tier: creditTier.value, model: credit.model, occurred_at: utc(credit.occurred_at) } }) }) }
async function loadAccounts() { await act(async () => { accounts.value = await getCostSourceAccounts(); message.value = tr(`已只读获取 ${accounts.value.length} 个账号，请明确选择。`, `Read ${accounts.value.length} accounts; choose explicitly.`) }) }
async function saveBinding() { await act(async () => { await save({ kind: 'account_interval', account_interval: { ...binding, start: utc(bindingStart.value), end: utc(bindingEnd.value) } }) }) }
async function readUsage() { reconciliation.value = null; await act(async () => { const result = await syncCostSource({ start: usageStart.value + 'T00:00:00Z', end: usageEnd.value + 'T00:00:00Z', model: usageModel.value }); reconciliation.value = result.report; message.value = tr(`已读取 ${result.account_count} 个来源账号，并核对 ${result.request_count} 次请求；成本库读回 ${result.stored_request_count} 次。`, `Read ${result.account_count} accounts and verified ${result.request_count} requests against ${result.stored_request_count} stored requests.`) }) }
async function readAnalysisFile(event: Event) { clearAnalysis(); const file = (event.target as HTMLInputElement).files?.[0]; if (!file) return; await act(async () => { if (file.size > 262144) throw new Error('Input limit'); const value: unknown = JSON.parse(await file.text()); if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error('Object required'); analysisInput.value = value as Record<string, unknown> }) }
async function runAnalysis() { analysisResult.value = null; await act(async () => { if (analysisInput.value) analysisResult.value = await analyzeNativeCost(analysisKind.value, analysisInput.value) }) }
async function saveAnalysis() { await act(async () => { const input = analysisInput.value; if (!input || !analysisResult.value) return; const kind = analysisKind.value; if (kind === 'traffic') await save({ kind, traffic: input }); else if (kind === 'replay') await save({ kind, replay: input }); else await save({ kind, forecast: input }) }) }
function reconciliationStatus(status: string) { const labels: Record<string, string> = { TIER_PERIOD_NOT_CONFIRMED: '档位期间待核对', MATCH: '一致', MISMATCH: '存在差额', NO_RECORDED_REQUESTS: '尚无请求数交付记录', CROSS_BOUNDARY_RECORDS: '交付记录跨越所查期间', RECORDED_SCOPE_INCOMPLETE: '交付记录未覆盖整段期间' }; return tr(labels[status] ?? status, status) }
const analysisDisplay = computed(() => {
 const result = analysisResult.value
 if (!result) return []
 const statuses: Record<string, string> = { TARIFF_ESTIMATE_NOT_INVOICE: '费率估算，非发票', INCOMPLETE_COVERAGE: '采样覆盖不完整', COUNTER_RESET: '计数器复位，不能估算', INCOMPLETE_WINDOW_COVERAGE: '窗口覆盖不完整', DECLARED_WINDOW_SCENARIO: '基于所填窗口的情景', CONDITIONAL_SCENARIO: '条件预测，非完整成本底价' }
 const labels: Record<string, string> = { status: '结果类型', rx_bytes: '接收字节', tx_bytes: '发送字节', estimated_charge: '预计超额费', currency: '币种', delivered: '情景交付量', unmet: '未满足需求', cost_p10: '单位费用 P10', cost_p50: '单位费用 P50', cost_p90: '单位费用 P90', event_rate_posterior_mean_per_week: '后验平均每周事件率' }
 return Object.entries(result).filter(([key]) => key in labels).map(([key, value]) => ({ label: tr(labels[key], key.replace(/_/g, ' ')), value: value === null ? tr('未知', 'Unknown') : key === 'status' ? tr(statuses[String(value)] ?? String(value), String(value)) : String(value) }))
})
</script>
