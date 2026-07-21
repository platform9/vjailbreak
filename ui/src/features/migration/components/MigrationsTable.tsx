import { GridColDef, GridRowSelectionModel } from '@mui/x-data-grid'
import { Button, Typography, Box, IconButton, Tooltip, Dialog, DialogTitle, DialogContent, DialogContentText, DialogActions, Alert, LinearProgress } from '@mui/material'
import DeleteIcon from '@mui/icons-material/DeleteOutlined'
import VisibilityIcon from '@mui/icons-material/Visibility'
import ReplayIcon from '@mui/icons-material/Replay'
import FiberManualRecordIcon from '@mui/icons-material/FiberManualRecord'
import { useCallback, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { CustomSearchToolbar, ListingToolbar } from 'src/components/grid'
import { CommonDataGrid } from 'src/components/grid'
import { Phase } from '../api/migrations'
import MigrationProgress from '../components/MigrationProgress'
import MigrationStatusChip from '../components/MigrationStatusChip'
import { calculateTimeElapsed, formatDateTime } from 'src/utils'
import { TriggerAdminCutoverButton } from '.'
import PlayArrowIcon from '@mui/icons-material/PlayArrow'
import { triggerAdminCutover, deleteMigration } from '../api/migrations'
import { ConfirmationDialog } from 'src/components/dialogs'
import { keyframes } from '@mui/material/styles'
import { useMigrationFormActions } from '../context/MigrationFormContext'
import type { CustomToolbarProps, MigrationsTableProps } from '../types'
import { TooltipContent } from 'src/components'
import { useMigrationPlanDestinationsQuery } from '../api/useMigrationPlanDestinationsQuery'
import { STATUS_ORDER } from '../constants'
import {
  getProgressText,
  getMigrationStatusCategory,
  STATUS_FILTER_TO_CATEGORY
} from '../utils/migrationTableUtils'

const pulse = keyframes`
  0% {
    opacity: 1;
  }
  50% {
    opacity: 0.4;
  }
  100% {
    opacity: 1;
  }
`

const CustomToolbar = ({
  numSelected,
  onDeleteSelected,
  onBulkAdminCutover,
  numEligibleForCutover,
  onBulkRetry,
  numEligibleForRetry,
  isBulkRetryLoading,
  filteredCount,
  totalCount
}: CustomToolbarProps) => {
  const search = (
    <Box sx={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 2 }}>
      {numSelected > 0 ? (
        <>
          <Button
            data-testid="delete-selected-button"
            variant="outlined"
            color="error"
            startIcon={<DeleteIcon />}
            onClick={onDeleteSelected}
            sx={{ height: 40 }}
          >
            Delete Selected ({numSelected})
          </Button>

          {numEligibleForCutover > 0 && (
            <Button
              variant="outlined"
              color="primary"
              startIcon={<PlayArrowIcon />}
              onClick={onBulkAdminCutover}
              sx={{ height: 40 }}
            >
              Trigger Cutover ({numEligibleForCutover})
            </Button>
          )}

          {numEligibleForRetry > 0 && (
            <Button
              data-testid="bulk-retry-button"
              variant="outlined"
              color="primary"
              startIcon={<ReplayIcon />}
              onClick={onBulkRetry}
              disabled={isBulkRetryLoading}
              sx={{ height: 40 }}
            >
              Retry Selected ({numEligibleForRetry})
            </Button>
          )}
        </>
      ) : null}
      <CustomSearchToolbar placeholder="Search VM, tenant, OS..." />
    </Box>
  )

  const actions = (
    <Typography variant="body2" color="text.secondary" data-testid="migrations-showing-count">
      Showing {filteredCount} of {totalCount}
    </Typography>
  )

  return <ListingToolbar search={search} actions={actions} />
}

