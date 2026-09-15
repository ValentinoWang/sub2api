<template>
  <AppLayout>
    <div class="cost-center">
      <header>
        <p class="eyebrow">PLUS / PRO 5X / PRO 20X</p>
        <h1>{{ zh ? '三档成本比较' : 'Three-tier cost comparison' }}</h1>
        <p>{{ zh ? '同一工作负载、同一账期、同一币种。参考月费不是实际采购账。' : 'One workload, billing period and currency. Reference fees are not procurement invoices.' }}</p>
      </header>
      <p class="notice">{{ zh ? '只读计算器：不会购买、升级、兑换额度或修改售价。生产采购账与额度事件尚未连接。' : 'Calculator only: no purchases, upgrades, credit redemption or price changes. Production ledgers are not connected.' }}</p>
      <p v-if="catalog" class="hint">{{ zh ? '目录核查时间：' : 'Catalog checked: ' }}{{ catalog.checked_at }} · {{ zh ? '新购资料有效至：' : 'New-purchase facts valid until: ' }}{{ catalog.valid_until }}</p>
      <p v-if="error" role="alert" class="error">{{ error }}</p>
      <form @submit.prevent="calculate">
        <div class="fields">
          <label>{{ zh ? '用途' : 'Purpose' }}<select v-model="input.purpose"><option value="existing">{{ zh ? '已有账号成本' : 'Existing accounts' }}</option><option value="new_purchase">{{ zh ? '新购可得性情景' : 'New-purchase scenario' }}</option></select></label>
          <label>{{ zh ? '币种' : 'Currency' }}<select v-model="input.currency"><option v-for="c in currencies" :key="c">{{ c }}</option></select></label>
          <label>{{ zh ? '期内合格交付目标' : 'Qualified period demand' }}<input v-model="input.demand" inputmode="decimal" required pattern="[0-9]+([.][0-9]+)?" /></label>
          <label>{{ zh ? '模型 / 工作负载版本' : 'Model / workload version' }}<input v-model="input.model" required maxlength="100" /></label>
          <label>{{ zh ? '计量单位' : 'Unit' }}<input v-model="input.unit" required maxlength="100" /></label>
          <label>{{ zh ? '情景时点（UTC）' : 'Scenario time (UTC)' }}<input v-model="asOf" type="datetime-local" required /></label>
          <label>{{ zh ? '账期起（UTC，含）' : 'Start (UTC, inclusive)' }}<input v-model="periodStart" type="date" required /></label>
          <label>{{ zh ? '账期止（UTC，不含）' : 'End (UTC, exclusive)' }}<input v-model="periodEnd" type="date" required /></label>
        </div>
        <div class="tiers">
          <section v-for="row in input.rows" :key="row.tier">
            <h2>{{ names[row.tier] }}</h2>
            <p>{{ zh ? '官方参考月费：' : 'Reference monthly fee: ' }}USD {{ catalog?.plans.find(p => p.id === row.tier)?.reference_monthly_usd ?? '—' }}</p>
            <p v-if="row.tier === 'pro20x'" class="hint">{{ zh ? '存量续费与新购分开；采购状态需核验时效。' : 'Renewals and new purchases differ; recheck availability.' }}</p>
            <label>{{ zh ? '本期实际 / 假设总费用' : 'Actual / assumed total cost' }}<input v-model="row.period_cost" inputmode="decimal" /></label>
            <label>{{ zh ? '独立测得 / 假设期内容量' : 'Independent period capacity' }}<input v-model="row.period_capacity" inputmode="decimal" /></label>
            <label>{{ zh ? '有用比例 0–1' : 'Useful fraction 0–1' }}<input v-model="row.useful_fraction" inputmode="decimal" /></label>
            <label>{{ zh ? '数据类型' : 'Evidence' }}<select v-model="row.evidence"><option value="assumption">{{ zh ? '假设' : 'Assumption' }}</option><option value="observed">{{ zh ? '自身观察' : 'Own observation' }}</option><option value="synthetic">{{ zh ? '合成测试' : 'Synthetic' }}</option></select></label>
            <label class="check"><input v-model="row.workload_verified" type="checkbox" />{{ zh ? '已确认该档支持同一工作负载' : 'Same workload verified for this tier' }}</label>
          </section>
        </div>
        <button type="submit" :disabled="busy">{{ busy ? (zh ? '计算中' : 'Calculating') : (zh ? '比较三档，不执行变更' : 'Compare without changes') }}</button>
      </form>
      <section v-if="result" aria-live="polite">
        <h2>{{ zh ? '满足需求后再比较成本' : 'Compare costs only after meeting demand' }}</h2>
        <p>{{ result.lowest_cost_feasible_tiers.length ? result.lowest_cost_feasible_tiers.map(t => names[t]).join(' / ') : (zh ? '没有满足全部条件的档位' : 'No tier meets all constraints') }}</p>
        <div class="table-wrap"><table><thead><tr><th>{{ zh ? '档位' : 'Tier' }}</th><th>{{ zh ? '有效容量' : 'Useful capacity' }}</th><th>{{ zh ? '未满足需求' : 'Unmet demand' }}</th><th>{{ zh ? '单位成本' : 'Unit cost' }}</th><th>{{ zh ? '限制' : 'Constraints' }}</th></tr></thead><tbody>
          <tr v-for="row in result.rows" :key="row.tier"><td>{{ names[row.tier] }}</td><td>{{ row.effective_capacity ?? '—' }}</td><td>{{ row.unmet ?? '—' }}</td><td>{{ row.unit_cost ?? '—' }}</td><td>{{ row.reasons.map(r => reason(r)).join('；') || (zh ? '满足输入条件' : 'Meets declared inputs') }}</td></tr>
        </tbody></table></div>
        <p class="hint">{{ zh ? '这些结果不证明短时窗口、并发、延迟及实际授权可用；不能据此直接开售。' : 'This does not establish short-window, concurrency, latency or authorization feasibility.' }}</p>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { compareCostTiers, getCostCatalog } from '@/api/admin/cost-center'
