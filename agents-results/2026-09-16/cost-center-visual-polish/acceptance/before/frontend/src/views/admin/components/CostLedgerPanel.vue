<template>
  <section class="native-ledger" aria-labelledby="ledger-title">
    <div class="heading"><h2 id="ledger-title">{{ tr('采购与期间费用账','Purchases and period expenses') }}</h2><button type="button" :disabled="busy" @click="refresh">{{ tr('刷新账本','Refresh') }}</button></div>
    <p class="status" role="status">{{ connected ? tr('持久化账本可读取；录入只改变账本，不发生购买或充值。','Persistent ledger is readable. Recording does not purchase or recharge anything.') : tr('账本尚未连接或不可读取。不会把缺失数据显示为零成本。','Ledger is not connected or readable. Missing data is not zero cost.') }}</p>
    <p v-if="error" class="error" role="alert">{{ error }}</p><p v-if="message" aria-live="polite">{{ message }}</p>
    <form class="filters" @submit.prevent="loadSummary">
      <label>{{ tr('账期起（UTC，含）','Start (UTC, inclusive)') }}<input v-model="filters.start" type="date" required /></label>
      <label>{{ tr('账期止（UTC，不含）','End (UTC, exclusive)') }}<input v-model="filters.end" type="date" required /></label>
      <label>{{ tr('币种','Currency') }}<select v-model="filters.currency"><option v-for="c in currencies" :key="c">{{ c }}</option></select></label>
      <label>{{ tr('模型 / 工作负载','Model / workload') }}<input v-model="filters.model" required maxlength="100" /></label>
      <label>{{ tr('交付量单位','Delivery unit') }}<input v-model="filters.unit" required maxlength="100" /></label>
      <button :disabled="busy || !connected">{{ tr('查询同口径账期','Query period') }}</button>
    </form>
    <div v-if="summary">
      <p class="note">{{ tr('已发生费用截止查询时点；整期费用包含尚未发生的服务时间。单位成本仅覆盖已记录范围，不是已对账的完整经营底价。','Recognized expense stops at the query time; full-period expense includes scheduled service. Unit cost covers recorded scope only, not a reconciled business cost floor.') }}</p>
      <div class="scroll"><table><thead><tr><th>{{ tr('归属','Tier') }}</th><th>{{ tr('现金已付','Cash paid') }}</th><th>{{ tr('整期费用','Full period') }}</th><th>{{ tr('已发生费用','Recognized') }}</th><th>{{ tr('剩余服务价值','Remaining service') }}</th><th>{{ tr('有效交付','Delivered') }}</th><th>{{ tr('范围内单位成本','Scope unit cost') }}</th></tr></thead><tbody>
        <tr v-for="row in summary.rows" :key="row.tier"><td>{{ tierNames[row.tier] ?? row.tier }}</td><td>{{ row.cash_paid }}</td><td>{{ row.period_expense }}</td><td>{{ row.recognized_expense }}</td><td>{{ row.remaining_service_value }}</td><td>{{ row.delivered }}</td><td>{{ row.recorded_scope_unit_cost ?? '—' }}</td></tr>
      </tbody></table></div>
      <p class="note">{{ summary.warnings.map(w => warning(w)).join('；') }}</p>
    </div>
    <details :open="!events.length"><summary>{{ tr('录入已付款采购','Record a paid purchase') }}</summary>
      <form @submit.prevent="savePurchase"><div class="fields">
        <label>{{ tr('凭据业务编号','Business reference') }}<input v-model="purchase.reference" required pattern="[A-Za-z0-9][A-Za-z0-9_.:/-]{0,119}" /></label>
        <label>{{ tr('供应商标识','Supplier reference') }}<input v-model="purchase.supplier" required /></label>
        <label>{{ tr('账号 / 主机标识','Account / host reference') }}<input v-model="purchase.asset" required /></label>
        <label>{{ tr('费用归属','Tier') }}<select v-model="purchase.tier"><option v-for="t in allTiers" :key="t" :value="t">{{ tierNames[t] }}</option></select></label>
        <label>{{ tr('费用类别','Expense category') }}<select v-model="purchase.kind"><option v-for="k in kinds" :key="k.value" :value="k.value">{{ zh ? k.zh : k.en }}</option></select></label>
        <label>{{ tr('付款金额（不含重复分摊）','Paid amount') }}<input v-model="purchase.amount" inputmode="decimal" required /></label>
        <label>{{ tr('币种','Currency') }}<select v-model="purchase.currency"><option v-for="c in currencies" :key="c">{{ c }}</option></select></label>
        <label>{{ tr('付款时间（UTC）','Paid time (UTC)') }}<input v-model="paidTime" type="datetime-local" required /></label>
        <label>{{ tr('服务起（UTC，含）','Service start (UTC)') }}<input v-model="serviceStart" type="date" required /></label>
        <label>{{ tr('服务止（UTC，不含）','Service end (UTC)') }}<input v-model="serviceEnd" type="date" required /></label>
        <label>{{ tr('证据类型','Evidence') }}<select v-model="purchase.evidence"><option value="manual">{{ tr('人工录入，未核验','Manual, unverified') }}</option><option value="invoice">{{ tr('附凭据引用，待对账','Invoice reference, to reconcile') }}</option><option value="synthetic">{{ tr('合成测试数据','Synthetic test data') }}</option></select></label>
        <label>{{ tr('凭据引用（不粘贴密钥）','Evidence reference (no secrets)') }}<input v-model="purchase.evidence_ref" required /></label>
      </div><button :disabled="busy || !connected">{{ tr('保存采购记录，不执行购买','Save record — no purchase') }}</button></form>
      <p class="note">{{ tr('预充值额度批次、退款、税务凭证和自动摊销调账尚未迁入本原生账本；不要用“其他费用”冒充预付余额核销。','Prepaid credit lots, refunds and tax workflows are not migrated here. Do not use an other-expense entry to imitate prepaid credit consumption.') }}</p>
    </details>
    <details><summary>{{ tr('分配共享费用','Allocate a shared expense') }}</summary><form @submit.prevent="saveAllocation"><div class="fields">
      <label>{{ tr('未分配采购记录 ID','Unallocated purchase ID') }}<input v-model="allocationTarget" required /></label>
      <label v-for="t in tiers" :key="t">{{ tierNames[t] }} {{ tr('权重（合计为 1）','weight (sum = 1)') }}<input v-model="weights[t]" inputmode="decimal" required /></label>
    </div><button :disabled="busy || !connected">{{ tr('记录分摊，不新增费用','Record allocation, no additional expense') }}</button></form></details>
    <details><summary>{{ tr('录入已完成交付量','Record completed delivery') }}</summary><form @submit.prevent="saveDelivery"><div class="fields">
      <label>{{ tr('交付聚合编号','Aggregate reference') }}<input v-model="delivered.reference" required /></label><label>{{ tr('账号标识','Account reference') }}<input v-model="delivered.asset" required /></label>
      <label>{{ tr('档位','Tier') }}<select v-model="delivered.tier"><option v-for="t in tiers" :key="t" :value="t">{{ tierNames[t] }}</option></select></label>
      <label>{{ tr('模型 / 工作负载','Model / workload') }}<input v-model="delivered.model" required /></label><label>{{ tr('单位','Unit') }}<input v-model="delivered.unit" required /></label>
      <label>{{ tr('已完成数量','Completed quantity') }}<input v-model="delivered.quantity" inputmode="decimal" required /></label>
      <label>{{ tr('区间起（UTC，含）','Start (UTC, inclusive)') }}<input v-model="deliveryStart" type="date" required /></label><label>{{ tr('区间止（UTC，不含）','End (UTC, exclusive)') }}<input v-model="deliveryEnd" type="date" required /></label>
    </div><button :disabled="busy || !connected">{{ tr('保存已完成交付','Save completed delivery') }}</button></form><p class="note">{{ tr('同一账号与工作负载的聚合区间不能重叠；跨查询边界的数据不会按天数伪分摊。','Aggregate intervals for one account and workload must not overlap. Crossing query boundaries does not trigger invented prorating.') }}</p></details>
    <h3>{{ tr('追加式记录与更正','Append-only records and corrections') }}</h3>
    <p class="note">{{ tr('显示已加载的记录。作废仅更正误录，不退款、不删原文；服务端仍会检查分摊依赖。','Showing loaded records. Voiding corrects a booking mistake, not a refund or deletion; dependencies remain server-validated.') }}</p>
    <div class="scroll"><table><thead><tr><th>ID</th><th>{{ tr('动作','Action') }}</th><th>{{ tr('记录时间','Recorded at') }}</th><th>{{ tr('详情','Details') }}</th><th>{{ tr('更正','Correction') }}</th></tr></thead><tbody>
      <tr v-for="event in events" :key="event.id"><td>{{ event.id }}</td><td>{{ event.command.kind }}</td><td>{{ event.recorded_at }}</td><td><details><summary>{{ tr('查看','View') }}</summary><pre>{{ JSON.stringify(event.command, null, 2) }}</pre></details></td><td><button v-if="event.command.kind !== 'void'" type="button" :disabled="busy" @click="voidTarget = event">{{ tr('更正误录','Correct booking') }}</button></td></tr>
    </tbody></table></div>
    <button v-if="nextAfter !== null" type="button" :disabled="busy" @click="loadMore">{{ tr('加载更多记录','Load more') }}</button>
    <form v-if="voidTarget" class="correction" @submit.prevent="saveVoid"><p>{{ tr('将追加作废记录：','Append a void for: ') }}{{ voidTarget.id }}</p><label>{{ tr('误录原因，不填写敏感信息','Reason, no sensitive information') }}<input v-model="voidReason" required maxlength="240" /></label><button :disabled="busy">{{ tr('确认作废误录（不是退款）','Confirm booking void (not a refund)') }}</button><button type="button" @click="voidTarget = null">{{ tr('取消','Cancel') }}</button></form>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { appendCostLedger, getCostLedgerHealth, getCostLedgerSummary, listCostLedgerEvents } from '@/api/admin/cost-center'
