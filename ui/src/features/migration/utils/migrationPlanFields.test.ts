import { describe, expect, it } from 'vitest'

import { buildAssignedIPsPerVM, buildNetworkOverridesPerVM } from './migrationPlanFields'
import type { VmData } from 'src/features/migration/api/migration-templates/model'

describe('migrationPlanFields', () => {
  describe('buildAssignedIPsPerVM', () => {
    it('returns undefined when no assigned IPs exist', () => {
      const vms: VmData[] = [{ name: 'vm1', assignedIPs: '' } as unknown as VmData]
      expect(buildAssignedIPsPerVM(vms)).toBeUndefined()
    })

    it('builds a map keyed by vm name', () => {
      const vms: VmData[] = [
        { name: 'vm1', assignedIPs: '10.0.0.1' } as unknown as VmData,
        { name: 'vm2', assignedIPs: '  ' } as unknown as VmData,
        { name: 'vm3', assignedIPs: '10.0.0.3' } as unknown as VmData
      ]

      expect(buildAssignedIPsPerVM(vms)).toEqual({
        vm1: '10.0.0.1',
        vm3: '10.0.0.3'
      })
    })
  })

  describe('buildNetworkOverridesPerVM', () => {
    it('returns undefined when no preserve flags exist', () => {
      const vms: VmData[] = [{ name: 'vm1' } as unknown as VmData]
      expect(buildNetworkOverridesPerVM(vms)).toBeUndefined()
    })

    it('creates sorted overrides and defaults preserve flags to true when undefined', () => {
      const vms: VmData[] = [
        {
          name: 'vm1',
          preserveIp: { 1: false },
          preserveMac: { 0: true }
        } as unknown as VmData
      ]

      expect(buildNetworkOverridesPerVM(vms)).toEqual({
        vm1: [
          { interfaceIndex: 0, preserveIP: true, preserveMAC: true },
          { interfaceIndex: 1, preserveIP: false, preserveMAC: true }
        ]
      })
    })
  })
})
