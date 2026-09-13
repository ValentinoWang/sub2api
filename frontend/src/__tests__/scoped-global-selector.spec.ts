import { readFileSync } from 'node:fs'
import { dirname, relative, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'
import { parse } from 'vue/compiler-sfc'

const srcRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const sfcFiles = import.meta.glob('../**/*.vue', { eager: false })

// Matches a selector that opens with `:global(...)` and keeps going afterwards,
// e.g. `:global(.dark) .payment-page`.
const GLOBAL_HEAD_WITH_DESCENDANT = /:global\([^)]*\)\s*[^,{\s][^,{]*\{/g

/**
 * `:global(.dark) .thing { … }` compiles down to `.dark { … }`: Vue's scoped-CSS
 * transform keeps the `:global()` head and silently drops every descendant after
 * it. The declarations then land on `<html>` instead of the component, so dark
 * palettes never reach the elements they were written for (white-on-white text)
 * while leaking onto every other page. Write `.dark .thing` instead — scoped
 * compilation only stamps `[data-v-x]` on the last compound selector, so an
 * ancestor class needs no `:global()` to begin with.
 */
describe('scoped SFC styles', () => {
  it('never starts a descendant selector with :global()', () => {
    const offenders: string[] = []

    for (const key of Object.keys(sfcFiles)) {
      const file = resolve(srcRoot, key.replace(/^\.\.\//, ''))
      const { descriptor } = parse(readFileSync(file, 'utf8'), { filename: file })

      for (const style of descriptor.styles) {
        if (!style.scoped) continue
        for (const match of style.content.matchAll(GLOBAL_HEAD_WITH_DESCENDANT)) {
          offenders.push(`${relative(srcRoot, file)}: ${match[0].slice(0, -1).trim()}`)
        }
      }
    }

    expect(offenders).toEqual([])
  })
})
