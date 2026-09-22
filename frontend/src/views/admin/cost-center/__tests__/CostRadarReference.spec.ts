import { mount, flushPromises } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import CostRadarReference from '../CostRadarReference.vue'
const mock = vi.hoisted(() => ({ read: vi.fn() }))
vi.mock('@/api/admin/cost-center-native', () => ({ getRadarReference: mock.read }))
vi.mock('../useCostLedger', () => ({ useCostLedger: () => ({ tr: (zh: string) => zh, tierName: () => 'Pro 20x' }) }))
describe('public Radar reference', () => {
 it('reads on demand and clears values when a later read fails', async () => {
  mock.read.mockResolvedValueOnce({ source: 'https://codexradar.com/en/', retrieved_at: '2026-09-16T00:00:00Z', source_updated_label: 'Updated Sep 13, 20:15', quotas: [{ tier: 'pro20x', model_label: 'Astra only', reference_usd: '1847.00', basis: 'Provided by the site owner', period: null, price_version: null }], speeds: [], warnings: [] }).mockRejectedValueOnce(new Error('source unavailable'))
  const wrapper = mount(CostRadarReference)
  expect(mock.read).not.toHaveBeenCalled()
  await wrapper.find('button').trigger('click'); await flushPromises()
  expect(wrapper.text()).toContain('1847.00')
  expect(wrapper.text()).toContain('美元等值参考量')
  expect(wrapper.text()).toContain('不能直接叫“你的周容量”')
  await wrapper.find('button').trigger('click'); await flushPromises()
  expect(wrapper.text()).not.toContain('1847.00')
  expect(wrapper.find('[role="alert"]').exists()).toBe(true)
 })
})
