import { apiClient } from './client'

export const LIANDONG_TOOLKIT_BASE_PATH = '/admin/tools/ldxp'
export const DEFAULT_LIANDONG_TARGET_STOCK = 50000

export interface LiandongInstallationStatus {
  available?: boolean
  endpoint_available?: boolean
  runtime_available?: boolean
  ready?: boolean
  asset_available?: boolean
  os?: string
  arch?: string
  platform?: string | { os?: string; arch?: string }
  expected_program_path?: string
  program_path?: string
  path?: string
  version?: string
  exists?: boolean
  executable?: boolean
  executable_bit?: boolean
  sha256?: string
  data_directory?: string
  data_directory_writable?: boolean
  writable_data_dir?: boolean
  writable_data_directory?: boolean
  diagnostics?: string[]
  install_command?: string
}

export interface LiandongInstallationResult {
  installed: boolean
  status: LiandongInstallationStatus
}

export type LiandongInstallationResponse = LiandongInstallationStatus | LiandongInstallationResult

export interface LiandongProductMapping {
  goods_id: number
  cny_amount: number
  usd_credit: number
  target_stock: number
  enabled: boolean
  grant_type?: string
  external_url?: string
  version?: number
  threshold?: number
  restock_count?: number
  current_stock?: number | null
  last_error?: string
  last_run_at?: string
}

export interface LiandongMappingSnapshot {
  mapping_key?: string
  version?: number
  goods_id: number
  cny_amount: number
  grant_type?: string
  usd_credit: number
  external_url?: string
  target_stock: number
}

export interface LiandongStatus {
  integration_mode?: string
  payment_readiness?: string
  configured?: boolean
  merchant_token_configured?: boolean
  code_secret_configured?: boolean
  enabled?: boolean
  running?: boolean
  interval_seconds?: number
  last_run_at?: string
  last_error?: string
  pending_batch?: boolean | LiandongBatchStatus | null
  pending_job?: LiandongJob | null
  current_job?: LiandongJob | null
  products: LiandongProductMapping[]
  batches?: LiandongBatchStatus[]
  jobs?: LiandongJob[]
}

export interface LiandongBatchStatus {
  batch_id: string
  job_id?: string
  goods_id: number
  cny_amount?: number
  usd_credit?: number
  code_count?: number
  status: string
  remote_stock_before?: number | null
  remote_stock_after?: number | null
  error?: string
  created_at?: string
  uploaded_at?: string
  updated_at?: string
}

export interface LiandongConfigUpdate {
  merchant_token: string
  generate_code_secret?: boolean
  products: LiandongProductMapping[]
}

export interface LiandongConnectionTestResponse {
  ok?: boolean
  configured?: boolean
  reachable?: boolean
  read_only?: boolean
  available?: boolean
  message?: string
  diagnostics?: string[]
}

export interface LiandongRemoteGood {
  goods_id: number
  name?: string
  title?: string
  description?: string
  type?: string
  price?: number
  selling_price?: number
  cny_amount?: number
  stock?: number
  current_stock?: number
  unsold_stock?: number
  is_proxy?: number | boolean
}

export type LiandongGoodsResponse =
  | LiandongRemoteGood[]
  | {
      items?: LiandongRemoteGood[]
      goods?: LiandongRemoteGood[]
    }

export interface LiandongJobSelectionRequest {
  selected_goods: number[]
}

export interface LiandongPreviewItem {
  goods_id?: number
  mapping?: LiandongMappingSnapshot
  cny_amount?: number
  usd_credit?: number
  current_stock?: number | null
  unsold_stock?: number | null
  target_stock?: number
  planned_addition?: number
  planned?: number
  enabled?: boolean
  eligible?: boolean
  reason?: string
  batch_id?: string
  mapping_error?: string
  error?: string
}

export type LiandongPreviewResponse =
  | LiandongPreviewItem[]
  | {
      items?: LiandongPreviewItem[]
      products?: LiandongPreviewItem[]
      preview?: LiandongPreviewItem[]
    }

