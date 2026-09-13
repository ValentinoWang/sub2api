<template>
  <PublicPageLayout>
    <article class="claude-experience" :data-experience-id="experience.id" aria-labelledby="page-title">
      <nav class="breadcrumb" aria-label="经验分享路径">
        <RouterLink to="/home">首页</RouterLink><span aria-hidden="true">/</span>
        <RouterLink to="/experiences">经验分享</RouterLink><span aria-hidden="true">/</span>
        <span aria-current="page">{{ caseNumber }}</span>
      </nav>

      <header class="intro">
        <p class="kicker">Claude Code 使用经验 / {{ caseNumber }}</p>
        <h1 id="page-title">{{ experience.title }}</h1>
        <p class="subtitle">{{ experience.subtitle }}</p>
        <p class="meta">适用：{{ experience.applicableTo }} · 更新：{{ experience.updatedAt }}</p>
      </header>

      <section class="content-section problem" aria-labelledby="problem-title">
        <p class="step">01</p>
        <h2 id="problem-title">问题说明</h2>
        <template v-if="isPermissionsCase">
          <p>Claude Code 在读取文件、修改代码或运行命令前反复要求确认。即使每次都选择允许，下一次操作仍可能再次弹窗，打断连续开发。</p>
          <p>你的目标是让自己信任的本机开发环境默认跳过逐项确认，而不是修改服务器权限或把所有安全限制永久关闭。</p>
          <aside class="warning"><strong>风险提示：</strong>跳过确认后，Claude Code 可以直接执行文件修改和命令。请只在可信仓库和清楚的任务范围内使用，不要把未知项目或来历不明的提示词放进该模式。</aside>
        </template>
        <template v-else>
          <p>中转站已经提供最新模型，但 Claude Code 的 <code>/model</code> 仍只显示内置模型。下面以 Fable 5.1（模型 ID：<code>claude-fable-5-1</code>）为例说明。</p>
          <p>这通常不是模型不存在，而是客户端没有主动读取自定义网关的模型目录。模型 ID 被客户端接受、网关目录列出模型、列表可见和实际请求成功，是四个不同结果。</p>
        </template>
      </section>

      <section class="content-section solution" aria-labelledby="solution-title">
        <div class="heading-row">
          <div><p class="step">02</p><h2 id="solution-title">解决方案</h2></div>
        </div>
        <template v-if="isPermissionsCase">
          <ol class="solution-list">
            <li>保留现有用户配置，只把 <code>permissions.defaultMode</code> 设置为 <code>bypassPermissions</code>。</li>
            <li>把 <code>skipDangerousModePermissionPrompt</code> 设置为 <code>true</code>，避免每次启动再次确认危险模式。</li>
            <li>关闭旧会话并启动新会话验证。需要临时恢复逐项确认时，使用 <code>claude --permission-mode manual</code>。</li>
          </ol>
        </template>
        <template v-else>
          <ol class="solution-list">
            <li>先确认实际网关的 <code>/v1/models</code> 返回 <code>claude-fable-5-1</code>。</li>
            <li>在用户级 <code>settings.json</code> 的 <code>env</code> 中加入 <code>CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY=1</code>，其他地址、认证和设置保持不变。</li>
            <li>关闭旧会话，重新启动 Claude Code，再打开 <code>/model</code> 确认 Fable 5.1 出现并可选。</li>
          </ol>
        </template>

        <div class="prompt-heading">
          <h3>让 Codex 帮你处理</h3>
          <button class="copy-button" type="button" @click="copyPrompt">
            <Icon :name="copyStatus === '提示词已复制' ? 'check' : 'copy'" size="sm" />
            <span>{{ copyStatus === '提示词已复制' ? '已复制' : '复制提示词' }}</span>
          </button>
        </div>
        <textarea ref="promptField" class="prompt-field" readonly :aria-label="`${caseNumber} 可直接发给 Codex 的完整提示词`" :value="experience.prompt"></textarea>
        <p class="copy-status" role="status" aria-live="polite">{{ copyStatus }}</p>
      </section>

      <section class="content-section explanation" aria-labelledby="explanation-title">
        <p class="step">03</p>
        <h2 id="explanation-title">原因、验证与注意事项</h2>
        <template v-if="isPermissionsCase">
          <h3>为什么两个设置都需要？</h3>
          <p><code>defaultMode</code> 决定新会话采用哪种权限模式；启动确认开关决定是否还要为高权限模式再弹一次提醒。只改其中一个，体验可能仍然不是“启动后直接工作”。</p>
          <h3>怎样确认已经生效？</h3>
          <p>先验证配置语法，再启动一个全新的 Claude Code 会话。用只读检查和普通项目命令确认不再逐项询问，不要用删除文件或改系统设置来测试。</p>
          <h3>什么时候不要使用？</h3>
          <p>陌生仓库、未经审查的脚本、包含生产凭证的目录或范围不明确的任务，应使用普通确认模式。跳过确认只减少交互，不扩大 Claude Code 应当执行的任务范围。</p>
        </template>
        <template v-else>
          <h3>为什么网关有模型，列表却没有？</h3>
          <p>Claude Code 默认可以只显示自己的内置模型目录。启用网关模型发现后，客户端才会向自定义接入地址读取 <code>/v1/models</code>，把网关提供的模型加入选择列表。</p>
          <h3>怎样确认已经生效？</h3>
          <ul>
            <li>目录检查：网关向你的凭证返回 <code>claude-fable-5-1</code>。</li>
            <li>客户端检查：新会话的 <code>/model</code> 中显示 Fable 5.1。</li>
            <li>选择检查：当前会话明确选中目标模型。</li>
            <li>请求检查：只有实际发送并完成一次请求，才能证明生成链路可用。</li>
          </ul>
          <p class="note">为了避免不必要的费用，查看目录、刷新列表和选择模型都不需要发送生成请求。模型可见不等于上游请求已经成功。</p>
        </template>
      </section>
    </article>

    <template #footer><Rest2BuildBrandFooter /></template>
  </PublicPageLayout>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import PublicPageLayout from '@/components/layout/PublicPageLayout.vue'
