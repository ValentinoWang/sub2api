<template>
  <span v-if="parts" class="brand-wordmark" :title="name">
    <span class="brand-wordmark-rest">{{ parts.left }}</span>
    <span class="brand-wordmark-two" aria-hidden="true">2</span>
    <span class="brand-wordmark-build">{{ parts.right }}</span>
    <span class="sr-only">2</span>
  </span>
  <span v-else class="brand-wordmark brand-wordmark-plain">{{ name }}</span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { splitWordmark } from '@/constants/brand'

const props = defineProps<{
  name: string
}>()

const parts = computed(() => splitWordmark(props.name))
</script>

<style scoped>
.brand-wordmark {
  display: inline-flex;
  align-items: baseline;
  white-space: nowrap;
  letter-spacing: -0.03em;
}

/* "rest": muted, sleepy */
.brand-wordmark-rest {
  color: rgb(100 116 139);
  font-weight: 500;
}
.dark .brand-wordmark-rest {
  color: rgb(148 163 184);
}

/* "2": the handoff */
.brand-wordmark-two {
  margin: 0 0.02em;
  font-weight: 800;
  background: linear-gradient(135deg, #2dd4bf, #38bdf8);
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
}

/* "build": glowing, awake */
.brand-wordmark-build {
  font-weight: 800;
  background: linear-gradient(100deg, #14b8a6 0%, #0891b2 55%, #4f46e5 100%);
  background-size: 200% 100%;
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
  animation: brand-wordmark-shift 8s ease-in-out infinite alternate;
}
.dark .brand-wordmark-build {
  background-image: linear-gradient(100deg, #5eead4 0%, #67e8f9 55%, #a5b4fc 100%);
}

.brand-wordmark-plain {
  font-weight: 800;
  background: linear-gradient(100deg, #0f172a 0%, #0d9488 45%, #0891b2 70%, #4f46e5 100%);
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
}
.dark .brand-wordmark-plain {
  background-image: linear-gradient(100deg, #ffffff 0%, #5eead4 40%, #67e8f9 65%, #a5b4fc 100%);
}

@keyframes brand-wordmark-shift {
  from {
    background-position: 0% 50%;
  }
  to {
    background-position: 100% 50%;
  }
}

@media (prefers-reduced-motion: reduce) {
  .brand-wordmark-build {
    animation: none;
  }
}
</style>
