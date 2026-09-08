import { describe, expect, it, vi } from 'vitest'
import { mount, RouterLinkStub } from '@vue/test-utils'

import ExperienceCollection from '../ExperienceCollection.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key }),
}))

describe('ExperienceCollection', () => {
  it('renders the requested featured card count in a three-column desktop grid', () => {
    const wrapper = mount(ExperienceCollection, {
      props: { limit: 3, columns: 3 },
      global: {
        stubs: {
          RouterLink: RouterLinkStub,
          Icon: true,
        },
      },
    })

    expect(wrapper.findAll('.experience-card')).toHaveLength(3)
    expect(wrapper.get('.experience-grid').classes()).toContain('experience-grid--three-columns')
  })

  it('keeps the two-column grid as the default for compact consumers', () => {
    const wrapper = mount(ExperienceCollection, {
      global: {
        stubs: {
          RouterLink: RouterLinkStub,
          Icon: true,
        },
      },
    })

    expect(wrapper.findAll('.experience-card')).toHaveLength(1)
    expect(wrapper.get('.experience-grid').classes()).not.toContain('experience-grid--three-columns')
  })
})
