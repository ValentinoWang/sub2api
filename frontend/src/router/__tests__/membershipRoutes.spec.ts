import { describe, expect, it } from 'vitest'
import router from '../index'

describe('membership routes', () => {
  it('registers the authenticated customer membership page', () => {
    const route = router.resolve('/memberships')

    expect(route.name).toBe('Memberships')
    expect(route.meta).toMatchObject({
      requiresAuth: true,
      requiresAdmin: false,
      titleKey: 'membership.title',
    })
  })

  it('registers the admin-only membership workbench', () => {
    const route = router.resolve('/admin/membership')

    expect(route.name).toBe('AdminMembership')
    expect(route.meta).toMatchObject({
      requiresAuth: true,
      requiresAdmin: true,
      titleKey: 'adminMembership.title',
    })
  })
})
