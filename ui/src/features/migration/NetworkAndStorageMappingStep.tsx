import {
  FormControl,
  FormHelperText,
  styled,
  Typography,
  Box,
  RadioGroup,
  FormControlLabel,
  Radio,
  Alert,
  Chip
} from '@mui/material'
import { useMemo } from 'react'
import { useNetworkIPsMap } from './hooks/useNetworkIPsMap'
import { useFilteredMappings } from './hooks/useFilteredMappings'
import { useNetworkSubnetCompatibility } from './hooks/useNetworkSubnetCompatibility'
import { ResourceMappingTableNew as ResourceMappingTable } from './components'
import { Step } from 'src/shared/components/forms'
import { FieldLabel } from 'src/components'
import { useArrayCredentialsQuery } from 'src/hooks/api/useArrayCredentialsQuery'
import type { NetworkAndStorageMappingStepProps } from './types'
import { STORAGE_COPY_METHOD_OPTIONS } from './constants'

export type { ResourceMap, StorageCopyMethod } from './types'

const VmsSelectionStepContainer = styled('div')(({ theme }) => ({
  display: 'grid',
  gridGap: theme.spacing(1)
}))

const FieldsContainer = styled('div')(({ theme }) => ({
  display: 'grid',
  gridGap: theme.spacing(2)
}))

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
  loading = false,
  showHeader = true,
  selectedVMs = [],
  openstackCredentials
}: NetworkAndStorageMappingStepProps) {
  const storageCopyMethod = params.storageCopyMethod || 'normal'

  // Fetch validated array credentials for StorageAcceleratedCopy
  const { data: arrayCredentials, isLoading: arrayCredsLoading } = useArrayCredentialsQuery(
    undefined,
    {
      enabled: storageCopyMethod === 'StorageAcceleratedCopy'
    }
  )

  // Filter to only validated array credentials
  const validatedArrayCreds = useMemo(
    () =>
      (arrayCredentials || []).filter((cred) => cred.status?.arrayValidationStatus === 'Succeeded'),
    [arrayCredentials]
  )

  const arrayCredsNames = useMemo(
    () => validatedArrayCreds.map((ac) => ac.metadata.name),
    [validatedArrayCreds]
  )

  const openstackNetworkNames = useMemo(
    () => openstackNetworks.map((net) => net.name),
    [openstackNetworks]
  )

  const { handleArrayCredsMappingsChange } = useFilteredMappings({
    params,
    vmwareNetworks,
    openstackNetworkNames,
    vmWareStorage,
    openstackStorage,
    arrayCredsNames,
    storageCopyMethod,
    validatedArrayCreds,
    onChange
  })

  const networkIPsMap = useNetworkIPsMap(selectedVMs)

  const subnetWarnings = useNetworkSubnetCompatibility({
    networkMappings: params.networkMappings,
    openstackCredentials,
    selectedVMs,
    networkIPsMap,
    openstackNetworks
  })

  // Calculate unmapped networks and storage
  const unmappedNetworks = useMemo(
    () =>
      vmwareNetworks.filter(
        (network) => !params.networkMappings?.some((mapping) => mapping.source === network)
      ),
    [vmwareNetworks, params.networkMappings]
  )

  const unmappedStorage = useMemo(() => {
    if (storageCopyMethod === 'StorageAcceleratedCopy') {
      return vmWareStorage.filter(
        (storage) => !params.arrayCredsMappings?.some((mapping) => mapping.source === storage)
      )
    }
    return vmWareStorage.filter(
      (storage) => !params.storageMappings?.some((mapping) => mapping.source === storage)
    )
  }, [vmWareStorage, params.storageMappings, params.arrayCredsMappings, storageCopyMethod])

  // Calculate completion status
  const hasSourceNetworks = vmwareNetworks.length > 0
  const networksFullyMapped = hasSourceNetworks && unmappedNetworks.length === 0
  const storageFullyMapped = unmappedStorage.length === 0 && vmWareStorage.length > 0

  return (
    <VmsSelectionStepContainer>
      {showHeader ? <Step stepNumber={stepNumber} label="Network And Storage Mapping" /> : null}
      <FieldsContainer>
        {loading ? (
          <Typography variant="body2" color="text.secondary">
            Loading PCD networks and storage options...
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
                <FieldLabel
                  label="Map Networks"
                  required={hasSourceNetworks}
                  align="flex-start"
                />
                {!hasSourceNetworks ? (
                  <Typography variant="body2" color="text.secondary">
                    Not required
                  </Typography>
                ) : networksFullyMapped ? (
                  <Typography variant="body2" color="success.main">
                    All networks mapped ✓
                  </Typography>
                ) : (
                  <Typography variant="body2" color="warning.main">
                    {unmappedNetworks.length} of {vmwareNetworks.length} networks unmapped
                  </Typography>
                )}
              </Box>
              {hasSourceNetworks ? (
                <>
                  <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                    Select source and target networks to automatically create mappings. All
                    networks must be mapped to proceed.
                  </Typography>
                  <ResourceMappingTable
                    sourceItems={vmwareNetworks}
                    targetItems={openstackNetworkNames}
                    sourceLabel="VMware Network"
                    targetLabel="PCD Network"
                    values={params.networkMappings || []}
                    onChange={(value) => onChange('networkMappings')(value)}
                    oneToManyMapping
                    fieldPrefix="networkMapping"
                  />
                  {Object.entries(subnetWarnings).map(([sourceNetwork, warning]) => (
                    <Alert key={sourceNetwork} severity="warning" sx={{ mt: 1 }}>
                      <strong>{sourceNetwork}:</strong> {warning}
                    </Alert>
                  ))}
                </>
              ) : (
                <Alert severity="info" sx={{ mt: 1 }}>
                  None of the selected VMs have network interfaces. Network mapping is not
                  required for this plan — you can proceed to the next step.
                </Alert>
              )}
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
                <FieldLabel label="Map Storage" required align="flex-start" />
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
                  onChange={(e) => onChange('storageCopyMethod')(e.target.value)}
                  row
                >
                  {STORAGE_COPY_METHOD_OPTIONS.map((option) => (
                    <FormControlLabel
                      key={option.value}
                      value={option.value}
                      control={<Radio />}
                      label={
                        option.value === 'StorageAcceleratedCopy' ? (
                          <Box sx={{ display: 'inline-flex', alignItems: 'center', gap: 1 }}>
                            <Box component="span">{option.label}</Box>
                            <Chip
                              label="Beta"
                              size="small"
                              color="warning"
                              variant="outlined"
                              sx={{
                                transform: 'translateY(-6px)',
                                height: 16,
                                '& .MuiChip-label': {
                                  px: 0.75,
                                  fontSize: '0.65rem',
                                  lineHeight: '16px',
                                  display: 'flex',
                                  alignItems: 'center'
                                }
                              }}
                            />
                          </Box>
                        ) : (
                          option.label
                        )
                      }
                    />
                  ))}
                </RadioGroup>
              </Box>

              {storageCopyMethod === 'normal' ? (
                <>
                  <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                    Select source and target storage to automatically create mappings. All storage
                    devices must be mapped in order to proceed.
                  </Typography>
                  <ResourceMappingTable
                    sourceItems={vmWareStorage}
                    targetItems={openstackStorage}
                    sourceLabel="VMware Datastore"
                    targetLabel="PCD Volume Type"
                    values={params.storageMappings || []}
                    onChange={(value) => onChange('storageMappings')(value)}
                    oneToManyMapping
                    fieldPrefix="storageMapping"
                  />
                </>
              ) : (
                <>
                  <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                    Map datastores to storage array credentials for storage array data copy.
                  </Typography>

                  {arrayCredsLoading ? (
                    <Alert severity="info" sx={{ mb: 2 }}>
                      Loading array credentials...
                    </Alert>
                  ) : validatedArrayCreds.length === 0 ? (
                    <Alert severity="warning" sx={{ mb: 2 }}>
                      No validated array credentials found. Please configure and validate array
                      credentials in the Storage Management page first.
                    </Alert>
                  ) : (
                    <>
                      {unmappedStorage.length > 0 && (
                        <Alert severity="info" sx={{ mb: 2 }}>
                          {unmappedStorage.length} datastore(s) need to be mapped. Please map them
                          below.
                        </Alert>
                      )}
                      <ResourceMappingTable
                        sourceItems={vmWareStorage}
                        targetItems={arrayCredsNames}
                        sourceLabel="VMware Datastore"
                        targetLabel="Array Credentials"
                        values={params.arrayCredsMappings || []}
                        onChange={handleArrayCredsMappingsChange}
                        oneToManyMapping
                        fieldPrefix="arrayCredsMapping"
                      />
                    </>
                  )}
                </>
              )}

              {storageMappingError && <FormHelperText error>{storageMappingError}</FormHelperText>}
            </FormControl>
          </>
        )}
      </FieldsContainer>
    </VmsSelectionStepContainer>
  )
}
