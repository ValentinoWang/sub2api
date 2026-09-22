import { apiClient } from './client'
import type { LiandongRechargeProduct } from '@/types'

export async function getLiandongProducts(): Promise<LiandongRechargeProduct[]> {
  return (await apiClient.get<{ products: LiandongRechargeProduct[] }>('/ldxp/products')).data.products
}

export interface BrowserRestockProduct {
  goods_id: number
  cny_amount: number
  usd_credit: number
  external_url: string
  title?: string
  target_stock: number
  batch_size: number
  enabled: boolean
  current_stock?: number
  identity_verified?: boolean
  inventory_at?: string
}
export interface BrowserRestockDevice {
  id: string
  name: string
  goods_ids?: number[]
  revoked: boolean
  paused_reason?: string
  last_seen_at?: string
  authorization_verified_at?: string
  runtime?: {
    execution_mode?: 'http' | 'browser'
    state: 'running' | 'paused'
    reason: string
    browser_verification_required: boolean
    checked_at?: string
    next_check_at?: string
    reported_at: string
  }
  recheck?: BrowserRestockRecheck
}
export interface BrowserRestockRecheck {
  id: string
  device_id: string
  state: 'queued' | 'checking' | 'passed' | 'failed'
  reason: string
  resumed: boolean
  requested_at: string
  started_at?: string
  finished_at?: string
}
export interface BrowserRestockStatus {
  enabled: boolean
  recheck_supported?: boolean
  paused_reason: string
  products: BrowserRestockProduct[]
  devices: BrowserRestockDevice[]
  batches: Array<{ batch_id: string; goods_id: number; status: 'claimed' | 'uncertain' | 'verified'; code_count: number; created_at: string }>
}
const base = '/admin/tools/ldxp/browser'
export const liandongBrowserAPI = {
  async getStatus(): Promise<BrowserRestockStatus> {
    return (await apiClient.get<BrowserRestockStatus>(`${base}/status`)).data
  },
  async requestRecheck(deviceId: string): Promise<BrowserRestockRecheck> {
    return (await apiClient.post<{ recheck: BrowserRestockRecheck }>(`${base}/devices/${encodeURIComponent(deviceId)}/recheck`, {})).data.recheck
  },
  async saveConfig(payload: { enabled: boolean; products: BrowserRestockProduct[] }): Promise<BrowserRestockStatus> {
    return (await apiClient.put<BrowserRestockStatus>(`${base}/config`, payload)).data
  },
  async createDevice(name: string, goodsIds?: number[]): Promise<{ device: BrowserRestockDevice; device_key: string }> {
    return (await apiClient.post<{ device: BrowserRestockDevice; device_key: string }>(`${base}/devices`, { name, ...(goodsIds ? { goods_ids: goodsIds } : {}) })).data
  },
  async revokeDevice(id: string): Promise<{ revoked: boolean }> {
    return (await apiClient.delete<{ revoked: boolean }>(`${base}/devices/${encodeURIComponent(id)}`)).data
  },
  async resume(): Promise<BrowserRestockStatus> {
    return (await apiClient.post<BrowserRestockStatus>(`${base}/resume`, {})).data
  },
}
