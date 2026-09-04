/**
 * Campaign attribution helpers: first-touch UTM capture and promo-code carry-over.
 * Everything stays in the visitor's browser until they register.
 */

const UTM_KEYS = ['utm_source', 'utm_medium', 'utm_campaign', 'utm_content', 'utm_term'] as const
export type UtmKey = (typeof UTM_KEYS)[number]
export type CampaignAttribution = Partial<Record<UtmKey, string>> & { landing_path?: string; captured_at?: string }

const UTM_STORAGE_KEY = 'sub2api_campaign'
const PROMO_STORAGE_KEY = 'sub2api_pending_promo'
const MAX_LEN = 120

function clean(value: unknown): string {
  return typeof value === 'string' ? value.trim().slice(0, MAX_LEN) : ''
}

function safeGet(storage: Storage | undefined, key: string): string | null {
  try {
    return storage?.getItem(key) ?? null
  } catch {
    return null
  }
}
function safeSet(storage: Storage | undefined, key: string, value: string): void {
  try {
    storage?.setItem(key, value)
  } catch {
    /* storage unavailable */
  }
}
function safeRemove(storage: Storage | undefined, key: string): void {
  try {
    storage?.removeItem(key)
  } catch {
    /* storage unavailable */
  }
}

function local(): Storage | undefined {
  return typeof window !== 'undefined' ? window.localStorage : undefined
}
function session(): Storage | undefined {
  return typeof window !== 'undefined' ? window.sessionStorage : undefined
}

/**
 * Record UTM parameters from a landing URL (first touch wins) and stash a promo code
 * so it can be pre-filled on the registration form later.
 */
export function captureCampaignParams(query: Record<string, unknown> | undefined | null, path = ''): void {
  if (!query || typeof query !== 'object') return
  const promo = clean(query.promo ?? query.promo_code)
  if (promo) safeSet(session(), PROMO_STORAGE_KEY, promo)

  const utm: CampaignAttribution = {}
  for (const key of UTM_KEYS) {
    const value = clean(query[key])
    if (value) utm[key] = value
  }
  if (Object.keys(utm).length === 0) return
  if (safeGet(local(), UTM_STORAGE_KEY)) return // first touch only
  utm.landing_path = path.slice(0, MAX_LEN)
  utm.captured_at = new Date().toISOString()
  safeSet(local(), UTM_STORAGE_KEY, JSON.stringify(utm))
}

export function getStoredCampaign(): CampaignAttribution | null {
  const raw = safeGet(local(), UTM_STORAGE_KEY)
  if (!raw) return null
  try {
    const parsed = JSON.parse(raw) as CampaignAttribution
    return parsed && typeof parsed === 'object' ? parsed : null
  } catch {
    return null
  }
}

export function consumePendingPromo(): string {
  const value = safeGet(session(), PROMO_STORAGE_KEY) ?? ''
  if (value) safeRemove(session(), PROMO_STORAGE_KEY)
  return value
}

export function peekPendingPromo(): string {
  return safeGet(session(), PROMO_STORAGE_KEY) ?? ''
}

/** Fields appended to the registration request so the backend can persist attribution. */
export function campaignRegisterFields(): Record<string, string> {
  const stored = getStoredCampaign()
  if (!stored) return {}
  const out: Record<string, string> = {}
  for (const key of UTM_KEYS) {
    if (stored[key]) out[key] = stored[key] as string
  }
  if (stored.landing_path) out.landing_path = stored.landing_path
  return out
}
