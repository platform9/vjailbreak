import { useState, useEffect, useRef, useMemo } from 'react'
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

const DEBOUNCE_MS = 350

/**
 * Primitive signature of everything that should trigger a recompute: which
 * source->target mappings exist, and the current IPs behind each source
 * network. Built from primitives (not object refs) so the effect below re-fires
 * on real content changes only — an IP edited/cleared after a mapping already
 * exists changes this signature just like a new mapping would, so the
 * subnet-mismatch warning never gets stuck showing a pre-edit IP.
 */
export function computeSubnetCheckSignature(
  mappings: Array<{ source?: string; target?: string }>,
  networkIPsMap: Map<string, string[]>
): string {
  return mappings
    .map((m) => {
      const ips = [...(networkIPsMap.get(m.source ?? '') ?? [])].sort()
      return `${m.source}>${m.target}:${ips.join(',')}`
    })
    .join('|')
}

export function useNetworkSubnetCompatibility({
  networkMappings,
  openstackCredentials,
  selectedVMs,
  networkIPsMap,
  openstackNetworks
}: UseNetworkSubnetCompatibilityParams): Record<string, string> {
  const [subnetWarnings, setSubnetWarnings] = useState<Record<string, string>>({})
  // Bumped on every recompute that actually starts. The check-compatibility API call
  // can take over a second — if the user edits/clears an IP while an older call for
  // the pre-edit IP is still in flight, only the latest result should be allowed to
  // win, otherwise a stale response can land after the fresh one and clobber it.
  const requestIdRef = useRef(0)

  const completeMappings = useMemo(
    () => (networkMappings || []).filter((m) => m.source && m.target),
    [networkMappings]
  )

  const signature = useMemo(
    () => computeSubnetCheckSignature(completeMappings, networkIPsMap),
    [completeMappings, networkIPsMap]
  )

  const credName = openstackCredentials?.metadata.name
  const credsNamespace = openstackCredentials?.metadata.namespace

  const networksKey = useMemo(
    () =>
      openstackNetworks
        .map((n) => `${n.name}:${Array.isArray(n.tags) && n.tags.includes('simple_network') ? 1 : 0}`)
        .sort()
        .join(','),
    [openstackNetworks]
  )

  useEffect(() => {
    if (!credName || completeMappings.length === 0 || selectedVMs.length === 0) {
      setSubnetWarnings({})
      return
    }

    const requestId = ++requestIdRef.current

    const timer = setTimeout(async () => {
      const nextWarnings: Record<string, string> = {}

      await Promise.all(
        completeMappings.map(async (mapping) => {
          const ips = networkIPsMap.get(mapping.source ?? '') ?? []
          if (ips.length === 0) return

          const isL2Network = openstackNetworks.some(
            (n) =>
              n.name === mapping.target && Array.isArray(n.tags) && n.tags.includes('simple_network')
          )
          if (isL2Network) return

          try {
            const result: CheckNetworkSubnetCompatibilityResponse =
              await checkNetworkSubnetCompatibility({
                ips,
                network_name: mapping.target as string,
                creds_name: credName,
                creds_namespace: credsNamespace as string
              })

            if (!result.all_compatible) {
              const incompatibleIPs = result.results
                .filter((r) => !r.is_compatible)
                .map((r) => r.ip)
              const cidrList =
                result.subnet_cidrs?.length > 0 ? ` (${result.subnet_cidrs.join(', ')})` : ''
              nextWarnings[mapping.source as string] =
                `${incompatibleIPs.length} IP address(es) of the selected VMs [${incompatibleIPs.join(', ')}] do not lie within the subnet of destination network ${mapping.target} ${cidrList}. ` +
                `Ensure fallback to DHCP is enabled, otherwise it may lead to migration failures`
            }
          } catch {
            // Ignore transient API errors; the next recompute will retry.
          }
        })
      )

      // A newer recompute has since started (e.g. the user edited the IP again while
      // this one's API call was in flight) — drop this stale result instead of
      // clobbering the newer one.
      if (requestIdRef.current !== requestId) return
      setSubnetWarnings(nextWarnings)
    }, DEBOUNCE_MS)

    return () => clearTimeout(timer)
    // signature + networksKey capture every input that should drive a recompute as
    // stable primitives; completeMappings/networkIPsMap/openstackNetworks are read
    // from the closure but intentionally left out of the deps so the effect re-fires
    // on content changes rather than on every new array/object reference.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [signature, networksKey, credName, credsNamespace, selectedVMs.length])

  return subnetWarnings
}
