import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post, put } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
}))

vi.mock('../client', () => ({
  apiClient: { get, post, put },
}))

import {
  exportJob,
  getInstallation,
  getJob,
  getStatus,
  getInventory,
  installOrRepair,
  isLiandongTerminalJob,
  listGoods,
  previewJob,
  resumeJob,
  runJob,
  setRestockEnabled,
  testConnection,
  updateConfig,
} from '../liandongToolkit'

describe('liandongToolkitAPI', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
    put.mockReset()
    get.mockResolvedValue({ data: {} })
    post.mockResolvedValue({ data: {} })
    put.mockResolvedValue({ data: {} })
  })

  it.each([true, false])('persists automatic restock enablement as %s', async enabled => {
    const status = { enabled, configured: true, running: false, products: [] }
    post.mockResolvedValueOnce({ data: status })
    await expect(setRestockEnabled(enabled)).resolves.toEqual(status)
    expect(post).toHaveBeenCalledWith('/admin/liandong/restock/enable', { enabled })
  })

  it('uses the dedicated installation and configuration paths', async () => {
    await getInstallation()
    expect(get).toHaveBeenCalledWith('/admin/tools/ldxp/installation')

    await installOrRepair()
    expect(post).toHaveBeenCalledWith('/admin/tools/ldxp/installation')

    const config = {
      merchant_token: 'merchant-token',
      generate_code_secret: false,
      products: [{ goods_id: 42, cny_amount: 20, usd_credit: 2.78, target_stock: 50000, enabled: true, grant_type: 'balance' as const }],
    }
    await updateConfig(config)
    expect(put).toHaveBeenCalledWith('/admin/tools/ldxp/config', config)

    await testConnection()
    expect(post).toHaveBeenCalledWith('/admin/tools/ldxp/config/test')
  })

  it('covers goods, preview, run, status, resume, and export paths', async () => {
    await getStatus()
    expect(get).toHaveBeenCalledWith('/admin/tools/ldxp/status')

    await listGoods()
    expect(get).toHaveBeenCalledWith('/admin/tools/ldxp/goods')

    const selection = { selected_goods: [42, 43] }
    await previewJob(selection)
    expect(post).toHaveBeenCalledWith('/admin/tools/ldxp/jobs/preview', selection)

    await runJob(selection)
    expect(post).toHaveBeenCalledWith('/admin/tools/ldxp/jobs/run', selection)

    await getJob('job/42')
    expect(get).toHaveBeenCalledWith('/admin/tools/ldxp/jobs/job%2F42')

    await resumeJob('job/42')
    expect(post).toHaveBeenCalledWith('/admin/tools/ldxp/jobs/job%2F42/resume')

    const blob = new Blob(['safe export'])
    get.mockResolvedValueOnce({ data: blob })
    await exportJob('job/42')
    expect(get).toHaveBeenCalledWith('/admin/tools/ldxp/jobs/job%2F42/export', { responseType: 'blob' })
  })

  it('reads inventory comparison through the dedicated read-only route', async () => {
    const report = { rows: [], reconciliation_required: true, observed_at: '2026-09-13T00:00:00Z' }
    get.mockResolvedValueOnce({ data: report })
    await expect(getInventory()).resolves.toEqual(report)
    expect(get).toHaveBeenCalledWith('/admin/liandong/restock/inventory')
    expect(post).not.toHaveBeenCalled()
    expect(put).not.toHaveBeenCalled()
  })

  it('treats only an explicitly completed job as terminal', () => {
    expect(isLiandongTerminalJob({ status: 'completed' })).toBe(true)
    expect(isLiandongTerminalJob({ status: 'failed' })).toBe(false)
    expect(isLiandongTerminalJob({ status: 'needs_reconciliation' })).toBe(false)
    expect(isLiandongTerminalJob(undefined)).toBe(false)
  })
})
