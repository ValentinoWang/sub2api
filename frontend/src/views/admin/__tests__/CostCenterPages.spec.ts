import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { createMemoryHistory, createRouter, RouterView, type Router } from 'vue-router'
import { createI18n } from 'vue-i18n'
import { costCenterSections, registerCostCenterRoute } from '@/router/cost-center'
import zh from '@/i18n/locales/zh/common'
import en from '@/i18n/locales/en/common'
import type { LedgerCommand, LedgerEvent } from '@/api/admin/cost-center'

const api = vi.hoisted(() => ({
  getCostCatalog: vi.fn(),
  compareCostTiers: vi.fn(),
  getCostLedgerHealth: vi.fn(),
  getCostLedgerSummary: vi.fn(),
  listCostLedgerEvents: vi.fn(),
  appendCostLedger: vi.fn()
}))

vi.mock('@/api/admin/cost-center', () => api)
vi.mock('@/components/layout/AppLayout.vue', () => ({
  default: { template: '<main><slot /></main>' }
}))

let wrapper: VueWrapper | undefined
let router: Router
let entries: LedgerEvent[]

async function openPage(path: string) {
  router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/outside', component: { template: '<p>Outside</p>' } }]
  })
  registerCostCenterRoute(router)
  await router.push(path)
  await router.isReady()
  wrapper = mount(RouterView, {
    global: {
      plugins: [router, createI18n({ legacy: false, locale: 'zh', messages: { zh, en } })]
    }
  })
  await flushPromises()
  return wrapper
}

async function navigate(path: string) {
  await router.push(path)
  await flushPromises()
}

beforeEach(() => {
  vi.clearAllMocks()
  entries = []
  api.getCostLedgerHealth.mockResolvedValue({ status: 'READABLE', event_count: 0 })
  api.listCostLedgerEvents.mockImplementation(async () => ({
    items: [...entries], next_after: null, total: entries.length
  }))
  api.getCostCatalog.mockResolvedValue({
    version: 'test-only', checked_at: '2026-09-16T00:00:00Z',
    valid_until: '2026-09-17T00:00:00Z', plans: []
  })
  api.appendCostLedger.mockImplementation(async (command: LedgerCommand, key: string) => {
    const existing = entries.find(entry => entry.idempotency_key === key)
    if (existing) return { event: existing, replayed: true }
    const event: LedgerEvent = {
      id: 'cost-test-1', sequence: 1, idempotency_key: key,
      request_hash: 'a'.repeat(64), actor_id: 1,
      recorded_at: '2026-09-16T00:00:00Z', command
    }
    entries.push(event)
    return { event, replayed: false }
  })
})

afterEach(() => {
  wrapper?.unmount()
  wrapper = undefined
  vi.restoreAllMocks()
})

describe('Cost center subpages', () => {
  it('opens accounting from the original address and keeps every child admin-only', async () => {
    const page = await openPage('/admin/cost-center')
    expect(router.currentRoute.value.path).toBe('/admin/cost-center/accounting')
    expect(page.get('h1').text()).toBe('成本核算')
    expect(page.find('#ledger-model').exists()).toBe(true)
    expect(page.find('#purchase-reference').exists()).toBe(false)
    expect(page.find('#cc-demand').exists()).toBe(false)
    for (const section of costCenterSections) {
      const resolved = router.resolve(section.path)
      expect(resolved.matched).toHaveLength(2)
      expect(resolved.meta.requiresAuth).toBe(true)
      expect(resolved.meta.requiresAdmin).toBe(true)
      expect(page.find(`nav a[href="${section.path}"]`).exists()).toBe(true)
    }
  })

  it('opens comparison directly without reading the ledger', async () => {
    const page = await openPage('/admin/cost-center/comparison')
    expect(page.get('h1').text()).toBe('三档情景比较')
    expect(page.get('#cc-compare-body').isVisible()).toBe(true)
    expect(page.find('#purchase-reference').exists()).toBe(false)
    expect(page.find('#ledger-model').exists()).toBe(false)
    expect(api.getCostCatalog).toHaveBeenCalledTimes(1)
    expect(api.getCostLedgerHealth).not.toHaveBeenCalled()
    expect(api.listCostLedgerEvents).not.toHaveBeenCalled()
  })

  it('preserves purchase drafts and retry keys when visiting ledger records', async () => {
    const page = await openPage('/admin/cost-center/purchases')
    await page.get('#purchase-reference').setValue('invoice-001')
    await page.get('#purchase-supplier').setValue('test-supplier')
    await page.get('#purchase-asset').setValue('test-account')
    await page.get('#purchase-amount').setValue('30')
    await page.get('#purchase-evidence-ref').setValue('test-proof')
    await page.get('#ledger-purchase form').trigger('submit')
    await flushPromises()
    expect(api.appendCostLedger).toHaveBeenCalledTimes(1)
    const [command, key] = api.appendCostLedger.mock.calls[0]
    expect(command).toMatchObject({
      kind: 'purchase',
      purchase: { reference: 'invoice-001', amount: '30', currency: 'USD' }
    })
    expect(key).toEqual(expect.any(String))

    await navigate('/admin/cost-center/ledger')
    expect(page.get('h1').text()).toBe('账本记录')
    expect(page.text()).toContain('cost-test-1')
    expect(page.find('#purchase-reference').exists()).toBe(false)
    const correct = page.findAll('button').find(button => button.text() === '更正')
    expect(correct).toBeDefined()
    await correct!.trigger('click')
    expect(page.find('#ledger-void-form').exists()).toBe(true)

    await navigate('/admin/cost-center/purchases')
    expect((page.get('#purchase-reference').element as HTMLInputElement).value).toBe('invoice-001')
    expect((page.get('#purchase-amount').element as HTMLInputElement).value).toBe('30')
    await page.get('#ledger-purchase form').trigger('submit')
    await flushPromises()
    expect(api.appendCostLedger.mock.calls[1]).toEqual([command, key])
    expect(entries).toHaveLength(1)
    expect(page.text()).toContain('没有重复记账')
  })

  it('reuses ledger loading between subpages and releases drafts when leaving the workbench', async () => {
    const page = await openPage('/admin/cost-center/purchases')
    await page.get('#purchase-reference').setValue('unsaved-draft')
    await navigate('/admin/cost-center/accounting')
    await navigate('/admin/cost-center/ledger')
    expect(api.getCostLedgerHealth).toHaveBeenCalledTimes(1)
    expect(api.listCostLedgerEvents).toHaveBeenCalledTimes(1)
    await navigate('/outside')
    await navigate('/admin/cost-center/purchases')
    expect((page.get('#purchase-reference').element as HTMLInputElement).value).toBe('')
    expect(api.getCostLedgerHealth).toHaveBeenCalledTimes(2)
  })
})
