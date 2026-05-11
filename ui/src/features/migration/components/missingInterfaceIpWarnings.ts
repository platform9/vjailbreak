type NetworkInterfaceWithIp = {
  ipAddress?: string[] | string | null
}

type VmWithNetworkInterfaces = {
  id?: string
  name?: string
  powerState?: string
  vmState?: string
  networkInterfaces?: NetworkInterfaceWithIp[]
}

export type MissingInterfaceIpWarning = {
  vmName: string
}

const hasDiscoveredIp = (ipAddress?: string[] | string | null) => {
  if (Array.isArray(ipAddress)) {
    return ipAddress.some((ip) => Boolean(ip?.trim()))
  }

  return Boolean(ipAddress?.trim())
}

const isPoweredOnVm = (vm: VmWithNetworkInterfaces) => {
  return vm.powerState === 'powered-on' || vm.vmState === 'running'
}

export const getMissingInterfaceIpWarnings = (
  selectedVms: VmWithNetworkInterfaces[]
): MissingInterfaceIpWarning[] => {
  const vmNames = selectedVms
    .filter(isPoweredOnVm)
    .filter((vm) => {
      const interfaces = Array.isArray(vm.networkInterfaces) ? vm.networkInterfaces : []
      return interfaces.some((networkInterface) => !hasDiscoveredIp(networkInterface.ipAddress))
    })
    .map((vm) => vm.name || vm.id || 'Unknown VM')

  return Array.from(new Set(vmNames)).map((vmName) => ({ vmName }))
}
