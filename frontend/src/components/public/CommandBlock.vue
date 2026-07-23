<template>
  <div class="overflow-hidden border border-gray-200 bg-gray-950 dark:border-dark-700">
    <div class="flex items-center justify-between border-b border-gray-800 px-4 py-2">
      <span class="text-xs font-semibold text-gray-300">{{ title }}</span>
      <button class="flex h-8 w-8 items-center justify-center rounded-md text-gray-400 hover:bg-gray-800 hover:text-white" :title="copied ? 'Copied' : 'Copy'" @click="copyCommand">
        <Icon :name="copied ? 'check' : 'copy'" size="sm" />
      </button>
    </div>
    <pre class="overflow-x-auto p-4 text-sm leading-6 text-gray-100"><code>{{ command }}</code></pre>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{ title: string; command: string }>()
const copied = ref(false)
async function copyCommand() {
  await navigator.clipboard.writeText(props.command)
  copied.value = true
  window.setTimeout(() => { copied.value = false }, 1200)
}
</script>
