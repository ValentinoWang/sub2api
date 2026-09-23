<template>
  <div class="space-y-4" data-testid="proxy-subscriptions-panel">
    <p v-if="!loading && subscriptions.length === 0" class="py-6 text-center text-sm text-gray-500 dark:text-gray-400">
      {{ t('admin.proxies.subscriptions.empty') }}
    </p>
    <section
      v-for="subscription in subscriptions"
      :key="subscription.id"
      class="rounded-2xl border border-gray-100 p-4 dark:border-dark-700"
      :data-testid="`subscription-${subscription.id}`"
    >
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div class="min-w-0">
          <div class="flex flex-wrap items-center gap-2">
            <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ subscription.name }}</h3>
            <span class="badge badge-gray">{{ t(`admin.proxies.subscriptions.formats.${subscription.format}`) }}</span>
            <span class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.proxies.subscriptions.nodes', { count: subscription.node_count }) }}</span>
          </div>
          <div class="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-xs text-gray-500 dark:text-gray-400">
            <span v-if="subscription.usage && subscription.usage.total > 0">
              {{ t('admin.proxies.subscriptions.usage', { used: formatBytes(subscription.usage.upload + subscription.usage.download), total: formatBytes(subscription.usage.total) }) }}
            </span>
            <span v-if="subscription.usage?.expire_at">{{ t('admin.proxies.subscriptions.expire', { date: formatDateOnly(subscription.usage.expire_at) }) }}</span>
            <span v-if="subscription.last_refresh_status === 'ok' && subscription.last_refresh_at">
              {{ t('admin.proxies.subscriptions.lastRefresh', { time: formatDateTime(subscription.last_refresh_at) }) }}
            </span>
            <span v-if="subscription.last_refresh_status === 'failed'" class="text-red-600 dark:text-red-400">
              {{ t('admin.proxies.subscriptions.lastRefreshFailed', { error: subscription.last_refresh_error || '' }) }}
            </span>
            <span v-if="subscription.next_refresh_at">{{ t('admin.proxies.subscriptions.nextRefresh', { time: formatDateTime(subscription.next_refresh_at) }) }}</span>
          </div>
          <p v-if="!subscription.has_url" class="mt-1 text-xs text-amber-600 dark:text-amber-400" data-testid="subscription-no-url">
            {{ t('admin.proxies.subscriptions.noUrl') }}
          </p>
        </div>
        <div class="flex items-center gap-2">
          <select
            class="input w-auto py-1.5 text-sm"
            :value="subscription.refresh_interval_minutes"
            :disabled="!subscription.has_url"
            :data-testid="`subscription-interval-${subscription.id}`"
            @change="changeInterval(subscription, Number(($event.target as HTMLSelectElement).value))"
          >
            <option v-for="option in intervalOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
          </select>
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            :disabled="!subscription.has_url || refreshing[subscription.id]"
            :data-testid="`subscription-refresh-${subscription.id}`"
            @click="refresh(subscription)"
          >
            {{ refreshing[subscription.id] ? t('admin.proxies.subscriptions.refreshing') : t('admin.proxies.subscriptions.refresh') }}
          </button>
        </div>
      </div>

      <div v-if="subscription.info.length > 0 || infoNodes(subscription).length > 0" class="mt-3 rounded-xl bg-gray-50 px-3 py-2 text-xs text-gray-600 dark:bg-dark-700/50 dark:text-gray-300">
        <span class="font-medium">{{ t('admin.proxies.subscriptions.info') }}：</span>
        {{ [...subscription.info, ...infoNodes(subscription)].join(' · ') }}
      </div>

      <div v-for="kind in groupKinds" :key="kind" class="mt-3">
        <template v-if="groupsOf(subscription, kind).length > 0">
          <div class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t(`admin.proxies.subscriptions.groupKinds.${kind}`) }}</div>
          <div class="mt-1 flex flex-wrap gap-1.5">
            <button
              v-for="group in groupsOf(subscription, kind)"
              :key="group.name"
              type="button"
              class="rounded-lg border px-2 py-1 text-xs transition-colors"
              :class="isExpanded(subscription.id, group.name) ? 'border-primary-500 bg-primary-50 text-primary-700 dark:bg-primary-900/30 dark:text-primary-300' : 'border-gray-200 text-gray-700 hover:border-primary-400 dark:border-dark-600 dark:text-gray-200'"
              :data-testid="`group-${subscription.id}-${group.name}`"
              @click="toggle(subscription.id, group.name)"
            >
              {{ group.name }} · {{ group.proxy_ids.length }}
            </button>
          </div>
          <ul
            v-for="group in groupsOf(subscription, kind).filter((g) => isExpanded(subscription.id, g.name))"
            :key="`members-${group.name}`"
            class="mt-2 grid gap-1 sm:grid-cols-2"
            :data-testid="`group-members-${subscription.id}-${group.name}`"
          >
            <li v-for="node in membersOf(subscription, group)" :key="node.proxy_id" class="flex flex-wrap items-center gap-1.5 text-xs">
              <span class="font-medium text-gray-900 dark:text-white">{{ node.meta.display_name }}</span>
              <span class="badge" :class="node.meta.residential ? 'badge-success' : 'badge-gray'">
                {{ node.meta.residential ? t('admin.proxies.subscriptions.residential') : t('admin.proxies.subscriptions.datacenter') }}
              </span>
              <span class="text-gray-500 dark:text-gray-400">{{ node.meta.multiplier }} · {{ node.meta.route }} · #{{ node.proxy_id }}</span>
            </li>
          </ul>
        </template>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { ProxySubscription, ProxySubscriptionGroup, ProxySubscriptionNode } from '@/api/admin/proxies'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatBytes, formatDateOnly, formatDateTime } from '@/utils/format'

