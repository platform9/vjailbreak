import { FormControl, FormHelperText, styled, Typography, Box, RadioGroup, FormControlLabel, Radio, Alert } from "@mui/material"
import { useEffect, useMemo } from "react"
import ResourceMappingTable from "src/components/forms/ResourceMappingTableNew"
import Step from "../../components/forms/Step"
import { STORAGE_COPY_METHOD_OPTIONS } from "./constants"
import { useArrayCredsQuery } from "src/hooks/api/useArrayCredsQuery"

const VmsSelectionStepContainer = styled('div')(({ theme }) => ({
  display: 'grid',
  gridGap: theme.spacing(1)
}))

const FieldsContainer = styled('div')(({ theme }) => ({
  display: 'grid',
  marginLeft: theme.spacing(6),
  gridGap: theme.spacing(2)
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
    storageCopyMethod?: string
    arrayCredsMappings?: ResourceMap[]
  }
  onChange: (key: string) => (value: any) => void
  networkMappingError?: string
  storageMappingError?: string
  stepNumber?: string
  loading?: boolean
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
  stepNumber = '3',
  loading = false
}: NetworkAndStorageMappingStepProps) {
  const { data: arrayCredsList = [], isLoading: arrayCredsLoading } = useArrayCredsQuery()
  
  // Filter to only show successfully validated ArrayCreds
  const validatedArrayCreds = useMemo(
    () => arrayCredsList.filter(ac => ac.status?.arrayValidationStatus === 'Succeeded'),
    [arrayCredsList]
  )
  
  const storageCopyMethod = params.storageCopyMethod || 'normal'
  
  // Initialize storageCopyMethod if not set
  useEffect(() => {
    if (!params.storageCopyMethod) {
      onChange('storageCopyMethod')('normal')
    }
  }, [params.storageCopyMethod, onChange])
  
  // Filter out any mappings that don't match the available networks/storage
  const filteredNetworkMappings = useMemo(
    () =>
      (params.networkMappings || []).filter(
        (mapping) =>
          vmwareNetworks.includes(mapping.source) && openstackNetworks.includes(mapping.target)
      ),
    [params.networkMappings, vmwareNetworks, openstackNetworks]
  )

  const filteredStorageMappings = useMemo(
    () =>
      (params.storageMappings || []).filter(
        (mapping) =>
          vmWareStorage.includes(mapping.source) && openstackStorage.includes(mapping.target)
      ),
    [params.storageMappings, vmWareStorage, openstackStorage]
  )

  useEffect(() => {
    if (filteredNetworkMappings.length !== params.networkMappings?.length) {
      onChange('networkMappings')(filteredNetworkMappings)
    }
  }, [filteredNetworkMappings, onChange, params.networkMappings])

  useEffect(() => {
    if (filteredStorageMappings.length !== params.storageMappings?.length) {
      onChange('storageMappings')(filteredStorageMappings)
    }
  }, [filteredStorageMappings, onChange, params.storageMappings])

  // Calculate unmapped networks and storage
  const unmappedNetworks = useMemo(
    () =>
      vmwareNetworks.filter(
        (network) => !params.networkMappings?.some((mapping) => mapping.source === network)
      ),
    [vmwareNetworks, params.networkMappings]
  )

  // Auto-map datastores to ArrayCreds when vendor-based is selected
  useEffect(() => {
    if (storageCopyMethod === 'vendor-based' && vmWareStorage.length > 0 && validatedArrayCreds.length > 0) {
      const autoMappings: ResourceMap[] = []
      
      vmWareStorage.forEach(datastore => {
        // Find ArrayCreds that has this datastore in its dataStore array
        const matchingArrayCreds = validatedArrayCreds.find(ac => 
          ac.status?.dataStore?.some(ds => ds.name === datastore)
        )
        
        if (matchingArrayCreds) {
          autoMappings.push({
            source: datastore,
            target: matchingArrayCreds.metadata.name
          })
        }
      })
      
      // Only update if mappings changed
      if (autoMappings.length > 0 && JSON.stringify(autoMappings) !== JSON.stringify(params.arrayCredsMappings)) {
        onChange("arrayCredsMappings")(autoMappings)
      }
    }
  }, [storageCopyMethod, vmWareStorage, validatedArrayCreds, onChange])

  const unmappedStorage = useMemo(
    () => {
      if (storageCopyMethod === 'vendor-based') {
        return vmWareStorage.filter(storage =>
          !params.arrayCredsMappings?.some(mapping => mapping.source === storage)
        )
      }
      return vmWareStorage.filter(storage =>
        !params.storageMappings?.some(mapping => mapping.source === storage)
      )
    },
    [vmWareStorage, params.storageMappings, params.arrayCredsMappings, storageCopyMethod]
  );

  // Calculate completion status
  const networksFullyMapped = unmappedNetworks.length === 0 && vmwareNetworks.length > 0;
  const storageFullyMapped = unmappedStorage.length === 0 && vmWareStorage.length > 0;
  
  // Get available ArrayCreds names for dropdown
  const arrayCredsNames = useMemo(
    () => validatedArrayCreds.map(ac => ac.metadata.name),
    [validatedArrayCreds]
  )

  return (
    <VmsSelectionStepContainer>
      <Step stepNumber={stepNumber} label="Network and Storage Mapping" />
      <FieldsContainer>
        {loading ? (
          <Typography variant="body2" color="text.secondary">
            Loading OpenStack networks and storage options...
          </Typography>
        ) : (
          <>
            <FormControl error={!!networkMappingError}>
              <Box
                sx={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                  mb: 1
                }}
              >
                <Typography variant="subtitle2">Map Networks</Typography>
                {networksFullyMapped ? (
                  <Typography variant="body2" color="success.main">
                    All networks mapped ✓
                  </Typography>
                ) : (
                  <Typography variant="body2" color="warning.main">
                    {unmappedNetworks.length} of {vmwareNetworks.length} networks unmapped
                  </Typography>
                )}
              </Box>
              <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                Select source and target networks to automatically create mappings. All networks
                must be mapped to proceed.
              </Typography>
              <ResourceMappingTable
                sourceItems={vmwareNetworks}
                targetItems={openstackNetworks}
                sourceLabel="VMware Network"
                targetLabel="OpenStack Network"
                values={params.networkMappings || []}
                onChange={(value) => onChange('networkMappings')(value)}
                oneToManyMapping
              />
              {networkMappingError && <FormHelperText error>{networkMappingError}</FormHelperText>}
            </FormControl>
            <FormControl error={!!storageMappingError}>
              <Box
                sx={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                  mb: 1
                }}
              >
                <Typography variant="subtitle2">Map Storage</Typography>
                {storageFullyMapped ? (
                  <Typography variant="body2" color="success.main">
                    All storage mapped ✓
                  </Typography>
                ) : (
                  <Typography variant="body2" color="warning.main">
                    {unmappedStorage.length} of {vmWareStorage.length} storage devices unmapped
                  </Typography>
                )}
              </Box>
              
              {/* Storage Copy Method Selection */}
              <Box sx={{ mb: 2 }}>
                <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
                  Select the data copy method for storage migration:
                </Typography>
                <RadioGroup
                  value={storageCopyMethod}
                  onChange={(e) => onChange("storageCopyMethod")(e.target.value)}
                  row
                >
                  {STORAGE_COPY_METHOD_OPTIONS.map(option => (
                    <FormControlLabel
                      key={option.value}
                      value={option.value}
                      control={<Radio />}
                      label={option.label}
                    />
                  ))}
                </RadioGroup>
              </Box>

              {storageCopyMethod === 'normal' ? (
                <>
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
                </>
              ) : (
                <>
                  <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                    Map datastores to storage array credentials for vendor-optimized data copy. 
                    Datastores are automatically mapped based on array discovery.
                  </Typography>
                  
                  {arrayCredsLoading ? (
                    <Alert severity="info" sx={{ mb: 2 }}>
                      Loading array credentials...
                    </Alert>
                  ) : validatedArrayCreds.length === 0 ? (
                    <Alert severity="warning" sx={{ mb: 2 }}>
                      No validated array credentials found. Please configure and validate array credentials first.
                    </Alert>
                  ) : (
                    <>
                      {unmappedStorage.length > 0 && (
                        <Alert severity="info" sx={{ mb: 2 }}>
                          {unmappedStorage.length} datastore(s) could not be auto-mapped. Please map them manually below.
                        </Alert>
                      )}
                      <ResourceMappingTable
                        sourceItems={vmWareStorage}
                        targetItems={arrayCredsNames}
                        sourceLabel="VMware Datastore"
                        targetLabel="Array Credentials"
                        values={params.arrayCredsMappings || []}
                        onChange={(value) => onChange("arrayCredsMappings")(value)}
                        oneToManyMapping
                      />
                    </>
                  )}
                </>
              )}
              
              {storageMappingError && (
                <FormHelperText error>{storageMappingError}</FormHelperText>
              )}
            </FormControl>
          </>
        )}
      </FieldsContainer>
    </VmsSelectionStepContainer>
  )
}
