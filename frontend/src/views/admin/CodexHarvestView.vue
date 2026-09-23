<template>
  <AppLayout>
    <div class="w-full min-w-0 space-y-6 pb-8">
      <header
        class="page-header mb-0 rounded-3xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700 sm:p-6"
      >
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div class="min-w-0">
            <h1 class="page-title flex items-center gap-2 text-xl font-black text-gray-900 dark:text-white">
              <span class="inline-flex h-8 w-8 items-center justify-center rounded-xl bg-blue-50 text-blue-500 dark:bg-blue-900/30 dark:text-blue-400">
                <Icon name="key" size="sm" />
              </span>
              {{ t('admin.codexHarvest.title') }}
            </h1>
            <p class="page-description mt-1.5 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.codexHarvest.description') }}
            </p>
          </div>
          <button type="button" class="btn btn-secondary btn-sm" data-testid="harvest-refresh" :disabled="loading" @click="refreshAll">
            {{ t('admin.codexHarvest.refresh') }}
          </button>
        </div>
        <div v-if="snapshot" class="mt-4 flex flex-wrap items-center gap-2 border-t border-gray-100 pt-4 text-xs dark:border-dark-700" data-testid="harvest-status">
          <span :class="['badge', snapshot.enabled ? 'badge-success' : 'badge-gray']">
            {{ snapshot.enabled ? t('admin.codexHarvest.status.enabled') : t('admin.codexHarvest.status.disabled') }}
          </span>
          <span class="badge badge-gray">
            {{ snapshot.fail_closed ? t('admin.codexHarvest.status.failClosed') : t('admin.codexHarvest.status.failOpen') }}
          </span>
          <span v-if="snapshot.runtime.running" class="badge badge-primary">{{ t('admin.codexHarvest.status.running') }}</span>
          <span v-if="snapshot.runtime.running" class="text-gray-500 dark:text-gray-400">
            {{ t('admin.codexHarvest.status.requests', { used: snapshot.runtime.requests_used, budget: snapshot.runtime.request_budget }) }}
          </span>
          <span v-if="snapshot.runtime.current_proxy" class="text-gray-500 dark:text-gray-400">
            {{ t('admin.codexHarvest.status.currentProxy', { name: snapshot.runtime.current_proxy }) }}
          </span>
          <span v-if="snapshot.runtime.last_round_at" class="text-gray-500 dark:text-gray-400">
            {{ t('admin.codexHarvest.status.lastRound', { time: formatTime(snapshot.runtime.last_round_at) }) }}
          </span>
          <span v-if="snapshot.runtime.next_round_at && !snapshot.runtime.running" class="text-gray-500 dark:text-gray-400">
            {{ t('admin.codexHarvest.status.nextRound', { time: formatTime(snapshot.runtime.next_round_at) }) }}
          </span>
          <router-link to="/admin/settings" class="ml-auto text-primary-600 hover:underline dark:text-primary-400">
            {{ t('admin.codexHarvest.status.openSettings') }}
          </router-link>
        </div>
        <p
          v-if="idleMessage"
          class="mt-3 rounded-xl bg-amber-50 px-3 py-2 text-xs text-amber-700 dark:bg-amber-900/20 dark:text-amber-300"
          data-testid="harvest-idle"
        >
          {{ idleMessage }}
        </p>
        <p
          v-if="snapshot?.settings_error"
          class="mt-3 rounded-xl bg-red-50 px-3 py-2 text-xs text-red-700 dark:bg-red-900/20 dark:text-red-300"
        >
          {{ t('admin.codexHarvest.settingsError') }}
        </p>
      </header>

      <template v-if="snapshot">
        <section class="card" data-testid="harvest-pool">
          <div class="flex flex-wrap items-center justify-between gap-2 border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <div class="min-w-0">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.codexHarvest.pool.title') }}</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.codexHarvest.pool.description') }}</p>
            </div>
            <div class="flex items-center gap-3 text-sm">
              <span class="text-gray-500 dark:text-gray-400">{{ t('admin.codexHarvest.pool.selected', { count: draft.proxy_ids.length }) }}</span>
              <router-link to="/admin/proxies" class="text-primary-600 hover:underline dark:text-primary-400">
                {{ t('admin.codexHarvest.pool.manageLink') }}
              </router-link>
            </div>
          </div>
          <div v-if="subscriptionGroups.length > 0" class="border-b border-gray-100 px-6 py-3 dark:border-dark-700" data-testid="harvest-pool-groups">
            <div class="text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.codexHarvest.pool.groups') }}</div>
            <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.codexHarvest.pool.groupsHint') }}</p>
            <div v-for="entry in subscriptionGroups" :key="entry.subscription" class="mt-2 flex flex-wrap items-center gap-1.5">
              <span class="text-xs text-gray-500 dark:text-gray-400">{{ entry.subscription }}</span>
              <button
                v-for="group in entry.groups"
                :key="group.name"
                type="button"
                class="rounded-lg border px-2 py-1 text-xs transition-colors"
                :class="groupSelection(group.ids) === 'all' ? 'border-primary-500 bg-primary-50 text-primary-700 dark:bg-primary-900/30 dark:text-primary-300' : groupSelection(group.ids) === 'some' ? 'border-primary-300 text-primary-700 dark:text-primary-300' : 'border-gray-200 text-gray-700 hover:border-primary-400 dark:border-dark-600 dark:text-gray-200'"
                :disabled="group.ids.length === 0"
                :data-testid="`pool-group-${group.name}`"
                @click="toggleGroup(group.ids)"
              >
                {{ group.name }} · {{ group.ids.length }}
              </button>
            </div>
          </div>
          <div class="overflow-x-auto">
            <table class="w-full min-w-[640px] text-sm">
              <thead class="bg-gray-50 text-left text-xs text-gray-500 dark:bg-dark-700/50 dark:text-gray-400">
                <tr>
                  <th class="px-4 py-2">{{ t('admin.codexHarvest.pool.columns.select') }}</th>
                  <th class="px-4 py-2">{{ t('admin.codexHarvest.pool.columns.name') }}</th>
                  <th class="px-4 py-2">{{ t('admin.codexHarvest.pool.columns.address') }}</th>
                  <th class="px-4 py-2">{{ t('admin.codexHarvest.pool.columns.exit') }}</th>
                  <th class="px-4 py-2">{{ t('admin.codexHarvest.pool.columns.latency') }}</th>
                  <th class="px-4 py-2">{{ t('admin.codexHarvest.pool.columns.actions') }}</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                <tr v-for="proxy in proxies" :key="proxy.id" :data-testid="`pool-proxy-${proxy.id}`">
                  <td class="px-4 py-2">
                    <input
                      type="checkbox"
                      class="h-4 w-4 rounded border-gray-300"
                      :checked="draft.proxy_ids.includes(proxy.id)"
                      @change="toggleProxy(proxy.id)"
                    />
                  </td>
                  <td class="px-4 py-2">
                    <div class="font-medium text-gray-900 dark:text-white">{{ nodeMeta(proxy.id)?.display_name || proxy.name }}</div>
                    <div v-if="nodeMeta(proxy.id)" class="mt-0.5 flex flex-wrap gap-1 text-xs text-gray-500 dark:text-gray-400">
                      <span :class="['badge', nodeMeta(proxy.id)?.residential ? 'badge-success' : 'badge-gray']">
                        {{ nodeMeta(proxy.id)?.residential ? t('admin.codexHarvest.pool.residential') : t('admin.codexHarvest.pool.datacenter') }}
                      </span>
                      <span>{{ nodeMeta(proxy.id)?.multiplier }} · {{ nodeMeta(proxy.id)?.route }}</span>
                    </div>
                  </td>
                  <td class="px-4 py-2 font-mono text-xs text-gray-500 dark:text-gray-400">{{ proxy.protocol }}://{{ proxy.host }}:{{ proxy.port }}</td>
                  <td class="px-4 py-2 text-xs text-gray-600 dark:text-gray-300">{{ exitLabel(proxy) }}</td>
                  <td class="px-4 py-2 text-xs text-gray-600 dark:text-gray-300">{{ latencyLabel(proxy) }}</td>
                  <td class="px-4 py-2">
                    <button type="button" class="btn btn-secondary btn-sm" :disabled="exitChecks[proxy.id]?.loading" @click="checkExit(proxy.id)">
                      {{ exitChecks[proxy.id]?.loading ? t('admin.codexHarvest.pool.checking') : t('admin.codexHarvest.pool.checkExit') }}
                    </button>
                  </td>
                </tr>
                <tr v-for="member in unusableMembers" :key="`unusable-${member.proxy_id}`" :data-testid="`pool-unusable-${member.proxy_id}`" class="bg-red-50/40 dark:bg-red-900/10">
                  <td class="px-4 py-2">
                    <input type="checkbox" class="h-4 w-4 rounded border-gray-300" :checked="draft.proxy_ids.includes(member.proxy_id)" @change="toggleProxy(member.proxy_id)" />
                  </td>
                  <td class="px-4 py-2 font-medium text-gray-900 dark:text-white">{{ member.name || `#${member.proxy_id}` }}</td>
                  <td colspan="4" class="px-4 py-2 text-xs text-red-600 dark:text-red-400">
                    {{ t('admin.codexHarvest.pool.unusable') }} · {{ poolReason(member.reason) }}
                  </td>
                </tr>
                <tr v-if="proxies.length === 0 && unusableMembers.length === 0">
                  <td colspan="6" class="px-4 py-6 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('admin.codexHarvest.pool.empty') }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>

        <section class="card" data-testid="harvest-speed">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.codexHarvest.speed.title') }}</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.codexHarvest.speed.description') }}</p>
          </div>
          <div class="space-y-5 p-6">
            <div class="tabs inline-flex flex-wrap" role="tablist">
              <button
                v-for="preset in presetButtons"
                :key="preset"
                type="button"
                role="tab"
                class="tab"
                :class="draft.preset === preset ? 'tab-active' : ''"
                :aria-selected="draft.preset === preset"
                :data-testid="`preset-${preset}`"
                @click="selectPreset(preset)"
              >
                {{ t(`admin.codexHarvest.speed.presets.${preset}`) }}
              </button>
            </div>
            <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
              <label v-for="field in speedFields" :key="field" class="block">
                <span class="text-sm font-medium text-gray-900 dark:text-white">{{ t(`admin.codexHarvest.speed.fields.${field}`) }}</span>
                <input
                  type="number"
                  class="input mt-1 w-full"
                  :data-testid="`speed-${field}`"
                  :min="snapshot.bounds[field]?.min"
                  :max="snapshot.bounds[field]?.max"
                  :value="draft.speed[field]"
                  @input="setSpeed(field, ($event.target as HTMLInputElement).value)"
                />
                <span class="mt-1 block text-xs text-gray-500 dark:text-gray-400">
                  {{ t(`admin.codexHarvest.speed.hints.${field}`) }}
                  {{ t('admin.codexHarvest.speed.range', { min: snapshot.bounds[field]?.min, max: snapshot.bounds[field]?.max }) }}
                </span>
              </label>
            </div>
            <div class="flex justify-end">
              <button type="button" class="btn btn-primary" data-testid="harvest-save" :disabled="saving" @click="saveControls">
                {{ saving ? t('admin.codexHarvest.speed.saving') : t('admin.codexHarvest.speed.save') }}
              </button>
            </div>
          </div>
        </section>

        <section class="card" data-testid="harvest-accounts">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.codexHarvest.accounts.title') }}</h2>
          </div>
          <div class="overflow-x-auto">
            <table class="w-full min-w-[720px] text-sm">
              <thead class="bg-gray-50 text-left text-xs text-gray-500 dark:bg-dark-700/50 dark:text-gray-400">
                <tr>
                  <th class="px-4 py-2">{{ t('admin.codexHarvest.accounts.columns.account') }}</th>
                  <th class="px-4 py-2">{{ t('admin.codexHarvest.accounts.columns.tickets') }}</th>
                  <th class="px-4 py-2">{{ t('admin.codexHarvest.accounts.columns.hour') }}</th>
                  <th class="px-4 py-2">{{ t('admin.codexHarvest.accounts.columns.manual') }}</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                <tr v-for="account in snapshot.accounts" :key="account.id" :data-testid="`harvest-account-${account.id}`">
                  <td class="px-4 py-3 align-top">
                    <div class="font-medium text-gray-900 dark:text-white">{{ account.name }}</div>
                    <div v-if="!account.schedulable" class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.codexHarvest.accounts.notSchedulable') }}</div>
                  </td>
                  <td class="px-4 py-3 align-top">
                    <ul class="space-y-1">
                      <li v-for="ticket in account.tickets" :key="ticket.model" class="text-xs">
                        <span class="font-mono text-gray-700 dark:text-gray-200">{{ ticket.model }}</span>
                        <span v-if="ticket.ready" class="ml-2 text-emerald-600 dark:text-emerald-400">
                          {{ t('admin.codexHarvest.accounts.ready', { seconds: readySeconds(ticket) }) }}
                        </span>
                        <span v-else-if="ticket.blocked" class="ml-2 text-red-600 dark:text-red-400">{{ t('admin.codexHarvest.accounts.blocked') }}</span>
                        <span v-else class="ml-2 text-amber-600 dark:text-amber-400">{{ t('admin.codexHarvest.accounts.waiting') }}</span>
                        <span v-if="ticket.proxy_name" class="ml-2 text-gray-500 dark:text-gray-400">{{ t('admin.codexHarvest.accounts.proxy', { name: ticket.proxy_name }) }}</span>
                        <span v-if="ticket.cooldown_until" class="ml-2 text-gray-500 dark:text-gray-400">
                          {{ t('admin.codexHarvest.accounts.cooldown', { time: formatTime(ticket.cooldown_until) }) }}
                        </span>
                      </li>
                    </ul>
                  </td>
                  <td class="px-4 py-3 align-top text-xs text-gray-600 dark:text-gray-300">{{ account.hour_used }}/{{ account.hour_limit }}</td>
                  <td class="px-4 py-3 align-top">
                    <button
                      type="button"
                      class="btn btn-secondary btn-sm"
                      :data-testid="`manual-${account.id}`"
                      :disabled="account.manual?.running || !snapshot.enabled"
                      @click="openManual(account)"
                    >
                      {{ t('admin.codexHarvest.accounts.manualStart') }}
                    </button>
                    <div v-if="account.manual" class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                      <template v-if="account.manual.running">
                        {{ t('admin.codexHarvest.accounts.manualRunning', { attempts: account.manual.attempts }) }}
                      </template>
                      <template v-else>
                        {{ t('admin.codexHarvest.accounts.manualResult', { result: manualResultLabel(account.manual.result), attempts: account.manual.attempts }) }}
                      </template>
                    </div>
                  </td>
                </tr>
                <tr v-if="snapshot.accounts.length === 0">
                  <td colspan="4" class="px-4 py-6 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('admin.codexHarvest.accounts.empty') }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>

        <section class="card" data-testid="harvest-nodes">
          <div class="flex flex-wrap items-center justify-between gap-2 border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <div class="min-w-0">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.codexHarvest.nodes.title') }}</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.codexHarvest.nodes.description') }}</p>
            </div>
            <button type="button" class="btn btn-secondary btn-sm" data-testid="nodes-reset-all" :disabled="nodes.length === 0" @click="showResetAll = true">
              {{ t('admin.codexHarvest.nodes.resetAll') }}
            </button>
          </div>
          <div class="overflow-x-auto">
            <table class="w-full min-w-[900px] text-sm">
              <thead class="bg-gray-50 text-left text-xs text-gray-500 dark:bg-dark-700/50 dark:text-gray-400">
                <tr>
                  <th class="px-4 py-2">{{ t('admin.codexHarvest.nodes.columns.proxy') }}</th>
                  <th class="px-4 py-2">{{ t('admin.codexHarvest.nodes.columns.account') }}</th>
                  <th class="px-4 py-2">{{ t('admin.codexHarvest.nodes.columns.model') }}</th>
                  <th class="px-4 py-2">{{ t('admin.codexHarvest.nodes.columns.counts') }}</th>
                  <th class="px-4 py-2">{{ t('admin.codexHarvest.nodes.columns.streak') }}</th>
                  <th class="px-4 py-2">{{ t('admin.codexHarvest.nodes.columns.lastSuccess') }}</th>
                  <th class="px-4 py-2">{{ t('admin.codexHarvest.nodes.columns.cooldown') }}</th>
                  <th class="px-4 py-2">{{ t('admin.codexHarvest.nodes.columns.latency') }}</th>
                  <th class="px-4 py-2">{{ t('admin.codexHarvest.nodes.columns.lastResult') }}</th>
                  <th class="px-4 py-2">{{ t('admin.codexHarvest.nodes.columns.actions') }}</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                <tr v-for="record in nodes" :key="record.id" :data-testid="`node-${record.id}`">
                  <td class="px-4 py-2 font-medium text-gray-900 dark:text-white">{{ record.proxy_name || `#${record.proxy_id}` }}</td>
                  <td class="px-4 py-2 text-xs">{{ record.account_name || `#${record.account_id}` }}</td>
                  <td class="px-4 py-2 font-mono text-xs">{{ record.model }}</td>
                  <td class="px-4 py-2 text-xs">{{ record.successes }} / {{ record.misses }} / {{ record.network_errors }} / {{ record.account_errors }}</td>
                  <td class="px-4 py-2 text-xs">{{ record.consecutive_failures }}</td>
                  <td class="px-4 py-2 text-xs">{{ record.last_success ? formatTime(record.last_success) : '—' }}</td>
                  <td class="px-4 py-2 text-xs">{{ record.cooldown_until ? formatTime(record.cooldown_until) : '—' }}</td>
                  <td class="px-4 py-2 text-xs">{{ record.latency_ms }} ms</td>
                  <td class="px-4 py-2 text-xs">{{ resultLabel(record.last_result) }}</td>
                  <td class="px-4 py-2">
                    <button type="button" class="btn btn-secondary btn-sm" @click="resetNode(record.id)">{{ t('admin.codexHarvest.nodes.reset') }}</button>
                  </td>
                </tr>
                <tr v-if="nodes.length === 0">
                  <td colspan="10" class="px-4 py-6 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('admin.codexHarvest.nodes.empty') }}</td>
                </tr>
              </tbody>
            </table>
          </div>
          <div v-if="nodeTotal > nodePageSize" class="border-t border-gray-100 px-4 py-3 dark:border-dark-700">
            <Pagination :page="nodePage" :total="nodeTotal" :page-size="nodePageSize" @update:page="loadNodes" />
          </div>
        </section>

        <section class="card" data-testid="harvest-events">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.codexHarvest.events.title') }}</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.codexHarvest.events.description') }}</p>
          </div>
          <div class="overflow-x-auto">
            <table class="w-full min-w-[820px] text-sm">
              <thead class="bg-gray-50 text-left text-xs text-gray-500 dark:bg-dark-700/50 dark:text-gray-400">
                <tr>
                  <th class="px-4 py-2">{{ t('admin.codexHarvest.events.columns.time') }}</th>
                  <th class="px-4 py-2">{{ t('admin.codexHarvest.events.columns.account') }}</th>
                  <th class="px-4 py-2">{{ t('admin.codexHarvest.events.columns.model') }}</th>
                  <th class="px-4 py-2">{{ t('admin.codexHarvest.events.columns.proxy') }}</th>
                  <th class="px-4 py-2">{{ t('admin.codexHarvest.events.columns.result') }}</th>
                  <th class="px-4 py-2">{{ t('admin.codexHarvest.events.columns.detail') }}</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                <tr v-for="event in recentEvents" :key="event.id" data-testid="harvest-event">
                  <td class="whitespace-nowrap px-4 py-2 text-xs text-gray-500 dark:text-gray-400">{{ formatTime(event.at) }}</td>
                  <td class="px-4 py-2 text-xs">{{ event.account_name || '—' }}</td>
                  <td class="px-4 py-2 font-mono text-xs">{{ event.model || '—' }}</td>
                  <td class="px-4 py-2 text-xs">{{ event.proxy_name || '—' }}</td>
                  <td class="px-4 py-2 text-xs">
                    <span :class="['badge', event.accepted ? 'badge-success' : 'badge-gray']">{{ resultLabel(event.result) }}</span>
                    <span v-if="event.manual" class="badge badge-primary ml-1">{{ t('admin.codexHarvest.events.manual') }}</span>
                  </td>
                  <td class="px-4 py-2 text-xs text-gray-600 dark:text-gray-300">
                    {{ event.detail }}
                    <span v-if="event.http_status" class="ml-1 text-gray-400">HTTP {{ event.http_status }}</span>
                  </td>
                </tr>
                <tr v-if="recentEvents.length === 0">
                  <td colspan="6" class="px-4 py-6 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('admin.codexHarvest.events.empty') }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>
      </template>
    </div>

    <BaseDialog
      :show="manualAccount !== null"
      :title="t('admin.codexHarvest.manual.title', { name: manualAccount?.name || '' })"
      width="narrow"
      @close="manualAccount = null"
    >
      <div class="space-y-4" data-testid="manual-dialog">
        <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('admin.codexHarvest.manual.description') }}</p>
        <fieldset>
          <legend class="text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.codexHarvest.manual.models') }}</legend>
          <label v-for="model in snapshot?.models || []" :key="model" class="mt-2 flex items-center gap-2 text-sm">
            <input v-model="manualForm.models" type="checkbox" class="h-4 w-4 rounded border-gray-300" :value="model" />
            <span class="font-mono">{{ model }}</span>
          </label>
        </fieldset>
        <label class="block">
          <span class="text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.codexHarvest.manual.maxAttempts') }}</span>
          <input v-model.number="manualForm.max_attempts" type="number" min="1" max="20" class="input mt-1 w-full" data-testid="manual-attempts" />
        </label>
        <label class="block">
          <span class="text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.codexHarvest.manual.interval') }}</span>
          <input v-model.number="manualForm.interval_seconds" type="number" min="1" max="60" class="input mt-1 w-full" />
        </label>
      </div>
      <template #footer>
        <button type="button" class="btn btn-secondary" @click="manualAccount = null">{{ t('admin.codexHarvest.manual.cancel') }}</button>
        <button type="button" class="btn btn-primary" data-testid="manual-start" :disabled="manualStarting || manualForm.models.length === 0" @click="startManual">
          {{ t('admin.codexHarvest.manual.start') }}
        </button>
      </template>
    </BaseDialog>

    <ConfirmDialog
      :show="showResetAll"
      :title="t('admin.codexHarvest.nodes.resetAll')"
      :message="t('admin.codexHarvest.nodes.resetConfirm')"
      :confirm-text="t('admin.codexHarvest.nodes.resetAll')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="confirmResetAll"
      @cancel="showResetAll = false"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import { extractApiErrorMessage } from '@/utils/apiError'
