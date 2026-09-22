<template>
  <details :key="page" class="card p-4 sm:p-5">
    <summary class="cursor-pointer select-none text-base font-semibold">
      填写说明与术语解释
      <span class="ml-2 text-xs font-normal text-gray-500">按需展开查看</span>
    </summary>
    <div class="mt-4">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <p class="text-sm leading-6 text-gray-500">不需要一次填完所有功能。先记录自己确实知道的付款和服务期间；没有真实资料的项目先留待补充，不用猜数。</p>
        <label class="text-xs text-gray-500">查找术语<input v-model="search" type="search" class="input mt-1" placeholder="例如：额度池、FIFO、P90、UTC" /></label>
      </div>
      <details class="mt-4 rounded-xl bg-primary-50 p-4 text-sm leading-6 text-primary-900 dark:bg-primary-950/30 dark:text-primary-200">
        <summary class="cursor-pointer font-medium">第一次用，从这里开始</summary>
        <p class="mt-2">① 在“采购与录入”记实际付款 → ② 在“成本核算”明确绑定账号及生效期间 → ③ 读取和核对用量 → ④ 有同口径容量资料后再做三档比较。预充值、流量采样和预测按实际需要使用。</p>
      </details>
      <div class="mt-4 grid items-start gap-3 lg:grid-cols-2">
        <details v-for="term in shown" :id="'cost-term-' + term.key" :key="term.key" class="min-w-0 rounded-xl border border-gray-200 p-4 dark:border-dark-700">
          <summary class="cursor-pointer font-medium">{{ term.name }}</summary>
          <p class="mt-2 text-sm leading-6 text-gray-700 dark:text-gray-300">{{ term.definition }}</p>
          <dl class="mt-3 space-y-2 text-xs leading-5">
            <div><dt class="font-medium text-gray-600 dark:text-gray-400">怎么填 / 例子</dt><dd class="mt-0.5 text-gray-500">{{ term.example }}</dd></div>
            <div><dt class="font-medium text-gray-600 dark:text-gray-400">去哪里找</dt><dd class="mt-0.5 text-gray-500">{{ term.source }}</dd></div>
            <div><dt class="font-medium text-gray-600 dark:text-gray-400">不知道时怎么办</dt><dd class="mt-0.5 text-gray-500">{{ term.missing }}</dd></div>
          </dl>
        </details>
      </div>
      <p v-if="!shown.length" class="mt-4 text-sm text-gray-500">没有匹配的术语，试试更短的词。</p>
      <p class="mt-4 text-xs text-gray-500">以上数字都只是解释填写方法的例子，不是你的实际成本、容量或推荐经营参数。</p>
    </div>
  </details>
</template>
<script setup lang="ts">
import { computed, ref } from 'vue'
import { costTerms } from './costFieldHelp'
const props = defineProps<{ page: string }>()
const search = ref('')
const shown = computed(() => { const query = search.value.trim().toLowerCase(); return costTerms.filter(term => query ? `${term.name} ${term.definition} ${term.example}`.toLowerCase().includes(query) : term.pages.includes(props.page)) })
</script>
