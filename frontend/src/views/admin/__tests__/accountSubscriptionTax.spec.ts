import { describe, expect, it } from 'vitest'
import {
  NO_STATE_GENERAL_SALES_TAX_CODES,
  US_STATES,
  detectStateTaxStatus,
  getTaxCaveatCode,
  isNoStateGeneralSalesTaxState
} from '../accountSubscriptionTax'

describe('account subscription state tax detection', () => {
  it('contains the five states without a general statewide sales tax', () => {
    expect(NO_STATE_GENERAL_SALES_TAX_CODES).toEqual(['AK', 'DE', 'MT', 'NH', 'OR'])
    expect(US_STATES).toHaveLength(50)
  })

  it('normalizes state codes before detection', () => {
    expect(isNoStateGeneralSalesTaxState(' or ')).toBe(true)
    expect(detectStateTaxStatus('DE')).toBe('no-state-general-sales-tax')
    expect(detectStateTaxStatus('CA')).toBe('state-general-sales-tax')
    expect(detectStateTaxStatus('')).toBe('unknown')
  })

  it('returns caveat codes for all listed states', () => {
    for (const code of NO_STATE_GENERAL_SALES_TAX_CODES) {
      expect(getTaxCaveatCode(code)).toBe(code)
    }
    expect(getTaxCaveatCode('CA')).toBeNull()
  })
})