import type {
  CodexHarvestAccountView,
  CodexHarvestNodeRecord,
  CodexHarvestPreset,
  CodexHarvestSnapshot,
  CodexHarvestSpeed,
  CodexHarvestSpeedField,
  CodexHarvestTicketView
} from '@/api/admin/codexHarvest'
import type { Proxy } from '@/types'
import type { ProxySubscription, ProxySubscriptionNodeMeta } from '@/api/admin/proxies'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'

const POLL_INTERVAL_MS = 5000

const { t, te } = useI18n()
const appStore = useAppStore()

const snapshot = ref<CodexHarvestSnapshot | null>(null)
const proxies = ref<Proxy[]>([])
const subscriptions = ref<ProxySubscription[]>([])
const loading = ref(false)
const saving = ref(false)
const dirty = ref(false)
const draft = reactive<{ preset: CodexHarvestPreset; speed: CodexHarvestSpeed; proxy_ids: number[] }>({
  preset: 'standard',
  speed: {
    round_interval_seconds: 20,
    probe_interval_seconds: 2,
    attempt_timeout_seconds: 25,
    cooldown_seconds: 60,
    max_requests_per_round: 8,
    max_proxy_attempts: 3,
    max_requests_per_account_hour: 60
  },
  proxy_ids: []
})
const exitChecks = reactive<Record<number, { loading: boolean; label?: string }>>({})
const nodes = ref<CodexHarvestNodeRecord[]>([])
const nodePage = ref(1)
const nodeTotal = ref(0)
const nodePageSize = 20
const showResetAll = ref(false)
const manualAccount = ref<CodexHarvestAccountView | null>(null)
const manualStarting = ref(false)
const manualForm = reactive<{ models: string[]; max_attempts: number; interval_seconds: number }>({
  models: [],
  max_attempts: 6,
  interval_seconds: 5
})
let pollTimer: ReturnType<typeof setInterval> | null = null

