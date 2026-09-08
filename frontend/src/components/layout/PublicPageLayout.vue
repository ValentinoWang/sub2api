<template>
  <div class="pub-root relative flex min-h-screen flex-col text-gray-900 dark:text-white">
    <header class="pub-header sticky top-0 z-30 px-4 py-3 sm:px-6">
      <nav class="pub-nav mx-auto flex max-w-7xl items-center justify-between gap-3 px-0">
        <router-link to="/home" class="flex min-w-0 items-center gap-3">
          <img :src="siteLogo || '/logo.svg'" alt="Logo" class="h-8 w-8 shrink-0 rounded-lg object-contain" />
          <span class="hidden truncate text-sm font-semibold sm:inline"><BrandWordmark :name="siteName" /></span>
        </router-link>
        <div class="pub-nav-actions flex min-w-0 items-center gap-1 sm:gap-2">
          <div class="pub-nav-links flex min-w-0 items-center gap-1 overflow-x-auto sm:gap-2">
          <router-link
            v-for="link in navLinks"
            :key="link.to"
            :to="link.to"
            class="pub-nav-link"
            :class="{ 'is-active': route.path === link.to }"
          >
            {{ link.label }}
          </router-link>
          </div>
          <LocaleSwitcher />
          <router-link to="/login" class="pub-cta ml-1 inline-flex min-h-8 items-center whitespace-nowrap rounded-md px-3.5 py-1.5 text-xs font-semibold">
            {{ t('marketing.pages.common.login') }}
          </router-link>
        </div>
      </nav>
    </header>

    <main class="relative z-10 flex-1 px-4 pb-16 pt-10 sm:px-6" :class="{ 'public-main-wide': isWideSurface }">
      <div class="public-content mx-auto">
        <slot />
      </div>
    </main>

    <footer class="relative z-10">
      <slot name="footer">
        <Rest2BuildBrandFooter />
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
import Rest2BuildBrandFooter from '@/components/common/Rest2BuildBrandFooter.vue'
import { PUBLIC_PAGES, resolveBrandName } from '@/constants/brand'
import { sanitizeUrl } from '@/utils/url'

const { t } = useI18n()
const route = useRoute()
const appStore = useAppStore()

const siteName = computed(() => resolveBrandName(appStore.cachedPublicSettings?.site_name || appStore.siteName))
const siteLogo = computed(() =>
  sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true })
)
const widePublicPaths: ReadonlySet<string> = new Set([
  PUBLIC_PAGES.experiences,
  PUBLIC_PAGES.codex,
  PUBLIC_PAGES.claudeCode,
  PUBLIC_PAGES.openaiCompat,
  PUBLIC_PAGES.publicBenefit,
  PUBLIC_PAGES.business,
  PUBLIC_PAGES.security,
  PUBLIC_PAGES.benchmarks,
])
const isWideSurface = computed(() => widePublicPaths.has(route.path))

const navLinks = computed(() => [
  { to: PUBLIC_PAGES.experiences, label: t('experiences.nav') },
  { to: PUBLIC_PAGES.publicBenefit, label: t('marketing.nav.publicBenefit') },
  { to: PUBLIC_PAGES.business, label: t('marketing.nav.business') },
  { to: PUBLIC_PAGES.benchmarks, label: t('marketing.nav.benchmarks') },
  { to: PUBLIC_PAGES.status, label: t('marketing.nav.status') }
])

</script>

<style scoped>
.pub-root {
  background: #f4f7f6;
}
.dark .pub-root {
  background: #07111d;
}
.pub-header {
  border-bottom: 1px solid rgba(15, 23, 42, 0.1);
  background: rgba(250, 252, 251, 0.96);
}
.dark .pub-header {
  border-color: rgba(148, 163, 184, 0.18);
  background: #0d1826;
}
.pub-nav {
  width: 100%;
}
.public-content { max-width: 48rem; }
.public-main-wide .public-content { max-width: 76rem; }
.pub-nav-actions {
  justify-content: flex-end;
}
.pub-nav-links {
  scrollbar-width: none;
}
.pub-nav-links::-webkit-scrollbar {
  display: none;
}
.pub-nav-link {
  white-space: nowrap;
  border-radius: 6px;
  padding: 6px 10px;
  font-size: 13px;
  font-weight: 600;
  color: #52615c;
  transition: color 0.2s ease, background-color 0.2s ease, box-shadow 0.2s ease;
}
.pub-nav-link:hover,
.pub-nav-link.is-active {
  color: #115e59;
  background: #e2f1eb;
}
.dark .pub-nav-link {
  color: #aabcc8;
}
.dark .pub-nav-link:hover,
.dark .pub-nav-link.is-active {
  color: #ccfbf1;
  background: #12302f;
}
.pub-cta {
  color: #fff;
  background: #0f766e;
  box-shadow: 0 5px 14px -10px rgba(15, 118, 110, 0.8);
}
.pub-cta:hover {
  background: #115e59;
}
.pub-nav-link:focus-visible,
.pub-cta:focus-visible {
  outline: 3px solid #2dd4bf;
  outline-offset: 2px;
}
@media (max-width: 640px) {
  .pub-nav {
    align-items: center;
  }
  .pub-nav-actions {
    max-width: calc(100% - 56px);
  }
  .pub-nav-links {
    max-width: min(42vw, 240px);
  }
  .public-main-wide { padding-inline: 20px; }
}
</style>
