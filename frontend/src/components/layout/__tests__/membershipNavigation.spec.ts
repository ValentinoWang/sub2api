import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

const sidebar = readFileSync(resolve(dirname(fileURLToPath(import.meta.url)), '../AppSidebar.vue'), 'utf8')

describe('membership navigation', () => {
  it('adds both customer and administrator entries using the existing simple-mode filter', () => {
    expect(sidebar).toContain("{ path: '/memberships', label: t('membership.title'), icon: CreditCardIcon, hideInSimpleMode: true }")
    expect(sidebar).toContain("{ path: '/admin/membership', label: t('adminMembership.title'), icon: CreditCardIcon, hideInSimpleMode: true }")
  })

  it('does not place supplier, CDK, or internal-task data in navigation source', () => {
    expect(sidebar).not.toMatch(/supplier|cdk|taskId|task_id/i)
  })
})
