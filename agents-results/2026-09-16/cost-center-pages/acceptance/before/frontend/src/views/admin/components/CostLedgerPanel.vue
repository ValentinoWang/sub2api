<template>
  <section class="space-y-4" aria-labelledby="ledger-title">
    <!-- 账本标题与连接状态 -->
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

    <!-- 账期汇总 -->
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
          </div>
          <div>
            <label class="ledger-label" for="ledger-end">{{ tr('账期止（UTC，不含当日）', 'End (UTC, exclusive)') }}</label>
            <input id="ledger-end" v-model="filters.end" class="input" type="date" required />
          </div>
          <div>
            <label class="ledger-label" for="ledger-currency">{{ tr('币种', 'Currency') }}</label>
            <select id="ledger-currency" v-model="filters.currency" class="input">
              <option v-for="c in currencies" :key="c">{{ c }}</option>
            </select>
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

    <!-- 录入 -->
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
                </div>
                <div>
                  <label class="ledger-label" for="purchase-asset">{{ tr('账号 / 主机', 'Account / host') }}</label>
                  <input id="purchase-asset" v-model="purchase.asset" class="input" required />
                </div>
                <div>
                  <label class="ledger-label" for="purchase-tier">{{ tr('费用归属', 'Tier') }}</label>
                  <select id="purchase-tier" v-model="purchase.tier" class="input">
                    <option v-for="t in allTiers" :key="t" :value="t">{{ tierName(t) }}</option>
                  </select>
                </div>
                <div>
                  <label class="ledger-label" for="purchase-kind">{{ tr('费用类别', 'Expense category') }}</label>
                  <select id="purchase-kind" v-model="purchase.kind" class="input">
                    <option v-for="k in kinds" :key="k.value" :value="k.value">{{ tr(k.zh, k.en) }}</option>
                  </select>
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
                </div>
                <div>
                  <label class="ledger-label" for="purchase-paid-at">{{ tr('付款时间（UTC）', 'Paid time (UTC)') }}</label>
                  <input id="purchase-paid-at" v-model="paidTime" class="input" type="datetime-local" required />
                </div>
                <div>
                  <label class="ledger-label" for="purchase-service-start">
                    {{ tr('服务起（UTC，含当日）', 'Service start (UTC, inclusive)') }}
                  </label>
                  <input id="purchase-service-start" v-model="serviceStart" class="input" type="date" required />
                </div>
                <div>
                  <label class="ledger-label" for="purchase-service-end">
                    {{ tr('服务止（UTC，不含当日）', 'Service end (UTC, exclusive)') }}
                  </label>
                  <input id="purchase-service-end" v-model="serviceEnd" class="input" type="date" required />
                </div>
                <div>
                  <label class="ledger-label" for="purchase-evidence">{{ tr('证据类型', 'Evidence') }}</label>
                  <select id="purchase-evidence" v-model="purchase.evidence" class="input">
                    <option value="manual">{{ tr('人工录入，未核验', 'Manual, unverified') }}</option>
                    <option value="invoice">{{ tr('附凭据引用，待对账', 'Invoice reference, to reconcile') }}</option>
                    <option value="synthetic">{{ tr('合成测试数据', 'Synthetic test data') }}</option>
                  </select>
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
                </div>
                <div>
                  <label class="ledger-label" for="delivery-asset">{{ tr('账号标识', 'Account reference') }}</label>
                  <input id="delivery-asset" v-model="delivered.asset" class="input" required />
                </div>
                <div>
                  <label class="ledger-label" for="delivery-tier">{{ tr('档位', 'Tier') }}</label>
                  <select id="delivery-tier" v-model="delivered.tier" class="input">
                    <option v-for="t in tiers" :key="t" :value="t">{{ tierName(t) }}</option>
                  </select>
                </div>
                <div>
                  <label class="ledger-label" for="delivery-model">{{ tr('模型 / 工作负载', 'Model / workload') }}</label>
                  <input id="delivery-model" v-model="delivered.model" class="input" required />
                </div>
                <div>
                  <label class="ledger-label" for="delivery-unit">{{ tr('单位', 'Unit') }}</label>
                  <input id="delivery-unit" v-model="delivered.unit" class="input" required />
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
                </div>
                <div>
                  <label class="ledger-label" for="delivery-start">{{ tr('区间起（UTC，含当日）', 'Start (UTC, inclusive)') }}</label>
                  <input id="delivery-start" v-model="deliveryStart" class="input" type="date" required />
                </div>
                <div>
                  <label class="ledger-label" for="delivery-end">{{ tr('区间止（UTC，不含当日）', 'End (UTC, exclusive)') }}</label>
                  <input id="delivery-end" v-model="deliveryEnd" class="input" type="date" required />
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

    <!-- 记录与更正 -->
    <div class="card overflow-hidden">
      <div
        class="flex flex-col gap-3 border-b border-gray-100 p-4 dark:border-dark-700 sm:flex-row sm:items-start sm:justify-between sm:p-5 md:p-6"
      >
        <div class="min-w-0">
          <h3 class="ledger-title">{{ tr('记录与更正', 'Records and corrections') }}</h3>
          <p class="ledger-desc">
            {{ tr('记录按顺序追加。作废只用于更正误录，不是退款，也不会删掉原记录；分摊依赖仍由服务端检查。', 'Records are appended in order. Voiding only corrects a booking mistake — it is not a refund and does not delete the original; allocation dependencies stay server-validated.') }}
          </p>
        </div>
        <span v-if="loaded && events.length" class="badge badge-gray shrink-0 self-start tabular-nums">{{ countLabel }}</span>
      </div>

      <div v-if="events.length" class="overflow-x-auto">
        <table class="table min-w-[880px]">
          <thead>
            <tr>
              <th scope="col">{{ tr('记录', 'Record') }}</th>
              <th scope="col">{{ tr('动作', 'Action') }}</th>
              <th scope="col">{{ tr('记录时间', 'Recorded at') }}</th>
              <th scope="col">{{ tr('摘要', 'Summary') }}</th>
              <th scope="col" class="text-right">{{ tr('操作', 'Actions') }}</th>
            </tr>
          </thead>
          <tbody>
            <template v-for="event in events" :key="event.id">
              <tr :class="voidTarget && voidTarget.id === event.id ? 'bg-amber-50/70 dark:bg-amber-950/20' : ''">
                <td>
                  <span class="block font-mono text-xs text-gray-500 dark:text-dark-400">#{{ event.sequence }}</span>
                  <span class="mt-0.5 block break-all font-mono text-xs text-gray-700 dark:text-gray-300">{{ event.id }}</span>
                </td>
                <td>
                  <span class="badge" :class="actionTone(event.command.kind)" :title="event.command.kind">
                    {{ actionLabel(event.command.kind) }}
                  </span>
                </td>
                <td class="whitespace-nowrap text-xs text-gray-600 dark:text-gray-300">
                  <time :datetime="event.recorded_at" :title="event.recorded_at">{{ utcLabel(event.recorded_at, true) }}</time>
                </td>
                <td class="text-xs text-gray-600 dark:text-gray-300">
                  <span class="line-clamp-2 break-all">{{ summaryLine(event) }}</span>
                </td>
                <td>
                  <div class="flex items-center justify-end gap-1.5">
                    <button
                      type="button"
                      class="btn btn-secondary btn-sm"
                      :aria-expanded="!!expandedIds[event.id]"
                      :aria-controls="`ledger-event-${event.id}`"
                      @click="toggleDetail(event.id)"
                    >
                      <Icon
                        name="chevronRight"
                        size="xs"
                        class="ledger-chevron"
                        :class="expandedIds[event.id] ? 'rotate-90' : ''"
                      />
                      {{ tr('详情', 'Details') }}
                    </button>
                    <button
                      v-if="event.command.kind !== 'void'"
                      type="button"
                      class="btn btn-secondary btn-sm"
                      :disabled="busy"
                      :aria-expanded="!!voidTarget && voidTarget.id === event.id"
                      aria-controls="ledger-void-form"
                      @click="startVoid(event)"
                    >
                      {{ tr('更正', 'Correct') }}
                    </button>
                  </div>
                </td>
              </tr>
              <tr v-if="expandedIds[event.id]">
                <td :id="`ledger-event-${event.id}`" colspan="5" class="bg-gray-50 dark:bg-dark-900/40">
                  <dl class="grid gap-x-6 gap-y-2 text-xs sm:grid-cols-3">
                    <div class="min-w-0">
                      <dt class="text-gray-500 dark:text-dark-400">{{ tr('幂等键', 'Idempotency key') }}</dt>
                      <dd class="mt-0.5 break-all font-mono text-gray-700 dark:text-gray-300">{{ event.idempotency_key }}</dd>
                    </div>
                    <div class="min-w-0">
                      <dt class="text-gray-500 dark:text-dark-400">{{ tr('请求摘要', 'Request hash') }}</dt>
                      <dd class="mt-0.5 break-all font-mono text-gray-700 dark:text-gray-300">{{ event.request_hash }}</dd>
                    </div>
                    <div class="min-w-0">
                      <dt class="text-gray-500 dark:text-dark-400">{{ tr('操作人 ID', 'Actor ID') }}</dt>
                      <dd class="mt-0.5 font-mono text-gray-700 dark:text-gray-300">{{ event.actor_id }}</dd>
                    </div>
                  </dl>
                  <pre
                    class="mt-3 max-h-64 overflow-auto rounded-lg border border-gray-200 bg-white p-3 font-mono text-[11px] leading-5 text-gray-700 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-300"
                  >{{ JSON.stringify(event.command, null, 2) }}</pre>
                </td>
              </tr>
            </template>
          </tbody>
        </table>
      </div>

      <div v-else class="ledger-empty">
        <span class="ledger-empty-icon"><Icon name="inbox" size="lg" /></span>
        <p class="text-sm font-medium text-gray-900 dark:text-white">
          {{ connected ? tr('账本里还没有记录', 'No records yet') : tr('账本未连接，读不到记录', 'Not connected, records unavailable') }}
        </p>
        <p class="max-w-md text-xs leading-5 text-gray-500 dark:text-gray-400">
          {{ connected
            ? tr('在上面录入第一笔采购、分摊或交付，记录会按追加顺序显示在这里。', 'Record the first purchase, allocation or delivery above; entries appear here in append order.')
            : tr('这里不会显示占位数据。接口恢复后点右上角“刷新”。', 'No placeholder data is shown here. Use Refresh at the top once the endpoint is reachable.') }}
        </p>
      </div>

      <div
        v-if="events.length || (lastAction === 'void' && (formError || message))"
        class="flex flex-wrap items-center gap-x-3 gap-y-2 border-t border-gray-100 p-4 dark:border-dark-700 sm:px-5 md:px-6"
      >
        <button v-if="nextAfter !== null" type="button" class="btn btn-secondary btn-sm" :disabled="busy" @click="loadMore">
          {{ tr('加载更多记录', 'Load more') }}
        </button>
        <span v-else-if="events.length" class="text-xs text-gray-500 dark:text-gray-400">
          {{ tr('已加载全部记录。', 'All records loaded.') }}
        </span>
        <p v-if="listError" role="alert" class="text-xs leading-5 text-red-600 dark:text-red-400">{{ listError }}</p>
        <p v-if="lastAction === 'void' && formError" role="alert" class="text-xs leading-5 text-red-600 dark:text-red-400">
          {{ formError }}
        </p>
        <p
          v-else-if="lastAction === 'void' && message"
          aria-live="polite"
          class="text-xs leading-5 text-emerald-700 dark:text-emerald-300"
        >
          {{ message }}
        </p>
      </div>

      <div
        v-if="voidTarget"
        id="ledger-void-form"
        class="border-t border-amber-200 bg-amber-50/50 p-4 dark:border-amber-900/50 dark:bg-amber-950/10 sm:p-5 md:p-6"
      >
        <form class="space-y-3" @submit.prevent="saveVoid">
          <div class="min-w-0">
            <p class="text-sm font-medium text-gray-900 dark:text-white">{{ tr('更正误录', 'Correct a booking mistake') }}</p>
            <p class="mt-1 break-all text-xs leading-5 text-gray-600 dark:text-gray-300">
              {{ tr('将对这条记录追加一条作废：', 'A void will be appended for: ') }}
              <span class="font-mono">{{ voidTarget.id }}</span>
              · {{ summaryLine(voidTarget) }}
            </p>
            <p class="mt-1 text-xs leading-5 text-amber-800 dark:text-amber-200">
              {{ tr('作废不是退款，也不会删掉原记录。', 'A void is not a refund and does not delete the original record.') }}
            </p>
          </div>
          <div class="max-w-xl">
            <label class="ledger-label" for="ledger-void-reason">{{ tr('误录原因', 'Reason') }}</label>
            <input
              id="ledger-void-reason"
              v-model="voidReason"
              class="input"
              required
              maxlength="240"
              aria-describedby="ledger-void-reason-hint"
            />
            <p id="ledger-void-reason-hint" class="ledger-hint">
              {{ tr('写清楚错在哪里，不要填写敏感信息。', 'State what was wrong; no sensitive information.') }}
            </p>
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <button class="btn btn-primary btn-sm" :disabled="busy">{{ tr('确认作废', 'Confirm void') }}</button>
            <button type="button" class="btn btn-secondary btn-sm" @click="cancelVoid">{{ tr('取消', 'Cancel') }}</button>
          </div>
        </form>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { appendCostLedger, getCostLedgerHealth, getCostLedgerSummary, listCostLedgerEvents } from '@/api/admin/cost-center'
