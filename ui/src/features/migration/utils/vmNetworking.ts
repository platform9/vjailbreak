interface VmLike {
  networks?: string[]
  networkInterfaces?: unknown[]
}

interface VmIpLike {
  assignedIPs?: string
  ipAddress?: string
  networkInterfaces?: { ipAddress?: string[] | unknown }[]
}

export const vmHasInterface = (vm: VmLike): boolean =>
  Boolean(
    (vm.networks && vm.networks.length > 0) ||
    (vm.networkInterfaces && vm.networkInterfaces.length > 0)
  )

export const vmHasIp = (vm: VmIpLike): boolean => {
  if (vm.assignedIPs && vm.assignedIPs.trim() !== '') return true
  if (vm.ipAddress && vm.ipAddress !== '—' && vm.ipAddress.trim() !== '') return true
  return Boolean(
    vm.networkInterfaces?.some(
      (nic) =>
        Array.isArray(nic.ipAddress) &&
        (nic.ipAddress as string[]).some((ip) => ip && ip.trim() !== '')
    )
  )
}
