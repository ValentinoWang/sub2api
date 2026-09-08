import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../CustomPageView.vue')
const componentSource = readFileSync(componentPath, 'utf8')

describe('CustomPageView user shell integration', () => {
  it('takes its geometry and tool surfaces from the user shell while retaining both content modes', () => {
    expect(componentSource).toContain('class="card custom-page-card flex-1 min-h-0 overflow-hidden"')
    expect(componentSource).toContain(
      'height: var(--user-content-viewport-height, calc(100vh - 64px - 4rem));'
    )
    expect(componentSource).toContain(':global(.user-brand-shell) .custom-page-card')
    expect(componentSource).toContain('background-color: var(--user-surface);')
    expect(componentSource).toContain('background-color: var(--user-canvas);')
    expect(componentSource).toContain('border-color: var(--user-border);')
    expect(componentSource).toContain('color: var(--user-foreground);')
    expect(componentSource).toContain('color: var(--user-muted);')
    expect(componentSource).toContain('border-radius: 8px;')
    expect(componentSource).toContain('v-html="renderedHtml"')
    expect(componentSource).toContain('class="custom-embed-frame"')
  })
})
