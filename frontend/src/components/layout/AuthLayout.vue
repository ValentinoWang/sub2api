<template>
  <div
    class="auth-root relative flex min-h-screen flex-col overflow-hidden text-gray-900 dark:text-white"
  >
    <!-- Background Layers -->
    <div class="pointer-events-none absolute inset-0 overflow-hidden" aria-hidden="true">
      <div class="auth-aurora auth-aurora-1"></div>
      <div class="auth-aurora auth-aurora-2"></div>
      <div class="auth-aurora auth-aurora-3"></div>
      <div class="auth-grid"></div>
      <div class="auth-ring auth-ring-1"></div>
      <div class="auth-ring auth-ring-2"></div>
    </div>

    <div
      class="relative z-10 mx-auto flex w-full max-w-6xl flex-1 flex-col items-center justify-center gap-10 px-4 py-10 lg:flex-row lg:gap-16 lg:px-8"
    >
      <!-- Brand Panel (desktop) -->
      <aside class="auth-brand hidden w-full max-w-lg flex-col lg:flex">
        <div v-if="settingsLoaded" class="mb-8 flex items-center gap-4">
          <div class="auth-logo-halo h-14 w-14 shrink-0">
            <img
              :src="siteLogo || '/logo.svg'"
              alt="Logo"
              class="h-14 w-14 rounded-2xl object-contain"
            />
          </div>
          <div class="min-w-0">
            <h1 class="truncate text-3xl font-extrabold tracking-tight">
              <BrandWordmark :name="siteName" />
            </h1>
            <p class="auth-domain mt-1">{{ BRAND_DOMAIN }}</p>
            <p class="mt-1 text-xs text-gray-500 dark:text-dark-300">
              {{ siteSubtitle }}
            </p>
          </div>
        </div>

        <p class="mb-1 text-2xl font-semibold leading-snug text-gray-800 dark:text-gray-100">
          {{ t('home.meme.tagline') }}
        </p>
        <p class="mb-5 text-sm text-gray-500 dark:text-dark-300">
          {{ t('home.meme.taglineSub') }}
        </p>

        <RelayStationVisual
          compact
          :left-label="t('home.station.you')"
          :core-label="t('home.station.core')"
          :latency-title="t('home.station.latencyLive')"
          :latency-probing="t('home.station.probing')"
          :latency-unavailable="t('home.station.unavailable')"
          :latency-ms="latencyMs"
          :latency-state="latencyState"
          :stamp-text="t('home.station.stamp')"
        />

        <ul class="mt-4 grid grid-cols-2 gap-3">
          <li class="auth-sell">
            <span class="auth-sell-dot" style="background: #2dd4bf"></span>
            <div>
              <p class="auth-sell-title">{{ t('home.sell.latency') }}</p>
              <p class="auth-sell-desc">{{ t('home.sell.latencyDesc') }}</p>
            </div>
          </li>
          <li class="auth-sell">
            <span class="auth-sell-dot" style="background: #fb7185"></span>
            <div>
              <p class="auth-sell-title">{{ t('home.sell.stable') }}</p>
              <p class="auth-sell-desc">{{ t('home.sell.stableDesc') }}</p>
            </div>
          </li>
          <li class="auth-sell">
            <span class="auth-sell-dot" style="background: #60a5fa"></span>
            <div>
              <p class="auth-sell-title">{{ t('home.sell.relay') }}</p>
              <p class="auth-sell-desc">{{ t('home.sell.relayDesc') }}</p>
            </div>
          </li>
          <li class="auth-sell">
            <span class="auth-sell-dot" style="background: #a78bfa"></span>
            <div>
              <p class="auth-sell-title">{{ t('home.sell.billing') }}</p>
              <p class="auth-sell-desc">{{ t('home.sell.billingDesc') }}</p>
            </div>
          </li>
        </ul>

        <div class="auth-address mt-5">
          <span class="auth-address-label">{{ t('home.address.title') }}</span>
          <code class="auth-address-url">{{ apiBaseUrl }}</code>
        </div>
      </aside>

      <!-- Form Column -->
      <div class="w-full max-w-md">
        <!-- Logo/Brand (mobile & tablet) -->
        <div class="mb-8 text-center lg:hidden">
          <template v-if="settingsLoaded">
            <div class="auth-logo-halo mx-auto mb-4 h-16 w-16">
              <img
                :src="siteLogo || '/logo.svg'"
                alt="Logo"
                class="h-16 w-16 rounded-2xl object-contain"
              />
            </div>
            <h1 class="mb-2 text-3xl font-bold tracking-tight">
              <BrandWordmark :name="siteName" />
            </h1>
            <p class="text-sm text-gray-500 dark:text-dark-400">
              {{ t('home.meme.tagline') }}
            </p>
          </template>
        </div>

        <!-- Card Container -->
        <div class="auth-card rounded-2xl p-8">
          <span class="auth-card-corner auth-card-corner-tl" aria-hidden="true"></span>
          <span class="auth-card-corner auth-card-corner-br" aria-hidden="true"></span>
          <slot />
        </div>

        <!-- Footer Links -->
        <div class="mt-6 text-center text-sm">
          <slot name="footer" />
        </div>

        <!-- Copyright -->
        <div class="mt-8 text-center text-xs text-gray-400 dark:text-dark-500">
          &copy; {{ currentYear }} {{ BRAND_DOMAIN }} · {{ t('home.meme.footer') }}
          <p class="mt-2 text-[11px] leading-relaxed">{{ t('marketing.nonOfficialShort') }}</p>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'
