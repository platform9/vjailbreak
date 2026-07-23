import { describe, expect, it } from 'vitest'
import { computeSubnetCheckSignature } from './useNetworkSubnetCompatibility'

describe('computeSubnetCheckSignature', () => {
  it('returns a stable signature for the same mappings and IPs', () => {
    const mappings = [{ source: 'VM Network', target: 'secondnetwork' }]
    const ipsMap = new Map([['VM Network', ['10.96.10.169']]])
    expect(computeSubnetCheckSignature(mappings, ipsMap)).toBe(
      computeSubnetCheckSignature(mappings, ipsMap)
    )
  })

  it('changes when a VM IP is cleared after the mapping is made (mappings unchanged)', () => {
    const mappings = [{ source: 'VM Network', target: 'secondnetwork' }]
    const before = computeSubnetCheckSignature(mappings, new Map([['VM Network', ['10.96.10.169']]]))
    const after = computeSubnetCheckSignature(mappings, new Map([['VM Network', []]]))
    expect(before).not.toBe(after)
  })

  it('changes when a VM IP is edited to a different address (mappings unchanged)', () => {
    const mappings = [{ source: 'VM Network', target: 'secondnetwork' }]
    const before = computeSubnetCheckSignature(mappings, new Map([['VM Network', ['10.96.10.169']]]))
    const after = computeSubnetCheckSignature(mappings, new Map([['VM Network', ['192.168.0.50']]]))
    expect(before).not.toBe(after)
  })

  it('changes when the network mapping target changes (IPs unchanged)', () => {
    const ipsMap = new Map([['VM Network', ['10.96.10.169']]])
    const before = computeSubnetCheckSignature(
      [{ source: 'VM Network', target: 'secondnetwork' }],
      ipsMap
    )
    const after = computeSubnetCheckSignature(
      [{ source: 'VM Network', target: 'thirdnetwork' }],
      ipsMap
    )
    expect(before).not.toBe(after)
  })

  it('is insensitive to IP ordering within a network', () => {
    const mappings = [{ source: 'VM Network', target: 'secondnetwork' }]
    const a = computeSubnetCheckSignature(
      mappings,
      new Map([['VM Network', ['10.0.0.1', '10.0.0.2']]])
    )
    const b = computeSubnetCheckSignature(
      mappings,
      new Map([['VM Network', ['10.0.0.2', '10.0.0.1']]])
    )
    expect(a).toBe(b)
  })

  it('handles multiple mappings independently', () => {
    const mappings = [
      { source: 'VM Network', target: 'secondnetwork' },
      { source: 'Mgmt', target: 'mgmt-net' }
    ]
    const before = computeSubnetCheckSignature(
      mappings,
      new Map([
        ['VM Network', ['10.96.10.169']],
        ['Mgmt', ['10.1.1.5']]
      ])
    )
    const after = computeSubnetCheckSignature(
      mappings,
      new Map([
        ['VM Network', ['10.96.10.169']],
        ['Mgmt', []]
      ])
    )
    expect(before).not.toBe(after)
  })

  it('returns a deterministic empty-state signature for no mappings', () => {
    expect(computeSubnetCheckSignature([], new Map())).toBe(computeSubnetCheckSignature([], new Map()))
  })

  it('treats a network with no IP entry the same as an empty IP list', () => {
    const mappings = [{ source: 'VM Network', target: 'secondnetwork' }]
    const withEmptyEntry = computeSubnetCheckSignature(mappings, new Map([['VM Network', []]]))
    const withNoEntry = computeSubnetCheckSignature(mappings, new Map())
    expect(withEmptyEntry).toBe(withNoEntry)
  })
})