const speedFields: CodexHarvestSpeedField[] = [
  'round_interval_seconds',
  'probe_interval_seconds',
  'attempt_timeout_seconds',
  'cooldown_seconds',
  'max_requests_per_round',
  'max_proxy_attempts',
  'max_requests_per_account_hour'
]

const presetButtons = computed<CodexHarvestPreset[]>(() => [
  ...((snapshot.value?.preset_order || ['slow', 'standard', 'fast', 'burst']) as CodexHarvestPreset[]),
  'custom'
])

const idleMessage = computed(() => {
  const reason = snapshot.value?.enabled === false ? 'disabled' : snapshot.value?.runtime.idle_reason
  if (!reason || reason === 'all_ready') return ''
  const key = `admin.codexHarvest.idle.${reason}`
  return te(key) ? t(key) : ''
})

const unusableMembers = computed(() => {
  const listed = new Set(proxies.value.map((p) => p.id))
  return (snapshot.value?.pool || []).filter((m) => !m.usable && !listed.has(m.proxy_id))
})

const recentEvents = computed(() => [...(snapshot.value?.events || [])].reverse())

const metaByProxy = computed(() => {
  const map = new Map<number, ProxySubscriptionNodeMeta>()
  for (const subscription of subscriptions.value) {
    for (const node of subscription.nodes) {
      if (!node.info) map.set(node.proxy_id, node.meta)
    }
  }
  return map
})

