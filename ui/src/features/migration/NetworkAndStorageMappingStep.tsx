import { FormControl, FormHelperText, styled, Typography, Box } from "@mui/material"
import { useEffect, useMemo } from "react"
import ResourceMappingTable from "src/components/forms/ResourceMappingTableNew"
import Step from "../../components/forms/Step"
// import ResourceMapping from "../../components/forms/ResourceMapping"

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
  stepNumber?: string
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
  stepNumber = "3",
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

  // Calculate unmapped networks and storage
  const unmappedNetworks = useMemo(
    () => vmwareNetworks.filter(network =>
      !params.networkMappings?.some(mapping => mapping.source === network)
    ),
    [vmwareNetworks, params.networkMappings]
  );

  const unmappedStorage = useMemo(
    () => vmWareStorage.filter(storage =>
      !params.storageMappings?.some(mapping => mapping.source === storage)
    ),
    [vmWareStorage, params.storageMappings]
  );

  // Calculate completion status
  const networksFullyMapped = unmappedNetworks.length === 0 && vmwareNetworks.length > 0;
  const storageFullyMapped = unmappedStorage.length === 0 && vmWareStorage.length > 0;

  return (
    <VmsSelectionStepContainer>
      <Step stepNumber={stepNumber} label="Network and Storage Mapping" />
      <FieldsContainer>
        <FormControl error={!!networkMappingError}>
          <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 1 }}>
            <Typography variant="subtitle2">Map Networks</Typography>
            {networksFullyMapped ? (
              <Typography variant="body2" color="success.main">All networks mapped ✓</Typography>
            ) : (
              <Typography variant="body2" color="warning.main">
                {unmappedNetworks.length} of {vmwareNetworks.length} networks unmapped
              </Typography>
            )}
          </Box>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            Select source and target networks to automatically create mappings. All networks must be mapped to proceed.
          </Typography>
          <ResourceMappingTable
            sourceItems={vmwareNetworks}
            targetItems={openstackNetworks}
            sourceLabel="VMware Network"
            targetLabel="OpenStack Network"
            values={params.networkMappings || []}
            onChange={(value) => onChange("networkMappings")(value)}
            oneToManyMapping
          />
          {networkMappingError && (
            <FormHelperText error>{networkMappingError}</FormHelperText>
          )}
        </FormControl>
        <FormControl error={!!storageMappingError}>
          <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 1 }}>
            <Typography variant="subtitle2">Map Storage</Typography>
            {storageFullyMapped ? (
              <Typography variant="body2" color="success.main">All storage mapped ✓</Typography>
            ) : (
              <Typography variant="body2" color="warning.main">
                {unmappedStorage.length} of {vmWareStorage.length} storage devices unmapped
              </Typography>
            )}
          </Box>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            Select source and target storage to automatically create mappings. All storage devices must be mapped to proceed.
          </Typography>
          <ResourceMappingTable
            sourceItems={vmWareStorage}
            targetItems={openstackStorage}
            sourceLabel="VMware Datastore"
            targetLabel="OpenStack VolumeType"
            values={params.storageMappings || []}
            onChange={(value) => onChange("storageMappings")(value)}
            oneToManyMapping
          />
          {storageMappingError && (
            <FormHelperText error>{storageMappingError}</FormHelperText>
          )}
        </FormControl>
      </FieldsContainer>
    </VmsSelectionStepContainer>
  )
}
