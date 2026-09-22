// Render the public Markdown and standalone HTML from the website's shared copy.
import { readFileSync, writeFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { marked } from '../../frontend/node_modules/marked/lib/marked.esm.js'
import { formatTutorialPrompt } from '../../frontend/src/utils/tutorialPrompt.mjs'

const root = new URL('../../', import.meta.url)
const guide = JSON.parse(readFileSync(new URL('frontend/src/content/codexModelCatalogGuide.json', root), 'utf8'))
const esc = (text) => text.replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;').replaceAll('"', '&quot;')
const localLinks = (text) => text.replaceAll('/downloads/sync-codex-model-catalog.py', '../../frontend/public/downloads/sync-codex-model-catalog.py').replaceAll('/error-experiences/gpt-6-astra-not-visible', '2026-09-06-codex-astra-visibility.md')
const brand = 'rest2build 提供面向 Codex、Claude Code 等工具的 AI 模型接入服务。同时围绕公益 Skills、AI 使用经验分享与 Harness 工程，持续开展内容与实践。'
const md = `# ERR-006 · ${guide.title}

更新：${guide.updatedAt} · 适用：${guide.applicableTo}

## 问题说明

${localLinks(guide.problem)}

## 解决方案

### 让 Codex 帮你处理

把完整提示词发给能操作你这台电脑的 Codex。

\`\`\`text
${localLinks(guide.prompt)}
\`\`\`

${localLinks(guide.solution)}

## 原因、验证与注意事项

${localLinks(guide.notes)}

---

rest2build

歇一会儿，让 AI 接着干。

rest 是你的，build 交给 AI。

${brand}

[ai.rest2build.lol](https://ai.rest2build.lol/)
`
const vue = readFileSync(new URL('frontend/src/views/public/CodexModelCatalogExperienceView.vue', root), 'utf8')
const css = vue.match(/<style scoped>([\s\S]*?)<\/style>/)[1].replaceAll(/:deep\(([^)]+)\)/g, '$1')
const html = `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>${esc(guide.title)} · rest2build</title><style>
*{box-sizing:border-box}body{margin:0;background:#f6f8f8;font:15px/1.8 -apple-system,BlinkMacSystemFont,"PingFang SC",sans-serif}main{width:min(960px,calc(100% - 32px));margin:30px auto}a{color:#0f766e}button,textarea{font:inherit}footer{padding:64px 24px;text-align:center}footer img{width:42px;height:42px}footer .brand{display:inline-flex;align-items:center;gap:12px;font-size:30px;font-weight:800;text-decoration:none}footer .tagline{font-size:32px;font-weight:800}footer .service{max-width:720px;margin:16px auto;color:#52615c}@media(max-width:640px){footer .tagline{font-size:27px}}
${css}</style></head><body><main><article class="catalog-experience"><nav class="breadcrumb"><a href="index.html">返回经验分享</a></nav><header class="intro"><p class="kicker">Codex 使用错误说明 / ERR-006</p><h1>${esc(guide.title)}</h1><p class="meta">适用：${esc(guide.applicableTo)} · 更新：${guide.updatedAt}</p></header>
<section class="content-section"><p class="step">01</p><h2>问题说明</h2><div class="markdown-body">${marked.parse(localLinks(guide.problem))}</div></section>
<section class="content-section solution"><p class="step">02</p><h2>解决方案</h2><div class="prompt-heading"><h3>让 Codex 帮你处理</h3><button type="button" class="copy-button">复制提示词</button></div><p class="prompt-download"><a href="../../frontend/public/downloads/sync-codex-model-catalog.py" download>下载目录同步脚本（Python 3.11+）</a></p><textarea id="prompt" class="prompt-field" readonly aria-label="ERR-006 完整排障与配置提示词">${esc(localLinks(guide.prompt))}</textarea><p class="copy-status" role="status" aria-live="polite"></p><div class="markdown-body">${marked.parse(localLinks(guide.solution))}</div></section>
<section class="content-section"><p class="step">03</p><h2>原因、验证与注意事项</h2><div class="markdown-body">${marked.parse(localLinks(guide.notes))}</div></section></article></main>
<footer><a class="brand" href="https://ai.rest2build.lol/"><img src="../../frontend/public/logo.svg" alt=""><span>rest2build</span></a><p class="tagline">歇一会儿，让 AI 接着干。</p><p>rest 是你的，build 交给 AI。</p><p class="service">${brand}</p><a href="https://ai.rest2build.lol/">ai.rest2build.lol</a></footer>
<script>${formatTutorialPrompt.toString()}
const promptField=document.getElementById('prompt');promptField.value=formatTutorialPrompt(promptField.value,location.href);
document.querySelector('.copy-button').addEventListener('click',async()=>{const field=document.getElementById('prompt'),status=document.querySelector('.copy-status');try{await navigator.clipboard.writeText(field.value);status.textContent='提示词已复制';document.querySelector('.copy-button').textContent='已复制'}catch{field.focus();field.select();status.textContent='自动复制未成功，提示词已选中，可手动复制'}})</script></body></html>`
for (const [extension, content] of [['md', md], ['html', html]]) {
  const path = new URL(`docs/error-experiences/2026-09-15-codex-model-catalog-context-window.${extension}`, root)
  if (process.argv.includes('--check')) {
    if (readFileSync(path, 'utf8') !== content) throw new Error(`Generated article differs: ${fileURLToPath(path)}`)
  } else writeFileSync(path, content)
}
console.log(process.argv.includes('--check') ? 'Article copies match.' : 'Markdown and HTML updated.')
