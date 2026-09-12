<template>
  <PublicPageLayout>
    <article class="tutorial" aria-labelledby="tutorial-title">
      <nav class="breadcrumb" aria-label="经验分享路径">
        <RouterLink to="/home">首页</RouterLink><span>/</span>
        <RouterLink to="/experiences">经验分享</RouterLink><span>/</span>
        <span aria-current="page">Windows 11 + WSL2 + Codex</span>
      </nav>

      <header class="hero">
        <div class="hero-copy">
          <p class="eyebrow">经验分享 · Windows 11 / WSL2 · 约 35 分钟</p>
          <h1 id="tutorial-title">Windows 11 + WSL2 使用 Codex：第一次真实前端开发</h1>
          <p class="hero-summary">如果你已经有一个前端项目，却不知道该怎样让 Codex 真正接手开发，这篇经验可以直接跟着做。我们从打开 WSL 开始，最后以你亲眼确认页面效果结束。</p>
        </div>
        <div class="responsibility" aria-label="你和 Codex 的职责分工">
          <div><span class="actor actor-human">你负责</span><strong>说清目标 · 判断关键方案 · 检查最终效果</strong></div>
          <div><span class="actor actor-ai">Codex 负责</span><strong>找文件 · 改代码 · 启动项目 · 测试与自查</strong></div>
        </div>
      </header>

      <section class="article-guide" aria-labelledby="guide-title">
        <div>
          <p class="section-label">这篇经验怎么用</p>
          <h2 id="guide-title">打开你自己的仓库，做到哪里看到哪里</h2>
          <p>这不是一张需要背下来的命令清单。带着一个真实项目照着操作，遇到仓库差异时让 Codex 自己读取配置并找出正确命令。</p>
        </div>
        <ol>
          <li><span>01</span><strong>标有“WSL 终端命令”</strong><p>输入 Ubuntu 的 Shell。</p></li>
          <li><span>02</span><strong>标有“发给 Codex”</strong><p>输入 Codex 对话框。</p></li>
          <li><span>03</span><strong>标有“你来做”</strong><p>需要你判断或验收。</p></li>
        </ol>
      </section>

      <section class="learning-goals" aria-labelledby="goals-title">
        <div>
          <p class="section-label">跟着做完以后</p>
          <h2 id="goals-title">你会完成一次真正的前端修改</h2>
        </div>
        <ul>
          <li v-for="goal in goals" :key="goal"><Icon name="check" size="sm" />{{ goal }}</li>
        </ul>
      </section>

      <section class="workflow" aria-labelledby="workflow-title">
        <div class="section-heading">
          <p class="section-label">这次要走的路</p>
          <h2 id="workflow-title">从打开 WSL，到亲手确认页面效果</h2>
        </div>
        <ol class="workflow-track">
          <li v-for="(node, index) in workflow" :key="node">
            <span>{{ String(index + 1).padStart(2, '0') }}</span><strong>{{ node }}</strong>
          </li>
        </ol>
      </section>

      <section class="steps" aria-label="实操步骤">
        <article v-for="step in steps" :key="step.id" class="step" :id="`step-${step.id}`">
          <aside class="step-rail">
            <span class="step-number">{{ step.id }}</span>
            <span :class="['actor', step.actor === '你来做' ? 'actor-human' : step.actor === 'Codex 来做' ? 'actor-ai' : 'actor-shared']">{{ step.actor }}</span>
          </aside>
          <div class="step-content">
            <p class="step-kicker">{{ step.kicker }}</p>
            <h2>{{ step.title }}</h2>
            <p v-for="paragraph in step.intro" :key="paragraph">{{ paragraph }}</p>

            <div v-if="step.warning" class="warning"><Icon name="shield" size="sm" /><strong>{{ step.warning }}</strong></div>

            <div v-for="block in step.blocks" :key="block.label" :class="['input-card', `input-${block.kind}`]">
              <div class="input-card-head">
                <div class="input-label"><Icon :name="block.kind === 'terminal' ? 'terminal' : 'sparkles'" size="sm" /><span>{{ block.label }}</span></div>
                <button type="button" class="copy-button" :aria-label="`复制${block.label}`" @click="copyBlock(step.id, block)">
                  <Icon :name="copiedKey === `${step.id}-${block.label}` ? 'check' : 'copy'" size="sm" />
                  <span>{{ copiedKey === `${step.id}-${block.label}` ? '已复制' : '复制' }}</span>
                </button>
              </div>
              <pre><code>{{ block.content }}</code></pre>
            </div>

            <div class="why">
              <span>Why</span>
              <div><strong>为什么这样做？</strong><p>{{ step.why }}</p></div>
            </div>

            <ul v-if="step.checks?.length" class="step-checks">
              <li v-for="check in step.checks" :key="check"><Icon name="check" size="sm" />{{ check }}</li>
            </ul>
          </div>
        </article>
      </section>

      <section class="acceptance-loop" aria-labelledby="loop-title">
        <div class="section-heading">
          <p class="section-label">把一次开发真正做完</p>
          <h2 id="loop-title">Codex 做重复工作，你负责最后判断</h2>
          <p>页面不符合预期时，把你看到的具体问题告诉 Codex，让它继续修复和重测。这个循环比你接过代码自己重做更有效。</p>
        </div>
        <ol>
          <li v-for="(node, index) in acceptanceLoop" :key="`${index}-${node}`" :class="node === '你看页面' || node === '验收通过' ? 'human-node' : ''">
            <span>{{ node }}</span><Icon v-if="index < acceptanceLoop.length - 1" name="chevronRight" size="sm" />
          </li>
        </ol>
      </section>

      <section class="exercise" aria-labelledby="exercise-title">
        <div class="exercise-heading">
          <div><p class="section-label">现在轮到你</p><h2 id="exercise-title">让 Codex 为你的项目增加 About 页面</h2></div>
          <span class="actor actor-human">你来验收</span>
        </div>
        <p>沿用当前设计系统，添加路由，支持移动端，提供返回入口，并且不影响已有页面。不要先自己逐个打开文件修改。</p>
        <div class="input-card input-prompt">
          <div class="input-card-head">
            <div class="input-label"><Icon name="sparkles" size="sm" /><span>› 发给 Codex 的 Prompt</span></div>
            <button type="button" class="copy-button" aria-label="复制最终练习提示词" @click="copyExercise">
              <Icon :name="copiedKey === 'exercise' ? 'check' : 'copy'" size="sm" />
              <span>{{ copiedKey === 'exercise' ? '已复制' : '复制' }}</span>
            </button>
          </div>
          <pre><code>{{ exercisePrompt }}</code></pre>
        </div>
        <div class="reflection">
          <strong>完成后回答</strong>
          <p>哪些判断必须由人完成？哪些机械工作已经没有必要由人手工完成？</p>
        </div>
      </section>

      <footer class="tutorial-footer">
        <div><strong>命令或界面和文章不一样？</strong><p>Codex 会持续更新。先让它读取当前仓库和 <code>--help</code>，再以官方文档为准。</p></div>
        <div class="footer-links">
          <a href="https://learn.chatgpt.com/docs/windows/wsl" target="_blank" rel="noopener noreferrer">查看 Codex WSL 官方文档</a>
          <RouterLink to="/experiences">返回经验分享</RouterLink>
        </div>
      </footer>

      <p class="copy-status" role="status" aria-live="polite">{{ copyStatus }}</p>
    </article>

    <template #footer><Rest2BuildBrandFooter /></template>
  </PublicPageLayout>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import Rest2BuildBrandFooter from '@/components/common/Rest2BuildBrandFooter.vue'
