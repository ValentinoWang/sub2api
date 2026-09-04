<template>
  <PublicPageLayout>
    <article class="mk-article">
      <p class="mk-kicker">{{ t('marketing.nonOfficialShort') }}</p>
      <h1 class="mk-title">{{ t(`marketing.pages.${pageKey}.title`) }}</h1>
      <p class="mk-subtitle">{{ t(`marketing.pages.${pageKey}.subtitle`) }}</p>

      <!-- ===== Static information pages ===== -->
      <template v-if="isStaticPage">
        <section v-for="(section, index) in sections" :key="index" class="mk-section">
          <h2 class="mk-h2">{{ section.h }}</h2>
          <p v-if="section.p" class="mk-p">{{ section.p }}</p>
          <ul v-if="section.items?.length" class="mk-list">
            <li v-for="(item, i) in section.items" :key="i">{{ item }}</li>
          </ul>
        </section>
      </template>

      <!-- ===== Xianyu verification ===== -->
      <template v-else-if="pageKey === 'verify'">
        <div class="mk-store">
          <span class="mk-store-label">{{ t('marketing.pages.verify.platform') }} · {{ t('marketing.pages.verify.storeLabel') }}</span>
          <strong class="mk-store-name">{{ storeName }}</strong>
          <span class="mk-store-domain">{{ BRAND_DOMAIN }}</span>
        </div>
        <section class="mk-section">
          <ul class="mk-list">
            <li v-for="(tip, i) in stringList('marketing.pages.verify.tips')" :key="i">{{ tip }}</li>
          </ul>
        </section>
        <section v-if="contactInfo" class="mk-section">
          <h2 class="mk-h2">{{ t('marketing.pages.verify.contactLabel') }}</h2>
          <p class="mk-p whitespace-pre-wrap">{{ contactInfo }}</p>
        </section>
      </template>

      <!-- ===== Integration guides ===== -->
      <template v-else-if="isGuidePage">
        <p class="mk-p">{{ t(`marketing.pages.${pageKey}.intro`) }}</p>

        <div class="mk-address">
          <span class="mk-address-label">{{ t('marketing.pages.common.baseUrl') }}</span>
          <code class="mk-address-url">{{ apiBaseUrl }}</code>
          <button type="button" class="mk-copy" @click="copy(apiBaseUrl)">
            {{ copied ? t('marketing.pages.common.copied') : t('marketing.pages.common.copy') }}
          </button>
        </div>

        <section v-if="pageKey !== 'openaiCompat'" class="mk-section mk-byok">
          <h2 class="mk-h2">{{ t('marketing.pages.common.byokTitle') }}</h2>
          <p class="mk-p">{{ t(`marketing.pages.${pageKey}.byok`) }}</p>
        </section>

        <section class="mk-section">
          <h2 v-if="pageKey !== 'openaiCompat'" class="mk-h2">{{ t('marketing.pages.common.managedTitle') }}</h2>
          <p v-if="pageKey !== 'openaiCompat'" class="mk-p">{{ t(`marketing.pages.${pageKey}.managedIntro`) }}</p>
          <div v-for="(step, i) in steps" :key="i" class="mk-step">
            <h3 class="mk-h3">{{ step.h }}</h3>
            <p class="mk-p">{{ step.p }}</p>
            <pre v-if="codeBlocks[i]" class="mk-code"><code>{{ codeBlocks[i] }}</code></pre>
          </div>
        </section>

        <section class="mk-section">
          <h2 class="mk-h2">{{ t(`marketing.pages.${pageKey}.troubleshooting.h`) }}</h2>
          <ul class="mk-list">
            <li v-for="(item, i) in stringList(`marketing.pages.${pageKey}.troubleshooting.items`)" :key="i">{{ item }}</li>
          </ul>
        </section>
      </template>

      <!-- ===== Latency self-test ===== -->
      <template v-else-if="pageKey === 'benchmarks'">
        <p class="mk-p">{{ t('marketing.pages.benchmarks.intro') }}</p>
        <div class="mk-bench">
          <div class="mk-bench-head">
            <div>
              <span class="mk-address-label">{{ t('marketing.pages.benchmarks.target') }}</span>
              <code class="mk-address-url">{{ apiBaseUrl }}/health</code>
            </div>
            <button type="button" class="mk-copy" :disabled="bench.running" @click="runBenchmark">
              {{ bench.running ? t('marketing.pages.benchmarks.running') : bench.results.length ? t('marketing.pages.benchmarks.again') : t('marketing.pages.benchmarks.run') }}
            </button>
          </div>
          <div class="mk-bench-grid">
            <div class="mk-stat"><span>{{ t('marketing.pages.benchmarks.samples') }}</span><strong>{{ bench.results.length || '—' }}</strong></div>
            <div class="mk-stat"><span>{{ t('marketing.pages.benchmarks.success') }}</span><strong>{{ benchStats ? `${benchStats.successRate}%` : '—' }}</strong></div>
            <div class="mk-stat"><span>{{ t('marketing.pages.benchmarks.p50') }}</span><strong>{{ benchStats ? `${benchStats.p50} ms` : '—' }}</strong></div>
            <div class="mk-stat"><span>{{ t('marketing.pages.benchmarks.p95') }}</span><strong>{{ benchStats ? `${benchStats.p95} ms` : '—' }}</strong></div>
            <div class="mk-stat"><span>{{ t('marketing.pages.benchmarks.min') }}</span><strong>{{ benchStats ? `${benchStats.min} ms` : '—' }}</strong></div>
            <div class="mk-stat"><span>{{ t('marketing.pages.benchmarks.max') }}</span><strong>{{ benchStats ? `${benchStats.max} ms` : '—' }}</strong></div>
          </div>
          <div class="mk-bench-bars" aria-hidden="true">
            <span
              v-for="(r, i) in bench.results"
              :key="i"
              class="mk-bench-bar"
              :class="{ 'is-fail': r === null }"
              :style="{ height: r === null ? '100%' : `${Math.max(6, (r / (benchStats?.max || 1)) * 100)}%` }"
            ></span>
          </div>
          <p class="mk-bench-note">
            <router-link :to="PUBLIC_PAGES.status" class="underline decoration-dotted underline-offset-4">{{ t('marketing.pages.benchmarks.serverSide') }}</router-link>
            ·
            {{ t('marketing.pages.benchmarks.method') }}
            <template v-if="bench.testedAt"> · {{ t('marketing.pages.benchmarks.testedAt') }}: {{ bench.testedAt }}</template>
            <template v-else> · {{ t('marketing.pages.benchmarks.empty') }}</template>
          </p>
        </div>
      </template>

      <div class="mk-cta-row">
        <router-link to="/login" class="mk-cta">{{ t('marketing.pages.common.getStarted') }}</router-link>
        <router-link to="/home" class="mk-cta-ghost">{{ t('marketing.pages.common.backHome') }}</router-link>
      </div>
    </article>
  </PublicPageLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { useAppStore } from '@/stores'
