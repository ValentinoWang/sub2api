/**
 * Brand constants for the rest2build fork.
 *
 * The meme: 人去 rest，AI 去 build —— you sleep, the agent ships.
 * `.lol` is part of the joke, so the domain is shown verbatim on the landing page.
 */

export const BRAND_NAME = 'rest2build'
export const BRAND_DOMAIN = 'ai.rest2build.lol'
export const BRAND_SERVICE_DESCRIPTION = 'rest2build 提供面向 Codex、Claude Code 等工具的 AI 模型接入与账号充值服务。公益 Skills 帮你完成配置与重复操作，AI 使用经验帮助定位和恢复问题，Harness 工程将任务拆解、复核与验收融入团队开发流程。'

/** Public-facing lab name used on marketplace listings and the verify page. */
export const BRAND_LAB_NAME = 'Rest2Build AI 接入实验室'
/** The one and only Xianyu (闲鱼) store name. Shown on /verify/xianyu to stop impersonation. */
export const XIANYU_STORE_NAME = 'Rest2Build AI 接入实验室'

/** Public, crawlable pages. Keep in sync with public/sitemap.xml and the router. */
export const PUBLIC_PAGES = {
  home: '/home',
  models: '/model-plaza',
  codex: '/codex-cli',
  claudeCode: '/claude-code',
  openaiCompat: '/openai-compatible-api',
  experiences: '/experiences',
  wslCodexTutorial: '/experiences/windows-11-wsl-codex-frontend',
  wslCodexTroubleshooting: '/error-experiences/windows-11-wsl2-codex-environment',
  errorExperience: '/error-experiences/gpt-6-astra-not-visible',
  publicBenefit: '/public-benefit',
  business: '/business-invoice',
  security: '/security',
  benchmarks: '/benchmarks',
  verify: '/verify/xianyu',
  share: '/share',
  status: '/status',
  keyUsage: '/key-usage'
} as const

/**
 * Upstream project default site name. The backend still ships this as the default
 * `site_name`, so when the admin has not customised the name we treat it as "unset"
 * and show the fork brand instead. Any other admin-provided name is respected as-is.
 */
export const UPSTREAM_DEFAULT_SITE_NAME = 'Sub2API'

export function resolveBrandName(raw?: string | null): string {
  const name = typeof raw === 'string' ? raw.trim() : ''
  if (!name || name === UPSTREAM_DEFAULT_SITE_NAME) return BRAND_NAME
  return name
}

export interface WordmarkParts {
  left: string
  right: string
}

/**
 * Split names shaped like `rest2build` / `Sub2API` into the two halves around the "2",
 * so the wordmark can style "rest" and "build" differently. Returns null when the
 * name does not follow that shape (it is then rendered as plain text).
 */
/** Admin-configured store name wins; the constant is the fallback. */
export function resolveStoreName(raw?: string | null): string {
  const name = typeof raw === 'string' ? raw.trim() : ''
  return name || XIANYU_STORE_NAME
}

export function splitWordmark(name: string): WordmarkParts | null {
  const match = /^([A-Za-z]+)2([A-Za-z]+)$/.exec(name.trim())
  if (!match) return null
  return { left: match[1], right: match[2] }
}