// Only proxies that are currently active can join the pool from a group.
const subscriptionGroups = computed(() => {
  const active = new Set(proxies.value.map((p) => p.id))
  return subscriptions.value
    .map((subscription) => ({
      subscription: subscription.name,
      groups: subscription.groups
        .filter((group) => group.kind !== 'multiplier')
        .map((group) => ({ name: group.name, ids: group.proxy_ids.filter((id) => active.has(id)) }))
    }))
    .filter((entry) => entry.groups.length > 0)
})

function nodeMeta(id: number) {
  return metaByProxy.value.get(id)
}

function groupSelection(ids: number[]): 'all' | 'some' | 'none' {
  const selected = ids.filter((id) => draft.proxy_ids.includes(id)).length
  if (selected === 0) return 'none'
  return selected === ids.length ? 'all' : 'some'
}

function toggleGroup(ids: number[]) {
  if (groupSelection(ids) === 'all') {
    draft.proxy_ids = draft.proxy_ids.filter((id) => !ids.includes(id))
  } else {
    draft.proxy_ids = [...draft.proxy_ids, ...ids.filter((id) => !draft.proxy_ids.includes(id))]
  }
  dirty.value = true
}

function applyControls(value: CodexHarvestSnapshot) {
  draft.preset = value.controls.preset
  draft.speed = { ...value.controls.speed }
  draft.proxy_ids = [...value.controls.proxy_ids]
  dirty.value = false
}

