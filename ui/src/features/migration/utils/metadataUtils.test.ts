import { describe, expect, it } from 'vitest'
import {
  countVmSourceEntries,
  customMetadataToRecord,
  summarizeSourceEntries
} from './metadataUtils'

describe('customMetadataToRecord', () => {
  it('returns undefined for undefined or empty rows', () => {
    expect(customMetadataToRecord(undefined)).toBeUndefined()
    expect(customMetadataToRecord([])).toBeUndefined()
  })

  it('converts rows to a record', () => {
    expect(
      customMetadataToRecord([
        { key: 'wave', value: '2' },
        { key: 'migrated_by', value: 'vjailbreak' }
      ])
    ).toEqual({ wave: '2', migrated_by: 'vjailbreak' })
  })

  it('trims keys and values and drops blank keys', () => {
    expect(
      customMetadataToRecord([
        { key: '  wave ', value: ' 2 ' },
        { key: '   ', value: 'ignored' },
        { key: '', value: 'ignored' }
      ])
    ).toEqual({ wave: '2' })
  })

  it('returns undefined when all keys are blank', () => {
    expect(customMetadataToRecord([{ key: ' ', value: 'x' }])).toBeUndefined()
  })

  it('last duplicate key wins', () => {
    expect(
      customMetadataToRecord([
        { key: 'env', value: 'staging' },
        { key: 'env', value: 'production' }
      ])
    ).toEqual({ env: 'production' })
  })
})

describe('countVmSourceEntries', () => {
  it('counts tags and custom attributes together', () => {
    expect(
      countVmSourceEntries({
        tags: { env: 'production', tier: 'web' },
        customAttributes: { Owner: 'alice@corp.com' }
      })
    ).toBe(3)
  })

  it('returns 0 when both are missing', () => {
    expect(countVmSourceEntries({})).toBe(0)
  })
})

describe('summarizeSourceEntries', () => {
  it('sums entries across VMs', () => {
    const result = summarizeSourceEntries([
      { tags: { env: 'production' }, customAttributes: { Owner: 'alice@corp.com' } },
      { tags: {}, customAttributes: {} },
      { customAttributes: { CostCenter: 'CC-1042' } }
    ])
    expect(result).toEqual({ vmCount: 3, entryCount: 3 })
  })

  it('handles empty VM list', () => {
    expect(summarizeSourceEntries([])).toEqual({ vmCount: 0, entryCount: 0 })
  })
})
