import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'

const {
  getInstallation,
  installOrRepair,
  getStatus,
  updateConfig,
  testConnection,
  listGoods,
  previewJob,
  runJob,
  getJob,
  resumeJob,
  exportJob,
  showError,
  showSuccess,
} = vi.hoisted(() => ({
  getInstallation: vi.fn(),
  installOrRepair: vi.fn(),
  getStatus: vi.fn(),
  updateConfig: vi.fn(),
  testConnection: vi.fn(),
  listGoods: vi.fn(),
  previewJob: vi.fn(),
  runJob: vi.fn(),
  getJob: vi.fn(),
  resumeJob: vi.fn(),
  exportJob: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/liandongToolkit', () => ({
  DEFAULT_LIANDONG_TARGET_STOCK: 50000,
  liandongToolkitAPI: {
    getInstallation,
    installOrRepair,
    getStatus,
    updateConfig,
    testConnection,
    listGoods,
    previewJob,
    runJob,
    getJob,
    resumeJob,
    exportJob,
  },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({ showError, showSuccess }),
}))

vi.mock('vue-i18n', async importOriginal => ({
  ...(await importOriginal<typeof import('vue-i18n')>()),
  useI18n: () => ({ t: (key: string) => key }),
}))

import LiandongToolkitView from '../LiandongToolkitView.vue'

const ConfirmDialogStub = defineComponent({
  props: { show: Boolean },
  emits: ['confirm', 'cancel'],
  template: '<div v-if="show" data-testid="run-confirmation"><button type="button" data-testid="confirm-run" @click="$emit(\'confirm\')">confirm</button></div>',
})

const baseInstallation = {
  os: 'linux',
  arch: 'amd64',
  expected_program_path: '/var/lib/sub2api/ldxp-toolkit',
  version: '1.0.0',
  ready: true,
  asset_available: true,
  exists: true,
  executable: true,
  diagnostics: [],
}

const baseStatus = () => ({
  merchant_token_configured: false,
  pending_batch: false,
  products: [
    {
      goods_id: 42,
      cny_amount: 20,
      usd_credit: 2.78,
      target_stock: 50000,
      current_stock: 12000,
      enabled: true,
    },
  ],
  batches: [],
})

function mountView() {
  return mount(LiandongToolkitView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        Icon: { template: '<span />' },
        ConfirmDialog: ConfirmDialogStub,
      },
    },
  })
}

