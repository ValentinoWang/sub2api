import { apiClient } from '../client'

export type CostTier = 'plus' | 'pro5x' | 'pro20x'
export type CostEvidence = 'observed' | 'assumption' | 'synthetic'
export interface CostPlan {
  id: CostTier
  label: string
  reference_monthly_usd: string
  advertised_usage_multiplier: string
  new_purchase_status: string
  renewal_status: string
  measured_weekly_capacity: string | null
}
export interface CostCatalog {
  version: string
  checked_at: string
  valid_until: string
  plans: CostPlan[]
}
export interface CostComparisonInput {
  currency: 'USD' | 'CNY' | 'HKD' | 'SGD' | 'EUR'
  unit: string
  model: string
  period_start: string
  period_end: string
  as_of: string
  purpose: 'existing' | 'new_purchase'
  demand: string
  rows: Array<{
    tier: CostTier
    period_cost: string | null
    period_capacity: string | null
    useful_fraction: string | null
    workload_verified: boolean
    evidence: CostEvidence
  }>
}
export interface CostComparisonResult {
  status: string
  currency: string
  unit: string
  lowest_cost_feasible_tiers: CostTier[]
  rows: Array<{
    tier: CostTier
    label: string
    period_cost: string | null
    effective_capacity: string | null
    delivered: string | null
    unmet: string | null
    unit_cost: string | null
    eligible: boolean
    reasons: string[]
    evidence: CostEvidence
  }>
  boundaries: string[]
}
export async function getCostCatalog(): Promise<CostCatalog> {
  const { data } = await apiClient.get<CostCatalog>('/admin/cost-center/catalog')
  return data
}
export async function compareCostTiers(input: CostComparisonInput): Promise<CostComparisonResult> {
  const { data } = await apiClient.post<CostComparisonResult>('/admin/cost-center/compare', input)
  return data
}
