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
import { useEffect, useMemo, useCallback, useRef, useState } from 'react'
import { ResourceMappingTableNew as ResourceMappingTable } from './components'
import { Step } from 'src/shared/components/forms'
import { FieldLabel } from 'src/components'
import { useArrayCredentialsQuery } from 'src/hooks/api/useArrayCredentialsQuery'
import { PCDNetworkInfo, OpenstackCreds } from 'src/api/openstack-creds/model'
import { checkNetworkSubnetCompatibility } from 'src/api/openstack-creds/openstackCreds'
import { VmData } from 'src/features/migration/api/migration-templates/model'

const VmsSelectionStepContainer = styled('div')(({ theme }) => ({
  display: 'grid',
  gridGap: theme.spacing(1)
}))

const FieldsContainer = styled('div')(({ theme }) => ({
  display: 'grid',
  gridGap: theme.spacing(2)
}))

export interface ResourceMap {
  source: string
  target: string
}

// Storage copy method options
export const STORAGE_COPY_METHOD_OPTIONS = [
  { value: 'normal', label: 'Standard Copy' },
  { value: 'StorageAcceleratedCopy', label: 'Storage Accelerated Copy' }
] as const

export type StorageCopyMethod = (typeof STORAGE_COPY_METHOD_OPTIONS)[number]['value']

