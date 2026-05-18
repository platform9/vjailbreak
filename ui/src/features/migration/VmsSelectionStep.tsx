import {
  Chip,
  FormHelperText,
  Paper,
  styled,
  Tooltip,
  Box,
  Button,
  Autocomplete,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  MenuItem,
  Select,
  TextField,
  Typography,
  Snackbar,
  Alert,
  GlobalStyles,
  Switch,
  CircularProgress,
  InputAdornment
} from '@mui/material'
import {
  DataGrid,
  GridColDef,
  GridToolbarColumnsButton,
  GridRow
} from '@mui/x-data-grid'
import { VmData } from 'src/features/migration/api/migration-templates/model'
import { OpenStackFlavor } from 'src/api/openstack-creds/model'
import { CustomLoadingOverlay, CustomSearchToolbar } from 'src/components/grid'
import { Step } from 'src/shared/components/forms'
import * as React from 'react'
import { getMigrationPlans } from 'src/features/migration/api/migration-plans/migrationPlans'
import { useVMwareMachinesQuery } from 'src/hooks/api/useVMwareMachinesQuery'
import InfoIcon from '@mui/icons-material/Info'
import WarningIcon from '@mui/icons-material/Warning'
import CheckCircleIcon from '@mui/icons-material/CheckCircle'
import ErrorIcon from '@mui/icons-material/Error'
import WindowsIcon from 'src/assets/windows_icon.svg'
import LinuxIcon from 'src/assets/linux_icon.svg'
import { useErrorHandler } from 'src/hooks/useErrorHandler'
import { useAmplitude } from 'src/hooks/useAmplitude'
import { useRdmConfigValidation } from 'src/hooks/useRdmConfigValidation'
import { RdmDisk } from 'src/api/rdm-disks/model'
import { RdmDiskConfigurationPanel } from './components'
import { MissingInterfaceIpWarningAlert } from './components/MissingInterfaceIpWarningAlert'
import { getMissingInterfaceIpWarnings } from './components/missingInterfaceIpWarnings'
import { FieldLabel } from 'src/components'
import { ActionButton } from 'src/components'
import { TextField as SharedTextField } from 'src/shared/components/forms'
import type { VmDataWithFlavor, VmsSelectionStepProps, RdmConfiguration } from './types'
import { useOsAssignment } from './hooks/useOsAssignment'
import { useVmSelection } from './hooks/useVmSelection'
import { useFlavorAssignment } from './hooks/useFlavorAssignment'
import { useRdmConfiguration } from './hooks/useRdmConfiguration'
import { useBulkIPEdit } from './hooks/useBulkIPEdit'
import {
  extractFirstIPv4,
  hasMultipleIPv4,
  parseIpList,
} from './utils/ipValidation'

const { useCallback, useEffect, useState } = React

const VmsSelectionStepContainer = styled('div')(({ theme }) => ({
  display: 'grid',
  gridGap: theme.spacing(1),
  '& .disabled-row': {
    opacity: 0.6,
    cursor: 'not-allowed'
  },
  '& .hidden-column': {
    display: 'none'
  },
  '& .warning-row': {
    color: '#856404',
    fontWeight: 'bold'
  }
}))

const FieldsContainer = styled('div')({
  display: 'grid'
})

// Style for Clarity icons
const CdsIconWrapper = styled('div')({
  marginRight: 8,
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center'
})

const CustomToolbarWithActions = (props) => {
  const {
    rowSelectionModel,
    onAssignFlavor,
    onAssignIP,
    hasRdmVMs,
    onAssignRdmConfiguration,
    selectedCount,
    rdmVMsCount,
    ...toolbarProps
  } = props

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
            <Button variant="text" color="primary" onClick={onAssignFlavor} size="small">
              Assign Flavor ({rowSelectionModel.length})
            </Button>
            {hasRdmVMs && rdmVMsCount > 0 && (
              <Button
                variant="text"
                color="secondary"
                onClick={onAssignRdmConfiguration}
                size="small"
              >
                Configure RDM ({rdmVMsCount})
              </Button>
            )}
            {selectedCount > 0 && (
              <Button variant="text" color="primary" onClick={onAssignIP} size="small">
                Assign IP ({selectedCount})
              </Button>
            )}
          </>
        )}
        <CustomSearchToolbar {...toolbarProps} />
      </Box>
    </Box>
  )
}

// Column definition moved inside component to access state

const paginationModel = { page: 0, pageSize: 5 }

const MIGRATED_TOOLTIP_MESSAGE = 'This VM is migrating or already has been migrated.'
const FLAVOR_NOT_FOUND_MESSAGE =
  'Appropriate flavor not found. Please assign a flavor before selecting this VM for migration or create a flavor.'

