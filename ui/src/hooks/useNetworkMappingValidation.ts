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
  required: boolean
  unmapped: string[]
  hasError: boolean
  isComplete: boolean
}

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