describe('LiandongToolkitView', () => {
  beforeEach(() => {
    getInstallation.mockReset().mockResolvedValue(baseInstallation)
    installOrRepair.mockReset().mockResolvedValue(baseInstallation)
    getStatus.mockReset().mockResolvedValue(baseStatus())
    updateConfig.mockReset().mockImplementation(async () => baseStatus())
    testConnection.mockReset().mockResolvedValue({ ok: true })
    listGoods.mockReset().mockResolvedValue([])
    previewJob.mockReset().mockResolvedValue([])
    runJob.mockReset().mockResolvedValue({ job_id: 'job-1', status: 'queued', selected_goods: [42] })
    getJob.mockReset().mockResolvedValue({ job_id: 'job-1', status: 'queued', selected_goods: [42] })
    resumeJob.mockReset().mockResolvedValue({ job_id: 'job-1', status: 'running', selected_goods: [42] })
    exportJob.mockReset().mockResolvedValue(new Blob(['safe export']))
    showError.mockReset()
    showSuccess.mockReset()
  })

  it('renders an unavailable runtime without claiming installation', async () => {
    getInstallation.mockRejectedValue({ status: 503, message: 'runtime unavailable' })
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-testid="runtime-section"]').text()).toContain('ldxpToolkit.runtime.endpointUnavailable')
    expect(wrapper.get('[data-testid="install-repair"]').attributes('disabled')).toBeDefined()
    expect(wrapper.find('[data-testid="installation-status"]').exists()).toBe(false)
  })

  it('honors the server-declared unavailable runtime state', async () => {
    getInstallation.mockResolvedValue({ version: 'unavailable', diagnostics: ['runtime unavailable'] })
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-testid="runtime-section"]').text()).toContain('ldxpToolkit.runtime.endpointUnavailable')
    expect(wrapper.find('[data-testid="installation-status"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="install-repair"]').attributes('disabled')).toBeDefined()
  })

  it('treats a configured but unreachable merchant as a failed connection test', async () => {
    testConnection.mockResolvedValue({ configured: true, reachable: false, message: 'merchant endpoint unreachable' })
    const wrapper = mountView()
    await flushPromises()

    const tokenInput = wrapper.get('[data-testid="merchant-token-input"]')
    await tokenInput.setValue('merchant-token')
    await wrapper.get('[data-testid="test-connection"]').trigger('click')
    await flushPromises()

    const feedback = wrapper.get('[data-testid="connection-section"] [role="status"]')
    expect(feedback.text()).toContain('merchant endpoint unreachable')
    expect(feedback.classes()).toContain('text-red-600')
    expect(feedback.text()).not.toContain('ldxpToolkit.connection.testSuccess')
    expect((tokenInput.element as HTMLInputElement).value).toBe('')
  })

  it('does not manufacture a pending job from the pending batch flag alone', async () => {
    getStatus.mockResolvedValue({ ...baseStatus(), pending_batch: true })
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.find('[data-testid="active-job-status"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="resume-job"]').exists()).toBe(false)
  })

  it('restores the durable current and recent jobs returned by status', async () => {
    getStatus.mockResolvedValue({
      ...baseStatus(),
      current_job: { job_id: 'job-current', status: 'failed', selected_goods: [42], error: 'remote rejected' },
      jobs: [{ job_id: 'job-recent', status: 'completed', selected_goods: [42] }],
    })
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-testid="active-job-status"]').text()).toContain('ldxpToolkit.status.failed')
    expect(wrapper.get('[data-testid="history-table"]').text()).toContain('job-current')
    expect(wrapper.get('[data-testid="history-table"]').text()).toContain('job-recent')
    expect(wrapper.find('[data-testid="resume-job"]').exists()).toBe(true)
  })

  it.each(['pending', 'queued', 'running', 'needs_reconciliation', 'completed', 'cancelled'])('does not offer resume for %s jobs', async jobStatus => {
    getStatus.mockResolvedValue({
      ...baseStatus(),
      current_job: { job_id: 'job-1', status: jobStatus, selected_goods: [42] },
      jobs: [],
    })
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-testid="active-job-status"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="resume-job"]').exists()).toBe(false)
  })

  it('renders the target gap returned by preview', async () => {
    previewJob.mockResolvedValue({
      products: [
        {
          mapping: { goods_id: 42, cny_amount: 20, usd_credit: 2.78, target_stock: 50000 },
          current_stock: 12000,
          target_stock: 50000,
          planned: 38000,
          eligible: true,
        },
      ],
    })
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-testid="preview-button"]').trigger('click')
    await flushPromises()

    expect(previewJob).toHaveBeenCalledWith({ selected_goods: [42] })
    expect(wrapper.get('[data-testid="preview-row-42"]').text()).toContain('38000')
  })

  it('blocks a run when a preview row is ineligible and shows its reason', async () => {
    previewJob.mockResolvedValue({
      products: [
        {
          mapping: { goods_id: 42, cny_amount: 20, usd_credit: 2.78, target_stock: 50000 },
          current_stock: 12000,
          target_stock: 50000,
          planned: 38000,
          enabled: false,
          eligible: false,
          reason: 'disabled',
        },
      ],
    })
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-testid="preview-button"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="preview-reason-42"]').text()).toContain('disabled')
    const runButton = wrapper.get('[data-testid="run-button"]')
    expect(runButton.attributes('disabled')).toBeDefined()
    await runButton.trigger('click')
    expect(wrapper.find('[data-testid="run-confirmation"]').exists()).toBe(false)
    expect(runJob).not.toHaveBeenCalled()
  })

  it('blocks a run when a preview row has a mapping error', async () => {
    previewJob.mockResolvedValue({
      products: [
        {
          mapping: { goods_id: 42, cny_amount: 20, usd_credit: 2.78, target_stock: 50000 },
          current_stock: 12000,
          target_stock: 50000,
          planned: 38000,
          eligible: true,
          mapping_error: 'mapping version is stale',
        },
      ],
    })
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-testid="preview-button"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="preview-reason-42"]').text()).toContain('mapping version is stale')
    expect(wrapper.get('[data-testid="run-button"]').attributes('disabled')).toBeDefined()
  })

  it('does not read back or retain the merchant token after saving', async () => {
    const secret = 'merchant-token-must-not-be-rendered'
    getStatus.mockResolvedValue({ ...baseStatus(), merchant_token_configured: true })
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).not.toContain(secret)
    const tokenInput = wrapper.get('[data-testid="merchant-token-input"]')
    await tokenInput.setValue(secret)
    await wrapper.get('[data-testid="save-config"]').trigger('click')
    await flushPromises()

    expect(updateConfig).toHaveBeenCalledWith(expect.objectContaining({ merchant_token: secret }))
    expect((tokenInput.element as HTMLInputElement).value).toBe('')
    expect(wrapper.html()).not.toContain(secret)
    expect(wrapper.text()).not.toContain(secret)
  })

  it('requires a preview and confirmation before running a job', async () => {
    previewJob.mockResolvedValue({
      products: [
        {
          mapping: { goods_id: 42, cny_amount: 20, usd_credit: 2.78, target_stock: 50000 },
          current_stock: 12000,
          target_stock: 50000,
          planned: 38000,
          eligible: true,
        },
      ],
    })
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-testid="preview-button"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="run-button"]').trigger('click')
    await flushPromises()

    expect(runJob).not.toHaveBeenCalled()
    expect(wrapper.get('[data-testid="run-confirmation"]').exists()).toBe(true)

    await wrapper.get('[data-testid="confirm-run"]').trigger('click')
    await flushPromises()

    expect(runJob).toHaveBeenCalledWith({ selected_goods: [42] })
  })

  it('renders an unknown remote good price without inventing zero CNY', async () => {
    listGoods.mockResolvedValue([{ goods_id: 99, title: 'Price not provided', stock: 8 }])
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-testid="sync-goods"]').trigger('click')
    await flushPromises()

    const goodsTable = wrapper.get('[data-testid="remote-goods-table"]')
    expect(goodsTable.text()).toContain('Price not provided')
    expect(goodsTable.text()).toContain('8')
    expect(goodsTable.text()).not.toContain('CNY 0.00')
  })
})
