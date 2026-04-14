import {
  Box,
  Button,
  Dialog,
  Paper,
  Tooltip,
  DialogTitle,
  DialogContent,
  DialogActions,
  Alert,
  Select,
  MenuItem,
  TextField,
  GlobalStyles,
  FormLabel,
  Switch,
  Snackbar,
  useMediaQuery,
  Divider,
  Typography
} from '@mui/material'
import { ActionButton } from 'src/components'
import ClusterIcon from '@mui/icons-material/Hub'
import React, { useState, useMemo, useEffect, useCallback } from 'react'
import {
  DataGrid,
  GridColDef,
  GridRowSelectionModel,
  GridToolbarColumnsButton
} from '@mui/x-data-grid'
import { useNavigate } from 'react-router-dom'
import { useKeyboardSubmit } from 'src/hooks/ui/useKeyboardSubmit'
import { CustomSearchToolbar } from 'src/components/grid'
import SyntaxHighlighter from 'react-syntax-highlighter/dist/esm/prism'
import { oneLight } from 'react-syntax-highlighter/dist/esm/styles/prism'
import { patchVMwareHost } from 'src/api/vmware-hosts/vmwareHosts'
import { patchVMwareMachine } from 'src/api/vmware-machines/vmwareMachines'
import { VJAILBREAK_DEFAULT_NAMESPACE } from 'src/api/constants'
import MaasConfigDetailsModal from './components/MaasConfigDetailsModal'
import { getOpenstackCredentials } from 'src/api/openstack-creds/openstackCreds'
import { OpenstackCreds } from 'src/api/openstack-creds/model'
import NetworkAndStorageMappingStep, { ResourceMap } from './steps/NetworkAndStorageMappingStep'
import {
  postRollingMigrationPlan,
  createRollingMigrationPlanJson,
  VMSequence,
  ClusterMapping
} from 'src/api/rolling-migration-plans'
import SourceDestinationClusterSelection from './steps/SourceDestinationClusterSelection'
import { createMigrationTemplateJson } from 'src/features/migration/api/migration-templates/helpers'
import {
  patchMigrationTemplate,
  postMigrationTemplate
} from 'src/features/migration/api/migration-templates/migrationTemplates'
import { createMigrationMappingsResources } from 'src/features/migration/hooks/createMigrationMappingsResources'
import useParams from 'src/hooks/useParams'
import MigrationOptions from './MigrationOptionsAlt'
import { CUTOVER_TYPES } from './constants'
import WindowsIcon from 'src/assets/windows_icon.svg'
import LinuxIcon from 'src/assets/linux_icon.svg'
import WarningIcon from '@mui/icons-material/Warning'
import { useClusterData } from './useClusterData'
import { useErrorHandler } from 'src/hooks/useErrorHandler'

// Import CDS icons
import '@cds/core/icon/register.js'
import { ClarityIcons, buildingIcon, clusterIcon, hostIcon, vmIcon } from '@cds/core/icon'
import { useAmplitude } from 'src/hooks/useAmplitude'
import { AMPLITUDE_EVENTS } from 'src/types/amplitude'

import {
  getRollingAreSelectedMigrationOptionsConfigured,
  getRollingIsSubmitDisabled,
  getRollingStep6Complete,
  getRollingStep6HasErrors,
  getUnmappedCount,
  isMappingComplete
} from 'src/features/migration/utils'

import { DrawerShell, DrawerHeader, DrawerFooter, SectionNav, SurfaceCard } from 'src/components'
import type { SectionNavItem } from 'src/components'
import { styled, useTheme } from '@mui/material/styles'
import { FormProvider, useForm, useWatch } from 'react-hook-form'
import { useRollingMigrationFormRHFParamsSync } from 'src/features/migration/hooks/useRollingMigrationFormRHFParamsSync'
import { useRollingMigrationClose } from 'src/features/migration/hooks/useRollingMigrationClose'
import { useRollingMaasConfig } from 'src/features/migration/hooks/useRollingMaasConfig'
import { useRollingVmwareInventory } from 'src/features/migration/hooks/useRollingVmwareInventory'
import { useRollingBulkIpEditor } from 'src/features/migration/hooks/useRollingBulkIpEditor'

// Define types for MigrationOptions
interface FormValues extends Record<string, unknown> {
  dataCopyMethod?: string
  dataCopyStartTime?: string
  cutoverOption?: string
  cutoverStartTime?: string
  cutoverEndTime?: string
  postMigrationScript?: string
  osFamily?: string
  useGPU?: boolean
  useFlavorless?: boolean
  disconnectSourceNetwork?: boolean
  fallbackToDHCP?: boolean
  networkPersistence?: boolean
  storageCopyMethod?: 'normal' | 'StorageAcceleratedCopy'
}

type RollingMigrationRHFValues = {
  dataCopyStartTime: string
  cutoverStartTime: string
  cutoverEndTime: string
  postMigrationActionSuffix: string
  postMigrationActionFolderName: string
}

export interface SelectedMigrationOptionsType extends Record<string, unknown> {
  dataCopyMethod: boolean
  dataCopyStartTime: boolean
  cutoverOption: boolean
  cutoverStartTime: boolean
  cutoverEndTime: boolean
  postMigrationScript: boolean
  osFamily: boolean
  useGPU?: boolean
  useFlavorless?: boolean
  postMigrationAction?: {
    suffix?: boolean
    folderName?: boolean
    renameVm?: boolean
    moveToFolder?: boolean
  }
}

// Default state for checkboxes
const defaultMigrationOptions: SelectedMigrationOptionsType = {
  dataCopyMethod: false,
  dataCopyStartTime: false,
  cutoverOption: false,
  cutoverStartTime: false,
  cutoverEndTime: false,
  postMigrationScript: false,
  osFamily: false,
  useGPU: false,
  useFlavorless: false,
  postMigrationAction: {
    suffix: false,
    folderName: false,
    renameVm: false,
    moveToFolder: false
  }
}

type FieldErrors = { [formId: string]: string }

// Register clarity icons
ClarityIcons.addIcons(buildingIcon, clusterIcon, hostIcon, vmIcon)

// Style for Clarity icons
const CdsIconWrapper = styled('div')({
  marginRight: 8,
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center'
})

const drawerWidth = 1400

interface VM {
  id: string
  name: string
  ip: string
  esxHost: string
  networks?: string[]
  datastores?: string[]
  cpu?: number
  memory?: number
  powerState: string
  osFamily?: string
  flavor?: string
  targetFlavorId?: string
  ipValidationStatus?: 'pending' | 'valid' | 'invalid' | 'validating'
  ipValidationMessage?: string
  networkInterfaces?: VmNetworkInterface[]
  preserveIp?: Record<number, boolean>
  preserveMac?: Record<number, boolean>
}

export interface VmNetworkInterface {
  mac: string
  network: string
  ipAddress: string[]
}

// ESX columns will be defined inside the component

const CustomToolbarWithActions = (props) => {
  const { rowSelectionModel, onAssignFlavor, onAssignIP, hasPoweredOffVMs, ...toolbarProps } = props

  return (
    <Box
      sx={{ display: 'flex', justifyContent: 'space-between', width: '100%', padding: '4px 8px' }}
    >
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
        <GridToolbarColumnsButton />
      </Box>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
        {rowSelectionModel && rowSelectionModel.length > 0 && (
          <>
            <ActionButton variant="text" color="primary" onClick={onAssignFlavor} size="small">
              Assign Flavor ({rowSelectionModel.length})
            </ActionButton>
            {hasPoweredOffVMs && (
              <ActionButton variant="text" color="primary" onClick={onAssignIP} size="small">
                Assign IP ({rowSelectionModel.length})
              </ActionButton>
            )}
          </>
        )}
        <CustomSearchToolbar {...toolbarProps} />
      </Box>
    </Box>
  )
}

const CustomESXToolbarWithActions = (props) => {
  const { onAssignHostConfig, ...toolbarProps } = props

  return (
    <Box
      sx={{
        display: 'flex',
        justifyContent: 'flex-end',
        width: '100%',
        padding: '4px 8px',
        gap: 1
      }}
    >
      <ActionButton variant="text" color="primary" onClick={onAssignHostConfig} size="small">
        Assign Host Config
      </ActionButton>
      <CustomSearchToolbar {...toolbarProps} />
    </Box>
  )
}

const MaasConfigDialog = styled(Dialog)({
  '& .MuiDialog-paper': {
    maxWidth: '900px',
    width: '100%'
  }
})

const ConfigSection = styled(Box)(({ theme }) => ({
  marginBottom: theme.spacing(3)
}))

const ConfigField = styled(Box)(({ theme }) => ({
  display: 'flex',
  gap: theme.spacing(1),
  marginBottom: theme.spacing(1.5)
}))

const FieldLabel = styled(Typography)(({ theme }) => ({
  fontWeight: 500,
  minWidth: '140px',
  color: theme.palette.text.secondary
}))

const FieldValue = styled(Typography)(({ theme }) => ({
  fontWeight: 400,
  color: theme.palette.text.primary
}))

const CodeEditorContainer = styled(Box)(({ theme }) => ({
  border: `1px solid ${theme.palette.grey[300]}`,
  borderRadius: theme.shape.borderRadius,
  overflow: 'auto',
  position: 'relative',
  resize: 'vertical',
  minHeight: '250px',
  maxHeight: '400px',
  backgroundColor: theme.palette.common.white,
  '& pre': {
    margin: 0,
    borderRadius: 0,
    height: '100%',
    overflow: 'auto',
    fontSize: '14px'
  },
  '&::-webkit-scrollbar': {
    width: '8px',
    height: '8px'
  },
  '&::-webkit-scrollbar-thumb': {
    backgroundColor: theme.palette.grey[300],
    borderRadius: '4px'
  },
  '&::-webkit-scrollbar-track': {
    backgroundColor: theme.palette.grey[100]
  }
}))

interface RollingMigrationFormDrawerProps {
  open: boolean
  onClose: () => void
}

