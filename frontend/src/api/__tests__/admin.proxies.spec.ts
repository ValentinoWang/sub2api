import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({ get: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { get } }))

import { getAll, getAllWithCount, list } from '@/api/admin/proxies'

const validProxy = {
  id: 1,
  name: 'valid proxy',
  protocol: 'http',
  host: 'proxy.example',
  port: 3128,
  username: null,
  status: 'active',
  expires_at: null,
  fallback_mode: 'none',
  backup_proxy_id: null,
  expiry_warn_days: 7,
  created_at: '2026-09-09T00:00:00Z',
  updated_at: '2026-09-09T00:00:00Z',
}

describe.each([
  { name: 'paginated list', load: () => list(), wrap: (items: unknown[]) => ({ items, total: items.length, page: 1, page_size: 20, pages: 1 }), counts: true, malformedError: 'Invalid proxy pagination response' },
  { name: 'account selector', load: getAll, wrap: (items: unknown[]) => items, malformedError: 'Invalid proxy list response' },
  { name: 'account selector with counts', load: getAllWithCount, wrap: (items: unknown[]) => items, counts: true, malformedError: 'Invalid proxy list response' },
])('$name', ({ load, wrap, counts, malformedError }) => {
  beforeEach(() => { get.mockReset() })

  it.each(['', undefined, null, {}, { items: null }, { items: {} }])(
    'rejects an empty or malformed successful response: %j',
    async (data) => {
      get.mockResolvedValue({ data })
      await expect(load()).rejects.toThrow(malformedError)
    },
  )

  it.each([{ items: [] }, { items: [validProxy] }])('preserves valid rows: %j', async ({ items }) => {
    const rows = counts ? items.map((item) => ({ ...item, account_count: 0 })) : items
    const data = wrap(rows)
    get.mockResolvedValue({ data })
    await expect(load()).resolves.toBe(data)
  })

  it.each(['id', 'name', 'protocol', 'host', 'port', 'username', 'status', 'expires_at', 'fallback_mode', 'expiry_warn_days', 'created_at', 'updated_at'])(
    'rejects a row missing %s',
    async (field) => {
      const row = { ...validProxy, ...(counts ? { account_count: 0 } : {}) } as Record<string, unknown>
      delete row[field]
      get.mockResolvedValue({ data: wrap([row]) })
      await expect(load()).rejects.toThrow('Invalid proxy list response')
    },
  )
})

describe('paginated proxy metadata', () => {
  beforeEach(() => { get.mockReset() })

  it.each(['total', 'page', 'page_size', 'pages'])('rejects missing %s', async (field) => {
    const data: Record<string, unknown> = {
      items: [{ ...validProxy, account_count: 0 }],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    }
    delete data[field]
    get.mockResolvedValue({ data })
    await expect(list()).rejects.toThrow('Invalid proxy pagination response')
  })
})
