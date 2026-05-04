import { useMemo } from 'react'
import { vmHasInterface } from 'src/features/migration/utils/vmNetworking'

interface ResourceMap {
  source: string
  target: string
}

interface NetworkMappingVmShape {
  networks?: string[]
  networkInterfaces?: unknown[]
}

interface NetworkMappingValidationProps {
  selectedVMs: NetworkMappingVmShape[]
  networkMappings: ResourceMap[]
  availableVmwareNetworks: string[]
}

interface NetworkMappingValidationResult {
  /** True when at least one selected VM has an interface AND there are source networks to map. */
  required: boolean
  /** Source VMware networks that have not been mapped yet. */
  unmapped: string[]
  /** True when mapping is required but at least one source network is unmapped. */
  hasError: boolean
  /** True when mapping is either not required (no NICs) or all source networks are mapped. */
  isComplete: boolean
}

/**
 * Centralised validation for the Network Mapping step of the Create Migration
 * Plan flows. Returns `required: false` when none of the selected VMs have an
 * interface, allowing the UI to skip the mapping step instead of trapping the
 * user behind an unsatisfiable "All networks must be mapped" gate.
 *
 * Mirrors the shape of useRdmConfigValidation so call-sites can read uniformly.
 */
export const useNetworkMappingValidation = ({
  selectedVMs,
  networkMappings,
  availableVmwareNetworks
}: NetworkMappingValidationProps): NetworkMappingValidationResult =>
  useMemo(() => {
    const vmsWithInterfaces = selectedVMs.filter(vmHasInterface)
    const required =
      vmsWithInterfaces.length > 0 && availableVmwareNetworks.length > 0

    const unmapped = availableVmwareNetworks.filter(
      (network) => !networkMappings.some((m) => m.source === network)
    )

    return {
      required,
      unmapped,
      hasError: required && unmapped.length > 0,
      isComplete: !required || unmapped.length === 0
    }
  }, [selectedVMs, networkMappings, availableVmwareNetworks])