export default function RollingMigrationFormDrawer({
  open,
  onClose
}: RollingMigrationFormDrawerProps) {
  const navigate = useNavigate()
  const { reportError } = useErrorHandler({ component: 'RollingMigrationForm' })
  const { track } = useAmplitude({ component: 'RollingMigrationForm' })
  const { sourceData, pcdData, loadingVMware: loading, loadingPCD } = useClusterData()
  const [sourceCluster, setSourceCluster] = useState('')
  const [destinationPCD, setDestinationPCD] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const [selectedVMwareCredName, setSelectedVMwareCredName] = useState('')

  const [selectedPcdCredName, setSelectedPcdCredName] = useState('')

  const [selectedVMs, setSelectedVMs] = useState<GridRowSelectionModel>([])
  const [pcdHostConfigDialogOpen, setPcdHostConfigDialogOpen] = useState(false)
  const [selectedPcdHostConfig, setSelectedPcdHostConfig] = useState('')
  const [updatingPcdMapping, setUpdatingPcdMapping] = useState(false)

  const {
    loadingHosts,
    loadingVMs,
    orderedESXHosts,
    setOrderedESXHosts,
    vmsWithAssignments,
    setVmsWithAssignments,
    fetchClusterVMs
  } = useRollingVmwareInventory({
    open,
    sourceCluster,
    sourceData,
    selectedVMs,
    setSelectedVMs
  })

  const {
    maasConfigDialogOpen,
    handleCloseMaasConfigDialog,
    maasConfigs,
    selectedMaasConfig,
    loadingMaasConfig,
    maasDetailsModalOpen,
    handleViewMaasConfig,
    handleCloseMaasDetailsModal
  } = useRollingMaasConfig({ open, reportError })

  const [networkMappings, setNetworkMappings] = useState<ResourceMap[]>([])
  const [storageMappings, setStorageMappings] = useState<ResourceMap[]>([])
  const [arrayCredsMappings, setArrayCredsMappings] = useState<ResourceMap[]>([])
  const [networkMappingError, setNetworkMappingError] = useState<string>('')
  const [storageMappingError, setStorageMappingError] = useState<string>('')

  const [openstackCredData, setOpenstackCredData] = useState<OpenstackCreds | null>(null)
  const [loadingOpenstackDetails, setLoadingOpenstackDetails] = useState(false)

  // IP editing and validation state - updated for multiple interfaces
  // IP editing and validation state removed - using bulk assignment instead

  // OS assignment state
  const [vmOSAssignments, setVmOSAssignments] = useState<Record<string, string>>({})
  const [osValidationError, setOsValidationError] = useState<string>('')

  // Migration Options state
  const { params, getParamsUpdater } = useParams<FormValues>({})
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({})
  const getFieldErrorsUpdater = useCallback(
    (key: string | number) => (value: string) => {
      setFieldErrors((prev) => ({ ...prev, [key]: value }))
    },
    []
  )

  const rhfForm = useForm<RollingMigrationRHFValues, any, RollingMigrationRHFValues>({
    defaultValues: {
      dataCopyStartTime: params.dataCopyStartTime ?? '',
      cutoverStartTime: params.cutoverStartTime ?? '',
      cutoverEndTime: params.cutoverEndTime ?? '',
      postMigrationActionSuffix: (params as any)?.postMigrationAction?.suffix ?? '',
      postMigrationActionFolderName: (params as any)?.postMigrationAction?.folderName ?? ''
    }
  })

  const rhfDataCopyStartTime = useWatch({ control: rhfForm.control, name: 'dataCopyStartTime' })
  const rhfCutoverStartTime = useWatch({ control: rhfForm.control, name: 'cutoverStartTime' })
  const rhfCutoverEndTime = useWatch({ control: rhfForm.control, name: 'cutoverEndTime' })
  const rhfPostMigrationActionSuffix = useWatch({
    control: rhfForm.control,
    name: 'postMigrationActionSuffix'
  })
  const rhfPostMigrationActionFolderName = useWatch({
    control: rhfForm.control,
    name: 'postMigrationActionFolderName'
  })
  const { params: selectedMigrationOptions, getParamsUpdater: updateSelectedMigrationOptions } =
    useParams<SelectedMigrationOptionsType>(defaultMigrationOptions)

  // IP validation error state
  const [vmIpValidationError, setVmIpValidationError] = useState<string>('')

  // ESX host config validation error state
  const [esxHostConfigValidationError, setEsxHostConfigValidationError] = useState<string>('')

  const {
    assigningIPs,
    bulkEditDialogOpen,
    bulkEditIPs,
    bulkPreserveIp,
    bulkPreserveMac,
    bulkExistingIPs,
    bulkValidationStatus,
    bulkValidationMessages,
    hasBulkIpValidationErrors,
    hasBulkIpsToApply,
    handleOpenBulkIPAssignment,
    handleCloseBulkEditDialog,
    handleClearAllIPs,
    handleApplyBulkIPs,
    handleBulkIpChange,
    handleBulkPreserveIpChange,
    handleBulkPreserveMacChange,
    renderValidationAdornment
  } = useRollingBulkIpEditor({
    selectedVMs,
    vmsWithAssignments,
    setVmsWithAssignments,
    openstackCredData,
    reportError,
    extractFirstIPv4: (value: string) => {
      if (!value) return ''
      const matches = value.match(IPV4_MATCH_REGEX)
      return matches?.[0] || ''
    }
  })

  // Flavor assignment state
  const [flavorDialogOpen, setFlavorDialogOpen] = useState(false)
  const [selectedFlavor, setSelectedFlavor] = useState('')
  const [updating, setUpdating] = useState(false)

  // Toast notification state
  const [toastOpen, setToastOpen] = useState(false)
  const [toastMessage] = useState('')
  const [toastSeverity] = useState<'success' | 'error' | 'warning' | 'info'>('success')

  const paginationModel = { page: 0, pageSize: 5 }

  const handleCloseToast = useCallback((_event?: React.SyntheticEvent | Event, reason?: string) => {
    if (reason === 'clickaway') {
      return
    }
    setToastOpen(false)
  }, [])

  // Clear selection when component is closed
  useEffect(() => {
    if (!open) {
      setSelectedVMs([])
    }
  }, [open])

  useRollingMigrationFormRHFParamsSync({
    form: rhfForm,
    params: params as any,
    getParamsUpdater,
    selectedMigrationOptions,
    rhfValues: {
      dataCopyStartTime: rhfDataCopyStartTime,
      cutoverStartTime: rhfCutoverStartTime,
      cutoverEndTime: rhfCutoverEndTime,
      postMigrationActionSuffix: rhfPostMigrationActionSuffix,
      postMigrationActionFolderName: rhfPostMigrationActionFolderName
    }
  })

  const handleCloseMaasConfig = handleCloseMaasConfigDialog

  const handleSourceClusterChange = (value) => {
    markTouched('sourceDestination')
    setSourceCluster(value)

    if (value) {
      const parts = value.split(':')
      const credName = parts[0]
      setSelectedVMwareCredName(credName)
    } else {
      setSelectedVMwareCredName('')
    }
  }

  const handleDestinationPCDChange = (value) => {
    markTouched('sourceDestination')
    setDestinationPCD(value)

    if (value) {
      const selectedPCD = pcdData.find((p) => p.id === value)
      if (selectedPCD) {
        setSelectedPcdCredName(selectedPCD.openstackCredName)
        fetchOpenstackCredentialDetails(selectedPCD.openstackCredName)
      }
    } else {
      setSelectedPcdCredName('')
      setOpenstackCredData(null)
    }
  }

  const fetchOpenstackCredentialDetails = async (credName) => {
    if (!credName) return

    setLoadingOpenstackDetails(true)
    try {
      const response = await getOpenstackCredentials(credName)
      setOpenstackCredData(response)
    } catch (error) {
      console.error('Failed to fetch OpenStack credential details:', error)
    } finally {
      setLoadingOpenstackDetails(false)
    }
  }

  // IP validation and editing functions
  const IPV4_MATCH_REGEX =
    /(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)/g

  const extractFirstIPv4 = (value: string): string => {
    if (!value) return ''
    const matches = value.match(IPV4_MATCH_REGEX)
    return matches?.[0] || ''
  }

  const hasMultipleIPv4 = (value: string): boolean => {
    if (!value) return false
    const matches = value.match(IPV4_MATCH_REGEX)
    return (matches?.length || 0) > 1
  }

  // Modal functions for multi-NIC IP editing removed - using bulk assignment instead

  // OS assignment handler
  const handleOSAssignment = async (vmId: string, osFamily: string) => {
    try {
      setVmOSAssignments((prev) => ({ ...prev, [vmId]: osFamily }))

      await patchVMwareMachine(
        vmId,
        {
          spec: {
            vms: {
              osFamily: osFamily
            }
          }
        },
        VJAILBREAK_DEFAULT_NAMESPACE
      )

      const updatedVMs = vmsWithAssignments.map((v) =>
        v.id === vmId ? { ...v, osFamily: osFamily } : v
      )
      setVmsWithAssignments(updatedVMs)
    } catch (error) {
      console.error('Failed to assign OS family:', error)
      reportError(error as Error, {
        context: 'os-family-assignment',
        metadata: {
          vmId: vmId,
          osFamily: osFamily,
          action: 'os-family-assignment'
        }
      })
      // Revert local state on error
      setVmOSAssignments((prev) => {
        const newState = { ...prev }
        delete newState[vmId]
        return newState
      })
    }
  }

  const availableVmwareNetworks = useMemo(() => {
    if (!vmsWithAssignments.length || !selectedVMs.length) return []

    const selectedVMsData = vmsWithAssignments.filter((vm) => selectedVMs.includes(vm.id))

    const extractedNetworks = selectedVMsData
      .filter((vm) => vm.networks)
      .flatMap((vm) => vm.networks || [])

    if (extractedNetworks.length > 0) {
      return extractedNetworks.sort() // Remove Array.from(new Set()) to keep duplicates
    }
    return []
  }, [vmsWithAssignments, selectedVMs])

  const availableVmwareDatastores = useMemo(() => {
    if (!vmsWithAssignments.length || !selectedVMs.length) return []

    const selectedVMsData = vmsWithAssignments.filter((vm) => selectedVMs.includes(vm.id))

    const extractedDatastores = selectedVMsData
      .filter((vm) => vm.datastores)
      .flatMap((vm) => vm.datastores || [])

    if (extractedDatastores.length > 0) {
      return Array.from(new Set(extractedDatastores)).sort()
    }
    return []
  }, [vmsWithAssignments, selectedVMs])

  // Define ESX columns inside component to access state and functions
  const esxColumns: GridColDef[] = [
    {
      field: 'name',
      headerName: 'ESX Name',
      flex: 2,
      renderCell: (params) => (
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
          <CdsIconWrapper>
            {/* eslint-disable-next-line @typescript-eslint/ban-ts-comment */}
            {/* @ts-ignore */}
            <cds-icon shape="host" size="md" badge="info"></cds-icon>
          </CdsIconWrapper>
          {params.value}
        </Box>
      )
    },
    {
      field: 'vms',
      headerName: 'VM Count',
      flex: 0.5,
      renderCell: (params) => (
        <Box sx={{ display: 'flex', alignItems: 'center', height: '100%' }}>
          <Typography variant="body2">{params.value}</Typography>
        </Box>
      )
    },
    {
      field: 'state',
      headerName: 'State',
      flex: 0.8,
      renderCell: (params) => {
        const state = params.value || 'Unknown'
        let color = 'text.secondary'
        if (state === 'connected') color = 'success.main'
        if (state === 'disconnected' || state === 'notResponding') color = 'error.main'
        if (state === 'maintenance') color = 'warning.main'

        return (
          <Box sx={{ display: 'flex', alignItems: 'center', height: '100%' }}>
            <Typography variant="body2" sx={{ color, textTransform: 'capitalize' }}>
              {state}
            </Typography>
          </Box>
        )
      }
    },
    {
      field: 'pcdHostConfigName',
      headerName: 'Host Config',
      flex: 1,
      renderCell: (params) => {
        const hostId = params.row.id
        const currentConfig = params.value || ''

        return (
          <Box sx={{ display: 'flex', alignItems: 'center', height: '100%' }}>
            <Select
              size="small"
              value={currentConfig}
              onChange={(e) => handleIndividualHostConfigChange(hostId, e.target.value)}
              displayEmpty
              sx={{
                width: 250,
                '& .MuiSelect-select': {
                  padding: '4px 8px',
                  fontSize: '0.875rem'
                }
              }}
              renderValue={(selected) => {
                if (!selected) {
                  return (
                    <Box
                      sx={{
                        display: 'flex',
                        alignItems: 'center',
                        gap: 1,
                        color: 'text.secondary'
                      }}
                    >
                      <WarningIcon sx={{ fontSize: 16 }} />
                      <em>Select Host Config</em>
                    </Box>
                  )
                }
                return <Typography variant="body2">{selected}</Typography>
              }}
            >
              <MenuItem value="">
                <Box
                  sx={{ display: 'flex', alignItems: 'center', gap: 1, color: 'text.secondary' }}
                >
                  <WarningIcon sx={{ fontSize: 16 }} />
                  <em>Select Host Config</em>
                </Box>
              </MenuItem>
              {(openstackCredData?.spec?.pcdHostConfig || []).map((config) => (
                <MenuItem key={config.id} value={config.name}>
                  <Box sx={{ display: 'flex', flexDirection: 'column' }}>
                    <Typography variant="body2">{config.name}</Typography>
                    <Typography variant="caption" color="text.secondary">
                      {config.mgmtInterface}
                    </Typography>
                  </Box>
                </MenuItem>
              ))}
            </Select>
          </Box>
        )
      }
    }
  ]

  // Calculate ESX host to PCD config mapping status
  const esxHostMappingStatus = useMemo(() => {
    const mappedHostsCount = orderedESXHosts.filter((host) => host.pcdHostConfigName).length
    return {
      mapped: mappedHostsCount,
      total: orderedESXHosts.length,
      fullyMapped: mappedHostsCount === orderedESXHosts.length
    }
  }, [orderedESXHosts])

  const openstackNetworks = useMemo(() => {
    if (!openstackCredData) return []

    const networks = openstackCredData?.status?.openstack?.networks || []
    return networks.sort((a, b) => a.name.toLowerCase().localeCompare(b.name.toLowerCase()))
  }, [openstackCredData])

  const openstackVolumeTypes = useMemo(() => {
    if (!openstackCredData) return []

    const volumeTypes = openstackCredData?.status?.openstack?.volumeTypes || []
    return volumeTypes.sort((a, b) => a.toLowerCase().localeCompare(b.toLowerCase()))
  }, [openstackCredData])

  const openstackFlavors = useMemo(() => {
    if (!openstackCredData) return []

    return openstackCredData?.spec?.flavors || []
  }, [openstackCredData])

  // Update VM flavor names when OpenStack flavors become available
  useEffect(() => {
    if (openstackFlavors.length > 0 && vmsWithAssignments.length > 0) {
      const updatedVMs = vmsWithAssignments.map((vm) => {
        if (vm.targetFlavorId) {
          const flavorObj = openstackFlavors.find((f) => f.id === vm.targetFlavorId)
          if (flavorObj && vm.flavor !== flavorObj.name) {
            return { ...vm, flavor: flavorObj.name }
          }
        }
        return vm
      })

      // Only update if there are actual changes
      const hasChanges = updatedVMs.some(
        (vm, index) => vm.flavor !== vmsWithAssignments[index]?.flavor
      )

      if (hasChanges) {
        setVmsWithAssignments(updatedVMs)
      }
    }
  }, [openstackFlavors, vmsWithAssignments])

  // Update ESXi host config names when OpenStack host configs become available
  useEffect(() => {
    const pcdHostConfigs = openstackCredData?.spec?.pcdHostConfig || []
    if (pcdHostConfigs.length === 0) return

    if (orderedESXHosts.length === 0) return

    const needsUpdate = orderedESXHosts.some((host) => {
      if (!host.pcdHostConfigId) return false
      const configObj = pcdHostConfigs.find((c) => c.id === host.pcdHostConfigId)
      if (!configObj) return false
      return host.pcdHostConfigName !== configObj.name
    })

    if (!needsUpdate) return

    setOrderedESXHosts((prevHosts) => {
      if (prevHosts.length === 0) return prevHosts

      const updatedHosts = prevHosts.map((host) => {
        if (!host.pcdHostConfigId) return host

        const configObj = pcdHostConfigs.find((c) => c.id === host.pcdHostConfigId)
        if (!configObj) return host

        if (host.pcdHostConfigName !== configObj.name) {
          return { ...host, pcdHostConfigName: configObj.name }
        }

        return host
      })

      const hasChanges = updatedHosts.some(
        (host, index) => host.pcdHostConfigName !== prevHosts[index]?.pcdHostConfigName
      )

      return hasChanges ? updatedHosts : prevHosts
    })
  }, [openstackCredData, orderedESXHosts])

  const handleMappingsChange = (key: string) => (value: unknown) => {
    markTouched('mapResources')

    if (!Array.isArray(value) && key !== 'storageCopyMethod') {
      return
    }

    switch (key) {
      case 'networkMappings': {
        const typed = value as ResourceMap[]
        setNetworkMappings(typed)
        getParamsUpdater('networkMappings')(typed)
        setNetworkMappingError('')
        break
      }
      case 'storageMappings': {
        const typed = value as ResourceMap[]
        setStorageMappings(typed)
        getParamsUpdater('storageMappings')(typed)
        setStorageMappingError('')
        break
      }
      case 'arrayCredsMappings': {
        const typed = value as ResourceMap[]
        setArrayCredsMappings(typed)
        getParamsUpdater('arrayCredsMappings')(typed)
        setStorageMappingError('')
        break
      }
      case 'storageCopyMethod':
        if (typeof value === 'string') {
          getParamsUpdater('storageCopyMethod')(value)
        }
        break
      default:
        break
    }
  }

  // Validate IP addresses for selected VMs
  const vmIpValidation = useMemo(() => {
    if (selectedVMs.length === 0) {
      setVmIpValidationError('Please select VMs to assign IP addresses.')
      return { hasError: true, vmsWithoutIPs: [] }
    }

    const selectedVMsData = vmsWithAssignments.filter((vm) => selectedVMs.includes(vm.id))
    const vmsWithoutIPs = selectedVMsData.filter((vm) => vm.ip === '—' || !vm.ip)

    if (vmsWithoutIPs.length > 0) {
      const errorMessage = `Cannot proceed with Migration: ${vmsWithoutIPs.length} selected VM${vmsWithoutIPs.length === 1 ? '' : 's'} do not have IP addresses assigned. Please assign IP addresses to all selected VMs before continuing.`
      setVmIpValidationError(errorMessage)
      return { hasError: true, vmsWithoutIPs }
    } else {
      setVmIpValidationError('')
      return { hasError: false, vmsWithoutIPs: [] }
    }
  }, [selectedVMs, vmsWithAssignments])

  // Validate ESX host configs for all hosts
  const esxHostConfigValidation = useMemo(() => {
    if (orderedESXHosts.length === 0) {
      setEsxHostConfigValidationError('Please select VMs to migrate.')
      return { hasError: true, hostsWithoutConfigs: [] }
    }

    const hostsWithoutConfigs = orderedESXHosts.filter((host) => !host.pcdHostConfigName)

    if (hostsWithoutConfigs.length > 0) {
      const errorMessage = `Cannot proceed with Migration: ${hostsWithoutConfigs.length} ESXi host${hostsWithoutConfigs.length === 1 ? '' : 's'} do not have Host Config assigned. Please assign Host Config to all ESXi hosts before continuing.`
      setEsxHostConfigValidationError(errorMessage)
      return { hasError: true, hostsWithoutConfigs }
    } else {
      setEsxHostConfigValidationError('')
      return { hasError: false, hostsWithoutConfigs: [] }
    }
  }, [orderedESXHosts])

  // Validate OS assignment for selected powered-off VMs
  const osValidation = useMemo(() => {
    if (selectedVMs.length === 0) {
      setOsValidationError('Please select VMs to assign OS.')
      return { hasError: true, vmsWithoutOS: [] }
    }

    const selectedVMsData = vmsWithAssignments.filter((vm) => selectedVMs.includes(vm.id))
    const poweredOffVMsWithoutOS = selectedVMsData.filter((vm) => {
      const assignedOS = vmOSAssignments[vm.id]
      const currentOS = assignedOS || vm.osFamily
      return vm.powerState === 'powered-off' && (!currentOS || currentOS === 'Unknown')
    })

    if (poweredOffVMsWithoutOS.length > 0) {
      const errorMessage = `Cannot proceed with Migration: ${poweredOffVMsWithoutOS.length} powered-off VM${poweredOffVMsWithoutOS.length === 1 ? '' : 's'} do not have Operating System assigned. Please assign OS to all powered-off VMs before continuing.`
      setOsValidationError(errorMessage)
      return { hasError: true, vmsWithoutOS: poweredOffVMsWithoutOS }
    } else {
      setOsValidationError('')
      return { hasError: false, vmsWithoutOS: [] }
    }
  }, [selectedVMs, vmsWithAssignments, vmOSAssignments])

  const handleSubmit = async () => {
    setSubmitting(true)

    const storageCopyMethod = (params.storageCopyMethod || 'normal') as
      | 'normal'
      | 'StorageAcceleratedCopy'

    if (selectedVMs.length > 0) {
      if (!isMappingComplete(availableVmwareNetworks, networkMappings)) {
        setNetworkMappingError('Please map all networks before submitting.')
        setSubmitting(false)
        return
      }

      if (storageCopyMethod === 'StorageAcceleratedCopy') {
        if (!isMappingComplete(availableVmwareDatastores, arrayCredsMappings)) {
          setStorageMappingError('Please map all datastores before submitting.')
          setSubmitting(false)
          return
        }
      } else {
        if (!isMappingComplete(availableVmwareDatastores, storageMappings)) {
          setStorageMappingError('Please map all datastores before submitting.')
          setSubmitting(false)
          return
        }
      }
    } else if (sourceCluster && destinationPCD) {
      alert('Please select at least one VM to migrate')
      setSubmitting(false)
      return
    }

    try {
      const parts = sourceCluster.split(':')
      const credName = parts[0]

      const sourceItem = sourceData.find((item) => item.credName === credName)
      const clusterObj = sourceItem?.clusters.find((cluster) => cluster.id === sourceCluster)
      const clusterName = clusterObj?.name || ''

      const selectedVMsData = vmsWithAssignments
        .filter((vm) => selectedVMs.includes(vm.id))
        .map((vm) => ({
          vmName: vm.name
        })) as VMSequence[]

      const networkOverridesPerVM: Record<
        string,
        Array<{ interfaceIndex: number; preserveIP: boolean; preserveMAC: boolean }>
      > = {}
      vmsWithAssignments
        .filter((vm) => selectedVMs.includes(vm.id))
        .forEach((vm) => {
          const preserveIp = vm.preserveIp || {}
          const preserveMac = vm.preserveMac || {}

          const indices = new Set<string>([...Object.keys(preserveIp), ...Object.keys(preserveMac)])

          if (indices.size === 0) return

          networkOverridesPerVM[vm.name] = Array.from(indices)
            .map((indexStr) => {
              const interfaceIndex = Number(indexStr)
              const ipFlag = preserveIp[interfaceIndex]
              const macFlag = preserveMac[interfaceIndex]
              return {
                interfaceIndex,
                preserveIP: ipFlag !== false,
                preserveMAC: macFlag !== false
              }
            })
            .sort((a, b) => a.interfaceIndex - b.interfaceIndex)
        })

      // Create cluster mapping between VMware cluster and PCD cluster
      const selectedPCD = pcdData.find((p) => p.id === destinationPCD)
      const pcdClusterName = selectedPCD?.name || ''
      const targetPCDClusterName = selectedPCD?.name

      const clusterMapping: ClusterMapping[] = [
        {
          vmwareClusterName: clusterName,
          pcdClusterName: pcdClusterName
        }
      ]

      // Update VMware hosts with their host config IDs
      const hostsToUpdate = orderedESXHosts.filter((host) => host.pcdHostConfigName)

      for (const host of hostsToUpdate) {
        try {
          // Find the config ID from the name
          const availablePcdHostConfigs = openstackCredData?.spec?.pcdHostConfig || []
          const selectedPcdConfig = availablePcdHostConfigs.find(
            (config) => config.name === host.pcdHostConfigName
          )
          const hostConfigId = selectedPcdConfig ? selectedPcdConfig.id : host.pcdHostConfigName

          if (hostConfigId) {
            console.log(`Updating host ${host.name} with hostConfigId: ${hostConfigId}`)
            await patchVMwareHost(host.id, hostConfigId, VJAILBREAK_DEFAULT_NAMESPACE)
          }
        } catch (error) {
          console.error(`Failed to update host config for ${host.name}:`, error)
          reportError(error as Error, {
            context: 'host-config-update',
            metadata: {
              hostId: host.id,
              hostName: host.name,
              hostConfigId: host.pcdHostConfigName,
              action: 'host-config-update'
            }
          })
          // Continue with other hosts even if one fails
        }
      }

      const mappingResources: Awaited<ReturnType<typeof createMigrationMappingsResources>> =
        await createMigrationMappingsResources({
          networkMappings: networkMappings.map((m) => ({ source: m.source, target: m.target })),
          storageMappings: storageMappings.map((m) => ({ source: m.source, target: m.target })),
          arrayCredsMappings: arrayCredsMappings.map((m) => ({
            source: m.source,
            target: m.target
          })),
          storageCopyMethod,
          reportError
        })

      // 3. Create migration template
      const migrationTemplateJson = createMigrationTemplateJson({
        vmwareRef: selectedVMwareCredName,
        openstackRef: selectedPcdCredName,
        networkMapping: mappingResources.networkMapping.metadata.name,
        ...(storageCopyMethod !== 'StorageAcceleratedCopy' &&
          mappingResources.storageMapping?.metadata?.name && {
            storageMapping: mappingResources.storageMapping.metadata.name
          }),
        targetPCDClusterName: targetPCDClusterName
      })
      const migrationTemplateResponse = await postMigrationTemplate(migrationTemplateJson)

      // Update template to include storageCopyMethod and mapping selection
      if (migrationTemplateResponse?.metadata?.name) {
        await patchMigrationTemplate(migrationTemplateResponse.metadata.name, {
          spec: {
            networkMapping: mappingResources.networkMapping.metadata.name,
            storageCopyMethod,
            ...(storageCopyMethod === 'StorageAcceleratedCopy' &&
              mappingResources.arrayCredsMapping?.metadata?.name && {
                arrayCredsMapping: mappingResources.arrayCredsMapping.metadata.name
              }),
            ...(storageCopyMethod !== 'StorageAcceleratedCopy' &&
              mappingResources.storageMapping?.metadata?.name && {
                storageMapping: mappingResources.storageMapping.metadata.name
              })
          }
        })
      }

      // 4. Create rolling migration plan with the template
      const migrationPlanJson = createRollingMigrationPlanJson({
        clusterName,
        vms: selectedVMsData,
        clusterMapping,
        bmConfigRef: {
          name: selectedMaasConfig?.metadata.name || ''
        },
        ...(Object.keys(networkOverridesPerVM).length > 0 && { networkOverridesPerVM }),
        migrationStrategy: {
          adminInitiatedCutOver:
            selectedMigrationOptions.cutoverOption &&
            params.cutoverOption === CUTOVER_TYPES.ADMIN_INITIATED,
          healthCheckPort: '443',
          performHealthChecks: false,
          type: selectedMigrationOptions.dataCopyMethod
            ? (params.dataCopyMethod as string)
            : 'cold',
          ...(selectedMigrationOptions.dataCopyStartTime &&
            params.dataCopyStartTime && {
              dataCopyStart: params.dataCopyStartTime
            }),
          ...(selectedMigrationOptions.cutoverOption &&
            params.cutoverOption === CUTOVER_TYPES.TIME_WINDOW &&
            params.cutoverStartTime && {
              vmCutoverStart: params.cutoverStartTime
            }),
          ...(selectedMigrationOptions.cutoverOption &&
            params.cutoverOption === CUTOVER_TYPES.TIME_WINDOW &&
            params.cutoverEndTime && {
              vmCutoverEnd: params.cutoverEndTime
            })
        },
        migrationTemplate: migrationTemplateResponse.metadata.name,
        namespace: VJAILBREAK_DEFAULT_NAMESPACE
      })

      await postRollingMigrationPlan(migrationPlanJson, VJAILBREAK_DEFAULT_NAMESPACE)

      console.log('Submitted rolling migration plan', migrationPlanJson)

      // Track successful cluster conversion creation
      track(AMPLITUDE_EVENTS.ROLLING_MIGRATION_CREATED, {
        clusterMigrationName: clusterName,
        sourceCluster: clusterObj?.name,
        destinationCluster: selectedPCD?.name,
        vmwareCredential: selectedVMwareCredName,
        pcdCredential: selectedPcdCredName,
        maasConfig: selectedMaasConfig?.metadata.name,
        virtualMachineCount: selectedVMsData?.length || 0,
        esxHostCount: orderedESXHosts?.length || 0,
        networkMappingCount: networkMappings?.length || 0,
        storageMappingCount:
          storageCopyMethod === 'StorageAcceleratedCopy'
            ? arrayCredsMappings?.length || 0
            : storageMappings?.length || 0,
        migrationType: params.dataCopyMethod || 'cold',
        hasAdminInitiatedCutover:
          selectedMigrationOptions.cutoverOption &&
          params.cutoverOption === CUTOVER_TYPES.ADMIN_INITIATED,
        hasTimedCutover:
          selectedMigrationOptions.cutoverOption &&
          params.cutoverOption === CUTOVER_TYPES.TIME_WINDOW,
        migrationTemplate: migrationTemplateResponse.metadata.name,
        namespace: VJAILBREAK_DEFAULT_NAMESPACE
      })

      onClose()
      navigate('/dashboard/cluster-conversions')
    } catch (error) {
      console.error('Failed to submit rolling migration plan:', error)

      // Track cluster conversion failure
      const parts = sourceCluster.split(':')
      const credName = parts[0]
      const sourceItem = sourceData.find((item) => item.credName === credName)
      const clusterObj = sourceItem?.clusters.find((cluster) => cluster.id === sourceCluster)
      const selectedPCD = pcdData.find((p) => p.id === destinationPCD)
      const selectedVMsData = vmsWithAssignments.filter((vm) => selectedVMs.includes(vm.id))

      track(AMPLITUDE_EVENTS.ROLLING_MIGRATION_SUBMISSION_FAILED, {
        clusterMigrationName: clusterObj?.name,
        sourceCluster: clusterObj?.name,
        destinationCluster: selectedPCD?.name,
        vmwareCredential: selectedVMwareCredName,
        pcdCredential: selectedPcdCredName,
        virtualMachineCount: selectedVMsData?.length || 0,
        esxHostCount: orderedESXHosts?.length || 0,
        errorMessage: error instanceof Error ? error.message : String(error),
        stage: 'creation'
      })

      reportError(error as Error, {
        context: 'rolling-migration-plan-submission',
        metadata: {
          sourceCluster: sourceCluster,
          destinationPCD: destinationPCD,
          selectedVMwareCredName: selectedVMwareCredName,
          selectedPcdCredName: selectedPcdCredName,
          action: 'rolling-migration-plan-submission'
        }
      })
      const errorMessage = error instanceof Error ? error.message : 'Unknown error'
      alert(`Failed to submit rolling migration plan: ${errorMessage}`)
    } finally {
      setSubmitting(false)
    }
  }

  const { handleClose } = useRollingMigrationClose({ submitting, onClose })

  const isSubmitDisabled = useMemo(
    () =>
      getRollingIsSubmitDisabled({
        sourceCluster,
        destinationPCD,
        selectedMaasConfig,
        selectedVMsLength: selectedVMs.length,
        submitting,
        params: params as any,
        selectedMigrationOptions,
        fieldErrors,
        availableVmwareNetworks,
        availableVmwareDatastores,
        networkMappings,
        storageMappings,
        arrayCredsMappings
      }),
    [
      sourceCluster,
      destinationPCD,
      selectedMaasConfig,
      selectedVMs.length,
      submitting,
      params,
      selectedMigrationOptions,
      fieldErrors,
      availableVmwareNetworks,
      availableVmwareDatastores,
      networkMappings,
      storageMappings,
      arrayCredsMappings
    ]
  )

  useKeyboardSubmit({
    open,
    isSubmitDisabled: isSubmitDisabled,
    onSubmit: handleSubmit,
    onClose: handleClose
  })

  const theme = useTheme()
  const isSmallNav = useMediaQuery(theme.breakpoints.down('md'))
  const [activeSectionId, setActiveSectionId] = useState<string>('source-destination')

  const [touchedSections, setTouchedSections] = useState({
    sourceDestination: false,
    baremetal: false,
    hosts: false,
    vms: false,
    mapResources: false,
    options: false
  })

  const markTouched = useCallback(
    (key: keyof typeof touchedSections) => {
      setTouchedSections((prev) => (prev[key] ? prev : { ...prev, [key]: true }))
    },
    [setTouchedSections]
  )

  useEffect(() => {
    if (!open) return
    setTouchedSections({
      sourceDestination: false,
      baremetal: false,
      hosts: false,
      vms: false,
      mapResources: false,
      options: false
    })
  }, [open])

  const contentRootRef = React.useRef<HTMLDivElement | null>(null)
  const section1Ref = React.useRef<HTMLDivElement | null>(null)
  const section2Ref = React.useRef<HTMLDivElement | null>(null)
  const section3Ref = React.useRef<HTMLDivElement | null>(null)
  const section4Ref = React.useRef<HTMLDivElement | null>(null)
  const section5Ref = React.useRef<HTMLDivElement | null>(null)
  const section6Ref = React.useRef<HTMLDivElement | null>(null)
  const section7Ref = React.useRef<HTMLDivElement | null>(null)

  useEffect(() => {
    if (!open) return

    let cancelled = false
    let observer: IntersectionObserver | undefined
    let rafId: number | undefined

    const init = () => {
      if (cancelled) {
        if (rafId) cancelAnimationFrame(rafId)
        return
      }

      const root = contentRootRef.current?.parentElement ?? undefined
      const nodes = [
        section1Ref.current,
        section2Ref.current,
        section3Ref.current,
        section4Ref.current,
        section5Ref.current,
        section6Ref.current
      ].filter(Boolean) as HTMLDivElement[]

      if (!root || nodes.length === 0) {
        rafId = requestAnimationFrame(init)
        return
      }

      const idByNode = new Map<Element, string>([
        [section1Ref.current as HTMLDivElement, 'source-destination'],
        [section2Ref.current as HTMLDivElement, 'baremetal'],
        [section3Ref.current as HTMLDivElement, 'hosts'],
        [section4Ref.current as HTMLDivElement, 'vms'],
        [section5Ref.current as HTMLDivElement, 'map-resources'],
        [section6Ref.current as HTMLDivElement, 'options']
      ])

      observer = new IntersectionObserver(
        (entries) => {
          const visible = entries
            .filter((e) => e.isIntersecting)
            .sort((a, b) => (b.intersectionRatio ?? 0) - (a.intersectionRatio ?? 0))[0]

          if (!visible) return
          const id = idByNode.get(visible.target)
          if (id) setActiveSectionId(id)
        },
        {
          root,
          threshold: [0.2, 0.35, 0.5, 0.65]
        }
      )

      nodes.forEach((n) => observer?.observe(n))
    }

    rafId = requestAnimationFrame(init)

    return () => {
      cancelled = true
      if (rafId) cancelAnimationFrame(rafId)
      if (observer) observer.disconnect()
    }
  }, [open])

  const step1HasErrors = false

  const step2HasErrors = false

  const step3HasErrors = Boolean(touchedSections.hosts && Boolean(esxHostConfigValidationError))

  const step4HasErrors = Boolean(
    touchedSections.vms && Boolean(vmIpValidationError || osValidationError)
  )

  const step5HasErrors = Boolean(
    touchedSections.mapResources && Boolean(networkMappingError || storageMappingError)
  )

  const step6HasErrors = getRollingStep6HasErrors({
    isTouched: touchedSections.options,
    selectedMigrationOptions,
    params: params as any,
    fieldErrors
  })

  const areSelectedMigrationOptionsConfigured = useMemo(
    () =>
      getRollingAreSelectedMigrationOptionsConfigured({
        selectedMigrationOptions,
        params: params as any,
        fieldErrors
      }),
    [selectedMigrationOptions, params, fieldErrors]
  )

  const sectionNavItems = useMemo<SectionNavItem[]>(
    () => [
      {
        id: 'source-destination',
        title: 'Source And Destination',
        description: 'Pick clusters',
        status:
          touchedSections.sourceDestination && sourceCluster && destinationPCD
            ? 'complete'
            : step1HasErrors
              ? 'attention'
              : 'incomplete'
      },
      {
        id: 'baremetal',
        title: 'Bare Metal Config',
        description: 'Verify configuration',
        status:
          touchedSections.baremetal && selectedMaasConfig
            ? 'complete'
            : step2HasErrors
              ? 'attention'
              : 'incomplete'
      },
      {
        id: 'hosts',
        title: 'ESXi Hosts',
        description: 'Assign host configs',
        status:
          touchedSections.hosts && orderedESXHosts.length > 0
            ? 'complete'
            : step3HasErrors
              ? 'attention'
              : 'incomplete'
      },
      {
        id: 'vms',
        title: 'Select VMs',
        description: 'Choose VMs and required fields',
        status:
          touchedSections.vms && selectedVMs.length > 0
            ? 'complete'
            : step4HasErrors
              ? 'attention'
              : 'incomplete'
      },
      {
        id: 'map-resources',
        title: 'Map Networks And Storage',
        description: 'Map VMware resources to PCD',
        status:
          touchedSections.mapResources &&
          isMappingComplete(availableVmwareNetworks, networkMappings) &&
          (params.storageCopyMethod === 'StorageAcceleratedCopy'
            ? isMappingComplete(availableVmwareDatastores, arrayCredsMappings)
            : isMappingComplete(availableVmwareDatastores, storageMappings))
            ? 'complete'
            : step5HasErrors
              ? 'attention'
              : 'incomplete'
      },
      {
        id: 'options',
        title: 'Migration Options',
        description: 'Scheduling and advanced behavior',
        status: step6HasErrors
          ? 'attention'
          : getRollingStep6Complete({
                isTouched: touchedSections.options,
                areSelectedMigrationOptionsConfigured,
                params: params as any,
                step6HasErrors
              })
            ? 'complete'
            : 'incomplete'
      }
    ],
    [
      sourceCluster,
      destinationPCD,
      selectedMaasConfig,
      orderedESXHosts.length,
      esxHostConfigValidation.hasError,
      selectedVMs.length,
      vmIpValidation.hasError,
      osValidation.hasError,
      networkMappingError,
      storageMappingError,
      availableVmwareNetworks,
      availableVmwareDatastores,
      networkMappings,
      storageMappings,
      params.cutoverOption,
      selectedMigrationOptions.cutoverOption,
      selectedMigrationOptions.dataCopyStartTime,
      selectedMigrationOptions.postMigrationScript,
      fieldErrors,
      step1HasErrors,
      step2HasErrors,
      step3HasErrors,
      step4HasErrors,
      step5HasErrors,
      step6HasErrors,
      areSelectedMigrationOptionsConfigured,
      touchedSections,
      params.disconnectSourceNetwork,
      params.fallbackToDHCP,
      params.networkPersistence
    ]
  )

  const scrollToSection = useCallback((id: string) => {
    const map: Record<string, React.RefObject<HTMLDivElement | null>> = {
      'source-destination': section1Ref,
      baremetal: section2Ref,
      hosts: section3Ref,
      vms: section4Ref,
      'map-resources': section5Ref,
      options: section6Ref
    }

    const el = map[id]?.current
    if (!el) return
    el.scrollIntoView({ behavior: 'smooth', block: 'start' })
    setActiveSectionId(id)
  }, [])

  const onOptionsChange = useCallback(
    (key: string | number) => (value: any) => {
      return (getParamsUpdater as any)(key)(value)
    },
    [getParamsUpdater]
  )

  const onOptionsSelectionChange = useCallback(
    (key: string | number) => (value: any) => {
      return (updateSelectedMigrationOptions as any)(key)(value)
    },
    [updateSelectedMigrationOptions]
  )

  const uniqueVmwareNetworks = useMemo(() => {
    return Array.from(new Set(availableVmwareNetworks))
  }, [availableVmwareNetworks])

  const unmappedNetworksCount = useMemo(() => {
    return getUnmappedCount(uniqueVmwareNetworks, networkMappings)
  }, [uniqueVmwareNetworks, networkMappings])

  const unmappedStorageCount = useMemo(() => {
    return params.storageCopyMethod === 'StorageAcceleratedCopy'
      ? getUnmappedCount(availableVmwareDatastores, arrayCredsMappings)
      : getUnmappedCount(availableVmwareDatastores, storageMappings)
  }, [params.storageCopyMethod, availableVmwareDatastores, storageMappings, arrayCredsMappings])

  const handleViewMaasConfigWithTouch = () => {
    markTouched('baremetal')
    handleViewMaasConfig()
  }

  const handleOpenPcdHostConfigDialog = () => {
    setPcdHostConfigDialogOpen(true)
  }

  const handleClosePcdHostConfigDialog = () => {
    setPcdHostConfigDialogOpen(false)
    setSelectedPcdHostConfig('')
  }

  const handlePcdHostConfigChange = (event) => {
    setSelectedPcdHostConfig(event.target.value)
  }

  const handleApplyPcdHostConfig = async () => {
    if (!selectedPcdHostConfig) {
      handleClosePcdHostConfigDialog()
      return
    }

    markTouched('hosts')

    setUpdatingPcdMapping(true)

    try {
      const availablePcdHostConfigs = openstackCredData?.spec?.pcdHostConfig || []
      const selectedPcdConfig = availablePcdHostConfigs.find(
        (config) => config.id === selectedPcdHostConfig
      )
      const pcdConfigName = selectedPcdConfig ? selectedPcdConfig.name : selectedPcdHostConfig

      // Update ALL ESX hosts with the selected host config
      const updatedESXHosts = orderedESXHosts.map((host) => ({
        ...host,
        pcdHostConfigName: pcdConfigName
      }))

      setOrderedESXHosts(updatedESXHosts)

      handleClosePcdHostConfigDialog()
    } catch (error) {
      console.error('Error updating PCD host config mapping:', error)
      reportError(error as Error, {
        context: 'pcd-host-config-mapping',
        metadata: {
          selectedPcdHostConfig: selectedPcdHostConfig,
          action: 'update-pcd-host-config-mapping'
        }
      })
    } finally {
      setUpdatingPcdMapping(false)
    }
  }

  // Define VM columns inside component to access state
  const vmColumns: GridColDef[] = [
    {
      field: 'name',
      headerName: 'VM Name',
      flex: 1.3,
      minWidth: 150,
      hideable: false,
      renderCell: (params) => (
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
          <Tooltip title={params.row.powerState === 'powered-on' ? 'Powered On' : 'Powered Off'}>
            <CdsIconWrapper>
              {/* eslint-disable-next-line @typescript-eslint/ban-ts-comment */}
              {/* @ts-ignore */}
              <cds-icon
                shape="vm"
                size="md"
                badge={params.row.powerState === 'powered-on' ? 'success' : 'danger'}
              ></cds-icon>
            </CdsIconWrapper>
          </Tooltip>
          <Box sx={{ overflow: 'hidden', textOverflow: 'ellipsis' }}>{params.value}</Box>
        </Box>
      )
    },
    {
      field: 'ip',
      headerName: 'IP Address(es)',
      flex: 1,
      hideable: true,
      renderCell: (params) => {
        const vm = params.row as VM
        const vmId = vm.id
        const isSelected = selectedVMs.includes(vmId)
        const powerState = vm.powerState

        // For powered-off VMs - consistent display with tooltip to use toolbar
        if (powerState === 'powered-off') {
          let ipDisplay = ''
          let tooltipMessage = ''

          if (vm.networkInterfaces && vm.networkInterfaces.length > 1) {
            // Multiple network interfaces
            ipDisplay = vm.networkInterfaces.map((nic) => nic.ipAddress || '—').join(', ')
            tooltipMessage =
              "Use 'Assign IP' button in toolbar to edit IP addresses for multiple network interfaces"
          } else {
            // Single interface
            ipDisplay = vm.ip || '—'
            tooltipMessage = "Use 'Assign IP' button in toolbar to assign IP address"
          }

          const content = (
            <Box
              sx={{
                display: 'flex',
                alignItems: 'center',
                width: '100%',
                height: '100%',
                gap: 1,
                minWidth: 0
              }}
            >
              <Typography
                variant="body2"
                sx={{
                  fontSize: '0.875rem',
                  flex: 1,
                  minWidth: 0,
                  whiteSpace: 'nowrap',
                  overflow: 'hidden',
                  textOverflow: 'ellipsis'
                }}
              >
                {ipDisplay}
              </Typography>
            </Box>
          )

          return isSelected ? (
            <Tooltip title={tooltipMessage} arrow placement="top">
              {content}
            </Tooltip>
          ) : (
            content
          )
        }

        // For single interface or when not using multi-interface modal
        const currentIp = vm.ip || '—'

        // For powered-on VMs, show IP but indicate it's not editable
        if (powerState === 'powered-on') {
          return (
            <Tooltip title="IP assignment is only available for powered-off VMs" arrow>
              <Box
                sx={{
                  display: 'flex',
                  alignItems: 'center',
                  width: '100%',
                  height: '100%'
                }}
              >
                <Typography
                  variant="body2"
                  sx={{
                    flex: 1,
                    minWidth: 0,
                    whiteSpace: 'nowrap',
                    overflow: 'hidden',
                    textOverflow: 'ellipsis'
                  }}
                >
                  {currentIp}
                </Typography>
              </Box>
            </Tooltip>
          )
        }

        return (
          <Box
            sx={{
              display: 'flex',
              alignItems: 'center',
              width: '100%',
              height: '100%'
            }}
          >
            <Typography variant="body2">{currentIp}</Typography>
          </Box>
        )
      }
    },
    {
      field: 'osFamily',
      headerName: 'Operating System',
      flex: 1,
      hideable: true,
      renderCell: (params) => {
        const vmId = params.row.id
        const isSelected = selectedVMs.includes(vmId)
        const powerState = params.row?.powerState
        const detectedOsFamily = params.row?.osFamily
        const assignedOsFamily = vmOSAssignments[vmId]
        const currentOsFamily = assignedOsFamily || detectedOsFamily

        // Show dropdown for ALL powered-off VMs (allows changing selection)
        if (isSelected && powerState === 'powered-off') {
          return (
            <Box sx={{ display: 'flex', alignItems: 'center', height: '100%' }}>
              <Select
                size="small"
                value={(() => {
                  if (!currentOsFamily || currentOsFamily === 'Unknown') return ''
                  const osLower = currentOsFamily.toLowerCase()
                  if (osLower.includes('windows')) return 'windowsGuest'
                  if (osLower.includes('linux')) return 'linuxGuest'
                  return ''
                })()}
                onChange={(e) => handleOSAssignment(vmId, e.target.value)}
                displayEmpty
                sx={{
                  minWidth: 120,
                  '& .MuiSelect-select': {
                    padding: '4px 8px',
                    fontSize: '0.875rem'
                  }
                }}
              >
                <MenuItem value="">
                  <Box
                    sx={{ display: 'flex', alignItems: 'center', gap: 1, color: 'text.secondary' }}
                  >
                    <WarningIcon sx={{ fontSize: 16 }} />
                    <em>Select OS</em>
                  </Box>
                </MenuItem>
                <MenuItem value="windowsGuest">
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                    <img src={WindowsIcon} alt="Windows" style={{ width: 16, height: 16 }} />
                    Windows
                  </Box>
                </MenuItem>
                <MenuItem value="linuxGuest">
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                    <img src={LinuxIcon} alt="Linux" style={{ width: 16, height: 16 }} />
                    Linux
                  </Box>
                </MenuItem>
              </Select>
            </Box>
          )
        }

        // Show OS with icon for assigned/detected OS
        let displayValue = currentOsFamily || 'Unknown'
        let icon: React.ReactNode = null

        if (currentOsFamily && currentOsFamily.toLowerCase().includes('windows')) {
          displayValue = 'Windows'
          icon = <img src={WindowsIcon} alt="Windows" style={{ width: 20, height: 20 }} />
        } else if (currentOsFamily && currentOsFamily.toLowerCase().includes('linux')) {
          displayValue = 'Linux'
          icon = <img src={LinuxIcon} alt="Linux" style={{ width: 20, height: 20 }} />
        } else if (currentOsFamily && currentOsFamily !== 'Unknown') {
          displayValue = 'Other'
        }

        return (
          <Tooltip
            title={
              powerState === 'powered-off'
                ? !currentOsFamily || currentOsFamily === 'Unknown'
                  ? 'OS assignment required for powered-off VMs'
                  : 'Click to change OS selection'
                : displayValue
            }
          >
            <Box
              sx={{
                display: 'flex',
                alignItems: 'center',
                height: '100%',
                gap: 1
              }}
            >
              {icon}
              {powerState === 'powered-off' &&
                (!currentOsFamily || currentOsFamily === 'Unknown') && (
                  <WarningIcon sx={{ color: 'warning.main', fontSize: 16 }} />
                )}
              <Typography
                variant="body2"
                sx={{
                  color:
                    !currentOsFamily || currentOsFamily === 'Unknown'
                      ? 'text.secondary'
                      : 'text.primary'
                }}
              >
                {displayValue}
              </Typography>
            </Box>
          </Tooltip>
        )
      }
    },
    {
      field: 'networks',
      headerName: 'Network Interface(s)',
      flex: 1,
      hideable: true,
      valueGetter: (value) => value || '—'
    },
    {
      field: 'cpu',
      headerName: 'CPU',
      flex: 0.3,
      hideable: true,
      valueGetter: (value) => value || '- '
    },
    {
      field: 'memory',
      headerName: 'Memory (MB)',
      flex: 0.8,
      hideable: true,
      valueGetter: (value) => value || '—'
    },
    {
      field: 'esxHost',
      headerName: 'ESX Host',
      flex: 1,
      hideable: true,
      valueGetter: (value) => value || '—'
    },
    {
      field: 'flavor',
      headerName: 'Flavor',
      flex: 1,
      hideable: true,
      renderCell: (params) => {
        const vmId = params.row.id
        const isSelected = selectedVMs.includes(vmId)
        const currentFlavor = params.value || 'auto-assign'

        if (isSelected) {
          return (
            <Box sx={{ display: 'flex', alignItems: 'center', height: '100%', width: '100%' }}>
              <Select
                size="small"
                value={(() => {
                  if (currentFlavor === 'auto-assign') return 'auto-assign'
                  const flavorByName = openstackFlavors.find((f) => f.name === currentFlavor)
                  const flavorById = openstackFlavors.find((f) => f.id === currentFlavor)
                  return flavorByName?.id || flavorById?.id || currentFlavor
                })()}
                onChange={(e) => handleIndividualFlavorChange(vmId, e.target.value)}
                displayEmpty
                sx={{
                  minWidth: 120,
                  width: '100%',
                  '& .MuiSelect-select': {
                    padding: '4px 8px',
                    fontSize: '0.875rem'
                  }
                }}
              >
                <MenuItem value="auto-assign">
                  <Typography variant="body2">Auto Assign</Typography>
                </MenuItem>
                {openstackFlavors.map((flavor) => (
                  <MenuItem key={flavor.id} value={flavor.id}>
                    <Typography variant="body2">{flavor.name}</Typography>
                  </MenuItem>
                ))}
              </Select>
            </Box>
          )
        }

        return (
          <Box
            sx={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%' }}
          >
            <Typography variant="body2">{currentFlavor}</Typography>
          </Box>
        )
      }
    },
    {
      field: 'powerState',
      headerName: 'Power State',
      hideable: true,
      flex: 0.8,
      valueGetter: (value) => value || '—'
    }
  ]

  // Flavor assignment handlers
  const handleOpenFlavorDialog = () => {
    if (selectedVMs.length === 0) return
    setFlavorDialogOpen(true)
  }

  const handleCloseFlavorDialog = () => {
    setFlavorDialogOpen(false)
    setSelectedFlavor('')
  }

  const handleFlavorChange = (event) => {
    setSelectedFlavor(event.target.value)
  }

  const handleIndividualFlavorChange = async (vmId: string, flavorValue: string) => {
    try {
      const isAutoAssign = flavorValue === 'auto-assign'
      const selectedFlavorObj = !isAutoAssign
        ? openstackFlavors.find((f) => f.id === flavorValue)
        : null
      const flavorName = isAutoAssign
        ? 'auto-assign'
        : selectedFlavorObj
          ? selectedFlavorObj.name
          : flavorValue

      // Update VM via API
      const payload = {
        spec: {
          targetFlavorId: isAutoAssign ? '' : flavorValue
        }
      }

      await patchVMwareMachine(vmId, payload, VJAILBREAK_DEFAULT_NAMESPACE)

      // Update local state
      const updatedVMs = vmsWithAssignments.map((vm) => {
        if (vm.id === vmId) {
          return {
            ...vm,
            flavor: flavorName,
            targetFlavorId: isAutoAssign ? '' : flavorValue
          }
        }
        return vm
      })
      setVmsWithAssignments(updatedVMs)

      console.log(`Successfully assigned flavor "${flavorName}" to VM ${vmId}`)
    } catch (error) {
      console.error(`Failed to update flavor for VM ${vmId}:`, error)
      reportError(error as Error, {
        context: 'individual-vm-flavor-update',
        metadata: {
          vmId: vmId,
          flavorValue: flavorValue,
          isAutoAssign: flavorValue === 'auto-assign',
          action: 'individual-vm-flavor-update'
        }
      })
      alert(
        `Failed to assign flavor to VM: ${error instanceof Error ? error.message : 'Unknown error'}`
      )
    }
  }

  const handleIndividualHostConfigChange = async (hostId: string, configName: string) => {
    try {
      markTouched('hosts')
      // Update the ESX host with the selected host config
      const updatedESXHosts = orderedESXHosts.map((host) => {
        if (host.id === hostId) {
          return {
            ...host,
            pcdHostConfigName: configName
          }
        }
        return host
      })

      setOrderedESXHosts(updatedESXHosts)

      console.log(`Successfully assigned host config "${configName}" to ESX host ${hostId}`)
    } catch (error) {
      console.error(`Failed to update host config for ESX host ${hostId}:`, error)
      reportError(error as Error, {
        context: 'individual-host-config-update',
        metadata: {
          hostId: hostId,
          configName: configName,
          action: 'individual-host-config-update'
        }
      })
      alert(
        `Failed to assign host config to ESX host: ${error instanceof Error ? error.message : 'Unknown error'}`
      )
    }
  }

  const handleApplyFlavor = async () => {
    if (!selectedFlavor) {
      handleCloseFlavorDialog()
      return
    }

    setUpdating(true)

    try {
      const isAutoAssign = selectedFlavor === 'auto-assign'
      const selectedFlavorObj = !isAutoAssign
        ? openstackFlavors.find((f) => f.id === selectedFlavor)
        : null
      const flavorName = isAutoAssign
        ? 'auto-assign'
        : selectedFlavorObj
          ? selectedFlavorObj.name
          : selectedFlavor

      // Update VMs via API
      const updatePromises = selectedVMs.map(async (vmId) => {
        try {
          const payload = {
            spec: {
              targetFlavorId: isAutoAssign ? '' : selectedFlavor
            }
          }

          await patchVMwareMachine(vmId as string, payload, VJAILBREAK_DEFAULT_NAMESPACE)
          return { success: true, vmId }
        } catch (error) {
          console.error(`Failed to update flavor for VM ${vmId}:`, error)
          return { success: false, vmId, error }
        }
      })

      const results = await Promise.all(updatePromises)
      const failedUpdates = results.filter((result) => !result.success)

      if (failedUpdates.length > 0) {
        console.error(`Failed to update flavor for ${failedUpdates.length} VMs`)
        reportError(new Error(`Failed to update flavor for ${failedUpdates.length} VMs`), {
          context: 'vm-flavor-batch-update-failures',
          metadata: {
            failedUpdates: failedUpdates,
            totalVMs: selectedVMs.length,
            successCount: results.length - failedUpdates.length,
            failedCount: failedUpdates.length,
            action: 'vm-flavor-batch-update'
          }
        })
        alert(
          `Failed to assign flavor to ${failedUpdates.length} VM${failedUpdates.length > 1 ? 's' : ''}`
        )
      } else {
        // Update local state only if all API calls succeeded
        const updatedVMs = vmsWithAssignments.map((vm) => {
          if (selectedVMs.includes(vm.id)) {
            return {
              ...vm,
              flavor: flavorName,
              targetFlavorId: isAutoAssign ? '' : selectedFlavor
            }
          }
          return vm
        })
        setVmsWithAssignments(updatedVMs)

        const actionText = isAutoAssign ? 'cleared flavor assignment for' : 'assigned flavor to'
        console.log(
          `Successfully ${actionText} ${selectedVMs.length} VM${selectedVMs.length > 1 ? 's' : ''}`
        )

        // Refresh VM list to get updated flavor information from API
        await fetchClusterVMs()
      }

      handleCloseFlavorDialog()
    } catch (error) {
      console.error('Error updating flavors:', error)
      reportError(error as Error, {
        context: 'vm-flavor-assignment',
        metadata: {
          selectedVMs: selectedVMs,
          selectedFlavor: selectedFlavor,
          action: 'vm-flavor-assignment'
        }
      })
      alert('Failed to assign flavor to VMs')
    } finally {
      setUpdating(false)
    }
  }

  return (
    <>
      <GlobalStyles
        styles={{
          '.MuiDataGrid-columnsManagement, .MuiDataGrid-columnsManagementPopover': {
            '& .MuiFormControlLabel-label': {
              fontSize: '0.875rem !important'
            },
            '& .MuiCheckbox-root': {
              padding: '4px !important'
            },
            '& .MuiListItem-root': {
              fontSize: '0.875rem !important',
              minHeight: '32px !important',
              padding: '2px 8px !important'
            },
            '& .MuiTypography-root': {
              fontSize: '0.875rem !important'
            },
            '& .MuiInputBase-input': {
              fontSize: '0.875rem !important'
            },
            '& .MuiTextField-root .MuiInputBase-input': {
              fontSize: '0.875rem !important'
            }
          }
        }}
      />
      <FormProvider {...rhfForm}>
        <DrawerShell
          data-testid="rolling-migration-form-drawer"
          open={open}
          onClose={handleClose}
          width={drawerWidth}
          ModalProps={{
            keepMounted: false,
            style: { zIndex: 1300 }
          }}
          header={
            <DrawerHeader
              data-testid="rolling-migration-form-header"
              icon={<ClusterIcon />}
              title="Rolling Cluster Conversion"
            />
          }
          footer={
            <DrawerFooter data-testid="rolling-migration-form-footer">
              <ActionButton
                tone="secondary"
                onClick={handleClose}
                disabled={submitting}
                data-testid="rolling-migration-form-cancel"
              >
                Cancel
              </ActionButton>
              <ActionButton
                tone="primary"
                onClick={handleSubmit}
                disabled={isSubmitDisabled}
                loading={submitting}
                data-testid="rolling-migration-form-submit"
              >
                Start Conversion
              </ActionButton>
            </DrawerFooter>
          }
        >
          <Box
            ref={contentRootRef}
            data-testid="rolling-migration-form-content"
            sx={{
              display: 'grid',
              gridTemplateColumns: isSmallNav ? '1fr' : '56px 1fr',
              gap: 3
            }}
          >
            {!isSmallNav ? (
              <SectionNav
                data-testid="rolling-migration-form-section-nav"
                items={sectionNavItems}
                activeId={activeSectionId}
                onSelect={scrollToSection}
                dense
                showDescriptions={false}
              />
            ) : null}

            <Box sx={{ display: 'grid', gap: 3 }}>
              {isSmallNav ? (
                <SurfaceCard
                  title="Steps"
                  subtitle="Jump to any section"
                  data-testid="rolling-migration-form-steps-card"
                >
                  <Select
                    size="small"
                    value={activeSectionId}
                    onChange={(e) => scrollToSection(e.target.value as string)}
                    fullWidth
                    data-testid="rolling-migration-form-steps-select"
                  >
                    {sectionNavItems.map((item) => (
                      <MenuItem key={item.id} value={item.id}>
                        {item.title}
                      </MenuItem>
                    ))}
                  </Select>
                </SurfaceCard>
              ) : null}

              <Box ref={section1Ref} data-testid="rolling-migration-form-step-source-destination">
                <SurfaceCard
                  variant="section"
                  title="Source And Destination"
                  subtitle="Choose where you convert from and where you convert to"
                  data-testid="rolling-migration-form-step1-card"
                >
                  <SourceDestinationClusterSelection
                    onChange={() => () => {}}
                    errors={{}}
                    onVmwareClusterChange={handleSourceClusterChange}
                    onPcdClusterChange={handleDestinationPCDChange}
                    vmwareCluster={sourceCluster}
                    pcdCluster={destinationPCD}
                    loadingVMware={loading}
                    loadingPCD={loadingPCD}
                    showHeader={false}
                  />
                </SurfaceCard>
              </Box>

              <Divider />

              <Box ref={section2Ref} data-testid="rolling-migration-form-step-baremetal">
                <SurfaceCard
                  variant="section"
                  title="Bare Metal Config"
                  subtitle="Verify the selected configuration"
                  data-testid="rolling-migration-form-step2-card"
                >
                  {loadingMaasConfig ? (
                    <Typography variant="body2">Loading Bare Metal Config...</Typography>
                  ) : maasConfigs.length === 0 ? (
                    <Typography variant="body2">No Bare Metal Config available</Typography>
                  ) : (
                    <Typography
                      variant="subtitle2"
                      component="a"
                      data-testid="rolling-migration-form-baremetal-view-details"
                      sx={{
                        color: 'primary.main',
                        textDecoration: 'underline',
                        cursor: 'pointer',
                        fontWeight: '500'
                      }}
                      onClick={handleViewMaasConfigWithTouch}
                    >
                      View Bare Metal Config Details
                    </Typography>
                  )}
                </SurfaceCard>
              </Box>

              <Divider />

              <Box ref={section3Ref} data-testid="rolling-migration-form-step-hosts">
                <SurfaceCard
                  variant="section"
                  title="ESXi Hosts"
                  subtitle="Assign PCD host configurations to all ESXi hosts"
                  data-testid="rolling-migration-form-step3-card"
                >
                  <Box
                    sx={{
                      display: 'flex',
                      justifyContent: 'space-between',
                      alignItems: 'center',
                      mb: 1
                    }}
                  >
                    <Typography variant="body2" color="text.secondary">
                      Select ESXi hosts and assign PCD host configurations
                    </Typography>
                    {esxHostMappingStatus.fullyMapped && esxHostMappingStatus.total > 0 ? (
                      <Typography variant="body2" color="success.main">
                        All hosts mapped ✓
                      </Typography>
                    ) : (
                      <Typography variant="body2" color="warning.main">
                        {esxHostMappingStatus.mapped} of {esxHostMappingStatus.total} hosts unmapped
                      </Typography>
                    )}
                  </Box>
                  <Paper
                    sx={{ width: '100%', height: 389 }}
                    data-testid="rolling-migration-form-hosts-grid"
                  >
                    <DataGrid
                      rows={orderedESXHosts}
                      columns={esxColumns}
                      initialState={{
                        pagination: { paginationModel },
                        columns: {
                          columnVisibilityModel: {}
                        }
                      }}
                      pageSizeOptions={[5, 10, 25]}
                      rowHeight={45}
                      slots={{
                        toolbar: (props) => (
                          <CustomESXToolbarWithActions
                            {...props}
                            onAssignHostConfig={handleOpenPcdHostConfigDialog}
                          />
                        )
                      }}
                      disableColumnMenu
                      disableColumnFilter
                      loading={loadingHosts}
                    />
                  </Paper>
                  {esxHostConfigValidationError && (
                    <Alert severity="warning" sx={{ mt: 2 }}>
                      {esxHostConfigValidationError}
                    </Alert>
                  )}
                </SurfaceCard>
              </Box>

              <Divider />

              <Box ref={section4Ref} data-testid="rolling-migration-form-step-vms">
                <SurfaceCard
                  variant="section"
                  title="Select VMs"
                  subtitle="Choose the virtual machines to convert and assign required fields"
                  data-testid="rolling-migration-form-step4-card"
                >
                  <Typography
                    variant="caption"
                    color="text.secondary"
                    sx={{ mt: 1, display: 'block', mb: 1 }}
                  >
                    Tip: Powered-off VMs require IP Address and OS assignment for proper migration
                    configuration
                  </Typography>
                  <Paper
                    sx={{ width: '100%', height: 389 }}
                    data-testid="rolling-migration-form-vms-grid"
                  >
                    <DataGrid
                      rows={vmsWithAssignments}
                      columns={vmColumns}
                      initialState={{
                        pagination: { paginationModel },
                        columns: {
                          columnVisibilityModel: {}
                        }
                      }}
                      pageSizeOptions={[5, 10, 25]}
                      rowHeight={52}
                      checkboxSelection
                      onRowSelectionModelChange={(selectedRowIds) => {
                        markTouched('vms')
                        setSelectedVMs(selectedRowIds)
                      }}
                      rowSelectionModel={selectedVMs.filter((vmId) =>
                        vmsWithAssignments.some((vm) => vm.id === vmId)
                      )}
                      slots={{
                        toolbar: (props) => (
                          <CustomToolbarWithActions
                            {...props}
                            rowSelectionModel={selectedVMs.filter((vmId) =>
                              vmsWithAssignments.some((vm) => vm.id === vmId)
                            )}
                            onAssignFlavor={handleOpenFlavorDialog}
                            onAssignIP={handleOpenBulkIPAssignment}
                            hasPoweredOffVMs={selectedVMs.some((vmId) => {
                              const vm = vmsWithAssignments.find((v) => v.id === vmId)
                              return vm && vm.powerState === 'powered-off'
                            })}
                          />
                        ),
                        noRowsOverlay: () => (
                          <Box
                            sx={{
                              display: 'flex',
                              flexDirection: 'column',
                              alignItems: 'center',
                              justifyContent: 'center',
                              height: '100%',
                              p: 2
                            }}
                          >
                            <Typography
                              variant="body1"
                              color="text.secondary"
                              align="center"
                              sx={{ mb: 1 }}
                            >
                              {loadingVMs
                                ? 'Loading VMs...'
                                : 'No VMs found for the selected cluster.'}
                            </Typography>
                          </Box>
                        )
                      }}
                      disableColumnFilter
                      disableRowSelectionOnClick
                      loading={loadingVMs}
                    />
                  </Paper>
                  {vmIpValidationError && (
                    <Alert severity="warning" sx={{ mt: 2 }}>
                      {vmIpValidationError}
                    </Alert>
                  )}
                  {osValidationError && (
                    <Alert severity="warning" sx={{ mt: 2 }}>
                      {osValidationError}
                    </Alert>
                  )}
                </SurfaceCard>
              </Box>

              <Divider />

              <Box ref={section5Ref} data-testid="rolling-migration-form-step-map-resources">
                <SurfaceCard
                  variant="section"
                  title="Map Networks And Storage"
                  subtitle="Ensure VMware networks/datastores have PCD targets"
                  data-testid="rolling-migration-form-step5-card"
                >
                  {sourceCluster && destinationPCD ? (
                    <NetworkAndStorageMappingStep
                      vmwareNetworks={availableVmwareNetworks}
                      vmWareStorage={availableVmwareDatastores}
                      openstackNetworks={openstackNetworks}
                      openstackStorage={openstackVolumeTypes}
                      params={{
                        networkMappings: networkMappings,
                        storageMappings: storageMappings,
                        arrayCredsMappings: arrayCredsMappings,
                        storageCopyMethod: params.storageCopyMethod as any
                      }}
                      onChange={handleMappingsChange}
                      networkMappingError={networkMappingError}
                      storageMappingError={storageMappingError}
                      loading={loadingOpenstackDetails}
                      showHeader={false}
                      selectedVMs={vmsWithAssignments as any}
                      openstackCredentials={openstackCredData || undefined}
                    />
                  ) : (
                    <Typography variant="body2" color="text.secondary">
                      Please select both source cluster and destination PCD to configure mappings.
                    </Typography>
                  )}
                </SurfaceCard>
              </Box>

              <Divider />

              <Box
                ref={section6Ref}
                data-testid="rolling-migration-form-step-options"
                onChangeCapture={() => markTouched('options')}
                onInputCapture={() => markTouched('options')}
              >
                <SurfaceCard
                  variant="section"
                  title="Migration Options"
                  subtitle="Optional scheduling, cutover behavior, and advanced settings"
                  data-testid="rolling-migration-form-step6-card"
                >
                  <MigrationOptions
                    stepNumber="6"
                    params={params}
                    onChange={onOptionsChange}
                    selectedMigrationOptions={selectedMigrationOptions}
                    updateSelectedMigrationOptions={onOptionsSelectionChange}
                    errors={fieldErrors}
                    getErrorsUpdater={getFieldErrorsUpdater}
                    showHeader={false}
                  />
                </SurfaceCard>
              </Box>

              <Divider />

              <Box ref={section7Ref} data-testid="rolling-migration-form-step-preview">
                <SurfaceCard
                  variant="section"
                  title="Preview"
                  subtitle="Verify your selections before starting the conversion"
                  data-testid="rolling-migration-form-step7-card"
                >
                  <Box sx={{ display: 'grid', gap: 1.5 }}>
                    <Typography variant="subtitle2">Summary</Typography>
                    <Box sx={{ display: 'grid', gap: 1 }}>
                      <Box sx={{ display: 'flex', justifyContent: 'space-between', gap: 2 }}>
                        <Typography variant="body2" color="text.secondary">
                          Source
                        </Typography>
                        <Typography variant="body2">{sourceCluster || '—'}</Typography>
                      </Box>
                      <Box sx={{ display: 'flex', justifyContent: 'space-between', gap: 2 }}>
                        <Typography variant="body2" color="text.secondary">
                          Destination
                        </Typography>
                        <Typography variant="body2">{destinationPCD || '—'}</Typography>
                      </Box>
                      <Box sx={{ display: 'flex', justifyContent: 'space-between', gap: 2 }}>
                        <Typography variant="body2" color="text.secondary">
                          VMs selected
                        </Typography>
                        <Typography variant="body2">{selectedVMs.length}</Typography>
                      </Box>
                      <Box sx={{ display: 'flex', justifyContent: 'space-between', gap: 2 }}>
                        <Typography variant="body2" color="text.secondary">
                          Network mappings
                        </Typography>
                        <Typography variant="body2">
                          {uniqueVmwareNetworks.length === 0
                            ? '—'
                            : unmappedNetworksCount === 0
                              ? 'All mapped'
                              : `${unmappedNetworksCount} unmapped`}
                        </Typography>
                      </Box>
                      <Box sx={{ display: 'flex', justifyContent: 'space-between', gap: 2 }}>
                        <Typography variant="body2" color="text.secondary">
                          Storage mappings
                        </Typography>
                        <Typography variant="body2">
                          {availableVmwareDatastores.length === 0
                            ? '—'
                            : unmappedStorageCount === 0
                              ? 'All mapped'
                              : `${unmappedStorageCount} unmapped`}
                        </Typography>
                      </Box>
                    </Box>
                  </Box>
                </SurfaceCard>
              </Box>
            </Box>
          </Box>
        </DrawerShell>
      </FormProvider>

      <MaasConfigDialog
        open={maasConfigDialogOpen}
        onClose={handleCloseMaasConfig}
        aria-labelledby="baremetal-config-dialog-title"
        data-testid="rolling-migration-form-baremetal-dialog"
      >
        <DialogTitle id="baremetal-config-dialog-title">
          <Typography variant="h6">ESXi - Bare Metal Configuration</Typography>
        </DialogTitle>
        <DialogContent dividers>
          {loadingMaasConfig ? (
            <Typography>Loading configuration details...</Typography>
          ) : !selectedMaasConfig ? (
            <Typography>No configuration available</Typography>
          ) : (
            <>
              <ConfigSection>
                <Typography variant="subtitle1" gutterBottom sx={{ fontWeight: 500 }}>
                  Provider Configuration
                </Typography>
                <Box sx={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 3, ml: 1 }}>
                  <ConfigField>
                    <FieldLabel>Provider Type:</FieldLabel>
                    <FieldValue>{selectedMaasConfig!.spec.providerType}</FieldValue>
                  </ConfigField>
                  <ConfigField>
                    <FieldLabel>Bare Metal Provider URL:</FieldLabel>
                    <FieldValue>{selectedMaasConfig!.spec.apiUrl}</FieldValue>
                  </ConfigField>
                  <ConfigField>
                    <FieldLabel>Insecure:</FieldLabel>
                    <FieldValue>{selectedMaasConfig!.spec.insecure ? 'Yes' : 'No'}</FieldValue>
                  </ConfigField>
                  {selectedMaasConfig!.spec.os && (
                    <ConfigField>
                      <FieldLabel>OS:</FieldLabel>
                      <FieldValue>{selectedMaasConfig!.spec.os}</FieldValue>
                    </ConfigField>
                  )}
                  <ConfigField>
                    <FieldLabel>Status:</FieldLabel>
                    <FieldValue>
                      {selectedMaasConfig!.status?.validationStatus || 'Pending validation'}
                    </FieldValue>
                  </ConfigField>
                  {selectedMaasConfig!.status?.validationMessage && (
                    <ConfigField>
                      <FieldLabel>Validation Message:</FieldLabel>
                      <FieldValue>{selectedMaasConfig!.status!.validationMessage}</FieldValue>
                    </ConfigField>
                  )}
                </Box>
              </ConfigSection>

              {selectedMaasConfig!.spec.userDataSecretRef && (
                <ConfigSection>
                  <Typography variant="subtitle1" gutterBottom sx={{ fontWeight: 500 }}>
                    Cloud-Init Configuration
                  </Typography>
                  <Typography
                    variant="caption"
                    sx={{ mb: 1, display: 'block', color: 'text.secondary' }}
                  >
                    User data is stored in a secret:{' '}
                    {selectedMaasConfig!.spec.userDataSecretRef.name}
                  </Typography>
                  <CodeEditorContainer>
                    <SyntaxHighlighter
                      language="yaml"
                      style={oneLight}
                      showLineNumbers
                      wrapLongLines
                      customStyle={{
                        margin: 0,
                        maxHeight: '100%'
                      }}
                    >
                      {`# Cloud-init configuration is stored in Kubernetes Secret: 
# ${selectedMaasConfig!.spec.userDataSecretRef.name}
# in namespace: ${selectedMaasConfig!.spec.userDataSecretRef.namespace || VJAILBREAK_DEFAULT_NAMESPACE}

# The cloud-init configuration includes:
# - package updates and installations
# - configuration files
# - commands to run on startup
# - network configuration
# - and other system setup parameters

# This will be used when provisioning ESXi hosts in the bare metal environment.`}
                    </SyntaxHighlighter>
                  </CodeEditorContainer>
                </ConfigSection>
              )}

              <ConfigSection>
                <Typography variant="subtitle1" gutterBottom sx={{ fontWeight: 500 }}>
                  Resource Information
                </Typography>
                <Box sx={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 3, ml: 1 }}>
                  <ConfigField>
                    <FieldLabel>Name:</FieldLabel>
                    <FieldValue>{selectedMaasConfig.metadata.name}</FieldValue>
                  </ConfigField>
                  <ConfigField>
                    <FieldLabel>Namespace:</FieldLabel>
                    <FieldValue>{selectedMaasConfig.metadata.namespace}</FieldValue>
                  </ConfigField>
                  <ConfigField>
                    <FieldLabel>Created:</FieldLabel>
                    <FieldValue>
                      {new Date(selectedMaasConfig.metadata.creationTimestamp).toLocaleString()}
                    </FieldValue>
                  </ConfigField>
                </Box>
              </ConfigSection>
            </>
          )}
        </DialogContent>
        <DialogActions sx={{ gap: 1, p: 2 }}>
          <ActionButton
            tone="primary"
            onClick={handleCloseMaasConfig}
            data-testid="rolling-migration-form-baremetal-dialog-close"
          >
            Close
          </ActionButton>
        </DialogActions>
      </MaasConfigDialog>

      {maasConfigs && maasConfigs.length > 0 && (
        <MaasConfigDetailsModal
          open={maasDetailsModalOpen}
          onClose={handleCloseMaasDetailsModal}
          config={maasConfigs[0]}
        />
      )}

      {/* PCD Host Config Assignment Dialog */}
      <Dialog
        open={pcdHostConfigDialogOpen}
        onClose={handleClosePcdHostConfigDialog}
        fullWidth
        maxWidth="sm"
        data-testid="rolling-migration-form-host-config-dialog"
      >
        <DialogTitle>Assign Host Config To All ESXi Hosts</DialogTitle>
        <DialogContent>
          <Box sx={{ my: 2 }}>
            <Typography variant="body2" gutterBottom>
              Select Host Configuration
            </Typography>
            <Select
              fullWidth
              value={selectedPcdHostConfig}
              onChange={handlePcdHostConfigChange}
              size="small"
              sx={{ mt: 1 }}
              displayEmpty
              data-testid="rolling-migration-form-host-config-select"
            >
              <MenuItem value="">
                <em>Select a host configuration</em>
              </MenuItem>
              {(openstackCredData?.spec?.pcdHostConfig || []).map((config) => (
                <MenuItem key={config.id} value={config.id}>
                  <Box sx={{ display: 'flex', flexDirection: 'column' }}>
                    <Typography variant="body1">{config.name}</Typography>
                    <Typography variant="caption" color="text.secondary">
                      Management Interface: {config.mgmtInterface}
                    </Typography>
                  </Box>
                </MenuItem>
              ))}
            </Select>
          </Box>
        </DialogContent>
        <DialogActions sx={{ gap: 1, p: 2 }}>
          <ActionButton
            tone="secondary"
            onClick={handleClosePcdHostConfigDialog}
            data-testid="rolling-migration-form-host-config-cancel"
          >
            Cancel
          </ActionButton>
          <ActionButton
            tone="primary"
            onClick={handleApplyPcdHostConfig}
            disabled={!selectedPcdHostConfig || updatingPcdMapping}
            loading={updatingPcdMapping}
            data-testid="rolling-migration-form-host-config-apply"
          >
            Apply To All Hosts
          </ActionButton>
        </DialogActions>
      </Dialog>

      {/* Bulk IP Editor Dialog */}
      <Dialog open={bulkEditDialogOpen} onClose={handleCloseBulkEditDialog} maxWidth="md" fullWidth>
        <DialogTitle>
          Edit IP Addresses for {selectedVMs.length} {selectedVMs.length === 1 ? 'VM' : 'VMs'}
        </DialogTitle>
        <DialogContent dividers>
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
            {/* Quick Actions */}
            <Box sx={{ display: 'flex', justifyContent: { xs: 'flex-start', sm: 'flex-end' } }}>
              <Button size="small" variant="outlined" onClick={handleClearAllIPs}>
                Clear All
              </Button>
            </Box>

            {/* IP Editor Fields */}
            <Box
              sx={{
                maxHeight: 420,
                overflowY: 'auto',
                pr: 1,
                display: 'flex',
                flexDirection: 'column',
                gap: 2
              }}
            >
              {Object.entries(bulkEditIPs).map(([vmId, interfaces]) => {
                const vm = vmsWithAssignments.find((v) => v.id === vmId)
                if (!vm) return null

                return (
                  <Box
                    key={vmId}
                    sx={{
                      p: 2,
                      border: '1px solid',
                      borderColor: 'divider',
                      borderRadius: 2,
                      bgcolor: 'background.paper',
                      boxShadow: 1,
                      display: 'flex',
                      flexDirection: 'column',
                      gap: 2
                    }}
                  >
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                      <Tooltip title={vm.powerState === 'powered-on' ? 'Running' : 'Stopped'}>
                        <CdsIconWrapper>
                          {/* eslint-disable-next-line @typescript-eslint/ban-ts-comment */}
                          {/* @ts-ignore */}
                          <cds-icon
                            shape="vm"
                            size="md"
                            badge={vm.powerState === 'powered-on' ? 'success' : 'danger'}
                          >
                            {/* @ts-ignore */}
                          </cds-icon>
                        </CdsIconWrapper>
                      </Tooltip>
                      <Typography variant="body2" sx={{ fontWeight: 700 }}>
                        {vm.name}
                      </Typography>
                    </Box>

                    {Object.entries(interfaces).map(([interfaceIndexStr, ip]) => {
                      const interfaceIndex = parseInt(interfaceIndexStr)
                      const networkInterface = vm.networkInterfaces?.[interfaceIndex]
                      const status = bulkValidationStatus[vmId]?.[interfaceIndex]
                      const message = bulkValidationMessages[vmId]?.[interfaceIndex]
                      const isPoweredOff = vm.powerState !== 'powered-on'
                      const preserveIp =
                        !isPoweredOff && bulkPreserveIp?.[vmId]?.[interfaceIndex] !== false
                      const preserveMac = bulkPreserveMac?.[vmId]?.[interfaceIndex] !== false

                      return (
                        <Box
                          key={interfaceIndex}
                          sx={{
                            display: 'grid',
                            gridTemplateColumns: { xs: '1fr', sm: '240px 150px 1fr' },
                            columnGap: { xs: 1.5, sm: 2 },
                            rowGap: 1,
                            alignItems: 'flex-start'
                          }}
                        >
                          <Box
                            sx={{
                              display: 'flex',
                              flexDirection: 'column',
                              gap: 0.75,
                              minWidth: 0
                            }}
                          >
                            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                              <Typography
                                variant="body2"
                                color="text.secondary"
                                sx={{ minWidth: 36 }}
                              >
                                IP:
                              </Typography>
                              <Box
                                component="span"
                                sx={{
                                  px: 1,
                                  py: 0.25,
                                  borderRadius: 1,
                                  bgcolor: (theme) =>
                                    theme.palette.mode === 'dark'
                                      ? 'rgba(255, 255, 255, 0.08)'
                                      : theme.palette.grey[100],
                                  border: '1px solid',
                                  borderColor: 'divider',
                                  color: 'text.primary',
                                  fontFamily: 'monospace'
                                }}
                              >
                                {(Array.isArray(networkInterface?.ipAddress)
                                  ? networkInterface.ipAddress
                                      .filter((v) => v && v.trim() !== '')
                                      .join(', ')
                                  : '') ||
                                  (!networkInterface &&
                                  interfaceIndex === 0 &&
                                  !hasMultipleIPv4(vm.ip || '')
                                    ? extractFirstIPv4(vm.ip || '')
                                    : '') ||
                                  '—'}
                              </Box>
                            </Box>
                            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                              <Typography
                                variant="body2"
                                color="text.secondary"
                                sx={{ minWidth: 36 }}
                              >
                                MAC:
                              </Typography>
                              <Box
                                sx={{
                                  display: 'flex',
                                  alignItems: 'center',
                                  gap: 0.75,
                                  minWidth: 0
                                }}
                              >
                                <Box
                                  component="span"
                                  sx={{
                                    px: 1,
                                    py: 0.25,
                                    borderRadius: 1,
                                    bgcolor: (theme) =>
                                      theme.palette.mode === 'dark'
                                        ? 'rgba(255, 255, 255, 0.08)'
                                        : theme.palette.grey[100],
                                    border: '1px solid',
                                    borderColor: 'divider',
                                    color: 'text.primary',
                                    fontFamily: 'monospace'
                                  }}
                                >
                                  {networkInterface?.mac || '—'}
                                </Box>
                                {!preserveMac ? (
                                  <Tooltip
                                    title="A new MAC address will be assigned in the destination"
                                    placement="right"
                                  >
                                    <WarningIcon sx={{ fontSize: 16, color: 'warning.main' }} />
                                  </Tooltip>
                                ) : null}
                              </Box>
                            </Box>
                          </Box>
                          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1, pt: 0.25 }}>
                            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                              <Switch
                                size="small"
                                checked={preserveIp}
                                disabled={isPoweredOff}
                                onChange={(e) =>
                                  handleBulkPreserveIpChange(vmId, interfaceIndex, e.target.checked)
                                }
                              />
                              <Typography variant="body2" sx={{ fontWeight: 600 }}>
                                Preserve IP
                              </Typography>
                            </Box>
                            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                              <Switch
                                size="small"
                                checked={preserveMac}
                                onChange={(e) =>
                                  handleBulkPreserveMacChange(
                                    vmId,
                                    interfaceIndex,
                                    e.target.checked
                                  )
                                }
                              />
                              <Typography variant="body2" sx={{ fontWeight: 600 }}>
                                Preserve MAC
                              </Typography>
                            </Box>
                          </Box>
                          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                            <TextField
                              value={ip}
                              onChange={(e) =>
                                handleBulkIpChange(vmId, interfaceIndex, e.target.value)
                              }
                              placeholder={
                                preserveIp ? 'Enter IP address' : 'Enter new IP (optional)'
                              }
                              size="small"
                              fullWidth
                              disabled={
                                preserveIp &&
                                Boolean(bulkExistingIPs?.[vmId]?.[interfaceIndex]?.trim())
                              }
                              error={status === 'invalid'}
                              InputProps={{
                                endAdornment: renderValidationAdornment(status)
                              }}
                              helperText={
                                preserveIp && !bulkExistingIPs?.[vmId]?.[interfaceIndex]?.trim()
                                  ? message
                                  : ''
                              }
                            />
                          </Box>
                        </Box>
                      )
                    })}
                  </Box>
                )
              })}
            </Box>
          </Box>
        </DialogContent>
        <DialogActions
          sx={{ justifyContent: 'flex-end', alignItems: 'center', gap: 1, px: 3, py: 2 }}
        >
          <ActionButton
            tone="secondary"
            onClick={handleCloseBulkEditDialog}
            disabled={assigningIPs}
            data-testid="rolling-migration-form-bulk-ip-cancel"
          >
            Cancel
          </ActionButton>
          <ActionButton
            tone="primary"
            onClick={handleApplyBulkIPs}
            disabled={!hasBulkIpsToApply || assigningIPs || hasBulkIpValidationErrors}
            loading={assigningIPs}
            data-testid="rolling-migration-form-bulk-ip-apply"
          >
            Apply Changes
          </ActionButton>
        </DialogActions>
      </Dialog>

      {/* Flavor Assignment Dialog */}
      <Dialog open={flavorDialogOpen} onClose={handleCloseFlavorDialog} fullWidth maxWidth="sm">
        <DialogTitle>
          Assign Flavor to {selectedVMs.length} {selectedVMs.length === 1 ? 'VM' : 'VMs'}
        </DialogTitle>
        <DialogContent>
          <Box sx={{ my: 2 }}>
            <FormLabel>Select Flavor</FormLabel>
            <Select
              fullWidth
              value={selectedFlavor}
              onChange={handleFlavorChange}
              size="small"
              sx={{ mt: 1 }}
              displayEmpty
            >
              <MenuItem value="">
                <em>Select a flavor</em>
              </MenuItem>
              <MenuItem value="auto-assign">
                <Box sx={{ display: 'flex', flexDirection: 'column' }}>
                  <Typography variant="body1">Auto Assign</Typography>
                  <Typography variant="caption" color="text.secondary">
                    Let OpenStack automatically assign the most suitable flavor
                  </Typography>
                </Box>
              </MenuItem>
              {openstackFlavors.map((flavor) => (
                <MenuItem key={flavor.id} value={flavor.id}>
                  <Box sx={{ display: 'flex', flexDirection: 'column' }}>
                    <Typography variant="body1">{flavor.name}</Typography>
                    <Typography variant="caption" color="text.secondary">
                      {flavor.vcpus} vCPU, {flavor.ram / 1024}GB RAM, {flavor.disk}GB Storage
                    </Typography>
                  </Box>
                </MenuItem>
              ))}
            </Select>
          </Box>
        </DialogContent>
        <DialogActions sx={{ gap: 1, p: 2 }}>
          <ActionButton tone="secondary" onClick={handleCloseFlavorDialog}>
            Cancel
          </ActionButton>
          <ActionButton
            tone="primary"
            onClick={handleApplyFlavor}
            disabled={!selectedFlavor || updating}
            loading={updating}
          >
            Apply to selected VMs
          </ActionButton>
        </DialogActions>
      </Dialog>

      {/* Toast Notification */}
      <Snackbar
        open={toastOpen}
        autoHideDuration={4000}
        onClose={handleCloseToast}
        anchorOrigin={{ vertical: 'top', horizontal: 'right' }}
      >
        <Alert
          onClose={handleCloseToast}
          severity={toastSeverity}
          sx={{ width: '100%' }}
          variant="standard"
        >
          {toastMessage}
        </Alert>
      </Snackbar>
    </>
  )
}
