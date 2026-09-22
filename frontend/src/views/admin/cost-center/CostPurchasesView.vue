<template>
  <div class="min-w-0 space-y-5">
    <CostLedgerStatus />
<div class="card overflow-hidden">
      <div class="border-b border-gray-100 p-4 dark:border-dark-700 sm:p-5 md:p-6">
        <h3 class="ledger-title">{{ tr('记录账本事项', 'Record ledger entries') }}</h3>
        <p class="ledger-desc">
          {{ tr('三类记录互不重复：采购记已付款的支出，分摊把共享支出拆到各档，交付记已完成的产出。', 'Three entry types that never overlap: purchases record paid spending, allocations split shared spending across tiers, deliveries record completed output.') }}
        </p>
      </div>

      <div class="divide-y divide-gray-100 dark:divide-dark-700">
        <!-- 采购 -->
        <section>
          <h4>
            <button
              type="button"
              class="ledger-disclosure"
              :aria-expanded="open.purchase"
              aria-controls="ledger-purchase"
              @click="open.purchase = !open.purchase"
            >
              <Icon
                name="chevronRight"
                size="sm"
                class="ledger-chevron mt-0.5 shrink-0 text-gray-400 dark:text-dark-400"
                :class="open.purchase ? 'rotate-90' : ''"
              />
              <span class="min-w-0 flex-1">
                <span class="block text-sm font-medium text-gray-900 dark:text-white">
                  {{ tr('录入已付款采购', 'Record a paid purchase') }}
                </span>
                <span class="mt-0.5 block text-xs leading-5 text-gray-500 dark:text-gray-400">
                  {{ tr('登记一笔已经付过款的支出，保存不会发起任何付款。', 'Log spending that has already been paid; saving never initiates a payment.') }}
                </span>
              </span>
            </button>
          </h4>
          <div v-show="open.purchase" id="ledger-purchase" class="px-4 pb-5 sm:px-5 md:px-6">
            <form class="space-y-4" @submit.prevent="savePurchase">
              <div class="grid grid-cols-1 gap-x-4 gap-y-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
                <div>
                  <label class="ledger-label" for="purchase-reference">{{ tr('业务编号', 'Business reference') }}</label>
                  <input
                    id="purchase-reference"
                    v-model="purchase.reference"
                    class="input"
                    required
                    pattern="[A-Za-z0-9][A-Za-z0-9_.:/-]{0,119}"
                    aria-describedby="purchase-reference-hint"
                  />
                  <p id="purchase-reference-hint" class="ledger-hint">
                    {{ tr('字母或数字开头，可用 _ . : / -', 'Starts with a letter or digit; _ . : / - allowed') }}
                  </p>
                </div>
                <div>
                  <label class="ledger-label" for="purchase-supplier">{{ tr('供应商', 'Supplier') }}</label>
                  <input id="purchase-supplier" v-model="purchase.supplier" class="input" required />
                  <CostInputHint term="supplier" />
                </div>
                <div>
                  <label class="ledger-label" for="purchase-asset">{{ tr('账号 / 主机', 'Account / host') }}</label>
                  <input id="purchase-asset" v-model="purchase.asset" class="input" required />
                  <CostInputHint term="asset" />
                </div>
                <div>
                  <label class="ledger-label" for="purchase-tier">{{ tr('费用归属', 'Tier') }}</label>
                  <select id="purchase-tier" v-model="purchase.tier" class="input">
                    <option v-for="t in allTiers" :key="t" :value="t">{{ tierName(t) }}</option>
                  </select>
                  <CostInputHint term="tier" />
                </div>
                <div>
                  <label class="ledger-label" for="purchase-kind">{{ tr('费用类别', 'Expense category') }}</label>
                  <select id="purchase-kind" v-model="purchase.kind" class="input">
                    <option v-for="k in kinds" :key="k.value" :value="k.value">{{ tr(k.zh, k.en) }}</option>
                  </select>
                  <CostInputHint term="kind" />
                </div>
                <div>
                  <label class="ledger-label" for="purchase-amount">{{ tr('付款金额', 'Paid amount') }}</label>
                  <input
                    id="purchase-amount"
                    v-model="purchase.amount"
                    class="input tabular-nums"
                    inputmode="decimal"
                    required
                    aria-describedby="purchase-amount-hint"
                  />
                  <p id="purchase-amount-hint" class="ledger-hint">
                    {{ tr('同一笔钱只登记一次，分摊在下面单独记录。', 'Log each payment once; splitting is recorded separately below.') }}
                  </p>
                </div>
                <div>
                  <label class="ledger-label" for="purchase-currency">{{ tr('币种', 'Currency') }}</label>
                  <select id="purchase-currency" v-model="purchase.currency" class="input">
                    <option v-for="c in currencies" :key="c">{{ c }}</option>
                  </select>
                  <CostInputHint term="currency" />
                </div>
                <div>
                  <label class="ledger-label" for="purchase-paid-at">{{ tr('付款时间（UTC）', 'Paid time (UTC)') }}</label>
                  <input id="purchase-paid-at" v-model="paidTime" class="input" type="datetime-local" required />
                  <CostInputHint term="time" />
                </div>
                <div>
                  <label class="ledger-label" for="purchase-service-start">
                    {{ tr('服务起（UTC，含当日）', 'Service start (UTC, inclusive)') }}
                  </label>
                  <input id="purchase-service-start" v-model="serviceStart" class="input" type="date" required />
                  <CostInputHint term="service_period" />
                </div>
                <div>
                  <label class="ledger-label" for="purchase-service-end">
                    {{ tr('服务止（UTC，不含当日）', 'Service end (UTC, exclusive)') }}
                  </label>
                  <input id="purchase-service-end" v-model="serviceEnd" class="input" type="date" required />
                  <CostInputHint term="service_period" />
                </div>
                <div>
                  <label class="ledger-label" for="purchase-evidence">{{ tr('证据类型', 'Evidence') }}</label>
                  <select id="purchase-evidence" v-model="purchase.evidence" class="input">
                    <option value="manual">{{ tr('人工录入，未核验', 'Manual, unverified') }}</option>
                    <option value="invoice">{{ tr('附凭据引用，待对账', 'Invoice reference, to reconcile') }}</option>
                    <option value="synthetic">{{ tr('合成测试数据', 'Synthetic test data') }}</option>
                  </select>
                  <CostInputHint term="evidence" />
                </div>
                <div>
                  <label class="ledger-label" for="purchase-evidence-ref">{{ tr('凭据引用', 'Evidence reference') }}</label>
                  <input
                    id="purchase-evidence-ref"
                    v-model="purchase.evidence_ref"
                    class="input"
                    required
                    aria-describedby="purchase-evidence-ref-hint"
                  />
                  <p id="purchase-evidence-ref-hint" class="ledger-hint">
                    {{ tr('不要粘贴密钥或完整账号资料。', 'Never paste secrets or full account data.') }}
                  </p>
                </div>
              </div>

              <div class="flex flex-wrap items-center gap-x-3 gap-y-2">
                <button class="btn btn-primary btn-sm" :disabled="busy || !connected">
                  {{ tr('保存采购记录', 'Save purchase record') }}
                </button>
                <span class="text-xs text-gray-500 dark:text-gray-400">
                  {{ connected ? tr('只写入账本，不执行购买。', 'Writes to the ledger only; no purchase.') : tr('账本连接后才能保存。', 'Available once the ledger is connected.') }}
                </span>
              </div>
              <p v-if="lastAction === 'purchase' && formError" role="alert" class="ledger-alert-error">
                <Icon name="exclamationCircle" size="sm" class="mt-px shrink-0" />
                <span>{{ formError }}</span>
              </p>
              <p v-else-if="lastAction === 'purchase' && message" class="ledger-alert-ok" aria-live="polite">
                <Icon name="checkCircle" size="sm" class="mt-px shrink-0" />
                <span>{{ message }}</span>
              </p>
              <p class="ledger-hint">
                {{ tr('预充值额度批次、退款、税务凭证和自动摊销调账还没有迁到这个账本；不要用“其他期间费用”代替预付余额核销。', 'Prepaid credit lots, refunds, tax documents and automatic amortisation are not migrated here. Do not use an other-expense entry to imitate prepaid credit consumption.') }}
              </p>
            </form>
          </div>
        </section>

        <!-- 分摊 -->
        <section>
          <h4>
            <button
              type="button"
              class="ledger-disclosure"
              :aria-expanded="open.allocate"
              aria-controls="ledger-allocate"
              @click="open.allocate = !open.allocate"
            >
              <Icon
                name="chevronRight"
                size="sm"
                class="ledger-chevron mt-0.5 shrink-0 text-gray-400 dark:text-dark-400"
                :class="open.allocate ? 'rotate-90' : ''"
              />
              <span class="min-w-0 flex-1">
                <span class="block text-sm font-medium text-gray-900 dark:text-white">
                  {{ tr('分配共享费用', 'Allocate a shared expense') }}
                </span>
                <span class="mt-0.5 block text-xs leading-5 text-gray-500 dark:text-gray-400">
                  {{ tr('把一笔未分配的采购按权重拆到各档，不会新增费用。', 'Split one unallocated purchase across tiers by weight; no extra expense is created.') }}
                </span>
              </span>
            </button>
          </h4>
          <div v-show="open.allocate" id="ledger-allocate" class="px-4 pb-5 sm:px-5 md:px-6">
            <form class="space-y-4" @submit.prevent="saveAllocation">
              <div class="grid grid-cols-1 gap-x-4 gap-y-3 sm:grid-cols-2 lg:grid-cols-4">
                <div>
                  <label class="ledger-label" for="allocate-target">{{ tr('未分配采购记录 ID', 'Unallocated purchase ID') }}</label>
                  <input
                    id="allocate-target"
                    v-model="allocationTarget"
                    class="input font-mono text-xs"
                    required
                    aria-describedby="allocate-target-hint"
                  />
                  <p id="allocate-target-hint" class="ledger-hint">
                    {{ tr('在下方记录列表里复制采购记录的 ID。', 'Copy the purchase ID from the record list below.') }}
                  </p>
                </div>
                <div v-for="t in tiers" :key="t">
                  <label class="ledger-label" :for="`allocate-${t}`">{{ tierName(t) }} {{ tr('权重', 'weight') }}</label>
                  <input
                    :id="`allocate-${t}`"
                    v-model="weights[t]"
                    class="input tabular-nums"
                    inputmode="decimal"
                    required
                    aria-describedby="allocate-weight-hint"
                  />
                </div>
              </div>
              <p id="allocate-weight-hint" class="ledger-hint">
                {{ tr('三档权重合计为 1；权重为 0 的档位不会写入分摊。', 'The three weights sum to 1; a zero weight is not written into the allocation.') }}
              </p>
              <div class="flex flex-wrap items-center gap-x-3 gap-y-2">
                <button class="btn btn-primary btn-sm" :disabled="busy || !connected">
                  {{ tr('记录分摊', 'Record allocation') }}
                </button>
                <span class="text-xs text-gray-500 dark:text-gray-400">
                  {{ connected ? tr('只改变归属，不新增费用。', 'Changes attribution only; no additional expense.') : tr('账本连接后才能保存。', 'Available once the ledger is connected.') }}
                </span>
              </div>
              <p v-if="lastAction === 'allocate' && formError" role="alert" class="ledger-alert-error">
                <Icon name="exclamationCircle" size="sm" class="mt-px shrink-0" />
                <span>{{ formError }}</span>
              </p>
              <p v-else-if="lastAction === 'allocate' && message" class="ledger-alert-ok" aria-live="polite">
                <Icon name="checkCircle" size="sm" class="mt-px shrink-0" />
                <span>{{ message }}</span>
              </p>
            </form>
          </div>
        </section>

        <!-- 交付 -->
        <section>
          <h4>
            <button
              type="button"
              class="ledger-disclosure"
              :aria-expanded="open.delivery"
              aria-controls="ledger-delivery"
              @click="open.delivery = !open.delivery"
            >
              <Icon
                name="chevronRight"
                size="sm"
                class="ledger-chevron mt-0.5 shrink-0 text-gray-400 dark:text-dark-400"
                :class="open.delivery ? 'rotate-90' : ''"
              />
              <span class="min-w-0 flex-1">
                <span class="block text-sm font-medium text-gray-900 dark:text-white">
                  {{ tr('录入已完成交付量', 'Record completed delivery') }}
                </span>
                <span class="mt-0.5 block text-xs leading-5 text-gray-500 dark:text-gray-400">
                  {{ tr('登记某个账号在一段区间内已完成的产出，作为单位成本的分母。', 'Log completed output for one account over an interval; it forms the unit cost denominator.') }}
                </span>
              </span>
            </button>
          </h4>
          <div v-show="open.delivery" id="ledger-delivery" class="px-4 pb-5 sm:px-5 md:px-6">
            <form class="space-y-4" @submit.prevent="saveDelivery">
              <div class="grid grid-cols-1 gap-x-4 gap-y-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
                <div>
                  <label class="ledger-label" for="delivery-reference">{{ tr('交付聚合编号', 'Aggregate reference') }}</label>
                  <input id="delivery-reference" v-model="delivered.reference" class="input" required />
                  <CostInputHint term="reference" />
                </div>
                <div>
                  <label class="ledger-label" for="delivery-asset">{{ tr('账号标识', 'Account reference') }}</label>
                  <input id="delivery-asset" v-model="delivered.asset" class="input" required />
                  <CostInputHint term="asset" />
                </div>
                <div>
                  <label class="ledger-label" for="delivery-tier">{{ tr('档位', 'Tier') }}</label>
                  <select id="delivery-tier" v-model="delivered.tier" class="input">
                    <option v-for="t in tiers" :key="t" :value="t">{{ tierName(t) }}</option>
                  </select>
                  <CostInputHint term="tier" />
                </div>
                <div>
                  <label class="ledger-label" for="delivery-model">{{ tr('模型 / 工作负载', 'Model / workload') }}</label>
                  <input id="delivery-model" v-model="delivered.model" class="input" required />
                  <CostInputHint term="model" />
                </div>
                <div>
                  <label class="ledger-label" for="delivery-unit">{{ tr('单位', 'Unit') }}</label>
                  <input id="delivery-unit" v-model="delivered.unit" class="input" required />
                  <CostInputHint term="unit" />
                </div>
                <div>
                  <label class="ledger-label" for="delivery-quantity">{{ tr('已完成数量', 'Completed quantity') }}</label>
                  <input
                    id="delivery-quantity"
                    v-model="delivered.quantity"
                    class="input tabular-nums"
                    inputmode="decimal"
                    required
                  />
                  <CostInputHint term="unit" />
                </div>
                <div>
                  <label class="ledger-label" for="delivery-start">{{ tr('区间起（UTC，含当日）', 'Start (UTC, inclusive)') }}</label>
                  <input id="delivery-start" v-model="deliveryStart" class="input" type="date" required />
                  <CostInputHint term="time" />
                </div>
                <div>
                  <label class="ledger-label" for="delivery-end">{{ tr('区间止（UTC，不含当日）', 'End (UTC, exclusive)') }}</label>
                  <input id="delivery-end" v-model="deliveryEnd" class="input" type="date" required />
                  <CostInputHint term="time" />
                </div>
              </div>
              <div class="flex flex-wrap items-center gap-x-3 gap-y-2">
                <button class="btn btn-primary btn-sm" :disabled="busy || !connected">
                  {{ tr('保存已完成交付', 'Save completed delivery') }}
                </button>
                <span class="text-xs text-gray-500 dark:text-gray-400">
                  {{ connected ? tr('只记录已完成的产出。', 'Records completed output only.') : tr('账本连接后才能保存。', 'Available once the ledger is connected.') }}
                </span>
              </div>
              <p v-if="lastAction === 'delivery' && formError" role="alert" class="ledger-alert-error">
                <Icon name="exclamationCircle" size="sm" class="mt-px shrink-0" />
                <span>{{ formError }}</span>
              </p>
              <p v-else-if="lastAction === 'delivery' && message" class="ledger-alert-ok" aria-live="polite">
                <Icon name="checkCircle" size="sm" class="mt-px shrink-0" />
                <span>{{ message }}</span>
              </p>
              <p class="ledger-hint">
                {{ tr('同一账号与工作负载的聚合区间不能重叠；跨查询边界的数据不会按天数伪分摊。', 'Aggregate intervals for one account and workload must not overlap; data crossing a query boundary is never prorated by invented day counts.') }}
              </p>
            </form>
          </div>
        </section>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import CostInputHint from './CostInputHint.vue'
import Icon from '@/components/icons/Icon.vue'
import CostLedgerStatus from './CostLedgerStatus.vue'
import { useCostLedger } from './useCostLedger'

const {
  tr, open, purchase, paidTime, serviceStart, serviceEnd,
  savePurchase, lastAction, formError, message, saveAllocation, allocationTarget, tiers,
  weights, tierName, saveDelivery, delivered, deliveryStart, deliveryEnd, allTiers,
  kinds, currencies, busy, connected,
} = useCostLedger()
</script>

<style scoped src="./ledger.css"></style>