import PublicPageLayout from '@/components/layout/PublicPageLayout.vue'
import { useClipboard } from '@/composables/useClipboard'

type Actor = '你来做' | 'Codex 来做' | '一起确认'
type InputBlock = { kind: 'terminal' | 'prompt'; label: string; content: string }
type TutorialStep = {
  id: string
  actor: Actor
  kicker: string
  title: string
  intro: string[]
  blocks: InputBlock[]
  why: string
  warning?: string
  checks?: string[]
}

const { copyToClipboard } = useClipboard()
const copiedKey = ref('')
const copyStatus = ref('')

const goals = [
  '确认自己打开的是 WSL，而不是 PowerShell 或 CMD',
  '让 Codex 先读懂真实仓库，再开始修改代码',
  '把启动、测试和代码自查交给 Codex 完成',
  '只在关键方案和最终页面效果上亲自做判断',
]

const workflow = ['Windows 11', '进入 WSL', '打开仓库', '启动 Codex', '说清需求', 'Codex 开发', 'Codex 自查', '你来验收']

const steps: TutorialStep[] = [
  {
    id: '01', actor: '你来做', kicker: '先看终端身份', title: '第一件事：确认打开的是 WSL 终端',
    intro: ['Windows 里可能同时装着 PowerShell、CMD 和 Ubuntu。下面三条命令能让你确认：当前窗口是不是 WSL、现在位于哪个目录、运行的是什么内核。'],
    blocks: [{ kind: 'terminal', label: '$ WSL 终端命令', content: 'echo $WSL_DISTRO_NAME\npwd\nuname -a' }],
    warning: '不要把 PowerShell、CMD 和 WSL Shell 混为一谈。',
    why: '后续路径、权限和安装命令都以 Linux 为准。先确认环境，可以避免把 Windows 路径和 Linux 工具链混在一起。',
    checks: ['看到 Ubuntu 或其他发行版名称', '当前路径类似 /home/username', 'uname 输出以 Linux 开头'],
  },
  {
    id: '02', actor: '你来做', kicker: '准备一个真实项目', title: '把仓库放到 WSL 自己的文件系统里',
    intro: ['新建一个专门放代码的目录，再把你自己的 GitHub 项目克隆进来。后面的修改、启动和测试都会发生在这个仓库里。'],
    blocks: [{ kind: 'terminal', label: '$ WSL 终端命令', content: 'mkdir -p ~/code\ncd ~/code\ngit clone <repository-url>\ncd <repository-name>' }],
    why: '相比 /mnt/c/Users/...，/home/username/code/project 通常有更好的文件 I/O、权限兼容性、symlink 支持和 Node.js 工具链体验。',
  },
  {
    id: '03', actor: '你来做', kicker: '安装与登录', title: '在 WSL 里面安装并启动 Codex',
    intro: ['这些命令都要在刚才确认过的 WSL 终端里执行。安装完成后先看路径和版本，再启动 Codex；第一次运行时跟随当时的界面提示登录。'],
    blocks: [{ kind: 'terminal', label: '$ WSL 终端命令', content: 'curl -fsSL https://chatgpt.com/codex/install.sh | sh\nwhich codex\ncodex --version\ncodex' }],
    why: 'which 和版本检查能确认你运行的是 WSL 内安装的 Codex。登录界面可能更新，因此教程不硬编码具体按钮和步骤。',
  },
  {
    id: '04', actor: 'Codex 来做', kicker: '先调查，再动手', title: '第一句话先让 Codex 带你看懂项目',
    intro: ['很多失败都来自一上来就让 AI 写代码。更稳妥的做法是先让 Codex 找到入口、路由、组件和测试命令，此时明确告诉它暂时不要修改。'],
    blocks: [{ kind: 'prompt', label: '› 发给 Codex 的 Prompt', content: `先不要修改任何代码。

请阅读这个仓库，告诉我：

1. 这个项目是什么；
2. 使用什么前端技术栈；
3. 项目入口在哪里；
4. 页面路由在哪里；
5. UI 组件放在哪里；
6. 如何启动开发环境；
7. 有哪些 lint、typecheck、test、Playwright 或其他验收命令；
8. 如果我要新增一个学习教程页面，大概率需要修改哪些文件。

先完成代码库调查，再给出实施计划。` }],
    why: 'Agent 只有先理解入口、组件和测试契约，才有条件复用现有设计并控制修改范围。',
  },
  {
    id: '05', actor: '一起确认', kicker: '把规则留在仓库里', title: '用 /init 建立项目的 AGENTS.md',
    intro: ['如果仓库还没有给 AI 看的项目说明，可以在 Codex 中输入 /init。Codex 会生成初稿，你需要检查它有没有把架构、禁区和测试要求写对。'],
    blocks: [{ kind: 'prompt', label: '› Codex 斜杠菜单', content: '/init' }],
    why: '架构、禁止修改区域、测试和提交要求写入仓库后，未来任务不必每次重复说明。生成的内容仍需要人判断是否符合项目实际。',
    checks: ['记录架构和技术栈', '写清禁止修改的位置', '写清测试与验收要求'],
  },
  {
    id: '06', actor: '一起确认', kicker: '把需求一次说清楚', title: '现在给 Codex 一个真实的前端任务',
    intro: ['不要只说“帮我写个页面”。告诉它目标、所在体系、不能破坏的内容，以及你最终会检查什么；Codex 给出方案后，你只需要判断关键方向是否合理。'],
    blocks: [{ kind: 'prompt', label: '› 发给 Codex 的 Prompt', content: `请在现有学习页面体系中增加一个新的教程页面：

《第一次使用 Codex 完成前端开发》

要求：
1. 复用现有组件和设计系统，不重复造 UI 组件；
2. 保持现有页面视觉风格，支持桌面端和移动端；
3. 教程包含可复制的终端命令块；
4. 每个阶段解释“为什么这样做”；
5. 页面最后增加一个实际练习；
6. 不修改与本需求无关的业务逻辑。

实施前先给出修改文件列表和实现方案。
确认方案没有明显问题后直接完成实现。` }],
    why: '“帮我写个页面”缺少上下文和完成条件。清楚的约束让 Agent 能自行做常规实现决定，同时保留人的关键判断权。',
  },
  {
    id: '07', actor: 'Codex 来做', kicker: '不要替它查命令', title: '让 Codex 自己把项目跑起来',
    intro: ['这一步不需要你先翻 package.json 或试一串 npm 命令。让 Codex 根据仓库现有配置判断包管理器、安装缺失依赖、启动项目并处理错误。'],
    blocks: [{ kind: 'prompt', label: '› 发给 Codex 的 Prompt', content: `请根据仓库现有配置启动本地开发环境。

如果遇到依赖、端口或者配置错误，先定位原因并解决。

不要修改与启动问题无关的业务代码。` }],
    why: '命令发现和重复排错属于 Agent 擅长的机械工作。人应关注启动后的页面是否符合目标。',
  },
  {
    id: '08', actor: 'Codex 来做', kicker: '写完不等于做完', title: '让 Codex 自己完成第一轮检查',
    intro: ['看到页面以后先别继续加功能。让 Codex 按仓库已有能力检查类型、Lint、测试、浏览器、Console 和本次 diff，发现问题就直接修复并重跑。'],
    blocks: [{ kind: 'prompt', label: '› 发给 Codex 的 Prompt', content: `现在不要继续增加功能。

请对刚才的实现进行验收：
1. 检查 TypeScript；
2. 执行 lint；
3. 执行与本页面有关的测试；
4. 如果项目已有 Playwright，执行对应浏览器验收；
5. 检查桌面和移动端页面；
6. 检查 console error；
7. 检查是否修改了需求之外的文件；
8. 总结所有测试结果。

发现问题直接修复，再重新执行失败项，直到达到可以交给人验收的状态。` }],
    why: '类型检查、Lint、测试、Smoke Test 和浏览器检查互相补充。失败项应由 Agent 修复并重跑，而不是把半成品直接交给人。',
    checks: ['开发服务器', 'Typecheck', 'Lint', '相关测试', '桌面与移动端', 'Console', 'Git diff'],
  },
  {
    id: '09', actor: 'Codex 来做', kicker: '再换一个角度看', title: '让 Codex 把自己的修改重新 Review 一遍',
    intro: ['测试通过后，再让 Codex 以审查者身份检查工作区。它需要重新质疑是否改多了、是否重复造组件，以及移动端和错误处理有没有遗漏。'],
    blocks: [
      { kind: 'prompt', label: '› Codex 斜杠菜单', content: '/review' },
      { kind: 'prompt', label: '› 发给 Codex 的 Prompt', content: `请站在代码审查者角度重新检查本次修改。

重点检查：不必要修改、重复组件、架构偏离、前端状态 bug、响应式问题、错误处理和测试缺口。

不要因为代码是你自己写的就默认它是正确的。` },
    ],
    why: 'Review 阶段重新质疑实现选择，更容易发现范围漂移、重复代码和只在特定视口出现的问题。',
  },
  {
    id: '10', actor: '你来做', kicker: '最后才轮到你验收', title: '打开页面，亲自判断结果是不是你要的',
    intro: ['现在再亲手操作页面。你不需要重复跑 Codex 已经完成的机械检查，只需要判断页面是否符合目标、用起来是否顺手，以及业务逻辑有没有偏差。'],
    blocks: [],
    why: '自动测试可以证明既定规则被满足，但无法替你判断产品是否解决了真实问题。把发现的问题描述给 Codex，再让它修复和重验。',
    checks: ['结果：页面是不是我要的', '体验：实际使用是否顺手', '业务：是否符合真实需求', '异常：机器是否遗漏了关键情况'],
  },
]