export default function MigrationsTable({
  migrations,
  onDeleteMigration,
  onDeleteSelected,
  refetchMigrations,
  loading = false,
  statusFilter: statusFilterProp
}: MigrationsTableProps) {
  const { openMigrationForm } = useMigrationFormActions()
  const navigate = useNavigate()

  const [selectedRows, setSelectedRows] = useState<GridRowSelectionModel>([])
  const [isBulkCutoverLoading, setIsBulkCutoverLoading] = useState(false)
  const [bulkCutoverDialogOpen, setBulkCutoverDialogOpen] = useState(false)
  const [bulkCutoverError, setBulkCutoverError] = useState<string | null>(null)
  const [isBulkRetryLoading, setIsBulkRetryLoading] = useState(false)
  const [bulkRetryDialogOpen, setBulkRetryDialogOpen] = useState(false)
  const [bulkRetryError, setBulkRetryError] = useState<string | null>(null)
  const statusFilter = statusFilterProp ?? 'All'

  const handleSelectionChange = useCallback((newSelection: GridRowSelectionModel) => {
    setSelectedRows(newSelection)
  }, [])

  const filteredMigrations = useMemo(() => {
    if (!migrations) return []

    const category = STATUS_FILTER_TO_CATEGORY[statusFilter]
    if (!category) return migrations

    return migrations.filter((m) => getMigrationStatusCategory(m.status?.phase) === category)
  }, [migrations, statusFilter])

  const destinationByPlanQuery = useMigrationPlanDestinationsQuery(filteredMigrations)

  const destinationByPlan = destinationByPlanQuery.data || {}

  const duplicateVmNames = useMemo(() => {
    const counts = new Map<string, number>()
    for (const m of migrations) {
      const name = m.spec?.vmName || ''
      if (name) counts.set(name, (counts.get(name) || 0) + 1)
    }
    return new Set(
      Array.from(counts.entries())
        .filter(([, c]) => c > 1)
        .map(([n]) => n)
    )
  }, [migrations])

  const columns: GridColDef[] = useMemo(() => {
    return [
      {
        field: 'name',
        headerName: 'Name',
        flex: 1.2,
        valueGetter: (_, row) => row.spec?.vmName,
        renderCell: (params) => {
          const vmName = params.row?.spec?.vmName || '-'
          const migrationType = params.row?.spec?.migrationType
          const phase = params.row?.status?.phase
          const isHotMigration = migrationType?.toLowerCase() === 'hot'
          const isColdMigration = migrationType?.toLowerCase() === 'cold'
          const isMockMigration = migrationType?.toLowerCase() === 'mock'

          const namespace = params.row.metadata?.namespace
          const planName =
            params.row.spec?.migrationPlan || params.row.metadata?.labels?.migrationplan
          const key = namespace && planName ? `${namespace}::${planName}` : ''
          const destination = key ? destinationByPlan[key] : null

          const destinationTenant = destination?.destinationTenant || 'N/A'
          const destinationCluster = destination?.destinationCluster || 'N/A'
          const osFamily = destination?.vmOsByName?.[vmName]

          const tooltipTitle = (
            <TooltipContent
              title="Destination"
              lines={[`Tenant: ${destinationTenant}`, `Cluster: ${destinationCluster}`]}
            />
          )

          // Logic for the blinking pulse
          const activePhases = new Set([
            Phase.Pending,
            Phase.Validating,
            Phase.AwaitingDataCopyStart,
            Phase.CopyingBlocks,
            Phase.CopyingChangedBlocks,
            Phase.SnapshottingSourceVM,
            Phase.AttachingDisksToProxy,
            Phase.IdentifyingBlockDevices,
            Phase.HotAddTransferInProgress,
            Phase.HotAddCleanup,
            Phase.ConvertingDisk,
            Phase.AwaitingCutOverStartTime,
            Phase.AwaitingAdminCutOver,
            Phase.Unknown
          ])
          const isInProgress = activePhases.has(phase)
          const syncedPulse = `${pulse} 2s ease-in-out -20s infinite`

          const isDuplicate = duplicateVmNames.has(vmName)
          const vmKey =
            (params.row?.metadata?.annotations?.[
              'vjailbreak.k8s.pf9.io/original-vm-name'
            ] as string) ||
            (params.row?.metadata?.labels?.['vjailbreak.k8s.pf9.io/vm-key'] as string) ||
            ''
          const displayVmName = isDuplicate && vmKey ? vmKey : vmName
          // Migration type is already conveyed by the colored dot before the name —
          // no need to repeat it as text here.
          const subtitle = osFamily || ''

          return (
            <Box
              sx={{
                display: 'flex',
                alignItems: 'flex-start',
                gap: 0.5,
                py: 0.5,
                width: '100%',
                minWidth: 0
              }}
            >
              {isHotMigration && (
                <Tooltip title="Hot Migration">
                  <FiberManualRecordIcon
                    sx={{
                      fontSize: 12,
                      color: '#FFAE42',
                      mt: '4px',
                      ...(isInProgress && { animation: syncedPulse })
                    }}
                  />
                </Tooltip>
              )}
              {isColdMigration && (
                <Tooltip title="Cold Migration">
                  <FiberManualRecordIcon
                    sx={{
                      fontSize: 12,
                      color: '#4293FF',
                      mt: '4px',
                      ...(isInProgress && { animation: syncedPulse })
                    }}
                  />
                </Tooltip>
              )}
              {isMockMigration && (
                <Tooltip title="Migration without poweroff">
                  <FiberManualRecordIcon
                    sx={{
                      fontSize: 12,
                      color: '#9e1111ff',
                      mt: '4px',
                      ...(isInProgress && { animation: syncedPulse })
                    }}
                  />
                </Tooltip>
              )}
              <Box sx={{ overflow: 'hidden', minWidth: 0, flex: 1 }}>
                <Tooltip title={tooltipTitle} arrow>
                  <Typography
                    variant="body2"
                    sx={{
                      color: 'primary.main',
                      cursor: 'pointer',
                      overflow: 'hidden',
                      textOverflow: 'ellipsis',
                      whiteSpace: 'nowrap',
                      '&:hover': { textDecoration: 'underline' }
                    }}
                  >
                    {displayVmName}
                  </Typography>
                </Tooltip>
                {subtitle && (
                  <Typography
                    variant="caption"
                    color="text.secondary"
                    sx={{
                      display: 'block',
                      overflow: 'hidden',
                      textOverflow: 'ellipsis',
                      whiteSpace: 'nowrap'
                    }}
                  >
                    {subtitle}
                  </Typography>
                )}
              </Box>
            </Box>
          )
        }
      },
      {
        field: 'sourceDestination',
        headerName: 'Source → Destination',
        flex: 1.4,
        sortable: false,
        valueGetter: (_, row) => row.spec?.vmName,
        renderCell: (params) => {
          const namespace = params.row.metadata?.namespace
          const planName =
            params.row.spec?.migrationPlan || params.row.metadata?.labels?.migrationplan
          const key = namespace && planName ? `${namespace}::${planName}` : ''
          const destination = key ? destinationByPlan[key] : null

          const sourceVmwareRef = destination?.sourceVmwareRef || 'N/A'
          const destinationOpenstackRef = destination?.destinationOpenstackRef || 'N/A'
          const rawSourceDatacenter = destination?.sourceDatacenter || 'N/A'
          const sourceDatacenter = rawSourceDatacenter === 'N/A' ? 'No cluster' : rawSourceDatacenter
          const destinationTenant = destination?.destinationTenant || 'N/A'

          return (
            <Box sx={{ py: 0.5, overflow: 'hidden', width: '100%', minWidth: 0 }}>
              <Typography
                variant="body2"
                noWrap
                title={`${sourceVmwareRef} → ${destinationOpenstackRef}`}
              >
                {sourceVmwareRef} <Box component="span" sx={{ color: 'text.secondary' }}>{'→'}</Box> {destinationOpenstackRef}
              </Typography>
              <Typography
                variant="caption"
                color="text.secondary"
                noWrap
                title={`${sourceDatacenter} → ${destinationTenant}`}
              >
                {sourceDatacenter} {'→'} {destinationTenant}
              </Typography>
            </Box>
          )
        }
      },
      {
        field: 'status',
        headerName: 'Status',
        valueGetter: (_, row) => row?.status?.phase || 'Pending',
        flex: 0.8,
        sortComparator: (v1, v2) => {
          const order1 = STATUS_ORDER[v1] ?? Number.MAX_SAFE_INTEGER
          const order2 = STATUS_ORDER[v2] ?? Number.MAX_SAFE_INTEGER
          return order1 - order2
        },
        renderCell: (params) => <MigrationStatusChip phase={params.row?.status?.phase} />
      },
      {
        field: 'agent',
        headerName: 'Agent',
        valueGetter: (_, row) => row.status?.agentName,
        flex: 0.8
      },
      {
        field: 'timeElapsed',
        headerName: 'Time Elapsed',
        valueGetter: (_, row) => calculateTimeElapsed(row.metadata?.creationTimestamp, row.status),
        flex: 0.5,
        renderCell: (params) => {
          const createdAt = formatDateTime(params.row.metadata?.creationTimestamp)
          const tooltip = createdAt === '-' ? 'Created at: N/A' : `Created at: ${createdAt}`
          return (
            <Tooltip title={tooltip} arrow>
              <Typography
                variant="body2"
                sx={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}
              >
                {String(params.value ?? '-')}
              </Typography>
            </Tooltip>
          )
        }
      },
      {
        field: 'createdAt',
        headerName: 'Created At',
        valueGetter: (_, row) => formatDateTime(row.metadata?.creationTimestamp),
        flex: 1
      },
      {
        field: 'status.conditions',
        headerName: 'Progress',
        valueGetter: (_, row) =>
          getProgressText(
            row.status?.phase,
            row.status?.conditions,
            row.status?.currentDisk,
            row.status?.totalDisks
          ),
        flex: 2,
        renderCell: (params) => {
          const phase = params.row?.status?.phase
          const conditions = params.row?.status?.conditions
          const currentDisk = params.row?.status?.currentDisk
          const totalDisks = params.row?.status?.totalDisks
          const syncWarningMessage = params.row?.status?.syncWarningMessage
          const migrationName = params.row?.metadata?.name

          const isCopyPhase = [
            Phase.CopyingBlocks,
            Phase.CopyingChangedBlocks,
            Phase.ConvertingDisk,
            Phase.AwaitingDataCopyStart
          ].includes(phase)
          const isValidating = phase === Phase.Validating

          const diskNum = currentDisk != null ? parseInt(currentDisk, 10) : null
          const diskProgress =
            diskNum !== null && totalDisks ? Math.round((diskNum / totalDisks) * 100) : null

          const progressVariant: 'indeterminate' | 'determinate' | undefined =
            isCopyPhase && diskProgress !== null
              ? 'determinate'
              : isCopyPhase || isValidating
                ? 'indeterminate'
                : undefined

          const progressValue = diskProgress ?? 0
          const barColor =
            phase === Phase.Failed || phase === Phase.ValidationFailed
              ? 'error'
              : syncWarningMessage
                ? 'warning'
                : 'primary'

          return conditions ? (
            <Box
              sx={{ cursor: 'pointer', width: '100%', py: 0.25 }}
              onClick={() => {
                if (migrationName) navigate(`/dashboard/migrations/${migrationName}`)
              }}
            >
              <MigrationProgress
                phase={phase}
                progressText={getProgressText(phase, conditions, currentDisk, totalDisks)}
                syncWarningMessage={syncWarningMessage}
              />
              {(progressVariant || phase === Phase.Failed || phase === Phase.ValidationFailed) && (
                <LinearProgress
                  variant={progressVariant ?? 'determinate'}
                  value={phase === Phase.Failed || phase === Phase.ValidationFailed ? 100 : progressValue}
                  color={barColor}
                  sx={{ mt: 0.5, borderRadius: 1, height: 3 }}
                />
              )}
            </Box>
          ) : null
        }
      },
      {
        field: 'actions',
        headerName: 'Actions',
        flex: 0.5,
        minWidth: 130,
        renderCell: (params) => {
          const phase = params.row?.status?.phase
          const initiateCutover = params.row?.spec?.initiateCutover
          const migrationName = params.row?.metadata?.name
          const namespace = params.row?.metadata?.namespace
          const retryable = params.row?.status?.retryable
          const showRetryButton = phase === Phase.Failed
          const isRetryDisabled = retryable === false

          // Opens the migration form in retry mode, pre-populated with the failed
          // migration's configuration. The form triggers the actual retry.
          const handleRetry = () => {
            if (!migrationName || !namespace) return
            openMigrationForm('standard', {
              migrationName,
              namespace,
              planName: params.row?.spec?.migrationPlan || params.row?.metadata?.labels?.migrationplan || '',
              vmName: params.row?.spec?.vmName || ''
            })
          }

          const showAdminCutover = initiateCutover && phase === Phase.AwaitingAdminCutOver

          return (
            <Box
              className="migration-row-actions"
              sx={{
                display: 'flex',
                gap: 0.5,
                alignItems: 'center',
                height: '100%'
              }}
              onClick={(e) => e.stopPropagation()}
            >
              <Tooltip title="View details">
                <IconButton
                  onClick={(e) => {
                    e.stopPropagation()
                    if (migrationName) navigate(`/dashboard/migrations/${migrationName}`)
                  }}
                  size="small"
                >
                  <VisibilityIcon fontSize="small" />
                </IconButton>
              </Tooltip>
              <Tooltip title={'Delete migration'}>
                <IconButton
                  onClick={(e) => {
                    e.stopPropagation()
                    params.row.onDelete(params.row.metadata?.name)
                  }}
                  size="small"
                >
                  <DeleteIcon />
                </IconButton>
              </Tooltip>
              {showAdminCutover && (
                <TriggerAdminCutoverButton
                  migrationName={migrationName}
                  onSuccess={() => params.row.refetchMigrations?.()}
                />
              )}
              {showRetryButton && (
                <Tooltip
                  title={
                    isRetryDisabled
                      ? 'This migration cannot be retried because the VM has RDM disks. To retry, manually restart the migration.'
                      : 'Retry migration'
                  }
                >
                  <span>
                    <IconButton
                      onClick={(e) => {
                        e.stopPropagation()
                        if (!isRetryDisabled) {
                          handleRetry()
                        }
                      }}
                      size="small"
                      disabled={isRetryDisabled}
                      sx={{
                        cursor: isRetryDisabled ? 'not-allowed' : 'pointer',
                        position: 'relative',
                        '&.Mui-disabled': {
                          opacity: 0.4
                        }
                      }}
                    >
                      <ReplayIcon />
                    </IconButton>
                  </span>
                </Tooltip>
              )}
            </Box>
          )
        }
      }
    ]
  }, [destinationByPlan, pulse, duplicateVmNames, openMigrationForm])

  const selectedMigrations = useMemo(
    () => migrations?.filter((m) => selectedRows.includes(m.metadata?.name)) || [],
    [migrations, selectedRows]
  )
  const eligibleForCutover = useMemo(
    () =>
      selectedMigrations.filter(
        (migration) => migration.status?.phase === Phase.AwaitingAdminCutOver
      ),
    [selectedMigrations]
  )

  const eligibleForRetry = useMemo(
    () =>
      selectedMigrations.filter(
        (m) => m.status?.phase === Phase.Failed && (m.status as { retryable?: boolean })?.retryable !== false
      ),
    [selectedMigrations]
  )
  const allSelectedRetryable =
    selectedMigrations.length > 0 && eligibleForRetry.length === selectedMigrations.length

  const handleDeleteSelected = useCallback(() => {
    if (onDeleteSelected) {
      onDeleteSelected(selectedMigrations)
    }
  }, [onDeleteSelected, selectedMigrations])

  const handleBulkAdminCutover = useCallback(async () => {
    if (eligibleForCutover.length === 0) return

    setBulkCutoverError(null)
    setIsBulkCutoverLoading(true)

    try {
      await Promise.all(
        eligibleForCutover.map(async (migration) => {
          const result = await triggerAdminCutover(
            'migration-system',
            migration.metadata?.name || ''
          )
          if (!result.success) {
            throw new Error(result.message)
          }
          return result
        })
      )

      await refetchMigrations()

      setSelectedRows([])
    } catch (error) {
      console.error('Failed to trigger bulk admin cutover:', error)
      const errorMessage =
        error instanceof Error ? error.message : 'Failed to trigger bulk admin cutover'
      setBulkCutoverError(errorMessage)
      throw error
    } finally {
      setIsBulkCutoverLoading(false)
    }
  }, [eligibleForCutover, refetchMigrations])

  const handleCloseBulkCutoverDialog = useCallback(() => {
    if (!isBulkCutoverLoading) {
      setBulkCutoverDialogOpen(false)
      setBulkCutoverError(null)
    }
  }, [isBulkCutoverLoading])

  const handleBulkRetry = useCallback(async () => {
    if (eligibleForRetry.length === 0) return
    setBulkRetryError(null)
    setIsBulkRetryLoading(true)
    try {
      await Promise.allSettled(
        eligibleForRetry.map((m) =>
          deleteMigration(
            m.metadata?.name || '',
            (m.metadata?.namespace as string | undefined) || 'migration-system'
          )
        )
      )
      await refetchMigrations()
      setSelectedRows([])
      setBulkRetryDialogOpen(false)
    } catch (err) {
      setBulkRetryError(err instanceof Error ? err.message : 'Failed to retry migrations')
    } finally {
      setIsBulkRetryLoading(false)
    }
  }, [eligibleForRetry, refetchMigrations])

  const hasSelectionActions = onDeleteSelected !== undefined && onDeleteMigration !== undefined

  const migrationsWithActions = useMemo(
    () =>
      filteredMigrations?.map((migration) => ({
        ...migration,
        onDelete: onDeleteMigration,
        refetchMigrations
      })) || [],
    [filteredMigrations, onDeleteMigration, refetchMigrations]
  )

  return (
    <>
      <CommonDataGrid
        data-testid="migrations-table"
        rows={migrationsWithActions}
        columns={
          onDeleteSelected === undefined && onDeleteMigration === undefined
            ? columns.filter((column) => column.field !== 'actions')
            : columns
        }
        initialState={{
          pagination: { paginationModel: { page: 0, pageSize: 25 } },
          sorting: {
            sortModel: [{ field: 'status', sort: 'asc' }]
          },
          columns: {
            columnVisibilityModel: {
              createdAt: false
            }
          }
        }}
        pageSizeOptions={[10, 25, 50, 100]}
        checkboxSelection={hasSelectionActions}
        disableRowSelectionOnClick
        onRowSelectionModelChange={handleSelectionChange}
        rowSelectionModel={selectedRows}
        slots={{
          // Pass CustomToolbar directly (stable module-level reference) to prevent DataGrid
          // from unmounting/remounting the toolbar on every MigrationsTable re-render.
          // Dynamic data flows through slotProps.toolbar instead of an inline wrapper.
          toolbar: hasSelectionActions ? CustomToolbar : undefined
        }}
        slotProps={{
          toolbar: hasSelectionActions
            ? ({
                numSelected: selectedRows.length,
                onDeleteSelected: handleDeleteSelected,
                onBulkAdminCutover: () => setBulkCutoverDialogOpen(true),
                numEligibleForCutover: eligibleForCutover.length,
                onBulkRetry: () => setBulkRetryDialogOpen(true),
                numEligibleForRetry: allSelectedRetryable ? eligibleForRetry.length : 0,
                isBulkRetryLoading,
                filteredCount: filteredMigrations.length,
                totalCount: migrations?.length || 0
                // eslint-disable-next-line @typescript-eslint/no-explicit-any
              } as any)
            : {}
        }}
        getRowId={(row) => row.metadata?.name}
        loading={loading}
        emptyMessage="No migrations available"
        onRowClick={(params) => {
          const migName = params.row?.metadata?.name
          if (migName) navigate(`/dashboard/migrations/${migName}`)
        }}
        sx={{ '& .MuiDataGrid-row': { cursor: 'pointer' } }}
      />

      <Dialog
        open={bulkRetryDialogOpen}
        onClose={() => { if (!isBulkRetryLoading) { setBulkRetryDialogOpen(false); setBulkRetryError(null) } }}
        maxWidth="xs"
        fullWidth
      >
        <DialogTitle sx={{ px: 3, pt: 3, pb: 1, display: 'flex', alignItems: 'center', gap: 1 }}>
          <ReplayIcon color="primary" fontSize="small" />
          {`Retry ${eligibleForRetry.length} migration${eligibleForRetry.length !== 1 ? 's' : ''}?`}
        </DialogTitle>
        <DialogContent sx={{ px: 3, pb: 2 }}>
          <DialogContentText>
            {`This will retry ${eligibleForRetry.length} migration object${eligibleForRetry.length !== 1 ? 's' : ''} without changing their configurations. Source VMs will not be modified.`}
          </DialogContentText>
          {bulkRetryError && (
            <Alert severity="error" sx={{ mt: 2 }} onClose={() => setBulkRetryError(null)}>
              {bulkRetryError}
            </Alert>
          )}
        </DialogContent>
        <DialogActions sx={{ px: 3, pb: 3, gap: 1 }}>
          <Button onClick={() => { setBulkRetryDialogOpen(false); setBulkRetryError(null) }} disabled={isBulkRetryLoading}>
            Cancel
          </Button>
          <Button
            variant="outlined"
            color="primary"
            onClick={handleBulkRetry}
            disabled={isBulkRetryLoading}
            data-testid="confirm-bulk-retry-button"
          >
            {isBulkRetryLoading ? 'Retrying…' : 'Retry'}
          </Button>
        </DialogActions>
      </Dialog>

      <ConfirmationDialog
        open={bulkCutoverDialogOpen}
        onClose={handleCloseBulkCutoverDialog}
        title="Confirm Admin Cutover"
        icon={<PlayArrowIcon color="primary" />}
        message={
          eligibleForCutover.length > 1
            ? `Are you sure you want to trigger admin cutover for these ${eligibleForCutover.length} migrations?\n\n${eligibleForCutover
                .map((m) => `• ${m.metadata?.name}`)
                .join('\n')}\n\nThis will start the cutover process and cannot be undone.`
            : `Are you sure you want to trigger admin cutover for migration "${
                eligibleForCutover[0]?.metadata?.name
              }"?\n\nThis will start the cutover process and cannot be undone.`
        }
        items={eligibleForCutover.map((migration) => ({
          id: migration.metadata?.name || '',
          name: migration.metadata?.name || ''
        }))}
        actionLabel="Trigger Cutover"
        actionColor="primary"
        actionVariant="contained"
        onConfirm={handleBulkAdminCutover}
        errorMessage={bulkCutoverError}
        onErrorChange={setBulkCutoverError}
      />
    </>
  )
}
