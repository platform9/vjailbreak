import { useMemo } from 'react'
import { VmData } from 'src/features/migration/api/migration-templates/model'

export function useNetworkIPsMap(selectedVMs: VmData[]): Map<string, string[]> {
  return useMemo(() => {
    const map = new Map<string, string[]>()
    for (const vm of selectedVMs) {
      for (const network of vm.networks || []) {
        const ips = map.get(network) ?? []

        if (vm.networkInterfaces && vm.networkInterfaces.length > 0) {
          const matchingNics = vm.networkInterfaces.filter((nic) => nic.network === network)
          const nicsToUse = matchingNics.length > 0 ? matchingNics : vm.networkInterfaces
          for (const nic of nicsToUse) {
            if (nic.preserveIP !== false && Array.isArray(nic.ipAddress)) {
              ips.push(...nic.ipAddress.filter((ip) => ip && ip.trim() !== ''))
            }
          }
        }

        if (vm.ipAddress && vm.ipAddress !== '—' && vm.ipAddress.trim()) {
          ips.push(...vm.ipAddress.split(',').map((ip) => ip.trim()).filter(Boolean))
        }

        if (ips.length > 0) map.set(network, ips)
      }
    }
    for (const [network, ips] of map) {
      map.set(network, [...new Set(ips)])
    }
    return map
  }, [selectedVMs])
}