import { sanitizeUrl } from '@/utils/url'
import BrandWordmark from '@/components/common/BrandWordmark.vue'
import RelayStationVisual from '@/components/common/RelayStationVisual.vue'
import { useLatencyProbe } from '@/composables/useLatencyProbe'
import { BRAND_DOMAIN, resolveBrandName } from '@/constants/brand'

const { t } = useI18n()
const appStore = useAppStore()

const siteName = computed(() => resolveBrandName(appStore.siteName))
const siteLogo = computed(() =>
  sanitizeUrl(appStore.siteLogo || '', {
    allowRelative: true,
    allowDataUrl: true
  })
)
const siteSubtitle = computed(
  () => appStore.cachedPublicSettings?.site_subtitle || 'Subscription to API Conversion Platform'
)
const settingsLoaded = computed(() => appStore.publicSettingsLoaded)

const currentYear = computed(() => new Date().getFullYear())

const apiBaseUrl = computed(() => {
  const origin = typeof window !== 'undefined' ? window.location.origin : ''
  return origin && !/^https?:\/\/(localhost|127\.0\.0\.1)/.test(origin) ? origin : `https://${BRAND_DOMAIN}`
})
const { latencyMs, state: latencyState, probe: probeLatency } = useLatencyProbe()

onMounted(() => {
  appStore.fetchPublicSettings()
  void probeLatency()
})
</script>

<style scoped>
.auth-root {
  --auth-line: rgba(15, 23, 42, 0.06);
  --auth-glass: rgba(255, 255, 255, 0.78);
  --auth-glass-border: rgba(15, 23, 42, 0.08);
  background:
    radial-gradient(1000px 500px at 50% -10%, rgba(20, 184, 166, 0.14), transparent 60%), #f6f9fc;
}
.dark .auth-root {
  --auth-line: rgba(148, 163, 184, 0.08);
  --auth-glass: rgba(10, 18, 32, 0.66);
  --auth-glass-border: rgba(148, 163, 184, 0.14);
  background:
    radial-gradient(1000px 500px at 50% -10%, rgba(20, 184, 166, 0.18), transparent 60%), #050b14;
}

/* Aurora blobs */
.auth-aurora {
  position: absolute;
  border-radius: 9999px;
  filter: blur(90px);
  opacity: 0.55;
  will-change: transform;
  animation: auth-aurora-drift 24s ease-in-out infinite alternate;
}
.auth-aurora-1 {
  top: -220px;
  right: -160px;
  width: 560px;
  height: 560px;
  background: radial-gradient(
    circle at 30% 30%,
    rgba(20, 184, 166, 0.55),
    rgba(34, 211, 238, 0.25) 45%,
    transparent 70%
  );
}
.auth-aurora-2 {
  bottom: -240px;
  left: -200px;
  width: 600px;
  height: 600px;
  background: radial-gradient(
    circle at 60% 40%,
    rgba(99, 102, 241, 0.4),
    rgba(20, 184, 166, 0.2) 50%,
    transparent 72%
  );
  animation-delay: -9s;
  animation-duration: 28s;
}
.auth-aurora-3 {
  top: 40%;
  left: 45%;
  width: 380px;
  height: 380px;
  background: radial-gradient(circle, rgba(34, 211, 238, 0.32), transparent 70%);
  opacity: 0.35;
  animation-delay: -15s;
  animation-duration: 32s;
}
@keyframes auth-aurora-drift {
  0% {
    transform: translate3d(0, 0, 0) scale(1);
  }
  50% {
    transform: translate3d(36px, -28px, 0) scale(1.08);
  }
  100% {
    transform: translate3d(-28px, 36px, 0) scale(0.96);
  }
}