async function loadSnapshot(silent = false) {
  try {
    const value = await adminAPI.codexHarvest.getSnapshot()
    snapshot.value = value
    if (!dirty.value) applyControls(value)
  } catch (err: unknown) {
    if (!silent) appStore.showError(extractApiErrorMessage(err, t('admin.codexHarvest.loadError')))
  }
}

async function loadProxies() {
  try {
    const [list, subs] = await Promise.all([
      adminAPI.proxies.getAllWithCount(),
      adminAPI.proxies.listSubscriptions().catch(() => [] as ProxySubscription[])
    ])
    proxies.value = list
    subscriptions.value = subs
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.codexHarvest.loadError')))
  }
}

async function loadNodes(page = nodePage.value) {
  try {
    const result = await adminAPI.codexHarvest.listNodes(page, nodePageSize)
    nodes.value = result.items || []
    nodeTotal.value = result.total
    nodePage.value = page
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.codexHarvest.loadError')))
  }
}

async function refreshAll() {
  loading.value = true
  try {
    await Promise.all([loadSnapshot(), loadProxies(), loadNodes()])
  } finally {
    loading.value = false
  }
}

function toggleProxy(id: number) {
  const index = draft.proxy_ids.indexOf(id)
  if (index >= 0) draft.proxy_ids.splice(index, 1)
  else draft.proxy_ids.push(id)
  dirty.value = true
}

