import { describe, expect, it } from 'vitest'
import en from '../locales/en'
import zh from '../locales/zh'

describe('membership locale aggregation', () => {
  it.each([['en', en], ['zh', zh]] as const)('%s aggregates customer and admin membership messages', (_locale, messages) => {
    expect(messages.membership.title).toBeTruthy()
    expect(messages.adminMembership.title).toBeTruthy()
    expect(messages.membership.days).toBeTruthy()
    expect(messages.membership.continue).toBeTruthy()
    for (const state of ['awaiting_input', 'queued', 'submitted', 'processing', 'review_required', 'succeeded', 'failed', 'canceled'] as const) {
      expect(messages.membership.states[state]).toBeTruthy()
    }
    expect(JSON.stringify(messages.membership)).not.toMatch(/supplier|taskId|task_id/i)
    expect(JSON.stringify(messages.adminMembership)).not.toMatch(/supplier|taskId|task_id/i)
  })
})
