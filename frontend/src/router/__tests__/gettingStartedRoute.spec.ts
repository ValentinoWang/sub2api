import { describe, expect, it } from 'vitest'
import { routes } from '@/router'

describe('getting started route contract', () => {
  it('is an authenticated user route with localized page metadata', () => {
    const route = routes.find((candidate) => candidate.path === '/getting-started')

    expect(route?.meta?.requiresAuth).toBe(true)
    expect(route?.meta?.requiresAdmin).toBe(false)
    expect(route?.meta?.titleKey).toBe('gettingStarted.title')
  })
})
