type NetworkInterfaceWithIp = {
  mac?: string
  network?: string
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
  key: string
  vmName: string
  macAddress: string
  networkName?: string
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
  return selectedVms.filter(isPoweredOnVm).flatMap((vm) => {
    const vmName = vm.name || vm.id || 'Unknown VM'
    const interfaces = Array.isArray(vm.networkInterfaces) ? vm.networkInterfaces : []

    return interfaces
      .map((networkInterface, interfaceIndex) => ({ networkInterface, interfaceIndex }))
      .filter(({ networkInterface }) => !hasDiscoveredIp(networkInterface.ipAddress))
      .map(({ networkInterface, interfaceIndex }) => {
        const macAddress = networkInterface.mac || 'MAC unavailable'

        return {
          key: `${vm.id || vmName}-${interfaceIndex}-${macAddress}`,
          vmName,
          macAddress,
          networkName: networkInterface.network
        }
      })
  })
}
