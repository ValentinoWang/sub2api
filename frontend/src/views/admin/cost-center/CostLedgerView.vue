<template>
  <div class="min-w-0 space-y-5">
    <CostLedgerStatus />
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
  </div>
</template>

<script setup lang="ts">
import Icon from '@/components/icons/Icon.vue'
import CostLedgerStatus from './CostLedgerStatus.vue'
import { useCostLedger } from './useCostLedger'

const {
  tr, busy, connected, loaded, events, nextAfter, loadMore,
  countLabel, listError, message, formError, lastAction, voidTarget, voidReason,
  saveVoid, startVoid, cancelVoid, expandedIds, toggleDetail, summaryLine, actionTone,
  actionLabel, utcLabel,
} = useCostLedger()
</script>

<style scoped src="./ledger.css"></style>
