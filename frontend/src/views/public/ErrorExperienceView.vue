<template>
  <PublicPageLayout>
    <article class="experience" aria-labelledby="page-title">
      <nav class="experience-breadcrumb" :aria-label="t('experiences.breadcrumbList')">
        <router-link to="/home">{{ t('experiences.breadcrumbHome') }}</router-link>
        <span aria-hidden="true">/</span>
        <router-link to="/experiences">{{ t('experiences.breadcrumbList') }}</router-link>
        <span aria-hidden="true">/</span>
        <router-link :to="{ path: '/experiences', query: { category: experience.category } }">{{ t(`experiences.categories.${experience.category}`) }}</router-link>
        <span aria-hidden="true">/</span>
        <span>{{ experience.series }}</span>
        <span aria-hidden="true">/</span>
        <span aria-current="page">{{ experience.title }}</span>
      </nav>
      <header class="experience-intro">
        <p class="experience-kicker">{{ t(`experiences.categories.${experience.category}`) }} / ERR-001</p>
        <h1 id="page-title">{{ experience.title }}</h1>
        <p class="experience-subtitle">{{ experience.subtitle }}</p>
        <p class="experience-meta">{{ t('experiences.appliesTo') }}：{{ experience.applicableTo }} · {{ t('experiences.updated', { date: experience.updatedAt }) }}</p>
      </header>

      <section class="experience-section" aria-labelledby="situation-title">
        <p class="experience-step">01</p>
        <h2 id="situation-title">情况说明</h2>
        <p>你已经按照本站接入说明配置了 Codex，站点也已提供 GPT-6-Astra，但桌面模型选择器里仍然只有旧模型。也可能已经出现 GPT-6，重新打开原来的任务时却还是 5.6；或者终端里的 Codex 已更新，桌面应用的模型列表却没有变化。</p>
        <p>这些表现很像“中转站还没同步新模型”，但不一定是站点故障。中转站能否调用模型、桌面应用是否展示模型、当前任务实际选择哪个模型，是不同环节；修复其中一个，不代表另外两个会自动更新。</p>
        <h3>本文适合这些情况</h3>
        <ul>
          <li>接入本站后，Codex 桌面模型列表没有 GPT-6-Astra。</li>
          <li>终端显示已经升级，桌面仍显示旧列表。</li>
          <li>模型已出现在列表中，但当前任务仍使用之前选择的模型。</li>
          <li>不确定 Codex 连接的是本站、旧地址，还是本机代理。</li>
        </ul>
        <p class="experience-note">如果你遇到登录失败、余额不足、明确的 401/403/429、持续超时或 5xx，请同时检查认证、权限、额度和服务状态，不要只按“升级客户端”处理。Claude Code 的安装、配置与模型选择方式不同，不能直接照搬本文版本号和路径。</p>
      </section>

      <section class="experience-section experience-prompt" aria-labelledby="codex-help-title">
        <div class="experience-heading-row">
          <div>
            <p class="experience-step">02</p>
            <h2 id="codex-help-title">Codex 帮你处理</h2>
          </div>
          <button class="copy-button" type="button" @click="copyPrompt">{{ copyLabel }}</button>
        </div>
        <textarea ref="promptField" class="prompt-field" readonly aria-label="可直接发给 Codex 的完整排障提示词" :value="prompt"></textarea>
        <p class="copy-status" role="status" aria-live="polite">{{ copyStatus }}</p>
      </section>

      <section class="experience-section" aria-labelledby="human-title">
        <p class="experience-step">03</p>
        <h2 id="human-title">给人看的：原因、证据与经验</h2>

        <section class="explanation" aria-labelledby="three-layers-title">
          <h3 id="three-layers-title">为什么站点支持了，桌面却不显示？</h3>
          <p>一次正常使用至少经过三个判断。<strong>“支持”“可见”“选中”不是同一个开关。</strong></p>
          <div class="table-scroll"><table><thead><tr><th>检查层</th><th>它决定什么</th><th>你可能看到的现象</th></tr></thead><tbody>
            <tr><td>中转站与个人访问权限</td><td>你的接入入口和 API Key 能否请求目标模型</td><td>站点介绍有模型，但自己的目录或请求仍可能受权限、路由或服务状态影响</td></tr>
            <tr><td>客户端模型目录</td><td>当前桌面版本把哪些模型放进选择器</td><td>API 请求成功，桌面仍不显示新模型</td></tr>
            <tr><td>默认配置与任务选择</td><td>本次任务实际请求哪个模型</td><td>新模型已经可选，原任务仍显示旧模型</td></tr>
          </tbody></table></div>
          <p>只在站点看到模型名称，不能证明你的 Key 已调用成功；只调用成功一次，也不能证明桌面选择器已更新。反过来，桌面没有新模型，也不能直接推出中转站不支持它。</p>
        </section>

        <section class="explanation" aria-labelledby="desktop-title">
          <h3 id="desktop-title">为什么更新终端 Codex 没有效果？</h3>
          <p>桌面应用和终端命令可以各自携带一份 Codex。<code>codex --version</code> 说明的是终端命令路径找到的程序，不自动代表桌面正在使用的引擎版本。检查应从正在运行的应用出发，而不是只看安装命令是否成功。</p>
          <div class="table-scroll"><table><thead><tr><th>历史案例中的检查对象</th><th>观察结果</th><th>排障意义</th></tr></thead><tbody>
            <tr><td>终端 Codex</td><td>已更新到 0.153.4</td><td>不能据此认定桌面完成升级</td></tr>
            <tr><td>桌面内置 Codex 0.153.3</td><td>Astra 的目录标记为 <code>hide</code></td><td>目标模型未在桌面列表正常展示</td></tr>
            <tr><td>桌面内置 Codex 0.153.4</td><td>目录为 <code>list</code>，运行时返回 <code>hidden=false</code></td><td>新引擎恢复模型可见性</td></tr>
          </tbody></table></div>
          <p>这些是带日期的历史观测，不是让你永久安装某个版本。通过官方更新入口或完整安装包升级；“安装包已经下载”与“重启后正在运行新版”仍是两件事。</p>
        </section>

        <section class="explanation" aria-labelledby="route-title">
          <h3 id="route-title">为什么实际连接可能走了别的地址？</h3>
          <p>模型名称与连接地址是两个配置项。模型填对了，但连接到旧中转站或错误的本机端口，仍可能得到旧模型目录、认证错误或连接失败。核对的是<strong>实际生效的 Base URL</strong>，而不只是某一段看起来正确的配置。</p>
          <ul>
            <li><strong>Base URL：</strong>以本站接入说明为准；不要凭印象重复添加 <code>/v1</code>。</li>
            <li><strong>provider 名称：</strong>通常是本地配置标识，不是站点域名；不要把别人的名称当成本站标准。</li>
            <li><strong>localhost / 127.0.0.1：</strong>表示自己的电脑。没有本地转发服务时，它不能代替远端站点；有意使用本机代理时，它也可能是正确配置。</li>
            <li><strong>旧配置引用：</strong>重命名或移除 provider 后，命名配置和启动入口仍可能引用旧名字。让 Codex 检查引用关系，不要删除全部 provider。</li>
          </ul>
        </section>

        <section class="explanation" aria-labelledby="fallback-title">
          <h3 id="fallback-title">仍显示 5.6，不一定是服务器回退</h3>
          <p>如果默认配置从一开始就选择了 5.6，或者打开的是之前使用 5.6 的任务，界面继续显示旧模型，首先说明的是客户端选择状态，而不是服务器偷偷替换了 GPT-6。先确认桌面列表能找到目标模型，再检查该任务的选择；更新默认配置后，新建一个短任务作为对照即可。</p>
          <p>如果界面显示 GPT-6，但仍怀疑实际请求被改写，应保留客户端请求目标、响应模型字段、请求标识和报错时间，让站点管理员结合服务日志判断。模型在对话里自称是谁不是可靠的身份检测方法；响应模型字段也只是链路线索。</p>
        </section>

        <section class="explanation" aria-labelledby="cache-title">
          <h3 id="cache-title">要不要清理 <code>models_cache.json</code>？</h3>
          <p>不要把清缓存作为第一步。先确定当前版本从哪里读取模型元数据，再判断是否真的存在陈旧数据。历史案例中，旧引擎内置目录已经把 Astra 标成 <code>hide</code>，因此删掉外部缓存也不会让旧引擎自动获得新版目录规则。</p>
          <div class="table-scroll"><table><thead><tr><th>对象</th><th>作用</th><th>处理边界</th></tr></thead><tbody>
            <tr><td>模型元数据缓存</td><td>保存客户端可用的模型描述或目录信息</td><td>确认存在、被读取且过时后，才定点清理</td></tr>
            <tr><td><code>config.toml</code> 等配置</td><td>决定入口、默认模型及其他功能</td><td>只调整相关字段，不把它当缓存删除</td></tr>
            <tr><td>本地 memories 与任务历史</td><td>保存长期工作上下文与对话记录</td><td>模型不可见不是删除它们的理由</td></tr>
            <tr><td>中转站 Redis / Docker 数据</td><td>服务端运行状态或存储</td><td>普通使用者不需要也不应为此清理</td></tr>
          </tbody></table></div>
        </section>

        <section class="explanation" aria-labelledby="upgrade-title">
          <h3 id="upgrade-title">怎样升级，才不容易把当前任务打断？</h3>
          <ol>
            <li>先准备更新来源、候选安装包或官方更新流程，保存必要的恢复信息。</li>
            <li>结束正在生成的回复和未完成的工具调用；需要退出应用时，不让同一个活跃任务继续承担最后一步安装。</li>
            <li>通过官方流程完成更新，重新打开应用，核对实际运行版本和模型列表。</li>
            <li>明确选择目标模型，用新任务做最小验证，最后再继续原来的工作。</li>
          </ol>
          <p>切换 API provider 与升级桌面应用不是同一件事。本地历史可继续保留，但不能承诺正在传输的连接和未完成工具调用无缝迁移。</p>
        </section>

        <section class="explanation" aria-labelledby="verify-title">
          <h3 id="verify-title">怎样确认自己的问题已经解决？</h3>
          <ul>
            <li><strong>客户端正确：</strong>重新打开后，实际运行的是预期安装位置和版本，不是旧副本。</li>
            <li><strong>模型可见：</strong>Codex 桌面的模型列表中可以找到 GPT-6-Astra。</li>
            <li><strong>任务选中：</strong>当前任务或用于验证的新任务明确选择目标模型，不只是配置文件写了名称。</li>
            <li><strong>请求完成：</strong>使用自己的本站入口与认证完成一个简短请求，没有认证、权限或服务错误。</li>
          </ul>
          <p>一个短请求成功只证明最小链路，不代表长对话、全部插件、MCP 或工具调用都已验证。</p>
        </section>

        <section class="explanation" aria-labelledby="support-title">
          <h3 id="support-title">仍未恢复时，应该查哪里、联系谁？</h3>
          <div class="table-scroll"><table><thead><tr><th>结果或报错</th><th>优先核对</th><th>何时联系站点管理员</th></tr></thead><tbody>
            <tr><td>API 能调用，桌面没有模型</td><td>桌面实际版本、客户端目录、是否启动旧副本</td><td>本机检查后仍有入口或兼容性疑问，附脱敏结果</td></tr>
            <tr><td>模型不可用或目录没有目标模型</td><td>目标模型 ID、个人 Key 权限、本站实际入口</td><td>需要确认个人访问范围、模型映射或站点支持状态</td></tr>
            <tr><td>401 / 403</td><td>Key 是否失效、是否发往正确地址、权限是否允许</td><td>排除地址错误后仍被拒绝；不要公开提交 Key</td></tr>
            <tr><td>404 / 429 / 超时 / 5xx</td><td>路径与协议、额度并发、本机网络代理和发生时间</td><td>持续复现或需要关联服务端请求日志</td></tr>
          </tbody></table></div>
          <p>联系管理员时提供：系统与客户端版本、桌面或终端、目标模型、发生时间和时区、已做检查、脱敏错误正文及响应给出的 request ID。不要发送完整认证文件、整份配置目录或全部聊天记录。</p>
        </section>

        <p class="experience-scope">本文整理的是 2026-09-06 的 Codex 桌面模型可见性案例，不代表当前每个账号的模型权限或站点实时状态，也不保证所有同类症状有相同根因。Claude Code 可以借鉴入口、权限、客户端版本与实际模型选择分别检查的思路，但本文没有对 Claude Code 做专项验证。</p>
      </section>
    </article>

    <template #footer>
      <Rest2BuildBrandFooter />
    </template>
  </PublicPageLayout>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Rest2BuildBrandFooter from '@/components/common/Rest2BuildBrandFooter.vue'
