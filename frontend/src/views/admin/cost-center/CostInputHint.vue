<template><p class="mt-1 text-xs leading-5 text-gray-500 dark:text-gray-400">{{ costHint(term) }}</p></template>
<script setup lang="ts">
import { costHint } from './costFieldHelp'
defineProps<{ term: string }>()
</script>
