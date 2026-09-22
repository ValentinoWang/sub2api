<template>
  <section class="card p-5 md:p-6" data-testid="chrome-restock">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('ldxpToolkit.browser.title') }}</h2>
        <p class="mt-1 max-w-3xl text-sm text-gray-500 dark:text-gray-400">{{ t('ldxpToolkit.browser.description') }}</p>
      </div>
      <button type="button" class="btn btn-secondary btn-sm" :disabled="busy" @click="load">{{ t('common.refresh') }}</button>
    </div>
    <p v-if="error" role="alert" class="mt-4 text-sm text-red-600 dark:text-red-400">{{ error }}</p>
    <p v-if="notice" role="status" class="mt-4 text-sm text-emerald-700 dark:text-emerald-300">{{ notice }}</p>
    <p v-if="loading && !status" class="mt-4 text-sm text-gray-500">{{ t('common.loading') }}</p>
    <template v-if="status">
      <div v-for="device in runtimeAlerts" :key="device.id" role="alert" class="mt-4 rounded-xl border border-amber-300 bg-amber-50 p-4 text-amber-950 dark:border-amber-800 dark:bg-amber-950/30 dark:text-amber-100" data-testid="restock-runtime-alert">
        <h3 class="font-semibold">{{ t(device.runtime!.browser_verification_required ? 'ldxpToolkit.browser.verificationNeeded' : 'ldxpToolkit.browser.workerPaused') }}</h3>
        <p class="mt-2 text-sm leading-6">{{ t(device.runtime!.execution_mode === 'browser' && needsVerification(device) ? 'ldxpToolkit.browser.dedicatedBrowserVerificationHelp' : device.runtime!.browser_verification_required ? 'ldxpToolkit.browser.verificationHelp' : runtimeReason(device.runtime!.reason)) }}</p>
        <p v-if="device.runtime!.execution_mode === 'browser'" class="mt-2 text-sm font-medium" data-testid="restock-execution-mode">{{ t('ldxpToolkit.browser.dedicatedBrowserExecutor') }}</p>
        <p class="mt-2 text-xs">{{ device.name }} · {{ t('ldxpToolkit.browser.reportedAt') }}: {{ dateLabel(device.runtime!.reported_at) }}</p>
        <p v-if="device.runtime!.checked_at" class="mt-1 text-xs">{{ t('ldxpToolkit.browser.merchantCheckedAt') }}: {{ dateLabel(device.runtime!.checked_at) }}</p>
        <p v-if="device.runtime!.next_check_at" class="mt-1 text-xs">{{ t('ldxpToolkit.browser.nextCheckAt') }}: {{ dateLabel(device.runtime!.next_check_at) }}</p>
        <p v-if="!fresh(device.runtime!.reported_at)" class="mt-2 text-xs">{{ t('ldxpToolkit.browser.oldRuntimeReport') }}</p>
        <template v-if="needsVerification(device)">
          <a v-if="device.runtime!.execution_mode !== 'browser' || localExecutionPage" :href="device.runtime!.execution_mode === 'browser' ? 'http://127.0.0.1:52401/' : 'https://www.ldxp.cn/merchant/'" target="_blank" rel="noopener noreferrer" class="btn btn-secondary btn-sm mt-3" data-testid="open-merchant-verification">{{ t(device.runtime!.execution_mode === 'browser' ? 'ldxpToolkit.browser.openDedicatedBrowser' : 'ldxpToolkit.browser.openMerchantVerification') }}</a>
          <p v-else class="mt-3 text-sm" data-testid="dedicated-browser-remote-help">{{ t('ldxpToolkit.browser.dedicatedBrowserRemoteHelp') }}</p>
        </template>
        <button v-if="status.recheck_supported && needsVerification(device)" type="button" class="btn btn-primary btn-sm mt-3 sm:ml-2" :disabled="busy || recheckPending(device)" data-testid="request-merchant-recheck" @click="requestRecheck(device)">{{ t(recheckPending(device) ? 'ldxpToolkit.browser.recheckPending' : 'ldxpToolkit.browser.recheckNow') }}</button>
        <p v-if="device.recheck" class="mt-3 text-sm" data-testid="merchant-recheck-result">{{ t(recheckMessage(device)) }}</p>
      </div>
      <div class="mt-4 grid gap-3 sm:grid-cols-3" data-testid="restock-status-summary" aria-live="polite">
        <div class="rounded-xl border border-gray-200 p-4 dark:border-dark-600">
          <p class="text-xs text-gray-500">{{ t('ldxpToolkit.browser.switchStatus') }}</p>
          <p class="mt-2 text-lg font-semibold" :class="status.enabled && !error ? 'text-emerald-700 dark:text-emerald-300' : 'text-gray-500'" data-testid="restock-enabled-status">{{ t(error ? 'ldxpToolkit.browser.statusUnknown' : status.enabled ? 'ldxpToolkit.browser.enabled' : 'ldxpToolkit.browser.disabled') }}</p>
        </div>
        <div class="rounded-xl border border-gray-200 p-4 dark:border-dark-600">
          <p class="text-xs text-gray-500">{{ t('ldxpToolkit.browser.connectionStatus') }}</p>
          <p class="mt-2 text-lg font-semibold text-gray-900 dark:text-white" data-testid="restock-connection-status">{{ t(connectionLabel) }}</p>
          <p class="mt-1 text-xs text-gray-500">{{ t('ldxpToolkit.browser.lastSeen') }}: {{ dateLabel(lastSeen) }}</p>
        </div>
        <div class="rounded-xl border border-gray-200 p-4 dark:border-dark-600">
          <p class="text-xs text-gray-500">{{ t('ldxpToolkit.browser.stockStatus') }}</p>
          <p class="mt-2 text-lg font-semibold text-gray-900 dark:text-white" data-testid="restock-stock-status">{{ error ? '—' : fullProducts }} / {{ enabledProducts.length }}</p>
          <p class="mt-1 text-xs text-gray-500">{{ t('ldxpToolkit.browser.stockFreshness') }}</p>
        </div>
      </div>
      <p class="mt-3 text-xs leading-5 text-gray-500 dark:text-gray-400">{{ t('ldxpToolkit.browser.statusHelp') }}</p>
      <p v-if="status.batches[0]" class="mt-2 text-sm text-gray-600 dark:text-gray-300" data-testid="restock-last-batch">{{ t('ldxpToolkit.browser.lastBatch') }}: {{ batchProductName(status.batches[0].goods_id) }} · {{ t('ldxpToolkit.browser.codeCount', { count: status.batches[0].code_count }) }} · {{ t(status.batches[0].status === 'verified' ? 'ldxpToolkit.browser.batchVerified' : status.batches[0].status === 'claimed' ? 'ldxpToolkit.browser.batchClaimed' : 'ldxpToolkit.browser.batchUncertain') }}</p>
      <div class="mt-4 flex flex-wrap items-center gap-3 rounded-lg bg-gray-50 p-3 dark:bg-dark-800">
        <span class="badge" :class="status.enabled && !status.paused_reason ? 'badge-success' : 'badge-warning'">{{ t(status.paused_reason ? 'ldxpToolkit.browser.paused' : status.enabled ? 'ldxpToolkit.browser.enabled' : 'ldxpToolkit.browser.disabled') }}</span>
        <span v-if="status.paused_reason" class="break-all text-sm text-amber-800 dark:text-amber-200">{{ pauseLabel }}</span>
        <button type="button" class="btn btn-secondary btn-sm" :disabled="busy || (!status.enabled && dirty)" data-testid="chrome-toggle" @click="toggle">{{ t(status.enabled ? 'ldxpToolkit.browser.disable' : 'ldxpToolkit.browser.enable') }}</button>
        <button v-if="status.paused_reason" type="button" class="btn btn-secondary btn-sm" :disabled="busy || dirty" @click="resume">{{ t('ldxpToolkit.browser.resume') }}</button>
      </div>
      <div v-if="pendingBatches.length" class="mt-5 rounded-xl border border-amber-200 p-4 dark:border-amber-900/50" data-testid="chrome-pending-batches">
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('ldxpToolkit.browser.pendingBatches') }}</h3>
        <div v-for="batch in pendingBatches" :key="batch.batch_id" class="mt-3 flex flex-wrap items-center justify-between gap-2 text-sm">
          <span class="text-gray-700 dark:text-gray-300">{{ batchProductName(batch.goods_id) }} · {{ t('ldxpToolkit.browser.codeCount', { count: batch.code_count }) }}</span>
          <span class="badge badge-warning">{{ t(batch.status === 'claimed' ? 'ldxpToolkit.browser.batchClaimed' : 'ldxpToolkit.browser.batchUncertain') }}</span>
        </div>
      </div>
      <div class="mt-6 border-t border-gray-100 pt-5 dark:border-dark-700">
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('ldxpToolkit.browser.devices') }}</h3>
        <form class="mt-3 flex flex-col gap-2 sm:flex-row" @submit.prevent="createDevice">
          <input v-model.trim="deviceName" class="input min-w-0 flex-1" maxlength="80" required :disabled="busy || !!deviceKey" :placeholder="t('ldxpToolkit.browser.deviceName')" :aria-label="t('ldxpToolkit.browser.deviceName')" />
          <select v-model="deviceGoodsId" class="input sm:max-w-56" :disabled="busy || !!deviceKey" :aria-label="t('ldxpToolkit.browser.deviceScope')">
            <option value="">{{ t('ldxpToolkit.browser.allEnabledProducts') }}</option>
            <option v-for="product in status.products.filter(product => product.enabled)" :key="product.goods_id" :value="String(product.goods_id)">{{ t('purchase.productName', { amount: product.cny_amount }) }}</option>
          </select>
          <button class="btn btn-secondary btn-sm" :disabled="busy || !deviceName || !!deviceKey || !status.products.some(product => product.enabled)" data-testid="chrome-create-device">{{ t('ldxpToolkit.browser.createDevice') }}</button>
        </form>
        <div v-if="deviceKey" class="mt-3 rounded-lg bg-amber-50 p-4 dark:bg-amber-900/20" data-testid="chrome-device-key">
          <p class="text-sm text-amber-900 dark:text-amber-200">{{ t('ldxpToolkit.browser.keyHelp') }}</p>
          <textarea :value="deviceKey" readonly rows="2" class="input mt-2 break-all font-mono text-xs" :aria-label="t('ldxpToolkit.browser.deviceKey')" autocomplete="off" spellcheck="false" />
          <button type="button" class="btn btn-secondary btn-sm mt-2" @click="deviceKey = ''">{{ t('ldxpToolkit.browser.dismissKey') }}</button>
        </div>
        <p v-if="!status.devices.length" class="mt-3 text-sm text-gray-500">{{ t('ldxpToolkit.browser.emptyDevices') }}</p>
        <div v-for="device in status.devices" :key="device.id" class="mt-3 flex flex-wrap items-center justify-between gap-3 rounded-lg border border-gray-200 p-3 dark:border-dark-600">
          <div class="min-w-0 text-sm">
            <p class="break-all font-medium text-gray-900 dark:text-white">{{ device.name }}</p>
            <p class="mt-1 text-xs text-gray-500">{{ t('ldxpToolkit.browser.lastSeen') }}: {{ dateLabel(device.runtime?.reported_at || device.last_seen_at) }} · {{ t(deviceStateLabel(device)) }}</p>
            <p v-if="device.recheck && device.runtime?.state !== 'paused'" class="mt-2 text-sm" data-testid="merchant-recheck-result">{{ t(recheckMessage(device)) }}</p>
          </div>
          <button v-if="!device.revoked" type="button" class="btn btn-secondary btn-sm" :disabled="busy" @click="revokeDevice(device.id)">{{ t('ldxpToolkit.browser.revoke') }}</button>
        </div>
      </div>
      <details class="mt-6 border-t border-gray-100 pt-5 dark:border-dark-700" data-testid="restock-product-mappings">
        <summary class="cursor-pointer text-sm font-semibold text-gray-900 dark:text-white">{{ t('ldxpToolkit.browser.products') }}</summary>
        <form class="mt-4 space-y-4" @submit.prevent="save">
          <div class="flex justify-end">
            <button type="button" class="btn btn-secondary btn-sm" :disabled="busy" @click="addProduct">{{ t('ldxpToolkit.browser.addProduct') }}</button>
          </div>
          <p class="text-xs leading-5 text-gray-500 dark:text-gray-400">{{ t('ldxpToolkit.browser.mappingHelp') }}</p>
          <p v-if="!products.length" class="text-sm text-gray-500">{{ t('ldxpToolkit.browser.emptyProducts') }}</p>
          <div v-for="(product, index) in products" :key="index" class="rounded-xl border border-gray-200 p-4 dark:border-dark-600" data-testid="chrome-product">
            <div class="grid gap-3 sm:grid-cols-2">
              <label class="text-xs text-gray-600 dark:text-gray-300">{{ t('ldxpToolkit.browser.goodsId') }}<input v-model.number="product.goods_id" type="number" min="1" step="1" required :disabled="busy" class="input mt-1" /></label>
              <label class="text-xs text-gray-600 dark:text-gray-300">{{ t('ldxpToolkit.browser.amount') }}<input v-model.number="product.cny_amount" type="number" min="1" max="10000" step="1" required :disabled="busy" class="input mt-1" /></label>
              <label class="text-xs text-gray-600 dark:text-gray-300 sm:col-span-2">{{ t('ldxpToolkit.browser.buyerUrl') }}<input v-model.trim="product.external_url" type="url" required :disabled="busy" class="input mt-1" placeholder="https://…" /></label>
            </div>
            <div class="mt-3 flex flex-wrap items-center justify-between gap-3 text-xs">
              <label class="flex items-center gap-2"><input v-model="product.enabled" type="checkbox" :disabled="busy" />{{ t('ldxpToolkit.browser.productEnabled') }}</label>
              <span class="text-gray-500 dark:text-gray-400">{{ t('ldxpToolkit.browser.stock') }}: {{ product.current_stock ?? '—' }} / {{ product.target_stock }} · {{ t(product.identity_verified ? 'ldxpToolkit.browser.identityVerified' : 'ldxpToolkit.browser.identityUnknown') }}</span>
              <button type="button" class="text-red-600 hover:underline" :disabled="busy" @click="products.splice(index, 1)">{{ t('common.delete') }}</button>
            </div>
            <p class="mt-2 text-xs text-gray-500">{{ t('ldxpToolkit.browser.inventoryCheckedAt') }}: {{ dateLabel(product.inventory_at) }}</p>
          </div>
          <p v-if="dirty && !valid" class="text-xs text-amber-700 dark:text-amber-300">{{ t('ldxpToolkit.browser.invalidMappings') }}</p>
          <div class="flex flex-wrap items-center gap-3">
            <button type="submit" class="btn btn-primary btn-sm" :disabled="busy || !valid || !dirty" data-testid="chrome-save">{{ t('common.save') }}</button>
            <span v-if="dirty" class="text-xs text-gray-500">{{ t('ldxpToolkit.browser.unsaved') }}</span>
          </div>
        </form>
      </details>
    </template>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useNow } from '@vueuse/core'