import { getExperienceById } from '@/content/experiences'
import PublicPageLayout from '@/components/layout/PublicPageLayout.vue'

const { t } = useI18n()
const experience = getExperienceById('gpt-6-astra-not-visible')!
const promptField = ref<HTMLTextAreaElement | null>(null)
const copyStatus = ref('')

const prompt = experience.prompt

const copyLabel = computed(() => (copyStatus.value === '提示词已复制' ? '已复制' : '复制提示词'))

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
.experience { overflow: hidden; border: 1px solid rgba(15, 23, 42, 0.09); border-radius: 8px; background: rgba(255, 255, 255, 0.84); box-shadow: 0 20px 40px -38px rgba(15, 23, 42, 0.45); }
.dark .experience { border-color: rgba(255, 255, 255, 0.12); background: rgba(10, 18, 32, 0.74); }
.experience-intro, .experience-section { padding: 32px 36px; }
.experience-breadcrumb { display: flex; flex-wrap: wrap; gap: 7px; padding: 16px 36px 0; color: #64748b; font-size: 12px; }.experience-breadcrumb a { color: #0f766e; text-decoration: none; }.experience-breadcrumb a:hover { text-decoration: underline; text-underline-offset: 3px; }.dark .experience-breadcrumb { color: #94a3b8; }.dark .experience-breadcrumb a { color: #5eead4; }
.experience-intro { border-bottom: 1px solid rgba(15, 23, 42, 0.09); background: rgba(248, 251, 250, 0.86); }
.dark .experience-intro { border-color: rgba(255, 255, 255, 0.1); background: rgba(13, 25, 31, 0.72); }
.experience-kicker, .experience-step { margin: 0 0 10px; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 12px; color: #0f766e; }
.experience h1 { margin: 0; font-size: 32px; line-height: 1.35; letter-spacing: 0; color: #1f2937; }
.dark .experience h1 { color: #f8fafc; }
.experience-subtitle { margin: 12px 0 0; font-size: 17px; color: #52615c; }.dark .experience-subtitle { color: #cbd5e1; }
.experience-meta { margin: 14px 0 0; font-size: 12px; color: #6b7b74; }.dark .experience-meta { color: #94a3b8; }
.experience-section { border-bottom: 1px solid rgba(15, 23, 42, 0.09); }.dark .experience-section { border-color: rgba(255, 255, 255, 0.1); }
.experience-section:last-child { border-bottom: 0; }.experience-section h2 { margin: 0 0 16px; font-size: 21px; line-height: 1.45; }.experience-section h3 { margin: 26px 0 12px; font-size: 17px; line-height: 1.55; }.experience-section p { margin: 12px 0; color: #45534e; }.dark .experience-section p { color: #d1d5db; }
.experience-section ul, .experience-section ol { margin: 14px 0; padding-left: 22px; color: #45534e; }.dark .experience-section ul, .dark .experience-section ol { color: #d1d5db; }.experience-section li { margin: 8px 0; }
.experience-note, .experience-scope { border-left: 3px solid #0f766e; padding-left: 14px; font-size: 14px; }.experience-scope { margin-top: 32px !important; color: #64748b !important; }
.experience-prompt { background: #f4f8f6; }.dark .experience-prompt { background: rgba(15, 35, 33, 0.64); }.experience-heading-row { display: flex; align-items: center; justify-content: space-between; gap: 18px; }.experience-heading-row .experience-step { margin-bottom: 6px; }.experience-heading-row h2 { margin-bottom: 0; }
.copy-button { flex: none; border: 1px solid #0f766e; border-radius: 5px; background: #0f766e; padding: 8px 12px; font-size: 13px; font-weight: 650; color: #fff; }.copy-button:hover { background: #0b5d57; }.copy-button:focus-visible { outline: 3px solid #38bdf8; outline-offset: 3px; }
.prompt-field { display: block; width: 100%; min-height: 340px; margin-top: 18px; resize: vertical; border: 1px solid #bfcec6; border-radius: 5px; background: #fff; padding: 16px; color: #1f2937; font: 13px/1.9 ui-monospace, SFMono-Regular, Menlo, monospace; }.dark .prompt-field { border-color: #466159; background: #0f1f1b; color: #e5e7eb; }.copy-status { min-height: 22px; margin-bottom: 0 !important; font-size: 12px; color: #0f766e !important; }
.explanation { padding-top: 2px; }.explanation + .explanation { margin-top: 30px; border-top: 1px solid rgba(15, 23, 42, 0.09); }.dark .explanation + .explanation { border-color: rgba(255, 255, 255, 0.1); }.explanation h3 { padding-top: 28px; }
.table-scroll { overflow-x: auto; margin: 18px 0; border: 1px solid #d9e2dd; }.dark .table-scroll { border-color: #3a514a; }.table-scroll table { width: 100%; min-width: 660px; border-collapse: collapse; font-size: 13px; }.table-scroll th { padding: 12px; text-align: left; font-weight: 650; color: #53625c; background: #edf3f0; }.dark .table-scroll th { color: #d1d5db; background: #183029; }.table-scroll td { padding: 12px; vertical-align: top; border-top: 1px solid #d9e2dd; color: #45534e; }.dark .table-scroll td { border-color: #3a514a; color: #d1d5db; }.table-scroll td:first-child { font-weight: 650; }
@media (max-width: 640px) { .experience-intro, .experience-section { padding: 24px 20px; }.experience-breadcrumb { padding: 14px 20px 0; }.experience h1 { font-size: 27px; }.experience-subtitle { font-size: 15px; }.experience-heading-row { align-items: flex-start; }.prompt-field { min-height: 420px; padding: 12px; font-size: 12px; }.table-scroll { margin-right: -20px; margin-left: -20px; border-right: 0; border-left: 0; } }
</style>
