import { apiClient } from '../client'
import type { CostTier, LedgerEvent } from './cost-center'

export interface CreditLotInput { reference: string; pool: string; unit: string; currency: string; amount: string; quantity: string; paid_at: string; expires_at: string; evidence: string }
export interface CreditUseInput { reference: string; pool: string; unit: string; currency: string; tier: CostTier; model: string; quantity: string; occurred_at: string; evidence: string }
export interface AccountIntervalInput { asset: string; account_id: number; tier: CostTier; start: string; end: string; evidence_ref: string }
export type AnalysisKind = 'traffic' | 'replay' | 'forecast'
export type NativeCommand =
  | { kind: 'credit_lot'; credit_lot: CreditLotInput }
  | { kind: 'credit_use'; credit_use: CreditUseInput }
  | { kind: 'account_interval'; account_interval: AccountIntervalInput }
  | { kind: 'traffic'; traffic: Record<string, unknown> }
  | { kind: 'replay'; replay: Record<string, unknown> }
  | { kind: 'forecast'; forecast: Record<string, unknown> }
export interface SourceAccount { id: number; platform: string; type: string; status: string; plan_type: string; subscription_expires_at: string | null }
export interface Reconciliation { accounts: SourceAccount[]; source_latest_usage_at: string | null; coverage_complete: boolean; status: string; rows: Array<{ account_id: number; asset: string; tier: string; model: string; requests: number; input_tokens: string; output_tokens: string; cache_read_tokens: string; cache_creation_tokens: string; reference_cost: string; recorded_requests: string | null; request_difference: string | null; reconciliation_status: string; start: string; end: string }>; warnings: string[] }
export interface PrepaidReport {
 cash_paid: string
 uses: Array<{ event_id: string; tier: string; cost: string; parts: Array<{ lot_id: string; quantity: string; cost: string }> }>
 balances: Array<{ event_id: string; pool: string; unit: string; remaining_quantity: string; remaining_value: string; expired_quantity: string; expired_value: string }>
}
export async function getCostSourceAccounts(): Promise<SourceAccount[]> { const { data } = await apiClient.get('/admin/cost-center/source/accounts'); return data.items }
export async function reconcileCostUsage(params: Record<string, string>): Promise<Reconciliation> { const { data } = await apiClient.get('/admin/cost-center/source/reconciliation', { params }); return data }
export async function appendNativeCost(command: NativeCommand, key: string): Promise<{ event: LedgerEvent; replayed: boolean }> { const { data } = await apiClient.post('/admin/cost-center/ledger/commands', command, { headers: { 'Idempotency-Key': key } }); return data }
export async function analyzeNativeCost(kind: AnalysisKind, input: Record<string, unknown>): Promise<Record<string, unknown>> { const { data } = await apiClient.post(`/admin/cost-center/analysis/${kind}`, input); return data }

export interface RadarReference {
 source: string; retrieved_at: string; source_updated_label: string; status: string; speed_model_label: string
 quotas: Array<{ tier: string; model_label: string; reference_usd: string; basis: string; period: string | null; price_version: string | null }>
 speeds: Array<{ effort: string; standard_tps: string; fast_tps: string }>
 seven_day_average: string | null; two_month_reset_rate: string | null; warnings: string[]
}
export async function getRadarReference(): Promise<RadarReference> { const { data } = await apiClient.get('/admin/cost-center/reference/radar'); return data }

export interface SourceSyncResult { id: string; account_count: number; request_count: number; stored_request_count: number; status: string; observed_at: string; report: Reconciliation }
export async function syncCostSource(input: Record<string,string>): Promise<SourceSyncResult> { const { data } = await apiClient.post('/admin/cost-center/source/sync', input); return data }

export interface NetworkSample { source: string; interface: string; adapter: string; scope: string; observed_at: string; epoch: string; rx_bytes: string; tx_bytes: string; billing_interface_verified: boolean }
export async function getCostTrafficSamples(): Promise<NetworkSample[]> { const { data } = await apiClient.get('/admin/cost-center/source/traffic'); return data.items }
