import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post } = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { get, post } }))
import { checkOpenAIResetCreditHistory, resetOpenAIQuota } from '@/api/admin/accounts'

describe('reset credit preflight API', () => {
  beforeEach(() => { get.mockReset(); post.mockReset() })

  it('uses the read-only history endpoint without refreshing or consuming', async () => {
    const check = { status: 'recent', confirmation_key: 'observed-state' }
    get.mockResolvedValueOnce({ data: check })
    await expect(checkOpenAIResetCreditHistory(10)).resolves.toEqual(check)
    expect(get).toHaveBeenCalledWith('/admin/openai/accounts/10/reset-credit-check')
    expect(post).not.toHaveBeenCalled()
  })

  it('sends the acknowledged observation in the final request without retrying errors', async () => {
    const conflict = { reason: 'RESET_CREDIT_CONFIRMATION_REQUIRED' }
    post.mockRejectedValueOnce(conflict)
    await expect(resetOpenAIQuota(10, 'observed-state')).rejects.toBe(conflict)
    expect(post).toHaveBeenCalledTimes(1)
    expect(post).toHaveBeenCalledWith('/admin/openai/accounts/10/reset-quota', { confirmation_key: 'observed-state' }, { timeout: 90_000 })
  })
})