import PublicPageLayout from '@/components/layout/PublicPageLayout.vue'
import { BRAND_DOMAIN, PUBLIC_PAGES, resolveStoreName } from '@/constants/brand'

export type MarketingPageKey =
  | 'publicBenefit'
  | 'business'
  | 'security'
  | 'verify'
  | 'codex'
  | 'claudeCode'
  | 'openaiCompat'
  | 'benchmarks'

interface Section {
  h: string
  p?: string
  items?: string[]
}

const { t, tm, rt } = useI18n()
const route = useRoute()
const appStore = useAppStore()

const pageKey = computed<MarketingPageKey>(() => (route.meta.marketingPage as MarketingPageKey) || 'publicBenefit')
const isStaticPage = computed(() => ['publicBenefit', 'business', 'security'].includes(pageKey.value))
const isGuidePage = computed(() => ['codex', 'claudeCode', 'openaiCompat'].includes(pageKey.value))

// vue-i18n returns raw resources from tm(); strings may be plain or compiled.
function str(value: unknown): string {
  if (typeof value === 'string') return value
  try {
    return rt(value as never)
  } catch {
    return ''
  }
}
function stringList(key: string): string[] {
  const raw = tm(key) as unknown
  return Array.isArray(raw) ? raw.map(str).filter(Boolean) : []
}
const sections = computed<Section[]>(() => {
  const raw = tm(`marketing.pages.${pageKey.value}.sections`) as unknown
  if (!Array.isArray(raw)) return []
  return raw.map((entry) => {
    const s = entry as Record<string, unknown>
    return {
      h: str(s.h),
      p: s.p ? str(s.p) : undefined,
      items: Array.isArray(s.items) ? s.items.map(str) : undefined
    }
  })
})
const steps = computed<Section[]>(() => {
  const raw = tm(`marketing.pages.${pageKey.value}.steps`) as unknown
  if (!Array.isArray(raw)) return []
  return raw.map((entry) => {
    const s = entry as Record<string, unknown>
    return { h: str(s.h), p: str(s.p) }
  })
})

const contactInfo = computed(() => appStore.cachedPublicSettings?.contact_info || appStore.contactInfo || '')
const storeName = computed(() => resolveStoreName(appStore.cachedPublicSettings?.xianyu_store_name))

const apiBaseUrl = computed(() => {
  const origin = typeof window !== 'undefined' ? window.location.origin : ''
  return origin && !/^https?:\/\/(localhost|127\.0\.0\.1)/.test(origin) ? origin : `https://${BRAND_DOMAIN}`
})

