import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const dashboardSource = readFileSync(
  resolve(dirname(fileURLToPath(import.meta.url)), '../DashboardView.vue'),
  'utf8'
)

describe('DashboardView user shell theme', () => {
  it('inherits shared dashboard tokens from AppLayout instead of defining a second palette', () => {
    expect(dashboardSource).not.toContain('--dashboard-surface:')
    expect(dashboardSource).not.toContain('--dashboard-border:')
    expect(dashboardSource).not.toContain('--dashboard-hover:')
    expect(dashboardSource).not.toContain('--dashboard-accent:')
    expect(dashboardSource).not.toContain('background: #f5f8f7;')
    expect(dashboardSource).not.toContain('background: #0c1315;')
    expect(dashboardSource).not.toContain(':global(.dark) .dashboard-layout')
  })
})
