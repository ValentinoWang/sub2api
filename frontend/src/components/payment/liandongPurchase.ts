import type { PublicSettings } from '@/types'
import { sanitizeUrl } from '@/utils/url'

export function resolveLiandongPurchaseUrl(settings: PublicSettings | null | undefined): string {
  if (!settings?.purchase_subscription_enabled) return ''

  const url = sanitizeUrl(settings.purchase_subscription_url || '')
  if (!url) return ''

  const parsed = new URL(url)
  return parsed.username || parsed.password ? '' : url
}