const KEY = computed(() => t('marketing.pages.common.apiKeyPlaceholder'))
const codeBlocks = computed<string[]>(() => {
  const base = apiBaseUrl.value
  switch (pageKey.value) {
    case 'codex':
      return [
        `# ~/.codex/config.toml
model_provider = "rest2build"
model = "gpt-5.5"

[model_providers.rest2build]
name = "Rest2Build"
base_url = "${base}/v1"
wire_api = "responses"
env_key = "REST2BUILD_API_KEY"`,
        `export REST2BUILD_API_KEY="${KEY.value}"`,
        `codex "print hello world"`
      ]
    case 'claudeCode':
      return [
        `export ANTHROPIC_BASE_URL="${base}"
export ANTHROPIC_AUTH_TOKEN="${KEY.value}"`,
        `claude`,
        ''
      ]
    case 'openaiCompat':
      return [
        `curl ${base}/v1/chat/completions \\
  -H "Authorization: Bearer ${KEY.value}" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"gpt-5.5","messages":[{"role":"user","content":"ping"}]}'`,
        `# Python · OpenAI SDK
from openai import OpenAI
client = OpenAI(base_url="${base}/v1", api_key="${KEY.value}")

# Python · Anthropic SDK
from anthropic import Anthropic
client = Anthropic(base_url="${base}", api_key="${KEY.value}")`,
        ''
      ]
    default:
      return []
  }
})

// copy
const copied = ref(false)
let copiedTimer: ReturnType<typeof setTimeout> | null = null
async function copy(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    copied.value = true
    if (copiedTimer) clearTimeout(copiedTimer)
    copiedTimer = setTimeout(() => (copied.value = false), 1600)
  } catch {
    /* leave selectable */
  }
}

// latency self-test (visitor-side, real requests)
const SAMPLES = 20
const bench = reactive<{ running: boolean; results: Array<number | null>; testedAt: string }>({
  running: false,
  results: [],
  testedAt: ''
})
let benchCancelled = false
async function runBenchmark() {
  if (bench.running || typeof fetch !== 'function') return
  bench.running = true
  bench.results = []
  benchCancelled = false
  for (let i = 0; i < SAMPLES; i++) {
    if (benchCancelled) break
    const start = performance.now()
    try {
      const res = await fetch('/health', { cache: 'no-store', credentials: 'omit' })
      bench.results.push(res.ok ? Math.round(performance.now() - start) : null)
    } catch {
      bench.results.push(null)
    }
  }
  bench.testedAt = new Date().toLocaleString()
  bench.running = false
}
const benchStats = computed(() => {
  const ok = bench.results.filter((r): r is number => r !== null).sort((a, b) => a - b)
  if (bench.results.length === 0) return null
  const q = (p: number) => (ok.length ? ok[Math.min(ok.length - 1, Math.floor(p * (ok.length - 1)))] : 0)
  return {
    successRate: Math.round((ok.length / bench.results.length) * 100),
    p50: q(0.5),
    p95: q(0.95),
    min: ok[0] ?? 0,
    max: ok[ok.length - 1] ?? 0
  }
})

onBeforeUnmount(() => {
  benchCancelled = true
  if (copiedTimer) clearTimeout(copiedTimer)
})
</script>