function selectPreset(preset: CodexHarvestPreset) {
  draft.preset = preset
  const values = snapshot.value?.presets[preset]
  if (values) draft.speed = { ...values }
  dirty.value = true
}

function setSpeed(field: CodexHarvestSpeedField, raw: string) {
  const value = Number.parseInt(raw, 10)
  draft.speed = { ...draft.speed, [field]: Number.isFinite(value) ? value : 0 }
  const presets = snapshot.value?.presets || {}
  const match = Object.keys(presets).find((name) =>
    speedFields.every((key) => presets[name][key] === draft.speed[key])
  )
  draft.preset = (match as CodexHarvestPreset | undefined) || 'custom'
  dirty.value = true
}

async function saveControls() {
  saving.value = true
  try {
    await adminAPI.codexHarvest.updateControls({
      version: 1,
      preset: draft.preset,
      speed: { ...draft.speed },
      proxy_ids: [...draft.proxy_ids]
    })
    dirty.value = false
    appStore.showSuccess(t('admin.codexHarvest.speed.saved'))
    await loadSnapshot(true)
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.codexHarvest.speed.saveError')))
  } finally {
    saving.value = false
  }
}

async function checkExit(id: number) {
  exitChecks[id] = { loading: true }
  try {
    const result = await adminAPI.proxies.testProxy(id)
    const location = [result.country, result.city].filter(Boolean).join(' ')
    exitChecks[id] = {
      loading: false,
      label: result.success ? [result.ip_address, location].filter(Boolean).join(' · ') : t('admin.codexHarvest.pool.checkFailed')
    }
  } catch {
    exitChecks[id] = { loading: false, label: t('admin.codexHarvest.pool.checkFailed') }
  }
}

