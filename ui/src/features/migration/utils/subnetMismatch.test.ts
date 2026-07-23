import { describe, expect, it } from 'vitest'
import { hasAnySubnetMismatch, hasAnyPreserveIpDisabled } from './subnetMismatch'

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

describe('hasAnyPreserveIpDisabled', () => {
  it('returns false for undefined vms', () => {
    expect(hasAnyPreserveIpDisabled(undefined)).toBe(false)
  })

  it('returns false for an empty vm list', () => {
    expect(hasAnyPreserveIpDisabled([])).toBe(false)
  })

  it('returns false when a VM never had an IP (no networkInterfaces, no preserveIp)', () => {
    expect(hasAnyPreserveIpDisabled([{ networkInterfaces: [] }])).toBe(false)
  })

  it('returns false when networkInterfaces are untouched (preserveIP undefined)', () => {
    expect(
      hasAnyPreserveIpDisabled([{ networkInterfaces: [{ preserveIP: undefined }] }])
    ).toBe(false)
  })

  it('returns true when a networkInterface has preserveIP explicitly false (IP cleared)', () => {
    expect(hasAnyPreserveIpDisabled([{ networkInterfaces: [{ preserveIP: false }] }])).toBe(true)
  })

  it('returns true when only one of several VMs has preserveIP disabled', () => {
    expect(
      hasAnyPreserveIpDisabled([
        { networkInterfaces: [{ preserveIP: true }] },
        { networkInterfaces: [{ preserveIP: false }] }
      ])
    ).toBe(true)
  })

  it('returns true when only one of several interfaces on a VM has preserveIP disabled', () => {
    expect(
      hasAnyPreserveIpDisabled([
        { networkInterfaces: [{ preserveIP: true }, { preserveIP: false }] }
      ])
    ).toBe(true)
  })

  it('falls back to the top-level preserveIp override map when there are no networkInterfaces', () => {
    expect(hasAnyPreserveIpDisabled([{ preserveIp: { 0: false } }])).toBe(true)
  })

  it('returns false for a top-level preserveIp map with no disabled entries', () => {
    expect(hasAnyPreserveIpDisabled([{ preserveIp: { 0: true } }])).toBe(false)
  })

  it('prefers the top-level preserveIp override over the nic value at the same index', () => {
    expect(
      hasAnyPreserveIpDisabled([
        { preserveIp: { 0: false }, networkInterfaces: [{ preserveIP: true }] }
      ])
    ).toBe(true)
    expect(
      hasAnyPreserveIpDisabled([
        { preserveIp: { 0: true }, networkInterfaces: [{ preserveIP: false }] }
      ])
    ).toBe(false)
  })
})
