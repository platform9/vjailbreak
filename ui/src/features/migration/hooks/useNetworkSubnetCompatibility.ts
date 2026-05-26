import { useState, useEffect, useRef } from 'react'
import {
  checkNetworkSubnetCompatibility,
  CheckNetworkSubnetCompatibilityResponse
} from 'src/api/openstack-creds/openstackCreds'
import { OpenstackCreds, PCDNetworkInfo } from 'src/api/openstack-creds/model'
import { VmData } from 'src/features/migration/api/migration-templates/model'
import type { ResourceMap } from '../types'

interface UseNetworkSubnetCompatibilityParams {
  networkMappings?: ResourceMap[]
  openstackCredentials?: OpenstackCreds
  selectedVMs: VmData[]
  networkIPsMap: Map<string, string[]>
  openstackNetworks: PCDNetworkInfo[]
}

export function useNetworkSubnetCompatibility({
  networkMappings,
  openstackCredentials,
  selectedVMs,
  networkIPsMap,
  openstackNetworks
}: UseNetworkSubnetCompatibilityParams): Record<string, string> {
  const [subnetWarnings, setSubnetWarnings] = useState<Record<string, string>>({})
  const prevMappingsRef = useRef<string>('')
  const debounceTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const apiCacheRef = useRef<Map<string, CheckNetworkSubnetCompatibilityResponse>>(new Map())
  const prevCredNameRef = useRef<string | undefined>(undefined)

  useEffect(() => {
    const completeMappings = (networkMappings || []).filter((m) => m.source && m.target)

    const mappingsKey = completeMappings.map((m) => `${m.source}|${m.target}`).join(',')
    if (mappingsKey === prevMappingsRef.current) return
    prevMappingsRef.current = mappingsKey

    if (!openstackCredentials || completeMappings.length === 0 || selectedVMs.length === 0) {
      setSubnetWarnings({})
      return
    }

    const credName = openstackCredentials.metadata.name
    if (credName !== prevCredNameRef.current) {
      apiCacheRef.current.clear()
      prevCredNameRef.current = credName
    }

    const credsNamespace = openstackCredentials.metadata.namespace

    if (debounceTimerRef.current) clearTimeout(debounceTimerRef.current)

    debounceTimerRef.current = setTimeout(async () => {
      const nextWarnings: Record<string, string> = {}

      await Promise.all(
        completeMappings.map(async (mapping) => {
          const ips = networkIPsMap.get(mapping.source) ?? []
          if (ips.length === 0) return

          const isL2Network = openstackNetworks.some(
            (n) =>
              n.name === mapping.target && Array.isArray(n.tags) && n.tags.includes('simple_network')
          )
          if (isL2Network) return

          const cacheKey = `${mapping.target}|${[...ips].sort().join(',')}`
          const cached = apiCacheRef.current.get(cacheKey)

          try {
            const result =
              cached ??
              (await checkNetworkSubnetCompatibility({
                ips,
                network_name: mapping.target,
                creds_name: credName,
                creds_namespace: credsNamespace
              }))

            if (!cached) apiCacheRef.current.set(cacheKey, result)

            if (!result.all_compatible) {
              const incompatibleIPs = result.results
                .filter((r) => !r.is_compatible)
                .map((r) => r.ip)
              const cidrList =
                result.subnet_cidrs?.length > 0 ? ` (${result.subnet_cidrs.join(', ')})` : ''
              nextWarnings[mapping.source] =
                `${incompatibleIPs.length} VM IP address(es) [${incompatibleIPs.join(', ')}] do not lie within the subnet of destination network ${mapping.target} ${cidrList}. ` +
                `Ensure fallback to DHCP is enabled, otherwise it may lead to migration failures`
            }
          } catch {
          }
        })
      )

      setSubnetWarnings(nextWarnings)
    }, 350)

    return () => {
      if (debounceTimerRef.current) clearTimeout(debounceTimerRef.current)
    }
  }, [networkMappings, openstackCredentials, selectedVMs, networkIPsMap, openstackNetworks])

  return subnetWarnings
}
