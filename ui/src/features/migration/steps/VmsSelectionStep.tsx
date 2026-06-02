import {
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
  Typography,
  Snackbar,
  Alert,
  GlobalStyles,
  CircularProgress,
} from '@mui/material'
import {
  DataGrid,
  GridToolbarColumnsButton,
  GridRow,
  GridRowSelectionModel,
} from '@mui/x-data-grid'
import { CustomLoadingOverlay, CustomSearchToolbar } from 'src/components/grid'
import { Step } from 'src/shared/components/forms'
import * as React from 'react'
import { RdmDisk } from 'src/api/rdm-disks/model'
import {
  RdmDiskConfigurationPanel,
  BulkIPEditDialog,
  FlavorAssignmentDialog,
} from '../components'
import { MissingInterfaceIpWarningAlert } from '../components/MissingInterfaceIpWarningAlert'
import { FieldLabel } from 'src/components'
import { ActionButton } from 'src/components'
import type { VmDataWithFlavor, VmsSelectionStepProps } from '../types'
import { useVmsSelectionState } from '../hooks/useVmsSelectionState'
import type { OpenStackFlavor } from 'src/api/openstack-creds/model'
import '@cds/core/icon/register.js'
import { ClarityIcons, vmIcon } from '@cds/core/icon'

ClarityIcons.addIcons(vmIcon)

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

interface StandardToolbarWithActionsProps {
  rowSelectionModel: GridRowSelectionModel
  onAssignFlavor: () => void
  onAssignIP: () => void
  hasRdmVMs: boolean
  onAssignRdmConfiguration: () => void
  selectedCount: number
  rdmVMsCount: number
  onRefresh?: () => void
  disableRefresh?: boolean
  placeholder?: string
  [key: string]: unknown
}

interface RollingToolbarWithActionsProps {
  rowSelectionModel: GridRowSelectionModel
  onAssignFlavor: () => void
  onAssignIP: () => void
  hasPoweredOffVMs: boolean
  [key: string]: unknown
}

const StandardToolbarWithActions = (props: StandardToolbarWithActionsProps) => {
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
            <Button data-testid="assign-flavor-button" variant="text" color="primary" onClick={onAssignFlavor} size="small">
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
              <Button data-testid="bulk-ip-edit-button" variant="text" color="primary" onClick={onAssignIP} size="small">
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

const RollingToolbarWithActions = (props: RollingToolbarWithActionsProps) => {
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

function VmsSelectionStep(props: VmsSelectionStepProps) {
  const { onRevalidateCreds } = props
  const {
    isRolling,
    showHeader,
    error,
    loadingVMs,
    vmIpValidationError,
    osValidationError,
    useGPU,
    onSelectionChange,
    vmwareCredsValidated,
    openstackCredsValidated,
    openstackCredentials,
    vmsWithFlavor,
    setRdmConfigurations,
    loadingVms,
    loadingMigratedVms,
    selectedVMsStandard,
    handleVmSelection,
    isRowSelectable,
    rowSelectionModelArray,
    refreshVMList,
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
    rdmValidation,
    standardBulkIP,
    standardFlavor,
    rdmConfigurations,
    vmsWithAssignments,
    rollingSelectedVMs,
    rollingBulkIP,
    rollingFlavor,
    standardColumns,
    rollingColumns,
    rollingFilteredSelection,
    missingInterfaceIpWarnings,
    toastOpen,
    toastMessage,
    toastSeverityIp,
    handleCloseToast,
    flavorDialogProps,
    bulkIPDialogProps,
  } = useVmsSelectionState(props)

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
            data-testid="vms-datagrid"
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
                      onRefresh={() => { refreshVMList(); onRevalidateCreds?.() }}
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
      <FlavorAssignmentDialog {...flavorDialogProps} />

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
      <BulkIPEditDialog {...bulkIPDialogProps} />

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
