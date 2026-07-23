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

/**
 * Signature used to decide whether a recompute is needed. Must change whenever
 * either the network mappings OR the per-network IP data changes — keying off
 * mappings alone misses IP edits/clears made after a mapping is created, which
 * leaves the subnet-mismatch warning stuck showing the pre-edit IP.
 */
export function computeSubnetCheckSignature(
  mappings: Array<{ source?: string; target?: string }>,
  networkIPsMap: Map<string, string[]>
): string {
  const mappingsKey = mappings.map((m) => `${m.source}|${m.target}`).join(',')
  const ipsKey = mappings
    .map((m) => `${m.source}:${[...(networkIPsMap.get(m.source ?? '') ?? [])].sort().join(',')}`)
    .join('|')
  return `${mappingsKey}::${ipsKey}`
}

export function useNetworkSubnetCompatibility({
  networkMappings,
  openstackCredentials,
  selectedVMs,
  networkIPsMap,
  openstackNetworks
}: UseNetworkSubnetCompatibilityParams): Record<string, string> {
  const [subnetWarnings, setSubnetWarnings] = useState<Record<string, string>>({})
  const prevSignatureRef = useRef<string>('')
  const debounceTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const apiCacheRef = useRef<Map<string, CheckNetworkSubnetCompatibilityResponse>>(new Map())
  const prevCredNameRef = useRef<string | undefined>(undefined)
  // Bumped on every recompute that actually starts. The check-compatibility API call
  // can take over a second — if the user edits/clears an IP while an older call for
  // the pre-edit IP is still in flight, clearTimeout only cancels the *timer*, not an
  // already-firing async body. Without this guard the stale call lands after the new
  // (correct) result and silently overwrites it back to the pre-edit warning text.
  const requestIdRef = useRef(0)

  useEffect(() => {
    const completeMappings = (networkMappings || []).filter((m) => m.source && m.target)

    const signature = computeSubnetCheckSignature(completeMappings, networkIPsMap)
    if (signature === prevSignatureRef.current) return
    prevSignatureRef.current = signature
    const requestId = ++requestIdRef.current

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
                `${incompatibleIPs.length} IP address(es) of the selected VMs [${incompatibleIPs.join(', ')}] do not lie within the subnet of destination network ${mapping.target} ${cidrList}. ` +
                `Ensure fallback to DHCP is enabled, otherwise it may lead to migration failures`
            }
          } catch {
          }
        })
      )

      // A newer recompute has since started (e.g. the user edited the IP again while
      // this one's API call was in flight) — drop this stale result instead of
      // clobbering the newer one.
      if (requestIdRef.current !== requestId) return
      setSubnetWarnings(nextWarnings)
    }, 350)

    return () => {
      if (debounceTimerRef.current) clearTimeout(debounceTimerRef.current)
    }
  }, [networkMappings, openstackCredentials, selectedVMs, networkIPsMap, openstackNetworks])

  return subnetWarnings
}