export type LiandongJobState =
  | 'pending'
  | 'queued'
  | 'running'
  | 'completed'
  | 'failed'
  | 'needs_reconciliation'
  | 'cancelled'
  | string

export interface LiandongJob {
  id?: string
  job_id?: string
  status: LiandongJobState
  goods_ids?: number[]
  selected_goods?: number[]
  items?: LiandongPreviewItem[]
  products?: LiandongPreviewItem[]
  batches?: LiandongBatchStatus[]
  error?: string
  message?: string
  export_available?: boolean
  export_url?: string
  created_at?: string
  updated_at?: string
  completed_at?: string
}

export type LiandongJobResponse = LiandongJob | { job: LiandongJob }

export async function getInstallation(): Promise<LiandongInstallationStatus> {
  const { data } = await apiClient.get<LiandongInstallationStatus>(`${LIANDONG_TOOLKIT_BASE_PATH}/installation`)
  return data
}

export async function installOrRepair(): Promise<LiandongInstallationResponse> {
  const { data } = await apiClient.post<LiandongInstallationResponse>(`${LIANDONG_TOOLKIT_BASE_PATH}/installation`)
  return data
}

export async function getStatus(): Promise<LiandongStatus> {
  const { data } = await apiClient.get<LiandongStatus>(`${LIANDONG_TOOLKIT_BASE_PATH}/status`)
  return data
}

export async function updateConfig(payload: LiandongConfigUpdate): Promise<LiandongStatus> {
  const { data } = await apiClient.put<LiandongStatus>(`${LIANDONG_TOOLKIT_BASE_PATH}/config`, payload)
  return data
}

export async function testConnection(): Promise<LiandongConnectionTestResponse> {
  const { data } = await apiClient.post<LiandongConnectionTestResponse>(`${LIANDONG_TOOLKIT_BASE_PATH}/config/test`)
  return data
}

export async function listGoods(): Promise<LiandongGoodsResponse> {
  const { data } = await apiClient.get<LiandongGoodsResponse>(`${LIANDONG_TOOLKIT_BASE_PATH}/goods`)
  return data
}

export async function previewJob(payload: LiandongJobSelectionRequest): Promise<LiandongPreviewResponse> {
  const { data } = await apiClient.post<LiandongPreviewResponse>(
    `${LIANDONG_TOOLKIT_BASE_PATH}/jobs/preview`,
    payload
  )
  return data
}

export async function runJob(payload: LiandongJobSelectionRequest): Promise<LiandongJobResponse> {
  const { data } = await apiClient.post<LiandongJobResponse>(`${LIANDONG_TOOLKIT_BASE_PATH}/jobs/run`, payload)
  return data
}

export async function getJob(id: string): Promise<LiandongJobResponse> {
  const { data } = await apiClient.get<LiandongJobResponse>(
    `${LIANDONG_TOOLKIT_BASE_PATH}/jobs/${encodeURIComponent(id)}`
  )
  return data
}

export async function resumeJob(id: string): Promise<LiandongJobResponse> {
  const { data } = await apiClient.post<LiandongJobResponse>(
    `${LIANDONG_TOOLKIT_BASE_PATH}/jobs/${encodeURIComponent(id)}/resume`
  )
  return data
}

export async function exportJob(id: string): Promise<Blob> {
  const { data } = await apiClient.get<Blob>(
    `${LIANDONG_TOOLKIT_BASE_PATH}/jobs/${encodeURIComponent(id)}/export`,
    { responseType: 'blob' }
  )
  return data
}

export const liandongToolkitAPI = {
  getInstallation,
  installOrRepair,
  install: installOrRepair,
  getStatus,
  updateConfig,
  testConnection,
  listGoods,
  previewJob,
  preview: previewJob,
  runJob,
  run: runJob,
  getJob,
  resumeJob,
  exportJob,
}

export const liandongToolkitApi = liandongToolkitAPI

export default liandongToolkitAPI
