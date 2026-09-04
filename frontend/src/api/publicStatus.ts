import { apiClient } from './client'

export interface PublicStatusItem {
  name: string
  provider: string
  model: string
  status: 'operational' | 'degraded' | 'failed' | 'error' | 'unknown'
  samples: number
  success_rate?: number
  latency_p50_ms?: number
  latency_p95_ms?: number
  window_start?: string
  last_checked_at?: string
}

export interface PublicStatusSummary {
  enabled: boolean
  generated_at?: string
  overall?: 'operational' | 'degraded' | 'failed' | 'unknown'
  items: PublicStatusItem[]
}

/** Unauthenticated aggregated monitor summary (gated by the admin setting). */
export async function getPublicStatus(): Promise<PublicStatusSummary> {
  const { data } = await apiClient.get<PublicStatusSummary>('/public/status')
  return data
}