/* Grid with radial mask */
.auth-grid {
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(var(--auth-line) 1px, transparent 1px),
    linear-gradient(90deg, var(--auth-line) 1px, transparent 1px);
  background-size: 56px 56px;
  -webkit-mask-image: radial-gradient(ellipse 80% 75% at 50% 40%, #000 25%, transparent 100%);
  mask-image: radial-gradient(ellipse 80% 75% at 50% 40%, #000 25%, transparent 100%);
}

/* Orbit rings */
.auth-ring {
  position: absolute;
  border-radius: 9999px;
  border: 1px solid rgba(20, 184, 166, 0.16);
  animation: auth-ring-spin linear infinite;
}
.auth-ring::before {
  content: '';
  position: absolute;
  top: -4px;
  left: 50%;
  width: 8px;
  height: 8px;
  margin-left: -4px;
  border-radius: 9999px;
  background: #2dd4bf;
  box-shadow: 0 0 14px 3px rgba(45, 212, 191, 0.7);
}
.auth-ring-1 {
  top: 50%;
  left: 50%;
  width: 900px;
  height: 900px;
  margin: -450px 0 0 -450px;
  animation-duration: 70s;
}
.auth-ring-2 {
  top: 50%;
  left: 50%;
  width: 1300px;
  height: 1300px;
  margin: -650px 0 0 -650px;
  border-color: rgba(99, 102, 241, 0.12);
  animation-duration: 110s;
  animation-direction: reverse;
}
.auth-ring-2::before {
  background: #818cf8;
  box-shadow: 0 0 14px 3px rgba(129, 140, 248, 0.6);
}
@keyframes auth-ring-spin {
  from {
    transform: rotate(0deg);
  }
  to {
    transform: rotate(360deg);
  }
}

/* Brand */
.auth-logo-halo {
  position: relative;
  display: inline-flex;
  border-radius: 16px;
}
.auth-logo-halo::before {
  content: '';
  position: absolute;
  inset: -4px;
  border-radius: inherit;
  background: conic-gradient(
    from 180deg,
    rgba(20, 184, 166, 0.7),
    rgba(34, 211, 238, 0.4),
    rgba(99, 102, 241, 0.5),
    rgba(20, 184, 166, 0.7)
  );
  filter: blur(10px);
  opacity: 0.6;
  z-index: -1;
}


.auth-domain {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12px;
  letter-spacing: 0.08em;
  color: #0d9488;
}
.dark .auth-domain {
  color: #5eead4;
}

.auth-sell {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  border-radius: 14px;
  border: 1px solid var(--auth-glass-border);
  background: var(--auth-glass);
  backdrop-filter: blur(12px);
  padding: 10px 12px;
}
.auth-sell-dot {
  margin-top: 6px;
  width: 7px;
  height: 7px;
  flex-shrink: 0;
  border-radius: 9999px;
  box-shadow: 0 0 10px currentColor;
}
.auth-sell-title {
  font-size: 13px;
  font-weight: 700;
}
.auth-sell-desc {
  margin-top: 1px;
  font-size: 11px;
  line-height: 1.45;
  color: rgb(107 114 128);
}
.dark .auth-sell-desc {
  color: rgb(148 163 184);
}
.auth-address {
  display: flex;
  align-items: center;
  gap: 10px;
  border-radius: 12px;
  border: 1px solid rgba(20, 184, 166, 0.35);
  background: rgba(20, 184, 166, 0.08);
  padding: 8px 12px;
}
.auth-address-label {
  font-size: 11px;
  font-weight: 700;
  white-space: nowrap;
  color: rgb(107 114 128);
}
.dark .auth-address-label {
  color: rgb(148 163 184);
}
.auth-address-url {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 13px;
  font-weight: 600;
  color: #0f766e;
}
.dark .auth-address-url {
  color: #5eead4;
}



/* Card */
.auth-card {
  position: relative;
  background: var(--auth-glass);
  border: 1px solid var(--auth-glass-border);
  backdrop-filter: blur(20px) saturate(160%);
  -webkit-backdrop-filter: blur(20px) saturate(160%);
  box-shadow:
    0 1px 0 rgba(255, 255, 255, 0.6) inset,
    0 30px 60px -30px rgba(2, 6, 23, 0.35),
    0 0 60px -30px rgba(20, 184, 166, 0.5);
}
.dark .auth-card {
  box-shadow:
    0 1px 0 rgba(255, 255, 255, 0.06) inset,
    0 30px 60px -30px rgba(0, 0, 0, 0.8),
    0 0 80px -30px rgba(20, 184, 166, 0.45);
}
.auth-card::before {
  content: '';
  position: absolute;
  top: 0;
  left: 12%;
  right: 12%;
  height: 1px;
  background: linear-gradient(
    90deg,
    transparent,
    rgba(45, 212, 191, 0.9),
    rgba(129, 140, 248, 0.8),
    transparent
  );
}
.auth-card-corner {
  position: absolute;
  width: 14px;
  height: 14px;
  border-color: rgba(20, 184, 166, 0.5);
  border-style: solid;
}
.auth-card-corner-tl {
  top: 10px;
  left: 10px;
  border-width: 1px 0 0 1px;
}
.auth-card-corner-br {
  bottom: 10px;
  right: 10px;
  border-width: 0 1px 1px 0;
}

@media (prefers-reduced-motion: reduce) {
  .auth-aurora,
  .auth-ring {
    animation: none;
  }
}
</style>