import { liandongBrowserAPI, type BrowserRestockDevice, type BrowserRestockProduct, type BrowserRestockStatus } from '@/api/liandongBrowser'
import { sanitizeUrl } from '@/utils/url'

const { t } = useI18n()
const localExecutionPage = ['localhost', '127.0.0.1'].includes(window.location.hostname)
const now = useNow({ interval: 30_000 })
const status = ref<BrowserRestockStatus | null>(null)
const products = ref<BrowserRestockProduct[]>([])
const loading = ref(false)
const working = ref(false)
const error = ref('')
const notice = ref('')
const deviceName = ref('')
const deviceGoodsId = ref('')
const deviceKey = ref('')
const busy = computed(() => loading.value || working.value)
let refreshTimer: ReturnType<typeof setInterval> | undefined
let lastRefreshAt = 0
const requestedChecks = new Set<string>()
const pendingChecks = computed(() => status.value?.devices.some(device => !device.revoked && recheckPending(device)) ?? false)
const fresh = (value?: string) => {
  // A newly received report can be newer than the last periodic display tick.
  const age = Math.max(now.value.getTime(), Date.now()) - Date.parse(value || '')
  return Number.isFinite(age) && age >= 0 && age <= 120_000
}
const onlineDevices = computed(() => status.value?.devices.filter(device => !device.revoked && !device.paused_reason && fresh(device.last_seen_at) && fresh(device.authorization_verified_at)) ?? [])
const runtimeAlerts = computed(() => status.value?.devices.filter(device => !device.revoked && device.runtime?.state === 'paused') ?? [])
const liveRuntime = computed(() => status.value?.devices.filter(device => !device.revoked && device.runtime && fresh(device.runtime.reported_at)) ?? [])
const connectionLabel = computed(() => error.value ? 'ldxpToolkit.browser.statusUnknown'
  : liveRuntime.value.some(device => device.runtime?.state === 'paused') ? 'ldxpToolkit.browser.workerPaused'
    : liveRuntime.value.length || onlineDevices.value.length ? 'ldxpToolkit.browser.connected' : 'ldxpToolkit.browser.noConnection')
