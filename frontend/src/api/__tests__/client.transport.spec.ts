import { createServer, type Server } from 'node:http'
import { once } from 'node:events'
import type { AddressInfo } from 'node:net'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import axios from 'axios'

vi.mock('@/i18n', () => ({ getLocale: () => 'zh' }))

describe('API transport failure recovery over loopback HTTP', () => {
  let server: Server
  let origin: string
  let requests: string[]
  let delay: number
  let refreshStatus: number

  beforeEach(async () => {
    vi.resetModules()
    localStorage.clear()
    sessionStorage.clear()
    window.history.replaceState({}, '', '/login')
    vi.stubGlobal('navigator', {
      locks: { request: (_name: string, callback: () => unknown) => callback() },
    })
    requests = []
    delay = 0
    refreshStatus = 200
    server = createServer((req, res) => {
      requests.push(`${req.method} ${req.url?.split('?')[0]}`)
      res.setHeader('Content-Type', 'application/json')
      if (req.url?.startsWith('/api/v1/auth/me')) {
        res.writeHead(401).end(JSON.stringify({ code: 'TOKEN_EXPIRED' }))
      } else if (req.url === '/api/v1/auth/refresh') {
        if (refreshStatus) res.writeHead(refreshStatus).end(JSON.stringify({ code: 'REFRESH_UNAVAILABLE' }))
      } else if (delay) {
        // The socket stays open without a response, reproducing a stalled server.
        const timer = setTimeout(() => res.end(JSON.stringify({ code: 0, data: {} })), delay)
        res.on('close', () => clearTimeout(timer))
      } else {
        res.end(JSON.stringify({ code: 0, data: { recovered: true } }))
      }
    })
    server.listen(0, '127.0.0.1')
    await once(server, 'listening')
    origin = `http://127.0.0.1:${(server.address() as AddressInfo).port}`
    vi.stubEnv('VITE_API_BASE_URL', `${origin}/api/v1`)
  })

  afterEach(async () => {
    vi.restoreAllMocks()
    vi.unstubAllEnvs()
    vi.unstubAllGlobals()
    server.closeAllConnections()
    if (server.listening) await new Promise<void>((resolve) => server.close(() => resolve()))
  })

  function seedSession() {
    localStorage.setItem('auth_token', 'synthetic-access')
    localStorage.setItem('refresh_token', 'synthetic-refresh')
    localStorage.setItem('auth_user', JSON.stringify({ id: 42 }))
  }

  async function client() {
    const { apiClient } = await import('@/api/client')
    apiClient.defaults.adapter = 'http'
    apiClient.defaults.proxy = false
    return apiClient
  }

  it('classifies a slow login as a timeout and permits a later request', async () => {
    const api = await client()
    delay = 1_000
    await expect(api.post('/auth/login', {}, { timeout: 100 })).rejects.toMatchObject({
      status: 0, code: 'REQUEST_TIMEOUT',
      message: expect.stringContaining('超时'),
    })
    expect(requests).toEqual(['POST /api/v1/auth/login'])
    delay = 0
    await expect(api.post('/auth/login', {})).resolves.toMatchObject({ data: { recovered: true } })
  })

  it('does not retry an uncertain redemption or remove the session after a timeout', async () => {
    seedSession()
    const api = await client()
    delay = 1_000
    await expect(api.post('/redeem', { code: 'synthetic-code' }, { timeout: 100 })).rejects.toMatchObject({
      status: 0, code: 'REQUEST_TIMEOUT',
    })
    expect(requests).toEqual(['POST /api/v1/redeem'])
    expect(localStorage.getItem('auth_token')).toBe('synthetic-access')
    expect(localStorage.getItem('refresh_token')).toBe('synthetic-refresh')
  })

  it('distinguishes a refused connection from a response timeout', async () => {
    const api = await client()
    await new Promise<void>((resolve) => server.close(() => resolve()))
    await expect(api.post('/auth/login', {})).rejects.toMatchObject({
      status: 0, code: 'NETWORK_ERROR',
      message: expect.stringContaining('连接'),
    })
  })

  it.each([503, 429])('preserves credentials when refresh returns %s', async (status) => {
    seedSession()
    const api = await client()
    refreshStatus = status
    // Keep refresh on real HTTP while the browser session storage stays in jsdom.
    const post = axios.post.bind(axios)
    vi.spyOn(axios, 'post').mockImplementation((url, data, config) =>
      post(url, data, { ...config, adapter: 'http', proxy: false }))
    await expect(api.get('/auth/me')).rejects.toMatchObject({
      status, code: 'AUTH_REFRESH_UNAVAILABLE',
    })
    expect(requests).toEqual(['GET /api/v1/auth/me', 'POST /api/v1/auth/refresh'])
    expect(localStorage.getItem('auth_token')).toBe('synthetic-access')
    expect(localStorage.getItem('refresh_token')).toBe('synthetic-refresh')
    expect(sessionStorage.getItem('auth_expired')).toBeNull()
  })

  it('still removes credentials when refresh is explicitly rejected', async () => {
    seedSession()
    const api = await client()
    refreshStatus = 401
    const post = axios.post.bind(axios)
    vi.spyOn(axios, 'post').mockImplementation((url, data, config) =>
      post(url, data, { ...config, adapter: 'http', proxy: false }))
    await expect(api.get('/auth/me')).rejects.toMatchObject({ status: 401, code: 'TOKEN_REFRESH_FAILED' })
    expect(localStorage.getItem('auth_token')).toBeNull()
    expect(localStorage.getItem('refresh_token')).toBeNull()
  })

  it('preserves the session when the refresh request itself times out', async () => {
    seedSession()
    const api = await client()
    refreshStatus = 0
    const post = axios.post.bind(axios)
    vi.spyOn(axios, 'post').mockImplementation((url, data, config) =>
      post(url, data, { ...config, adapter: 'http', proxy: false, timeout: 100 }))
    await expect(api.get('/auth/me')).rejects.toMatchObject({ status: 0, code: 'REQUEST_TIMEOUT' })
    expect(requests).toEqual(['GET /api/v1/auth/me', 'POST /api/v1/auth/refresh'])
    expect(localStorage.getItem('auth_token')).toBe('synthetic-access')
    expect(localStorage.getItem('refresh_token')).toBe('synthetic-refresh')
    expect(sessionStorage.getItem('auth_expired')).toBeNull()
  })
})