import type { CostCatalog, CostComparisonInput, CostComparisonResult, CostTier } from '@/api/admin/cost-center'
const { locale } = useI18n()
const zh = computed(() => locale.value.startsWith('zh'))
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
const error = ref('')
const reasons: Record<string, string> = { MISSING_COST_OR_CAPACITY: '费用或容量缺失', WORKLOAD_NOT_VERIFIED: '未确认工作负载可用', NEW_PURCHASE_PAUSED: '参考资料显示暂停新购', ACQUISITION_FACTS_REQUIRE_REFRESH: '采购资料已过期，需复核', INSUFFICIENT_USEFUL_CAPACITY: '有效容量不足', NO_DEMAND: '没有需求' }
function reason(code: string): string { return zh.value ? reasons[code] ?? code : code }
onMounted(async () => { try { catalog.value = await getCostCatalog() } catch { error.value = zh.value ? '套餐目录读取失败，请检查原生接口是否已接入。' : 'Catalog unavailable. Verify native route integration.' } })
async function calculate() {
  busy.value = true; error.value = ''; result.value = null
  try {
    const rows = input.rows.map(r => ({ ...r, period_cost: r.period_cost?.trim() || null, period_capacity: r.period_capacity?.trim() || null, useful_fraction: r.useful_fraction?.trim() || null }))
    result.value = await compareCostTiers({ ...input, rows, period_start: periodStart.value + 'T00:00:00Z', period_end: periodEnd.value + 'T00:00:00Z', as_of: asOf.value + ':00Z' })
  } catch { error.value = zh.value ? '计算未完成，请核对输入和接口状态。' : 'Calculation failed. Check inputs and endpoint availability.' }
  finally { busy.value = false }
}
</script>

<style scoped>
.cost-center{max-width:1200px;margin:auto;padding:24px;color:var(--color-text-primary,#173247)}
.eyebrow{font-size:12px;letter-spacing:.12em}.notice{padding:16px;border:1px solid #d9c7a7;background:#fbf6ec;color:#69532d}.error{padding:12px;color:#9e253b}.fields,.tiers{display:grid;gap:18px;margin:20px 0}.fields{grid-template-columns:repeat(4,minmax(0,1fr))}.tiers{grid-template-columns:repeat(3,minmax(0,1fr))}.tiers section{padding:20px;border:1px solid #ccd8e0;border-radius:8px}label{display:flex;flex-direction:column;gap:6px;margin:10px 0;font-size:14px}input,select{width:100%;padding:9px;border:1px solid #bbcbd7;border-radius:4px;color:#173247;background:white}.check{flex-direction:row;align-items:center}.check input{width:auto}button{padding:12px 20px;background:#24536b;color:white;border-radius:6px}.hint{font-size:14px;color:#5e7280}.table-wrap{overflow-x:auto}table{width:100%;min-width:640px}th,td{text-align:left;padding:12px;border-bottom:1px solid #dbe3e9}header h1{font-size:28px}section h2{font-size:20px}@media(max-width:800px){.fields{grid-template-columns:1fr 1fr}.tiers{grid-template-columns:1fr}}@media(max-width:400px){.fields{grid-template-columns:1fr}.cost-center{padding:12px}}
</style>