import Rest2BuildBrandFooter from '@/components/common/Rest2BuildBrandFooter.vue'
import { getExperienceById } from '@/content/experiences'

const props = defineProps<{
  experienceId: 'claude-code-bypass-permissions' | 'claude-code-fable-5-1-not-visible'
  caseNumber: 'ERR-004' | 'ERR-005'
}>()

const experience = getExperienceById(props.experienceId)!
const isPermissionsCase = computed(() => props.experienceId === 'claude-code-bypass-permissions')
const promptField = ref<HTMLTextAreaElement | null>(null)
const copyStatus = ref('')

async function copyPrompt() {
  try {
    await navigator.clipboard.writeText(experience.prompt)
    copyStatus.value = '提示词已复制'
  } catch {
    promptField.value?.focus()
    promptField.value?.select()
    copyStatus.value = '自动复制未成功，提示词已选中，可手动复制'
  }
}
</script>

<style scoped>
.claude-experience{overflow:hidden;border:1px solid #d6e0dc;border-radius:8px;background:#fff;color:#22302b;box-shadow:0 20px 42px -38px rgba(15,23,42,.45)}
.dark .claude-experience{border-color:#31463f;background:#0d1916;color:#eaf4ef}
.breadcrumb{display:flex;flex-wrap:wrap;gap:7px;padding:16px 36px 0;color:#64748b;font-size:12px}.breadcrumb a{color:#0f766e;text-decoration:none}.breadcrumb a:hover{text-decoration:underline;text-underline-offset:3px}
.intro,.content-section{padding:32px 36px}.intro{border-bottom:1px solid #dce5e1;background:#f8fbfa}.dark .intro{border-color:#31463f;background:#12231e}
.kicker,.step{margin:0 0 10px;color:#0f766e;font:700 12px ui-monospace,SFMono-Regular,Menlo,monospace}.intro h1{max-width:820px;margin:0;color:#18332b;font-size:32px;line-height:1.35;letter-spacing:0}.dark .intro h1{color:#f0fdfa}.subtitle{margin:12px 0 0;color:#52615c;font-size:17px}.meta{margin:14px 0 0;color:#6b7b74;font-size:12px}
.content-section{border-bottom:1px solid #dce5e1}.content-section:last-child{border-bottom:0}.content-section h2{margin:0 0 16px;font-size:22px;line-height:1.45}.content-section h3{margin:26px 0 12px;font-size:17px}.content-section p,.content-section li{color:#45534e;line-height:1.75}.dark .content-section p,.dark .content-section li{color:#d1d5db}.content-section code{overflow-wrap:anywhere;border-radius:4px;background:#edf3f0;padding:2px 5px;color:#164e3e}.dark .content-section code{background:#17352c;color:#ccfbf1}
.warning,.note{margin-top:20px;border-left:4px solid #b45309;background:#fff7ed;padding:14px 16px;color:#7c2d12;line-height:1.7}.dark .warning,.dark .note{background:#3b2514;color:#fed7aa}.solution{background:#f3f8f6}.dark .solution{background:#10241e}.solution-list{margin:0;padding-left:24px}.solution-list li{margin:10px 0}.prompt-heading{display:flex;align-items:center;justify-content:space-between;gap:16px;margin-top:30px}.prompt-heading h3{margin:0}.copy-button{display:inline-flex;min-height:38px;flex:none;align-items:center;gap:7px;border:1px solid #0f766e;border-radius:5px;background:#0f766e;padding:8px 12px;color:#fff;font-size:13px;font-weight:700}.copy-button:hover{background:#0b5d57}.copy-button:focus-visible{outline:3px solid #38bdf8;outline-offset:3px}.prompt-field{display:block;width:100%;min-height:320px;margin-top:16px;resize:vertical;border:1px solid #b8cac1;border-radius:5px;background:#fff;padding:16px;color:#1f2937;font:13px/1.85 ui-monospace,SFMono-Regular,Menlo,monospace}.dark .prompt-field{border-color:#466159;background:#0f1f1b;color:#e5e7eb}.copy-status{min-height:22px;margin:8px 0 0!important;color:#0f766e!important;font-size:12px}.explanation ul{padding-left:22px}.explanation li{margin:8px 0}
@media (max-width:640px){.breadcrumb{padding:14px 20px 0}.intro,.content-section{padding:24px 20px}.intro h1{font-size:27px}.subtitle{font-size:15px}.prompt-heading{align-items:flex-start}.prompt-field{min-height:420px;padding:12px;font-size:12px}}
</style>
