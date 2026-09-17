<template>
  <section class="card space-y-4 p-4 sm:p-5" aria-labelledby="cost-radar-title">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div><h2 id="cost-radar-title" class="text-base font-semibold">Codex Radar {{ tr('公开参考', 'public reference') }}</h2><p class="mt-1 max-w-3xl text-xs leading-5 text-gray-500">{{ tr('查看第三方站点公布的额度等值与速度对照。它反映站方的样本与口径，不能直接当成你账号的可用容量，也不会自动填进实际成本。', 'Read third-party quota valuations and speed comparisons. These reflect the site’s samples and definitions, not your own usable capacity, and never fill actual cost fields.') }}</p></div>
      <button class="btn btn-secondary btn-sm" :disabled="loading" @click="load">{{ loading ? tr('正在读取…', 'Reading…') : tr('读取公开参考', 'Read public reference') }}</button>
    </div>
    <p v-if="error" role="alert" class="text-sm text-amber-700 dark:text-amber-300">{{ error }}</p>
    <template v-if="reference">
      <p class="text-xs leading-5 text-gray-500">{{ tr('来源', 'Source') }}：<a :href="reference.source" target="_blank" rel="noopener noreferrer" class="text-primary-600 underline">Codex Radar</a> · {{ tr('站方更新时间标签', 'Published update label') }}：{{ reference.source_updated_label }} · {{ tr('本次读取', 'Retrieved') }}：{{ new Date(reference.retrieved_at).toLocaleString() }}</p>
      <div class="grid gap-3 md:grid-cols-3"><article v-for="row in reference.quotas" :key="row.tier + row.model_label" class="rounded-xl border border-gray-200 p-4 dark:border-dark-700"><h3 class="text-sm font-medium">{{ tier(row.tier) }} · {{ modelLabel(row.model_label) }}</h3><p class="mt-2 text-xl font-semibold tabular-nums">${{ row.reference_usd }}</p><p class="mt-1 text-xs text-gray-500">{{ tr('站方美元等值参考量', 'Site dollar-equivalent valuation') }}</p><p class="mt-3 text-xs leading-5">{{ basis(row.basis) }}</p><p class="mt-2 text-xs leading-5 text-amber-700 dark:text-amber-300">{{ tr('周期和换算价格版本尚未明确，不能直接叫“你的周容量”。不同模型的这几项也不能相加。', 'The period and conversion price version are not declared. This is not your weekly capacity, and model scenarios cannot be added together.') }}</p></article></div>
      <p class="text-xs text-gray-500">{{ tr('当前来源未提供可读的 Plus、Pro 5x 同口径数值时，这里不按倍数补算。', 'Missing Plus and Pro 5x values are not calculated from advertised multipliers.') }}</p>
      <div v-if="reference.speeds.length" class="table-container"><table class="table min-w-[420px]"><caption class="pb-2 text-left text-sm font-medium">{{ reference.speed_model_label }} · {{ tr('站方速度对照（每秒输出 token 数）', 'Site speed comparison (output tokens per second)') }}</caption><thead><tr><th>{{ tr('思考强度', 'Reasoning effort') }}</th><th>{{ tr('普通模式', 'Standard') }}</th><th>Fast</th></tr></thead><tbody><tr v-for="row in reference.speeds" :key="row.effort"><td>{{ effort(row.effort) }}</td><td>{{ row.standard_tps }}</td><td>{{ row.fast_tps }}</td></tr></tbody></table><p class="mt-2 text-xs text-gray-500">{{ tr('这是站方测试速度，不代表你的延迟、吞吐或 Fast 的实际成本。', 'These site measurements do not establish your latency, throughput or actual Fast cost.') }}</p></div>
      <p class="rounded-xl bg-gray-50 p-3 text-xs leading-6 dark:bg-dark-900">{{ tr('近 7 天平均额度：尚无完整逐日资料。近两个月重置频率：尚无完整可核对的事件序列。历史总次数、公告次数、到账次数和实际净增是不同的量，不能相互替代。', 'Seven-day mean: complete daily data unavailable. Two-month reset rate: complete verifiable event history unavailable. Historical totals, announcements, deliveries and actual net gains are different quantities.') }}</p>
    </template>
    <p v-else-if="!loading && !error" class="text-sm text-gray-500">{{ tr('点击读取后展示当前公开资料。没有读取前保持空白，不使用旧截图中的数字补位。', 'Read the current public reference to display it. Nothing is filled from old screenshots.') }}</p>
  </section>
</template>
<script setup lang="ts">
import { ref } from 'vue'
import { useCostLedger } from './useCostLedger'
import { getRadarReference } from '@/api/admin/cost-center-native'
import type { RadarReference } from '@/api/admin/cost-center-native'
const { tr, tierName: tier } = useCostLedger()
const reference = ref<RadarReference | null>(null), loading = ref(false), error = ref('')
function modelLabel(value: string) { return tr(value.endsWith(' only') ? `仅用 ${value.slice(0, -5)}` : value, value) }
function basis(value: string) { const map: Record<string, string> = { 'Provided by the site owner': '来源标注：站长提供', 'Quota observed': '来源标注：站方额度观察' }; return tr(map[value] ?? value, value) }
function effort(value: string) { return tr(({ low: '低', medium: '中', high: '高' } as Record<string, string>)[value] ?? value, value) }
async function load() { loading.value = true; error.value = ''; reference.value = null; try { reference.value = await getRadarReference() } catch { error.value = tr('本次没有读到可核对的 Radar 资料。可能是来源暂不可达或页面结构变化，请稍后重试；不会用默认容量替代。', 'No verifiable Radar data was returned. The source may be unavailable or its layout may have changed; retry later. No default capacity is substituted.') } finally { loading.value = false } }
</script>
