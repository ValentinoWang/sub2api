<template>
  <PublicPageLayout>
    <article class="migration-page" aria-labelledby="page-title">
      <nav class="breadcrumb" aria-label="经验分享路径">
        <RouterLink to="/home">首页</RouterLink><span>/</span>
        <RouterLink to="/experiences">经验分享</RouterLink><span>/</span>
        <span aria-current="page">{{ migration.title }}</span>
      </nav>
      <header class="intro">
        <p class="kicker">接入与排障 / ERR-002</p>
        <h1 id="page-title">{{ migration.title }}</h1>
        <p>离线诊断和迁移本机 Codex 历史中的失效 provider 关联。先审阅计划，再备份、应用、验证或回滚。</p>
      </header>

      <section aria-labelledby="situation-title">
        <p class="step">01</p>
        <h2 id="situation-title">情况说明</h2>
        <p>切换官方登录、其他中转站或本地转发后，旧任务可能仍指向已删除的 provider 名称。此时配置看上去正确，打开旧任务却提示 provider 不存在。</p>
        <div class="notice"><strong>这个工具只处理本机历史关联。</strong>它不迁移 ChatGPT 网页历史、云端记忆、正在进行的请求或服务端 Redis 状态。</div>
        <ul>
          <li>从 P1 接入说明配置新 provider 后，旧任务报 provider 未找到。</li>
          <li>配置、profile 或启动覆盖太多，无法判断实际生效的目标。</li>
          <li>希望先备份、查看准确影响范围，再决定是否改动本地任务。</li>
        </ul>
      </section>

      <section class="tool-panel" aria-labelledby="tool-title">
        <div class="heading-row"><div><p class="step">02</p><h2 id="tool-title">Codex 帮你处理</h2></div><button type="button" class="icon-button" :aria-label="copyLabel" @click="copyPrompt">{{ copyLabel }}</button></div>
        <p>先把下面的指令交给 Codex 做只读诊断；需要写入时，下载离线工具并在关闭 Codex 后从外部终端执行。</p>
        <textarea ref="promptField" readonly aria-label="可直接发给 Codex 的迁移提示词" :value="migration.prompt"></textarea>
        <p class="status" role="status" aria-live="polite">{{ copyStatus }}</p>
        <div class="downloads" aria-label="迁移工具下载">
          <a class="download" :href="migration.packageDownload" download>下载离线工具包 <span>1.0.0</span></a>
          <a class="secondary-download" :href="migration.promptDownload" download>下载提示词</a>
          <a class="secondary-download" :href="migration.manifestDownload" download>查看校验清单</a>
        </div>
      </section>

      <section aria-labelledby="human-title">
        <p class="step">03</p>
        <h2 id="human-title">给人看的：原因、证据与经验</h2>
        <div class="explanation-grid">
          <section><h3>不要写死 provider 名称</h3><p>本站不同接入场景可能使用不同 provider 标识。工具从用户实际生效的配置解析目标，不会把所有历史强制改成 <code>sub2api</code> 或 <code>OpenAI</code>。</p></section>
          <section><h3>备份失败时必须零写入</h3><p>迁移前要为全部选中 JSONL 和 SQLite 对象建立并读回验证备份。SQLite 使用一致性备份，不能只复制主数据库后忽略已提交日志。</p></section>
          <section><h3>一次中断不等于全部失败</h3><p>JSONL 与索引数据库不能组成一个整体事务。工具保存阶段日志，发生中断后提示恢复或回滚；遇到新消息和输入漂移则停止覆盖。</p></section>
          <section><h3>结构验证不等于对话接续</h3><p>文件和索引通过校验后，仍要重开同一个旧任务，引用迁移前的测试事实并追加一轮交流。这是与新建任务成功不同的证明。</p></section>
        </div>
        <h3>建议的安全顺序</h3>
        <ol><li>运行 <code>inspect</code>，确认实际数据根和故障类别。</li><li>运行 <code>plan</code>，检查目标、任务范围和前态摘要。</li><li>关闭使用同一数据根的 Codex，确认备份与计划摘要后运行 <code>apply</code>。</li><li>运行 <code>verify</code>，重新打开原任务完成一次真实接续检查。</li></ol>
        <p class="scope">当前工具要求 Python 3.11 或更高版本，支持已登记的 JSONL 元数据和 SQLite 索引结构。未知格式、损坏历史、多来源冲突或未声明目标都会停在只读诊断阶段。</p>
      </section>
    </article>
    <template #footer><Rest2BuildBrandFooter /></template>
  </PublicPageLayout>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import Rest2BuildBrandFooter from '@/components/common/Rest2BuildBrandFooter.vue'
