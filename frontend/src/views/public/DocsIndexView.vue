<template>
  <div class="min-h-screen bg-gray-50 text-gray-900 dark:bg-dark-950 dark:text-white">
    <header class="border-b border-gray-200 bg-white dark:border-dark-800 dark:bg-dark-900">
      <div class="mx-auto flex max-w-6xl items-center justify-between px-5 py-4">
        <router-link to="/home" class="flex items-center gap-3 font-semibold">
          <img src="/logo.png" alt="Sub2API" class="h-8 w-8 object-contain" />
          <span>Sub2API Docs</span>
        </router-link>
        <router-link to="/login" class="text-sm font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400">{{ copy.login }}</router-link>
      </div>
    </header>

    <main class="mx-auto max-w-6xl px-5 py-12 md:py-16">
      <div class="max-w-3xl">
        <p class="mb-3 text-sm font-semibold text-primary-600 dark:text-primary-400">{{ copy.eyebrow }}</p>
        <h1 class="text-3xl font-bold md:text-4xl">{{ copy.title }}</h1>
        <p class="mt-4 text-base leading-7 text-gray-600 dark:text-dark-300">{{ copy.subtitle }}</p>
      </div>

      <section class="mt-10 border-y border-gray-200 dark:border-dark-800">
        <router-link to="/docs/codex-memory" class="group grid gap-5 py-7 md:grid-cols-[auto_1fr_auto] md:items-center">
          <span class="flex h-11 w-11 items-center justify-center rounded-md bg-primary-50 text-primary-700 dark:bg-primary-950/40 dark:text-primary-300">
            <Icon name="sync" size="lg" />
          </span>
          <span>
            <span class="block text-lg font-semibold">Codex Memory</span>
            <span class="mt-1 block text-sm leading-6 text-gray-600 dark:text-dark-300">{{ copy.memory }}</span>
          </span>
          <Icon name="arrowRight" class="text-gray-400 transition-transform group-hover:translate-x-1" />
        </router-link>
      </section>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import { getLocale } from '@/i18n'

const messages = {
  zh: { login: '登录', eyebrow: '公开文档', title: '部署、接入与本地工具', subtitle: '无需登录即可查看当前实例公开提供的工具、版本和安全边界。', memory: '合并同一台电脑上的 Codex 记忆与任务记录，并保持认证凭据相互独立。' },
  en: { login: 'Sign in', eyebrow: 'PUBLIC DOCUMENTATION', title: 'Deployment, access, and local tools', subtitle: 'Review public tools, versions, and security boundaries for this instance without signing in.', memory: 'Merge local Codex memories and task transcripts on one computer while keeping credentials isolated.' },
}
const copy = computed(() => messages[getLocale()])
</script>
