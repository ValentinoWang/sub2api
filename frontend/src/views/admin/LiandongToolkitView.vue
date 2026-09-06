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
          @click="loadStatus"
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
            <button type="button" class="btn btn-primary btn-sm" :disabled="savingConfig || !products.length" data-testid="save-config" @click="saveConfiguration">
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
                      <input :id="`ldxp-cny-${product.goods_id}`" v-model.number="product.cny_amount" type="number" min="1" step="1" class="input w-28" />
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
            :disabled="previewLoading || !selectedGoodsIds.length"
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

        <div v-if="previewError" class="mt-4 rounded-lg bg-red-50 p-3 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-300" role="alert">{{ previewError }}</div>
        <div v-if="jobError" class="mt-4 rounded-lg bg-amber-50 p-3 text-sm text-amber-800 dark:bg-amber-900/20 dark:text-amber-200" role="alert">{{ jobError }}</div>
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
              <tr v-for="row in previewRows" :key="row.goods_id" class="border-b border-gray-100 last:border-0 dark:border-dark-800" :data-testid="`preview-row-${row.goods_id}`">
                <td class="px-3 py-3 font-mono text-gray-900 dark:text-white">{{ row.goods_id }}</td>
                <td class="px-3 py-3">{{ row.current_stock }}</td>
                <td class="px-3 py-3">{{ row.target_stock }}</td>
                <td class="px-3 py-3 font-semibold" :class="row.planned_addition > 0 ? 'text-amber-700 dark:text-amber-300' : 'text-gray-600 dark:text-gray-300'">{{ row.planned_addition }}</td>
                <td class="px-3 py-3" :class="row.mapping_error || !row.eligible || !row.enabled ? 'text-red-600 dark:text-red-400' : 'text-gray-500 dark:text-gray-400'" :data-testid="`preview-reason-${row.goods_id}`">
                  <div v-if="row.reason">{{ row.reason }}</div>
                  <div v-if="row.mapping_error" class="mt-1">{{ row.mapping_error }}</div>
                  <span v-if="!row.reason && !row.mapping_error">-</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <p v-else class="mt-5 text-sm text-gray-500 dark:text-gray-400">{{ t('ldxpToolkit.preview.empty') }}</p>
        <div v-if="activeJob && resumableJob" class="mt-4 rounded-lg bg-amber-50 p-3 text-sm text-amber-800 dark:bg-amber-900/20 dark:text-amber-200" data-testid="pending-job-notice">
          {{ t('ldxpToolkit.preview.pendingNotice') }}
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
          <button type="button" class="btn btn-secondary btn-sm" :disabled="loadingStatus" :title="t('common.refresh')" @click="loadStatus">
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
                  <button v-if="row.jobId" type="button" class="btn btn-secondary btn-xs" :disabled="exportingJobId === row.jobId" :title="t('ldxpToolkit.history.export')" @click="downloadJobExport(row.jobId)">
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
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'
import AppLayout from '@/components/layout/AppLayout.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import {
  DEFAULT_LIANDONG_TARGET_STOCK,
  liandongToolkitAPI,
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

type ErrorRecord = { status?: number; code?: string | number; message?: string }
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
}
type HistoryRow = {
  key: string
  type: 'job' | 'batch'
  displayId: string
  jobId?: string
  goodsId?: number
  count?: number
  status: string
  createdAt?: string
  updatedAt?: string
}

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
const previewRows = ref<PreviewRow[]>([])
const previewSelectionKey = ref('')
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

function errorRecord(error: unknown): ErrorRecord {
  if (typeof error !== 'object' || error === null) return {}
  return error as ErrorRecord
}

