import type { VmData } from 'src/features/migration/api/migration-templates/model'

export const buildAssignedIPsPerVM = (
  vms: VmData[] | undefined
): Record<string, string> | undefined => {
  if (!vms || vms.length === 0) return undefined

  const assigned: Record<string, string> = {}
  vms.forEach((vm) => {
    if (vm.assignedIPs && vm.assignedIPs.trim() !== '') {
      assigned[vm.vmKey || vm.name] = vm.assignedIPs
    }
  })

  return Object.keys(assigned).length > 0 ? assigned : undefined
}

export type NetworkOverride = {
  interfaceIndex: number
  preserveIP: boolean
  preserveMAC: boolean
}

export const buildNetworkOverridesPerVM = (
  vms: VmData[] | undefined
): Record<string, NetworkOverride[]> | undefined => {
  if (!vms || vms.length === 0) return undefined

  const overrides: Record<string, NetworkOverride[]> = {}

  vms.forEach((vm) => {
    const preserveIp = vm.preserveIp || {}
    const preserveMac = vm.preserveMac || {}

    const indices = new Set<string>([...Object.keys(preserveIp), ...Object.keys(preserveMac)])
    if (indices.size === 0) return

    overrides[vm.vmKey || vm.name] = Array.from(indices)
      .map((indexStr) => {
        const interfaceIndex = Number(indexStr)
        const ipFlag = (preserveIp as Record<number, boolean>)[interfaceIndex]
        const macFlag = (preserveMac as Record<number, boolean>)[interfaceIndex]
        return {
          interfaceIndex,
          preserveIP: ipFlag !== false,
          preserveMAC: macFlag !== false
        }
      })
      .sort((a, b) => a.interfaceIndex - b.interfaceIndex)
  })

  return Object.keys(overrides).length > 0 ? overrides : undefined
}