import type { CostDelivery, CostPurchase, CostTier, LedgerCommand, LedgerEvent, LedgerSummary } from '@/api/admin/cost-center'
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
  void: ['作废', 'Void']
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
  return [command.void.target_id, command.void.reason].filter(Boolean).join(' · ')
}
// UTC 展示，原始值保留在 title/datetime 中。
function utcLabel(value: string, withSeconds = false): string {
  const ms = Date.parse(value)
  if (!Number.isFinite(ms)) return value
  const iso = new Date(ms).toISOString()
  return `${iso.slice(0, 10)} ${iso.slice(11, withSeconds ? 19 : 16)} UTC`
}
onMounted(refresh)
</script>

<style scoped>
.ledger-title {
  @apply text-sm font-semibold text-gray-900 dark:text-white sm:text-base;
}

.ledger-desc {
  @apply mt-1 text-xs leading-5 text-gray-500 dark:text-gray-400 sm:text-sm sm:leading-6;
}

.ledger-label {
  @apply mb-1.5 block text-xs font-medium text-gray-600 dark:text-gray-400;
}

.ledger-hint {
  @apply mt-1 text-xs leading-5 text-gray-500 dark:text-dark-400;
}

.ledger-disclosure {
  @apply flex w-full items-start gap-3 px-4 py-3.5 text-left transition-colors sm:px-5 md:px-6;
  @apply hover:bg-gray-50 dark:hover:bg-dark-700/40;
  @apply focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary-500/60;
}

.ledger-alert-error {
  @apply flex items-start gap-2 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-xs leading-5 text-red-700;
  @apply dark:border-red-900/50 dark:bg-red-950/30 dark:text-red-300;
}

.ledger-alert-ok {
  @apply flex items-start gap-2 rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-xs leading-5 text-emerald-800;
  @apply dark:border-emerald-900/50 dark:bg-emerald-950/30 dark:text-emerald-300;
}

.ledger-empty {
  @apply flex flex-col items-center gap-2 rounded-xl border border-dashed border-gray-300 px-4 py-10 text-center;
  @apply dark:border-dark-600;
}

.ledger-empty-icon {
  @apply mb-1 flex h-12 w-12 items-center justify-center rounded-2xl bg-gray-100 text-gray-400;
  @apply dark:bg-dark-700 dark:text-dark-400;
}

.ledger-chevron {
  @apply transition-transform duration-200;
}

@media (prefers-reduced-motion: reduce) {
  .ledger-chevron,
  .ledger-disclosure {
    transition: none;
  }

  .ledger-refresh-icon {
    animation: none;
  }
}
</style>
