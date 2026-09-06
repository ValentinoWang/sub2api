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
  installOrRepair,
  listGoods,
  previewJob,
  resumeJob,
  runJob,
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

  it('uses the dedicated installation and configuration paths', async () => {
    await getInstallation()
    expect(get).toHaveBeenCalledWith('/admin/tools/ldxp/installation')

    await installOrRepair()
    expect(post).toHaveBeenCalledWith('/admin/tools/ldxp/installation')

    const config = {
      merchant_token: 'merchant-token',
      generate_code_secret: false,
      products: [{ goods_id: 42, cny_amount: 20, usd_credit: 2.78, target_stock: 50000, enabled: true }],
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
})
