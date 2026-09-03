import { describe, expect, it } from 'vitest'

import { BRAND_NAME, resolveBrandName, splitWordmark } from '../brand'

describe('brand helpers', () => {
  it('falls back to the fork brand when the site name is empty or the upstream default', () => {
    expect(resolveBrandName()).toBe(BRAND_NAME)
    expect(resolveBrandName('')).toBe(BRAND_NAME)
    expect(resolveBrandName('   ')).toBe(BRAND_NAME)
    expect(resolveBrandName('Sub2API')).toBe(BRAND_NAME)
  })

  it('keeps an admin-provided site name', () => {
    expect(resolveBrandName('My Gateway')).toBe('My Gateway')
    expect(resolveBrandName('  rest2build  ')).toBe('rest2build')
  })

  it('splits xxx2yyy names into wordmark halves', () => {
    expect(splitWordmark('rest2build')).toEqual({ left: 'rest', right: 'build' })
    expect(splitWordmark('Sub2API')).toEqual({ left: 'Sub', right: 'API' })
  })

  it('returns null for names that are not shaped like a wordmark', () => {
    expect(splitWordmark('Test site')).toBeNull()
    expect(splitWordmark('ai.rest2build.lol')).toBeNull()
    expect(splitWordmark('2build')).toBeNull()
  })
})