function isEndpointUnavailable(error: unknown): boolean {
  const record = errorRecord(error)
  const statusCode = Number(record.status)
  if (statusCode === 404 || statusCode === 501 || statusCode === 503) return true
  const code = String(record.code || '').toLowerCase()
  const message = String(record.message || '').toLowerCase()
  return code.includes('unavailable') || code.includes('not_implemented') || message.includes('not available') || message.includes('unavailable') || message.includes('not implemented')
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

function operationError(error: unknown, fallback: string): string {
  if (isEndpointUnavailable(error)) return t('ldxpToolkit.errors.endpointUnavailable')
  const message = errorRecord(error).message
  return message ? redactDisplayText(message) : fallback
}

function numberOr(value: unknown, fallback: number): number {
  const parsed = typeof value === 'number' ? value : Number(value)
  return Number.isFinite(parsed) ? parsed : fallback
}

function optionalNumber(value: unknown): number | undefined {
  if (value === null || value === undefined || value === '') return undefined
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : undefined
}

function normalizeProduct(product: LiandongProductMapping): LiandongProductMapping {
  const targetStock = numberOr(product.target_stock, DEFAULT_LIANDONG_TARGET_STOCK)
  return {
    goods_id: numberOr(product.goods_id, 0),
    cny_amount: Math.max(1, Math.round(numberOr(product.cny_amount, 0))),
    usd_credit: numberOr(product.usd_credit, 0),
    target_stock: targetStock > 0 ? targetStock : DEFAULT_LIANDONG_TARGET_STOCK,
    enabled: product.enabled !== false,
    grant_type: product.grant_type,
    external_url: product.external_url,
    version: optionalNumber(product.version),
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

function normalizeJobResponse(response: LiandongJobResponse): LiandongJob {
  if ('status' in response) return response
  return response.job
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
  return candidates.filter((job): job is LiandongJob => Boolean(job))
}

function applyStatus(nextStatus: LiandongStatus): void {
  status.value = nextStatus
  products.value = Array.isArray(nextStatus.products) ? nextStatus.products.map(normalizeProduct) : []
  selectedGoodsIds.value = products.value.map(product => product.goods_id)
  const nextPending = nextStatus.pending_job
  pendingJob.value = nextPending ? normalizeJobResponse(nextPending) : null
  const durableJob = statusJobs(nextStatus)[0] || null
  const exposesDurableJobs = 'current_job' in nextStatus || 'pending_job' in nextStatus || Array.isArray(nextStatus.jobs)
  currentJob.value = durableJob || (exposesDurableJobs ? null : currentJob.value)
  previewRows.value = []
  previewSelectionKey.value = ''
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
    installationEndpointUnavailable.value = isEndpointUnavailable(error)
    installationError.value = installationEndpointUnavailable.value
      ? ''
      : operationError(error, t('ldxpToolkit.runtime.loadFailed'))
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
    installationEndpointUnavailable.value = isEndpointUnavailable(error)
    installationError.value = installationEndpointUnavailable.value
      ? ''
      : operationError(error, t('ldxpToolkit.runtime.installFailed'))
  } finally {
    installing.value = false
  }
}

async function loadStatus(): Promise<void> {
  loadingStatus.value = true
  statusError.value = ''
  try {
    const result = await liandongToolkitAPI.getStatus()
    applyStatus(result)
  } catch (error) {
    status.value = null
    products.value = []
    selectedGoodsIds.value = []
    statusError.value = operationError(error, t('ldxpToolkit.connection.loadFailed'))
  } finally {
    loadingStatus.value = false
  }
}

function configProduct(product: LiandongProductMapping): LiandongProductMapping {
  const payload: LiandongProductMapping = {
    goods_id: numberOr(product.goods_id, 0),
    cny_amount: numberOr(product.cny_amount, 0),
    usd_credit: numberOr(product.usd_credit, 0),
    target_stock: Math.max(1, numberOr(product.target_stock, DEFAULT_LIANDONG_TARGET_STOCK)),
    enabled: product.enabled === true,
    grant_type: product.grant_type || 'balance',
    version: numberOr(product.version, 1),
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
  if (!products.value.length) return
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
    connectionFeedback.value = operationError(error, t('ldxpToolkit.connection.saveFailed'))
  } finally {
    savingConfig.value = false
  }
}

async function testMerchantConnection(): Promise<void> {
  testingConnection.value = true
  connectionFeedback.value = ''
  try {
    const result = await liandongToolkitAPI.testConnection()
    merchantToken.value = ''
    if (result.configured !== true || result.reachable !== true || result.ok === false || result.available === false) {
      connectionFeedbackType.value = 'error'
      connectionFeedback.value = redactDisplayText(result.message || t('ldxpToolkit.connection.testFailed'))
      return
    }
    connectionFeedbackType.value = 'success'
    connectionFeedback.value = t('ldxpToolkit.connection.testSuccess')
  } catch (error) {
    connectionFeedbackType.value = 'error'
    connectionFeedback.value = operationError(error, t('ldxpToolkit.connection.testFailed'))
  } finally {
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
    goodsError.value = operationError(error, t('ldxpToolkit.goods.syncFailed'))
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
  if (selectionMode.value === 'all') return products.value.map(product => product.goods_id).filter(id => id > 0)
  return selectedGoodsIds.value.filter(id => id > 0)
})
const selectionKey = computed(() => `${selectionMode.value}:${selectedGoodsForRequest.value.join(',')}`)
const activeJob = computed(() => currentJob.value || pendingJob.value)
const canRun = computed(() => {
  const activeStatus = activeJob.value?.status
  const hasActiveJob = activeStatus && ['pending', 'queued', 'running', 'needs_reconciliation'].includes(activeStatus)
  const hasBlockedPreviewRow = previewRows.value.some(row => !row.enabled || !row.eligible || Boolean(row.mapping_error))
  return previewRows.value.length > 0 && previewSelectionKey.value === selectionKey.value && !hasBlockedPreviewRow && !hasActiveJob
})
const resumableJob = computed(() => Boolean(activeJob.value && activeJob.value.status === 'failed' && jobIdentifier(activeJob.value)))

function previewItemsFromResponse(response: LiandongPreviewItem[] | { items?: LiandongPreviewItem[]; products?: LiandongPreviewItem[]; preview?: LiandongPreviewItem[] }): LiandongPreviewItem[] {
  if (Array.isArray(response)) return response
  if (Array.isArray(response.items)) return response.items
  if (Array.isArray(response.products)) return response.products
  return Array.isArray(response.preview) ? response.preview : []
}

function normalizePreviewRow(item: LiandongPreviewItem): PreviewRow {
  const goodsId = numberOr(item.goods_id, item.mapping?.goods_id || 0)
  const product = products.value.find(candidate => candidate.goods_id === goodsId)
  const currentValue = item.current_stock ?? item.unsold_stock
  const currentStock = currentValue === null || currentValue === undefined ? '-' : numberOr(currentValue, 0)
  const targetStock = numberOr(item.target_stock ?? item.mapping?.target_stock, product?.target_stock || DEFAULT_LIANDONG_TARGET_STOCK)
  const plannedValue = item.planned_addition ?? item.planned
  const planned = plannedValue === undefined
    ? typeof currentStock === 'number' ? Math.max(0, targetStock - currentStock) : 0
    : Math.max(0, numberOr(plannedValue, 0))
  const mappingError = redactDisplayText(item.mapping_error || item.error || '')
  return {
    goods_id: goodsId,
    current_stock: currentStock,
    target_stock: targetStock,
    planned_addition: planned,
    enabled: item.enabled !== false,
    eligible: item.eligible === true,
    reason: redactDisplayText(item.reason || ''),
    mapping_error: mappingError,
  }
}

async function runPreview(): Promise<void> {
  if (!selectedGoodsForRequest.value.length) return
  previewLoading.value = true
  previewError.value = ''
  jobError.value = ''
  previewRows.value = []
  previewSelectionKey.value = ''
  try {
    const result = await liandongToolkitAPI.previewJob({ selected_goods: selectedGoodsForRequest.value })
    previewRows.value = previewItemsFromResponse(result).map(normalizePreviewRow)
    previewSelectionKey.value = selectionKey.value
  } catch (error) {
    previewError.value = operationError(error, t('ldxpToolkit.preview.previewFailed'))
  } finally {
    previewLoading.value = false
  }
}

function requestRun(): void {
  if (canRun.value) showRunConfirmation.value = true
}

async function confirmRun(): Promise<void> {
  showRunConfirmation.value = false
  if (!canRun.value) return
  runningJob.value = true
  jobError.value = ''
  try {
    currentJob.value = normalizeJobResponse(await liandongToolkitAPI.runJob({ selected_goods: selectedGoodsForRequest.value }))
    await loadStatus()
  } catch (error) {
    jobError.value = operationError(error, t('ldxpToolkit.preview.runFailed'))
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
    currentJob.value = normalizeJobResponse(await liandongToolkitAPI.getJob(id))
  } catch (error) {
    jobError.value = operationError(error, t('ldxpToolkit.preview.refreshFailed'))
  } finally {
    loadingJob.value = false
  }
}

async function resumeActiveJob(): Promise<void> {
  const id = jobIdentifier(activeJob.value)
  if (!id || !resumableJob.value) return
  resumingJob.value = true
  jobError.value = ''
  try {
    currentJob.value = normalizeJobResponse(await liandongToolkitAPI.resumeJob(id))
    await loadStatus()
  } catch (error) {
    jobError.value = operationError(error, t('ldxpToolkit.preview.resumeFailed'))
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

function formatCny(value: number | undefined): string {
  return typeof value === 'number' && Number.isFinite(value) ? `CNY ${value.toFixed(2)}` : '-'
}

function remoteGoodPrice(good: LiandongRemoteGood): number | undefined {
  for (const value of [good.selling_price, good.price, good.cny_amount]) {
    if (typeof value === 'number' && Number.isFinite(value)) return value
  }
  return undefined
}

function remoteGoodStock(good: LiandongRemoteGood): number {
  return numberOr(good.unsold_stock ?? good.current_stock ?? good.stock, 0)
}

function isMappedGood(goodsId: number): boolean {
  return products.value.some(product => product.goods_id === goodsId)
}

async function downloadJobExport(jobId: string): Promise<void> {
  exportingJobId.value = jobId
  try {
    const blob = await liandongToolkitAPI.exportJob(jobId)
    const url = URL.createObjectURL(blob)
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = `ldxp-${jobId}.csv`
    anchor.rel = 'noopener'
    anchor.click()
    URL.revokeObjectURL(url)
  } catch (error) {
    appStore.showError(operationError(error, t('ldxpToolkit.history.exportFailed')))
  } finally {
    exportingJobId.value = ''
  }
}

onMounted(() => {
  void Promise.all([loadInstallation(), loadStatus()])
})
</script>
