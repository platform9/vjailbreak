import { describe, expect, it } from 'vitest'

import { getUnmappedCount, isMappingComplete } from './mappings'

describe('mappings utils', () => {
  it('treats empty sources as complete', () => {
    expect(isMappingComplete([], [])).toBe(true)
    expect(getUnmappedCount([], [])).toBe(0)
  })

  it('detects complete mapping', () => {
    const sources = ['a', 'b']
    const mappings = [
      { source: 'a', target: 'x' },
      { source: 'b', target: 'y' }
    ]

    expect(isMappingComplete(sources, mappings)).toBe(true)
    expect(getUnmappedCount(sources, mappings)).toBe(0)
  })

  it('detects incomplete mapping', () => {
    const sources = ['a', 'b', 'c']
    const mappings = [{ source: 'a', target: 'x' }]

    expect(isMappingComplete(sources, mappings)).toBe(false)
    expect(getUnmappedCount(sources, mappings)).toBe(2)
  })
})
