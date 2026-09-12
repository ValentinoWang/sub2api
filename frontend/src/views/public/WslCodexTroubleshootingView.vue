<template>
  <PublicPageLayout>
    <article class="wsl-experience" aria-labelledby="page-title">
      <nav class="breadcrumb" aria-label="经验分享路径">
        <RouterLink to="/home">首页</RouterLink><span aria-hidden="true">/</span>
        <RouterLink to="/experiences">经验分享</RouterLink><span aria-hidden="true">/</span>
        <RouterLink :to="{ path: '/experiences', query: { category: experience.category } }">故障排查</RouterLink><span aria-hidden="true">/</span>
        <span aria-current="page">{{ experience.title }}</span>
      </nav>

      <header class="intro">
        <div>
          <p class="kicker">故障排查 / ERR-003</p>
          <h1 id="page-title">{{ experience.title }}</h1>
          <p class="subtitle">{{ experience.subtitle }}</p>
          <p class="meta">适用：{{ experience.applicableTo }} · 更新：{{ experience.updatedAt }}</p>
        </div>
        <p class="verification-state"><strong>内容状态</strong><span>排查边界已整理</span><small>真实 Windows / WSL2 实操待验证</small></p>
      </header>

      <section class="content-section" aria-labelledby="situation-title">
        <p class="step">01</p>
        <h2 id="situation-title">情况说明</h2>
        <p>你已经在 Windows 11 上打开 Ubuntu，也安装过 Codex，但终端仍提示找不到命令，或者同一条命令在 PowerShell 和 WSL 中得到不同结果。还有一种常见表现是：Codex 可以启动，项目却一直很慢、权限异常，或者 Windows 能联网而 WSL 中登录和请求失败。</p>
        <p>这些现象往往不是同一个故障。Windows、WSL2 发行版、WSL 内的 Codex、工作目录和网络连接是五个连续环节。先确认故障停在哪一层，比反复重装更有效。</p>

        <h3>本文适合这些情况</h3>
        <ul>
          <li>在 PowerShell 能看到某个命令，进入 Ubuntu 后却提示 <code>command not found</code>。</li>
          <li>不确定当前窗口到底是 PowerShell、CMD 还是 WSL Shell。</li>
          <li><code>codex --version</code> 有输出，但不知道运行的是 Windows 侧还是 WSL 侧的程序。</li>
          <li>项目位于 <code>/mnt/c/Users/...</code>，出现速度慢、权限、大小写或软链接异常。</li>
          <li>Windows 浏览器能访问本站，WSL 内却出现 DNS、证书、代理或连接超时。</li>
        </ul>
        <p class="notice"><strong>本文只处理本机 Windows / WSL2 / Codex CLI 环境。</strong>它不涉及让 Codex 编写前端页面，也不要求你登录、重启或修改中转站服务器。</p>
      </section>

      <section class="content-section prompt-section" aria-labelledby="codex-help-title">
        <div class="heading-row">
          <div><p class="step">02</p><h2 id="codex-help-title">Codex 帮你处理</h2></div>
          <button type="button" class="copy-button" :aria-label="copyLabel" @click="copyPrompt">
            <Icon :name="copyStatus === '提示词已复制' ? 'check' : 'copy'" size="sm" />
            <span>{{ copyLabel }}</span>
          </button>
        </div>
        <p>把下面的完整提示词发给能操作你这台电脑的 Codex。它会先标明命令应该在哪个终端执行，再从只读检查开始。</p>
        <textarea ref="promptField" class="prompt-field" readonly aria-label="可直接发给 Codex 的 Windows 11 与 WSL2 排障提示词" :value="experience.prompt"></textarea>
        <p class="copy-status" role="status" aria-live="polite">{{ copyStatus }}</p>
      </section>

      <section class="content-section" aria-labelledby="human-title">
        <p class="step">03</p>
        <h2 id="human-title">给人看的：原因、证据与经验</h2>

        <section class="explanation" aria-labelledby="layers-title">
          <h3 id="layers-title">先把问题放回正确的一层</h3>
          <p>“Codex 用不了”只是最终表现。下面五层中任意一层不对，都会让结果看起来很相似。</p>
          <div class="table-scroll"><table><thead><tr><th>检查层</th><th>它决定什么</th><th>失败时常见表现</th></tr></thead><tbody>
            <tr><td>Windows 与 WSL2</td><td>发行版能否启动，是否实际运行 WSL2</td><td>Ubuntu 打不开、发行版不存在、版本仍为 1</td></tr>
            <tr><td>当前 Shell</td><td>命令由 PowerShell 还是 Linux Shell 解释</td><td><code>$HOME</code>、路径和安装命令行为不一致</td></tr>
            <tr><td>Codex 可执行文件</td><td>当前命令实际运行哪一份 Codex</td><td>找不到命令、版本不对、调用 Windows 挂载路径</td></tr>
            <tr><td>HOME 与工作目录</td><td>配置从哪里读取，项目使用哪套文件语义</td><td>登录状态不同、配置不生效、权限或软链接异常</td></tr>
            <tr><td>WSL 网络</td><td>DNS、TLS、代理和 API 入口能否连通</td><td>Windows 可访问，WSL 中超时或认证失败</td></tr>
          </tbody></table></div>
        </section>

        <section class="explanation" aria-labelledby="shell-title">
          <h3 id="shell-title">PowerShell 和 WSL Shell 各自检查什么？</h3>
          <p>下面两组命令不能混着运行。PowerShell 管理 WSL 发行版；Ubuntu Shell 检查 Linux 环境本身。</p>
          <div class="command-grid">
            <div class="command-block"><strong>Windows PowerShell</strong><pre><code>wsl --status
