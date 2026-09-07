import { chromium } from '/Applications/ChatGPT.app/Contents/Resources/cua_node/lib/node_modules/playwright/index.mjs'
import fs from 'node:fs/promises'
import path from 'node:path'

const baseUrl = process.env.BASE_URL || 'http://127.0.0.1:4173'
const output = path.dirname(new URL(import.meta.url).pathname)
const browser = await chromium.launch({
  executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
  headless: true,
})
const cases = [
  { name: 'docs-desktop', route: '/docs', viewport: { width: 1440, height: 900 }, text: '部署、接入与本地工具' },
  { name: 'memory-desktop', route: '/docs/codex-memory', viewport: { width: 1440, height: 900 }, text: '统一本机 Codex 记忆' },
  { name: 'memory-mobile', route: '/docs/codex-memory', viewport: { width: 390, height: 844 }, text: '统一本机 Codex 记忆' },
  { name: 'login-mobile', route: '/login', viewport: { width: 390, height: 844 }, text: 'Docs' },
]
const results = []
try {
  for (const item of cases) {
    const page = await browser.newPage({ viewport: item.viewport })
    const pageErrors = []
    page.on('pageerror', (error) => pageErrors.push(error.message))
    const response = await page.goto(`${baseUrl}${item.route}`, { waitUntil: 'domcontentloaded' })
    await page.getByText(item.text, { exact: false }).first().waitFor({ state: 'visible' })
    const dimensions = await page.evaluate(() => ({
      scrollWidth: document.documentElement.scrollWidth,
      clientWidth: document.documentElement.clientWidth,
    }))
    if (dimensions.scrollWidth > dimensions.clientWidth + 1) {
      throw new Error(`${item.name} has horizontal overflow: ${JSON.stringify(dimensions)}`)
    }
    if (item.name.startsWith('memory')) {
      await page.getByText('当前部署尚未提供经过校验的 GitHub Release 清单').waitFor({ state: 'visible' })
      await page.getByRole('heading', { name: '完整边界说明' }).waitFor({ state: 'visible' })
    }
    if (item.name === 'login-mobile') {
      const href = await page.getByRole('link', { name: 'Docs' }).getAttribute('href')
      if (href !== '/docs') throw new Error(`login Docs link is ${href}`)
    }
    const screenshot = path.join(output, `${item.name}.png`)
    await page.screenshot({ path: screenshot, fullPage: true })
    results.push({
      name: item.name,
      route: item.route,
      status: response?.status(),
      viewport: item.viewport,
      dimensions,
      pageErrors,
      screenshot: path.basename(screenshot),
    })
    await page.close()
  }
} finally {
  await browser.close()
}
const report = { status: 'passed', baseUrl, cases: results }
await fs.writeFile(path.join(output, 'visual-acceptance.json'), `${JSON.stringify(report, null, 2)}\n`)
console.log(JSON.stringify(report))
