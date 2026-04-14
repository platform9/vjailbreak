import { describe, expect, it } from 'vitest'

import { validateSelectedVmsOsAssigned } from './vmValidation'
import type { VmData } from 'src/features/migration/api/migration-templates/model'

describe('validateSelectedVmsOsAssigned', () => {
  it('returns no error for empty list', () => {
    expect(validateSelectedVmsOsAssigned(undefined)).toEqual({ hasError: false, errorMessage: '' })
    expect(validateSelectedVmsOsAssigned([])).toEqual({ hasError: false, errorMessage: '' })
  })

  it('errors when OS is missing/unknown', () => {
    const vms: VmData[] = [
      { name: 'vm1', vmState: 'stopped', osFamily: 'Unknown' } as unknown as VmData,
      { name: 'vm2', vmState: 'running', osFamily: '' } as unknown as VmData
    ]

    const result = validateSelectedVmsOsAssigned(vms)
    expect(result.hasError).toBe(true)
    expect(result.errorMessage).toContain('Cannot proceed with migration')
    expect(result.errorMessage).toContain('2 VM')
  })

  it('returns no error when OS is present', () => {
    const vms: VmData[] = [
      { name: 'vm1', vmState: 'stopped', osFamily: 'Linux' } as unknown as VmData,
      { name: 'vm2', vmState: 'running', osFamily: 'Windows' } as unknown as VmData
    ]

    expect(validateSelectedVmsOsAssigned(vms)).toEqual({ hasError: false, errorMessage: '' })
  })
})
