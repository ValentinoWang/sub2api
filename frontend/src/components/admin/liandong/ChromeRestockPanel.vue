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
      <div class="mt-4 flex flex-wrap items-center gap-3 rounded-lg bg-gray-50 p-3 dark:bg-dark-800">
        <span class="badge" :class="status.enabled && !status.paused_reason ? 'badge-success' : 'badge-warning'">{{ t(status.paused_reason ? 'ldxpToolkit.browser.paused' : status.enabled ? 'ldxpToolkit.browser.enabled' : 'ldxpToolkit.browser.disabled') }}</span>
        <span v-if="status.paused_reason" class="break-all text-sm text-amber-800 dark:text-amber-200">{{ pauseLabel }}</span>
        <button type="button" class="btn btn-secondary btn-sm" :disabled="busy || (!status.enabled && dirty)" data-testid="chrome-toggle" @click="toggle">{{ t(status.enabled ? 'ldxpToolkit.browser.disable' : 'ldxpToolkit.browser.enable') }}</button>
        <button v-if="status.paused_reason" type="button" class="btn btn-secondary btn-sm" :disabled="busy || dirty" @click="resume">{{ t('ldxpToolkit.browser.resume') }}</button>
      </div>
      <p class="mt-3 text-sm leading-6 text-gray-500 dark:text-gray-400">{{ t('ldxpToolkit.browser.installHelp') }}</p>
      <a href="/assets/ldxp-browser-extension.zip" download class="mt-2 inline-flex text-sm font-medium text-primary-600 hover:underline dark:text-primary-400">{{ t('ldxpToolkit.browser.download') }}</a>
      <form class="mt-5 space-y-4" @submit.prevent="save">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('ldxpToolkit.browser.products') }}</h3>
          <button type="button" class="btn btn-secondary btn-sm" :disabled="busy" @click="addProduct">{{ t('ldxpToolkit.browser.addProduct') }}</button>
        </div>
        <p class="text-xs leading-5 text-gray-500 dark:text-gray-400">{{ t('ldxpToolkit.browser.mappingHelp') }}</p>
        <p v-if="!products.length" class="text-sm text-gray-500">{{ t('ldxpToolkit.browser.emptyProducts') }}</p>
        <div v-for="(product, index) in products" :key="index" class="rounded-xl border border-gray-200 p-4 dark:border-dark-600" data-testid="chrome-product">
          <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            <label class="text-xs text-gray-600 dark:text-gray-300">{{ t('ldxpToolkit.browser.goodsId') }}<input v-model.number="product.goods_id" type="number" min="1" step="1" required :disabled="busy" class="input mt-1" /></label>
            <label class="text-xs text-gray-600 dark:text-gray-300">{{ t('ldxpToolkit.browser.amount') }}<input v-model.number="product.cny_amount" type="number" min="1" max="10000" step="1" required :disabled="busy" class="input mt-1" /></label>
            <label class="text-xs text-gray-600 dark:text-gray-300">{{ t('ldxpToolkit.browser.target') }}<input v-model.number="product.target_stock" type="number" min="1" max="100" step="1" required :disabled="busy" class="input mt-1" /></label>
            <label class="text-xs text-gray-600 dark:text-gray-300">{{ t('ldxpToolkit.browser.batchSize') }}<input v-model.number="product.batch_size" type="number" min="1" max="20" step="1" required :disabled="busy" class="input mt-1" /></label>
            <label class="text-xs text-gray-600 dark:text-gray-300 sm:col-span-2 lg:col-span-4">{{ t('ldxpToolkit.browser.buyerUrl') }}<input v-model.trim="product.external_url" type="url" required :disabled="busy" class="input mt-1" placeholder="https://…" /></label>
          </div>
          <div class="mt-3 flex flex-wrap items-center justify-between gap-3 text-xs">
            <label class="flex items-center gap-2"><input v-model="product.enabled" type="checkbox" :disabled="busy" />{{ t('ldxpToolkit.browser.productEnabled') }}</label>
            <span class="text-gray-500 dark:text-gray-400">{{ t('ldxpToolkit.browser.stock') }}: {{ product.current_stock ?? '—' }} · {{ t(product.identity_verified ? 'ldxpToolkit.browser.identityVerified' : 'ldxpToolkit.browser.identityUnknown') }}</span>
            <button type="button" class="text-red-600 hover:underline" :disabled="busy" @click="products.splice(index, 1)">{{ t('common.delete') }}</button>
          </div>
        </div>
        <p v-if="dirty && !valid" class="text-xs text-amber-700 dark:text-amber-300">{{ t('ldxpToolkit.browser.invalidMappings') }}</p>
        <div class="flex flex-wrap items-center gap-3">
          <button type="submit" class="btn btn-primary btn-sm" :disabled="busy || !valid || !dirty" data-testid="chrome-save">{{ t('common.save') }}</button>
          <span v-if="dirty" class="text-xs text-gray-500">{{ t('ldxpToolkit.browser.unsaved') }}</span>
        </div>
      </form>
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
          <div class="min-w-0 text-sm"><p class="break-all font-medium text-gray-900 dark:text-white">{{ device.name }}</p><p class="mt-1 text-xs text-gray-500">{{ t('ldxpToolkit.browser.lastSeen') }}: {{ dateLabel(device.last_seen_at) }} · {{ t(deviceStateLabel(device)) }}</p></div>
          <button v-if="!device.revoked" type="button" class="btn btn-secondary btn-sm" :disabled="busy" @click="revokeDevice(device.id)">{{ t('ldxpToolkit.browser.revoke') }}</button>
        </div>
      </div>
    </template>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useNow } from '@vueuse/core'
import { liandongBrowserAPI, type BrowserRestockDevice, type BrowserRestockProduct, type BrowserRestockStatus } from '@/api/liandongBrowser'
import { sanitizeUrl } from '@/utils/url'

const { t } = useI18n()
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
      && Number.isSafeInteger(product.target_stock) && product.target_stock > 0 && product.target_stock <= 100
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
  if (device.paused_reason) return 'ldxpToolkit.browser.paused'
  if (device.last_seen_at && now.value.getTime() - Date.parse(device.last_seen_at) > 120_000) return 'ldxpToolkit.browser.offline'
  return device.authorization_verified_at ? 'ldxpToolkit.browser.authorized' : 'ldxpToolkit.browser.unverified'
}
const pauseLabel = computed(() => t('ldxpToolkit.browser.pauseHelp'))
function apply(next: BrowserRestockStatus, replaceProducts = true) {
  status.value = next
  if (replaceProducts) {
    products.value = next.products.map(product => ({ ...product }))
    savedProducts.value = JSON.stringify(configProducts.value)
  }
}
async function load() {
  if (busy.value) return
  loading.value = true
  error.value = ''
  try { apply(await liandongBrowserAPI.getStatus(), !dirty.value) }
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
function addProduct() { products.value.push({ goods_id: 0, cny_amount: 0, usd_credit: 0, external_url: '', target_stock: 20, batch_size: 10, enabled: false }) }
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
onMounted(load)
</script>
