import React, { useState, useMemo, useEffect, useCallback } from 'react'
import {
  Box,
  Button,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Paper,
  Select,
  MenuItem,
  TextField,
  Switch,
  FormLabel,
  Tooltip,
  InputAdornment,
  CircularProgress,
  Snackbar,
  Alert,
  Typography
} from '@mui/material'
import {
  DataGrid,
  GridColDef,
  GridRowSelectionModel,
  GridToolbarColumnsButton
} from '@mui/x-data-grid'
import CheckCircleIcon from '@mui/icons-material/CheckCircle'
import ErrorIcon from '@mui/icons-material/Error'
import WarningIcon from '@mui/icons-material/Warning'
import { styled } from '@mui/material/styles'
import { ActionButton } from 'src/components'
import { CustomSearchToolbar } from 'src/components/grid'
import { MissingInterfaceIpWarningAlert } from '../components/MissingInterfaceIpWarningAlert'
import { getMissingInterfaceIpWarnings } from '../components/missingInterfaceIpWarnings'
import { patchVMwareMachine } from 'src/api/vmware-machines/vmwareMachines'
import { VJAILBREAK_DEFAULT_NAMESPACE } from 'src/api/constants'
import { useBulkIPHandlers } from '../hooks/useBulkIPHandlers'
import { useFlavorHandlers } from '../hooks/useFlavorHandlers'
import WindowsIcon from 'src/assets/windows_icon.svg'
import LinuxIcon from 'src/assets/linux_icon.svg'
import '@cds/core/icon/register.js'
import { ClarityIcons, vmIcon } from '@cds/core/icon'
import type { VM } from '../types'
import type { OpenstackCreds } from 'src/api/openstack-creds/model'
import type { ErrorContext } from 'src/services/errorReporting'
import {
  extractFirstIPv4,
  hasMultipleIPv4,
} from '../utils/ipValidation'

ClarityIcons.addIcons(vmIcon)

const CdsIconWrapper = styled('div')({
  marginRight: 8,
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center'
})

const CustomToolbarWithActions = (props: any) => {
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

interface RollingVmsSelectionStepProps {
  vmsWithAssignments: VM[]
  setVmsWithAssignments: React.Dispatch<React.SetStateAction<VM[]>>
  selectedVMs: GridRowSelectionModel
  onSelectionChange: (ids: GridRowSelectionModel) => void
  vmOSAssignments: Record<string, string>
  setVmOSAssignments: React.Dispatch<React.SetStateAction<Record<string, string>>>
  openstackCredData: OpenstackCreds | null
  loadingVMs: boolean
  reportError: (error: Error, additionalContext?: ErrorContext) => void
  fetchClusterVMs: () => Promise<void>
  vmIpValidationError: string
  osValidationError: string
}

export default function RollingVmsSelectionStep({
  vmsWithAssignments,
  setVmsWithAssignments,
  selectedVMs,
  onSelectionChange,
  vmOSAssignments,
  setVmOSAssignments,
  openstackCredData,
  loadingVMs,
  reportError,
  fetchClusterVMs,
  vmIpValidationError,
  osValidationError,
}: RollingVmsSelectionStepProps) {
  const paginationModel = { page: 0, pageSize: 5 }
  const [toastOpen, setToastOpen] = useState(false)
  const [toastMessage] = useState('')
  const [toastSeverity] = useState<'success' | 'error' | 'warning' | 'info'>('success')

  const handleCloseToast = useCallback((_event?: React.SyntheticEvent | Event, reason?: string) => {
    if (reason === 'clickaway') return
    setToastOpen(false)
  }, [])

  const openstackFlavors = useMemo(() => openstackCredData?.spec?.flavors || [], [openstackCredData])

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
    handleCloseBulkEditDialog,
    handleBulkPreserveIpChange,
    handleBulkPreserveMacChange,
    handleBulkIpChange,
    handleClearAllIPs,
    handleApplyBulkIPs,
    handleOpenBulkIPAssignment
  } = useBulkIPHandlers({
    vmsWithAssignments,
    setVmsWithAssignments,
    selectedVMs,
    openstackCredData,
    reportError
  })

  const {
    flavorDialogOpen,
    selectedFlavor,
    updating,
    handleOpenFlavorDialog,
    handleCloseFlavorDialog,
    handleFlavorChange,
    handleIndividualFlavorChange,
    handleApplyFlavor
  } = useFlavorHandlers({
    vmsWithAssignments,
    setVmsWithAssignments,
    selectedVMs,
    openstackFlavors,
    reportError,
    fetchClusterVMs
  })

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

      const hasChanges = updatedVMs.some(
        (vm, index) => vm.flavor !== vmsWithAssignments[index]?.flavor
      )

      if (hasChanges) {
        setVmsWithAssignments(updatedVMs)
      }
    }
  }, [openstackFlavors, vmsWithAssignments])

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
      setVmOSAssignments((prev) => {
        const newState = { ...prev }
        delete newState[vmId]
        return newState
      })
    }
  }

  const missingInterfaceIpWarnings = useMemo(
    () =>
      getMissingInterfaceIpWarnings(
        vmsWithAssignments.filter((vm) => selectedVMs.includes(vm.id))
      ),
    [vmsWithAssignments, selectedVMs]
  )

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
        <InputAdornment position="end">
          <CheckCircleIcon color="success" fontSize="small" />
        </InputAdornment>
      )
    }

    if (status === 'invalid') {
      return (
        <InputAdornment position="end">
          <ErrorIcon color="error" fontSize="small" />
        </InputAdornment>
      )
    }

    return null
  }

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

        if (powerState === 'powered-off') {
          let ipDisplay = ''
          let tooltipMessage = ''

          if (vm.networkInterfaces && vm.networkInterfaces.length > 1) {
            ipDisplay = vm.networkInterfaces.map((nic) => nic.ipAddress || '—').join(', ')
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

        const currentIp = vm.ip || '—'

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

  return (
    <>
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
            onSelectionChange(selectedRowIds)
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
      <MissingInterfaceIpWarningAlert
        warnings={missingInterfaceIpWarnings}
        sx={{ mt: 2 }}
      />
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

      {/* Bulk IP Assignment Dialog */}
      <Dialog open={bulkEditDialogOpen} onClose={handleCloseBulkEditDialog} maxWidth="md" fullWidth>
        <DialogTitle>
          Edit IP Addresses for {selectedVMs.length} {selectedVMs.length === 1 ? 'VM' : 'VMs'}
        </DialogTitle>
        <DialogContent dividers>
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
            <Box sx={{ display: 'flex', justifyContent: { xs: 'flex-start', sm: 'flex-end' } }}>
              <Button size="small" variant="outlined" onClick={handleClearAllIPs}>
                Clear All
              </Button>
            </Box>

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
