<template>
  <PublicPageLayout>
    <article class="catalog-experience" data-experience-id="codex-model-catalog-context-window" aria-labelledby="page-title">
      <nav class="breadcrumb" aria-label="经验分享路径">
        <RouterLink to="/home">首页</RouterLink><span aria-hidden="true">/</span>
        <RouterLink to="/experiences">经验分享</RouterLink><span aria-hidden="true">/</span>
        <span aria-current="page">ERR-006</span>
      </nav>
      <header class="intro">
        <p class="kicker">Codex 使用错误说明 / ERR-006</p>
        <h1 id="page-title">{{ guide.title }}</h1>
        <p class="subtitle">模型选对以后，还要让客户端加载正确的能力目录</p>
        <p class="meta">适用：{{ guide.applicableTo }} · 更新：{{ guide.updatedAt }}</p>
      </header>
      <section class="content-section" aria-labelledby="problem-title">
        <p class="step">01</p><h2 id="problem-title">问题说明</h2>
        <div class="markdown-body" v-html="problemHtml"></div>
      </section>
      <section class="content-section solution" aria-labelledby="solution-title">
        <p class="step">02</p><h2 id="solution-title">解决方案</h2>
        <div class="prompt-heading">
          <h3>让 Codex 帮你处理</h3>
          <button type="button" class="copy-button" @click="copyPrompt">{{ copyStatus === '提示词已复制' ? '已复制' : '复制提示词' }}</button>
        </div>
        <p>复制完整提示词，让 Codex 根据你自己的入口和客户端生成目录，保留配置与回滚方式。</p>
        <p class="prompt-download"><a href="/downloads/sync-codex-model-catalog.py" download>下载目录同步脚本（Python 3.11+）</a> · 提示词已包含当前站点的下载地址，可直接复制使用。</p>
        <textarea ref="promptField" class="prompt-field" readonly aria-label="ERR-006 完整排障与配置提示词" :value="prompt"></textarea>
        <p class="copy-status" role="status" aria-live="polite">{{ copyStatus }}</p>
        <div class="markdown-body" v-html="solutionHtml"></div>
      </section>
      <section class="content-section" aria-labelledby="notes-title">
        <p class="step">03</p><h2 id="notes-title">原因、验证与注意事项</h2>
        <div class="markdown-body" v-html="notesHtml"></div>
      </section>
    </article>
    <template #footer><Rest2BuildBrandFooter /></template>
  </PublicPageLayout>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import PublicPageLayout from '@/components/layout/PublicPageLayout.vue'
import Rest2BuildBrandFooter from '@/components/common/Rest2BuildBrandFooter.vue'
import guide from '@/content/codexModelCatalogGuide.json'
import { formatTutorialPrompt } from '@/utils/tutorialPrompt.mjs'
import { PUBLIC_PAGES } from '@/constants/brand'

const render = (markdown: string) => DOMPurify.sanitize(marked.parse(markdown, { async: false }))
const problemHtml = render(guide.problem)
const solutionHtml = render(guide.solution)
const notesHtml = render(guide.notes)
const prompt = formatTutorialPrompt(guide.prompt, new URL(PUBLIC_PAGES.codexModelCatalog, window.location.origin).href)
const promptField = ref<HTMLTextAreaElement | null>(null)
const copyStatus = ref('')
async function copyPrompt() {
  try {
    await navigator.clipboard.writeText(prompt)
    copyStatus.value = '提示词已复制'
  } catch {
    promptField.value?.focus()
    promptField.value?.select()
    copyStatus.value = '自动复制未成功，提示词已选中，可手动复制'
  }
}
</script>

<style scoped>
.catalog-experience{min-width:0;overflow:hidden;border:1px solid #d6e0dc;border-radius:8px;background:#fff;color:#22302b;box-shadow:0 20px 42px -38px rgba(15,23,42,.45)}
.dark .catalog-experience{border-color:#31463f;background:#0d1916;color:#eaf4ef}
.breadcrumb{display:flex;flex-wrap:wrap;gap:7px;padding:16px 36px 0;color:#64748b;font-size:12px}.breadcrumb a{color:#0f766e}
.intro,.content-section{padding:32px 36px}.intro{border-bottom:1px solid #dce5e1;background:#f8fbfa}.dark .intro{border-color:#31463f;background:#12231e}
.kicker,.step{margin:0 0 10px;color:#0f766e;font:700 12px ui-monospace,monospace}.intro h1{max-width:820px;margin:0;color:#18332b;font-size:32px;line-height:1.35}.dark .intro h1{color:#f0fdfa}.subtitle{margin:12px 0 0;color:#52615c;font-size:17px}.meta{margin:14px 0 0;color:#6b7b74;font-size:12px}
.content-section{border-bottom:1px solid #dce5e1}.content-section:last-child{border-bottom:0}.content-section h2{margin:0 0 16px;font-size:22px;line-height:1.45}.content-section :deep(h3){margin:26px 0 12px;font-size:17px}.content-section :deep(p),.content-section :deep(li){margin-bottom:12px;color:#45534e;line-height:1.8;overflow-wrap:anywhere}.dark .content-section :deep(p),.dark .content-section :deep(li){color:#d1d5db}
.markdown-body :deep(a){color:#0f766e;text-decoration:underline;text-underline-offset:3px}.dark .markdown-body :deep(a){color:#5eead4}.markdown-body :deep(code){border-radius:4px;background:#edf3f0;padding:2px 5px;color:#164e3e;overflow-wrap:anywhere}.dark .markdown-body :deep(code){background:#17352c;color:#ccfbf1}.markdown-body :deep(pre){max-width:100%;overflow-x:auto;padding:16px;background:#edf3f0;line-height:1.7;font-size:13px}.markdown-body :deep(pre code){padding:0}.dark .markdown-body :deep(pre){background:#17352c}.markdown-body :deep(ul),.markdown-body :deep(ol){padding-left:24px;list-style:revert}.markdown-body :deep(table){display:block;width:100%;overflow-x:auto;border-collapse:collapse;font-size:13px;margin:18px 0}.markdown-body :deep(th),.markdown-body :deep(td){min-width:140px;border:1px solid #d6e0dc;padding:10px 12px;text-align:left}
.solution{background:#f3f8f6}.dark .solution{background:#10241e}.prompt-heading{display:flex;align-items:center;justify-content:space-between;gap:16px;margin:24px 0 12px}.prompt-heading h3{margin:0}.copy-button{min-height:38px;flex:none;border:1px solid #0f766e;border-radius:5px;background:#0f766e;padding:8px 12px;color:#fff;font-size:13px;font-weight:700}.copy-button:hover{background:#0b5d57}.copy-button:focus-visible{outline:3px solid #38bdf8;outline-offset:3px}.prompt-field{display:block;width:100%;min-height:320px;margin-top:16px;resize:vertical;border:1px solid #b8cac1;border-radius:5px;background:#fff;padding:16px;color:#1f2937;font:13px/1.85 ui-monospace,monospace}.dark .prompt-field{border-color:#466159;background:#0f1f1b;color:#e5e7eb}.copy-status{min-height:22px;margin:8px 0 0!important;color:#0f766e!important;font-size:12px}
.prompt-download a{color:#0f766e;text-decoration:underline;text-underline-offset:3px}.dark .prompt-download a{color:#5eead4}
@media(max-width:640px){.breadcrumb{padding:14px 20px 0}.intro,.content-section{padding:24px 20px}.intro h1{font-size:27px}.subtitle{font-size:15px}.prompt-heading{align-items:flex-start}.prompt-field{min-height:420px;padding:12px;font-size:12px}}
</style>