const acceptanceLoop = ['说清目标', 'Codex 调查', '确认方案', 'Codex 修改', '自动检查', '自我 Review', '你看页面', '反馈问题', '再次修复', '验收通过']

const exercisePrompt = `请为当前项目增加一个 About 页面。

要求：
- 沿用当前设计系统；
- 添加路由；
- 支持移动端；
- 有返回入口；
- 不影响已有页面。

请依次完成：调查项目、制定计划、实现、启动项目、自行测试、自行 Review，最后把结果交给我验收。`

async function copyText(key: string, text: string) {
  const success = await copyToClipboard(text, '内容已复制')
  copiedKey.value = success ? key : ''
  copyStatus.value = success ? '内容已复制，可以粘贴到对应的终端或 Codex 输入框。' : '复制失败，请选中文本后手动复制。'
  if (success) window.setTimeout(() => { if (copiedKey.value === key) copiedKey.value = '' }, 2000)
}

function copyBlock(stepId: string, block: InputBlock) {
  return copyText(`${stepId}-${block.label}`, block.content)
}

function copyExercise() {
  return copyText('exercise', exercisePrompt)
}
</script>

<style scoped>
.tutorial{overflow:hidden;border:1px solid #d9e3df;border-radius:8px;background:#fff;color:#1f2d28;box-shadow:0 24px 54px -48px rgba(15,23,42,.55)}
.dark .tutorial{border-color:#2c433d;background:#0d1916;color:#edf7f2}
.breadcrumb{display:flex;flex-wrap:wrap;gap:8px;padding:18px 32px 0;color:#687972;font-size:12px}.breadcrumb a{color:#0f766e;text-decoration:none}.dark .breadcrumb{color:#92a9a0}.dark .breadcrumb a{color:#5eead4}
.hero{display:grid;grid-template-columns:minmax(0,1.65fr) minmax(260px,.75fr);gap:36px;padding:38px 32px 34px;border-bottom:1px solid #d9e3df;background:#f2f8f5}.dark .hero{border-color:#2c433d;background:#12231e}
.eyebrow,.section-label,.step-kicker{margin:0 0 9px;color:#0f766e;font:700 12px/1.4 ui-monospace,SFMono-Regular,Menlo,monospace}.dark .eyebrow,.dark .section-label,.dark .step-kicker{color:#5eead4}
.hero h1{max-width:820px;margin:0;font-size:36px;line-height:1.25;letter-spacing:0}.hero-summary{max-width:760px;margin:16px 0 0;color:#52635c;font-size:17px;line-height:1.75}.dark .hero-summary{color:#bdcec7}
.responsibility{align-self:end;border-left:1px solid #bfcfc8;padding-left:24px}.dark .responsibility{border-color:#395149}.responsibility div{display:flex;flex-direction:column;gap:8px;padding:14px 0}.responsibility div+div{border-top:1px solid #d4dfda}.dark .responsibility div+div{border-color:#30483f}.responsibility strong{font-size:13px;line-height:1.55}
.actor{display:inline-flex;width:max-content;align-items:center;border-radius:999px;padding:4px 8px;font:700 11px/1.2 ui-monospace,SFMono-Regular,Menlo,monospace}.actor-human{background:#e9f0ff;color:#1d4ed8}.actor-ai{background:#dff7ed;color:#0f766e}.actor-shared{background:#fff1d6;color:#9a5a05}.dark .actor-human{background:#172c50;color:#bfdbfe}.dark .actor-ai{background:#12372f;color:#99f6e4}.dark .actor-shared{background:#3e2b13;color:#fde68a}
.article-guide,.learning-goals,.workflow,.acceptance-loop,.exercise,.tutorial-footer{padding:32px;border-bottom:1px solid #d9e3df}.dark .article-guide,.dark .learning-goals,.dark .workflow,.dark .acceptance-loop,.dark .exercise,.dark .tutorial-footer{border-color:#2c433d}
.article-guide{display:grid;grid-template-columns:minmax(0,.8fr) minmax(0,1.2fr);gap:36px;background:#fff}.dark .article-guide{background:#0d1916}.article-guide h2{margin:0;font-size:23px;line-height:1.4}.article-guide>div>p:last-child{max-width:620px;margin:12px 0 0;color:#52635c;line-height:1.75}.dark .article-guide>div>p:last-child{color:#bdcec7}.article-guide ol{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:18px;margin:0;padding:0;list-style:none}.article-guide li{border-top:2px solid #8bbcac;padding-top:12px}.article-guide li>span{color:#0f766e;font:700 11px ui-monospace,SFMono-Regular,Menlo,monospace}.dark .article-guide li>span{color:#5eead4}.article-guide li strong{display:block;margin-top:6px;font-size:13px;line-height:1.45}.article-guide li p{margin:5px 0 0;color:#687972;font-size:12px;line-height:1.55}.dark .article-guide li p{color:#9fb1a9}
.learning-goals{display:grid;grid-template-columns:minmax(220px,.65fr) minmax(0,1.35fr);gap:36px}.learning-goals h2,.section-heading h2,.exercise h2{margin:0;font-size:23px;line-height:1.4}.learning-goals ul{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px;margin:0;padding:0;list-style:none}.learning-goals li,.step-checks li{display:flex;gap:9px;align-items:flex-start;color:#45564f;line-height:1.55}.dark .learning-goals li,.dark .step-checks li{color:#c8d7d1}.learning-goals svg,.step-checks svg{flex:none;margin-top:3px;color:#0f766e}.dark .learning-goals svg,.dark .step-checks svg{color:#5eead4}
.section-heading{max-width:760px}.section-heading>p:last-child{margin:10px 0 0;color:#617169}.dark .section-heading>p:last-child{color:#acbeb6}
.workflow-track{display:grid;grid-template-columns:repeat(8,minmax(0,1fr));gap:0;margin:28px 0 0;padding:0;list-style:none}.workflow-track li{position:relative;min-width:0;border-top:2px solid #9ac2b3;padding:14px 7px 0}.workflow-track li::after{position:absolute;top:-5px;left:8px;width:8px;height:8px;border-radius:50%;background:#0f766e;content:""}.workflow-track span{display:block;color:#71817b;font:11px ui-monospace,SFMono-Regular,Menlo,monospace}.workflow-track strong{display:block;margin-top:4px;overflow-wrap:anywhere;font-size:12px;line-height:1.35}
.steps{padding:0 32px}.step{display:grid;grid-template-columns:104px minmax(0,1fr);gap:28px;padding:38px 0;border-bottom:1px solid #d9e3df}.dark .step{border-color:#2c433d}.step:last-child{border-bottom:0}.step-rail{display:flex;flex-direction:column;gap:12px;align-items:flex-start}.step-number{font:700 30px/1 ui-monospace,SFMono-Regular,Menlo,monospace;color:#b0c0b9}.dark .step-number{color:#4f6960}.step-content{min-width:0}.step-content>h2{margin:0 0 12px;font-size:24px;line-height:1.4}.step-content>p{max-width:780px;margin:9px 0;color:#52635c;line-height:1.75}.dark .step-content>p{color:#bdcec7}
.warning{display:flex;gap:10px;align-items:center;margin-top:18px;border-left:3px solid #d97706;background:#fff8e7;padding:12px 14px;color:#8a4b05;font-size:14px}.dark .warning{background:#352711;color:#fde68a}.warning svg{flex:none}
.input-card{overflow:hidden;margin-top:20px;border:1px solid;border-radius:7px}.input-terminal{border-color:#283b35;background:#101916;color:#e8f2ed}.input-prompt{border-color:#9bc8b8;background:#eff9f5;color:#18362e}.dark .input-prompt{border-color:#356455;background:#102a23;color:#dff8ef}.input-card-head{display:flex;align-items:center;justify-content:space-between;gap:12px;min-height:42px;border-bottom:1px solid currentColor;padding:0 8px 0 14px}.input-terminal .input-card-head{border-color:#314840}.input-prompt .input-card-head{border-color:#c2ddd3}.dark .input-prompt .input-card-head{border-color:#2d5448}.input-label{display:flex;align-items:center;gap:8px;font:700 12px ui-monospace,SFMono-Regular,Menlo,monospace}.input-label svg{flex:none}.copy-button{display:inline-flex;align-items:center;gap:6px;border:0;border-radius:5px;background:transparent;padding:7px 8px;color:inherit;font-size:12px;font-weight:700}.copy-button:hover{background:rgba(15,118,110,.12)}.input-terminal .copy-button:hover{background:rgba(255,255,255,.1)}.copy-button:focus-visible,.footer-links a:focus-visible{outline:3px solid #38bdf8;outline-offset:2px}.input-card pre{overflow-x:auto;margin:0;padding:18px;font:13px/1.75 ui-monospace,SFMono-Regular,Menlo,monospace;white-space:pre}.input-card code{font:inherit}
.why{display:grid;grid-template-columns:50px minmax(0,1fr);gap:14px;margin-top:20px;border-left:3px solid #0f766e;padding:4px 0 4px 15px}.why>span{color:#0f766e;font:700 12px ui-monospace,SFMono-Regular,Menlo,monospace}.dark .why>span{color:#5eead4}.why strong{font-size:14px}.why p{margin:5px 0 0;color:#5d6d66;line-height:1.65}.dark .why p{color:#aec0b8}.step-checks{display:flex;flex-wrap:wrap;gap:10px 18px;margin:20px 0 0;padding:0;list-style:none}.step-checks li{font-size:13px}
.acceptance-loop{background:#f6f9f8}.dark .acceptance-loop{background:#101f1b}.acceptance-loop ol{display:flex;flex-wrap:wrap;align-items:center;gap:7px;margin:24px 0 0;padding:0;list-style:none}.acceptance-loop li{display:flex;align-items:center;gap:7px}.acceptance-loop li>span{border:1px solid #c6d5cf;border-radius:5px;background:#fff;padding:8px 10px;font-size:12px;font-weight:700}.dark .acceptance-loop li>span{border-color:#355047;background:#152820}.acceptance-loop .human-node>span{border-color:#9eb8e7;background:#eef4ff;color:#1d4ed8}.dark .acceptance-loop .human-node>span{border-color:#31548c;background:#172c50;color:#bfdbfe}.acceptance-loop svg{color:#81928b}
.exercise{background:#edf8f4}.dark .exercise{background:#102b23}.exercise-heading{display:flex;align-items:flex-start;justify-content:space-between;gap:20px}.exercise>p{max-width:780px;color:#4f6159;line-height:1.7}.dark .exercise>p{color:#bed0c8}.reflection{margin-top:20px;border-top:1px solid #bfd7ce;padding-top:18px}.dark .reflection{border-color:#315348}.reflection p{margin:5px 0 0;color:#52635c}.dark .reflection p{color:#b8c9c1}
.tutorial-footer{display:flex;align-items:center;justify-content:space-between;gap:24px;border-bottom:0}.tutorial-footer strong{font-size:14px}.tutorial-footer p{margin:4px 0 0;color:#687972;font-size:13px}.dark .tutorial-footer p{color:#9fb1a9}.tutorial-footer code{font-family:ui-monospace,SFMono-Regular,Menlo,monospace}.footer-links{display:flex;flex-wrap:wrap;justify-content:flex-end;gap:10px}.footer-links a{border:1px solid #9db7ad;border-radius:5px;padding:9px 11px;color:#0f766e;font-size:13px;font-weight:700;text-decoration:none}.dark .footer-links a{border-color:#3f6658;color:#5eead4}.copy-status{position:fixed;right:18px;bottom:18px;z-index:40;max-width:min(420px,calc(100vw - 36px));margin:0;border-radius:6px;background:#13251f;padding:10px 14px;color:#fff;font-size:12px;box-shadow:0 12px 28px rgba(0,0,0,.22)}.copy-status:empty{display:none}
@media(max-width:900px){.hero{grid-template-columns:1fr}.responsibility{display:grid;grid-template-columns:1fr 1fr;border-top:1px solid #bfcfc8;border-left:0;padding-top:8px;padding-left:0}.dark .responsibility{border-color:#395149}.responsibility div+div{border-top:0;border-left:1px solid #d4dfda;padding-left:20px}.dark .responsibility div+div{border-color:#30483f}.workflow-track{grid-template-columns:repeat(4,minmax(0,1fr));row-gap:22px}}
@media(max-width:640px){.breadcrumb{padding:14px 20px 0}.hero,.article-guide,.learning-goals,.workflow,.acceptance-loop,.exercise,.tutorial-footer{padding:26px 20px}.hero{gap:24px}.hero h1{font-size:29px}.hero-summary{font-size:15px}.responsibility{grid-template-columns:1fr}.responsibility div+div{border-top:1px solid #d4dfda;border-left:0;padding-left:0}.dark .responsibility div+div{border-color:#30483f}.article-guide{grid-template-columns:1fr;gap:22px}.article-guide ol{grid-template-columns:1fr}.learning-goals{grid-template-columns:1fr;gap:20px}.learning-goals ul{grid-template-columns:1fr}.workflow-track{grid-template-columns:repeat(2,minmax(0,1fr))}.steps{padding:0 20px}.step{grid-template-columns:1fr;gap:16px;padding:30px 0}.step-rail{flex-direction:row;align-items:center}.step-content>h2{font-size:21px}.input-card pre{padding:15px;font-size:12px}.why{grid-template-columns:1fr;gap:5px}.exercise-heading,.tutorial-footer{align-items:flex-start;flex-direction:column}.footer-links{justify-content:flex-start}.acceptance-loop li>span{padding:7px 8px}}
</style>
