import { describe, expect, it } from 'vitest'
import { isBackendModePublicRouteAllowed, routes } from '@/router'

describe('public documentation route contract', () => {
  it.each(['/docs', '/docs/codex-memory'])('%s is public in normal and backend modes', (path) => {
    const route = routes.find((candidate) => candidate.path === path)

    expect(route?.meta?.requiresAuth).toBe(false)
    expect(isBackendModePublicRouteAllowed(path, false)).toBe(true)
  })

  it('/Api_subscribe remains admin-only', () => {
    const route = routes.find((candidate) => candidate.path === '/Api_subscribe')

    expect(route?.meta?.requiresAuth).toBe(true)
    expect(route?.meta?.requiresAdmin).toBe(true)
  })
})
