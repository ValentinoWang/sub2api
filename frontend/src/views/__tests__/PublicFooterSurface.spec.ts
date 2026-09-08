import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const viewsDirectory = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const legalDocumentSource = readFileSync(resolve(viewsDirectory, 'public/LegalDocumentView.vue'), 'utf8')
const modelPlazaSource = readFileSync(resolve(viewsDirectory, 'ModelPlazaView.vue'), 'utf8')

describe('standalone public footer coverage', () => {
  it('keeps the shared rest2build footer on legal documents', () => {
    expect(legalDocumentSource).toContain("import Rest2BuildBrandFooter from '@/components/common/Rest2BuildBrandFooter.vue'")
    expect(legalDocumentSource).toContain('<footer>')
    expect(legalDocumentSource).toContain('<Rest2BuildBrandFooter />')
    expect(legalDocumentSource).toContain('class="legal-page flex min-h-screen flex-col')
  })

  it('keeps the shared rest2build footer on the standalone model plaza', () => {
    expect(modelPlazaSource).toContain("import Rest2BuildBrandFooter from '@/components/common/Rest2BuildBrandFooter.vue'")
    expect(modelPlazaSource).toContain('<footer>')
    expect(modelPlazaSource).toContain('<Rest2BuildBrandFooter />')
    expect(modelPlazaSource).toContain('class="model-plaza-page flex min-h-screen flex-col"')
  })
})