wsl -l -v</code></pre><p>目标发行版应存在，并在 <code>VERSION</code> 列显示 <code>2</code>。如果不是 2，先查当前微软文档和机器条件，不要删除或注销发行版。</p></div>
            <div class="command-block"><strong>WSL 的 Ubuntu Shell</strong><pre><code>echo "$WSL_DISTRO_NAME"
pwd
uname -a
printf '%s\n' "$HOME"</code></pre><p>应看到发行版名称、Linux 内核和 WSL 用户的 HOME。若命令语法在当前窗口无法解释，先确认是否开错终端。</p></div>
          </div>
        </section>

        <section class="explanation" aria-labelledby="binary-title">
          <h3 id="binary-title">“安装过 Codex”不等于 WSL 正在运行它</h3>
          <p>Windows 和 WSL 有各自的用户目录与 <code>PATH</code>。即使 Windows 侧已经安装，WSL 也不一定拥有同一份命令；反过来，WSL 可能通过互操作解析到 Windows 挂载目录里的可执行文件。</p>
          <div class="command-block compact"><strong>在 WSL 中检查</strong><pre><code>command -v codex
type -a codex
codex --version</code></pre></div>
          <p><code>command -v</code> 给出本次会运行的路径，<code>type -a</code> 列出当前 Shell 能找到的候选。先确认路径和版本，再决定是否安装或修复。为了解决一个 <code>command not found</code> 就删除整个 <code>~/.codex</code>，会把配置、任务历史和其他本地状态一起置于风险中。</p>
        </section>

        <section class="explanation" aria-labelledby="filesystem-title">
          <h3 id="filesystem-title">为什么工作目录在 <code>/mnt/c</code> 时更容易出问题？</h3>
          <p><code>/mnt/c</code> 是 WSL 对 Windows 文件系统的挂载入口。它可以使用，但文件 I/O、权限位、大小写、文件监听和软链接的行为可能与 WSL 自己的 Linux 文件系统不同。遇到依赖安装慢、权限反复变化或软链接失败时，工作目录是需要核对的证据。</p>
          <p>新的仓库通常放在 <code>~/code/项目名</code> 更省事。已有仓库不能直接搬走：先确认未提交修改、未跟踪文件和大文件，再制定复制、校验和回退步骤。原目录验证完成前不要删除。</p>
        </section>

        <section class="explanation" aria-labelledby="network-title">
          <h3 id="network-title">Windows 能联网，为什么 WSL 仍可能失败？</h3>
          <p>WSL 有自己的进程环境。Windows 浏览器使用的代理、证书或登录状态不一定自动传给 Ubuntu Shell。排查时要把 DNS、TLS、代理和认证分开看。</p>
          <div class="table-scroll"><table><thead><tr><th>结果</th><th>说明了什么</th><th>下一步</th></tr></thead><tbody>
            <tr><td>DNS 解析失败</td><td>还没有到达 HTTP 服务</td><td>检查 WSL DNS、VPN 和网络策略</td></tr>
            <tr><td>TLS 或证书错误</td><td>已开始建立安全连接，但信任链或代理可能有问题</td><td>保留脱敏错误，核对系统时间、代理和证书来源</td></tr>
            <tr><td>连接超时</td><td>可能是路由、代理、防火墙或服务不可达</td><td>记录时间、目标主机和是否持续复现</td></tr>
            <tr><td>401 / 403</td><td>到达了某个 HTTP 服务，但认证或权限未通过</td><td>核对实际 Base URL 与 Key 状态，不公开 Key</td></tr>
            <tr><td>请求成功</td><td>这一次最小链路可用</td><td>仍需确认 Codex 实际使用同一配置完成交互</td></tr>
          </tbody></table></div>
          <p>网络探针成功不等于 Codex 已经选中正确配置；Codex 能启动也不等于付费模型请求已完成。可能产生费用的验证应先说明并只做一次。</p>
        </section>

        <section class="explanation" aria-labelledby="order-title">
          <h3 id="order-title">建议按这个顺序处理</h3>
          <ol class="recovery-steps">
            <li><span>1</span><div><strong>确认 WSL2 本身</strong><p>在 PowerShell 查看发行版、状态和版本。</p></div></li>
            <li><span>2</span><div><strong>确认当前 Shell</strong><p>在 Ubuntu 中查看发行版名称、内核、HOME 和工作目录。</p></div></li>
            <li><span>3</span><div><strong>确认 Codex 身份</strong><p>读取实际命令路径和版本，判断它属于 Windows 还是 WSL。</p></div></li>
            <li><span>4</span><div><strong>检查目录与配置</strong><p>确认项目位置和 WSL 用户目录中的配置，不用另一侧的结果代替。</p></div></li>
            <li><span>5</span><div><strong>检查网络并最小验证</strong><p>分开确认可达性、认证和一次真实 Codex 交互。</p></div></li>
          </ol>
        </section>

        <section class="explanation" aria-labelledby="verify-title">
          <h3 id="verify-title">怎样才算恢复？</h3>
          <ul>
            <li><strong>环境明确：</strong>目标发行版运行在 WSL2，当前窗口能识别为该发行版的 Linux Shell。</li>
            <li><strong>命令明确：</strong><code>command -v codex</code> 指向预期的 WSL 安装位置，版本可读取。</li>
            <li><strong>目录明确：</strong>知道当前 HOME 和工作目录位于哪一侧，不再混用 Windows 与 Linux 路径。</li>
            <li><strong>连接明确：</strong>DNS、TLS、代理、认证分别检查，错误停在哪一层有可复述的证据。</li>
            <li><strong>行为恢复：</strong>Codex 能从 WSL 启动，并在获得费用授权后完成一次简短交互。</li>
          </ul>
          <p>联系站点管理员时，提供 Windows 版本、发行版名称与 WSL 版本、Codex 路径与版本、发生时间和时区、脱敏错误正文及请求标识。不要发送完整 API Key、认证文件、整份配置目录或私人对话。</p>
        </section>

        <p class="scope">本文从原 Windows 11 + WSL2 教程中提炼环境排障部分，并补齐了诊断边界。页面结构、分类、提示词复制和响应式布局可在本地验证；真实 Windows 11 / WSL2 命令执行与初学者理解仍需人工实操，本文不把它们描述为已验证。</p>
        <div class="references" aria-label="相关资料">
          <a href="https://learn.chatgpt.com/docs/windows/wsl" target="_blank" rel="noopener noreferrer">Codex WSL 官方文档 <Icon name="externalLink" size="sm" /></a>
          <RouterLink to="/experiences/windows-11-wsl-codex-frontend">查看完整前端开发教程</RouterLink>
          <RouterLink :to="{ path: '/experiences', query: { category: 'troubleshooting' } }">返回故障排查</RouterLink>
        </div>
      </section>
    </article>

    <template #footer><Rest2BuildBrandFooter /></template>
  </PublicPageLayout>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import Rest2BuildBrandFooter from '@/components/common/Rest2BuildBrandFooter.vue'
