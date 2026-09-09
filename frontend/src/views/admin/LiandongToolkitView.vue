<template>
  <AppLayout>
    <div class="mx-auto max-w-[1500px] space-y-6" data-testid="ldxp-toolkit-page">
      <div class="flex flex-wrap items-start justify-between gap-4">
        <div>
          <div class="mb-2 flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400">
            <Icon name="beaker" size="sm" />
            <span>{{ t('ldxpToolkit.tools') }}</span>
            <Icon name="chevronRight" size="xs" />
            <span>{{ t('ldxpToolkit.navLabel') }}</span>
          </div>
          <h1 class="text-2xl font-bold text-gray-900 dark:text-white">{{ t('ldxpToolkit.title') }}</h1>
          <p class="mt-1 max-w-3xl text-sm text-gray-500 dark:text-gray-400">
            {{ t('ldxpToolkit.description') }}
          </p>
        </div>
        <button
          type="button"
          class="btn btn-secondary btn-sm"
          :disabled="loadingStatus"
          :title="t('common.refresh')"
          data-testid="refresh-status"
          @click="loadStatus()"
        >
          <Icon name="refresh" size="sm" :class="loadingStatus ? 'animate-spin' : ''" />
          {{ t('common.refresh') }}
        </button>
      </div>

      <!-- Runtime -->
      <section class="card p-5 md:p-6" data-testid="runtime-section">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div>
            <div class="flex items-center gap-2">
              <Icon name="terminal" size="md" class="text-primary-600 dark:text-primary-400" />
              <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('ldxpToolkit.runtime.title') }}</h2>
            </div>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('ldxpToolkit.runtime.description') }}</p>
          </div>
          <div class="flex flex-wrap gap-2">
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              :disabled="loadingInstallation"
              :title="t('ldxpToolkit.runtime.refresh')"
              data-testid="refresh-installation"
              @click="loadInstallation"
            >
              <Icon name="refresh" size="sm" :class="loadingInstallation ? 'animate-spin' : ''" />
              {{ t('ldxpToolkit.runtime.refresh') }}
            </button>
            <button
              type="button"
              class="btn btn-primary btn-sm"
              :disabled="loadingInstallation || installing || !installationAvailable || installation?.asset_available === false"
              data-testid="install-repair"
              @click="installToolkit"
            >
              <Icon name="cog" size="sm" :class="installing ? 'animate-spin' : ''" />
              {{ t('ldxpToolkit.runtime.installRepair') }}
            </button>
          </div>
        </div>

        <div v-if="installationError" class="mt-4 rounded-lg bg-red-50 p-3 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-300" role="alert">
          <div class="flex items-start gap-2">
            <Icon name="exclamationCircle" size="sm" class="mt-0.5 shrink-0" />
            <span>{{ installationError }}</span>
          </div>
        </div>
        <div v-else-if="runtimeUnavailable" class="mt-4 rounded-lg bg-amber-50 p-3 text-sm text-amber-800 dark:bg-amber-900/20 dark:text-amber-200" role="alert">
          <div class="flex items-start gap-2">
            <Icon name="exclamationTriangle" size="sm" class="mt-0.5 shrink-0" />
            <span>{{ t('ldxpToolkit.runtime.endpointUnavailable') }}</span>
          </div>
        </div>
        <div v-else-if="loadingInstallation" class="mt-4 text-sm text-gray-500 dark:text-gray-400">
          {{ t('common.loading') }}
        </div>
        <div v-else-if="installation" class="mt-5 space-y-4">
          <div class="flex flex-wrap items-center gap-2">
            <span
              class="badge"
              :class="installationInstalled ? 'badge-success' : 'badge-warning'"
              data-testid="installation-status"
            >
              {{ installationInstalled ? t('ldxpToolkit.runtime.installed') : t('ldxpToolkit.runtime.notInstalled') }}
            </span>
            <span class="text-xs text-gray-500 dark:text-gray-400">{{ t('ldxpToolkit.runtime.fixedCommand') }}</span>
            <code class="rounded bg-gray-100 px-2 py-1 text-xs text-gray-700 dark:bg-dark-700 dark:text-gray-300">POST /admin/tools/ldxp/installation</code>
          </div>
          <dl class="grid grid-cols-1 gap-x-6 gap-y-3 text-sm sm:grid-cols-2 lg:grid-cols-4">
            <div>
              <dt class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">{{ t('ldxpToolkit.runtime.platform') }}</dt>
              <dd class="mt-1 font-mono text-gray-900 dark:text-white">{{ platformLabel }}</dd>
            </div>
            <div>
              <dt class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">{{ t('ldxpToolkit.runtime.path') }}</dt>
              <dd class="mt-1 break-all font-mono text-gray-900 dark:text-white">{{ programPath }}</dd>
            </div>
            <div>
              <dt class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">{{ t('ldxpToolkit.runtime.version') }}</dt>
              <dd class="mt-1 font-mono text-gray-900 dark:text-white">{{ installation.version || '-' }}</dd>
            </div>
            <div>
              <dt class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">{{ t('ldxpToolkit.runtime.dataDirectory') }}</dt>
              <dd class="mt-1 break-all font-mono text-gray-900 dark:text-white">{{ installation.data_directory || '-' }}</dd>
            </div>
          </dl>
          <ul v-if="installation.diagnostics?.length" class="space-y-1 text-xs text-gray-500 dark:text-gray-400" data-testid="runtime-diagnostics">
            <li v-for="diagnostic in installation.diagnostics" :key="diagnostic" class="flex items-start gap-2">
              <span class="mt-1 h-1.5 w-1.5 shrink-0 rounded-full bg-gray-400"></span>
              <span>{{ redactDisplayText(diagnostic) }}</span>
            </li>
          </ul>
        </div>
      </section>

      <!-- Connection and mappings -->
      <section class="card p-5 md:p-6" data-testid="connection-section">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div>
            <div class="flex items-center gap-2">
              <Icon name="link" size="md" class="text-primary-600 dark:text-primary-400" />
              <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('ldxpToolkit.connection.title') }}</h2>
            </div>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('ldxpToolkit.connection.description') }}</p>
          </div>
          <div class="flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
            <span class="h-2 w-2 rounded-full" :class="status?.merchant_token_configured ? 'bg-green-500' : 'bg-gray-400'"></span>
            {{ status?.merchant_token_configured ? t('ldxpToolkit.connection.tokenConfigured') : t('ldxpToolkit.connection.tokenNotConfigured') }}
          </div>
        </div>

        <div v-if="statusError" class="mt-4 rounded-lg bg-red-50 p-3 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-300" role="alert">
          <div class="flex items-start gap-2">
            <Icon name="exclamationCircle" size="sm" class="mt-0.5 shrink-0" />
            <span>{{ statusError }}</span>
          </div>
        </div>
        <div v-else-if="loadingStatus" class="mt-4 text-sm text-gray-500 dark:text-gray-400">{{ t('common.loading') }}</div>
        <div v-else class="mt-5 space-y-5">
          <div class="max-w-xl">
            <label class="input-label" for="ldxp-merchant-token">{{ t('ldxpToolkit.connection.merchantToken') }}</label>
            <input
              id="ldxp-merchant-token"
              v-model="merchantToken"
              type="password"
              class="input w-full"
              autocomplete="new-password"
              :placeholder="t('ldxpToolkit.connection.tokenPlaceholder')"
              data-testid="merchant-token-input"
            />
            <p class="input-hint">{{ t('ldxpToolkit.connection.tokenHint') }}</p>
          </div>

          <div class="flex flex-wrap gap-2">
            <button type="button" class="btn btn-secondary btn-sm" :disabled="testingConnection" data-testid="test-connection" @click="testMerchantConnection">
              <Icon name="link" size="sm" :class="testingConnection ? 'animate-pulse' : ''" />
              {{ testingConnection ? t('common.loading') : t('ldxpToolkit.connection.testConnection') }}
            </button>
            <button type="button" class="btn btn-primary btn-sm" :disabled="savingConfig || !hasValidConfiguration" data-testid="save-config" @click="saveConfiguration">
              <Icon name="check" size="sm" />
              {{ savingConfig ? t('common.saving') : t('common.save') }}
            </button>
          </div>
          <p v-if="connectionFeedback" class="text-sm" :class="connectionFeedbackType === 'error' ? 'text-red-600 dark:text-red-400' : 'text-green-600 dark:text-green-400'" role="status">
            {{ connectionFeedback }}
          </p>

          <div class="overflow-x-auto rounded-lg border border-gray-200 dark:border-dark-700">
            <table class="w-full min-w-[760px] text-sm" data-testid="mapping-table">
              <thead>
                <tr class="border-b border-gray-200 bg-gray-50 text-left text-xs font-medium uppercase tracking-wide text-gray-500 dark:border-dark-700 dark:bg-dark-800/60 dark:text-gray-400">
                  <th class="px-3 py-3">{{ t('ldxpToolkit.mapping.goodsId') }}</th>
                  <th class="px-3 py-3">{{ t('ldxpToolkit.mapping.sellingPrice') }}</th>
                  <th class="px-3 py-3">{{ t('ldxpToolkit.mapping.creditedBalance') }}</th>
                  <th class="px-3 py-3">{{ t('ldxpToolkit.mapping.targetStock') }}</th>
                  <th class="px-3 py-3">{{ t('ldxpToolkit.mapping.enabled') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="product in products" :key="product.goods_id" class="border-b border-gray-100 last:border-0 dark:border-dark-800" :data-testid="`mapping-row-${product.goods_id}`">
                  <td class="px-3 py-3 font-mono text-gray-900 dark:text-white">{{ product.goods_id }}</td>
                  <td class="px-3 py-3">
                    <label class="sr-only" :for="`ldxp-cny-${product.goods_id}`">{{ t('ldxpToolkit.mapping.sellingPrice') }}</label>
                    <div class="flex items-center gap-2">
                      <span class="text-gray-500">CNY</span>
                      <input
                        :id="`ldxp-cny-${product.goods_id}`"
                        :value="positiveCnyAmount(product.cny_amount) ?? ''"
                        type="number"
                        min="1"
                        step="1"
                        class="input w-28"
                        @input="updateProductPrice(product, $event)"
                      />
                      <span v-if="!hasPositivePrice(product)" class="text-gray-500 dark:text-gray-400" :data-testid="`mapping-price-unknown-${product.goods_id}`">-</span>
                    </div>
                  </td>
                  <td class="px-3 py-3">
                    <label class="sr-only" :for="`ldxp-usd-${product.goods_id}`">{{ t('ldxpToolkit.mapping.creditedBalance') }}</label>
                    <div class="flex items-center gap-2">
                      <span class="text-gray-500">USD</span>
                      <input :id="`ldxp-usd-${product.goods_id}`" v-model.number="product.usd_credit" type="number" min="0.000001" step="0.01" class="input w-28" />
                    </div>
                  </td>
                  <td class="px-3 py-3">
                    <label class="sr-only" :for="`ldxp-target-${product.goods_id}`">{{ t('ldxpToolkit.mapping.targetStock') }}</label>
                    <input :id="`ldxp-target-${product.goods_id}`" v-model.number="product.target_stock" type="number" min="1" step="1" class="input w-32" />
                  </td>
                  <td class="px-3 py-3">
                    <label class="inline-flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
                      <input v-model="product.enabled" type="checkbox" />
                      <span>{{ product.enabled ? t('common.enabled') : t('common.disabled') }}</span>
                    </label>
                  </td>
                </tr>
                <tr v-if="!products.length">
                  <td colspan="5" class="px-3 py-8 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('ldxpToolkit.mapping.empty') }}</td>
                </tr>
              </tbody>
            </table>
          </div>
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('ldxpToolkit.mapping.priceHint') }}</p>

          <div class="border-t border-gray-200 pt-5 dark:border-dark-700">
            <div class="flex flex-wrap items-center justify-between gap-3">
              <div>
                <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('ldxpToolkit.goods.title') }}</h3>
                <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('ldxpToolkit.goods.description') }}</p>
              </div>
              <button type="button" class="btn btn-secondary btn-sm" :disabled="loadingGoods" data-testid="sync-goods" @click="syncRemoteGoods">
                <Icon name="sync" size="sm" :class="loadingGoods ? 'animate-spin' : ''" />
                {{ loadingGoods ? t('common.loading') : t('ldxpToolkit.goods.sync') }}
              </button>
            </div>
            <div v-if="goodsError" class="mt-3 rounded-lg bg-amber-50 p-3 text-sm text-amber-800 dark:bg-amber-900/20 dark:text-amber-200" role="alert">{{ goodsError }}</div>
            <div v-else-if="remoteGoods.length" class="mt-3 overflow-x-auto rounded-lg border border-gray-200 dark:border-dark-700">
              <table class="w-full min-w-[620px] text-sm" data-testid="remote-goods-table">
                <thead>
                  <tr class="border-b border-gray-200 bg-gray-50 text-left text-xs uppercase tracking-wide text-gray-500 dark:border-dark-700 dark:bg-dark-800/60 dark:text-gray-400">
                    <th class="px-3 py-2">{{ t('ldxpToolkit.mapping.goodsId') }}</th>
                    <th class="px-3 py-2">{{ t('ldxpToolkit.goods.name') }}</th>
                    <th class="px-3 py-2">{{ t('ldxpToolkit.mapping.sellingPrice') }}</th>
                    <th class="px-3 py-2">{{ t('ldxpToolkit.goods.unsoldStock') }}</th>
                    <th class="px-3 py-2">{{ t('ldxpToolkit.goods.mapping') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="good in remoteGoods" :key="good.goods_id" class="border-b border-gray-100 last:border-0 dark:border-dark-800">
                    <td class="px-3 py-2 font-mono">{{ good.goods_id }}</td>
                    <td class="max-w-[18rem] truncate px-3 py-2">{{ good.title || good.name || '-' }}</td>
                    <td class="px-3 py-2">{{ formatCny(remoteGoodPrice(good)) }}</td>
                    <td class="px-3 py-2">{{ remoteGoodStock(good) }}</td>
                    <td class="px-3 py-2">
                      <span :class="isMappedGood(good.goods_id) ? 'badge-success' : 'badge-gray'" class="badge">
                        {{ isMappedGood(good.goods_id) ? t('ldxpToolkit.goods.mapped') : t('ldxpToolkit.goods.unmapped') }}
                      </span>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
            <p v-else class="mt-3 text-sm text-gray-500 dark:text-gray-400">{{ t('ldxpToolkit.goods.empty') }}</p>
          </div>
        </div>
      </section>

      <!-- Preview and run -->
      <section class="card p-5 md:p-6" data-testid="preview-section">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div>
            <div class="flex items-center gap-2">
              <Icon name="calculator" size="md" class="text-primary-600 dark:text-primary-400" />
              <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('ldxpToolkit.preview.title') }}</h2>
            </div>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('ldxpToolkit.preview.description') }}</p>
          </div>
          <span v-if="activeJob" class="badge" :class="jobStatusClass(activeJob.status)" data-testid="active-job-status">
            {{ t('ldxpToolkit.status.job') }}: {{ jobStatusLabel(activeJob.status) }}
          </span>
        </div>

        <div class="mt-5 flex flex-wrap items-end gap-3">
          <div class="min-w-[14rem]">
            <label class="input-label" for="ldxp-selection-mode">{{ t('ldxpToolkit.preview.selection') }}</label>
            <select id="ldxp-selection-mode" v-model="selectionMode" class="input w-full" data-testid="selection-mode">
              <option value="all">{{ t('ldxpToolkit.preview.allConfigured') }}</option>
              <option value="selected">{{ t('ldxpToolkit.preview.selectedProducts') }}</option>
            </select>
          </div>
          <button type="button" class="btn btn-secondary btn-sm" :disabled="loadingStatus || loadingJob" data-testid="refresh-job" @click="refreshActiveJob">
            <Icon name="refresh" size="sm" :class="loadingJob ? 'animate-spin' : ''" />
            {{ t('ldxpToolkit.preview.refreshJob') }}
          </button>
          <button
            type="button"
            class="btn btn-primary btn-sm"
            :disabled="previewLoading || !selectedGoodsForRequest.length"
            data-testid="preview-button"
            @click="runPreview"
          >
            <Icon name="eye" size="sm" />
            {{ previewLoading ? t('common.loading') : t('ldxpToolkit.preview.previewAction') }}
          </button>
          <button
            type="button"
            class="btn btn-danger btn-sm"
            :disabled="!canRun || runningJob"
            data-testid="run-button"
            @click="requestRun"
          >
            <Icon name="play" size="sm" />
            {{ runningJob ? t('common.loading') : t('ldxpToolkit.preview.runAction') }}
          </button>
          <button
            v-if="activeJob && resumableJob && jobIdentifier(activeJob)"
            type="button"
            class="btn btn-secondary btn-sm"
            :disabled="resumingJob"
            data-testid="resume-job"
            @click="resumeActiveJob"
          >
            <Icon name="refresh" size="sm" :class="resumingJob ? 'animate-spin' : ''" />
            {{ resumingJob ? t('common.loading') : t('ldxpToolkit.preview.resumeAction') }}
          </button>
        </div>

        <div v-if="selectionMode === 'selected'" class="mt-3 flex flex-wrap gap-x-4 gap-y-2 rounded-lg bg-gray-50 p-3 text-sm dark:bg-dark-800/60">
          <label v-for="product in products" :key="product.goods_id" class="inline-flex items-center gap-2 text-gray-700 dark:text-gray-300">
            <input v-model="selectedGoodsIds" type="checkbox" :value="product.goods_id" />
            <span>{{ product.goods_id }} · {{ formatCny(product.cny_amount) }}</span>
          </label>
          <span v-if="!products.length" class="text-gray-500 dark:text-gray-400">{{ t('ldxpToolkit.mapping.empty') }}</span>
        </div>

        <div v-if="previewError" class="mt-4 rounded-lg bg-red-50 p-3 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-300" role="alert" data-testid="preview-error">{{ previewError }}</div>
        <div v-if="jobError" class="mt-4 rounded-lg bg-amber-50 p-3 text-sm text-amber-800 dark:bg-amber-900/20 dark:text-amber-200" role="alert" data-testid="job-error">{{ jobError }}</div>
        <div v-if="previewRows.length" class="mt-4 overflow-x-auto rounded-lg border border-gray-200 dark:border-dark-700">
          <table class="w-full min-w-[780px] text-sm" data-testid="preview-table">
            <thead>
              <tr class="border-b border-gray-200 bg-gray-50 text-left text-xs uppercase tracking-wide text-gray-500 dark:border-dark-700 dark:bg-dark-800/60 dark:text-gray-400">
                <th class="px-3 py-3">{{ t('ldxpToolkit.mapping.goodsId') }}</th>
                <th class="px-3 py-3">{{ t('ldxpToolkit.preview.currentStock') }}</th>
                <th class="px-3 py-3">{{ t('ldxpToolkit.mapping.targetStock') }}</th>
                <th class="px-3 py-3">{{ t('ldxpToolkit.preview.plannedAddition') }}</th>
                <th class="px-3 py-3">{{ t('ldxpToolkit.preview.mappingError') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(row, index) in previewRows" :key="`${row.goods_id}-${index}`" class="border-b border-gray-100 last:border-0 dark:border-dark-800" :data-testid="`preview-row-${row.goods_id}`">
                <td class="px-3 py-3 font-mono text-gray-900 dark:text-white">{{ row.goods_id }}</td>
                <td class="px-3 py-3">{{ row.current_stock }}</td>
                <td class="px-3 py-3">{{ row.target_stock }}</td>
                <td class="px-3 py-3 font-semibold" :class="row.planned_addition > 0 ? 'text-amber-700 dark:text-amber-300' : 'text-gray-600 dark:text-gray-300'">{{ row.planned_addition }}</td>
                <td class="px-3 py-3" :class="row.mapping_error || !row.eligible || !row.enabled ? 'text-red-600 dark:text-red-400' : 'text-gray-500 dark:text-gray-400'" :data-testid="`preview-reason-${row.goods_id}`">
                  <div v-if="row.reason">{{ row.reason }}</div>
                  <div v-if="row.mapping_error" class="mt-1">{{ row.mapping_error }}</div>
                  <div v-if="row.validation_error" class="mt-1">{{ row.validation_error }}</div>
                  <span v-if="!row.reason && !row.mapping_error && !row.validation_error">-</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <p v-else class="mt-5 text-sm text-gray-500 dark:text-gray-400">{{ t('ldxpToolkit.preview.empty') }}</p>
        <div v-if="jobStateNotice" class="mt-4 rounded-lg bg-amber-50 p-3 text-sm text-amber-800 dark:bg-amber-900/20 dark:text-amber-200" data-testid="job-state-notice">
          {{ jobStateNotice }}
        </div>
      </section>

      <!-- History -->
      <section class="card p-5 md:p-6" data-testid="history-section">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div>
            <div class="flex items-center gap-2">
              <Icon name="document" size="md" class="text-primary-600 dark:text-primary-400" />
              <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('ldxpToolkit.history.title') }}</h2>
            </div>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('ldxpToolkit.history.description') }}</p>
          </div>
          <button type="button" class="btn btn-secondary btn-sm" :disabled="loadingStatus" :title="t('common.refresh')" @click="loadStatus()">
            <Icon name="refresh" size="sm" :class="loadingStatus ? 'animate-spin' : ''" />
            {{ t('common.refresh') }}
          </button>
        </div>
        <div class="mt-4 overflow-x-auto rounded-lg border border-gray-200 dark:border-dark-700">
          <table class="w-full min-w-[820px] text-sm" data-testid="history-table">
            <thead>
              <tr class="border-b border-gray-200 bg-gray-50 text-left text-xs uppercase tracking-wide text-gray-500 dark:border-dark-700 dark:bg-dark-800/60 dark:text-gray-400">
                <th class="px-3 py-3">{{ t('ldxpToolkit.history.id') }}</th>
                <th class="px-3 py-3">{{ t('ldxpToolkit.history.type') }}</th>
                <th class="px-3 py-3">{{ t('ldxpToolkit.mapping.goodsId') }}</th>
                <th class="px-3 py-3">{{ t('ldxpToolkit.history.count') }}</th>
                <th class="px-3 py-3">{{ t('ldxpToolkit.history.status') }}</th>
                <th class="px-3 py-3">{{ t('ldxpToolkit.history.updatedAt') }}</th>
                <th class="px-3 py-3">{{ t('ldxpToolkit.history.actions') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in historyRows" :key="row.key" class="border-b border-gray-100 last:border-0 dark:border-dark-800">
                <td class="px-3 py-3 font-mono text-xs">{{ row.displayId }}</td>
                <td class="px-3 py-3">{{ row.type === 'job' ? t('ldxpToolkit.history.job') : t('ldxpToolkit.history.batch') }}</td>
                <td class="px-3 py-3 font-mono">{{ row.goodsId || '-' }}</td>
                <td class="px-3 py-3">{{ row.count ?? '-' }}</td>
                <td class="px-3 py-3"><span class="badge" :class="jobStatusClass(row.status)">{{ jobStatusLabel(row.status) }}</span></td>
                <td class="px-3 py-3 text-xs text-gray-500 dark:text-gray-400">{{ formatDate(row.updatedAt || row.createdAt) }}</td>
                <td class="px-3 py-3">
                  <button v-if="canExportHistoryRow(row)" type="button" class="btn btn-secondary btn-xs" :disabled="exportingJobId === row.jobId" :title="t('ldxpToolkit.history.export')" :data-testid="`export-job-${row.jobId}`" @click="downloadJobExport(row.jobId)">
                    <Icon name="download" size="xs" :class="exportingJobId === row.jobId ? 'animate-pulse' : ''" />
                    {{ exportingJobId === row.jobId ? t('common.loading') : t('ldxpToolkit.history.export') }}
                  </button>
                  <span v-else class="text-xs text-gray-400">-</span>
                </td>
              </tr>
              <tr v-if="!historyRows.length">
                <td colspan="7" class="px-3 py-8 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('ldxpToolkit.history.empty') }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </div>

    <ConfirmDialog
      :show="showRunConfirmation"
      :title="t('ldxpToolkit.preview.confirmTitle')"
      :message="t('ldxpToolkit.preview.confirmMessage')"
      :confirm-text="t('ldxpToolkit.preview.confirmRun')"
      :cancel-text="t('common.cancel')"
      danger
      data-testid="run-confirmation"
      @confirm="confirmRun"
      @cancel="showRunConfirmation = false"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'
import AppLayout from '@/components/layout/AppLayout.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import {
  DEFAULT_LIANDONG_TARGET_STOCK,
  isLiandongTerminalJob,
  LIANDONG_UNSAFE_JOB_STATES,
  liandongToolkitAPI,
  type LiandongApplicationError,
  type LiandongOperationContext,
  type LiandongBatchStatus,
  type LiandongConfigUpdate,
  type LiandongInstallationStatus,
  type LiandongInstallationResponse,
  type LiandongJob,
  type LiandongJobResponse,
  type LiandongPreviewItem,
  type LiandongProductMapping,
  type LiandongRemoteGood,
  type LiandongStatus,
} from '@/api/liandongToolkit'

const { t } = useI18n()
const appStore = useAppStore()

type ErrorRecord = LiandongApplicationError
type SelectionMode = 'all' | 'selected'
type PreviewRow = {
  goods_id: number
  current_stock: number | string
  target_stock: number
  planned_addition: number
  enabled: boolean
  eligible: boolean
  reason: string
  mapping_error: string
  validation_error: string
}
type HistoryRow = {
  key: string
  type: 'job' | 'batch'
  displayId: string
  jobId?: string
  goodsId?: number
  count?: number
  status: string
  exportAvailable?: boolean
  createdAt?: string
  updatedAt?: string
}
type PreviewRequestSnapshot = Readonly<{
  requestedGoods: readonly number[]
  selectionKey: string
  mappingVersion: string
  configurationVersion: string
  executionKey: string
}>
type PreviewValidation = Readonly<{
  valid: boolean
  errors: readonly string[]
}>
type StatusLoadOptions = Readonly<{
  preservePreview?: boolean
}>

const installation = ref<LiandongInstallationStatus | null>(null)
const installationError = ref('')
const installationEndpointUnavailable = ref(false)
const loadingInstallation = ref(false)
const installing = ref(false)

const status = ref<LiandongStatus | null>(null)
const statusError = ref('')
const loadingStatus = ref(false)
const merchantToken = ref('')
const products = ref<LiandongProductMapping[]>([])
const savingConfig = ref(false)
const testingConnection = ref(false)
const connectionFeedback = ref('')
const connectionFeedbackType = ref<'success' | 'error'>('success')

const remoteGoods = ref<LiandongRemoteGood[]>([])
const goodsError = ref('')
const loadingGoods = ref(false)

const selectionMode = ref<SelectionMode>('all')
const selectedGoodsIds = ref<number[]>([])
const previewRows = ref<readonly PreviewRow[]>([])
const previewSnapshot = ref<PreviewRequestSnapshot | null>(null)
const previewValidation = ref<PreviewValidation>({ valid: false, errors: [] })
const previewError = ref('')
const previewLoading = ref(false)
const showRunConfirmation = ref(false)
const runningJob = ref(false)
const resumingJob = ref(false)
const loadingJob = ref(false)
const jobError = ref('')
const currentJob = ref<LiandongJob | null>(null)
const pendingJob = ref<LiandongJob | null>(null)
const exportingJobId = ref('')
let previewRequestSequence = 0

function errorRecord(error: unknown): ErrorRecord {
  if (typeof error !== 'object' || error === null) return {}
  const record = error as Record<string, unknown>
  const response = typeof record.response === 'object' && record.response !== null ? record.response as Record<string, unknown> : undefined
  const responseData = typeof response?.data === 'object' && response.data !== null ? response.data as Record<string, unknown> : undefined
  const source = responseData || record
  const statusValue = record.status ?? response?.status
  const status = Number(statusValue)
  return {
    status: Number.isFinite(status) ? status : undefined,
    code: typeof source.code === 'string' || typeof source.code === 'number' ? source.code : undefined,
    reason: typeof source.reason === 'string' ? source.reason : undefined,
    message: typeof source.message === 'string'
      ? source.message
      : typeof source.detail === 'string'
        ? source.detail
        : undefined,
    metadata: typeof source.metadata === 'object' && source.metadata !== null
      ? source.metadata as Record<string, unknown>
      : undefined,
  }
}

function stableApplicationCode(error: unknown): string {
  const record = errorRecord(error)
  const candidates = [
    record.reason,
    typeof record.code === 'string' ? record.code : undefined,
    record.message,
  ]
  for (const value of candidates) {
    const candidate = String(value || '').trim().toUpperCase()
    if (/^[A-Z][A-Z0-9_]*$/.test(candidate)) return candidate
  }
  return ''
}

const runtimeUnavailableCodes = new Set([
  'LDXP_TOOLKIT_UNAVAILABLE',
  'LDXP_TOOLKIT_NOT_READY',
  'LDXP_TOOLKIT_ASSET_UNREADABLE',
  'LDXP_TOOLKIT_CHECKSUM_MISMATCH',
  'LDXP_TOOLKIT_CHECKSUM_INVALID',
  'LDXP_TOOLKIT_DATA_DIRECTORY_UNAVAILABLE',
  'LDXP_TOOLKIT_DESTINATION_UNAVAILABLE',
  'LDXP_TOOLKIT_INSTALL_FAILED',
])

function isEndpointUnavailable(error: unknown, operation: 'installation' | 'other'): boolean {
  if (operation !== 'installation') return false
  const record = errorRecord(error)
  const code = stableApplicationCode(error)
  const statusCode = Number(record.status)
  if (code) return runtimeUnavailableCodes.has(code)
  return statusCode === 404 || statusCode === 501 || statusCode === 503
}

function serverDeclaresUnavailable(value: unknown): boolean {
  if (typeof value !== 'object' || value === null) return false
  const record = value as Record<string, unknown>
  return record.available === false || record.endpoint_available === false || record.runtime_available === false
}

function redactDisplayText(value: unknown): string {
  let text = String(value ?? '')
  const token = merchantToken.value.trim()
  if (token) text = text.split(token).join('[redacted]')
  return text
    .replace(/Bearer\s+[A-Za-z0-9._~+/=-]+/gi, 'Bearer [redacted]')
    .replace(/\b(?:LD[-_])?[A-Za-z0-9-]{20,}\b/g, '[redacted]')
}

function operationError(error: unknown, fallback: string, operation: LiandongOperationContext): string {
  if (operation === 'installation' && isEndpointUnavailable(error, 'installation')) return t('ldxpToolkit.errors.endpointUnavailable')
  const record = errorRecord(error)
  const code = stableApplicationCode(error)
  const message = record.message ? redactDisplayText(record.message) : fallback
  return code ? `${message} [${operation}:${code}]` : message
}

function numberOr(value: unknown, fallback: number): number {
  const parsed = typeof value === 'number' ? value : Number(value)
  return Number.isFinite(parsed) ? parsed : fallback
}

function positivePrice(value: unknown): number | undefined {
  const parsed = typeof value === 'number' ? value : Number(value)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : undefined
}

function positiveCnyAmount(value: unknown): number | undefined {
  return positiveInteger(value)
}

function positiveInteger(value: unknown): number | undefined {
  const parsed = positivePrice(value)
  return parsed !== undefined && Number.isInteger(parsed) ? parsed : undefined
}

function nonNegativeInteger(value: unknown): number | undefined {
  if (value === null || value === undefined || (typeof value === 'string' && value.trim() === '')) return undefined
  const parsed = typeof value === 'number' ? value : Number(value)
  return Number.isFinite(parsed) && Number.isInteger(parsed) && parsed >= 0 ? parsed : undefined
}

function normalizedGrantType(value: unknown): string {
  if (value === undefined || value === null) return 'balance'
  if (typeof value !== 'string') return ''
  return value.trim() || 'balance'
}

function normalizedExternalUrl(value: unknown): string {
  if (value === undefined) return ''
  return typeof value === 'string' ? value.trim() : '[invalid]'
}

function isValidExternalUrl(value: unknown): boolean {
  if (value === undefined || value === '') return true
  if (typeof value !== 'string' || !value.trim()) return false
  try {
    const parsed = new URL(value.trim())
    return (parsed.protocol === 'http:' || parsed.protocol === 'https:') &&
      Boolean(parsed.hostname) &&
      !parsed.username &&
      !parsed.password &&
      !parsed.search &&
      !parsed.hash
  } catch {
    return false
  }
}

function optionalNumber(value: unknown): number | undefined {
  if (value === null || value === undefined || value === '') return undefined
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : undefined
}

function normalizeProduct(product: LiandongProductMapping): LiandongProductMapping {
  const targetStock = positiveInteger(product.target_stock) || DEFAULT_LIANDONG_TARGET_STOCK
  return {
    goods_id: positiveInteger(product.goods_id) || 0,
    cny_amount: positiveCnyAmount(product.cny_amount),
    usd_credit: numberOr(product.usd_credit, 0),
    target_stock: targetStock,
    enabled: product.enabled !== false,
    grant_type: product.grant_type,
    external_url: product.external_url,
    version: positiveInteger(product.version) || 1,
    threshold: optionalNumber(product.threshold),
    restock_count: optionalNumber(product.restock_count),
    current_stock: optionalNumber(product.current_stock),
    last_error: product.last_error ? redactDisplayText(product.last_error) : undefined,
    last_run_at: product.last_run_at,
  }
}

function isInstallationResult(response: LiandongInstallationResponse): response is { installed: boolean; status: LiandongInstallationStatus } {
  return 'status' in response && Boolean(response.status) && typeof response.status === 'object'
}

function normalizeInstallationResponse(response: LiandongInstallationResponse): LiandongInstallationStatus {
  if (isInstallationResult(response)) return response.status
  return response
}

function isJob(value: unknown): value is LiandongJob {
  return typeof value === 'object' && value !== null && typeof (value as Record<string, unknown>).status === 'string'
}

function normalizeJobResponse(response: LiandongJobResponse): LiandongJob | null {
  if (isJob(response)) return response
  if (typeof response === 'object' && response !== null && isJob((response as { job?: unknown }).job)) return response.job
  return null
}

function jobIdentifier(job: LiandongJob | null): string {
  return job?.id || job?.job_id || ''
}

function jobGoodsIds(job: LiandongJob): number[] {
  return job.goods_ids || job.selected_goods || []
}

function jobItems(job: LiandongJob): LiandongPreviewItem[] {
  return job.items || job.products || []
}

function statusJobs(nextStatus: LiandongStatus): LiandongJob[] {
  const candidates = [
    nextStatus.current_job,
    nextStatus.pending_job,
    ...(Array.isArray(nextStatus.jobs) ? nextStatus.jobs : []),
  ]
  return candidates.filter(isJob)
}

function applyStatus(nextStatus: LiandongStatus, options: StatusLoadOptions = {}): void {
  const previousExecutionKey = executionKey.value
  status.value = nextStatus
  products.value = Array.isArray(nextStatus.products) ? nextStatus.products.map(normalizeProduct) : []
  if (selectionMode.value === 'all') {
    selectedGoodsIds.value = products.value.map(product => product.goods_id)
  } else {
    const availableGoods = new Set(products.value.map(product => product.goods_id))
    selectedGoodsIds.value = selectedGoodsIds.value.filter(goodsId => availableGoods.has(goodsId))
  }
  const nextPending = nextStatus.pending_job
  pendingJob.value = isJob(nextPending) ? nextPending : null
  const durableJob = statusJobs(nextStatus)[0] || null
  const exposesDurableJobs = 'current_job' in nextStatus || 'pending_job' in nextStatus || Array.isArray(nextStatus.jobs)
  currentJob.value = durableJob || (exposesDurableJobs ? null : currentJob.value)
  if (!options.preservePreview || previousExecutionKey !== executionKey.value) invalidatePreview()
}

async function loadInstallation(): Promise<void> {
  loadingInstallation.value = true
  installationError.value = ''
  installationEndpointUnavailable.value = false
  installation.value = null
  try {
    const result = normalizeInstallationResponse(await liandongToolkitAPI.getInstallation())
    installation.value = result
    installationEndpointUnavailable.value = serverDeclaresUnavailable(result)
  } catch (error) {
    installationEndpointUnavailable.value = isEndpointUnavailable(error, 'installation')
    installationError.value = installationEndpointUnavailable.value
      ? ''
      : operationError(error, t('ldxpToolkit.runtime.loadFailed'), 'installation')
  } finally {
    loadingInstallation.value = false
  }
}

async function installToolkit(): Promise<void> {
  if (!installationAvailable.value) return
  installing.value = true
  installationError.value = ''
  try {
    const result = normalizeInstallationResponse(await liandongToolkitAPI.installOrRepair())
    installation.value = result
    installationEndpointUnavailable.value = serverDeclaresUnavailable(result)
    if (serverDeclaresUnavailable(result)) {
      installationError.value = t('ldxpToolkit.runtime.endpointUnavailable')
      return
    }
    appStore.showSuccess(t('ldxpToolkit.runtime.installSuccess'))
  } catch (error) {
    installationEndpointUnavailable.value = isEndpointUnavailable(error, 'installation')
    installationError.value = installationEndpointUnavailable.value
      ? ''
      : operationError(error, t('ldxpToolkit.runtime.installFailed'), 'installation')
  } finally {
    installing.value = false
  }
}

async function loadStatus(options: StatusLoadOptions = {}): Promise<boolean> {
  loadingStatus.value = true
  statusError.value = ''
  try {
    const result = await liandongToolkitAPI.getStatus()
    applyStatus(result, options)
    return true
  } catch (error) {
    status.value = null
    products.value = []
    selectedGoodsIds.value = []
    invalidatePreview()
    statusError.value = operationError(error, t('ldxpToolkit.connection.loadFailed'), 'status')
    return false
  } finally {
    loadingStatus.value = false
  }
}

function hasPositivePrice(product: LiandongProductMapping): boolean {
  return positiveCnyAmount(product.cny_amount) !== undefined
}

function updateProductPrice(product: LiandongProductMapping, event: Event): void {
  const rawValue = (event.target as HTMLInputElement | null)?.value.trim() || ''
  if (!rawValue) {
    product.cny_amount = undefined
    return
  }
  const parsed = positiveCnyAmount(rawValue)
  product.cny_amount = parsed === undefined ? undefined : parsed
}

function isConfiguredProduct(product: LiandongProductMapping): boolean {
  return positiveInteger(product.goods_id) !== undefined &&
    positiveCnyAmount(product.cny_amount) !== undefined &&
    positivePrice(product.usd_credit) !== undefined &&
    positiveInteger(product.target_stock) !== undefined &&
    normalizedGrantType(product.grant_type) === 'balance' &&
    positiveInteger(product.version) !== undefined &&
    isValidExternalUrl(product.external_url)
}

function configProduct(product: LiandongProductMapping): LiandongProductMapping {
  const cnyAmount = positiveCnyAmount(product.cny_amount)
  if (cnyAmount === undefined) throw new Error('LDXP_INVALID_PRICE')
  const payload: LiandongProductMapping = {
    goods_id: positiveInteger(product.goods_id) || 0,
    cny_amount: cnyAmount,
    usd_credit: numberOr(product.usd_credit, 0),
    target_stock: positiveInteger(product.target_stock) || DEFAULT_LIANDONG_TARGET_STOCK,
    enabled: product.enabled === true,
    grant_type: 'balance',
    version: positiveInteger(product.version) || 1,
  }
  const externalUrl = product.external_url?.trim()
  if (externalUrl) payload.external_url = externalUrl
  if (product.threshold !== undefined) payload.threshold = numberOr(product.threshold, 0)
  if (product.restock_count !== undefined) payload.restock_count = Math.max(1, numberOr(product.restock_count, 1))
  return payload
}

function configurationPayload(): LiandongConfigUpdate {
  return {
    merchant_token: merchantToken.value.trim(),
    generate_code_secret: false,
    products: products.value.map(configProduct),
  }
}

async function saveConfiguration(): Promise<void> {
  if (!products.value.length || !hasValidConfiguration.value) return
  savingConfig.value = true
  connectionFeedback.value = ''
  try {
    const result = await liandongToolkitAPI.updateConfig(configurationPayload())
    merchantToken.value = ''
    applyStatus(result)
    connectionFeedbackType.value = 'success'
    connectionFeedback.value = t('ldxpToolkit.connection.saveSuccess')
    appStore.showSuccess(t('ldxpToolkit.connection.saveSuccess'))
  } catch (error) {
    connectionFeedbackType.value = 'error'
    connectionFeedback.value = operationError(error, t('ldxpToolkit.connection.saveFailed'), 'configuration')
  } finally {
    merchantToken.value = ''
    savingConfig.value = false
  }
}

async function testMerchantConnection(): Promise<void> {
  testingConnection.value = true
  connectionFeedback.value = ''
  invalidatePreview()
  try {
    const result = await liandongToolkitAPI.testConnection()
    if (result.configured !== true || result.reachable !== true || result.read_only !== true || result.ok === false || result.available === false) {
      connectionFeedbackType.value = 'error'
      const code = result.read_only !== true ? 'LDXP_MERCHANT_NOT_READ_ONLY' : 'LDXP_MERCHANT_VALIDATION_FAILED'
      connectionFeedback.value = `${redactDisplayText(result.message || t('ldxpToolkit.connection.testFailed'))} [connection:${code}]`
      return
    }
    connectionFeedbackType.value = 'success'
    connectionFeedback.value = t('ldxpToolkit.connection.testSuccess')
  } catch (error) {
    connectionFeedbackType.value = 'error'
    connectionFeedback.value = operationError(error, t('ldxpToolkit.connection.testFailed'), 'connection')
  } finally {
    merchantToken.value = ''
    testingConnection.value = false
  }
}

function goodsFromResponse(response: LiandongRemoteGood[] | { items?: LiandongRemoteGood[]; goods?: LiandongRemoteGood[] }): LiandongRemoteGood[] {
  if (Array.isArray(response)) return response
  return Array.isArray(response.items) ? response.items : Array.isArray(response.goods) ? response.goods : []
}

async function syncRemoteGoods(): Promise<void> {
  loadingGoods.value = true
  goodsError.value = ''
  try {
    remoteGoods.value = goodsFromResponse(await liandongToolkitAPI.listGoods())
  } catch (error) {
    remoteGoods.value = []
    goodsError.value = operationError(error, t('ldxpToolkit.goods.syncFailed'), 'goods')
  } finally {
    loadingGoods.value = false
  }
}

const runtimeUnavailable = computed(() => {
  const current = installation.value
  return installationEndpointUnavailable.value || serverDeclaresUnavailable(current) || current?.version === 'unavailable'
})
const installationAvailable = computed(() => Boolean(installation.value) && !runtimeUnavailable.value && !installationError.value)
const installationInstalled = computed(() => {
  if (runtimeUnavailable.value) return false
  if (installation.value?.ready !== undefined) return installation.value.ready === true
  return Boolean(installation.value?.exists && (installation.value.executable ?? installation.value.executable_bit))
})
const platformLabel = computed(() => {
  const platform = installation.value?.platform
  if (typeof platform === 'string' && platform) return platform
  if (platform && typeof platform === 'object') return `${platform.os || installation.value?.os || '-'} / ${platform.arch || installation.value?.arch || '-'}`
  return `${installation.value?.os || '-'} / ${installation.value?.arch || '-'}`
})
const programPath = computed(() => installation.value?.expected_program_path || installation.value?.program_path || installation.value?.path || '-')

const selectedGoodsForRequest = computed(() => {
  return selectionMode.value === 'all' ? products.value.map(product => product.goods_id) : [...selectedGoodsIds.value]
})
function selectionKeyFor(mode: SelectionMode, goodsIds: readonly number[]): string {
  return `${mode}:${[...goodsIds].sort((left, right) => left - right).join(',')}`
}

const selectionKey = computed(() => selectionKeyFor(selectionMode.value, selectedGoodsForRequest.value))

function mappingVersionFor(productsToDigest: readonly LiandongProductMapping[]): string {
  return JSON.stringify(productsToDigest.map(product => ({
    goods_id: positiveInteger(product.goods_id) ?? null,
    cny_amount: positiveCnyAmount(product.cny_amount) ?? null,
    usd_credit: positivePrice(product.usd_credit) ?? null,
    target_stock: positiveInteger(product.target_stock) ?? null,
    enabled: product.enabled === true,
    grant_type: normalizedGrantType(product.grant_type),
    external_url: normalizedExternalUrl(product.external_url),
    version: positiveInteger(product.version) ?? null,
    threshold: optionalNumber(product.threshold) ?? null,
    restock_count: optionalNumber(product.restock_count) ?? null,
  })).sort((left, right) => Number(left.goods_id || 0) - Number(right.goods_id || 0)))
}

const mappingVersion = computed(() => mappingVersionFor(products.value))
const configurationVersion = computed(() => JSON.stringify({
  mapping: mappingVersion.value,
  configured: status.value?.configured ?? null,
  merchant_token_configured: status.value?.merchant_token_configured ?? null,
  code_secret_configured: status.value?.code_secret_configured ?? null,
  running: status.value?.running ?? null,
}))
const activeJob = computed(() => currentJob.value || pendingJob.value)
const statusReadyForRun = computed(() => {
  const current = status.value
  return current?.configured === true &&
    current.merchant_token_configured === true &&
    current.code_secret_configured === true &&
    current.running === false
})
const hasValidConfiguration = computed(() => products.value.length > 0 && products.value.every(isConfiguredProduct))
const validRequestedGoods = computed(() => {
  const requested = selectedGoodsForRequest.value
  if (!requested.length || requested.some(goodsId => positiveInteger(goodsId) === undefined)) return false
  const unique = new Set(requested)
  if (unique.size !== requested.length) return false
  const configuredGoods = new Set(products.value.map(product => product.goods_id))
  return requested.every(goodsId => configuredGoods.has(goodsId))
})
const selectedProductsHaveValidMappings = computed(() => {
  if (!validRequestedGoods.value) return false
  const byGoodsId = new Map(products.value.map(product => [product.goods_id, product]))
  return selectedGoodsForRequest.value.every(goodsId => {
    const product = byGoodsId.get(goodsId)
    return Boolean(product && isConfiguredProduct(product))
  })
})
const executionKey = computed(() => JSON.stringify({
  selection: selectionKey.value,
  mapping: mappingVersion.value,
  configuration: configurationVersion.value,
  job: activeJob.value ? `${jobIdentifier(activeJob.value)}:${activeJob.value.status}` : null,
}))
const hasUnsafeActiveJob = computed(() => {
  const job = activeJob.value
  if (!job) return false
  if (LIANDONG_UNSAFE_JOB_STATES.includes(job.status as typeof LIANDONG_UNSAFE_JOB_STATES[number])) return true
  return !['completed', 'failed', 'cancelled'].includes(job.status)
})
const canRun = computed(() => {
  const snapshot = previewSnapshot.value
  const hasBlockedPreviewRow = previewRows.value.some(row => !row.enabled || !row.eligible || Boolean(row.mapping_error) || Boolean(row.validation_error))
  return Boolean(
    snapshot &&
    previewValidation.value.valid &&
    previewRows.value.length === selectedGoodsForRequest.value.length &&
    snapshot.selectionKey === selectionKey.value &&
    snapshot.mappingVersion === mappingVersion.value &&
    snapshot.configurationVersion === configurationVersion.value &&
    snapshot.executionKey === executionKey.value &&
    validRequestedGoods.value &&
    selectedProductsHaveValidMappings.value &&
    statusReadyForRun.value &&
    !hasUnsafeActiveJob.value &&
    !hasBlockedPreviewRow
  )
})
const resumableJob = computed(() => Boolean(
  activeJob.value &&
  activeJob.value.status === 'failed' &&
  jobIdentifier(activeJob.value) &&
  statusReadyForRun.value &&
  !hasUnsafeActiveJob.value,
))
const jobStateNotice = computed(() => {
  const job = activeJob.value
  if (!job) return ''
  const detail = job.error || job.message ? `: ${redactDisplayText(job.error || job.message)}` : ''
  switch (job.status) {
    case 'pending':
    case 'queued':
    case 'running':
      return `${t('ldxpToolkit.preview.pendingNotice')} [preview:LDXP_JOB_${job.status.toUpperCase()}]`
    case 'needs_reconciliation':
      return `${t('ldxpToolkit.status.needs_reconciliation')}${detail} [run:LDXP_NEEDS_RECONCILIATION]`
    case 'failed':
      return `${t('ldxpToolkit.status.failed')}${detail} [run:LDXP_JOB_FAILED]`
    default:
      return ''
  }
})

function invalidatePreview(): void {
  previewRequestSequence += 1
  previewRows.value = []
  previewSnapshot.value = null
  previewValidation.value = { valid: false, errors: [] }
}

watch(selectionKey, () => invalidatePreview())
watch(mappingVersion, () => invalidatePreview())

function objectRecord(value: unknown): Record<string, unknown> | null {
  return typeof value === 'object' && value !== null ? value as Record<string, unknown> : null
}

function previewItemsFromResponse(response: unknown): unknown[] {
  if (Array.isArray(response)) return response
  const record = objectRecord(response)
  if (!record) return []
  if (Array.isArray(record.items)) return record.items
  if (Array.isArray(record.products)) return record.products
  return Array.isArray(record.preview) ? record.preview : []
}

function sameNumber(left: number | undefined, right: number | undefined): boolean {
  if (left === undefined || right === undefined) return false
  return Math.abs(left - right) <= Math.max(1, Math.abs(left), Math.abs(right)) * Number.EPSILON * 4
}

function normalizePreviewRow(item: unknown): PreviewRow {
  const record = objectRecord(item)
  const mapping = objectRecord(record?.mapping)
  const goodsId = positiveInteger(record?.goods_id) || positiveInteger(mapping?.goods_id) || 0
  const currentValue = record?.current_stock ?? record?.unsold_stock
  const currentStock = currentValue === null || currentValue === undefined ? '-' : nonNegativeInteger(currentValue) ?? '-'
  const targetStock = positiveInteger(record?.target_stock) || positiveInteger(mapping?.target_stock) || 0
  const plannedValue = record?.planned_addition !== undefined ? record.planned_addition : record?.planned
  const planned = nonNegativeInteger(plannedValue) ?? 0
  const mappingErrorValue = typeof record?.mapping_error === 'string'
    ? record.mapping_error
    : typeof record?.error === 'string'
      ? record.error
      : ''
  return {
    goods_id: goodsId,
    current_stock: currentStock,
    target_stock: targetStock,
    planned_addition: planned,
    enabled: record?.enabled === true,
    eligible: record?.eligible === true,
    reason: typeof record?.reason === 'string' ? redactDisplayText(record.reason) : '',
    mapping_error: redactDisplayText(mappingErrorValue),
    validation_error: '',
  }
}

function previewItemErrors(item: unknown, expectedGoodsId: number): string[] {
  const record = objectRecord(item)
  if (!record) return ['LDXP_PREVIEW_MALFORMED_ROW']
  const mapping = objectRecord(record.mapping)
  if (!mapping) return ['LDXP_PREVIEW_MAPPING_MISSING']

  const errors: string[] = []
  const itemGoodsId = record.goods_id === undefined ? undefined : positiveInteger(record.goods_id)
  const mappingGoodsId = positiveInteger(mapping.goods_id)
  const goodsId = itemGoodsId || mappingGoodsId
  if (goodsId !== expectedGoodsId) errors.push('LDXP_PREVIEW_UNKNOWN_GOODS')
  if (record.goods_id !== undefined && itemGoodsId === undefined) errors.push('LDXP_PREVIEW_GOODS_ID_INVALID')
  if (mappingGoodsId === undefined || mappingGoodsId !== expectedGoodsId) errors.push('LDXP_PREVIEW_MAPPING_GOODS_MISMATCH')

  const mappingKey = typeof mapping.mapping_key === 'string' ? mapping.mapping_key.trim() : ''
  const mappingVersion = positiveInteger(mapping.version)
  const mappingCny = positiveInteger(mapping.cny_amount)
  const mappingUsd = positivePrice(mapping.usd_credit)
  const mappingTarget = positiveInteger(mapping.target_stock)
  const grantType = typeof mapping.grant_type === 'string' ? mapping.grant_type.trim() : ''
  if (!mappingKey) errors.push('LDXP_PREVIEW_MAPPING_KEY_INVALID')
  if (mappingVersion === undefined) errors.push('LDXP_PREVIEW_MAPPING_VERSION_INVALID')
  if (mappingCny === undefined) errors.push('LDXP_PREVIEW_PRICE_UNKNOWN')
  if (mappingUsd === undefined) errors.push('LDXP_PREVIEW_CREDIT_INVALID')
  if (mappingTarget === undefined) errors.push('LDXP_PREVIEW_TARGET_INVALID')
  if (grantType !== 'balance') errors.push('LDXP_PREVIEW_GRANT_TYPE_INVALID')
  if (mapping.external_url !== undefined && typeof mapping.external_url !== 'string') errors.push('LDXP_PREVIEW_EXTERNAL_URL_INVALID')

  const targetStock = positiveInteger(record.target_stock)
  const plannedValue = record.planned_addition !== undefined ? record.planned_addition : record.planned
  if (targetStock === undefined || targetStock !== mappingTarget) errors.push('LDXP_PREVIEW_TARGET_MISMATCH')
  if (nonNegativeInteger(plannedValue) === undefined) errors.push('LDXP_PREVIEW_PLANNED_INVALID')
  if (typeof record.enabled !== 'boolean') errors.push('LDXP_PREVIEW_ENABLED_MISSING')
  if (typeof record.eligible !== 'boolean') errors.push('LDXP_PREVIEW_ELIGIBLE_MISSING')

  const currentValue = record.current_stock ?? record.unsold_stock
  if (currentValue !== undefined && currentValue !== null && nonNegativeInteger(currentValue) === undefined) {
    errors.push('LDXP_PREVIEW_CURRENT_STOCK_INVALID')
  }
  for (const field of ['reason', 'error', 'mapping_error'] as const) {
    if (record[field] !== undefined && typeof record[field] !== 'string') errors.push(`LDXP_PREVIEW_${field.toUpperCase()}_INVALID`)
  }

  const product = products.value.find(candidate => candidate.goods_id === expectedGoodsId)
  if (!product || !isConfiguredProduct(product)) {
    errors.push('LDXP_PREVIEW_LOCAL_MAPPING_INVALID')
  } else {
    const productCny = positiveCnyAmount(product.cny_amount)
    const productUsd = positivePrice(product.usd_credit)
    const productTarget = positiveInteger(product.target_stock)
    const productVersion = positiveInteger(product.version)
    const productGrantType = (product.grant_type || 'balance').trim()
    const productExternalURL = product.external_url?.trim() || ''
    if (!sameNumber(mappingCny, productCny)) errors.push('LDXP_PREVIEW_PRICE_STALE')
    if (!sameNumber(mappingUsd, productUsd)) errors.push('LDXP_PREVIEW_CREDIT_STALE')
    if (mappingTarget !== productTarget) errors.push('LDXP_PREVIEW_TARGET_STALE')
    if (mappingVersion !== productVersion) errors.push('LDXP_PREVIEW_MAPPING_VERSION_STALE')
    if (grantType !== productGrantType) errors.push('LDXP_PREVIEW_GRANT_TYPE_STALE')
    if ((typeof mapping.external_url === 'string' ? mapping.external_url.trim() : '') !== productExternalURL) {
      errors.push('LDXP_PREVIEW_EXTERNAL_URL_STALE')
    }
    if (record.enabled !== product.enabled) errors.push('LDXP_PREVIEW_ENABLED_STALE')
  }
  return [...new Set(errors)]
}

function validatePreviewResponse(items: readonly unknown[], requestedGoods: readonly number[]): { rows: PreviewRow[]; errors: string[] } {
  const errors: string[] = []
  const expected = new Set(requestedGoods)
  const seen = new Set<number>()
  const rows: PreviewRow[] = []
  if (!requestedGoods.length || expected.size !== requestedGoods.length) errors.push('LDXP_PREVIEW_REQUEST_INVALID')
  if (items.length !== requestedGoods.length) errors.push('LDXP_PREVIEW_COVERAGE_INCOMPLETE')

  for (const item of items) {
    const row = normalizePreviewRow(item)
    const rowErrors = previewItemErrors(item, row.goods_id)
    if (!expected.has(row.goods_id)) rowErrors.push('LDXP_PREVIEW_UNKNOWN_GOODS')
    if (seen.has(row.goods_id)) rowErrors.push('LDXP_PREVIEW_DUPLICATE_GOODS')
    if (row.goods_id > 0) seen.add(row.goods_id)
    row.validation_error = [...new Set(rowErrors)].join(', ')
    if (row.validation_error) errors.push(row.validation_error)
    rows.push(row)
  }
  for (const goodsId of requestedGoods) {
    if (!seen.has(goodsId)) errors.push(`LDXP_PREVIEW_MISSING_GOODS_${goodsId}`)
  }
  return { rows, errors: [...new Set(errors)] }
}

function sameGoodsSet(left: readonly number[], right: readonly number[]): boolean {
  return selectionKeyFor('selected', left) === selectionKeyFor('selected', right)
}

function createPreviewSnapshot(requestedGoods: readonly number[]): PreviewRequestSnapshot {
  const frozenGoods = Object.freeze([...requestedGoods])
  return Object.freeze({
    requestedGoods: frozenGoods,
    selectionKey: selectionKey.value,
    mappingVersion: mappingVersion.value,
    configurationVersion: configurationVersion.value,
    executionKey: executionKey.value,
  })
}

function previewSnapshotIsCurrent(snapshot: PreviewRequestSnapshot): boolean {
  return snapshot.selectionKey === selectionKey.value &&
    snapshot.mappingVersion === mappingVersion.value &&
    snapshot.configurationVersion === configurationVersion.value &&
    snapshot.executionKey === executionKey.value &&
    sameGoodsSet(snapshot.requestedGoods, selectedGoodsForRequest.value)
}

function previewValidationMessage(errors: readonly string[]): string {
  const detail = errors.length ? `: ${redactDisplayText(errors[0])}` : ''
  return `${t('ldxpToolkit.preview.previewFailed')} [preview:LDXP_PREVIEW_INVALID]${detail}`
}

async function runPreview(): Promise<void> {
  const requestedGoods = [...selectedGoodsForRequest.value]
  if (!requestedGoods.length || !validRequestedGoods.value) {
    invalidatePreview()
    previewError.value = 'LDXP preview requires a unique, configured goods set [preview:LDXP_INVALID_SELECTION]'
    return
  }
  const snapshot = createPreviewSnapshot(requestedGoods)
  const requestSequence = ++previewRequestSequence
  previewLoading.value = true
  previewError.value = ''
  jobError.value = ''
  previewRows.value = []
  previewSnapshot.value = null
  previewValidation.value = { valid: false, errors: [] }
  try {
    const result = await liandongToolkitAPI.previewJob({ selected_goods: [...snapshot.requestedGoods] })
    if (requestSequence !== previewRequestSequence) return
    if (!previewSnapshotIsCurrent(snapshot)) {
      invalidatePreview()
      previewError.value = 'LDXP preview was superseded by a current selection or mapping change [preview:LDXP_PREVIEW_STALE]'
      return
    }
    const validation = validatePreviewResponse(previewItemsFromResponse(result), snapshot.requestedGoods)
    previewRows.value = Object.freeze(validation.rows.map(row => Object.freeze(row)))
    previewValidation.value = Object.freeze({ valid: validation.errors.length === 0, errors: Object.freeze(validation.errors) })
    if (validation.errors.length) {
      previewError.value = previewValidationMessage(validation.errors)
      return
    }
    previewSnapshot.value = snapshot
  } catch (error) {
    if (requestSequence === previewRequestSequence) previewError.value = operationError(error, t('ldxpToolkit.preview.previewFailed'), 'preview')
  } finally {
    if (requestSequence === previewRequestSequence) previewLoading.value = false
  }
}

function requestRun(): void {
  if (canRun.value) showRunConfirmation.value = true
}

async function confirmRun(): Promise<void> {
  showRunConfirmation.value = false
  const snapshot = previewSnapshot.value
  if (!snapshot || !canRun.value) return
  if (!await loadStatus({ preservePreview: true }) || previewSnapshot.value !== snapshot || !previewSnapshotIsCurrent(snapshot) || !canRun.value) {
    jobError.value = 'The latest LDXP readiness or job state no longer permits this run [run:LDXP_RUN_PRECONDITION_FAILED]'
    return
  }
  runningJob.value = true
  jobError.value = ''
  try {
    const job = normalizeJobResponse(await liandongToolkitAPI.runJob({ selected_goods: [...snapshot.requestedGoods] }))
    if (!job) throw new Error('LDXP_RUN_INVALID_RESPONSE')
    currentJob.value = job
    await loadStatus()
  } catch (error) {
    jobError.value = operationError(error, t('ldxpToolkit.preview.runFailed'), 'run')
  } finally {
    runningJob.value = false
  }
}

async function refreshActiveJob(): Promise<void> {
  const id = jobIdentifier(activeJob.value)
  if (!id) {
    await loadStatus()
    return
  }
  loadingJob.value = true
  jobError.value = ''
  try {
    const job = normalizeJobResponse(await liandongToolkitAPI.getJob(id))
    if (!job) throw new Error('LDXP_JOB_INVALID_RESPONSE')
    currentJob.value = job
  } catch (error) {
    jobError.value = operationError(error, t('ldxpToolkit.preview.refreshFailed'), 'job_status')
  } finally {
    loadingJob.value = false
  }
}

async function resumeActiveJob(): Promise<void> {
  const id = jobIdentifier(activeJob.value)
  if (!id || !resumableJob.value) return
  if (!await loadStatus({ preservePreview: true }) || !resumableJob.value) {
    jobError.value = 'The latest LDXP readiness or job state no longer permits resume [resume:LDXP_RESUME_PRECONDITION_FAILED]'
    return
  }
  resumingJob.value = true
  jobError.value = ''
  try {
    const job = normalizeJobResponse(await liandongToolkitAPI.resumeJob(id))
    if (!job) throw new Error('LDXP_RESUME_INVALID_RESPONSE')
    currentJob.value = job
    await loadStatus()
  } catch (error) {
    jobError.value = operationError(error, t('ldxpToolkit.preview.resumeFailed'), 'resume')
  } finally {
    resumingJob.value = false
  }
}

function historyRowsFromBatch(batch: LiandongBatchStatus, index: number): HistoryRow {
  return {
    key: `batch:${batch.batch_id || index}`,
    type: 'batch',
    displayId: batch.batch_id || '-',
    jobId: batch.job_id,
    goodsId: batch.goods_id,
    count: batch.code_count,
    status: batch.status,
    createdAt: batch.created_at,
    updatedAt: batch.updated_at || batch.uploaded_at,
  }
}

const historyRows = computed<HistoryRow[]>(() => {
  const rows: HistoryRow[] = []
  const knownJobIds = new Set<string>()
  for (const job of status.value?.jobs || []) {
    const id = jobIdentifier(job)
    if (!id) continue
    knownJobIds.add(id)
    rows.push({
      key: `job:${id}`,
      type: 'job',
      displayId: id,
      jobId: id,
      goodsId: jobGoodsIds(job).length === 1 ? jobGoodsIds(job)[0] : undefined,
      count: jobItems(job).reduce((sum, item) => sum + Math.max(0, numberOr(item.planned_addition ?? item.planned, 0)), 0),
      status: job.status,
      exportAvailable: job.export_available,
      createdAt: job.created_at,
      updatedAt: job.updated_at || job.completed_at,
    })
  }
  for (const [index, batch] of (status.value?.batches || []).entries()) {
    rows.push(historyRowsFromBatch(batch, index))
  }
  const currentId = jobIdentifier(currentJob.value)
  if (currentJob.value && currentId && !knownJobIds.has(currentId) && !rows.some(row => row.displayId === currentId)) {
    rows.unshift({
      key: `current:${currentId}`,
      type: 'job',
      displayId: currentId,
      jobId: currentId,
      goodsId: jobGoodsIds(currentJob.value).length === 1 ? jobGoodsIds(currentJob.value)[0] : undefined,
      status: currentJob.value.status,
      exportAvailable: currentJob.value.export_available,
      createdAt: currentJob.value.created_at,
      updatedAt: currentJob.value.updated_at || currentJob.value.completed_at,
    })
  }
  return rows
})

function jobStatusLabel(value: string): string {
  if (value === 'queued') return t('ldxpToolkit.status.pending')
  const knownStatuses = ['pending', 'running', 'completed', 'failed', 'needs_reconciliation', 'cancelled']
  return knownStatuses.includes(value) ? t(`ldxpToolkit.status.${value}`) : redactDisplayText(value)
}

function jobStatusClass(value: string): string {
  if (value === 'completed') return 'badge-success'
  if (value === 'failed' || value === 'needs_reconciliation') return 'badge-danger'
  if (value === 'pending' || value === 'queued' || value === 'running') return 'badge-warning'
  return 'badge-gray'
}

function formatDate(value?: string): string {
  if (!value) return '-'
  const parsed = new Date(value)
  return Number.isNaN(parsed.getTime()) ? redactDisplayText(value) : parsed.toLocaleString()
}

function formatCny(value: number | null | undefined): string {
  const price = positivePrice(value)
  return price === undefined ? '-' : `CNY ${price.toFixed(2)}`
}

function remoteGoodPrice(good: LiandongRemoteGood): number | undefined {
  for (const value of [good.selling_price, good.price, good.cny_amount]) {
    const price = positivePrice(value)
    if (price !== undefined) return price
  }
  return undefined
}

function remoteGoodStock(good: LiandongRemoteGood): number {
  return numberOr(good.unsold_stock ?? good.current_stock ?? good.stock, 0)
}

function isMappedGood(goodsId: number): boolean {
  return products.value.some(product => product.goods_id === goodsId)
}

function canExportHistoryRow(row: HistoryRow): boolean {
  return row.type === 'job' && row.status === 'completed' && row.exportAvailable !== false
}

async function downloadJobExport(jobId: string | undefined): Promise<void> {
  if (!jobId) return
  exportingJobId.value = jobId
  try {
    const confirmedJob = normalizeJobResponse(await liandongToolkitAPI.getJob(jobId))
    if (!confirmedJob || !isLiandongTerminalJob(confirmedJob) || confirmedJob.export_available === false) {
      const status = confirmedJob?.status || 'unknown'
      appStore.showError(`LDXP export is available only for a confirmed completed job [export:LDXP_EXPORT_UNAVAILABLE:${status}]`)
      return
    }
    const blob = await liandongToolkitAPI.exportJob(jobId)
    const url = URL.createObjectURL(blob)
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = `ldxp-${jobId.replace(/[^A-Za-z0-9_-]/g, '_')}.csv`
    anchor.rel = 'noopener'
    anchor.click()
    URL.revokeObjectURL(url)
  } catch (error) {
    appStore.showError(operationError(error, t('ldxpToolkit.history.exportFailed'), 'export'))
  } finally {
    exportingJobId.value = ''
  }
}

onMounted(() => {
  void Promise.all([loadInstallation(), loadStatus()])
})
</script>
