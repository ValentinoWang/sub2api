import { mount, flushPromises } from '@vue/test-utils'
import { ref } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import CostNativeTools from '../CostNativeTools.vue'

const mocks = vi.hoisted(() => ({ append: vi.fn(), accounts: vi.fn(), analyze: vi.fn(), reconcile: vi.fn(), refresh: vi.fn() }))
vi.mock('@/api/admin/cost-center-native', () => ({ appendNativeCost: mocks.append, getCostSourceAccounts: mocks.accounts, analyzeNativeCost: mocks.analyze, syncCostSource: mocks.reconcile }))
vi.mock('../useCostLedger', () => ({ useCostLedger: () => ({ tr: (zh: string) => zh, tiers: ['plus', 'pro5x', 'pro20x'], tierName: (value: string) => value, currencies: ['USD'], connected: ref(true), refresh: mocks.refresh, summary: ref(null) }) }))

describe('native cost tools', () => {
 beforeEach(() => { vi.clearAllMocks(); mocks.refresh.mockResolvedValue(undefined) })
 it('keeps an idempotency key after a failed payment recording', async () => {
  mocks.append.mockRejectedValueOnce(new Error('response lost')).mockResolvedValueOnce({ event: { id: 'cost-1' }, replayed: true })
  const wrapper = mount(CostNativeTools)
  const fields = wrapper.findAll('form input')
  for (const [i, value] of ['receipt-1', 'pool-1', 'credit', '100', '10', '2026-09-01T00:00', '2026-10-01T00:00'].entries()) await fields[i]!.setValue(value)
  await wrapper.find('form').trigger('submit'); await flushPromises()
  await wrapper.find('form').trigger('submit'); await flushPromises()
  expect(mocks.append).toHaveBeenCalledTimes(2)
  expect(mocks.append.mock.calls[1]![1]).toBe(mocks.append.mock.calls[0]![1])
  expect(mocks.append.mock.calls[1]![0].credit_lot.paid_at).toBe('2026-09-01T00:00:00Z')
  expect(wrapper.text()).toContain('没有重复记账')
 })
 it('reads real account choices without automatically choosing an identity', async () => {
  mocks.accounts.mockResolvedValue([{ id: 7, platform: 'openai', type: 'oauth', status: 'active' }])
  const wrapper = mount(CostNativeTools)
  await wrapper.find('select').setValue('account_interval')
  await wrapper.find('form button').trigger('click'); await flushPromises()
  expect(wrapper.find('form select').element.value).toBe('0')
  expect(wrapper.find('form button.btn-primary').attributes('disabled')).toBeDefined()
  expect(wrapper.text()).toContain('#7')
 })
 it('does not turn an unreachable source into a zero-usage report', async () => {
  mocks.reconcile.mockRejectedValue(new Error('offline'))
  const wrapper = mount(CostNativeTools)
  await wrapper.find('select').setValue('reconcile')
  for (const [i, value] of ['2026-09-01', '2026-09-16', 'model'].entries()) await wrapper.findAll('form input')[i]!.setValue(value)
  await wrapper.find('form').trigger('submit'); await flushPromises()
  expect(wrapper.find('[role="alert"]').exists()).toBe(true)
  expect(wrapper.find('table').exists()).toBe(false)
 })
})
