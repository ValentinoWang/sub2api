<template>
  <div class="min-h-screen bg-white text-gray-900 dark:bg-dark-950 dark:text-white">
    <header class="sticky top-0 z-20 border-b border-gray-200 bg-white/95 backdrop-blur dark:border-dark-800 dark:bg-dark-950/95">
      <div class="mx-auto flex max-w-6xl items-center justify-between px-5 py-4">
        <router-link to="/docs" class="inline-flex items-center gap-2 text-sm font-medium text-gray-600 hover:text-gray-900 dark:text-dark-300 dark:hover:text-white">
          <Icon name="arrowLeft" size="sm" /> {{ copy.docs }}
        </router-link>
        <router-link to="/login" class="text-sm font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400">{{ copy.login }}</router-link>
      </div>
    </header>

    <main>
      <section class="border-b border-gray-200 bg-gray-50 dark:border-dark-800 dark:bg-dark-900">
        <div class="mx-auto max-w-6xl px-5 py-12 md:py-16">
          <div class="max-w-4xl">
            <div class="mb-5 flex items-center gap-3">
              <span class="flex h-11 w-11 items-center justify-center rounded-md bg-primary-100 text-primary-700 dark:bg-primary-950/50 dark:text-primary-300"><Icon name="sync" size="lg" /></span>
              <span class="text-sm font-semibold text-primary-700 dark:text-primary-300">Codex Memory CLI</span>
            </div>
            <h1 class="text-3xl font-bold md:text-4xl">{{ copy.title }}</h1>
            <p class="mt-5 max-w-3xl text-base leading-7 text-gray-600 dark:text-dark-300">{{ copy.intro }}</p>
            <div class="mt-6 flex flex-wrap gap-x-6 gap-y-2 text-sm text-gray-600 dark:text-dark-300">
              <span>{{ copy.platforms }}</span>
              <span>Python {{ manifest?.python_minimum ?? '3.11' }}+</span>
              <span v-if="manifest">v{{ manifest.version }}</span>
            </div>
          </div>
        </div>
      </section>

      <section class="mx-auto max-w-6xl px-5 py-12">
        <div class="grid gap-10 lg:grid-cols-[minmax(0,1fr)_18rem]">
          <div class="min-w-0 space-y-12">
            <section aria-labelledby="download-title">
              <div class="flex items-center justify-between gap-4">
                <h2 id="download-title" class="text-xl font-semibold">{{ copy.download }}</h2>
                <button v-if="loadError" class="inline-flex h-9 w-9 items-center justify-center rounded-md border border-gray-300 text-gray-600 hover:bg-gray-50 dark:border-dark-700 dark:text-dark-300 dark:hover:bg-dark-800" :title="copy.retry" @click="loadManifest">
                  <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
                </button>
              </div>
              <div v-if="loading" class="mt-5 flex items-center gap-2 text-sm text-gray-500"><Icon name="refresh" size="sm" class="animate-spin" />{{ copy.loading }}</div>
              <div v-else-if="loadError" class="mt-5 border-l-4 border-amber-500 bg-amber-50 px-4 py-3 text-sm leading-6 text-amber-900 dark:bg-amber-950/30 dark:text-amber-200">
                {{ copy.unpublished }}
              </div>
              <div v-else class="mt-5 divide-y divide-gray-200 border-y border-gray-200 dark:divide-dark-800 dark:border-dark-800">
                <a v-for="asset in manifest?.assets" :key="asset.platform" :href="asset.url" class="grid gap-3 py-5 hover:bg-gray-50 dark:hover:bg-dark-900 sm:grid-cols-[8rem_1fr_auto] sm:items-center sm:px-3">
                  <span class="font-semibold">{{ platformLabel(asset.platform) }}</span>
                  <span class="min-w-0">
                    <span class="block truncate text-sm text-gray-600 dark:text-dark-300">{{ asset.filename }} · {{ formatAssetSize(asset.size) }}</span>
                    <code class="mt-1 block truncate text-xs text-gray-500">SHA-256 {{ asset.sha256 }}</code>
                  </span>
                  <Icon name="download" class="text-primary-600 dark:text-primary-400" />
                </a>
              </div>
            </section>

            <section aria-labelledby="workflow-title">
              <h2 id="workflow-title" class="text-xl font-semibold">{{ copy.workflow }}</h2>
              <ol class="mt-5 space-y-6 border-l border-gray-300 pl-6 dark:border-dark-700">
                <li v-for="(step, index) in copy.steps" :key="step.title" class="relative">
                  <span class="absolute -left-[2.15rem] flex h-6 w-6 items-center justify-center rounded-full bg-gray-900 text-xs font-semibold text-white dark:bg-white dark:text-gray-900">{{ index + 1 }}</span>
                  <h3 class="font-semibold">{{ step.title }}</h3>
                  <p class="mt-1 text-sm leading-6 text-gray-600 dark:text-dark-300">{{ step.text }}</p>
                </li>
              </ol>
            </section>

            <section aria-labelledby="commands-title">
              <h2 id="commands-title" class="text-xl font-semibold">{{ copy.commands }}</h2>
              <div class="mt-5 space-y-5">
                <CommandBlock title="1. Plan" :command="planCommand" />
                <CommandBlock title="2. Merge" :command="mergeCommand" />
                <CommandBlock title="3. Restore" :command="restoreCommand" />
              </div>
            </section>

            <section aria-labelledby="boundary-title" class="border-y border-gray-200 py-8 dark:border-dark-800">
              <h2 id="boundary-title" class="text-xl font-semibold">{{ copy.boundary }}</h2>
              <div class="mt-5 grid gap-6 md:grid-cols-2">
                <div>
                  <h3 class="text-sm font-semibold text-emerald-700 dark:text-emerald-400">{{ copy.included }}</h3>
                  <ul class="mt-3 space-y-2 text-sm text-gray-600 dark:text-dark-300"><li v-for="item in copy.includes" :key="item">{{ item }}</li></ul>
                </div>
                <div>
                  <h3 class="text-sm font-semibold text-red-700 dark:text-red-400">{{ copy.excluded }}</h3>
                  <ul class="mt-3 space-y-2 text-sm text-gray-600 dark:text-dark-300"><li v-for="item in copy.excludes" :key="item">{{ item }}</li></ul>
                </div>
              </div>
            </section>

            <section aria-labelledby="reference-title">
              <h2 id="reference-title" class="text-xl font-semibold">{{ copy.reference }}</h2>
              <div
                class="codex-memory-reference mt-5 text-sm leading-7 text-gray-700 dark:text-dark-200"
                v-html="renderedDocumentation"
              ></div>
            </section>
          </div>

          <aside class="lg:sticky lg:top-24 lg:self-start">
            <div class="border border-gray-200 p-5 dark:border-dark-800">
              <div class="flex items-center gap-2 text-sm font-semibold"><Icon name="shield" size="sm" class="text-emerald-600" />{{ copy.safety }}</div>
              <p class="mt-3 text-sm leading-6 text-gray-600 dark:text-dark-300">{{ copy.safetyText }}</p>
              <a v-if="manifest" :href="manifest.checksums.url" class="mt-4 inline-flex items-center gap-2 text-sm font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400"><Icon name="document" size="sm" />{{ copy.checksums }}</a>
            </div>
          </aside>
        </div>
      </section>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import DOMPurify from 'dompurify'
