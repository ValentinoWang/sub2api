import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../TablePageLayout.vue')
const componentSource = readFileSync(componentPath, 'utf8')

describe('TablePageLayout responsive table scrolling', () => {
  it('does not disable the table horizontal scroll container in mobile mode', () => {
    const tableWrapperBlocks = Array.from(
      componentSource.matchAll(/([^{}]*:deep\(\.table-wrapper\)[^{}]*)\{([^{}]*)\}/g)
    )

    expect(tableWrapperBlocks.length).toBeGreaterThan(0)

    const baseBlock = tableWrapperBlocks.find(([selector]) => !selector.includes('.mobile-mode'))
    const mobileBlocks = tableWrapperBlocks.filter(([selector]) => selector.includes('.mobile-mode'))

    expect(baseBlock?.[2]).toContain('overflow-x-auto')
    expect(mobileBlocks.every(([, , declarations]) => !declarations.includes('overflow-visible'))).toBe(
      true
    )
  })

  it('uses the user shell color and viewport contracts without changing mobile scrolling', () => {
    expect(componentSource).toContain(
      'height: var(--user-content-viewport-height, calc(100vh - 64px - 4rem));'
    )
    expect(componentSource).toContain(':global(.user-brand-shell) .table-page-layout')
    expect(componentSource).toContain('background-color: var(--user-surface);')
    expect(componentSource).toContain('border-color: var(--user-border);')
    expect(componentSource).toContain('background-color: var(--user-canvas);')
    expect(componentSource).toContain('color: var(--user-foreground);')
    expect(componentSource).toContain('color: var(--user-muted);')
    expect(componentSource).toContain('border-radius: 8px;')
    expect(componentSource).toContain(
      ':global(.user-brand-shell) .table-page-layout.mobile-mode .table-scroll-container'
    )
    expect(componentSource).toContain('background-color: transparent;')
  })
})
