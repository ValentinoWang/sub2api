import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import AppLayout from '../AppLayout.vue'

const state = vi.hoisted(() => ({
  authenticated: false,
  meta: {} as { requiresAuth?: boolean; requiresAdmin?: boolean },
}))

vi.mock('vue-router', () => ({ useRoute: () => ({ meta: state.meta }) }))
vi.mock('@/stores', () => ({ useAppStore: () => ({ sidebarCollapsed: false }) }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ isAuthenticated: state.authenticated }) }))
vi.mock('@/composables/useOnboardingTour', () => ({ useOnboardingTour: () => ({ replayTour: vi.fn() }) }))
vi.mock('@/stores/onboarding', () => ({ useOnboardingStore: () => ({ setReplayCallback: vi.fn() }) }))

describe('AppLayout user shell selection', () => {
  it.each([
    { authenticated: false, meta: {}, expected: false },
    { authenticated: true, meta: {}, expected: true },
    { authenticated: false, meta: { requiresAuth: true }, expected: true },
    { authenticated: true, meta: { requiresAuth: true }, expected: true },
    { authenticated: true, meta: { requiresAuth: true, requiresAdmin: true }, expected: false },
    { authenticated: false, meta: { requiresAuth: true, requiresAdmin: true }, expected: false },
  ])('selects the user theme for $meta with authenticated=$authenticated: $expected', ({ authenticated, meta, expected }) => {
    state.authenticated = authenticated
    state.meta = meta
    const wrapper = mount(AppLayout, {
      global: { stubs: { AppSidebar: true, AppHeader: true } },
      slots: { default: '<article>Content</article>' },
    })
    expect(wrapper.classes('user-brand-shell')).toBe(expected)
    expect(wrapper.text()).toContain('Content')
    wrapper.unmount()
  })
})
