import { describe, expect, it } from 'vitest'
import { hasAnySubnetMismatch } from './subnetMismatch'

describe('hasAnySubnetMismatch', () => {
  it('returns false for undefined warnings', () => {
    expect(hasAnySubnetMismatch(undefined)).toBe(false)
  })

  it('returns false for empty warnings (no mismatch detected → do not block)', () => {
    expect(hasAnySubnetMismatch({})).toBe(false)
  })

  it('returns false when warning values are empty strings', () => {
    expect(hasAnySubnetMismatch({ 'VM Network': '', Mgmt: '   ' })).toBe(false)
  })

  it('returns true when a single network has a mismatch warning', () => {
    expect(
      hasAnySubnetMismatch({
        'VM Network': '2 IP address(es) of the selected VMs do not lie within the subnet of destination network'
      })
    ).toBe(true)
  })

  it('returns true when only one of several networks has a mismatch', () => {
    expect(
      hasAnySubnetMismatch({
        'VM Network': '',
        Mgmt: '1 IP address(es) of the selected VMs do not lie within the subnet of destination network'
      })
    ).toBe(true)
  })
})