function exitLabel(proxy: Proxy): string {
  const checked = exitChecks[proxy.id]?.label
  if (checked) return checked
  const location = [proxy.country, proxy.city].filter(Boolean).join(' ')
  const label = [proxy.ip_address, location].filter(Boolean).join(' · ')
  return label || t('admin.codexHarvest.pool.unknownExit')
}

function latencyLabel(proxy: Proxy): string {
  return typeof proxy.latency_ms === 'number' ? `${proxy.latency_ms} ms` : '—'
}

function poolReason(reason?: string): string {
  const key = `admin.codexHarvest.pool.reasons.${reason}`
  return reason && te(key) ? t(key) : reason || ''
}

function resultLabel(result?: string): string {
  if (!result) return '—'
  const key = `admin.codexHarvest.results.${result}`
  return te(key) ? t(key) : result
}

function manualResultLabel(result?: string): string {
  if (!result) return '—'
  const key = `admin.codexHarvest.manual.results.${result}`
  return te(key) ? t(key) : result
}

function readySeconds(ticket: CodexHarvestTicketView): number {
  return ticket.remaining_seconds
}

function formatTime(value?: string | null): string {
  if (!value) return '—'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '—' : date.toLocaleTimeString()
}

function openManual(account: CodexHarvestAccountView) {
  manualForm.models = [...(snapshot.value?.models || [])]
  manualForm.max_attempts = 6
  manualForm.interval_seconds = 5
  manualAccount.value = account
}

async function startManual() {
  if (!manualAccount.value) return
  manualStarting.value = true
  try {
    await adminAPI.codexHarvest.startManual(manualAccount.value.id, {
      models: [...manualForm.models],
      max_attempts: manualForm.max_attempts,
      interval_seconds: manualForm.interval_seconds
    })
    appStore.showSuccess(t('admin.codexHarvest.manual.started'))
    manualAccount.value = null
    await loadSnapshot(true)
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.codexHarvest.manual.error')))
  } finally {
    manualStarting.value = false
  }
}

async function resetNode(id: number) {
  try {
    await adminAPI.codexHarvest.resetNodes(id)
    appStore.showSuccess(t('admin.codexHarvest.nodes.resetDone'))
    await loadNodes()
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.codexHarvest.nodes.resetError')))
  }
}

async function confirmResetAll() {
  showResetAll.value = false
  await resetNode(0)
}

onMounted(async () => {
  await refreshAll()
  pollTimer = setInterval(() => {
    if (typeof document === 'undefined' || document.visibilityState !== 'hidden') {
      void loadSnapshot(true)
    }
  }, POLL_INTERVAL_MS)
})

onUnmounted(() => {
  if (pollTimer) clearInterval(pollTimer)
})
</script>