interface NetworkAndStorageMappingStepProps {
  vmwareNetworks: string[]
  vmWareStorage: string[]
  openstackNetworks: PCDNetworkInfo[]
  openstackStorage: string[]
  params: {
    networkMappings?: ResourceMap[]
    storageMappings?: ResourceMap[]
    arrayCredsMappings?: ResourceMap[]
    storageCopyMethod?: StorageCopyMethod
  }
  onChange: (key: string) => (value: any) => void
  networkMappingError?: string
  storageMappingError?: string
  stepNumber?: string
  loading?: boolean
  showHeader?: boolean
  selectedVMs?: VmData[]
  openstackCredentials?: OpenstackCreds
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
  loading = false,
  showHeader = true,
  selectedVMs = [],
  openstackCredentials
}: NetworkAndStorageMappingStepProps) {
  const storageCopyMethod = params.storageCopyMethod || 'normal'

  // subnet compatibility warnings keyed by source VMware network name
  const [subnetWarnings, setSubnetWarnings] = useState<Record<string, string>>({})

  const removedAutoArrayCredsSourcesRef = useRef<Set<string>>(new Set())

  // Track previous mappings JSON to avoid redundant API calls
  const prevMappingsRef = useRef<string>('')

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

  // Get available ArrayCreds names for dropdown
  const arrayCredsNames = useMemo(
    () => validatedArrayCreds.map((ac) => ac.metadata.name),
    [validatedArrayCreds]
  )

  // Filter out any mappings that don't match the available networks/storage
  // Extract network names from PCDNetworkInfo for filtering
  const openstackNetworkNames = useMemo(
    () => openstackNetworks.map((net) => net.name),
    [openstackNetworks]
  )

  const filteredNetworkMappings = useMemo(
    () =>
      (params.networkMappings || []).filter(
        (mapping) =>
          vmwareNetworks.includes(mapping.source) && openstackNetworkNames.includes(mapping.target)
      ),
    [params.networkMappings, vmwareNetworks, openstackNetworkNames]
  )

  const filteredStorageMappings = useMemo(
    () =>
      (params.storageMappings || []).filter(
        (mapping) =>
          vmWareStorage.includes(mapping.source) && openstackStorage.includes(mapping.target)
      ),
    [params.storageMappings, vmWareStorage, openstackStorage]
  )

  const filteredArrayCredsMappings = useMemo(
    () =>
      (params.arrayCredsMappings || []).filter(
        (mapping) =>
          vmWareStorage.includes(mapping.source) && arrayCredsNames.includes(mapping.target)
      ),
    [params.arrayCredsMappings, vmWareStorage, arrayCredsNames]
  )

  useEffect(() => {
    if (filteredNetworkMappings.length !== params.networkMappings?.length) {
      onChange('networkMappings')(filteredNetworkMappings)
    }
  }, [filteredNetworkMappings, onChange, params.networkMappings])

  useEffect(() => {
    if (
      storageCopyMethod === 'normal' &&
      filteredStorageMappings.length !== params.storageMappings?.length
    ) {
      onChange('storageMappings')(filteredStorageMappings)
    }
  }, [filteredStorageMappings, onChange, params.storageMappings, storageCopyMethod])

  useEffect(() => {
    if (
      storageCopyMethod === 'StorageAcceleratedCopy' &&
      filteredArrayCredsMappings.length !== params.arrayCredsMappings?.length
    ) {
      onChange('arrayCredsMappings')(filteredArrayCredsMappings)
    }
  }, [filteredArrayCredsMappings, onChange, params.arrayCredsMappings, storageCopyMethod])

  // Auto-map datastores to ArrayCreds based on dataStore information in ArrayCreds status
  useEffect(() => {
    if (
      storageCopyMethod !== 'StorageAcceleratedCopy' ||
      !validatedArrayCreds.length ||
      !vmWareStorage.length
    ) {
      return
    }

    // Create a map of datastore names to ArrayCreds names
    const datastoreToArrayCredsMap = new Map<string, string>()
    validatedArrayCreds.forEach((cred) => {
      const datastores = cred.status?.dataStore || []
      datastores.forEach((ds) => {
        // Map datastore name to ArrayCreds name
        datastoreToArrayCredsMap.set(ds.name, cred.metadata.name)
      })
    })

    // Get current mappings
    const currentMappings = params.arrayCredsMappings || []
    const currentMappedSources = new Set(currentMappings.map((m) => m.source))

    // Find datastores that can be auto-mapped but aren't already mapped
    const autoMappings: ResourceMap[] = []
    vmWareStorage.forEach((datastore) => {
      if (removedAutoArrayCredsSourcesRef.current.has(datastore)) {
        return
      }
      if (!currentMappedSources.has(datastore) && datastoreToArrayCredsMap.has(datastore)) {
        autoMappings.push({
          source: datastore,
          target: datastoreToArrayCredsMap.get(datastore)!
        })
      }
    })

    // If we found any auto-mappings, add them to existing mappings
    if (autoMappings.length > 0) {
      onChange('arrayCredsMappings')([...currentMappings, ...autoMappings])
    }
  }, [storageCopyMethod, validatedArrayCreds, vmWareStorage, params.arrayCredsMappings, onChange])

  const handleArrayCredsMappingsChange = useCallback(
    (nextMappings: ResourceMap[]) => {
      const prevMappings = params.arrayCredsMappings || []

      const prevSources = new Set(prevMappings.map((m) => m.source))
      const nextSources = new Set(nextMappings.map((m) => m.source))

      for (const source of prevSources) {
        if (!nextSources.has(source)) {
          removedAutoArrayCredsSourcesRef.current.add(source)
        }
      }

      for (const source of nextSources) {
        if (removedAutoArrayCredsSourcesRef.current.has(source)) {
          removedAutoArrayCredsSourcesRef.current.delete(source)
        }
      }

      onChange('arrayCredsMappings')(nextMappings)
    },
    [onChange, params.arrayCredsMappings]
  )

  // Collect IPs for VMs on a given source VMware network
  const collectIPsForNetwork = useCallback(
    (sourceNetwork: string): string[] => {
      const ips: string[] = []
      for (const vm of selectedVMs) {
        // Use vm.networks as authoritative check for whether VM is on this network
        const isOnNetwork = vm.networks?.includes(sourceNetwork) ?? false
        if (!isOnNetwork) continue

        if (vm.networkInterfaces && vm.networkInterfaces.length > 0) {
          // Try to match NICs by network name first
          const matchingNics = vm.networkInterfaces.filter((nic) => nic.network === sourceNetwork)
          const nicsToUse = matchingNics.length > 0 ? matchingNics : vm.networkInterfaces

          for (const nic of nicsToUse) {
            const shouldPreserve = nic.preserveIP !== false
            if (shouldPreserve && Array.isArray(nic.ipAddress)) {
              ips.push(...nic.ipAddress.filter((ip) => ip && ip.trim() !== ''))
            }
          }
        }

        // Also collect from vm.ipAddress as fallback
        if (vm.ipAddress && vm.ipAddress !== '—' && vm.ipAddress.trim()) {
          const ipStrings = vm.ipAddress.split(',').map((ip) => ip.trim()).filter((ip) => ip !== '')
          ips.push(...ipStrings)
        }
      }
      return [...new Set(ips)]
    },
    [selectedVMs]
  )

  // Run subnet compatibility check whenever network mappings change
  useEffect(() => {
    const completeMappings = (params.networkMappings || []).filter(
      (m) => m.source && m.target
    )
    const mappingsKey = JSON.stringify(completeMappings)
    if (mappingsKey === prevMappingsRef.current) return
    prevMappingsRef.current = mappingsKey

    if (!openstackCredentials || completeMappings.length === 0 || selectedVMs.length === 0) {
      setSubnetWarnings({})
      return
    }

    const secretName = `${openstackCredentials.metadata.name}-openstack-secret`
    const secretNamespace = openstackCredentials.metadata.namespace

    const runChecks = async () => {
      const nextWarnings: Record<string, string> = {}

      await Promise.all(
        completeMappings.map(async (mapping) => {
          const ips = collectIPsForNetwork(mapping.source)
          
          if (ips.length === 0) {
            return
          }

          const isL2Network = openstackNetworks.some(
            (n) => n.name === mapping.target && Array.isArray(n.tags) && n.tags.includes('simple_network')
          )
          if (isL2Network) return

          try {
            const result = await checkNetworkSubnetCompatibility({
              ips,
              network_name: mapping.target,
              access_info: {
                secret_name: secretName,
                secret_namespace: secretNamespace
              }
            })

            if (!result.all_compatible) {
              const incompatibleIPs = result.results
                .filter((r) => !r.is_compatible)
                .map((r) => r.ip)
              const cidrList =
                result.subnet_cidrs?.length > 0
                  ? ` (${result.subnet_cidrs.join(', ')})`
                  : ''
              nextWarnings[mapping.source] =
                `${incompatibleIPs.length} VM IP address(es) [${incompatibleIPs.join(', ')}] do not lie within the subnet of destination network ${mapping.target} ${cidrList}. ` +
                `Ensure fallback to DHCP is enabled, otherwise it may lead to migration failures`
            }
          } catch (error) {
          }
        })
      )

      setSubnetWarnings(nextWarnings)
    }

    runChecks()
  }, [params.networkMappings, openstackCredentials, selectedVMs, collectIPsForNetwork, openstackNetworks])

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
  const networksFullyMapped = unmappedNetworks.length === 0 && vmwareNetworks.length > 0
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
                <FieldLabel label="Map Networks" required align="flex-start" />
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
