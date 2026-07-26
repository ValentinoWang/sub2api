export interface LiandongBalanceProduct {
  cnyAmount: 1 | 5 | 20 | 50 | 100 | 500
  usdCredit: number
  productUrl: string
}

const PRODUCT_DEFINITIONS = Object.freeze([
  { cnyAmount: 1, usdCredit: 0.14, envKey: 'VITE_LIANDONG_SUB2API_CNY_1_URL' },
  { cnyAmount: 5, usdCredit: 0.69, envKey: 'VITE_LIANDONG_SUB2API_CNY_5_URL' },
  { cnyAmount: 20, usdCredit: 2.78, envKey: 'VITE_LIANDONG_SUB2API_CNY_20_URL' },
  { cnyAmount: 50, usdCredit: 6.94, envKey: 'VITE_LIANDONG_SUB2API_CNY_50_URL' },
  { cnyAmount: 100, usdCredit: 13.89, envKey: 'VITE_LIANDONG_SUB2API_CNY_100_URL' },
  { cnyAmount: 500, usdCredit: 69.44, envKey: 'VITE_LIANDONG_SUB2API_CNY_500_URL' },
] as const)

function readProductUrl(envKey: string): string {
  const value = import.meta.env[envKey]?.trim() ?? ''
  if (!value) return ''

  const parsed = new URL(value)
  if (parsed.protocol !== 'https:' || !['www.ldxp.cn', 'pay.ldxp.cn'].includes(parsed.hostname)) {
    throw new Error(`${envKey} must be an HTTPS ldxp.cn product URL`)
  }
  return parsed.toString()
}

export const LIANDONG_BALANCE_PRODUCTS: readonly LiandongBalanceProduct[] = Object.freeze(
  PRODUCT_DEFINITIONS.map(({ cnyAmount, usdCredit, envKey }) => Object.freeze({
    cnyAmount,
    usdCredit,
    productUrl: readProductUrl(envKey),
  }))
)