<style scoped>
.mk-kicker {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 11px;
  letter-spacing: 0.2em;
  text-transform: uppercase;
  color: #0d9488;
}
.dark .mk-kicker {
  color: #5eead4;
}
.mk-title {
  margin-top: 10px;
  font-size: 32px;
  font-weight: 800;
  letter-spacing: -0.02em;
  line-height: 1.15;
}
.mk-subtitle {
  margin-top: 10px;
  font-size: 16px;
  color: rgb(75 85 99);
}
.dark .mk-subtitle {
  color: rgb(148 163 184);
}
.mk-section {
  margin-top: 28px;
  border-radius: 18px;
  border: 1px solid rgba(15, 23, 42, 0.08);
  background: rgba(255, 255, 255, 0.72);
  backdrop-filter: blur(12px);
  padding: 20px 22px;
}
.dark .mk-section {
  border-color: rgba(148, 163, 184, 0.12);
  background: rgba(10, 18, 32, 0.6);
}
.mk-byok {
  border-color: rgba(20, 184, 166, 0.4);
}
.mk-h2 {
  font-size: 17px;
  font-weight: 700;
}
.mk-h3 {
  margin-top: 14px;
  font-size: 14px;
  font-weight: 700;
}
.mk-p {
  margin-top: 8px;
  font-size: 14px;
  line-height: 1.7;
  color: rgb(55 65 81);
}
.dark .mk-p {
  color: rgb(203 213 225);
}
.mk-list {
  margin-top: 8px;
  display: grid;
  gap: 6px;
  font-size: 14px;
  line-height: 1.6;
  color: rgb(55 65 81);
}
.dark .mk-list {
  color: rgb(203 213 225);
}
.mk-list li {
  position: relative;
  padding-left: 18px;
}
.mk-list li::before {
  content: '';
  position: absolute;
  left: 4px;
  top: 10px;
  width: 6px;
  height: 6px;
  border-radius: 9999px;
  background: #14b8a6;
}
.mk-code {
  margin-top: 8px;
  overflow-x: auto;
  border-radius: 12px;
  background: #070d19;
  padding: 14px 16px;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12.5px;
  line-height: 1.6;
  color: #cbd5e1;
  border: 1px solid rgba(148, 163, 184, 0.15);
}
.mk-address,
.mk-bench-head {
  margin-top: 20px;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
  border-radius: 14px;
  border: 1px solid rgba(20, 184, 166, 0.35);
  background: rgba(20, 184, 166, 0.08);
  padding: 10px 14px;
}
.mk-address-label {
  font-size: 11px;
  font-weight: 700;
  color: rgb(107 114 128);
  margin-right: 8px;
}
.dark .mk-address-label {
  color: rgb(148 163 184);
}
.mk-address-url {
  flex: 1;
  min-width: 0;
  overflow-wrap: anywhere;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 14px;
  font-weight: 600;
  color: #0f766e;
}
.dark .mk-address-url {
  color: #5eead4;
}
.mk-copy {
  border-radius: 10px;
  padding: 8px 14px;
  font-size: 13px;
  font-weight: 600;
  color: #fff;
  background: linear-gradient(135deg, #14b8a6 0%, #0891b2 60%, #4f46e5 140%);
  white-space: nowrap;
}
.mk-copy:disabled {
  opacity: 0.6;
}
.mk-store {
  margin-top: 20px;
  display: flex;
  flex-direction: column;
  gap: 6px;
  border-radius: 18px;
  border: 1px solid rgba(20, 184, 166, 0.45);
  background: rgba(20, 184, 166, 0.08);
  padding: 22px 24px;
}
.mk-store-label {
  font-size: 12px;
  font-weight: 700;
  color: #0f766e;
}
.dark .mk-store-label {
  color: #5eead4;
}
.mk-store-name {
  font-size: 26px;
  font-weight: 900;
  letter-spacing: -0.01em;
}
.mk-store-domain {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 13px;
  color: rgb(107 114 128);
}
.mk-bench {
  margin-top: 8px;
}
.mk-bench-head {
  justify-content: space-between;
}
.mk-bench-head > div {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
}
.mk-bench-grid {
  margin-top: 14px;
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 10px;
}
@media (min-width: 640px) {
  .mk-bench-grid {
    grid-template-columns: repeat(6, 1fr);
  }
}
.mk-stat {
  display: flex;
  flex-direction: column;
  gap: 4px;
  border-radius: 12px;
  border: 1px solid rgba(15, 23, 42, 0.08);
  background: rgba(255, 255, 255, 0.72);
  padding: 10px 12px;
}
.dark .mk-stat {
  border-color: rgba(148, 163, 184, 0.12);
  background: rgba(10, 18, 32, 0.6);
}
.mk-stat span {
  font-size: 11px;
  color: rgb(107 114 128);
}
.mk-stat strong {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 16px;
}
.mk-bench-bars {
  margin-top: 14px;
  display: flex;
  align-items: flex-end;
  gap: 4px;
  height: 64px;
}
.mk-bench-bar {
  flex: 1;
  border-radius: 3px 3px 0 0;
  background: linear-gradient(180deg, #2dd4bf, #0891b2);
  min-height: 4px;
}
.mk-bench-bar.is-fail {
  background: #f43f5e;
}
.mk-bench-note {
  margin-top: 10px;
  font-size: 12px;
  color: rgb(107 114 128);
}
.mk-cta-row {
  margin-top: 32px;
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}
.mk-cta {
  border-radius: 12px;
  padding: 10px 18px;
  font-size: 14px;
  font-weight: 600;
  color: #fff;
  background: linear-gradient(135deg, #14b8a6 0%, #0891b2 60%, #4f46e5 140%);
  box-shadow: 0 10px 24px -12px rgba(20, 184, 166, 0.6);
}
.mk-cta-ghost {
  border-radius: 12px;
  padding: 10px 18px;
  font-size: 14px;
  font-weight: 500;
  border: 1px solid rgba(15, 23, 42, 0.12);
}
.dark .mk-cta-ghost {
  border-color: rgba(148, 163, 184, 0.2);
}
</style>
