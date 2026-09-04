<template>
  <PublicPageLayout>
    <article>
      <p class="st-kicker">{{ t('marketing.nonOfficialShort') }}</p>
      <h1 class="st-title">{{ t('statusPage.title') }}</h1>
      <p class="st-subtitle">{{ t('statusPage.subtitle') }}</p>

      <div v-if="loading && !summary" class="st-card st-muted">{{ t('statusPage.loading') }}</div>
      <div v-else-if="error" class="st-card st-muted">{{ t('statusPage.error') }}</div>
      <div v-else-if="summary && !summary.enabled" class="st-card st-muted">{{ t('statusPage.disabled') }}</div>

      <template v-else-if="summary">
        <div class="st-overall" :class="`is-${summary.overall || 'unknown'}`">
          <span class="st-dot"></span>
          <strong>{{ t(`statusPage.overall.${summary.overall || 'unknown'}`) }}</strong>
          <span class="st-generated">{{ t('statusPage.generatedAt') }} {{ formatTime(summary.generated_at) }} · {{ t('statusPage.autoRefresh') }}</span>
        </div>

        <div v-if="summary.items.length === 0" class="st-card st-muted">{{ t('statusPage.empty') }}</div>
        <div v-else class="st-table-wrap">
          <table class="st-table">
            <thead>
              <tr>
                <th>{{ t('statusPage.columns.service') }}</th>
                <th>{{ t('statusPage.columns.status') }}</th>
                <th>{{ t('statusPage.columns.success') }}</th>
                <th>P50</th>
                <th>P95</th>
                <th>{{ t('statusPage.columns.samples') }}</th>
                <th>{{ t('statusPage.columns.lastChecked') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="item in summary.items" :key="`${item.name}-${item.model}`">
                <td>
                  <div class="st-name">{{ item.name }}</div>
                  <div class="st-model">{{ item.provider }} · {{ item.model }}</div>
                </td>
                <td><span class="st-badge" :class="`is-${item.status}`">{{ t(`statusPage.status.${item.status}`) }}</span></td>
                <td class="st-mono">{{ item.success_rate !== undefined ? `${item.success_rate}%` : '—' }}</td>
                <td class="st-mono">{{ item.latency_p50_ms !== undefined ? `${item.latency_p50_ms} ms` : '—' }}</td>
                <td class="st-mono">{{ item.latency_p95_ms !== undefined ? `${item.latency_p95_ms} ms` : '—' }}</td>
                <td class="st-mono">{{ item.samples }}<span v-if="item.window_start" class="st-window"> · {{ t('statusPage.since') }} {{ formatTime(item.window_start) }}</span></td>
                <td class="st-mono">{{ formatTime(item.last_checked_at) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </template>

      <p class="st-note">{{ t('statusPage.method') }}</p>
      <div class="st-links">
        <router-link :to="PUBLIC_PAGES.benchmarks" class="st-link">{{ t('marketing.nav.benchmarks') }} →</router-link>
        <router-link :to="PUBLIC_PAGES.models" class="st-link">{{ t('marketing.nav.models') }} →</router-link>
      </div>
    </article>
  </PublicPageLayout>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import PublicPageLayout from '@/components/layout/PublicPageLayout.vue'
import { PUBLIC_PAGES } from '@/constants/brand'
import { getPublicStatus, type PublicStatusSummary } from '@/api/publicStatus'

const { t } = useI18n()
const summary = ref<PublicStatusSummary | null>(null)
const loading = ref(true)
const error = ref(false)
let timer: ReturnType<typeof setInterval> | null = null

async function load() {
  loading.value = true
  try {
    summary.value = await getPublicStatus()
    error.value = false
  } catch {
    error.value = true
  } finally {
    loading.value = false
  }
}

function formatTime(value?: string): string {
  if (!value) return '—'
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return value
  return d.toLocaleString()
}

onMounted(() => {
  void load()
  timer = setInterval(() => void load(), 60_000)
})
onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
})
</script>

<style scoped>
.st-kicker {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 11px;
  letter-spacing: 0.2em;
  text-transform: uppercase;
  color: #0d9488;
}
.dark .st-kicker {
  color: #5eead4;
}
.st-title {
  margin-top: 10px;
  font-size: 32px;
  font-weight: 800;
  letter-spacing: -0.02em;
}
.st-subtitle {
  margin-top: 8px;
  font-size: 15px;
  color: rgb(75 85 99);
}
.dark .st-subtitle {
  color: rgb(148 163 184);
}
.st-card {
  margin-top: 20px;
  border-radius: 16px;
  border: 1px solid rgba(15, 23, 42, 0.08);
  background: rgba(255, 255, 255, 0.72);
  padding: 18px 20px;
}
.dark .st-card {
  border-color: rgba(148, 163, 184, 0.12);
  background: rgba(10, 18, 32, 0.6);
}
.st-muted {
  font-size: 14px;
  color: rgb(107 114 128);
}
.st-overall {
  margin-top: 20px;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
  border-radius: 16px;
  border: 1px solid rgba(20, 184, 166, 0.35);
  background: rgba(20, 184, 166, 0.08);
  padding: 14px 18px;
  font-size: 16px;
}
.st-overall.is-degraded {
  border-color: rgba(245, 158, 11, 0.45);
  background: rgba(245, 158, 11, 0.08);
}
.st-overall.is-failed {
  border-color: rgba(244, 63, 94, 0.45);
  background: rgba(244, 63, 94, 0.08);
}
.st-overall.is-unknown {
  border-color: rgba(148, 163, 184, 0.35);
  background: rgba(148, 163, 184, 0.08);
}
.st-dot {
  width: 10px;
  height: 10px;
  border-radius: 9999px;
  background: #2dd4bf;
  box-shadow: 0 0 10px #2dd4bf;
}
.is-degraded .st-dot {
  background: #f59e0b;
  box-shadow: 0 0 10px #f59e0b;
}
.is-failed .st-dot {
  background: #f43f5e;
  box-shadow: 0 0 10px #f43f5e;
}
.is-unknown .st-dot {
  background: #94a3b8;
  box-shadow: none;
}
.st-generated {
  margin-left: auto;
  font-size: 12px;
  color: rgb(107 114 128);
}
.st-table-wrap {
  margin-top: 16px;
  overflow-x: auto;
  border-radius: 16px;
  border: 1px solid rgba(15, 23, 42, 0.08);
  background: rgba(255, 255, 255, 0.72);
}
.dark .st-table-wrap {
  border-color: rgba(148, 163, 184, 0.12);
  background: rgba(10, 18, 32, 0.6);
}
.st-table {
  width: 100%;
  min-width: 720px;
  border-collapse: collapse;
  font-size: 13px;
}
.st-table th {
  padding: 10px 14px;
  text-align: left;
  font-size: 11px;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: rgb(107 114 128);
  border-bottom: 1px solid rgba(148, 163, 184, 0.2);
}
.st-table td {
  padding: 12px 14px;
  border-bottom: 1px solid rgba(148, 163, 184, 0.12);
  vertical-align: top;
}
.st-table tr:last-child td {
  border-bottom: 0;
}
.st-name {
  font-weight: 600;
}
.st-model {
  margin-top: 2px;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 11px;
  color: rgb(107 114 128);
}
.st-mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  white-space: nowrap;
}
.st-window {
  font-size: 11px;
  color: rgb(107 114 128);
}
.st-badge {
  display: inline-block;
  border-radius: 6px;
  padding: 2px 8px;
  font-size: 11px;
  font-weight: 700;
  background: rgba(20, 184, 166, 0.14);
  color: #0f766e;
}
.dark .st-badge {
  color: #5eead4;
}
.st-badge.is-degraded {
  background: rgba(245, 158, 11, 0.14);
  color: #b45309;
}
.dark .st-badge.is-degraded {
  color: #fbbf24;
}
.st-badge.is-failed,
.st-badge.is-error {
  background: rgba(244, 63, 94, 0.14);
  color: #be123c;
}
.dark .st-badge.is-failed,
.dark .st-badge.is-error {
  color: #fb7185;
}
.st-badge.is-unknown {
  background: rgba(148, 163, 184, 0.16);
  color: rgb(100 116 139);
}
.st-note {
  margin-top: 16px;
  font-size: 12px;
  line-height: 1.6;
  color: rgb(107 114 128);
}
.st-links {
  margin-top: 16px;
  display: flex;
  flex-wrap: wrap;
  gap: 14px;
}
.st-link {
  font-size: 13px;
  font-weight: 600;
  color: #0f766e;
}
.dark .st-link {
  color: #5eead4;
}
</style>
