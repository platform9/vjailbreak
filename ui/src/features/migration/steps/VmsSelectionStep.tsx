import {
  Chip,
  FormHelperText,
  Paper,
  styled,
  Tooltip,
  Box,
  Button,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  MenuItem,
  Select,
  Typography,
  Snackbar,
  Alert,
  GlobalStyles,
  CircularProgress,
} from '@mui/material'
import {
  DataGrid,
  GridColDef,
  GridToolbarColumnsButton,
  GridRow,
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
import WindowsIcon from 'src/assets/windows_icon.svg'
import LinuxIcon from 'src/assets/linux_icon.svg'
import { useErrorHandler } from 'src/hooks/useErrorHandler'
import { useAmplitude } from 'src/hooks/useAmplitude'
import { useRdmConfigValidation } from 'src/hooks/useRdmConfigValidation'
import { RdmDisk } from 'src/api/rdm-disks/model'
import { RdmDiskConfigurationPanel, BulkIPEditDialog, FlavorAssignmentDialog } from '../components'
import { MissingInterfaceIpWarningAlert } from '../components/MissingInterfaceIpWarningAlert'
import { getMissingInterfaceIpWarnings } from '../components/missingInterfaceIpWarnings'
import { FieldLabel } from 'src/components'
import { ActionButton } from 'src/components'
import type { VmDataWithFlavor, VmsSelectionStepProps, RdmConfiguration } from '../types'
import { useOsAssignment } from '../hooks/useOsAssignment'
import { useVmSelection } from '../hooks/useVmSelection'
import { useFlavorAssignment } from '../hooks/useFlavorAssignment'
import { useRdmConfiguration } from '../hooks/useRdmConfiguration'
import { useBulkIPEdit } from '../hooks/useBulkIPEdit'
import { useBulkIPHandlers } from '../hooks/useBulkIPHandlers'
import { useFlavorHandlers } from '../hooks/useFlavorHandlers'
import { parseIpList } from '../utils/ipValidation'
import { fromVmDataWithFlavor, fromVM } from '../utils/vmAdapters'
import { patchVMwareMachine } from 'src/api/vmware-machines/vmwareMachines'
import { VJAILBREAK_DEFAULT_NAMESPACE } from 'src/api/constants'
import '@cds/core/icon/register.js'
import { ClarityIcons, vmIcon } from '@cds/core/icon'

ClarityIcons.addIcons(vmIcon)

const { useCallback, useEffect, useMemo, useState } = React

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

const CdsIconWrapper = styled('div')({
  marginRight: 8,
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center'
})

const StandardToolbarWithActions = (props: any) => {
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

const RollingToolbarWithActions = (props: any) => {
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

const defaultPaginationModel = { page: 0, pageSize: 5 }

const MIGRATED_TOOLTIP_MESSAGE = 'This VM is migrating or already has been migrated.'
const FLAVOR_NOT_FOUND_MESSAGE =
  'Appropriate flavor not found. Please assign a flavor before selecting this VM for migration or create a flavor.'

function VmsSelectionStep({
  mode = 'standard',
  // standard props
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
  showHeader = true,
  // rolling props
  vmsWithAssignments: vmsWithAssignmentsProp = [],
  setVmsWithAssignments: setVmsWithAssignmentsProp,
  vmOSAssignments: vmOSAssignmentsProp = {},
  setVmOSAssignments: setVmOSAssignmentsProp,
  selectedVMs: selectedVMsProp = [],
  onSelectionChange,
  loadingVMs = false,
  vmIpValidationError = '',
  osValidationError = '',
  fetchClusterVMs,
  openstackCredData,
  reportError: reportErrorProp,
}: VmsSelectionStepProps) {
  const isRolling = mode === 'rolling'

  // --- Error / analytics ---
  const { reportError: internalReportError } = useErrorHandler({ component: 'VmsSelectionStep' })
  const reportError = isRolling && reportErrorProp ? reportErrorProp : internalReportError
  const { track } = useAmplitude({ component: 'VmsSelectionStep' })

  // --- Standard-mode state ---
  const [migratedVms, setMigratedVms] = useState<Set<string>>(new Set())
  const [loadingMigratedVms, setLoadingMigratedVms] = useState(false)
  const [vmsWithFlavor, setVmsWithFlavor] = useState<VmDataWithFlavor[]>([])
  const [rdmConfigurations, setRdmConfigurations] = useState<RdmConfiguration[]>([])

  // --- Toast ---
  const [toastOpen, setToastOpen] = useState(false)
  const [toastMessage, setToastMessage] = useState('')
  const [toastSeverityIp, setToastSeverityIp] = useState<'success' | 'error' | 'warning' | 'info'>(
    'success'
  )
  const showToast = useCallback(
    (message: string, severity: 'success' | 'error' | 'warning' | 'info' = 'success') => {
      setToastMessage(message)
      setToastSeverityIp(severity)
      setToastOpen(true)
    },
    []
  )
  const handleCloseToast = useCallback(
    (_event?: React.SyntheticEvent | Event, reason?: string) => {
      if (reason === 'clickaway') return
      setToastOpen(false)
    },
    []
  )

  // --- Stable no-ops for inactive-mode hook defaults ---
  const noOpFn = useCallback(() => {}, [])
  const noOpAsync = useCallback(async () => {}, [])
  const noOpSetter = useCallback(() => {}, []) as React.Dispatch<React.SetStateAction<any>>

  // --- Rolling props with fallbacks ---
  const vmsWithAssignments = vmsWithAssignmentsProp ?? []
  const setVmsWithAssignments = setVmsWithAssignmentsProp ?? noOpSetter
  const rollingSelectedVMs = selectedVMsProp ?? []

  // --- Standard form callbacks ---
  const setFormVms = useMemo(() => onChange?.('vms') ?? noOpFn, [onChange, noOpFn])
  const setFormRdmConfigurations = useMemo(
    () => onChange?.('rdmConfigurations') ?? noOpFn,
    [onChange, noOpFn]
  )

  // --- Standard: vm selection ---
  const {
    selectedVMs: selectedVMsStandard,
    setSelectedVMs,
    handleVmSelection,
    isRowSelectable,
    rowSelectionModelArray,
  } = useVmSelection({
    vmsWithFlavor,
    rdmConfigurations,
    setFormVms,
    setFormRdmConfigurations,
  })

  // --- Standard: OS assignment ---
  const { vmOSAssignments: standardVmOSAssignments, handleOSAssignment: standardHandleOSAssignment } =
    useOsAssignment({
      vmsWithFlavor,
      setVmsWithFlavor,
      showToast,
      track,
      reportError,
    })

  const vmOSAssignments = isRolling ? (vmOSAssignmentsProp ?? {}) : standardVmOSAssignments

  // --- Standard: RDM ---
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
    selectedVMs: selectedVMsStandard,
    rdmConfigurations,
    openstackCredName,
    openstackCredentials,
    showToast,
    track,
    reportError,
  })

  const rdmValidation = useRdmConfigValidation({
    selectedVMs: Array.from(selectedVMsStandard)
      .map((vmId) => vmsWithFlavor.find((vm) => vm.id === vmId))
      .filter(Boolean) as VmData[],
    rdmDisks,
    backendVolumeTypeMap: openstackCredentials?.status?.openstack?.backendVolumeTypeMap,
  })

  // --- Standard: bulk IP edit ---
  const standardBulkIP = useBulkIPEdit({
    vmsWithFlavor,
    setVmsWithFlavor,
    selectedVMs: selectedVMsStandard,
    setFormVms,
    openstackCredentials,
    showToast,
    reportError,
  })

  // --- Standard: VMware query ---
  const clusterName = useMemo(() => {
    if (!vmwareCluster) return undefined
    const parts = vmwareCluster.split(':')
    return parts.length === 3 ? parts[2] : undefined
  }, [vmwareCluster])

  const datacenterName = useMemo(() => {
    if (!vmwareCluster) return undefined
    const parts = vmwareCluster.split(':')
    return parts.length === 3 ? parts[1] : undefined
  }, [vmwareCluster])

  const {
    data: vmList = [],
    isLoading: loadingVms,
    refetch: refreshVMList,
  } = useVMwareMachinesQuery({
    vmwareCredsValidated,
    openstackCredsValidated,
    enabled: !isRolling && open,
    sessionId,
    vmwareCredName,
    clusterName,
    datacenterName,
  })

  // --- Standard: flavor assignment ---
  const standardFlavor = useFlavorAssignment({
    selectedVMs: selectedVMsStandard,
    vmsWithFlavor,
    setVmsWithFlavor,
    openstackFlavors: openstackFlavors ?? [],
    vmList,
    refreshVMList,
    onChange: onChange ?? (() => () => {}),
    reportError,
  })

  // --- Rolling: bulk IP handlers ---
  const rollingBulkIP = useBulkIPHandlers({
    vmsWithAssignments,
    setVmsWithAssignments,
    selectedVMs: rollingSelectedVMs,
    openstackCredData: openstackCredData ?? null,
    reportError,
  })

  // --- Rolling: flavor handlers ---
  const rollingOpenstackFlavors = useMemo(
    () => openstackCredData?.spec?.flavors ?? [],
    [openstackCredData]
  )

  const rollingFlavor = useFlavorHandlers({
    vmsWithAssignments,
    setVmsWithAssignments,
    selectedVMs: rollingSelectedVMs,
    openstackFlavors: rollingOpenstackFlavors,
    reportError,
    fetchClusterVMs: fetchClusterVMs ?? noOpAsync,
  })

  // --- Rolling: OS assignment handler ---
  const handleRollingOSAssignment = useCallback(
    async (vmId: string, osFamily: string) => {
      try {
        setVmOSAssignmentsProp?.((prev) => ({ ...prev, [vmId]: osFamily }))
        await patchVMwareMachine(
          vmId,
          { spec: { vms: { osFamily } } },
          VJAILBREAK_DEFAULT_NAMESPACE
        )
        setVmsWithAssignments((prev: typeof vmsWithAssignments) =>
          prev.map((v) => (v.id === vmId ? { ...v, osFamily } : v))
        )
      } catch (err) {
        reportError(err as Error, {
          context: 'os-family-assignment',
          metadata: { vmId, osFamily, action: 'os-family-assignment' },
        })
        setVmOSAssignmentsProp?.((prev) => {
          const next = { ...prev }
          delete next[vmId]
          return next
        })
      }
    },
    [setVmOSAssignmentsProp, setVmsWithAssignments, reportError]
  )

  // --- Rolling: sync flavor names from openstackFlavors ---
  useEffect(() => {
    if (!isRolling) return
    if (rollingOpenstackFlavors.length === 0 || vmsWithAssignments.length === 0) return
    const updatedVMs = vmsWithAssignments.map((vm) => {
      if (vm.targetFlavorId) {
        const flavorObj = rollingOpenstackFlavors.find((f) => f.id === vm.targetFlavorId)
        if (flavorObj && vm.flavor !== flavorObj.name) {
          return { ...vm, flavor: flavorObj.name }
        }
      }
      return vm
    })
    const hasChanges = updatedVMs.some((vm, i) => vm.flavor !== vmsWithAssignments[i]?.flavor)
    if (hasChanges) setVmsWithAssignments(updatedVMs)
  }, [isRolling, rollingOpenstackFlavors, vmsWithAssignments, setVmsWithAssignments])

  // --- Standard: duplicate names ---
  const duplicateNames = useMemo(() => {
    const counts = new Map<string, number>()
    vmsWithFlavor.forEach((vm) => counts.set(vm.name, (counts.get(vm.name) ?? 0) + 1))
    return new Set(
      Array.from(counts.entries())
        .filter(([, c]) => c > 1)
        .map(([n]) => n)
    )
  }, [vmsWithFlavor])

  // --- Canonical VMs for dialogs ---
  const canonicalVMs = useMemo(
    () =>
      isRolling ? vmsWithAssignments.map(fromVM) : vmsWithFlavor.map(fromVmDataWithFlavor),
    [isRolling, vmsWithAssignments, vmsWithFlavor]
  )

  // --- Standard effects ---
  useEffect(() => {
    if (!open) setSelectedVMs(new Set())
  }, [open])

  useEffect(() => {
    if (!isRolling && open && vmwareCluster !== undefined) {
      refreshVMList()
    }
  }, [vmwareCluster])

  useEffect(() => {
    if (isRolling) return
    const fetchMigratedVms = async () => {
      if (!open) return
      setLoadingMigratedVms(true)
      try {
        const plans = await getMigrationPlans()
        const migratedVmSet = new Set<string>()
        plans.forEach((plan) => {
          plan.spec.virtualMachines.forEach((list) => {
            list.forEach((vm) => migratedVmSet.add(vm))
          })
        })
        setMigratedVms(migratedVmSet)
      } catch (err) {
        console.error('Error fetching migrated VMs:', err)
      } finally {
        setLoadingMigratedVms(false)
      }
    }
    fetchMigratedVms()
  }, [open, vmList, isRolling])

  const normalizeNetworkInterfaces = (networkInterfaces?: VmData['networkInterfaces']) => {
    if (!networkInterfaces || networkInterfaces.length === 0) return networkInterfaces
    return networkInterfaces.map((nic) => ({
      ...nic,
      ipAddress: Array.isArray((nic as any).ipAddress)
        ? (nic as any).ipAddress
        : (nic as any).ipAddress
          ? [(nic as any).ipAddress]
          : [],
    }))
  }

  useEffect(() => {
    if (isRolling) return
    const existingVmsMap = new Map(vmsWithFlavor.map((vm) => [vm.id, vm]))
    const initialVmsWithFlavor = vmList.map((vm) => {
      let flavor = ''
      if (vm.targetFlavorId) {
        const foundFlavor = openstackFlavors.find((f) => f.id === vm.targetFlavorId)
        flavor = foundFlavor ? foundFlavor.name : vm.targetFlavorId
      }

      const flavorNotFound = openstackCredName
        ? vm.labels?.[openstackCredName] === 'NOT_FOUND'
        : false

      const powerState = vm.vmState === 'running' ? 'powered-on' : 'powered-off'
      const existingVm = existingVmsMap.get(vm.id)

      let allIPs = vm.networkInterfaces
        ? vm.networkInterfaces
            .flatMap((nic) => (Array.isArray(nic.ipAddress) ? nic.ipAddress : []))
            .filter((ip) => ip && ip.trim() !== '')
            .join(', ')
        : vm.ipAddress || ''

      if (existingVm && existingVm.ipAddress && existingVm.ipAddress !== '—') {
        allIPs = existingVm.ipAddress ?? allIPs
      }

      let preferredNetworkInterfaces = vm.networkInterfaces
      if (existingVm && existingVm.networkInterfaces && existingVm.networkInterfaces.length > 0) {
        preferredNetworkInterfaces = existingVm.networkInterfaces
      }
      preferredNetworkInterfaces = normalizeNetworkInterfaces(preferredNetworkInterfaces)

      return {
        ...vm,
        ipAddress: allIPs || '—',
        isMigrated:
          migratedVms.has(vm.vmKey || vm.name) ||
          migratedVms.has(vm.name) ||
          Boolean(vm.isMigrated),
        flavor,
        flavorNotFound,
        powerState,
        osFamily: vm.osFamily,
        ipValidationStatus: 'pending' as const,
        ipValidationMessage: '',
        networkInterfaces: preferredNetworkInterfaces,
      }
    })
    setVmsWithFlavor(initialVmsWithFlavor)
  }, [vmList, migratedVms, openstackFlavors, openstackCredName, isRolling, vmsWithFlavor.length])

  // --- Missing IP warnings ---
  const missingInterfaceIpWarnings = useMemo(
    () =>
      isRolling
        ? getMissingInterfaceIpWarnings(
            vmsWithAssignments.filter((vm) => rollingSelectedVMs.includes(vm.id))
          )
        : getMissingInterfaceIpWarnings(vmsWithFlavor.filter((vm) => selectedVMsStandard.has(vm.id))),
    [isRolling, vmsWithAssignments, rollingSelectedVMs, vmsWithFlavor, selectedVMsStandard]
  )

  // --- Standard columns ---
  const standardColumns: GridColDef[] = [
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
      },
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
        const isSelected = selectedVMsStandard.has(vmId)
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
            const original = standardBulkIP.originalIPsPerVM?.[vmId]?.[index] || ''
            if (original.trim() !== '') return formatNicIps(parseIpList(original))
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
              minWidth: 0,
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
                textOverflow: 'ellipsis',
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
      },
    },
    {
      field: 'osFamily',
      headerName: 'Operating System',
      flex: 1,
      hideable: true,
      renderCell: (params) => {
        const vmId = params.row.id
        const isSelected = selectedVMsStandard.has(vmId)
        const powerState = params.row?.powerState
        const detectedOsFamily = params.row?.osFamily
        const assignedOsFamily = vmOSAssignments[vmId]
        const currentOsFamily = assignedOsFamily === undefined ? detectedOsFamily : assignedOsFamily
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
                onChange={(e) => standardHandleOSAssignment(vmId, e.target.value)}
                displayEmpty
                sx={{
                  minWidth: 120,
                  '& .MuiSelect-select': { padding: '4px 8px', fontSize: '0.875rem' },
                }}
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
            <Box sx={{ display: 'flex', alignItems: 'center', height: '100%', gap: 1 }}>
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
                      : 'text.primary',
                }}
              >
                {displayValue}
              </Typography>
            </Box>
          </Tooltip>
        )
      },
    },
    {
      field: 'networks',
      headerName: 'Network Interface(s)',
      flex: 1.2,
      valueGetter: (value: string[]) => value?.join(', ') || '- ',
    },
    {
      field: 'cpuCount',
      headerName: 'CPU',
      flex: 0.7,
      valueGetter: (value) => value || '- ',
    },
    {
      field: 'memory',
      headerName: 'Memory (MB)',
      flex: 0.9,
      valueGetter: (value) => value || '- ',
    },
    {
      field: 'esxHost',
      headerName: 'ESX Host',
      flex: 1,
      valueGetter: (value) => value || '—',
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
      ),
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
      ),
    },
    {
      field: 'vmState',
      headerName: 'Status',
      flex: 1,
      sortable: true,
      sortComparator: (v1, v2) => {
        if (v1 === 'running' && v2 === 'stopped') return -1
        if (v1 === 'stopped' && v2 === 'running') return 1
        return 0
      },
    },
  ]

  // --- Rolling columns ---
  const rollingColumns: GridColDef[] = [
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
      ),
    },
    {
      field: 'ip',
      headerName: 'IP Address(es)',
      flex: 1,
      hideable: true,
      renderCell: (params) => {
        const vm = params.row
        const vmId = vm.id
        const isSelected = rollingSelectedVMs.includes(vmId)
        const powerState = vm.powerState

        if (powerState === 'powered-off') {
          let ipDisplay = ''
          let tooltipMessage = ''
          if (vm.networkInterfaces && vm.networkInterfaces.length > 1) {
            ipDisplay = vm.networkInterfaces.map((nic: any) => nic.ipAddress || '—').join(', ')
            tooltipMessage =
              "Use 'Assign IP' button in toolbar to edit IP addresses for multiple network interfaces"
          } else {
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
                minWidth: 0,
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
                  textOverflow: 'ellipsis',
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

        const currentIp = vm.ip || '—'
        if (powerState === 'powered-on') {
          return (
            <Tooltip title="IP assignment is only available for powered-off VMs" arrow>
              <Box
                sx={{ display: 'flex', alignItems: 'center', width: '100%', height: '100%' }}
              >
                <Typography
                  variant="body2"
                  sx={{
                    flex: 1,
                    minWidth: 0,
                    whiteSpace: 'nowrap',
                    overflow: 'hidden',
                    textOverflow: 'ellipsis',
                  }}
                >
                  {currentIp}
                </Typography>
              </Box>
            </Tooltip>
          )
        }
        return (
          <Box sx={{ display: 'flex', alignItems: 'center', width: '100%', height: '100%' }}>
            <Typography variant="body2">{currentIp}</Typography>
          </Box>
        )
      },
    },
    {
      field: 'osFamily',
      headerName: 'Operating System',
      flex: 1,
      hideable: true,
      renderCell: (params) => {
        const vmId = params.row.id
        const isSelected = rollingSelectedVMs.includes(vmId)
        const powerState = params.row?.powerState
        const detectedOsFamily = params.row?.osFamily
        const assignedOsFamily = vmOSAssignments[vmId]
        const currentOsFamily = assignedOsFamily || detectedOsFamily

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
                onChange={(e) => handleRollingOSAssignment(vmId, e.target.value)}
                displayEmpty
                sx={{
                  minWidth: 120,
                  '& .MuiSelect-select': { padding: '4px 8px', fontSize: '0.875rem' },
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
            <Box sx={{ display: 'flex', alignItems: 'center', height: '100%', gap: 1 }}>
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
                      : 'text.primary',
                }}
              >
                {displayValue}
              </Typography>
            </Box>
          </Tooltip>
        )
      },
    },
    {
      field: 'networks',
      headerName: 'Network Interface(s)',
      flex: 1,
      hideable: true,
      valueGetter: (value) => value || '—',
    },
    {
      field: 'cpu',
      headerName: 'CPU',
      flex: 0.3,
      hideable: true,
      valueGetter: (value) => value || '- ',
    },
    {
      field: 'memory',
      headerName: 'Memory (MB)',
      flex: 0.8,
      hideable: true,
      valueGetter: (value) => value || '—',
    },
    {
      field: 'esxHost',
      headerName: 'ESX Host',
      flex: 1,
      hideable: true,
      valueGetter: (value) => value || '—',
    },
    {
      field: 'flavor',
      headerName: 'Flavor',
      flex: 1,
      hideable: true,
      renderCell: (params) => {
        const vmId = params.row.id
        const isSelected = rollingSelectedVMs.includes(vmId)
        const currentFlavor = params.value || 'auto-assign'

        if (isSelected) {
          return (
            <Box sx={{ display: 'flex', alignItems: 'center', height: '100%', width: '100%' }}>
              <Select
                size="small"
                value={(() => {
                  if (currentFlavor === 'auto-assign') return 'auto-assign'
                  const flavorByName = rollingOpenstackFlavors.find((f) => f.name === currentFlavor)
                  const flavorById = rollingOpenstackFlavors.find((f) => f.id === currentFlavor)
                  return flavorByName?.id || flavorById?.id || currentFlavor
                })()}
                onChange={(e) => rollingFlavor.handleIndividualFlavorChange(vmId, e.target.value)}
                displayEmpty
                sx={{
                  minWidth: 120,
                  width: '100%',
                  '& .MuiSelect-select': { padding: '4px 8px', fontSize: '0.875rem' },
                }}
              >
                <MenuItem value="auto-assign">
                  <Typography variant="body2">Auto Assign</Typography>
                </MenuItem>
                {rollingOpenstackFlavors.map((flavor) => (
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
            sx={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              height: '100%',
            }}
          >
            <Typography variant="body2">{currentFlavor}</Typography>
          </Box>
        )
      },
    },
    {
      field: 'powerState',
      headerName: 'Power State',
      hideable: true,
      flex: 0.8,
      valueGetter: (value) => value || '—',
    },
  ]

  // --- Rolling: filtered selection (only VMs present in current list) ---
  const rollingFilteredSelection = useMemo(
    () => rollingSelectedVMs.filter((vmId) => vmsWithAssignments.some((vm) => vm.id === vmId)),
    [rollingSelectedVMs, vmsWithAssignments]
  )

  return (
    <VmsSelectionStepContainer>
      {!isRolling && showHeader && (
        <Step stepNumber="2" label="Select Virtual Machines to Migrate" />
      )}
      {isRolling && (
        <Typography variant="caption" color="text.secondary" sx={{ mt: 1, display: 'block', mb: 1 }}>
          Tip: Powered-off VMs require IP Address and OS assignment for proper migration
          configuration
        </Typography>
      )}

      <FieldsContainer>
        {!isRolling && rdmValidation.hasRdmVMs && (
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

        {!isRolling && (
          <Box sx={{ mb: 1 }}>
            <FieldLabel label="Virtual Machines" required align="flex-start" />
          </Box>
        )}

        <Box>
          <Paper
            sx={{ width: '100%', height: 389 }}
            data-testid={isRolling ? 'rolling-migration-form-vms-grid' : undefined}
          >
            {isRolling ? (
              <DataGrid
                rows={vmsWithAssignments}
                columns={rollingColumns}
                initialState={{
                  pagination: { paginationModel: defaultPaginationModel },
                  columns: { columnVisibilityModel: {} },
                }}
                pageSizeOptions={[5, 10, 25]}
                rowHeight={52}
                checkboxSelection
                disableRowSelectionOnClick
                onRowSelectionModelChange={(ids) => onSelectionChange?.(ids)}
                rowSelectionModel={rollingFilteredSelection}
                slots={{
                  toolbar: (props) => (
                    <RollingToolbarWithActions
                      {...props}
                      rowSelectionModel={rollingFilteredSelection}
                      onAssignFlavor={rollingFlavor.handleOpenFlavorDialog}
                      onAssignIP={rollingBulkIP.handleOpenBulkIPAssignment}
                      hasPoweredOffVMs={rollingSelectedVMs.some((vmId) => {
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
                        p: 2,
                      }}
                    >
                      <Typography
                        variant="body1"
                        color="text.secondary"
                        align="center"
                        sx={{ mb: 1 }}
                      >
                        {loadingVMs ? 'Loading VMs...' : 'No VMs found for the selected cluster.'}
                      </Typography>
                    </Box>
                  ),
                }}
                disableColumnFilter
                loading={loadingVMs}
              />
            ) : (
              <DataGrid
                rows={vmsWithFlavor}
                columns={standardColumns}
                initialState={{
                  pagination: { paginationModel: defaultPaginationModel },
                  sorting: { sortModel: [{ field: 'vmState', sort: 'asc' }] },
                  columns: {
                    columnVisibilityModel: {
                      vmState: false,
                      rdmDisks: false,
                      networks: false,
                      esxHost: false,
                    },
                  },
                }}
                pageSizeOptions={[5, 10, 25]}
                localeText={{ noRowsLabel: 'No VMs discovered' }}
                rowHeight={45}
                onRowSelectionModelChange={handleVmSelection}
                rowSelectionModel={rowSelectionModelArray}
                getRowId={(row) => row.id}
                isRowSelectable={isRowSelectable}
                disableRowSelectionOnClick
                slots={{
                  toolbar: (props) => (
                    <StandardToolbarWithActions
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
                      onAssignFlavor={standardFlavor.handleOpenFlavorDialog}
                      onAssignRdmConfiguration={handleOpenRdmConfigurationDialog}
                      hasRdmVMs={rdmValidation.hasRdmVMs}
                      onAssignIP={standardBulkIP.handleOpenBulkIPAssignment}
                      selectedCount={rowSelectionModelArray.length}
                      rdmVMsCount={(() => {
                        const vmIdToName = new Map<string, string>(
                          vmsWithFlavor.map(
                            (v: VmDataWithFlavor) => [v.id, v.name] as [string, string]
                          )
                        )
                        return rowSelectionModelArray.filter((vmId: string) => {
                          const name = vmIdToName.get(vmId)
                          return (
                            name && rdmDisks.some((disk: RdmDisk) => disk.spec.ownerVMs.includes(name))
                          )
                        }).length
                      })()}
                    />
                  ),
                  loadingOverlay: () => <CustomLoadingOverlay loadingMessage="Loading VMs ..." />,
                  row: (props) => {
                    const isMigrated = props.row.isMigrated
                    const hasFlavorNotFound = props.row.flavorNotFound
                    let tooltipMessage = ''
                    if (isMigrated) tooltipMessage = MIGRATED_TOOLTIP_MESSAGE
                    else if (hasFlavorNotFound) tooltipMessage = FLAVOR_NOT_FOUND_MESSAGE
                    return (
                      <Tooltip title={tooltipMessage} followCursor>
                        <span style={{ display: 'contents' }}>
                          <GridRow {...props} />
                        </span>
                      </Tooltip>
                    )
                  },
                }}
                loading={loadingVms || loadingMigratedVms}
                checkboxSelection
                disableColumnMenu
                getRowClassName={(params) => (params.row.isMigrated ? 'disabled-row' : '')}
                keepNonExistentRowsSelected
              />
            )}
          </Paper>
        </Box>

        <MissingInterfaceIpWarningAlert warnings={missingInterfaceIpWarnings} sx={{ mt: 2 }} />

        {!isRolling && error && <FormHelperText error>{error}</FormHelperText>}

        {isRolling && vmIpValidationError && (
          <Alert severity="warning" sx={{ mt: 2 }}>
            {vmIpValidationError}
          </Alert>
        )}
        {isRolling && osValidationError && (
          <Alert severity="warning" sx={{ mt: 2 }}>
            {osValidationError}
          </Alert>
        )}

        {!isRolling && rdmValidation.hasSelectionError && (
          <Alert severity="error" sx={{ mt: 2 }}>
            {rdmValidation.selectionErrorMessage}
          </Alert>
        )}
        {!isRolling && rdmValidation.hasPowerStateError && (
          <Alert severity="error" sx={{ mt: 2 }}>
            {rdmValidation.powerStateErrorMessage}
          </Alert>
        )}
        {!isRolling && rdmValidation.hasVolumeTypeError && (
          <Alert severity="warning" sx={{ mt: 2 }}>
            {rdmValidation.volumeTypeErrorMessage}
          </Alert>
        )}

        {!isRolling &&
          (() => {
            const selectedVmsData = vmsWithFlavor.filter((vm) => selectedVMsStandard.has(vm.id))
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
      <FlavorAssignmentDialog
        open={isRolling ? rollingFlavor.flavorDialogOpen : standardFlavor.flavorDialogOpen}
        selectedVMCount={isRolling ? rollingSelectedVMs.length : selectedVMsStandard.size}
        flavors={isRolling ? rollingOpenstackFlavors : (openstackFlavors ?? [])}
        selectedFlavor={isRolling ? rollingFlavor.selectedFlavor : standardFlavor.selectedFlavor}
        updating={isRolling ? rollingFlavor.updating : standardFlavor.updating}
        onClose={
          isRolling
            ? rollingFlavor.handleCloseFlavorDialog
            : standardFlavor.handleCloseFlavorDialog
        }
        onApply={isRolling ? rollingFlavor.handleApplyFlavor : standardFlavor.handleApplyFlavor}
        onFlavorChange={
          isRolling ? rollingFlavor.setSelectedFlavor : standardFlavor.setSelectedFlavor
        }
      />

      {/* RDM Configuration Dialog (standard only) */}
      {!isRolling && (
        <Dialog
          open={rdmConfigDialogOpen}
          onClose={handleCloseRdmConfigurationDialog}
          fullWidth
          maxWidth="md"
        >
          <DialogTitle>
            Configure RDM Disks for {selectedVMsStandard.size}{' '}
            {selectedVMsStandard.size === 1 ? 'VM' : 'VMs'}
          </DialogTitle>
          <DialogContent>
            {process.env.NODE_ENV === 'development' && (
              <Box sx={{ mb: 2, p: 1, bgcolor: 'grey.100', fontSize: '0.8rem' }}>
                Debug: hasRdmVMs={String(rdmValidation.hasRdmVMs)}, rdmDisks.length=
                {rdmDisks.length}, loading={String(rdmDisksLoading)}
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
                    Array.from(selectedVMsStandard)
                      .map((vmId) => vmIdToName.get(vmId as string))
                      .filter((n): n is string => !!n)
                  )
                  return rdmDisks.filter((disk: RdmDisk) =>
                    disk.spec.ownerVMs.some((ownerVM: string) => selectedVMNames.has(ownerVM))
                  )
                })()}
                openstackCreds={openstackCredentials}
                selectedVMs={Array.from(selectedVMsStandard)}
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
      )}

      {/* RDM Confirm Dialog (standard only) */}
      {!isRolling && (
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
      )}

      {/* Flavor snackbar (standard only) */}
      {!isRolling && (
        <Snackbar
          open={standardFlavor.snackbarOpen}
          autoHideDuration={6000}
          onClose={standardFlavor.handleCloseSnackbar}
          anchorOrigin={{ vertical: 'bottom', horizontal: 'left' }}
        >
          <Alert onClose={standardFlavor.handleCloseSnackbar} severity={standardFlavor.snackbarSeverity}>
            {standardFlavor.snackbarMessage}
          </Alert>
        </Snackbar>
      )}

      {/* Bulk IP Edit Dialog */}
      <BulkIPEditDialog
        open={isRolling ? rollingBulkIP.bulkEditDialogOpen : standardBulkIP.bulkEditDialogOpen}
        selectedVMCount={isRolling ? rollingSelectedVMs.length : selectedVMsStandard.size}
        vms={canonicalVMs}
        bulkEditIPs={isRolling ? rollingBulkIP.bulkEditIPs : standardBulkIP.bulkEditIPs}
        bulkPreserveIp={isRolling ? rollingBulkIP.bulkPreserveIp : standardBulkIP.bulkPreserveIp}
        bulkPreserveMac={
          isRolling ? rollingBulkIP.bulkPreserveMac : standardBulkIP.bulkPreserveMac
        }
        bulkExistingIPs={
          isRolling ? rollingBulkIP.bulkExistingIPs : standardBulkIP.bulkExistingIPs
        }
        bulkCurrentIPs={!isRolling ? standardBulkIP.bulkCurrentIPs : undefined}
        bulkValidationStatus={
          isRolling ? rollingBulkIP.bulkValidationStatus : standardBulkIP.bulkValidationStatus
        }
        bulkValidationMessages={
          isRolling ? rollingBulkIP.bulkValidationMessages : standardBulkIP.bulkValidationMessages
        }
        assigningIPs={isRolling ? rollingBulkIP.assigningIPs : standardBulkIP.assigningIPs}
        hasBulkIpsToApply={
          isRolling ? rollingBulkIP.hasBulkIpsToApply : standardBulkIP.hasBulkIpsToApply
        }
        hasBulkIpValidationErrors={
          isRolling
            ? rollingBulkIP.hasBulkIpValidationErrors
            : standardBulkIP.hasBulkIpValidationErrors
        }
        duplicateNames={!isRolling ? duplicateNames : undefined}
        onClose={
          isRolling
            ? rollingBulkIP.handleCloseBulkEditDialog
            : standardBulkIP.handleCloseBulkEditDialog
        }
        onApply={isRolling ? rollingBulkIP.handleApplyBulkIPs : standardBulkIP.handleApplyBulkIPs}
        onClearAll={isRolling ? rollingBulkIP.handleClearAllIPs : standardBulkIP.handleClearAllIPs}
        onPreserveIpChange={
          isRolling
            ? rollingBulkIP.handleBulkPreserveIpChange
            : standardBulkIP.handleBulkPreserveIpChange
        }
        onPreserveMacChange={
          isRolling
            ? rollingBulkIP.handleBulkPreserveMacChange
            : standardBulkIP.handleBulkPreserveMacChange
        }
        onIpChange={
          isRolling ? rollingBulkIP.handleBulkIpChange : standardBulkIP.handleBulkIpChange
        }
      />

      {/* GlobalStyles (standard only) */}
      {!isRolling && (
        <GlobalStyles
          styles={{
            '.MuiDataGrid-columnsManagement, .MuiDataGrid-columnsManagementPopover': {
              '& .MuiFormControlLabel-label': { fontSize: '0.875rem !important' },
              '& .MuiCheckbox-root': { padding: '4px !important' },
              '& .MuiListItem-root': {
                fontSize: '0.875rem !important',
                minHeight: '32px !important',
                padding: '2px 8px !important',
              },
              '& .MuiTypography-root': { fontSize: '0.875rem !important' },
              '& .MuiInputBase-input': { fontSize: '0.875rem !important' },
              '& .MuiTextField-root .MuiInputBase-input': { fontSize: '0.875rem !important' },
            },
          }}
        />
      )}

      {/* Toast */}
      <Snackbar
        open={toastOpen}
        autoHideDuration={4000}
        onClose={handleCloseToast}
        anchorOrigin={{ vertical: 'top', horizontal: 'right' }}
      >
        <Alert onClose={handleCloseToast} severity={toastSeverityIp} sx={{ width: '100%' }} variant="standard">
          {toastMessage}
        </Alert>
      </Snackbar>
    </VmsSelectionStepContainer>
  )
}

const areOpenstackFlavorsEqual = (prev?: OpenStackFlavor[], next?: OpenStackFlavor[]): boolean => {
  if (prev === next) return true
  if (!prev || !next || prev.length !== next.length) return false
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
  if (prevProps.mode !== nextProps.mode) return false
  // Rolling mode always re-renders (parent owns state)
  if (prevProps.mode === 'rolling') return false

  if (prevProps.onChange !== nextProps.onChange) return false
  if (prevProps.open !== nextProps.open) return false
  if (prevProps.error !== nextProps.error) return false
  if (prevProps.vmwareCredsValidated !== nextProps.vmwareCredsValidated) return false
  if (prevProps.openstackCredsValidated !== nextProps.openstackCredsValidated) return false
  if (prevProps.sessionId !== nextProps.sessionId) return false
  if (!areOpenstackFlavorsEqual(prevProps.openstackFlavors, nextProps.openstackFlavors)) return false
  if (prevProps.vmwareCredName !== nextProps.vmwareCredName) return false
  if (prevProps.openstackCredName !== nextProps.openstackCredName) return false
  if (prevProps.openstackCredentials !== nextProps.openstackCredentials) return false
  if (prevProps.vmwareCluster !== nextProps.vmwareCluster) return false
  return true
}

export default React.memo(VmsSelectionStep, arePropsEqual)
