<template>
  <div class="pub-root relative flex min-h-screen flex-col overflow-hidden text-gray-900 dark:text-white">
    <div class="pointer-events-none absolute inset-0 overflow-hidden" aria-hidden="true">
      <div class="pub-aurora pub-aurora-1"></div>
      <div class="pub-aurora pub-aurora-2"></div>
      <div class="pub-grid"></div>
    </div>

    <header class="sticky top-0 z-30 px-4 pt-4 sm:px-6">
      <nav class="pub-nav mx-auto flex max-w-5xl items-center justify-between gap-3 rounded-2xl px-3 py-2 sm:px-4">
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

    <main class="relative z-10 flex-1 px-4 pb-16 pt-10 sm:px-6">
      <div class="mx-auto max-w-3xl">
        <slot />
      </div>
    </main>

    <footer class="relative z-10 border-t border-gray-200/60 px-6 py-8 dark:border-white/5">
      <div class="mx-auto flex max-w-5xl flex-col gap-4 text-center text-xs text-gray-500 dark:text-dark-400 sm:text-left">
        <p>{{ t('marketing.disclaimer') }}</p>
        <div class="flex flex-wrap items-center justify-center gap-x-4 gap-y-2 sm:justify-start">
          <router-link v-for="link in footerLinks" :key="link.to" :to="link.to" class="hover:text-gray-900 dark:hover:text-white">
            {{ link.label }}
          </router-link>
        </div>
        <p>&copy; {{ currentYear }} <span class="font-mono">{{ BRAND_DOMAIN }}</span> · {{ t('marketing.nonOfficialShort') }}</p>
      </div>
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
  { to: PUBLIC_PAGES.publicBenefit, label: t('marketing.nav.publicBenefit') },
  { to: PUBLIC_PAGES.business, label: t('marketing.nav.business') },
  { to: PUBLIC_PAGES.benchmarks, label: t('marketing.nav.benchmarks') }
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
  --pub-line: rgba(15, 23, 42, 0.06);
  --pub-glass: rgba(255, 255, 255, 0.72);
  --pub-glass-border: rgba(15, 23, 42, 0.08);
  background:
    radial-gradient(1000px 500px at 50% -10%, rgba(20, 184, 166, 0.12), transparent 60%),
    #f6f9fc;
}
.dark .pub-root {
  --pub-line: rgba(148, 163, 184, 0.08);
  --pub-glass: rgba(10, 18, 32, 0.6);
  --pub-glass-border: rgba(148, 163, 184, 0.12);
  background:
    radial-gradient(1000px 500px at 50% -10%, rgba(20, 184, 166, 0.16), transparent 60%),
    #050b14;
}
.pub-aurora {
  position: absolute;
  border-radius: 9999px;
  filter: blur(90px);
  opacity: 0.45;
}
.pub-aurora-1 {
  top: -220px;
  right: -160px;
  width: 540px;
  height: 540px;
  background: radial-gradient(circle at 30% 30%, rgba(20, 184, 166, 0.5), rgba(34, 211, 238, 0.2) 45%, transparent 70%);
}
.pub-aurora-2 {
  bottom: -240px;
  left: -200px;
  width: 560px;
  height: 560px;
  background: radial-gradient(circle at 60% 40%, rgba(99, 102, 241, 0.35), transparent 70%);
}
.pub-grid {
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(var(--pub-line) 1px, transparent 1px),
    linear-gradient(90deg, var(--pub-line) 1px, transparent 1px);
  background-size: 56px 56px;
  -webkit-mask-image: radial-gradient(ellipse 80% 60% at 50% 10%, #000 30%, transparent 100%);
  mask-image: radial-gradient(ellipse 80% 60% at 50% 10%, #000 30%, transparent 100%);
}
.pub-nav {
  background: var(--pub-glass);
  border: 1px solid var(--pub-glass-border);
  backdrop-filter: blur(18px) saturate(160%);
}
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
</style>
