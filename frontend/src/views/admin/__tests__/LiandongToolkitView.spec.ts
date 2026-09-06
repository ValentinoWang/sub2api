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
  LIANDONG_UNSAFE_JOB_STATES: ['pending', 'queued', 'running', 'needs_reconciliation'],
  isLiandongTerminalJob: (job: { status?: string } | null | undefined) => job?.status === 'completed',
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
  configured: true,
  merchant_token_configured: true,
  code_secret_configured: true,
  running: false,
  pending_batch: false,
  products: [
    {
      goods_id: 42,
      cny_amount: 20,
      usd_credit: 2.78,
      target_stock: 50000,
      current_stock: 12000,
      enabled: true,
      grant_type: 'balance',
      version: 1,
    },
  ],
  batches: [],
})

function previewItem(goodsId = 42, overrides: Record<string, unknown> = {}) {
  return {
    mapping: {
      mapping_key: `mapping-${goodsId}`,
      version: 1,
      goods_id: goodsId,
      cny_amount: 20,
      grant_type: 'balance',
      usd_credit: 2.78,
      target_stock: 50000,
    },
    current_stock: 12000,
    target_stock: 50000,
    planned: 38000,
    enabled: true,
    eligible: true,
    ...overrides,
  }
}

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
    testConnection.mockReset().mockResolvedValue({ ok: true, configured: true, reachable: true, read_only: true })
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
        previewItem(),
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
        previewItem(42, { enabled: false, eligible: false, reason: 'disabled' }),
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
        previewItem(42, { mapping_error: 'mapping version is stale' }),
      ],
    })
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-testid="preview-button"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="preview-reason-42"]').text()).toContain('mapping version is stale')
    expect(wrapper.get('[data-testid="run-button"]').attributes('disabled')).toBeDefined()
  })

  it('discards an in-flight preview when the requested selection changes', async () => {
    let resolvePreview!: (value: unknown) => void
    previewJob.mockImplementation(() => new Promise(resolve => { resolvePreview = resolve }))
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-testid="preview-button"]').trigger('click')
    await wrapper.get('[data-testid="selection-mode"]').setValue('selected')
    resolvePreview({ products: [previewItem()] })
    await flushPromises()

    expect(wrapper.find('[data-testid="preview-table"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="run-button"]').attributes('disabled')).toBeDefined()
    expect(runJob).not.toHaveBeenCalled()
  })

  it('invalidates a completed preview when a mapping field changes', async () => {
    previewJob.mockResolvedValue({ products: [previewItem()] })
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-testid="preview-button"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="run-button"]').attributes('disabled')).toBeUndefined()

    await wrapper.get('[id="ldxp-target-42"]').setValue('40000')
    await flushPromises()

    expect(wrapper.get('[data-testid="run-button"]').attributes('disabled')).toBeDefined()
  })

  it('blocks a partial preview instead of running unpreviewed products', async () => {
    const firstStatus = baseStatus()
    getStatus.mockResolvedValue({
      ...firstStatus,
      products: [...firstStatus.products, {
        goods_id: 43,
        cny_amount: 30,
        usd_credit: 4.17,
        target_stock: 50000,
        current_stock: 12000,
        enabled: true,
        grant_type: 'balance',
        version: 1,
      }],
    })
    previewJob.mockResolvedValue({ products: [previewItem()] })
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-testid="preview-button"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="preview-section"]').text()).toContain('LDXP_PREVIEW_COVERAGE_INCOMPLETE')
    expect(wrapper.get('[data-testid="run-button"]').attributes('disabled')).toBeDefined()
    expect(runJob).not.toHaveBeenCalled()
  })

  it('blocks duplicate and unknown preview rows', async () => {
    const firstStatus = baseStatus()
    getStatus.mockResolvedValue({
      ...firstStatus,
      products: [...firstStatus.products, {
        goods_id: 43,
        cny_amount: 30,
        usd_credit: 4.17,
        target_stock: 50000,
        current_stock: 12000,
        enabled: true,
        grant_type: 'balance',
        version: 1,
      }],
    })
    previewJob.mockResolvedValue({ products: [previewItem(), previewItem(99)] })
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-testid="preview-button"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="preview-section"]').text()).toContain('LDXP_PREVIEW_UNKNOWN_GOODS')
    expect(wrapper.get('[data-testid="run-button"]').attributes('disabled')).toBeDefined()
    expect(runJob).not.toHaveBeenCalled()
  })

  it.each([
    ['configuration', { configured: false }],
    ['merchant credential', { merchant_token_configured: false }],
    ['code secret', { code_secret_configured: false }],
    ['service cycle', { running: true }],
  ])('blocks a run when %s readiness is unsafe', async (_name, statusOverride) => {
    getStatus.mockResolvedValue({ ...baseStatus(), ...statusOverride })
    previewJob.mockResolvedValue({ products: [previewItem()] })
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-testid="preview-button"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="run-button"]').attributes('disabled')).toBeDefined()
    expect(runJob).not.toHaveBeenCalled()
  })

  it('does not offer resume while the latest service status is running', async () => {
    getStatus.mockResolvedValue({
      ...baseStatus(),
      running: true,
      current_job: { job_id: 'job-failed', status: 'failed', selected_goods: [42], error: 'retryable failure' },
    })
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.find('[data-testid="resume-job"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="job-state-notice"]').text()).toContain('LDXP_JOB_FAILED')
  })

  it('refreshes readiness before confirming a run', async () => {
    getStatus
      .mockResolvedValueOnce(baseStatus())
      .mockResolvedValueOnce({ ...baseStatus(), running: true })
    previewJob.mockResolvedValue({ products: [previewItem()] })
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-testid="preview-button"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="run-button"]').trigger('click')
    await wrapper.get('[data-testid="confirm-run"]').trigger('click')
    await flushPromises()

    expect(getStatus).toHaveBeenCalledTimes(2)
    expect(runJob).not.toHaveBeenCalled()
    expect(wrapper.get('[data-testid="job-error"]').text()).toContain('LDXP_RUN_PRECONDITION_FAILED')
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

  it('clears the merchant token after a failed save', async () => {
    const secret = 'merchant-token-failed-save'
    updateConfig.mockRejectedValue({ status: 400, reason: 'LDXP_CONFIG_INVALID', message: `invalid token ${secret}` })
    const wrapper = mountView()
    await flushPromises()

    const tokenInput = wrapper.get('[data-testid="merchant-token-input"]')
    await tokenInput.setValue(secret)
    await wrapper.get('[data-testid="save-config"]').trigger('click')
    await flushPromises()

    expect((tokenInput.element as HTMLInputElement).value).toBe('')
    expect(wrapper.html()).not.toContain(secret)
    expect(wrapper.get('[data-testid="connection-section"] [role="status"]').text()).toContain('configuration:LDXP_CONFIG_INVALID')
  })

  it('clears the merchant token after a failed connection request', async () => {
    const secret = 'merchant-token-failed-connection'
    testConnection.mockRejectedValue({ status: 503, reason: 'LDXP_NOT_CONFIGURED', message: `merchant unavailable ${secret}` })
    const wrapper = mountView()
    await flushPromises()

    const tokenInput = wrapper.get('[data-testid="merchant-token-input"]')
    await tokenInput.setValue(secret)
    await wrapper.get('[data-testid="test-connection"]').trigger('click')
    await flushPromises()

    const feedback = wrapper.get('[data-testid="connection-section"] [role="status"]')
    expect((tokenInput.element as HTMLInputElement).value).toBe('')
    expect(feedback.text()).not.toContain(secret)
    expect(feedback.text()).toContain('connection:LDXP_NOT_CONFIGURED')
    expect(feedback.text()).not.toContain('ldxpToolkit.errors.endpointUnavailable')
  })

  it('accepts merchant validation only when every required fact is true', async () => {
    testConnection.mockResolvedValue({ configured: true, reachable: true, read_only: true, message: 'read-only probe succeeded' })
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-testid="test-connection"]').trigger('click')
    await flushPromises()

    const feedback = wrapper.get('[data-testid="connection-section"] [role="status"]')
    expect(feedback.text()).toContain('ldxpToolkit.connection.testSuccess')
    expect(feedback.classes()).toContain('text-green-600')
  })

  it.each([false, undefined])('rejects merchant validation without read_only=true (%s)', async readOnly => {
    testConnection.mockResolvedValue({
      configured: true,
      reachable: true,
      ...(readOnly === undefined ? {} : { read_only: readOnly }),
      message: 'merchant probe did not satisfy the read-only contract',
    })
    const wrapper = mountView()
    await flushPromises()

    const tokenInput = wrapper.get('[data-testid="merchant-token-input"]')
    await tokenInput.setValue('merchant-token-read-only')
    await wrapper.get('[data-testid="test-connection"]').trigger('click')
    await flushPromises()

    const feedback = wrapper.get('[data-testid="connection-section"] [role="status"]')
    expect(feedback.classes()).toContain('text-red-600')
    expect(feedback.text()).toContain('LDXP_MERCHANT_NOT_READ_ONLY')
    expect(feedback.text()).not.toContain('ldxpToolkit.connection.testSuccess')
    expect((tokenInput.element as HTMLInputElement).value).toBe('')
  })

  it('requires a preview and confirmation before running a job', async () => {
    previewJob.mockResolvedValue({
      products: [
        previewItem(),
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

  it('preserves an unknown configured price and blocks save and run', async () => {
    const currentStatus = baseStatus()
    getStatus.mockResolvedValue({
      ...currentStatus,
      products: [{ ...currentStatus.products[0], cny_amount: undefined }],
    })
    previewJob.mockResolvedValue({ products: [previewItem()] })
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-testid="mapping-price-unknown-42"]').text()).toBe('-')
    expect(wrapper.get('[data-testid="save-config"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="preview-button"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="run-button"]').attributes('disabled')).toBeDefined()
    expect(updateConfig).not.toHaveBeenCalled()
    expect(runJob).not.toHaveBeenCalled()
  })

  it('offers export only for a completed job and confirms terminal state before download', async () => {
    getStatus.mockResolvedValue({
      ...baseStatus(),
      jobs: [
        { job_id: 'job-queued', status: 'queued', selected_goods: [42] },
        { job_id: 'job-running', status: 'running', selected_goods: [42] },
        { job_id: 'job-failed', status: 'failed', selected_goods: [42] },
        { job_id: 'job-reconcile', status: 'needs_reconciliation', selected_goods: [42] },
        { job_id: 'job-completed', status: 'completed', selected_goods: [42] },
      ],
    })
    getJob.mockResolvedValue({ job_id: 'job-completed', status: 'completed', selected_goods: [42] })
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.findAll('[data-testid^="export-job-"]')).toHaveLength(1)
    expect(wrapper.find('[data-testid="export-job-job-completed"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="export-job-job-queued"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="export-job-job-running"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="export-job-job-failed"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="export-job-job-reconcile"]').exists()).toBe(false)

    await wrapper.get('[data-testid="export-job-job-completed"]').trigger('click')
    await flushPromises()

    expect(getJob).toHaveBeenCalledWith('job-completed')
    expect(exportJob).toHaveBeenCalledWith('job-completed')
  })

  it('rejects export when the server confirmation is no longer terminal', async () => {
    getStatus.mockResolvedValue({
      ...baseStatus(),
      jobs: [{ job_id: 'job-completed', status: 'completed', selected_goods: [42] }],
    })
    getJob.mockResolvedValue({ job_id: 'job-completed', status: 'queued', selected_goods: [42] })
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-testid="export-job-job-completed"]').trigger('click')
    await flushPromises()

    expect(exportJob).not.toHaveBeenCalled()
    expect(showError).toHaveBeenCalledWith(expect.stringContaining('LDXP_EXPORT_UNAVAILABLE'))
  })

  it.each(['queued', 'running', 'needs_reconciliation', 'failed'])('renders a state-specific notice for %s jobs', async jobStatus => {
    getStatus.mockResolvedValue({
      ...baseStatus(),
      current_job: { job_id: 'job-1', status: jobStatus, selected_goods: [42], error: 'sanitized failure reason' },
    })
    const wrapper = mountView()
    await flushPromises()

    const notice = wrapper.get('[data-testid="job-state-notice"]').text()
    const noticeCode = jobStatus === 'needs_reconciliation'
      ? 'LDXP_NEEDS_RECONCILIATION'
      : `LDXP_JOB_${jobStatus.toUpperCase()}`
    expect(notice).toContain(noticeCode)
    if (jobStatus === 'failed') expect(notice).not.toContain('ldxpToolkit.preview.pendingNotice')
  })
})
