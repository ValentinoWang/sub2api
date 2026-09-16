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


export interface CostPurchase {
  reference: string
  supplier: string
  asset: string
  tier: CostTier | 'unallocated'
  kind: 'subscription' | 'server' | 'traffic' | 'labor' | 'other'
  currency: string
  amount: string
  paid_at: string
  service_start: string
  service_end: string
  evidence: 'invoice' | 'manual' | 'synthetic'
  evidence_ref: string
}
export interface CostDelivery {
  reference: string; asset: string; tier: CostTier; model: string; unit: string
  quantity: string; period_start: string; period_end: string
  evidence: 'invoice' | 'manual' | 'synthetic'
}
export type LedgerCommand =
  | { kind: 'purchase'; purchase: CostPurchase }
  | { kind: 'delivery'; delivery: CostDelivery }
  | { kind: 'allocate'; allocation: { purchase_id: string; parts: Array<{ tier: CostTier; weight: string }> } }
  | { kind: 'void'; void: { target_id: string; expected_hash: string; reason: string } }
export interface LedgerEvent {
  id: string; sequence: number; idempotency_key: string; request_hash: string
  actor_id: number; recorded_at: string; command: LedgerCommand
}
export interface LedgerSummary {
  currency: string; start: string; end: string; as_of: string; status: string; warnings: string[]
  rows: Array<{ tier: string; cash_paid: string; period_expense: string; recognized_expense: string
    remaining_service_value: string; delivered: string; recorded_scope_unit_cost: string | null }>
}
export async function getCostLedgerHealth(): Promise<{ status: string; event_count: number }> {
  const { data } = await apiClient.get('/admin/cost-center/ledger/health'); return data
}
export async function listCostLedgerEvents(after = 0): Promise<{ items: LedgerEvent[]; next_after: number | null; total: number }> {
  const { data } = await apiClient.get('/admin/cost-center/ledger/events', { params: { after, limit: 50 } }); return data
}
export async function appendCostLedger(command: LedgerCommand, idempotencyKey: string): Promise<{ event: LedgerEvent; replayed: boolean }> {
  const { data } = await apiClient.post('/admin/cost-center/ledger/commands', command, { headers: { 'Idempotency-Key': idempotencyKey } }); return data
}
export async function getCostLedgerSummary(params: Record<string, string>): Promise<LedgerSummary> {
  const { data } = await apiClient.get('/admin/cost-center/ledger/summary', { params }); return data
}