function VmsSelectionStep({
  onChange,
  error,
  open = false,
  vmwareCredsValidated,
  openstackCredsValidated,
  sessionId = Date.now().toString(),
  openstackFlavors = [],
  vmwareCredName,
  openstackCredName,
  openstackCredentials,
  vmwareCluster,
  useGPU = false,
  showHeader = true
}: VmsSelectionStepProps) {
  const { reportError } = useErrorHandler({ component: 'VmsSelectionStep' })
  const { track } = useAmplitude({ component: 'VmsSelectionStep' })

  const normalizeNetworkInterfaces = (networkInterfaces?: VmData['networkInterfaces']) => {
    if (!networkInterfaces || networkInterfaces.length === 0) return networkInterfaces
    return networkInterfaces.map((nic) => ({
      ...nic,
      ipAddress: Array.isArray((nic as any).ipAddress)
        ? (nic as any).ipAddress
        : (nic as any).ipAddress
          ? [(nic as any).ipAddress]
          : []
    }))
  }

  const [migratedVms, setMigratedVms] = useState<Set<string>>(new Set())
  const [loadingMigratedVms, setLoadingMigratedVms] = useState(false)
  const [vmsWithFlavor, setVmsWithFlavor] = useState<VmDataWithFlavor[]>([])

  // Toast notification for IP assignments
  const [toastOpen, setToastOpen] = useState(false)
  const [toastMessage, setToastMessage] = useState('')
  const [toastSeverityIp, setToastSeverityIp] = useState<'success' | 'error' | 'warning' | 'info'>(
    'success'
  )

  // Toast notification helper
  const showToast = useCallback(
    (message: string, severity: 'success' | 'error' | 'warning' | 'info' = 'success') => {
      setToastMessage(message)
      setToastSeverityIp(severity)
      setToastOpen(true)
    },
    []
  )

  const handleCloseToast = useCallback((_event?: React.SyntheticEvent | Event, reason?: string) => {
    if (reason === 'clickaway') {
      return
    }
    setToastOpen(false)
  }, [])

  // rdmConfigurations hoisted before useVmSelection (selectedVMs dep ordering)
  const [rdmConfigurations, setRdmConfigurations] = useState<RdmConfiguration[]>([])

  const setFormVms = React.useMemo(() => onChange('vms'), [onChange])
  const setFormRdmConfigurations = React.useMemo(() => onChange('rdmConfigurations'), [onChange])

  const { selectedVMs, setSelectedVMs, handleVmSelection, isRowSelectable, rowSelectionModelArray } =
    useVmSelection({
      vmsWithFlavor,
      rdmConfigurations,
      setFormVms,
      setFormRdmConfigurations,
    })

  const { vmOSAssignments, handleOSAssignment } = useOsAssignment({
    vmsWithFlavor,
    setVmsWithFlavor,
    showToast,
    track,
    reportError,
  })

  const {
    rdmDisks,
    rdmDisksLoading,
    rdmConfigDialogOpen,
    rdmConfirmDialogOpen,
    setRdmConfirmDialogOpen,
    rdmUpdating,
    handleOpenRdmConfigurationDialog,
    handleCloseRdmConfigurationDialog,
    handleApplyRdmConfigurations,
    handleApplyRdmConfigurationsClick,
  } = useRdmConfiguration({
    selectedVMs,
    rdmConfigurations,
    openstackCredName,
    openstackCredentials,
    showToast,
    track,
    reportError,
  })

  // RDM validation logic
  const rdmValidation = useRdmConfigValidation({
    selectedVMs: Array.from(selectedVMs)
      .map((vmId) => vmsWithFlavor.find((vm) => vm.id === vmId))
      .filter(Boolean) as VmData[],
    rdmDisks: rdmDisks,
    backendVolumeTypeMap: openstackCredentials?.status?.openstack?.backendVolumeTypeMap
  })

  const {
    originalIPsPerVM,
    bulkEditDialogOpen,
    bulkEditIPs,
    bulkValidationStatus,
    bulkValidationMessages,
    bulkPreserveIp,
    bulkPreserveMac,
    bulkExistingIPs,
    bulkCurrentIPs,
    assigningIPs,
    hasBulkIpsToApply,
    hasBulkIpValidationErrors,
    handleOpenBulkIPAssignment,
    handleCloseBulkEditDialog,
    handleApplyBulkIPs,
    handleBulkPreserveIpChange,
    handleBulkPreserveMacChange,
    handleBulkIpChange,
    handleClearAllIPs,
  } = useBulkIPEdit({
    vmsWithFlavor,
    setVmsWithFlavor,
    selectedVMs,
    setFormVms,
    openstackCredentials,
    showToast,
    reportError,
  })

  const clusterName = React.useMemo(() => {
    if (!vmwareCluster) return undefined
    const parts = vmwareCluster.split(':')
    // The value is "credName:datacenter:clusterName"
    return parts.length === 3 ? parts[2] : undefined
  }, [vmwareCluster])

  const datacenterName = React.useMemo(() => {
    if (!vmwareCluster) return undefined
    const parts = vmwareCluster.split(':')
    // Extract datacenter from cluster ID
    return parts.length === 3 ? parts[1] : undefined
  }, [vmwareCluster])

  const duplicateNames = React.useMemo(() => {
    const counts = new Map<string, number>()
    vmsWithFlavor.forEach((vm) => counts.set(vm.name, (counts.get(vm.name) ?? 0) + 1))
    return new Set(
      Array.from(counts.entries())
        .filter(([, c]) => c > 1)
        .map(([n]) => n)
    )
  }, [vmsWithFlavor])

  // Define columns inside component to access state and functions
  const columns: GridColDef[] = [
    {
      field: 'name',
      headerName: 'VM Name',
      flex: 2.5,
      renderCell: (params) => {
        const displayName = duplicateNames.has(params.row.name)
          ? params.row.vmKey || params.value
          : params.value
        return (
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
            <Tooltip title={params.row.vmState === 'running' ? 'Running' : 'Stopped'}>
              <CdsIconWrapper>
                {/* eslint-disable-next-line @typescript-eslint/ban-ts-comment */}
                {/* @ts-ignore */}
                <cds-icon
                  shape="vm"
                  size="md"
                  badge={params.row.vmState === 'running' ? 'success' : 'danger'}
                >
                  {/* @ts-ignore */}
                </cds-icon>
              </CdsIconWrapper>
            </Tooltip>
            <Box>{displayName}</Box>
            {params.row.isMigrated && (
              <Chip variant="outlined" label="Migrated" color="info" size="small" />
            )}
            {params.row.flavorNotFound && (
              <Box display="flex" alignItems="center" gap={0.5}>
                <WarningIcon color="warning" fontSize="small" />
              </Box>
            )}
            {params.row.hasSharedRdm && (
              <Tooltip title="This VM has shared RDM disks">
                <Chip
                  variant="outlined"
                  label="RDM"
                  color="secondary"
                  size="small"
                  sx={{ fontSize: '0.7rem', height: '20px' }}
                />
              </Tooltip>
            )}
          </Box>
        )
      }
    },
    {
      field: 'ipAddress',
      headerName: 'IP Address(es)',
      flex: 0.8,
      minWidth: 190,
      hideable: true,
      renderCell: (params) => {
        const vm = params.row as VmDataWithFlavor
        const vmId = vm.id
        const isSelected = selectedVMs.has(vmId)
        const networkInterfaces = Array.isArray(vm.networkInterfaces) ? vm.networkInterfaces : []
        const hasMultipleInterfaces = networkInterfaces.length > 1
        const formatNicIps = (ips?: string[]) => {
          const cleaned = (Array.isArray(ips) ? ips : []).filter((ip) => ip && ip.trim() !== '')
          return cleaned.length > 0 ? cleaned.join(', ') : '—'
        }

        const getNicIpDisplay = (nic: any, index: number) => {
          const preserveIP =
            (vm as any)?.preserveIp?.[index] !== undefined
              ? (vm as any).preserveIp[index] !== false
              : nic?.preserveIP !== false
          if (preserveIP) {
            const original = originalIPsPerVM?.[vmId]?.[index] || ''
            if (original.trim() !== '') {
              return formatNicIps(parseIpList(original))
            }
          }
          return formatNicIps(nic?.ipAddress)
        }

        const ipDisplay = hasMultipleInterfaces
          ? networkInterfaces.map((nic, index) => getNicIpDisplay(nic as any, index)).join(', ')
          : getNicIpDisplay(networkInterfaces[0] as any, 0) !== '—'
            ? getNicIpDisplay(networkInterfaces[0] as any, 0)
            : vm.ipAddress || '—'

        const tooltipMessage = hasMultipleInterfaces
          ? "Use 'Assign IP' button in toolbar to edit IP addresses for multiple network interfaces"
          : "Use 'Assign IP' button in toolbar to assign IP address"

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
    },
    {
      field: 'osFamily',
      headerName: 'Operating System',
      flex: 1,
      hideable: true,
      renderCell: (params) => {
        const vmId = params.row.id
        const isSelected = selectedVMs.has(vmId)
        const powerState = params.row?.powerState
        const detectedOsFamily = params.row?.osFamily
        const assignedOsFamily = vmOSAssignments[vmId]
        const currentOsFamily = assignedOsFamily === undefined ? detectedOsFamily : assignedOsFamily
        // Show dropdown when:
        // - VM is selected
        if (isSelected) {
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
                // small optimization: keep menu mounted to avoid remount cost
                MenuProps={{ keepMounted: true }}
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

        let displayValue = currentOsFamily || 'Unknown'
        let icon: React.ReactNode = null

        if (currentOsFamily && currentOsFamily.toLowerCase().includes('windows')) {
          displayValue = 'Windows'
          icon = <img src={WindowsIcon} alt="Windows" style={{ width: 20, height: 20 }} />
        } else if (currentOsFamily && currentOsFamily.toLowerCase().includes('linux')) {
          displayValue = 'Linux'
          icon = <img src={LinuxIcon} alt="Linux" style={{ width: 20, height: 20 }} />
        } else if (currentOsFamily && currentOsFamily !== 'Unknown') {
          displayValue = 'Unknown'
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
              {(!currentOsFamily || currentOsFamily === 'Unknown') && (
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
      flex: 1.2,
      valueGetter: (value: string[]) => value?.join(', ') || '- '
    },
    {
      field: 'cpuCount',
      headerName: 'CPU',
      flex: 0.7,
      valueGetter: (value) => value || '- '
    },
    {
      field: 'memory',
      headerName: 'Memory (MB)',
      flex: 0.9,
      valueGetter: (value) => value || '- '
    },
    {
      field: 'esxHost',
      headerName: 'ESX Host',
      flex: 1,
      valueGetter: (value) => value || '—'
    },
    {
      field: 'flavor',
      headerName: 'Flavor',
      flex: 1,
      getApplyQuickFilterFn: () => null,
      valueGetter: (value) => value || 'auto-assign',
      renderHeader: () => (
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
          <div style={{ fontWeight: 500 }}>Flavor</div>
          <Tooltip title="Target PCD flavor to be assigned to this VM after migration.">
            <InfoIcon fontSize="small" sx={{ color: 'info.info', opacity: 0.7, cursor: 'help' }} />
          </Tooltip>
        </Box>
      )
    },
    {
      field: 'rdmDisks',
      headerName: 'RDM Disks',
      flex: 1.2,
      hideable: true,
      valueGetter: (value: string[]) => value?.join(', ') || '—',
      renderHeader: () => (
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
          <div style={{ fontWeight: 500 }}>RDM Disks</div>
          <Tooltip title="Raw Device Mapping disks associated with this VM.">
            <InfoIcon fontSize="small" sx={{ color: 'info.info', opacity: 0.7, cursor: 'help' }} />
          </Tooltip>
        </Box>
      )
    },
    // Hidden column for sorting by vmState
    {
      field: 'vmState',
      headerName: 'Status',
      flex: 1,
      sortable: true,
      sortComparator: (v1, v2) => {
        if (v1 === 'running' && v2 === 'stopped') return -1
        if (v1 === 'stopped' && v2 === 'running') return 1
        return 0
      }
    }
  ]

  // Removed getOpenstackAccessInfo - no longer needed with new API

  useEffect(() => {
    if (!open) {
      setSelectedVMs(new Set())
    }
  }, [open])

  const {
    data: vmList = [],
    isLoading: loadingVms,
    refetch: refreshVMList
  } = useVMwareMachinesQuery({
    vmwareCredsValidated,
    openstackCredsValidated,
    enabled: open,
    sessionId,
    vmwareCredName,
    clusterName,
    datacenterName
  })

  const {
    flavorDialogOpen,
    selectedFlavor,
    setSelectedFlavor,
    snackbarOpen,
    snackbarMessage,
    snackbarSeverity,
    updating: flavorUpdating,
    handleOpenFlavorDialog,
    handleCloseFlavorDialog,
    handleCloseSnackbar,
    handleApplyFlavor,
  } = useFlavorAssignment({
    selectedVMs,
    vmsWithFlavor,
    setVmsWithFlavor,
    openstackFlavors: openstackFlavors ?? [],
    vmList,
    refreshVMList,
    onChange,
    reportError,
  })

  useEffect(() => {
    if (open && vmwareCluster !== undefined) {
      refreshVMList()
    }
  }, [vmwareCluster])

  useEffect(() => {
    const fetchMigratedVms = async () => {
      if (!open) return

      setLoadingMigratedVms(true)
      try {
        const plans = await getMigrationPlans()
        const migratedVmSet = new Set<string>()

        plans.forEach((plan) => {
          plan.spec.virtualMachines.forEach((vmList) => {
            vmList.forEach((vm) => migratedVmSet.add(vm))
          })
        })

        setMigratedVms(migratedVmSet)
      } catch (error) {
        console.error('Error fetching migrated VMs:', error)
      } finally {
        setLoadingMigratedVms(false)
      }
    }

    fetchMigratedVms()
  }, [open, vmList])

  useEffect(() => {
    // Create a map of existing VM data (including assigned IPs) for quick lookup
    const existingVmsMap = new Map(vmsWithFlavor.map((vm) => [vm.id, vm]))

    const initialVmsWithFlavor = vmList.map((vm) => {
      let flavor = ''
      if (vm.targetFlavorId) {
        const foundFlavor = openstackFlavors.find((f) => f.id === vm.targetFlavorId)
        if (foundFlavor) {
          flavor = foundFlavor.name
        } else {
          flavor = vm.targetFlavorId
        }
      }

      // Check for NOT_FOUND label for OpenStack credentials
      const flavorNotFound = openstackCredName
        ? vm.labels?.[openstackCredName] === 'NOT_FOUND'
        : false

      // Map power state from vmState
      const powerState = vm.vmState === 'running' ? 'powered-on' : 'powered-off'

      // FIX: Use existing IP data from vmsWithFlavor if the VM exists,
      // otherwise, use the fresh data from vmList.
      const existingVm = existingVmsMap.get(vm.id)

      let allIPs = vm.networkInterfaces
        ? vm.networkInterfaces
            .flatMap((nic) => (Array.isArray(nic.ipAddress) ? nic.ipAddress : []))
            .filter((ip) => ip && ip.trim() !== '')
            .join(', ')
        : vm.ipAddress || ''

      // If the existing VM has a meaningful ipAddress (and not the placeholder '—'), prefer that
      // This preserves local assignments (assignedIPs, updated networkInterfaces) done elsewhere
      if (existingVm && existingVm.ipAddress && existingVm.ipAddress !== '—') {
        allIPs = existingVm.ipAddress ?? allIPs
      }

      // If existingVm has modified networkInterfaces, prefer them
      let preferredNetworkInterfaces = vm.networkInterfaces
      if (existingVm && existingVm.networkInterfaces && existingVm.networkInterfaces.length > 0) {
        preferredNetworkInterfaces = existingVm.networkInterfaces
      }

      preferredNetworkInterfaces = normalizeNetworkInterfaces(preferredNetworkInterfaces)

      // Use assigned OS family if available is handled by separate effect (do not include vmOSAssignments here)
      const finalOsFamily = vm.osFamily

      return {
        ...vm,
        ipAddress: allIPs || '—', // Update the main IP field to contain comma-separated IPs
        isMigrated:
          migratedVms.has(vm.vmKey || vm.name) ||
          migratedVms.has(vm.name) ||
          Boolean(vm.isMigrated),
        flavor,
        flavorNotFound,
        powerState,
        osFamily: finalOsFamily, // Use detected OS family (assignments applied separately)
        ipValidationStatus: 'pending' as const,
        ipValidationMessage: '',
        networkInterfaces: preferredNetworkInterfaces,
      }
    })
    setVmsWithFlavor(initialVmsWithFlavor)
  }, [
    vmList,
    migratedVms,
    openstackFlavors,
    openstackCredName,
    // removed vmOSAssignments here intentionally to avoid full remap when only OS assignment changes
    vmsWithFlavor.length
  ])



  const renderValidationAdornment = (status?: 'empty' | 'valid' | 'invalid' | 'validating') => {
    if (!status || status === 'empty') return null

    if (status === 'validating') {
      return (
        <InputAdornment position="end" sx={{ alignItems: 'center' }}>
          <CircularProgress size={16} />
        </InputAdornment>
      )
    }

    if (status === 'valid') {
      return (
        <InputAdornment position="end" sx={{ alignItems: 'center' }}>
          <CheckCircleIcon color="success" fontSize="small" />
        </InputAdornment>
      )
    }

    if (status === 'invalid') {
      return (
        <InputAdornment position="end" sx={{ alignItems: 'center' }}>
          <ErrorIcon color="error" fontSize="small" />
        </InputAdornment>
      )
    }

    return null
  }


  const getNoRowsLabel = () => {
    return 'No VMs discovered'
  }

  const missingInterfaceIpWarnings = React.useMemo(
    () => getMissingInterfaceIpWarnings(vmsWithFlavor.filter((vm) => selectedVMs.has(vm.id))),
    [selectedVMs, vmsWithFlavor]
  )

  return (
    <VmsSelectionStepContainer>
      {showHeader ? <Step stepNumber="2" label="Select Virtual Machines to Migrate" /> : null}
      <FieldsContainer>
        {rdmValidation.hasRdmVMs && (
          <Alert severity="info" sx={{ mb: 2 }}>
            <Box>
              <Typography variant="body2" sx={{ fontWeight: 'medium', mb: 1 }}>
                RDM (Raw Device Mapping) Migration Detected
              </Typography>
              <Typography variant="body2" color="text.secondary">
                Try and migrate VMs with shared RDM disks together as a group. You can configure
                Cinder backend settings for these disks using the "Configure RDM" button.
              </Typography>
            </Box>
          </Alert>
        )}
        <Box sx={{ mb: 1 }}>
          <FieldLabel label="Virtual Machines" required align="flex-start" />
        </Box>
        <Box>
          <Paper sx={{ width: '100%', height: 389 }}>
            <DataGrid
              rows={vmsWithFlavor}
              columns={columns}
              initialState={{
                pagination: { paginationModel },
                sorting: {
                  sortModel: [{ field: 'vmState', sort: 'asc' }]
                },
                columns: {
                  columnVisibilityModel: {
                    vmState: false, // Hide the vmState column that we use only for sorting
                    rdmDisks: false, // Hide the RDM disks column by default
                    networks: false, // Hide the networks column by default
                    esxHost: false // Hide the esxHost column by default
                  }
                }
              }}
              pageSizeOptions={[5, 10, 25]}
              localeText={{ noRowsLabel: getNoRowsLabel() }}
              rowHeight={45}
              onRowSelectionModelChange={handleVmSelection}
              rowSelectionModel={rowSelectionModelArray}
              getRowId={(row) => row.id}
              isRowSelectable={isRowSelectable}
              disableRowSelectionOnClick
              slots={{
                toolbar: (props) => {
                  return (
                    <CustomToolbarWithActions
                      {...props}
                      onRefresh={() => refreshVMList()}
                      disableRefresh={
                        loadingVms ||
                        loadingMigratedVms ||
                        !vmwareCredsValidated ||
                        !openstackCredsValidated
                      }
                      placeholder="Search by Name, Network Interface, CPU, or Memory"
                      rowSelectionModel={rowSelectionModelArray}
                      onAssignFlavor={handleOpenFlavorDialog}
                      onAssignRdmConfiguration={handleOpenRdmConfigurationDialog}
                      hasRdmVMs={rdmValidation.hasRdmVMs}
                      onAssignIP={handleOpenBulkIPAssignment}
                      selectedCount={rowSelectionModelArray.length}
                      rdmVMsCount={(() => {
                        const vmIdToName = new Map<string, string>(
                          vmsWithFlavor.map((v: VmDataWithFlavor) => [v.id, v.name] as [string, string])
                        )
                        return rowSelectionModelArray.filter((vmId: string) => {
                          const name = vmIdToName.get(vmId)
                          return (
                            name &&
                            rdmDisks.some((disk: RdmDisk) => disk.spec.ownerVMs.includes(name))
                          )
                        }).length
                      })()}
                    />
                  )
                },
                loadingOverlay: () => <CustomLoadingOverlay loadingMessage="Loading VMs ..." />,
                row: (props) => {
                  const isMigrated = props.row.isMigrated
                  const hasFlavorNotFound = props.row.flavorNotFound

                  let tooltipMessage = ''
                  if (isMigrated) {
                    tooltipMessage = MIGRATED_TOOLTIP_MESSAGE
                  } else if (hasFlavorNotFound) {
                    tooltipMessage = FLAVOR_NOT_FOUND_MESSAGE
                  }

                  return (
                    <Tooltip title={tooltipMessage} followCursor>
                      <span style={{ display: 'contents' }}>
                        <GridRow {...props} />
                      </span>
                    </Tooltip>
                  )
                }
              }}
              loading={loadingVms || loadingMigratedVms}
              checkboxSelection
              disableColumnMenu
              getRowClassName={(params) => {
                if (params.row.isMigrated) {
                  return 'disabled-row'
                } else {
                  return ''
                }
              }}
              keepNonExistentRowsSelected
            />
          </Paper>
        </Box>
        <MissingInterfaceIpWarningAlert warnings={missingInterfaceIpWarnings} sx={{ mt: 2 }} />
        {error && <FormHelperText error>{error}</FormHelperText>}
        {/* Separate RDM Error Messages */}
        {rdmValidation.hasSelectionError && (
          <Alert severity="error" sx={{ mt: 2 }}>
            {rdmValidation.selectionErrorMessage}
          </Alert>
        )}

        {rdmValidation.hasPowerStateError && (
          <Alert severity="error" sx={{ mt: 2 }}>
            {rdmValidation.powerStateErrorMessage}
          </Alert>
        )}

        {rdmValidation.hasVolumeTypeError && (
          <Alert severity="warning" sx={{ mt: 2 }}>
            {rdmValidation.volumeTypeErrorMessage}
          </Alert>
        )}

        {/* GPU Warning Message */}
        {(() => {
          const selectedVmsData = vmsWithFlavor.filter((vm) => selectedVMs.has(vm.id))
          const hasGPUVMs = selectedVmsData.some((vm) => (vm as any).useGPU)
          const hasAssignedFlavors = selectedVmsData.some((vm) => vm.targetFlavorId)

          if (hasGPUVMs && !useGPU && !hasAssignedFlavors) {
            return (
              <Alert severity="warning" sx={{ mt: 2 }}>
                You have selected VMs with GPU enabled. Please assign appropriate flavour or select
                "Use GPU enabled flavours" checkbox in Migration Options.
              </Alert>
            )
          }
          return null
        })()}
      </FieldsContainer>

      {/* Flavor Assignment Dialog */}
      <Dialog open={flavorDialogOpen} onClose={handleCloseFlavorDialog} fullWidth maxWidth="sm">
        <DialogTitle>
          Assign Flavor to {selectedVMs.size} {selectedVMs.size === 1 ? 'VM' : 'VMs'}
        </DialogTitle>
        <DialogContent>
          <Box sx={{ my: 2 }}>
            <FieldLabel label="Select Flavor" align="flex-start" />
            <Autocomplete
              sx={{ mt: 1 }}
              size="small"
              options={[
                { id: 'auto-assign', name: 'Auto-assign', vcpus: 0, ram: 0, disk: 0 },
                ...openstackFlavors
              ]}
              value={
                selectedFlavor
                  ? ([
                      { id: 'auto-assign', name: 'Auto-assign', vcpus: 0, ram: 0, disk: 0 },
                      ...openstackFlavors
                    ].find((f) => f.id === selectedFlavor) ?? null)
                  : null
              }
              onChange={(_e, value) => {
                setSelectedFlavor(value?.id ?? '')
              }}
              getOptionLabel={(option) => {
                if (option.id === 'auto-assign') return option.name
                return `${option.name} (${option.vcpus} vCPU, ${option.ram}MB RAM, ${option.disk}GB Disk)`
              }}
              isOptionEqualToValue={(option, value) => option.id === value.id}
              renderInput={(params) => (
                <SharedTextField {...params} placeholder="Search flavors" fullWidth />
              )}
            />
          </Box>
        </DialogContent>
        <DialogActions
          sx={{ justifyContent: 'flex-end', alignItems: 'center', gap: 1, px: 3, py: 2 }}
        >
          <ActionButton tone="secondary" onClick={handleCloseFlavorDialog} disabled={flavorUpdating}>
            Cancel
          </ActionButton>
          <ActionButton
            tone="primary"
            onClick={handleApplyFlavor}
            disabled={!selectedFlavor || flavorUpdating}
            loading={flavorUpdating}
          >
            Apply to selected VMs
          </ActionButton>
        </DialogActions>
      </Dialog>

      {/* RDM Configuration Dialog */}
      <Dialog
        open={rdmConfigDialogOpen}
        onClose={handleCloseRdmConfigurationDialog}
        fullWidth
        maxWidth="md"
      >
        <DialogTitle>
          Configure RDM Disks for {selectedVMs.size} {selectedVMs.size === 1 ? 'VM' : 'VMs'}
        </DialogTitle>
        <DialogContent>
          {/* Debug info */}
          {process.env.NODE_ENV === 'development' && (
            <Box sx={{ mb: 2, p: 1, bgcolor: 'grey.100', fontSize: '0.8rem' }}>
              Debug: hasRdmVMs={String(rdmValidation.hasRdmVMs)}, rdmDisks.length={rdmDisks.length},
              loading={String(rdmDisksLoading)}
            </Box>
          )}
          {rdmDisksLoading ? (
            <Box sx={{ p: 2, textAlign: 'center' }}>
              <CircularProgress size={24} sx={{ mr: 1 }} />
              <Typography color="text.secondary">Loading RDM disk information...</Typography>
            </Box>
          ) : rdmValidation.hasRdmVMs && rdmDisks.length > 0 ? (
            <RdmDiskConfigurationPanel
              rdmDisks={(() => {
                const vmIdToName = new Map<string, string>(
                  vmsWithFlavor.map((v: VmDataWithFlavor) => [v.id, v.name] as [string, string])
                )
                const selectedVMNames = new Set(
                  Array.from(selectedVMs)
                    .map((vmId) => vmIdToName.get(vmId as string))
                    .filter((n): n is string => !!n)
                )
                return rdmDisks.filter((disk: RdmDisk) =>
                  disk.spec.ownerVMs.some((ownerVM: string) => selectedVMNames.has(ownerVM))
                )
              })()}
              openstackCreds={openstackCredentials}
              selectedVMs={Array.from(selectedVMs)}
              onConfigurationChange={setRdmConfigurations}
            />
          ) : (
            <Box sx={{ p: 2, textAlign: 'center' }}>
              <Typography color="text.secondary">
                {rdmValidation.hasRdmVMs
                  ? 'No RDM disk configurations found.'
                  : 'No RDM disks detected for selected VMs.'}
              </Typography>
            </Box>
          )}
        </DialogContent>
        <DialogActions
          sx={{ justifyContent: 'flex-end', alignItems: 'center', gap: 1, px: 3, py: 2 }}
        >
          <ActionButton
            tone="secondary"
            onClick={handleCloseRdmConfigurationDialog}
            disabled={rdmUpdating}
          >
            Close
          </ActionButton>
          {rdmValidation.hasRdmVMs && rdmDisks.length > 0 && (
            <ActionButton
              tone="primary"
              onClick={handleApplyRdmConfigurationsClick}
              disabled={
                rdmUpdating ||
                !rdmConfigurations ||
                rdmConfigurations.length === 0 ||
                rdmConfigurations.some((config) => !config.cinderBackendPool || !config.volumeType)
              }
              loading={rdmUpdating}
            >
              Apply RDM Configuration
            </ActionButton>
          )}
        </DialogActions>
      </Dialog>

      {/* RDM Volume Type Warning Confirmation Dialog */}
      <Dialog
        open={rdmConfirmDialogOpen}
        onClose={() => setRdmConfirmDialogOpen(false)}
        maxWidth="sm"
        fullWidth
      >
        <DialogTitle>Confirm RDM Configuration</DialogTitle>
        <DialogContent>
          <Alert severity="warning" sx={{ mb: 2 }}>
            <Typography variant="body2">
              One or more RDM disk configurations have volume type mismatches with the selected
              backend pool. This may cause issues during migration.
            </Typography>
          </Alert>
          <Typography variant="body2">
            Are you sure you want to apply this configuration?
          </Typography>
        </DialogContent>
        <DialogActions sx={{ px: 3, py: 2 }}>
          <ActionButton tone="secondary" onClick={() => setRdmConfirmDialogOpen(false)}>
            Cancel
          </ActionButton>
          <ActionButton tone="primary" onClick={handleApplyRdmConfigurations}>
            Yes, Apply Configuration
          </ActionButton>
        </DialogActions>
      </Dialog>

      <Snackbar
        open={snackbarOpen}
        autoHideDuration={6000}
        onClose={handleCloseSnackbar}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'left' }}
      >
        <Alert onClose={handleCloseSnackbar} severity={snackbarSeverity}>
          {snackbarMessage}
        </Alert>
      </Snackbar>

      {/* Bulk IP Editor Dialog */}
      <Dialog open={bulkEditDialogOpen} onClose={handleCloseBulkEditDialog} maxWidth="md" fullWidth>
        <DialogTitle>
          Edit IP Addresses for {selectedVMs.size} {selectedVMs.size === 1 ? 'VM' : 'VMs'}
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
                const vm = vmsWithFlavor.find((v) => v.id === vmId)
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
                      <Tooltip title={vm.vmState === 'running' ? 'Running' : 'Stopped'}>
                        <CdsIconWrapper>
                          {/* eslint-disable-next-line @typescript-eslint/ban-ts-comment */}
                          {/* @ts-ignore */}
                          <cds-icon
                            shape="vm"
                            size="md"
                            badge={vm.vmState === 'running' ? 'success' : 'danger'}
                          >
                            {/* @ts-ignore */}
                          </cds-icon>
                        </CdsIconWrapper>
                      </Tooltip>
                      <Typography variant="body2" sx={{ fontWeight: 700 }}>
                        {duplicateNames.has(vm.name) ? vm.vmKey || vm.name : vm.name}
                      </Typography>
                    </Box>

                    {Object.entries(interfaces).map(([interfaceIndexStr, ip]) => {
                      const interfaceIndex = parseInt(interfaceIndexStr)
                      const networkInterface = vm.networkInterfaces?.[interfaceIndex]
                      const status = bulkValidationStatus[vmId]?.[interfaceIndex]
                      const message = bulkValidationMessages[vmId]?.[interfaceIndex]
                      const isPoweredOff = vm.vmState !== 'running'
                      const preserveIp =
                        !isPoweredOff && bulkPreserveIp?.[vmId]?.[interfaceIndex] !== false
                      const preserveMac = bulkPreserveMac?.[vmId]?.[interfaceIndex] !== false
                      const discoveredIp = bulkExistingIPs?.[vmId]?.[interfaceIndex] || ''
                      const currentIp =
                        bulkCurrentIPs?.[vmId]?.[interfaceIndex] ||
                        (Array.isArray(networkInterface?.ipAddress)
                          ? networkInterface?.ipAddress
                              ?.filter((v) => v && v.trim() !== '')
                              .join(', ')
                          : '') ||
                        ''
                      const displayIp = preserveIp ? discoveredIp : currentIp
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
                          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.75 }}>
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
                                {displayIp.trim() !== ''
                                  ? displayIp
                                  : !preserveIp &&
                                      !networkInterface &&
                                      interfaceIndex === 0 &&
                                      !hasMultipleIPv4(vm.ipAddress || '')
                                    ? extractFirstIPv4(vm.ipAddress || '')
                                    : '—'}
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
                              InputProps={{
                                endAdornment: renderValidationAdornment(status)
                              }}
                              error={status === 'invalid'}
                              helperText={status === 'invalid' ? message || 'Invalid IP' : ''}
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
          >
            Cancel
          </ActionButton>
          <ActionButton
            tone="primary"
            onClick={handleApplyBulkIPs}
            disabled={!hasBulkIpsToApply || assigningIPs || hasBulkIpValidationErrors}
            loading={assigningIPs}
          >
            Apply Changes
          </ActionButton>
        </DialogActions>
      </Dialog>

      {/* Add GlobalStyles similar to RollingMigrationForm */}
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

      {/* Toast Notification for IP assignments */}
      <Snackbar
        open={toastOpen}
        autoHideDuration={4000}
        onClose={handleCloseToast}
        anchorOrigin={{ vertical: 'top', horizontal: 'right' }}
      >
        <Alert
          onClose={handleCloseToast}
          severity={toastSeverityIp}
          sx={{ width: '100%' }}
          variant="standard"
        >
          {toastMessage}
        </Alert>
      </Snackbar>
    </VmsSelectionStepContainer>
  )
}
 
const areOpenstackFlavorsEqual = (prev?: OpenStackFlavor[], next?: OpenStackFlavor[]): boolean => {
  if (prev === next) {
    return true
  }

  if (!prev || !next || prev.length !== next.length) {
    return false
  }

  return prev.every((prevFlavor, index) => {
    const nextFlavor = next[index]
    return (
      prevFlavor.id === nextFlavor.id &&
      prevFlavor.name === nextFlavor.name &&
      prevFlavor.disk === nextFlavor.disk &&
      prevFlavor.ram === nextFlavor.ram &&
      prevFlavor.vcpus === nextFlavor.vcpus
    )
  })
}

const arePropsEqual = (
  prevProps: VmsSelectionStepProps,
  nextProps: VmsSelectionStepProps
): boolean => {
  if (prevProps.onChange !== nextProps.onChange) {
    return false
  }

  if (prevProps.open !== nextProps.open) return false
  if (prevProps.error !== nextProps.error) return false
  if (prevProps.vmwareCredsValidated !== nextProps.vmwareCredsValidated) return false
  if (prevProps.openstackCredsValidated !== nextProps.openstackCredsValidated) return false
  if (prevProps.sessionId !== nextProps.sessionId) return false
  if (!areOpenstackFlavorsEqual(prevProps.openstackFlavors, nextProps.openstackFlavors))
    return false
  if (prevProps.vmwareCredName !== nextProps.vmwareCredName) return false
  if (prevProps.openstackCredName !== nextProps.openstackCredName) return false
  if (prevProps.openstackCredentials !== nextProps.openstackCredentials) return false
  if (prevProps.vmwareCluster !== nextProps.vmwareCluster) return false

  return true
}

export default React.memo(VmsSelectionStep, arePropsEqual)
