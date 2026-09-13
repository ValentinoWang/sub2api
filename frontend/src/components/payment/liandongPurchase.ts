import type { LiandongRechargeProduct, PublicSettings } from '@/types'
import { sanitizeUrl } from '@/utils/url'

function buyerUrl(value: string): string {
  const url = sanitizeUrl(value)
  if (!url) return ''
  const parsed = new URL(url)
  return parsed.username || parsed.password ? '' : url
}

export function resolveLiandongPurchaseUrl(settings: PublicSettings | null | undefined): string {
  if (!settings?.purchase_subscription_enabled) return ''
  return buyerUrl(settings.purchase_subscription_url || '')
}

export function resolveLiandongProducts(catalog: LiandongRechargeProduct[]): LiandongRechargeProduct[] {
  return catalog.flatMap(product => {
    const external_url = buyerUrl(product.external_url || '')
    if (!Number.isSafeInteger(product.goods_id) || product.goods_id <= 0
      || !Number.isFinite(product.cny_amount) || product.cny_amount <= 0
      || !Number.isFinite(product.usd_credit) || product.usd_credit <= 0 || !external_url) return []
    return [{ ...product, external_url }]
  })
}