import PublicPageLayout from '@/components/layout/PublicPageLayout.vue'
import { getExperienceById } from '@/content/experiences'

const experience = getExperienceById('windows-11-wsl2-codex-environment')!
const promptField = ref<HTMLTextAreaElement | null>(null)
const copyStatus = ref('')
const copyLabel = computed(() => copyStatus.value === '提示词已复制' ? '已复制' : '复制提示词')

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
.wsl-experience{overflow:hidden;border:1px solid #d6e0dc;border-radius:8px;background:#fff;color:#22302b;box-shadow:0 20px 42px -38px rgba(15,23,42,.45)}
.dark .wsl-experience{border-color:#31463f;background:#0d1916;color:#eaf4ef}
.breadcrumb{display:flex;flex-wrap:wrap;gap:7px;padding:16px 32px 0;color:#6b7a74;font-size:12px}.breadcrumb a{color:#0f766e;text-decoration:none}.breadcrumb a:hover{text-decoration:underline;text-underline-offset:3px}.dark .breadcrumb{color:#94a3b8}.dark .breadcrumb a{color:#5eead4}
.intro{display:grid;grid-template-columns:minmax(0,1fr) 230px;gap:32px;align-items:end;padding:34px 32px;border-bottom:1px solid #d6e0dc;background:#eef7f3}.dark .intro{border-color:#31463f;background:#12261f}
.kicker,.step{margin:0 0 9px;color:#0f766e;font:700 12px/1.4 ui-monospace,SFMono-Regular,Menlo,monospace}.dark .kicker,.dark .step{color:#5eead4}
.intro h1{max-width:780px;margin:0;font-size:34px;line-height:1.35;letter-spacing:0}.subtitle{max-width:730px;margin:13px 0 0;color:#4d6058;font-size:17px;line-height:1.65}.dark .subtitle{color:#c1d0ca}.meta{margin:14px 0 0;color:#6b7a74;font-size:12px}.dark .meta{color:#9fb0a9}
.verification-state{display:flex;flex-direction:column;gap:6px;margin:0;border-left:2px solid #0f766e;padding:4px 0 4px 16px}.verification-state strong{font-size:12px;color:#0f766e}.verification-state span{font-size:14px;font-weight:700}.verification-state small{color:#697a73;font-size:12px;line-height:1.55}.dark .verification-state strong{color:#5eead4}.dark .verification-state small{color:#a9bbb3}
.content-section{padding:32px;border-bottom:1px solid #d6e0dc}.dark .content-section{border-color:#31463f}.content-section:last-child{border-bottom:0}.content-section h2{margin:0 0 16px;font-size:22px;line-height:1.45}.content-section h3{margin:28px 0 12px;font-size:18px;line-height:1.5}.content-section p,.content-section li{color:#465750;line-height:1.75}.dark .content-section p,.dark .content-section li{color:#cad8d2}.content-section ul{padding-left:22px}.content-section li{margin:7px 0}.content-section code{font-family:ui-monospace,SFMono-Regular,Menlo,monospace}
.notice,.scope{border-left:3px solid #0f766e;background:#f0f7f4;padding:13px 15px;font-size:14px}.dark .notice,.dark .scope{background:#122a22}.scope{margin-top:34px!important;color:#5d6e67!important}
.prompt-section{background:#f5f9f7}.dark .prompt-section{background:#10231d}.heading-row{display:flex;align-items:flex-start;justify-content:space-between;gap:18px}.heading-row .step{margin-bottom:6px}.heading-row h2{margin-bottom:0}.copy-button{display:inline-flex;flex:none;align-items:center;gap:7px;border:1px solid #0f766e;border-radius:5px;background:#0f766e;padding:8px 11px;color:#fff;font-size:13px;font-weight:700}.copy-button:hover{background:#0b5f59}.copy-button:focus-visible,.references a:focus-visible{outline:3px solid #38bdf8;outline-offset:3px}.prompt-field{display:block;box-sizing:border-box;width:100%;min-height:420px;margin-top:17px;resize:vertical;border:1px solid #b8cac2;border-radius:5px;background:#fff;padding:16px;color:#1f2937;font:13px/1.85 ui-monospace,SFMono-Regular,Menlo,monospace}.dark .prompt-field{border-color:#426056;background:#0b1713;color:#e5eee9}.copy-status{min-height:22px;margin-bottom:0!important;color:#0f766e!important;font-size:12px}
.explanation{padding-top:2px}.explanation+.explanation{margin-top:30px;border-top:1px solid #dde5e1}.dark .explanation+.explanation{border-color:#2d433b}.explanation+.explanation h3{padding-top:28px}
.table-scroll{overflow-x:auto;margin:18px 0;border:1px solid #d5dfda}.dark .table-scroll{border-color:#354f46}.table-scroll table{width:100%;min-width:680px;border-collapse:collapse;font-size:13px}.table-scroll th{padding:12px;text-align:left;color:#51615b;background:#edf3f0}.dark .table-scroll th{color:#d4dfda;background:#183027}.table-scroll td{padding:12px;vertical-align:top;border-top:1px solid #d5dfda;color:#465750;line-height:1.65}.dark .table-scroll td{border-color:#354f46;color:#cad8d2}.table-scroll td:first-child{font-weight:700}
.command-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:16px;margin-top:18px}.command-block{min-width:0;border:1px solid #cedbd5;background:#f8fbfa}.dark .command-block{border-color:#354f46;background:#101e19}.command-block>strong{display:block;padding:11px 14px;color:#0f766e;font-size:13px}.dark .command-block>strong{color:#5eead4}.command-block pre{overflow-x:auto;margin:0;border-block:1px solid #cedbd5;background:#101916;padding:15px;color:#e8f2ed;font:13px/1.75 ui-monospace,SFMono-Regular,Menlo,monospace}.dark .command-block pre{border-color:#354f46}.command-block p{margin:0;padding:12px 14px;font-size:13px}.command-block.compact{max-width:720px;margin-top:18px}
.recovery-steps{display:grid;grid-template-columns:repeat(5,minmax(0,1fr));gap:0;margin:20px 0 0;padding:0!important;list-style:none}.recovery-steps li{position:relative;display:grid;grid-template-columns:28px minmax(0,1fr);gap:8px;margin:0;padding:14px 14px 0 0;border-top:2px solid #9ec1b4}.recovery-steps li>span{display:grid;width:24px;height:24px;place-items:center;border-radius:50%;background:#0f766e;color:#fff;font:700 11px ui-monospace,SFMono-Regular,Menlo,monospace}.recovery-steps strong{font-size:13px}.recovery-steps p{margin:5px 0 0;font-size:12px;line-height:1.55}
.references{display:flex;flex-wrap:wrap;gap:10px;margin-top:22px}.references a{display:inline-flex;align-items:center;gap:6px;border:1px solid #9db7ad;border-radius:5px;padding:9px 11px;color:#0f766e;font-size:13px;font-weight:700;text-decoration:none}.dark .references a{border-color:#42675a;color:#5eead4}
@media(max-width:900px){.intro{grid-template-columns:1fr}.verification-state{max-width:360px}.recovery-steps{grid-template-columns:repeat(2,minmax(0,1fr));row-gap:18px}}
@media(max-width:640px){.breadcrumb{padding:14px 20px 0}.intro,.content-section{padding:25px 20px}.intro{gap:22px}.intro h1{font-size:28px}.subtitle{font-size:15px}.heading-row{align-items:flex-start}.copy-button span{display:none}.prompt-field{min-height:520px;padding:13px;font-size:12px}.command-grid{grid-template-columns:1fr}.recovery-steps{grid-template-columns:1fr;row-gap:16px}.references{align-items:stretch;flex-direction:column}.references a{justify-content:center;text-align:center}}
</style>
