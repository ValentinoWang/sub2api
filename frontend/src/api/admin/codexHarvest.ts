/**
 * Admin Codex ticket harvesting API: proxy pool, speed controls, per-proxy
 * learning, the event log and bounded manual runs. Responses never carry
 * ticket state or proxy credentials.
 */

import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'

export type CodexHarvestPreset = 'slow' | 'standard' | 'fast' | 'burst' | 'custom'

export interface CodexHarvestSpeed {
  round_interval_seconds: number
  probe_interval_seconds: number
  attempt_timeout_seconds: number
  cooldown_seconds: number
  max_requests_per_round: number
  max_proxy_attempts: number
  max_requests_per_account_hour: number
}

export type CodexHarvestSpeedField = keyof CodexHarvestSpeed

export interface CodexHarvestControls {
  version: number
  preset: CodexHarvestPreset
  speed: CodexHarvestSpeed
  proxy_ids: number[]
}

export interface CodexHarvestBound {
  min: number
  max: number
}

export interface CodexHarvestPoolMember {
  proxy_id: number
  name?: string
  usable: boolean
  reason?: 'deleted' | 'inactive' | 'expired' | string
}

export interface CodexHarvestRuntime {
  running: boolean
  next_round_at?: string
  last_round_at?: string
  requests_used: number
  request_budget: number
  current_proxy?: string
  selection_reason?: string
  idle_reason?: string
}

export interface CodexHarvestTicketView {
  model: string
  ready: boolean
  blocked: boolean
  length?: number
  remaining_seconds: number
  age_seconds?: number
  proxy_name?: string
  cooldown_until?: string
}

export interface CodexHarvestManualRun {
  account_id: number
  models: string[]
  running: boolean
  started_at: string
  finished_at?: string
  attempts: number
  harvested: string[]
  result?: string
}

export interface CodexHarvestAccountView {
  id: number
  name: string
  status: string
  schedulable: boolean
  hour_used: number
  hour_limit: number
  tickets: CodexHarvestTicketView[]
  manual?: CodexHarvestManualRun
}

export interface CodexHarvestFlowEvent {
  id: string
  at: string
  stage: string
  kind: string
  account_id?: number
  account_name?: string
  model?: string
  proxy_id?: number
  proxy_name?: string
  http_status?: number
  length?: number
  accepted?: boolean
  manual?: boolean
  result?: string
  detail?: string
}

export interface CodexHarvestSnapshot {
  generated_at: string
  enabled: boolean
  fail_closed: boolean
  models: string[]
  target_length: number
  controls: CodexHarvestControls
  configured: boolean
  settings_error?: string
  presets: Record<string, CodexHarvestSpeed>
  preset_order: string[]
  bounds: Record<CodexHarvestSpeedField, CodexHarvestBound>
  pool: CodexHarvestPoolMember[]
  pool_error?: string
  runtime: CodexHarvestRuntime
  accounts: CodexHarvestAccountView[]
  events: CodexHarvestFlowEvent[]
}

export interface CodexHarvestNodeRecord {
  id: number
  proxy_id: number
  proxy_name: string
  account_id: number
  account_name?: string
  model: string
  successes: number
  misses: number
  network_errors: number
  account_errors: number
  consecutive_failures: number
  last_success: string | null
  cooldown_until: string | null
  latency_ms: number
  last_result: string
  updated_at: string
}

export interface CodexHarvestManualRequest {
  models?: string[]
  max_attempts?: number
  interval_seconds?: number
}

export async function getSnapshot(): Promise<CodexHarvestSnapshot> {
  const { data } = await apiClient.get<CodexHarvestSnapshot>('/admin/codex-harvest')
  return data
}

export async function updateControls(controls: CodexHarvestControls): Promise<CodexHarvestControls> {
  const { data } = await apiClient.put<CodexHarvestControls>('/admin/codex-harvest/controls', controls)
  return data
}

export async function listNodes(page = 1, pageSize = 20): Promise<PaginatedResponse<CodexHarvestNodeRecord>> {
  const { data } = await apiClient.get<PaginatedResponse<CodexHarvestNodeRecord>>('/admin/codex-harvest/nodes', {
    params: { page, page_size: pageSize }
  })
  return data
}

export async function resetNodes(recordId = 0): Promise<void> {
  await apiClient.post('/admin/codex-harvest/nodes/reset', { record_id: recordId })
}

export async function startManual(accountId: number, request: CodexHarvestManualRequest = {}): Promise<CodexHarvestManualRun> {
  const { data } = await apiClient.post<CodexHarvestManualRun>(`/admin/codex-harvest/accounts/${accountId}/manual`, request)
  return data
}

export const codexHarvestAPI = {
  getSnapshot,
  updateControls,
  listNodes,
  resetNodes,
  startManual
}

export default codexHarvestAPI