import type { CostDelivery, CostPurchase, CostTier, LedgerCommand, LedgerEvent, LedgerSummary } from '@/api/admin/cost-center'
const { locale } = useI18n()
const zh = computed(() => locale.value.startsWith('zh'))
const tr = (cn: string, en: string) => zh.value ? cn : en
const tiers: CostTier[] = ['plus','pro5x','pro20x']
const allTiers: Array<CostTier | 'unallocated'> = [...tiers,'unallocated']
const tierNames: Record<string,string> = { plus:'Plus',pro5x:'Pro 5x',pro20x:'Pro 20x',unallocated:'未分配 / Unallocated' }
const currencies = ['USD','CNY','HKD','SGD','EUR']
const kinds = [{value:'subscription',zh:'账号订阅',en:'Subscription'},{value:'server',zh:'服务器',en:'Server'},{value:'traffic',zh:'流量',en:'Traffic'},{value:'labor',zh:'人工',en:'Labor'},{value:'other',zh:'其他期间费用',en:'Other period expense'}]
const now = new Date(), today = now.toISOString().slice(0,10), first = today.slice(0,8)+'01'
const nextMonth = new Date(Date.UTC(now.getUTCFullYear(),now.getUTCMonth()+1,1)).toISOString().slice(0,10)
const filters = reactive({ start:first,end:nextMonth,currency:'USD',model:'',unit:'' })
const purchase = reactive<CostPurchase>({reference:'',supplier:'',asset:'',tier:'plus',kind:'subscription',currency:'USD',amount:'',paid_at:'',service_start:'',service_end:'',evidence:'manual',evidence_ref:''})
const paidTime=ref(now.toISOString().slice(0,16)), serviceStart=ref(first),serviceEnd=ref(nextMonth)
const delivered=reactive<CostDelivery>({reference:'',asset:'',tier:'plus',model:'',unit:'',quantity:'',period_start:'',period_end:'',evidence:'manual'})
const deliveryStart=ref(first),deliveryEnd=ref(today),allocationTarget=ref(''),weights=reactive<Record<CostTier,string>>({plus:'0',pro5x:'0',pro20x:'0'})
const connected=ref(false),busy=ref(false),error=ref(''),message=ref(''),events=ref<LedgerEvent[]>([]),nextAfter=ref<number|null>(null),summary=ref<LedgerSummary|null>(null)
const voidTarget=ref<LedgerEvent|null>(null),voidReason=ref('')
const pendingKeys = new Map<string,string>()
function fail(): void { error.value=tr('操作未完成。请检查账本连接、金额与账期、业务编号、分摊依赖；相同内容可安全重试。','Operation failed. Check ledger connection, values, intervals, references and dependencies; identical content can be safely retried.') }
async function refresh(): Promise<void> {
  busy.value=true;error.value=''
  try {await getCostLedgerHealth();connected.value=true;const data=await listCostLedgerEvents();events.value=data.items;nextAfter.value=data.next_after}
  catch {connected.value=false;fail()} finally {busy.value=false}
}
async function loadMore(): Promise<void> { if(nextAfter.value===null)return;busy.value=true;try{const data=await listCostLedgerEvents(nextAfter.value);events.value.push(...data.items);nextAfter.value=data.next_after}catch{fail()}finally{busy.value=false} }
async function loadSummary(): Promise<void> {busy.value=true;error.value='';summary.value=null;try{summary.value=await getCostLedgerSummary({...filters,start:filters.start+'T00:00:00Z',end:filters.end+'T00:00:00Z'})}catch{fail()}finally{busy.value=false} }
async function save(command: LedgerCommand): Promise<boolean> {
 busy.value=true;error.value='';message.value=''
 const payload=JSON.stringify(command);let key=pendingKeys.get(payload)
 if(!key){key=crypto.randomUUID();pendingKeys.set(payload,key)}
 try{const result=await appendCostLedger(command,key);message.value=tr('记录已保存：','Record saved: ')+result.event.id+(result.replayed?tr('（重试读回，没有重复记账）',' (replayed, not duplicated)'):'');await refresh();summary.value=null;return true}
 catch{fail();return false}finally{busy.value=false}
}
async function savePurchase(): Promise<void> {await save({kind:'purchase',purchase:{...purchase,paid_at:paidTime.value+':00Z',service_start:serviceStart.value+'T00:00:00Z',service_end:serviceEnd.value+'T00:00:00Z'}})}
async function saveDelivery(): Promise<void> {await save({kind:'delivery',delivery:{...delivered,period_start:deliveryStart.value+'T00:00:00Z',period_end:deliveryEnd.value+'T00:00:00Z'}})}
async function saveAllocation(): Promise<void> {const parts=tiers.filter(t=>!/^0(?:\.0+)?$/.test(weights[t].trim())).map(t=>({tier:t,weight:weights[t].trim()}));await save({kind:'allocate',allocation:{purchase_id:allocationTarget.value,parts}})}
async function saveVoid(): Promise<void> {if(!voidTarget.value)return;if(await save({kind:'void',void:{target_id:voidTarget.value.id,expected_hash:voidTarget.value.request_hash,reason:voidReason.value}})){voidTarget.value=null;voidReason.value=''}}
const warningMessages:Record<string,string>={OTHER_CURRENCIES_EXCLUDED:'其他币种已排除，单位成本暂停计算',OTHER_WORKLOAD_DELIVERIES_EXCLUDED:'其他工作负载未计入分母，单位成本暂停计算',CROSS_BOUNDARY_DELIVERY_NOT_PRORATED:'跨边界交付未分摊',COST_EVIDENCE_NOT_FULLY_INVOICED:'包含人工或合成费用记录',MANUAL_SCOPE_NOT_COMPLETE_COST_OR_MODEL_COST_ALLOCATION:'仅覆盖已记录范围，不代表完整账务或模型成本归因'}
function warning(s:string):string{return zh.value ? warningMessages[s]??s:s}
onMounted(refresh)
</script>

