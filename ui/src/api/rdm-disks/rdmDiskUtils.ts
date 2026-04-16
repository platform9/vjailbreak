import { getRdmDisksList } from './rdmDisks'
import { RdmDisk } from './model'
import { VMwareMachine } from '../vmware-machines/model'
import { VmData } from '../migration-templates/model'

/**
 * Check if a VM has shared RDM disks
 */
export const hasSharedRdmDisks = (vm: VMwareMachine): boolean => {
  return !!(vm.spec.vms.rdmDisks && vm.spec.vms.rdmDisks.length > 0)
}

/**
 * Fetch all RDM disks and build a map for quick lookup
 */
export const fetchRdmDisksMap = async (namespace?: string): Promise<Map<string, RdmDisk>> => {
  try {
    const rdmDisks = await getRdmDisksList(namespace)
    const rdmDisksMap = new Map<string, RdmDisk>()

    rdmDisks.forEach((disk) => {
      rdmDisksMap.set(disk.metadata.name, disk)
    })

    return rdmDisksMap
  } catch (error) {
    console.error('Failed to fetch RDM disks:', error)
    return new Map()
  }
}

/**
 * Get RDM dependencies for a VM (other VMs that share RDM disks)
 */
export const getRdmDependencies = (
  vm: VMwareMachine,
  rdmDisksMap: Map<string, RdmDisk>,
  allMachines?: VMwareMachine[]
): string[] => {
  if (!vm.spec.vms.rdmDisks || vm.spec.vms.rdmDisks.length === 0) {
    return []
  }

  const dependentNames = new Set<string>()

  vm.spec.vms.rdmDisks.forEach((rdmDiskName) => {
    const rdmDisk = rdmDisksMap.get(rdmDiskName)
    if (rdmDisk && rdmDisk.spec.ownerVMs) {
      rdmDisk.spec.ownerVMs.forEach((ownerVm) => {
        if (ownerVm !== vm.spec.vms.name) {
          dependentNames.add(ownerVm)
        }
      })
    }
  })

  if (allMachines) {
    return Array.from(dependentNames).map((depName) => {
      const match = allMachines.find((m) => m.spec.vms.name === depName)
      if (match && match.spec.vms.vmid) {
        return `${match.spec.vms.name}-${match.spec.vms.vmid.replace(/^vm-/, '')}`
      }
      return depName
    })
  }

  return Array.from(dependentNames)
}

/**
 * Enhanced mapToVmData function that includes RDM information
 */
export const mapToVmDataWithRdm = (
  machines: VMwareMachine[],
  rdmDisksMap: Map<string, RdmDisk>
): VmData[] => {
  return machines.map((machine) => {
    const hasSharedRdm = hasSharedRdmDisks(machine)
    const rdmDisks = machine.spec.vms.rdmDisks || []
    const rdmDependencies = getRdmDependencies(machine, rdmDisksMap, machines)
    const vmKey = machine.spec.vms.vmid
      ? `${machine.spec.vms.name}-${machine.spec.vms.vmid.replace(/^vm-/, '')}`
      : machine.spec.vms.name

    return {
      id: vmKey,
      name: machine.spec.vms.name,
      vmid: machine.spec.vms.vmid,
      vmKey,
      vmState: machine.status.powerState === 'running' ? 'running' : 'stopped',
      ipAddress: machine.spec.vms.ipAddress,
      networks: machine.spec.vms.networks || [],
      datastores: machine.spec.vms.datastores || [],
      memory: machine.spec.vms.memory,
      cpuCount: machine.spec.vms.cpu,
      isMigrated: machine.status.migrated,
      disks: machine.spec.vms.disks || [],
      targetFlavorId: machine.spec.targetFlavorId,
      labels: machine.metadata.labels,
      osFamily: machine.spec.vms.osFamily,
      esxHost: machine.metadata?.labels?.[`vjailbreak.k8s.pf9.io/esxi-name`] || '',
      vmWareMachineName: machine.metadata.name,
      networkInterfaces: machine.spec.vms.networkInterfaces?.map((nic) => ({
        mac: nic.mac,
        network: nic.network,
        ipAddress: nic.ipAddress
      })),
      // RDM-related properties
      rdmDisks,
      hasSharedRdm,
      rdmDependencies,
      useGPU: machine.spec.vms.useGPU
    }
  })
}

/**
 * Get all VMs that need to be selected together due to RDM dependencies
 */
export const getRdmRequiredSelections = (
  selectedVmIds: string[],
  allVms: VmData[]
): {
  requiredVms: string[]
  missingVms: string[]
  rdmGroups: Record<string, string[]>
} => {
  const requiredVms = new Set<string>()
  const rdmGroups: Record<string, string[]> = {}

  // Add initially selected VMs
  selectedVmIds.forEach((vmId) => requiredVms.add(vmId))

  // Find all VMs that have RDM dependencies
  const vmsWithRdm = allVms.filter(
    (vm) => vm.hasSharedRdm && vm.rdmDependencies && vm.rdmDependencies.length > 0
  )

  // Build RDM groups
  vmsWithRdm.forEach((vm) => {
    if (selectedVmIds.includes(vm.id)) {
      const groupKey = `rdm-group-${vm.id}`
      const groupVms = [vm.id, ...(vm.rdmDependencies || [])]
      rdmGroups[groupKey] = groupVms

      // Add all VMs in this RDM group to required selections
      groupVms.forEach((vmId) => requiredVms.add(vmId))
    }
  })

  const allRequiredVms = Array.from(requiredVms)
  const missingVms = allRequiredVms.filter((vmId) => !selectedVmIds.includes(vmId))

  return {
    requiredVms: allRequiredVms,
    missingVms,
    rdmGroups
  }
}
