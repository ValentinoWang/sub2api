/**
 * Brand constants for the rest2build fork.
 *
 * The meme: 人去 rest，AI 去 build —— you sleep, the agent ships.
 * `.lol` is part of the joke, so the domain is shown verbatim on the landing page.
 */

export const BRAND_NAME = 'rest2build'
export const BRAND_DOMAIN = 'ai.rest2build.lol'

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
export function splitWordmark(name: string): WordmarkParts | null {
  const match = /^([A-Za-z]+)2([A-Za-z]+)$/.exec(name.trim())
  if (!match) return null
  return { left: match[1], right: match[2] }
}
