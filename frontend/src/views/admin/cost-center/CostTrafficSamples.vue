<template>
  <section class="card space-y-3 p-4 sm:p-5">
    <div class="flex flex-wrap items-center justify-between gap-2"><h3 class="text-base font-semibold">自动流量采样</h3><button class="btn btn-secondary btn-sm" :disabled="busy" @click="load">{{ busy ? '正在读取…' : '读取采样' }}</button></div>
    <p class="text-xs leading-5 text-gray-500">采样器自动记录已配置网卡的累计收发量。容器网卡只覆盖该服务的网络命名空间，不等于云主机计费出口；确认网卡和费率之前不估算账单。</p>
    <p v-if="error" role="alert" class="text-sm text-amber-700">采样暂时不可读，请检查采集服务；缺失值不会当成 0。</p>
    <p v-else-if="loaded && !samples.length" class="text-sm text-gray-500">当前还没有可读采样。需要先连接采集源。</p>
    <div v-if="samples.length" class="table-container"><table class="table min-w-[660px]"><thead><tr><th>来源 / 网卡</th><th>采集方式</th><th>观察时间</th><th>累计接收</th><th>累计发送</th></tr></thead><tbody><tr v-for="sample in samples.slice(0, 6)" :key="sample.source + sample.interface + sample.observed_at"><td>{{ sample.source }} / {{ sample.interface }}<span class="block text-xs text-gray-500">{{ sample.scope === 'container_interface' ? '服务容器网卡，非整机账单口径' : '主机网卡，计费口径待核对' }}</span></td><td>{{ sample.adapter === 'vnstat' ? 'vnStat' : 'Linux 内核计数器' }}</td><td>{{ new Date(sample.observed_at).toLocaleString() }}</td><td>{{ bytes(sample.rx_bytes) }}</td><td>{{ bytes(sample.tx_bytes) }}</td></tr></tbody></table></div>
  </section>
</template>
<script setup lang="ts">
import { ref } from 'vue'
import { getCostTrafficSamples } from '@/api/admin/cost-center-native'
import type { NetworkSample } from '@/api/admin/cost-center-native'
const samples = ref<NetworkSample[]>([]), busy = ref(false), loaded = ref(false), error = ref(false)
function bytes(value: string) { const n = Number(value); return Number.isSafeInteger(n) ? `${(n / 1024 ** 3).toFixed(3)} GiB` : `${value} 字节` }
async function load() { busy.value = true; error.value = false; try { samples.value = await getCostTrafficSamples(); loaded.value = true } catch { error.value = true; samples.value = [] } finally { busy.value = false } }
</script>
