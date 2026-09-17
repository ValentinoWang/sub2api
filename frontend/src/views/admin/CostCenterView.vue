<template>
  <AppLayout>
    <div class="min-w-0 space-y-6">
      <header class="flex items-start gap-3">
        <span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-primary-50 text-primary-600 ring-1 ring-inset ring-primary-100 dark:bg-primary-900/25 dark:text-primary-300 dark:ring-primary-800/50">
          <Icon name="calculator" size="md" />
        </span>
        <div class="min-w-0">
          <RouterLink to="/admin/cost-center" class="text-xs text-gray-500 hover:text-primary-600 dark:text-gray-400 dark:hover:text-primary-300">
            {{ t('nav.costCenter') }}
          </RouterLink>
          <h1 class="mt-1 text-xl font-semibold tracking-tight text-gray-900 dark:text-white sm:text-2xl">
            {{ t(String(route.meta.titleKey ?? 'nav.costCenter')) }}
          </h1>
          <p v-if="route.meta.descriptionKey" class="mt-1 text-sm leading-6 text-gray-500 dark:text-gray-400">
            {{ t(String(route.meta.descriptionKey)) }}
          </p>
        </div>
      </header>

      <nav :aria-label="t('nav.costCenter')" class="grid grid-cols-2 gap-1 rounded-xl border border-gray-200 bg-white p-1 dark:border-dark-700 dark:bg-dark-800 md:flex md:flex-wrap">
        <RouterLink
          v-for="section in costCenterSections"
          :key="section.path"
          :to="section.path"
          class="min-w-0 rounded-lg px-3 py-2.5 text-center text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 motion-reduce:transition-none md:px-4"
          :class="route.path === section.path
            ? 'bg-primary-50 text-primary-700 dark:bg-primary-900/30 dark:text-primary-300'
            : 'text-gray-600 hover:bg-gray-50 hover:text-gray-900 dark:text-gray-400 dark:hover:bg-dark-700 dark:hover:text-white'"
        >
          {{ t(section.titleKey) }}
        </RouterLink>
      </nav>

      <RouterView v-slot="{ Component }">
        <KeepAlive :max="4">
          <component :is="Component" />
        </KeepAlive>
      </RouterView>
      <CostFieldGuide :page="route.path.split('/').pop() ?? 'accounting'" />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import CostFieldGuide from './cost-center/CostFieldGuide.vue'
import { RouterLink, RouterView, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { costCenterSections } from '@/router/cost-center'
import { provideCostLedger } from './cost-center/useCostLedger'

const route = useRoute()
const { t } = useI18n()
provideCostLedger()
</script>
