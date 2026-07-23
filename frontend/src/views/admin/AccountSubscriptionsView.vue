<template>
  <AppLayout>
    <div class="space-y-6">
      <header class="flex flex-wrap items-start justify-between gap-4">
        <div class="flex items-start gap-3">
          <div class="flex h-11 w-11 flex-shrink-0 items-center justify-center rounded-xl bg-primary-100 text-primary-600 dark:bg-primary-900/30 dark:text-primary-300">
            <Icon name="creditCard" size="lg" />
          </div>
          <div>
            <h1 class="text-xl font-semibold text-gray-900 dark:text-white">
              {{ t('admin.accounts.subscriptions.title') }}
            </h1>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.subscriptions.description') }}
            </p>
          </div>
        </div>
        <div class="flex flex-wrap items-center gap-3">
          <router-link
            to="/docs/codex-memory"
            class="inline-flex items-center gap-2 text-sm font-medium text-primary-600 transition-colors hover:text-primary-500 dark:text-primary-400 dark:hover:text-primary-300"
          >
            <Icon name="book" size="sm" />
            Codex Memory Docs
          </router-link>
          <span
            v-if="savedAddress"
            class="inline-flex items-center gap-1.5 rounded-full bg-emerald-50 px-3 py-1.5 text-xs font-medium text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300"
          >
            <Icon name="checkCircle" size="sm" />
            {{ t('admin.accounts.subscriptions.saved') }}
          </span>
        </div>
      </header>

      <div
        v-if="savedMessage"
        class="flex items-center gap-2 rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700 dark:border-emerald-800 dark:bg-emerald-900/20 dark:text-emerald-300"
        role="status"
        aria-live="polite"
      >
        <Icon name="checkCircle" size="sm" />
        {{ savedMessage }}
      </div>

      <div class="grid grid-cols-1 gap-6 xl:grid-cols-[minmax(0,1.15fr)_minmax(320px,0.85fr)]">
        <section class="card p-6">
          <div class="mb-5">
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">
              {{ t('admin.accounts.subscriptions.billingAddress') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.subscriptions.billingAddressHint') }}
            </p>
          </div>

          <form class="space-y-5" @submit.prevent="saveAddress">
            <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
              <label class="block md:col-span-2">
                <span class="input-label">{{ t('admin.accounts.subscriptions.accountLabel') }}</span>
                <input
                  v-model.trim="form.accountLabel"
                  type="text"
                  class="input w-full"
                  :placeholder="t('admin.accounts.subscriptions.accountLabelPlaceholder')"
                  autocomplete="organization"
                />
              </label>

              <label class="block md:col-span-2">
                <span class="input-label">
                  {{ t('admin.accounts.subscriptions.billingName') }}
                  <span class="text-red-500">*</span>
                </span>
                <input
                  v-model.trim="form.billingName"
                  type="text"
                  class="input w-full"
                  :placeholder="t('admin.accounts.subscriptions.billingNamePlaceholder')"
                  autocomplete="name"
                  required
                />
              </label>

              <label class="block">
                <span class="input-label">{{ t('admin.accounts.subscriptions.country') }}</span>
                <select v-model="form.country" class="input w-full" disabled>
                  <option value="US">{{ t('admin.accounts.subscriptions.unitedStates') }}</option>
                </select>
              </label>

              <label class="block">
                <span class="input-label">
                  {{ t('admin.accounts.subscriptions.state') }}
                  <span class="text-red-500">*</span>
                </span>
                <select v-model="form.state" class="input w-full" required>
                  <option value="" disabled>{{ t('admin.accounts.subscriptions.selectState') }}</option>
                  <option v-for="state in US_STATES" :key="state.code" :value="state.code">
                    {{ state.name }} ({{ state.code }})
                  </option>
                </select>
              </label>

              <label class="block md:col-span-2">
                <span class="input-label">
                  {{ t('admin.accounts.subscriptions.addressLine1') }}
                  <span class="text-red-500">*</span>
                </span>
                <input
                  v-model.trim="form.addressLine1"
                  type="text"
                  class="input w-full"
                  :placeholder="t('admin.accounts.subscriptions.addressLine1Placeholder')"
                  autocomplete="address-line1"
                  required
                />
              </label>

              <label class="block md:col-span-2">
                <span class="input-label">{{ t('admin.accounts.subscriptions.addressLine2') }}</span>
                <input
                  v-model.trim="form.addressLine2"
                  type="text"
                  class="input w-full"
                  :placeholder="t('admin.accounts.subscriptions.addressLine2Placeholder')"
                  autocomplete="address-line2"
                />
              </label>

              <label class="block">
                <span class="input-label">
                  {{ t('admin.accounts.subscriptions.city') }}
                  <span class="text-red-500">*</span>
                </span>
                <input
                  v-model.trim="form.city"
                  type="text"
                  class="input w-full"
                  :placeholder="t('admin.accounts.subscriptions.cityPlaceholder')"
                  autocomplete="address-level2"
                  required
                />
              </label>

              <label class="block">
                <span class="input-label">
                  {{ t('admin.accounts.subscriptions.postalCode') }}
                  <span class="text-red-500">*</span>
                </span>
                <input
                  v-model.trim="form.postalCode"
                  type="text"
                  inputmode="numeric"
                  class="input w-full"
                  :placeholder="t('admin.accounts.subscriptions.postalCodePlaceholder')"
                  autocomplete="postal-code"
                  required
                />
              </label>
            </div>

            <label class="flex items-start gap-3 rounded-lg border border-gray-200 p-3 dark:border-dark-600">
              <input
                v-model="form.actualAddressConfirmed"
                type="checkbox"
                class="mt-0.5 h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500 dark:border-dark-500 dark:bg-dark-700"
              />
              <span class="text-sm leading-5 text-gray-600 dark:text-gray-300">
                {{ t('admin.accounts.subscriptions.actualAddressConfirmation') }}
              </span>
            </label>

            <p
              v-if="validationError"
              class="flex items-center gap-2 text-sm text-red-600 dark:text-red-400"
              role="alert"
            >
              <Icon name="exclamationCircle" size="sm" />
              {{ validationError }}
            </p>

            <div class="flex flex-wrap justify-end gap-2 border-t border-gray-100 pt-4 dark:border-dark-700">
              <button type="button" class="btn btn-secondary" @click="clearForm">
                <Icon name="x" size="sm" class="mr-1.5" />
                {{ t('admin.accounts.subscriptions.clearForm') }}
              </button>
              <button type="submit" class="btn btn-primary" :disabled="saving">
                <Icon name="check" size="sm" class="mr-1.5" />
                {{ t('admin.accounts.subscriptions.saveAddress') }}
              </button>
            </div>
          </form>
        </section>

        <section class="card p-6">
          <div class="mb-5 flex items-start justify-between gap-3">
            <div>
              <h2 class="text-base font-semibold text-gray-900 dark:text-white">
                {{ t('admin.accounts.subscriptions.statusTitle') }}
              </h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ t('admin.accounts.subscriptions.stateTaxNote') }}
              </p>
            </div>
            <Icon name="calculator" size="lg" class="text-gray-400 dark:text-dark-400" />
          </div>

          <div
            v-if="taxStatus === 'unknown'"
            class="rounded-lg border border-gray-200 bg-gray-50 p-4 text-sm text-gray-600 dark:border-dark-600 dark:bg-dark-900/50 dark:text-gray-300"
          >
            {{ t('admin.accounts.subscriptions.statusEmpty') }}
          </div>
          <div
            v-else-if="taxStatus === 'no-state-general-sales-tax'"
            class="rounded-lg border border-emerald-200 bg-emerald-50 p-4 dark:border-emerald-800 dark:bg-emerald-900/20"
          >
            <div class="flex items-start gap-3">
              <Icon name="checkCircle" size="lg" class="flex-shrink-0 text-emerald-600 dark:text-emerald-400" />
              <div>
                <p class="font-medium text-emerald-800 dark:text-emerald-300">
                  {{ t('admin.accounts.subscriptions.noStateGeneralSalesTax') }}
                </p>
                <p v-if="taxCaveatCode" class="mt-1 text-sm text-emerald-700 dark:text-emerald-400">
                  {{ t(`admin.accounts.subscriptions.caveats.${taxCaveatCode}`) }}
                </p>
              </div>
            </div>
          </div>
          <div
            v-else
            class="rounded-lg border border-amber-200 bg-amber-50 p-4 dark:border-amber-800 dark:bg-amber-900/20"
          >
            <div class="flex items-start gap-3">
              <Icon name="infoCircle" size="lg" class="flex-shrink-0 text-amber-600 dark:text-amber-400" />
              <p class="font-medium text-amber-800 dark:text-amber-300">
                {{ t('admin.accounts.subscriptions.stateGeneralSalesTax') }}
              </p>
            </div>
          </div>

          <div class="mt-6 border-t border-gray-100 pt-5 dark:border-dark-700">
            <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
              {{ t('admin.accounts.subscriptions.noStateListTitle') }}
            </h3>
            <p class="mt-1 text-xs leading-5 text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.subscriptions.noStateListHint') }}
            </p>
            <div class="mt-3 grid grid-cols-1 gap-2 sm:grid-cols-2">
              <div
                v-for="state in noStateSalesTaxStates"
                :key="state.code"
                class="flex items-center justify-between rounded-lg border border-gray-200 px-3 py-2 text-sm dark:border-dark-600"
                :class="form.state === state.code ? 'border-primary-400 bg-primary-50 dark:border-primary-700 dark:bg-primary-900/20' : ''"
              >
                <span class="font-medium text-gray-800 dark:text-gray-200">{{ state.name }}</span>
                <span class="text-xs text-gray-500 dark:text-gray-400">{{ state.code }}</span>
              </div>
            </div>
          </div>
        </section>
      </div>

      <section v-if="savedAddress" class="card p-6">
        <div class="flex flex-wrap items-start justify-between gap-4">
          <div>
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">
              {{ savedAddress.accountLabel || t('admin.accounts.subscriptions.billingAddress') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.subscriptions.savedAt', { time: savedAtLabel }) }}
            </p>
          </div>
          <button type="button" class="btn btn-secondary btn-sm" @click="restoreSavedAddress">
            <Icon name="refresh" size="sm" class="mr-1.5" />
            {{ t('common.edit') }}
          </button>
        </div>
        <div class="mt-4 grid grid-cols-1 gap-3 text-sm text-gray-700 dark:text-gray-300 md:grid-cols-2">
          <div>
            <span class="text-gray-500 dark:text-gray-400">{{ t('admin.accounts.subscriptions.billingName') }}:</span>
            {{ savedAddress.billingName }}
          </div>
          <div>
            <span class="text-gray-500 dark:text-gray-400">{{ t('admin.accounts.subscriptions.state') }}:</span>
            {{ selectedSavedStateName }} ({{ savedAddress.state }})
          </div>
          <div class="md:col-span-2">
            <span class="text-gray-500 dark:text-gray-400">{{ t('admin.accounts.subscriptions.billingAddress') }}:</span>
            {{ savedAddress.addressLine1 }}<span v-if="savedAddress.addressLine2">, {{ savedAddress.addressLine2 }}</span>,
            {{ savedAddress.city }}, {{ savedAddress.state }} {{ savedAddress.postalCode }}
          </div>
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import {
  NO_STATE_GENERAL_SALES_TAX_CODES,
  US_STATES,
  detectStateTaxStatus,
  getTaxCaveatCode
} from './accountSubscriptionTax'

interface BillingAddress {
  accountLabel: string
  billingName: string
  country: 'US'
  addressLine1: string
  addressLine2: string
  city: string
  state: string
  postalCode: string
  actualAddressConfirmed: boolean
}

interface StoredAddressSnapshot {
  address: BillingAddress
  savedAt: string
}

const STORAGE_KEY = 'admin-account-subscription-billing-address'

const { t } = useI18n()

const form = reactive<BillingAddress>(createEmptyAddress())
const savedAddress = ref<BillingAddress | null>(null)
const savedAt = ref<string | null>(null)
const saving = ref(false)
const validationError = ref('')
const savedMessage = ref('')

const noStateSalesTaxStates = computed(() =>
  US_STATES.filter((state) => NO_STATE_GENERAL_SALES_TAX_CODES.includes(state.code as (typeof NO_STATE_GENERAL_SALES_TAX_CODES)[number]))
)

const taxStatus = computed(() => detectStateTaxStatus(form.state))
const taxCaveatCode = computed(() => getTaxCaveatCode(form.state))
const savedAtLabel = computed(() => {
  if (!savedAt.value) return ''
  return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(savedAt.value))
})
const selectedSavedStateName = computed(() => {
  if (!savedAddress.value) return ''
  return US_STATES.find((state) => state.code === savedAddress.value?.state)?.name || savedAddress.value.state
})
const canSave = computed(() =>
  Boolean(
    form.billingName &&
      form.addressLine1 &&
      form.city &&
      form.state &&
      form.postalCode &&
      form.actualAddressConfirmed
  )
)

function createEmptyAddress(): BillingAddress {
  return {
    accountLabel: '',
    billingName: '',
    country: 'US',
    addressLine1: '',
    addressLine2: '',
    city: '',
    state: '',
    postalCode: '',
    actualAddressConfirmed: false
  }
}

function isBillingAddress(value: unknown): value is BillingAddress {
  if (!value || typeof value !== 'object') return false
  const record = value as Record<string, unknown>
  return (
    typeof record.accountLabel === 'string' &&
    typeof record.billingName === 'string' &&
    record.country === 'US' &&
    typeof record.addressLine1 === 'string' &&
    typeof record.addressLine2 === 'string' &&
    typeof record.city === 'string' &&
    typeof record.state === 'string' &&
    typeof record.postalCode === 'string' &&
    typeof record.actualAddressConfirmed === 'boolean'
  )
}

function isStoredSnapshot(value: unknown): value is StoredAddressSnapshot {
  if (!value || typeof value !== 'object') return false
  const record = value as Record<string, unknown>
  return isBillingAddress(record.address) && typeof record.savedAt === 'string'
}

function loadSavedAddress() {
  const raw = localStorage.getItem(STORAGE_KEY)
  if (!raw) return

  try {
    const parsed: unknown = JSON.parse(raw)
    if (!isStoredSnapshot(parsed)) return
    savedAddress.value = parsed.address
    savedAt.value = parsed.savedAt
    Object.assign(form, parsed.address)
  } catch (error) {
    console.warn('Failed to load saved account subscription address:', error)
  }
}

function clearForm() {
  Object.assign(form, createEmptyAddress())
  validationError.value = ''
  savedMessage.value = ''
}

function restoreSavedAddress() {
  if (!savedAddress.value) return
  Object.assign(form, savedAddress.value)
  validationError.value = ''
  savedMessage.value = ''
}

function saveAddress() {
  validationError.value = ''
  savedMessage.value = ''

  if (!canSave.value) {
    validationError.value = t('admin.accounts.subscriptions.validation.required')
    return
  }

  if (!/^\d{5}(?:-\d{4})?$/.test(form.postalCode)) {
    validationError.value = t('admin.accounts.subscriptions.validation.postalCode')
    return
  }

  saving.value = true
  try {
    const snapshot: StoredAddressSnapshot = {
      address: { ...form },
      savedAt: new Date().toISOString()
    }
    localStorage.setItem(STORAGE_KEY, JSON.stringify(snapshot))
    savedAddress.value = snapshot.address
    savedAt.value = snapshot.savedAt
    savedMessage.value = t('admin.accounts.subscriptions.saved')
  } finally {
    saving.value = false
  }
}

onMounted(loadSavedAddress)
</script>
