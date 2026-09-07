<template>
  <div class="pub-root relative flex min-h-screen flex-col text-gray-900 dark:text-white">
    <header class="sticky top-0 z-30 border-b border-slate-200/80 bg-white/90 px-4 py-3 backdrop-blur dark:border-slate-700 dark:bg-slate-950/90 sm:px-6">
      <nav class="pub-nav mx-auto flex max-w-7xl items-center justify-between gap-3 px-0">
        <router-link to="/home" class="flex min-w-0 items-center gap-3">
          <img :src="siteLogo || '/logo.svg'" alt="Logo" class="h-8 w-8 shrink-0 rounded-lg object-contain" />
          <span class="hidden truncate text-sm font-semibold sm:inline"><BrandWordmark :name="siteName" /></span>
        </router-link>
        <div class="flex items-center gap-1 overflow-x-auto sm:gap-2">
          <router-link
            v-for="link in navLinks"
            :key="link.to"
            :to="link.to"
            class="pub-nav-link"
            :class="{ 'is-active': route.path === link.to }"
          >
            {{ link.label }}
          </router-link>
          <LocaleSwitcher />
          <router-link to="/login" class="pub-cta ml-1 whitespace-nowrap rounded-full px-3.5 py-1.5 text-xs font-semibold">
            {{ t('marketing.pages.common.login') }}
          </router-link>
        </div>
      </nav>
    </header>

    <main class="relative z-10 flex-1 px-4 pb-16 pt-10 sm:px-6" :class="{ 'public-main-wide': route.path === '/experiences' }">
      <div class="public-content mx-auto">
        <slot />
      </div>
    </main>

    <footer
      class="relative z-10"
      :class="$slots.footer ? '' : 'border-t border-gray-200/60 px-6 py-8 dark:border-white/5'"
    >
      <slot name="footer">
        <div class="mx-auto flex max-w-5xl flex-col gap-4 text-center text-xs text-gray-500 dark:text-dark-400 sm:text-left">
          <p>{{ t('marketing.disclaimer') }}</p>
          <div class="flex flex-wrap items-center justify-center gap-x-4 gap-y-2 sm:justify-start">
            <router-link v-for="link in footerLinks" :key="link.to" :to="link.to" class="hover:text-gray-900 dark:hover:text-white">
              {{ link.label }}
            </router-link>
          </div>
          <p>&copy; {{ currentYear }} <span class="font-mono">{{ BRAND_DOMAIN }}</span> · {{ t('marketing.nonOfficialShort') }}</p>
        </div>
      </slot>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { useAppStore } from '@/stores'
import BrandWordmark from '@/components/common/BrandWordmark.vue'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import { BRAND_DOMAIN, PUBLIC_PAGES, resolveBrandName } from '@/constants/brand'
import { sanitizeUrl } from '@/utils/url'

const { t } = useI18n()
const route = useRoute()
const appStore = useAppStore()

const siteName = computed(() => resolveBrandName(appStore.cachedPublicSettings?.site_name || appStore.siteName))
const siteLogo = computed(() =>
  sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true })
)
const currentYear = computed(() => new Date().getFullYear())

const navLinks = computed(() => [
  { to: PUBLIC_PAGES.codex, label: t('marketing.nav.codex') },
  { to: PUBLIC_PAGES.claudeCode, label: t('marketing.nav.claudeCode') },
  { to: PUBLIC_PAGES.openaiCompat, label: t('marketing.nav.openaiCompat') },
  { to: PUBLIC_PAGES.experiences, label: t('experiences.nav') },
  { to: PUBLIC_PAGES.publicBenefit, label: t('marketing.nav.publicBenefit') },
  { to: PUBLIC_PAGES.business, label: t('marketing.nav.business') },
  { to: PUBLIC_PAGES.benchmarks, label: t('marketing.nav.benchmarks') },
  { to: PUBLIC_PAGES.status, label: t('marketing.nav.status') }
])

const footerLinks = computed(() => [
  { to: PUBLIC_PAGES.home, label: t('marketing.pages.common.backHome') },
  { to: PUBLIC_PAGES.openaiCompat, label: t('marketing.nav.openaiCompat') },
  { to: PUBLIC_PAGES.security, label: t('marketing.nav.security') },
  { to: PUBLIC_PAGES.verify, label: t('marketing.nav.verify') },
  { to: PUBLIC_PAGES.models, label: t('marketing.nav.models') },
  { to: PUBLIC_PAGES.keyUsage, label: t('marketing.nav.keyUsage') }
])
</script>

<style scoped>
.pub-root {
  background: #f8fafc;
}
.dark .pub-root {
  background: #0b1220;
}
.pub-nav {
  width: 100%;
}
.public-content { max-width: 48rem; }
.public-main-wide .public-content { max-width: 80rem; }
.pub-nav-link {
  white-space: nowrap;
  border-radius: 8px;
  padding: 6px 10px;
  font-size: 13px;
  font-weight: 500;
  color: rgb(100 116 139);
  transition: color 0.2s ease, background-color 0.2s ease;
}
.pub-nav-link:hover,
.pub-nav-link.is-active {
  color: rgb(17 24 39);
  background: rgba(15, 23, 42, 0.06);
}
.dark .pub-nav-link {
  color: rgb(148 163 184);
}
.dark .pub-nav-link:hover,
.dark .pub-nav-link.is-active {
  color: #fff;
  background: rgba(255, 255, 255, 0.07);
}
.pub-cta {
  color: #fff;
  background: linear-gradient(135deg, #14b8a6 0%, #0891b2 60%, #4f46e5 140%);
  box-shadow: 0 8px 20px -10px rgba(20, 184, 166, 0.6);
}
@media (max-width: 640px) { .public-main-wide { padding-inline: 20px; } }
</style>