import PublicPageLayout from '@/components/layout/PublicPageLayout.vue'
import { CODEX_SESSION_MIGRATION as migration } from '@/constants/codexMigration'

const promptField = ref<HTMLTextAreaElement | null>(null)
const copyStatus = ref('')
const copyLabel = computed(() => copyStatus.value === '提示词已复制' ? '已复制' : '复制提示词')

async function copyPrompt() {
  try {
    await navigator.clipboard.writeText(migration.prompt)
    copyStatus.value = '提示词已复制'
  } catch {
    promptField.value?.focus()
    promptField.value?.select()
    copyStatus.value = '自动复制未成功，提示词已选中，可手动复制'
  }
}
</script>

<style scoped>
.migration-page{overflow:hidden;border:1px solid rgba(15,23,42,.1);border-radius:8px;background:#fff;color:#26332f}.dark .migration-page{border-color:rgba(255,255,255,.12);background:#101b18;color:#e8f2ed}.breadcrumb{display:flex;flex-wrap:wrap;gap:8px;padding:16px 32px 0;font-size:12px;color:#687772}.breadcrumb a{color:#0f766e;text-decoration:none}.intro,section{padding:30px 32px;border-bottom:1px solid rgba(15,23,42,.1)}.dark .intro,.dark section{border-color:rgba(255,255,255,.12)}.intro{background:#f0f8f5}.dark .intro{background:#122722}.kicker,.step{margin:0 0 8px;color:#0f766e;font:700 12px ui-monospace,SFMono-Regular,monospace}.migration-page h1{margin:0;font-size:32px;line-height:1.35}.migration-page h2{margin:0 0 15px;font-size:21px}.migration-page h3{margin:0 0 8px;font-size:16px}.migration-page p,.migration-page li{line-height:1.7}.notice{border-left:3px solid #0f766e;background:#edf8f3;padding:12px 14px;line-height:1.6}.dark .notice{background:#15352d}.tool-panel{background:#f6faf8}.dark .tool-panel{background:#10241e}.heading-row{display:flex;justify-content:space-between;gap:16px;align-items:flex-start}.icon-button{border:1px solid #0f766e;border-radius:5px;background:#0f766e;padding:8px 11px;color:#fff;font-weight:700}.icon-button:focus-visible,.download:focus-visible,.secondary-download:focus-visible{outline:3px solid #38bdf8;outline-offset:3px}textarea{display:block;box-sizing:border-box;width:100%;min-height:330px;margin-top:16px;resize:vertical;border:1px solid #b6c9c0;border-radius:5px;background:#fff;padding:14px;color:#1f2937;font:13px/1.8 ui-monospace,SFMono-Regular,monospace}.dark textarea{background:#0e1b17;color:#e8f2ed}.status{min-height:24px;margin:8px 0;color:#0f766e}.downloads{display:flex;flex-wrap:wrap;gap:10px;align-items:center}.download,.secondary-download{display:inline-flex;align-items:center;min-height:38px;border-radius:5px;padding:0 12px;font-size:14px;font-weight:700;text-decoration:none}.download{background:#0f766e;color:#fff}.download span{margin-left:8px;font:12px ui-monospace,SFMono-Regular,monospace}.secondary-download{border:1px solid #8ba79a;color:#0f766e}.explanation-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:14px}.explanation-grid section{padding:16px;border:1px solid #d5e3dc;border-radius:6px;background:#fafdfb}.dark .explanation-grid section{border-color:#365449;background:#11231d}.explanation-grid p{margin:0}.scope{border-left:3px solid #0f766e;padding-left:14px;color:#52645c}@media(max-width:640px){.breadcrumb{padding:14px 20px 0}.intro,section{padding:24px 20px}.migration-page h1{font-size:27px}.explanation-grid{grid-template-columns:1fr}textarea{min-height:430px;font-size:12px}}
</style>