const enabledProducts = computed(() => status.value?.products.filter(product => product.enabled) ?? [])
const fullProducts = computed(() => error.value ? 0 : enabledProducts.value.filter(product => product.identity_verified && fresh(product.inventory_at) && (product.current_stock ?? -1) >= product.target_stock).length)
const lastSeen = computed(() => status.value?.devices.filter(device => !device.revoked).flatMap(device => [device.runtime?.reported_at, device.last_seen_at]).filter((value): value is string => !!value && Number.isFinite(Date.parse(value))).sort((a, b) => Date.parse(b) - Date.parse(a))[0])
const configProducts = computed(() => products.value.map(product => ({ goods_id: product.goods_id, cny_amount: product.cny_amount, usd_credit: product.cny_amount, external_url: product.external_url, target_stock: product.target_stock, batch_size: product.batch_size, enabled: product.enabled })))
const savedProducts = ref('[]')
const dirty = computed(() => JSON.stringify(configProducts.value) !== savedProducts.value)
const valid = computed(() => {
  const ids = new Set<number>()
  const amounts = new Set<number>()
  return configProducts.value.length <= 30 && configProducts.value.every(product => {
    const url = sanitizeUrl(product.external_url)
    if (!url) return false
    const parsed = new URL(url)
    if (parsed.protocol !== 'https:' || parsed.port || parsed.username || parsed.password
      || !(parsed.hostname === 'wzyp.cn' || parsed.hostname === 'ldxp.cn' || parsed.hostname.endsWith('.ldxp.cn'))
      || ids.has(product.goods_id) || amounts.has(product.cny_amount)) return false
    ids.add(product.goods_id)
    amounts.add(product.cny_amount)
    return Number.isSafeInteger(product.goods_id) && product.goods_id > 0
      && Number.isSafeInteger(product.cny_amount) && product.cny_amount > 0 && product.cny_amount <= 10000
      && Number.isSafeInteger(product.target_stock) && product.target_stock > 0 && product.target_stock <= 999
      && Number.isSafeInteger(product.batch_size) && product.batch_size > 0 && product.batch_size <= 20
  })
})
const pendingBatches = computed(() => status.value?.batches.filter(batch => batch.status !== 'verified') ?? [])
function batchProductName(goodsId: number) {
  const product = status.value?.products.find(product => product.goods_id === goodsId)
  return product ? t('purchase.productName', { amount: product.cny_amount }) : t('ldxpToolkit.browser.productId', { id: goodsId })
}
function deviceStateLabel(device: BrowserRestockDevice): string {
  if (device.revoked) return 'ldxpToolkit.browser.revoked'
  if (device.runtime?.state === 'paused') return 'ldxpToolkit.browser.workerPaused'
  if (device.paused_reason) return 'ldxpToolkit.browser.paused'
  if (device.last_seen_at && now.value.getTime() - Date.parse(device.last_seen_at) > 120_000) return 'ldxpToolkit.browser.offline'
  return device.authorization_verified_at ? 'ldxpToolkit.browser.authorized' : 'ldxpToolkit.browser.unverified'
}
const pauseLabel = computed(() => t('ldxpToolkit.browser.pauseHelp'))
function needsVerification(device: BrowserRestockDevice) {
  return device.runtime?.browser_verification_required || ['non_json', 'login_required', 'login_rejected', 'verification_required'].includes(device.runtime?.reason || '')
}
function runtimeReason(reason: string) {
  const labels: Record<string, string> = {
    non_json: 'upstreamPage', verification_required: 'verificationHelp',
    login_required: 'merchantLoginNeeded', login_rejected: 'merchantLoginNeeded',
    network_error: 'upstreamNetwork', upstream_unavailable: 'upstreamNetwork',
    manual: 'workerManualPause', disabled: 'workerDisabled',
    uncertain: 'workerBatchPending', recovery_wait: 'workerBatchPending',
    inventory_mismatch: 'workerInventoryMismatch',
  }
  return `ldxpToolkit.browser.${labels[reason] || 'workerNeedsCheck'}`
}
function recheckPending(device: BrowserRestockDevice) {
  return device.recheck?.state === 'queued' || device.recheck?.state === 'checking'
}
function recheckMessage(device: BrowserRestockDevice) {
  const result = device.recheck
  if (result?.state === 'queued') return 'ldxpToolkit.browser.recheckQueued'
  if (result?.state === 'checking') return 'ldxpToolkit.browser.recheckChecking'
  if (result?.state === 'passed') return result.resumed ? 'ldxpToolkit.browser.recheckPassed' : 'ldxpToolkit.browser.recheckVerifiedPaused'
  if (device.runtime?.execution_mode === 'browser' && ['verification_required', 'non_json', 'login_required', 'login_rejected'].includes(result?.reason || '')) return 'ldxpToolkit.browser.dedicatedBrowserStillBlocked'
  if (result?.reason === 'verification_required' || (result?.reason === 'non_json' && device.runtime?.browser_verification_required)) return 'ldxpToolkit.browser.recheckStillBlocked'
  return 'ldxpToolkit.browser.recheckFailed'
}
async function requestRecheck(device: BrowserRestockDevice) {
  if (recheckPending(device)) return
  await perform(async () => {
    const recheck = await liandongBrowserAPI.requestRecheck(device.id)
    requestedChecks.add(recheck.id)
    if (status.value) status.value.devices = status.value.devices.map(row => row.id === device.id ? { ...row, recheck } : row)
    notice.value = t('ldxpToolkit.browser.recheckQueued')
  })
}
function apply(next: BrowserRestockStatus, replaceProducts = true) {
  status.value = next
  for (const device of next.devices) {
    if (device.recheck && requestedChecks.has(device.recheck.id) && !recheckPending(device)) {
      notice.value = t(recheckMessage(device))
      requestedChecks.delete(device.recheck.id)
    }
  }
  if (replaceProducts) {
    products.value = next.products.map(product => ({ ...product }))
    savedProducts.value = JSON.stringify(configProducts.value)
  }
}
async function load() {
  if (busy.value) return
  loading.value = true
  lastRefreshAt = Date.now()
  try {
    apply(await liandongBrowserAPI.getStatus(), !dirty.value)
    error.value = ''
  }
  catch { error.value = t('ldxpToolkit.browser.loadError') }
  finally { loading.value = false }
}
async function perform(operation: () => Promise<void>) {
  if (busy.value) return
  working.value = true
  error.value = ''
  notice.value = ''
  try { await operation() }
  catch { error.value = t('ldxpToolkit.browser.operationError') }
  finally { working.value = false }
}
function addProduct() { products.value.push({ goods_id: 0, cny_amount: 0, usd_credit: 0, external_url: '', target_stock: 999, batch_size: 20, enabled: false }) }
async function save() {
  if (!valid.value || !status.value || !dirty.value) return
  await perform(async () => { apply(await liandongBrowserAPI.saveConfig({ enabled: status.value!.enabled, products: configProducts.value })); notice.value = t('ldxpToolkit.browser.saved') })
}
async function toggle() {
  if (!status.value || (!status.value.enabled && dirty.value)) return
  const enabled = !status.value.enabled
  const mappings = enabled ? configProducts.value : status.value.products.map(product => ({ goods_id: product.goods_id, cny_amount: product.cny_amount, usd_credit: product.usd_credit, external_url: product.external_url, target_stock: product.target_stock, batch_size: product.batch_size, enabled: product.enabled }))
  await perform(async () => { apply(await liandongBrowserAPI.saveConfig({ enabled, products: mappings }), !dirty.value) })
}
async function resume() { await perform(async () => { apply(await liandongBrowserAPI.resume()) }) }
async function createDevice() {
  if (!deviceName.value || deviceKey.value) return
  await perform(async () => {
    const result = await liandongBrowserAPI.createDevice(deviceName.value, deviceGoodsId.value ? [Number(deviceGoodsId.value)] : undefined)
    deviceKey.value = result.device_key
    deviceName.value = ''
    if (status.value) status.value.devices = [...status.value.devices, result.device]
  })
}
async function revokeDevice(id: string) {
  await perform(async () => {
    const result = await liandongBrowserAPI.revokeDevice(id)
    if (!result.revoked) throw new Error('Device revocation was not confirmed')
    if (status.value) status.value.devices = status.value.devices.map(device => device.id === id ? { ...device, revoked: true } : device)
    deviceKey.value = ''
  })
}
function dateLabel(value?: string) { return value && Number.isFinite(Date.parse(value)) ? new Date(value).toLocaleString() : t('ldxpToolkit.browser.never') }
onMounted(() => {
  void load()
  refreshTimer = setInterval(() => {
    if (!document.hidden && Date.now() - lastRefreshAt >= (pendingChecks.value ? 2_000 : 15_000)) void load()
  }, 1_000)
})
onUnmounted(() => { if (refreshTimer) clearInterval(refreshTimer) })
</script>
