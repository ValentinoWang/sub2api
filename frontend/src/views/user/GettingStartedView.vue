<template>
  <AppLayout>
    <div class="mx-auto max-w-5xl space-y-8">
      <header class="border-b border-gray-200 pb-6 dark:border-dark-800">
        <div class="flex items-start gap-4">
          <div class="flex h-12 w-12 flex-shrink-0 items-center justify-center rounded-lg bg-primary-100 text-primary-700 dark:bg-primary-950/60 dark:text-primary-300">
            <Icon name="book" size="lg" />
          </div>
          <div class="min-w-0">
            <p class="text-sm font-semibold text-primary-700 dark:text-primary-300">
              {{ t('gettingStarted.startHere') }}
            </p>
            <h1 class="mt-1 text-2xl font-bold text-gray-900 dark:text-white sm:text-3xl">
              {{ t('gettingStarted.heading') }}
            </h1>
            <p class="mt-2 max-w-3xl text-sm leading-6 text-gray-600 dark:text-dark-300 sm:text-base">
              {{ t('gettingStarted.intro') }}
            </p>
          </div>
        </div>
      </header>

      <ol data-testid="getting-started-steps" class="relative space-y-0">
        <li
          v-for="(step, index) in steps"
          :key="step.id"
          class="relative grid grid-cols-[2.5rem_minmax(0,1fr)] gap-4 pb-8 last:pb-0 sm:grid-cols-[3rem_minmax(0,1fr)] sm:gap-6"
        >
          <div v-if="index < steps.length - 1" class="absolute bottom-0 left-5 top-10 w-px bg-gray-200 dark:bg-dark-700 sm:left-6" aria-hidden="true"></div>
          <div class="relative z-10 flex h-10 w-10 items-center justify-center rounded-full border-2 border-primary-200 bg-white text-sm font-bold text-primary-700 dark:border-primary-800 dark:bg-dark-900 dark:text-primary-300 sm:h-12 sm:w-12">
            {{ index + 1 }}
          </div>

          <section class="min-w-0 border-b border-gray-200 pb-8 last:border-b-0 dark:border-dark-800">
            <p class="text-xs font-semibold text-gray-500 dark:text-dark-400">
              {{ t('gettingStarted.stepLabel', { step: index + 1 }) }}
            </p>
            <div class="mt-1 flex items-start gap-3">
              <Icon :name="step.icon" size="lg" class="mt-0.5 flex-shrink-0 text-gray-500 dark:text-dark-400" />
              <div class="min-w-0">
                <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t(step.titleKey) }}</h2>
                <p class="mt-2 max-w-3xl text-sm leading-6 text-gray-600 dark:text-dark-300">{{ t(step.descriptionKey) }}</p>
              </div>
            </div>
            <div class="mt-5 flex flex-wrap gap-3">
              <RouterLink
                v-for="(action, actionIndex) in step.actions"
                :key="action.to"
                :to="action.to"
                :class="actionIndex === 0 ? 'btn btn-primary' : 'btn btn-secondary'"
                class="inline-flex min-h-10 items-center justify-center gap-2"
              >
                {{ t(action.labelKey) }}
                <Icon name="arrowRight" size="sm" />
              </RouterLink>
            </div>

            <div
              v-if="step.id === 'connect'"
              data-testid="codex-setup-methods"
              class="mt-5 grid gap-4 border-t border-gray-200 pt-5 dark:border-dark-700 md:grid-cols-2"
            >
              <div class="min-w-0 border-l-2 border-primary-500 pl-4">
                <p class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('gettingStarted.connect.ccswitchTitle') }}</p>
                <p class="mt-1 text-sm leading-6 text-gray-600 dark:text-dark-300">{{ t('gettingStarted.connect.ccswitchDescription') }}</p>
              </div>
              <div class="min-w-0 border-l-2 border-emerald-500 pl-4">
                <p class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('gettingStarted.connect.manualTitle') }}</p>
                <p class="mt-1 text-sm leading-6 text-gray-600 dark:text-dark-300">{{ t('gettingStarted.connect.manualDescription') }}</p>
              </div>
              <p class="text-xs leading-5 text-amber-700 dark:text-amber-300 md:col-span-2">
                {{ t('gettingStarted.connect.warning') }}
              </p>
            </div>
          </section>
        </li>
      </ol>

      <section class="border-t border-gray-200 pt-6 dark:border-dark-800" aria-labelledby="getting-started-help-title">
        <div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <div class="min-w-0">
            <h2 id="getting-started-help-title" class="font-semibold text-gray-900 dark:text-white">{{ t('gettingStarted.helpTitle') }}</h2>
            <p class="mt-1 max-w-2xl text-sm leading-6 text-gray-600 dark:text-dark-300">{{ t('gettingStarted.helpDescription') }}</p>
          </div>
          <div class="flex flex-wrap gap-3">
            <RouterLink to="/keys" class="inline-flex items-center gap-2 text-sm font-medium text-primary-700 hover:text-primary-800 dark:text-primary-300 dark:hover:text-primary-200">
              {{ t('gettingStarted.openKeys') }}
            </RouterLink>
            <RouterLink to="/docs/codex-memory" class="inline-flex items-center gap-2 text-sm font-medium text-primary-700 hover:text-primary-800 dark:text-primary-300 dark:hover:text-primary-200">
              {{ t('gettingStarted.openMemoryDocs') }}
              <Icon name="externalLink" size="sm" />
            </RouterLink>
          </div>
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { Icon } from '@/components/icons'

type GuideIcon = 'creditCard' | 'key' | 'terminal' | 'checkCircle' | 'sync'

interface GuideAction {
  to: string
  labelKey: string
}

interface GuideStep {
  id: string
  icon: GuideIcon
  titleKey: string
  descriptionKey: string
  actions: GuideAction[]
}

const { t } = useI18n()

const steps: GuideStep[] = [
  {
    id: 'balance',
    icon: 'creditCard',
    titleKey: 'gettingStarted.steps.balance.title',
    descriptionKey: 'gettingStarted.steps.balance.description',
    actions: [
      { to: '/purchase', labelKey: 'gettingStarted.steps.balance.primary' },
      { to: '/redeem', labelKey: 'gettingStarted.steps.balance.secondary' }
    ]
  },
  {
    id: 'key',
    icon: 'key',
    titleKey: 'gettingStarted.steps.key.title',
    descriptionKey: 'gettingStarted.steps.key.description',
    actions: [{ to: '/keys', labelKey: 'gettingStarted.steps.key.action' }]
  },
  {
    id: 'connect',
    icon: 'terminal',
    titleKey: 'gettingStarted.steps.connect.title',
    descriptionKey: 'gettingStarted.steps.connect.description',
    actions: [{ to: '/keys', labelKey: 'gettingStarted.steps.connect.action' }]
  },
  {
    id: 'verify',
    icon: 'checkCircle',
    titleKey: 'gettingStarted.steps.verify.title',
    descriptionKey: 'gettingStarted.steps.verify.description',
    actions: [
      { to: '/usage', labelKey: 'gettingStarted.steps.verify.primary' },
      { to: '/subscriptions', labelKey: 'gettingStarted.steps.verify.secondary' }
    ]
  },
  {
    id: 'memory',
    icon: 'sync',
    titleKey: 'gettingStarted.steps.memory.title',
    descriptionKey: 'gettingStarted.steps.memory.description',
    actions: [{ to: '/docs/codex-memory', labelKey: 'gettingStarted.steps.memory.action' }]
  }
]
</script>