const emit = defineEmits<{ (e: 'refreshed'): void }>()

const { t } = useI18n()
const appStore = useAppStore()

const subscriptions = ref<ProxySubscription[]>([])
const loading = ref(false)
const refreshing = reactive<Record<string, boolean>>({})
const expanded = reactive<Record<string, boolean>>({})
const groupKinds: ProxySubscriptionGroup['kind'][] = ['provider', 'purpose', 'multiplier', 'region']
const intervalOptions = [
  { value: 0, label: t('admin.proxies.subscriptions.intervals.off') },
  { value: 15, label: t('admin.proxies.subscriptions.intervals.m15') },
  { value: 30, label: t('admin.proxies.subscriptions.intervals.m30') },
  { value: 60, label: t('admin.proxies.subscriptions.intervals.m60') },
  { value: 180, label: t('admin.proxies.subscriptions.intervals.m180') },
  { value: 360, label: t('admin.proxies.subscriptions.intervals.m360') },
  { value: 720, label: t('admin.proxies.subscriptions.intervals.m720') },
  { value: 1440, label: t('admin.proxies.subscriptions.intervals.m1440') }
]

async function load() {
  loading.value = true
  try {
    subscriptions.value = await adminAPI.proxies.listSubscriptions()
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.proxies.subscriptions.loadFailed')))
  } finally {
    loading.value = false
  }
}

function groupsOf(subscription: ProxySubscription, kind: ProxySubscriptionGroup['kind']) {
  return subscription.groups.filter((group) => group.kind === kind)
}

function membersOf(subscription: ProxySubscription, group: ProxySubscriptionGroup): ProxySubscriptionNode[] {
  const byID = new Map(subscription.nodes.map((node) => [node.proxy_id, node]))
  return group.proxy_ids.map((id) => byID.get(id)).filter((node): node is ProxySubscriptionNode => !!node)
}

function infoNodes(subscription: ProxySubscription): string[] {
  return subscription.nodes.filter((node) => node.info).map((node) => node.name)
}

function isExpanded(id: string, name: string) {
  return !!expanded[`${id}\u0000${name}`]
}

function toggle(id: string, name: string) {
  const key = `${id}\u0000${name}`
  expanded[key] = !expanded[key]
}

async function refresh(subscription: ProxySubscription) {
  refreshing[subscription.id] = true
  try {
    const result = await adminAPI.proxies.refreshSubscription(subscription.id)
    appStore.showSuccess(t('admin.proxies.subscriptions.refreshSuccess', {
      node_count: result.node_count,
      created: result.created,
      reused: result.reused,
      deactivated: result.deactivated
    }))
    emit('refreshed')
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.proxies.subscriptions.refreshFailed')))
  } finally {
    refreshing[subscription.id] = false
    await load()
  }
}

async function changeInterval(subscription: ProxySubscription, minutes: number) {
  try {
    await adminAPI.proxies.updateSubscription(subscription.id, minutes)
    appStore.showSuccess(t('admin.proxies.subscriptions.intervalSaved'))
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.proxies.subscriptions.intervalFailed')))
  } finally {
    await load()
  }
}

onMounted(load)

defineExpose({ load })
</script>