<style scoped>
.native-ledger{margin:24px 0;padding:22px;border:1px solid #cfdae2;border-radius:10px;background:#fff;color:#173247}.heading{display:flex;justify-content:space-between;gap:12px;align-items:center}h2{font-size:23px}h3{font-size:19px}.fields,.filters{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:14px;margin:16px 0}label{display:flex;flex-direction:column;gap:6px;font-size:14px}input,select{font:inherit;font-size:16px;min-width:0;width:100%;padding:9px;border:1px solid #bbcbd7;border-radius:4px;background:white;color:#173247}button{font:inherit;padding:10px 15px;border:1px solid #abc1cf;border-radius:5px;background:#24536b;color:white;cursor:pointer}button:disabled{opacity:.55;cursor:not-allowed}.note{font-size:14px;color:#526775}.status{border-left:3px solid #8c703d;background:#faf4e8;padding:12px}.error{color:#9e253b}.scroll{overflow-x:auto}table{width:100%;min-width:840px;font-size:14px}th,td{text-align:left;padding:11px;border-bottom:1px solid #dbe3e9;vertical-align:top}details{margin:14px 0;border-top:1px solid #dbe3e9;padding-top:12px}summary{font-weight:600;cursor:pointer}pre{white-space:pre-wrap;overflow-wrap:anywhere;max-width:440px}.correction{border:1px solid #ad6870;padding:15px;margin-top:18px}.correction button{margin:8px 8px 0 0}:global(.dark) .native-ledger{background:#101c2c;color:#e0e7ef;border-color:#3b4d60}:global(.dark) .native-ledger .note{color:#b4c2d0}:global(.dark) .native-ledger .status{background:#342d20;color:#eadabc}@media(max-width:800px){.fields,.filters{grid-template-columns:1fr 1fr}.native-ledger{padding:15px}}@media(max-width:450px){.fields,.filters{grid-template-columns:1fr}.heading{display:block}}
</style>
