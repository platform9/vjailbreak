import { FormControl, FormHelperText, styled } from "@mui/material"
import { useEffect, useMemo } from "react"
import ResourceMapping from "../../components/forms/ResourceMapping"
import Step from "../../components/forms/Step"

const VmsSelectionStepContainer = styled("div")(({ theme }) => ({
  display: "grid",
  gridGap: theme.spacing(1),
}))

const FieldsContainer = styled("div")(({ theme }) => ({
  display: "grid",
  marginLeft: theme.spacing(6),
  gridGap: theme.spacing(2),
}))

export interface ResourceMap {
  source: string
  target: string
}

interface NetworkAndStorageMappingStepProps {
  vmwareNetworks: string[]
  vmWareStorage: string[]
  openstackNetworks: string[]
  openstackStorage: string[]
  params: {
    networkMappings?: ResourceMap[]
    storageMappings?: ResourceMap[]
  }
  onChange: (key: string) => (value: ResourceMap[]) => void
  networkMappingError?: string
  storageMappingError?: string
}

export default function NetworkAndStorageMappingStep({
  vmwareNetworks = [],
  vmWareStorage = [],
  openstackNetworks = [],
  openstackStorage = [],
  params,
  onChange,
  networkMappingError,
  storageMappingError,
}: NetworkAndStorageMappingStepProps) {
  // Filter out any mappings that don't match the available networks/storage
  const filteredNetworkMappings = useMemo(
    () =>
      (params.networkMappings || []).filter(
        (mapping) =>
          vmwareNetworks.includes(mapping.source) &&
          openstackNetworks.includes(mapping.target)
      ),
    [params.networkMappings, vmwareNetworks, openstackNetworks]
  )

  const filteredStorageMappings = useMemo(
    () =>
      (params.storageMappings || []).filter(
        (mapping) =>
          vmWareStorage.includes(mapping.source) &&
          openstackStorage.includes(mapping.target)
      ),
    [params.storageMappings, vmWareStorage, openstackStorage]
  )

  useEffect(() => {
    if (filteredNetworkMappings.length !== params.networkMappings?.length) {
      onChange("networkMappings")(filteredNetworkMappings)
    }
  }, [filteredNetworkMappings, onChange, params.networkMappings])

  useEffect(() => {
    if (filteredStorageMappings.length !== params.storageMappings?.length) {
      onChange("storageMappings")(filteredStorageMappings)
    }
  }, [filteredStorageMappings, onChange, params.storageMappings])

  return (
    <VmsSelectionStepContainer>
      <Step stepNumber="3" label="Network and Storage Mapping" />
      <FieldsContainer>
        <FormControl error={!!networkMappingError}>
          <ResourceMapping
            label="Map Networks"
            sourceItems={vmwareNetworks}
            targetItems={openstackNetworks}
            sourceLabel="VMware Network"
            targetLabel="Openstack Network"
            values={params.networkMappings || []}
            onChange={(value) => onChange("networkMappings")(value)}
          />
          {networkMappingError && (
            <FormHelperText error>{networkMappingError}</FormHelperText>
          )}
        </FormControl>
        <FormControl error={!!storageMappingError}>
          <ResourceMapping
            label="Map Storage"
            sourceItems={vmWareStorage}
            targetItems={openstackStorage}
            sourceLabel="VMWare Datastore"
            targetLabel="OpenStack VolumeType"
            values={params.storageMappings || []}
            onChange={(value) => onChange("storageMappings")(value)}
          />
          {storageMappingError && (
            <FormHelperText error>{storageMappingError}</FormHelperText>
          )}
        </FormControl>
      </FieldsContainer>
    </VmsSelectionStepContainer>
  )
}
