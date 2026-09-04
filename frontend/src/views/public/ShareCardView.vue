<template>
  <PublicPageLayout>
    <article class="mk-article">
      <p class="sc-kicker">{{ t('marketing.nonOfficialShort') }}</p>
      <h1 class="sc-title">{{ t('share.title') }}</h1>
      <p class="sc-subtitle">{{ t('share.subtitle') }}</p>

      <div class="sc-preview">
        <canvas ref="canvasRef" class="sc-canvas"></canvas>
      </div>

      <div class="sc-controls">
        <div class="sc-row">
          <label class="sc-label">{{ t('share.size') }}</label>
          <div class="sc-seg">
            <button type="button" :class="{ 'is-on': size === 'og' }" @click="size = 'og'">1200 × 630</button>
            <button type="button" :class="{ 'is-on': size === 'square' }" @click="size = 'square'">1080 × 1080</button>
          </div>
        </div>
        <div class="sc-row">
          <label class="sc-label" for="sc-note">{{ t('share.note') }}</label>
          <input id="sc-note" v-model="note" class="input" :placeholder="t('share.notePlaceholder')" maxlength="60" />
        </div>
        <div class="sc-row">
          <label class="sc-label">{{ t('share.latency') }}</label>
          <span class="sc-latency" :class="`is-${latencyState}`">
            <template v-if="latencyState === 'ok' && latencyMs !== null">{{ latencyMs }} ms</template>
            <template v-else-if="latencyState === 'error'">{{ t('home.station.unavailable') }}</template>
            <template v-else>{{ t('home.station.probing') }}</template>
          </span>
          <button type="button" class="sc-mini" @click="probeLatency">{{ t('share.remeasure') }}</button>
        </div>
        <div class="sc-actions">
          <button type="button" class="mk-cta" @click="download">{{ t('share.download') }}</button>
          <button type="button" class="mk-cta-ghost" :disabled="!canCopyImage" @click="copyImage">
            {{ copied ? t('share.copied') : t('share.copyImage') }}
          </button>
        </div>
        <p class="sc-hint">{{ t('share.hint') }}</p>
      </div>
    </article>
  </PublicPageLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'
import PublicPageLayout from '@/components/layout/PublicPageLayout.vue'
import { BRAND_DOMAIN, resolveBrandName } from '@/constants/brand'
import { useLatencyProbe } from '@/composables/useLatencyProbe'
import { canvasToBlob, downloadCanvas, renderShareCard, type ShareCardSize } from '@/composables/useShareCard'

const { t } = useI18n()
const appStore = useAppStore()
const canvasRef = ref<HTMLCanvasElement | null>(null)
const size = ref<ShareCardSize>('og')
const note = ref('')
const copied = ref(false)
const { latencyMs, state: latencyState, probe: probeLatency } = useLatencyProbe()

const siteName = computed(() => resolveBrandName(appStore.cachedPublicSettings?.site_name || appStore.siteName))
const siteUrl = computed(() => {
  const origin = typeof window !== 'undefined' ? window.location.origin : ''
  return origin && !/^https?:\/\/(localhost|127\.0\.0\.1)/.test(origin) ? origin : `https://${BRAND_DOMAIN}`
})
const canCopyImage = typeof navigator !== 'undefined' && typeof ClipboardItem !== 'undefined' && !!navigator.clipboard?.write

const meta = computed(() => {
  const date = new Date().toISOString().slice(0, 10)
  const latency = latencyState.value === 'ok' && latencyMs.value !== null ? `${t('home.station.latencyLive')} ${latencyMs.value} ms · ` : ''
  return `${latency}${date}`
})

let rendering = false
async function render() {
  if (!canvasRef.value || rendering) return
  rendering = true
  try {
    await renderShareCard(canvasRef.value, {
      size: size.value,
      brand: siteName.value,
      domain: BRAND_DOMAIN,
      headline: t('home.meme.tagline'),
      subline: t('home.meme.taglineSub'),
      note: note.value.trim() || undefined,
      meta: meta.value,
      qrText: siteUrl.value,
      qrCaption: t('share.scan'),
      footer: t('marketing.nonOfficialShort')
    })
  } finally {
    rendering = false
  }
}

async function download() {
  if (!canvasRef.value) return
  await downloadCanvas(canvasRef.value, `rest2build-${size.value}.png`)
}

async function copyImage() {
  if (!canvasRef.value || !canCopyImage) return
  try {
    const blob = await canvasToBlob(canvasRef.value)
    if (!blob) return
    await navigator.clipboard.write([new ClipboardItem({ 'image/png': blob })])
    copied.value = true
    setTimeout(() => (copied.value = false), 1600)
  } catch {
    /* clipboard unavailable */
  }
}

watch([size, note, meta, siteName], () => void render())
onMounted(async () => {
  await render()
  await probeLatency()
  await render()
})
</script>

<style scoped>
.sc-kicker {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 11px;
  letter-spacing: 0.2em;
  text-transform: uppercase;
  color: #0d9488;
}
.dark .sc-kicker {
  color: #5eead4;
}
.sc-title {
  margin-top: 10px;
  font-size: 32px;
  font-weight: 800;
  letter-spacing: -0.02em;
}
.sc-subtitle {
  margin-top: 8px;
  font-size: 15px;
  color: rgb(75 85 99);
}
.dark .sc-subtitle {
  color: rgb(148 163 184);
}
.sc-preview {
  margin-top: 22px;
  border-radius: 18px;
  overflow: hidden;
  border: 1px solid rgba(148, 163, 184, 0.2);
  box-shadow: 0 30px 60px -30px rgba(0, 0, 0, 0.6);
  background: #050b14;
}
.sc-canvas {
  display: block;
  width: 100%;
  height: auto;
}
.sc-controls {
  margin-top: 20px;
  display: grid;
  gap: 14px;
  border-radius: 18px;
  border: 1px solid rgba(15, 23, 42, 0.08);
  background: rgba(255, 255, 255, 0.72);
  padding: 18px 20px;
}
.dark .sc-controls {
  border-color: rgba(148, 163, 184, 0.12);
  background: rgba(10, 18, 32, 0.6);
}
.sc-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
}
.sc-label {
  min-width: 72px;
  font-size: 13px;
  font-weight: 600;
}
.sc-row .input {
  flex: 1;
  min-width: 200px;
}
.sc-seg {
  display: inline-flex;
  overflow: hidden;
  border-radius: 10px;
  border: 1px solid rgba(148, 163, 184, 0.3);
}
.sc-seg button {
  padding: 6px 12px;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12px;
  color: rgb(100 116 139);
}
.sc-seg button.is-on {
  color: #fff;
  background: linear-gradient(135deg, #14b8a6, #0891b2);
}
.sc-latency {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 14px;
  font-weight: 700;
}
.sc-latency.is-ok {
  color: #0f766e;
}
.dark .sc-latency.is-ok {
  color: #5eead4;
}
.sc-mini {
  border-radius: 8px;
  border: 1px solid rgba(148, 163, 184, 0.3);
  padding: 3px 10px;
  font-size: 12px;
}
.sc-actions {
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
}
.mk-cta-ghost {
  border-radius: 12px;
  padding: 10px 18px;
  font-size: 14px;
  border: 1px solid rgba(148, 163, 184, 0.3);
}
.mk-cta-ghost:disabled {
  opacity: 0.5;
}
.sc-hint {
  font-size: 12px;
  color: rgb(107 114 128);
}
</style>
