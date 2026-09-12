import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const layoutDirectory = dirname(fileURLToPath(import.meta.url))
const appLayoutSource = readFileSync(resolve(layoutDirectory, '../AppLayout.vue'), 'utf8')
const homeViewSource = readFileSync(resolve(layoutDirectory, '../../../views/HomeView.vue'), 'utf8')

function ruleBody(source: string, selector: RegExp) {
  return source.match(selector)?.[1] ?? ''
}

function variableValue(rule: string, name: string) {
  return rule.match(new RegExp(`${name}:\\s*([^;]+);`))?.[1].trim()
}

describe('AppLayout user theme baseline', () => {
  it('uses the same canvas, grid, glass, and border tokens as home in both themes', () => {
    const homeLight = ruleBody(homeViewSource, /\.home-root\s*\{([\s\S]*?)\n\}/)
    const homeDark = ruleBody(homeViewSource, /\.dark \.home-root\s*\{([\s\S]*?)\n\}/)
    const layoutLight = ruleBody(appLayoutSource, /\.app-layout\.user-brand-shell\s*\{([\s\S]*?)\n\}/)
    const layoutDark = ruleBody(appLayoutSource, /:global\(\.dark\) \.app-layout\.user-brand-shell\s*\{([\s\S]*?)\n\}/)

    expect(variableValue(layoutLight, '--user-canvas')).toBe(variableValue(homeLight, '--home-bg'))
    expect(variableValue(layoutLight, '--user-grid-line')).toBe(variableValue(homeLight, '--home-line'))
    expect(variableValue(layoutLight, '--user-surface')).toBe(variableValue(homeLight, '--home-glass'))
    expect(variableValue(layoutLight, '--user-border')).toBe(variableValue(homeLight, '--home-glass-border'))
    expect(variableValue(layoutDark, '--user-canvas')).toBe(variableValue(homeDark, '--home-bg'))
    expect(variableValue(layoutDark, '--user-grid-line')).toBe(variableValue(homeDark, '--home-line'))
    expect(variableValue(layoutDark, '--user-surface')).toBe(variableValue(homeDark, '--home-glass'))
    expect(variableValue(layoutDark, '--user-border')).toBe(variableValue(homeDark, '--home-glass-border'))
  })

  it('keeps dashboard panels and ordinary user pages inside the shared shell contract', () => {
    expect(appLayoutSource).toContain(":class=\"{ 'user-brand-shell': usesUserBrandShell }\"")
    expect(appLayoutSource).toContain('--dashboard-surface: var(--user-surface);')
    expect(appLayoutSource).toContain('--dashboard-border: var(--user-border);')
    expect(appLayoutSource).toContain('--user-content-viewport-height: calc(100vh - 64px - 4rem);')
    expect(appLayoutSource).toContain('width: min(100%, 80rem);')
    expect(appLayoutSource).toContain('padding: 1rem 0.875rem 2rem;')
  })
})