import { marked } from 'marked'
import Icon from '@/components/icons/Icon.vue'
import CommandBlock from '@/components/public/CommandBlock.vue'
import codexMemoryDocumentation from '@/content/codex-memory.md?raw'
import { getLocale } from '@/i18n'
import { fetchCodexMemoryReleaseManifest, formatAssetSize, type CodexMemoryReleaseManifest } from '@/utils/codexMemoryRelease'

const messages = {
  zh: {
    docs: '文档中心', login: '登录', title: '统一本机 Codex 记忆，不绑定模型提供方', intro: '将同一台电脑上由不同登录方式或 provider 使用的记忆、活动任务与归档任务合并到统一 CODEX_HOME。来源数据默认保留，凭据永不参与合并。', platforms: 'macOS · Windows · Linux', download: '下载', retry: '重新加载', loading: '正在读取发布清单', unpublished: '当前部署尚未提供经过校验的 GitHub Release 清单。请等待维护者发布，不要从非官方镜像下载。', workflow: '合并流程', commands: '命令', boundary: '数据边界', reference: '完整边界说明', included: '参与合并', excluded: '永不合并', safety: '恢复保证', safetyText: '写入前自动创建本地备份。默认不覆盖来源目录；恢复前还会再创建一次安全备份。', checksums: '下载完整校验文件',
    steps: [
      { title: '生成预览', text: '扫描来源目录，校验 JSONL、普通文件、路径和 config.toml，并输出计划。' },
      { title: '核对冲突', text: '身份与摘要完全相同才去重；差异内容双份保留并标记来源。' },
      { title: '备份并合并', text: '结束活动请求后明确确认，工具先备份再写入独立合并结果。' },
      { title: '激活并验证', text: '设置用户级 CODEX_HOME，重启 Codex 后检查记忆和任务记录。' },
    ],
    includes: ['memories/', 'sessions/', 'archived_sessions/', '来源与 SHA-256 元数据'],
    excludes: ['auth.json、OAuth 令牌和 API 密钥', '系统钥匙串', 'Sub2API Redis/PostgreSQL', '正在进行的流式响应、工具调用和 WebSocket'],
  },
  en: {
    docs: 'Docs', login: 'Sign in', title: 'Unify local Codex memory without binding it to a provider', intro: 'Merge memories, active tasks, and archived tasks used by different sign-in methods or providers on one computer into one CODEX_HOME. Source data remains intact and credentials are never merged.', platforms: 'macOS · Windows · Linux', download: 'Download', retry: 'Reload', loading: 'Loading release manifest', unpublished: 'This deployment does not have a verified GitHub Release manifest yet. Wait for a maintainer release and do not use unofficial mirrors.', workflow: 'Merge workflow', commands: 'Commands', boundary: 'Data boundary', reference: 'Complete boundary reference', included: 'Merged', excluded: 'Never merged', safety: 'Recovery guarantee', safetyText: 'A local backup is created before writing. Source homes are not overwritten, and restore creates another safety backup.', checksums: 'Download checksum file',
    steps: [
      { title: 'Build a preview', text: 'Audit source paths, JSONL, regular files, and config.toml before writing a plan.' },
      { title: 'Review conflicts', text: 'Deduplicate only exact identity and digest matches; preserve different content with source labels.' },
      { title: 'Back up and merge', text: 'End active requests, confirm explicitly, then back up before creating the merged result.' },
      { title: 'Activate and verify', text: 'Set the user CODEX_HOME, restart Codex, and verify memories and task records.' },
    ],
    includes: ['memories/', 'sessions/', 'archived_sessions/', 'Source and SHA-256 metadata'],
    excludes: ['auth.json, OAuth tokens, and API keys', 'System keychains', 'Sub2API Redis/PostgreSQL', 'In-flight streams, tool calls, and WebSockets'],
  },
}
const copy = computed(() => messages[getLocale()])
const renderedDocumentation = computed(() => {
  const withoutTitle = codexMemoryDocumentation.replace(/^# .+\n+/, '')
  return DOMPurify.sanitize(marked.parse(withoutTitle, { async: false }) as string)
})
const manifest = ref<CodexMemoryReleaseManifest | null>(null)
const loading = ref(true)
const loadError = ref(false)
const planCommand = './codex-memory plan --source "$HOME/.codex-a" --source "$HOME/.codex-b" --target "$HOME/.codex" --output "$HOME/codex-memory-plan.json"'
const mergeCommand = './codex-memory merge --plan "$HOME/codex-memory-plan.json" --confirm --confirm-no-active-requests --activate'
const restoreCommand = './codex-memory restore --backup "/path/from/merge" --target "$HOME/.codex" --confirm --confirm-no-active-requests'

function platformLabel(value: string) { return value === 'macos' ? 'macOS' : value === 'windows' ? 'Windows' : 'Linux' }
async function loadManifest() {
  loading.value = true
  loadError.value = false
  try { manifest.value = await fetchCodexMemoryReleaseManifest() }
  catch { manifest.value = null; loadError.value = true }
  finally { loading.value = false }
}
onMounted(loadManifest)
</script>

<style scoped>
.codex-memory-reference :deep(h2) {
  margin-top: 2rem;
  font-size: 1rem;
  font-weight: 600;
}

.codex-memory-reference :deep(p),
.codex-memory-reference :deep(ul),
.codex-memory-reference :deep(ol) {
  margin-top: 0.75rem;
}

.codex-memory-reference :deep(ul),
.codex-memory-reference :deep(ol) {
  padding-left: 1.25rem;
}

.codex-memory-reference :deep(ul) {
  list-style: disc;
}

.codex-memory-reference :deep(ol) {
  list-style: decimal;
}

.codex-memory-reference :deep(code) {
  border-radius: 0.25rem;
  background: rgb(243 244 246);
  padding: 0.125rem 0.3rem;
  color: rgb(17 24 39);
}

:global(.dark) .codex-memory-reference :deep(code) {
  background: rgb(31 41 55);
  color: rgb(229 231 235);
}
</style>
