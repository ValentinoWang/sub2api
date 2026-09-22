import { computed, inject, provide, reactive, ref, type InjectionKey } from 'vue'
import { useI18n } from 'vue-i18n'
import { appendCostLedger, getCostLedgerHealth, getCostLedgerSummary, listCostLedgerEvents } from '@/api/admin/cost-center'
import type { CostDelivery, CostPurchase, CostTier, LedgerCommand, LedgerEvent, LedgerSummary } from '@/api/admin/cost-center'

function createCostLedger() {
  const { locale } = useI18n()
  const zh = computed(() => locale.value.startsWith('zh'))
  const tr = (cn: string, en: string) => zh.value ? cn : en
  const tiers: CostTier[] = ['plus','pro5x','pro20x']
  const allTiers: Array<CostTier | 'unallocated'> = [...tiers,'unallocated']
  const tierText: Record<string, [string, string]> = { plus:['Plus','Plus'], pro5x:['Pro 5x','Pro 5x'], pro20x:['Pro 20x','Pro 20x'], unallocated:['未分配','Unallocated'] }
  function tierName(tier: string): string { const text = tierText[tier]; return text ? tr(text[0], text[1]) : tier }
  const currencies = ['USD','CNY','HKD','SGD','EUR']
  const kinds = [{value:'subscription',zh:'账号订阅',en:'Subscription'},{value:'server',zh:'服务器',en:'Server'},{value:'traffic',zh:'流量',en:'Traffic'},{value:'labor',zh:'人工',en:'Labor'},{value:'other',zh:'其他期间费用',en:'Other period expense'}]
  const kindText = new Map(kinds.map(kind => [kind.value, kind] as const))
  const moneyColumns = [
    { key: 'cash_paid', zh: '现金已付', en: 'Cash paid' },
    { key: 'period_expense', zh: '整期费用', en: 'Full period' },
    { key: 'recognized_expense', zh: '已发生费用', en: 'Recognized' },
    { key: 'remaining_service_value', zh: '剩余服务价值', en: 'Remaining service' }
  ]
  const now = new Date(), today = now.toISOString().slice(0,10), first = today.slice(0,8)+'01'
  const nextMonth = new Date(Date.UTC(now.getUTCFullYear(),now.getUTCMonth()+1,1)).toISOString().slice(0,10)
  const filters = reactive({ start:first,end:nextMonth,currency:'USD',model:'',unit:'' })
  const purchase = reactive<CostPurchase>({reference:'',supplier:'',asset:'',tier:'plus',kind:'subscription',currency:'USD',amount:'',paid_at:'',service_start:'',service_end:'',evidence:'manual',evidence_ref:''})
  const paidTime=ref(now.toISOString().slice(0,16)), serviceStart=ref(first),serviceEnd=ref(nextMonth)
  const delivered=reactive<CostDelivery>({reference:'',asset:'',tier:'plus',model:'',unit:'',quantity:'',period_start:'',period_end:'',evidence:'manual'})
  const deliveryStart=ref(first),deliveryEnd=ref(today),allocationTarget=ref(''),weights=reactive<Record<CostTier,string>>({plus:'0',pro5x:'0',pro20x:'0'})
  const connected=ref(false),busy=ref(false),events=ref<LedgerEvent[]>([]),nextAfter=ref<number|null>(null),summary=ref<LedgerSummary|null>(null)
  const voidTarget=ref<LedgerEvent|null>(null),voidReason=ref('')
  const pendingKeys = new Map<string,string>()
  // 仅用于展示：连接状态、分区展开、消息归属与已读取计数，不参与任何核算。
  const health = ref<{ status: string; event_count: number } | null>(null)
  const total = ref<number|null>(null)
  const loaded = ref(false)
  const checkedAt = ref('')
  const queryError = ref(''), listError = ref(''), formError = ref(''), message = ref('')
  const lastAction = ref<'purchase'|'allocate'|'delivery'|'void'|''>('')
  const open = reactive({ purchase: true, allocate: false, delivery: false })
  const expandedIds = reactive<Record<string, boolean>>({})
  const summaryScope = ref<{ model: string; unit: string } | null>(null)
  const countLabel = computed(() => total.value === null
    ? tr(`已加载 ${events.value.length} 条`, `${events.value.length} loaded`)
    : tr(`已加载 ${events.value.length} / 共 ${total.value} 条`, `${events.value.length} of ${total.value} loaded`))
  function failText(): string { return tr('操作没有完成。请检查账本连接、金额与账期、业务编号和分摊依赖；内容不变时可以直接重试，不会重复记账。','Operation did not complete. Check the ledger connection, values, intervals, references and dependencies; retrying identical content will not double-book.') }
  function toggleDetail(id: string): void { expandedIds[id] = !expandedIds[id] }
  function startVoid(event: LedgerEvent): void { voidTarget.value = event; formError.value = ''; message.value = '' }
  function cancelVoid(): void { voidTarget.value = null }
  async function refresh(): Promise<void> {
    busy.value=true;listError.value=''
    try {health.value=await getCostLedgerHealth();connected.value=true;const data=await listCostLedgerEvents();events.value=data.items;nextAfter.value=data.next_after;total.value=data.total}
    catch {connected.value=false;health.value=null} finally {busy.value=false;loaded.value=true;checkedAt.value=new Date().toTimeString().slice(0,8)}
  }
  async function loadMore(): Promise<void> { if(nextAfter.value===null)return;busy.value=true;listError.value='';try{const data=await listCostLedgerEvents(nextAfter.value);events.value.push(...data.items);nextAfter.value=data.next_after;total.value=data.total}catch{listError.value=failText()}finally{busy.value=false} }
  async function loadSummary(): Promise<void> {busy.value=true;queryError.value='';summary.value=null;summaryScope.value=null;try{summary.value=await getCostLedgerSummary({...filters,start:filters.start+'T00:00:00Z',end:filters.end+'T00:00:00Z'});summaryScope.value={model:filters.model,unit:filters.unit}}catch{queryError.value=failText()}finally{busy.value=false} }
  async function save(command: LedgerCommand): Promise<boolean> {
   busy.value=true;formError.value='';message.value=''
   const payload=JSON.stringify(command);let key=pendingKeys.get(payload)
   if(!key){key=crypto.randomUUID();pendingKeys.set(payload,key)}
   try{const result=await appendCostLedger(command,key);message.value=tr('记录已保存：','Record saved: ')+result.event.id+(result.replayed?tr('（重试读回，没有重复记账）',' (replayed, not duplicated)'):'');await refresh();summary.value=null;summaryScope.value=null;return true}
   catch{formError.value=failText();return false}finally{busy.value=false}
  }
  async function savePurchase(): Promise<void> {lastAction.value='purchase';await save({kind:'purchase',purchase:{...purchase,paid_at:paidTime.value+':00Z',service_start:serviceStart.value+'T00:00:00Z',service_end:serviceEnd.value+'T00:00:00Z'}})}
  async function saveDelivery(): Promise<void> {lastAction.value='delivery';await save({kind:'delivery',delivery:{...delivered,period_start:deliveryStart.value+'T00:00:00Z',period_end:deliveryEnd.value+'T00:00:00Z'}})}
  async function saveAllocation(): Promise<void> {lastAction.value='allocate';const parts=tiers.filter(t=>!/^0(?:\.0+)?$/.test(weights[t].trim())).map(t=>({tier:t,weight:weights[t].trim()}));await save({kind:'allocate',allocation:{purchase_id:allocationTarget.value,parts}})}
  async function saveVoid(): Promise<void> {if(!voidTarget.value)return;lastAction.value='void';if(await save({kind:'void',void:{target_id:voidTarget.value.id,expected_hash:voidTarget.value.request_hash,reason:voidReason.value}})){voidTarget.value=null;voidReason.value=''}}
  const warningText: Record<string, [string, string]> = {
    OTHER_CURRENCIES_EXCLUDED: ['其他币种已排除，单位成本暂不计算', 'Other currencies excluded; unit cost withheld'],
    OTHER_WORKLOAD_DELIVERIES_EXCLUDED: ['其他工作负载没有计入分母，单位成本暂不计算', 'Other workload deliveries excluded; unit cost withheld'],
    CROSS_BOUNDARY_DELIVERY_NOT_PRORATED: ['跨账期边界的交付没有按天分摊', 'Delivery crossing the period boundary is not prorated'],
    COST_EVIDENCE_NOT_FULLY_INVOICED: ['包含人工录入或合成的费用记录', 'Includes manual or synthetic cost records'],
    MANUAL_SCOPE_NOT_COMPLETE_COST_OR_MODEL_COST_ALLOCATION: ['只覆盖已记录范围，不代表完整账务或模型成本归因', 'Recorded scope only; not complete accounting or model cost attribution']
  }
  function warning(code: string): string { const text = warningText[code]; return text ? tr(text[0], text[1]) : code }
  const summaryStatusText: Record<string, [string, string]> = {
    RECORDED_SCOPE_ONLY: ['仅已记录范围', 'Recorded scope only'],
    IN_PROGRESS_PERIOD: ['账期进行中', 'Period in progress']
  }
  function summaryStatus(code: string): string { const text = summaryStatusText[code]; return text ? tr(text[0], text[1]) : code }
  const actionText: Record<string, [string, string]> = {
    purchase: ['采购', 'Purchase'],
    delivery: ['交付', 'Delivery'],
    allocate: ['分摊', 'Allocation'],
    void: ['作废', 'Void'],
    credit_lot: ['预充值批次', 'Prepaid lot'],
    credit_use: ['额度核销', 'Credit consumption'],
    account_interval: ['账号归属期间', 'Account interval'],
    traffic: ['流量采样', 'Traffic sample'],
    replay: ['双窗口回放', 'Window replay'],
    forecast: ['条件预测', 'Conditional forecast']
  }
  function actionLabel(kind: string): string { const text = actionText[kind]; return text ? tr(text[0], text[1]) : kind }
  function actionTone(kind: string): string { return kind === 'void' ? 'badge-warning' : kind === 'purchase' ? 'badge-primary' : 'badge-gray' }
  // 摘要只重排已有字段，完整命令保留在详情里。
  function summaryLine(event: LedgerEvent): string {
    const command = event.command
    if (command.kind === 'purchase') {
      const kind = kindText.get(command.purchase.kind)
      return [command.purchase.supplier, command.purchase.asset, `${command.purchase.amount} ${command.purchase.currency}`, tierName(command.purchase.tier), kind ? tr(kind.zh, kind.en) : command.purchase.kind].filter(Boolean).join(' · ')
    }
    if (command.kind === 'delivery') {
      return [command.delivery.asset, command.delivery.model, `${command.delivery.quantity} ${command.delivery.unit}`, tierName(command.delivery.tier)].filter(Boolean).join(' · ')
    }
    if (command.kind === 'allocate') {
      return [command.allocation.purchase_id, command.allocation.parts.map(part => `${tierName(part.tier)} ${part.weight}`).join(' / ')].filter(Boolean).join(' · ')
    }
    if (command.kind === 'void') return [command.void.target_id, command.void.reason].filter(Boolean).join(' · ')
    if (command.kind === 'credit_lot') return `${command.credit_lot.pool} · ${command.credit_lot.amount} ${command.credit_lot.currency}`
    if (command.kind === 'credit_use') return `${command.credit_use.pool} · ${command.credit_use.quantity} ${command.credit_use.unit}`
    if (command.kind === 'account_interval') return `${command.account_interval.asset} · ${tierName(command.account_interval.tier)} · ${utcLabel(command.account_interval.start)}`
    return actionLabel(command.kind)
  }
  // UTC 展示，原始值保留在 title/datetime 中。
  function utcLabel(value: string, withSeconds = false): string {
    const ms = Date.parse(value)
    if (!Number.isFinite(ms)) return value
    const iso = new Date(ms).toISOString()
    return `${iso.slice(0, 10)} ${iso.slice(11, withSeconds ? 19 : 16)} UTC`
  }

  let pendingLoad: Promise<void> | null = null
  function ensureLoaded(): Promise<void> {
    if (loaded.value) return Promise.resolve()
    if (!pendingLoad) pendingLoad = refresh().finally(() => { pendingLoad = null })
    return pendingLoad
  }

  return {
    tr, zh, tiers, allTiers, tierName, currencies, kinds, moneyColumns,
    filters, purchase, paidTime, serviceStart, serviceEnd, delivered, deliveryStart, deliveryEnd,
    allocationTarget, weights, connected, busy, events, nextAfter, summary, voidTarget,
    voidReason, health, total, loaded, checkedAt, queryError, listError, formError,
    message, lastAction, open, expandedIds, summaryScope, countLabel, toggleDetail, startVoid,
    cancelVoid, refresh, loadMore, loadSummary, savePurchase, saveDelivery, saveAllocation, saveVoid,
    warning, summaryStatus, actionLabel, actionTone, summaryLine, utcLabel,
    ensureLoaded
  }
}

const ledgerKey: InjectionKey<ReturnType<typeof createCostLedger>> = Symbol('cost-ledger')

export function provideCostLedger(): void {
  provide(ledgerKey, createCostLedger())
}

export function useCostLedger() {
  const ledger = inject(ledgerKey)
  if (!ledger) throw new Error('Cost ledger requires the cost center layout')
  return ledger
}
