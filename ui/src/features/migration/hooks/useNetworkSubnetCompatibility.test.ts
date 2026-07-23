import { describe, expect, it, vi } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { computeSubnetCheckSignature, useNetworkSubnetCompatibility } from './useNetworkSubnetCompatibility'
import type { OpenstackCreds } from 'src/api/openstack-creds/model'
import type { VmData } from 'src/features/migration/api/migration-templates/model'

vi.mock('src/api/openstack-creds/openstackCreds', () => ({
  checkNetworkSubnetCompatibility: vi.fn()
}))

import { checkNetworkSubnetCompatibility } from 'src/api/openstack-creds/openstackCreds'

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

describe('useNetworkSubnetCompatibility — stale response race', () => {
  // checkNetworkSubnetCompatibility can take over a second in practice. If the user
  // clears the IP while that call for the pre-edit IP is still in flight, clearTimeout
  // only cancels the pending *timer* — it can't cancel an already-running async body.
  // Without the requestId guard, that stale call lands after the correct (cleared)
  // result and silently reinstates the pre-edit warning.
  it('does not let a slow, superseded API call overwrite a newer result', async () => {
    let resolveFirstCall!: (value: unknown) => void
    const firstCallPromise = new Promise((resolve) => {
      resolveFirstCall = resolve
    })
    vi.mocked(checkNetworkSubnetCompatibility).mockImplementationOnce(
      () => firstCallPromise as ReturnType<typeof checkNetworkSubnetCompatibility>
    )

    const openstackCredentials = {
      metadata: { name: 'creds-1', namespace: 'default' }
    } as unknown as OpenstackCreds
    const networkMappings = [{ source: 'VM Network', target: 'secondnetwork' }]
    const selectedVMs: VmData[] = [{ id: 'vm-1', name: 'vm-1', datastores: [] }]

    const { result, rerender } = renderHook(
      ({ networkIPsMap }) =>
        useNetworkSubnetCompatibility({
          networkMappings,
          openstackCredentials,
          selectedVMs,
          networkIPsMap,
          openstackNetworks: []
        }),
      { initialProps: { networkIPsMap: new Map([['VM Network', ['10.96.9.11']]]) } }
    )

    // Let the 350ms debounce fire so the (slow, still-unresolved) first API call starts.
    await new Promise((r) => setTimeout(r, 400))

    // User clears the IP — ips.length===0 for this mapping so the new recompute
    // resolves without an API call, well before the stale first call finishes.
    rerender({ networkIPsMap: new Map([['VM Network', []]]) })
    await waitFor(() => expect(result.current['VM Network']).toBeUndefined(), { timeout: 1000 })

    // The slow original call for the pre-edit IP finally resolves as incompatible.
    resolveFirstCall({
      all_compatible: false,
      results: [{ ip: '10.96.9.11', is_compatible: false }],
      subnet_cidrs: ['192.168.0.0/24']
    })
    await new Promise((r) => setTimeout(r, 50))

    expect(result.current['VM Network']).toBeUndefined()
  }, 10000)
})
