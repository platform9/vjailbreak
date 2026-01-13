import { GridColDef, GridRowSelectionModel, GridToolbarContainer } from '@mui/x-data-grid'
import { Button, Typography, Box, IconButton, Tooltip } from '@mui/material'
import DeleteIcon from '@mui/icons-material/DeleteOutlined'
import MigrationIcon from '@mui/icons-material/SwapHoriz'
import ReplayIcon from '@mui/icons-material/Replay'
import { useState, useMemo } from 'react'
import { CustomSearchToolbar } from 'src/components/grid'
import { CommonDataGrid } from 'src/components/grid'
import ListAltIcon from '@mui/icons-material/ListAlt'
import { LogsDrawer } from '.'
import { Condition, Migration, Phase } from '../api/migrations'
import MigrationDetailModal from 'src/components/migrations/MigrationDetailModal'
import MigrationProgress from '../components/MigrationProgress'
import { QueryObserverResult } from '@tanstack/react-query'
import { useQuery } from '@tanstack/react-query'
import { RefetchOptions } from '@tanstack/react-query'
import { calculateTimeElapsed } from 'src/utils'
import { TriggerAdminCutoverButton } from '.'
import PlayArrowIcon from '@mui/icons-material/PlayArrow'
import { triggerAdminCutover, deleteMigration } from '../api/migrations'
import { ConfirmationDialog } from 'src/components/dialogs'
import { getMigrationPlan } from 'src/api/migration-plans/migrationPlans'
import { getMigrationTemplate } from 'src/api/migration-templates/migrationTemplates'
import { getOpenstackCredentials } from 'src/api/openstack-creds/openstackCreds'

const STATUS_ORDER = {
  Running: 0,
  Failed: 1,
  Succeeded: 2,
  Pending: 3
}
const PHASE_STEPS = {
  [Phase.Pending]: 1,
  [Phase.Validating]: 2,
  [Phase.AwaitingDataCopyStart]: 3,
  [Phase.CopyingBlocks]: 4,
  [Phase.CopyingChangedBlocks]: 5,
  [Phase.ConvertingDisk]: 6,
  [Phase.AwaitingCutOverStartTime]: 7,
  [Phase.AwaitingAdminCutOver]: 8,
  [Phase.Succeeded]: 9,
  [Phase.Failed]: 10,
  [Phase.ValidationFailed]: 11
}

const IN_PROGRESS_PHASES = [
  Phase.Pending,
  Phase.Validating,
  Phase.AwaitingDataCopyStart,
  Phase.CopyingBlocks,
  Phase.CopyingChangedBlocks,
  Phase.ConvertingDisk,
  Phase.AwaitingCutOverStartTime,
  Phase.AwaitingAdminCutOver
]

const getProgressText = (phase: Phase | undefined, conditions: Condition[] | undefined) => {
  if (!phase || phase === Phase.Unknown) {
    return 'Unknown Status'
  }

  const stepNumber = PHASE_STEPS[phase] || 0
  const totalSteps = 9

  const latestCondition = conditions?.sort(
    (a, b) => new Date(b.lastTransitionTime).getTime() - new Date(a.lastTransitionTime).getTime()
  )[0]

  const message = latestCondition?.message || phase

  if (phase === Phase.Failed || phase === Phase.ValidationFailed || phase === Phase.Succeeded) {
    return `${phase} - ${message}`
  }

  return `STEP ${stepNumber}/${totalSteps}: ${phase} - ${message}`
}

const formatCreatedAt = (value: unknown) => {
  if (!value) return '-'
  const d = value instanceof Date ? value : new Date(String(value))
  if (Number.isNaN(d.getTime())) return '-'
  return d.toLocaleString('en-GB', {
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false
  })
}

interface CustomToolbarProps {
  numSelected: number
  onDeleteSelected: () => void
  onBulkAdminCutover: () => void
  numEligibleForCutover: number
  refetchMigrations: (options?: RefetchOptions) => Promise<QueryObserverResult<Migration[], Error>>
  onStatusFilterChange: (filter: string) => void
  currentStatusFilter: string
  onDateFilterChange: (filter: string) => void
  currentDateFilter: string
}

const CustomToolbar = ({
  numSelected,
  onDeleteSelected,
  onBulkAdminCutover,
  numEligibleForCutover,
  refetchMigrations,
  onStatusFilterChange,
  currentStatusFilter,
  onDateFilterChange,
  currentDateFilter
}: CustomToolbarProps) => {
  return (
    <GridToolbarContainer
      sx={{
        p: 2,
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'center'
      }}
    >
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
        <MigrationIcon />
        <Typography variant="h6" component="h2">
          Migrations
        </Typography>
      </Box>
      <Box sx={{ display: 'flex', gap: 2, alignItems: 'center' }}>
        {numSelected > 0 ? (
          <>
            <Button
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
          </>
        ) : null}
        <CustomSearchToolbar
          placeholder="Search by Name, Status, or Progress"
          onRefresh={refetchMigrations}
          onStatusFilterChange={numSelected === 0 ? onStatusFilterChange : undefined}
          currentStatusFilter={currentStatusFilter}
          onDateFilterChange={numSelected === 0 ? onDateFilterChange : undefined}
          currentDateFilter={currentDateFilter}
        />
      </Box>
    </GridToolbarContainer>
  )
}

interface MigrationsTableProps {
  migrations: Migration[]
  onDeleteMigration?: (name: string) => void
  onDeleteSelected?: (migrations: Migration[]) => void
  refetchMigrations: (options?: RefetchOptions) => Promise<QueryObserverResult<Migration[], Error>>
}

export default function MigrationsTable({
  migrations,
  onDeleteMigration,
  onDeleteSelected,
  refetchMigrations
}: MigrationsTableProps) {
  const [selectedRows, setSelectedRows] = useState<GridRowSelectionModel>([])
  const [isBulkCutoverLoading, setIsBulkCutoverLoading] = useState(false)
  const [bulkCutoverDialogOpen, setBulkCutoverDialogOpen] = useState(false)
  const [bulkCutoverError, setBulkCutoverError] = useState<string | null>(null)
  const [migrationDetailModalOpen, setMigrationDetailModalOpen] = useState(false)
  const [selectedMigrationDetail, setSelectedMigrationDetail] = useState<Migration | null>(null)
  const [statusFilter, setStatusFilter] = useState('All')
  const [dateFilter, setDateFilter] = useState('All Time')
  const [logsDrawerOpen, setLogsDrawerOpen] = useState(false)
  const [selectedPod, setSelectedPod] = useState<{
    name: string
    namespace: string
    migrationName?: string
  } | null>(null)

  const handleSelectionChange = (newSelection: GridRowSelectionModel) => {
    setSelectedRows(newSelection)
  }

  const filteredMigrations = useMemo(() => {
    if (!migrations) return []

    const now = new Date()
    let timeCutoff = 0

    switch (dateFilter) {
      case 'Last 24 hours':
        timeCutoff = now.getTime() - 24 * 60 * 60 * 1000
        break
      case 'Last 7 days':
        timeCutoff = now.getTime() - 7 * 24 * 60 * 60 * 1000
        break
      case 'Last 30 days':
        timeCutoff = now.getTime() - 30 * 24 * 60 * 60 * 1000
        break
      default:
        timeCutoff = 0
    }

    const dateFiltered = migrations.filter((m) => {
      if (!m.metadata?.creationTimestamp) return false
      return new Date(m.metadata.creationTimestamp).getTime() >= timeCutoff
    })

    switch (statusFilter) {
      case 'Succeeded':
        return dateFiltered.filter((m) => m.status?.phase === Phase.Succeeded)
      case 'Failed':
        return dateFiltered.filter((m) => m.status?.phase === Phase.Failed)
      case 'In Progress':
        return dateFiltered.filter(
          (m) => m.status?.phase && IN_PROGRESS_PHASES.includes(m.status.phase)
        )
      case 'All':
      default:
        return dateFiltered
    }
  }, [migrations, statusFilter, dateFilter])

  const destinationByPlanQuery = useQuery({
    queryKey: [
      'migration-plan-destinations',
      filteredMigrations.map((m) => m.metadata?.namespace).join(','),
      filteredMigrations
        .map((m) => (m.spec as any)?.migrationPlan || (m.metadata as any)?.labels?.migrationplan)
        .filter(Boolean)
        .sort()
        .join(',')
    ],
    enabled: filteredMigrations.length > 0,
    refetchOnWindowFocus: false,
    staleTime: 60_000,
    queryFn: async () => {
      const safeGet = async <T,>(fn: () => Promise<T>): Promise<T | null> => {
        try {
          return await fn()
        } catch {
          return null
        }
      }

      const planKeys = Array.from(
        new Set(
          filteredMigrations
            .map((m) => {
              const namespace = m.metadata?.namespace
              const planName = (m.spec as any)?.migrationPlan || (m.metadata as any)?.labels?.migrationplan
              if (!namespace || !planName) return ''
              return `${namespace}::${planName}`
            })
            .filter(Boolean)
        )
      )

      const results = await Promise.all(
        planKeys.map(async (key) => {
          const [namespace, planName] = key.split('::')
          const plan = await safeGet(() => getMigrationPlan(planName, namespace))
          const templateName = (plan?.spec as any)?.migrationTemplate as string | undefined
          const template = templateName ? await safeGet(() => getMigrationTemplate(templateName, namespace)) : null
          const templateSpec = (template?.spec as any) || {}
          const openstackRef = templateSpec?.destination?.openstackRef as string | undefined
          const creds = openstackRef ? await safeGet(() => getOpenstackCredentials(openstackRef, namespace)) : null

          const destinationCluster = (templateSpec?.targetPCDClusterName as string) || 'N/A'
          const destinationTenant = (creds?.spec as any)?.projectName || 'N/A'
          return [key, { destinationCluster, destinationTenant }] as const
        })
      )

      return Object.fromEntries(results)
    }
  })

  const destinationByPlan = destinationByPlanQuery.data || {}

  const columns: GridColDef[] = useMemo(() => {
    return [
      {
        field: 'name',
        headerName: 'Name',
        valueGetter: (_, row) => row.spec?.vmName,
        flex: 0.7,
        renderCell: (params) => {
          const name = (params.value as string) || params.row.spec?.vmName || '-'
          const namespace = params.row.metadata?.namespace
          const planName = (params.row.spec as any)?.migrationPlan || (params.row.metadata as any)?.labels?.migrationplan
          const key = namespace && planName ? `${namespace}::${planName}` : ''
          const destination = key ? destinationByPlan[key] : null

          const tooltipTitle = (
            <Box sx={{ display: 'grid', gap: 0.5 }}>
              <Typography variant="caption" sx={{ fontWeight: 600 }}>
                Destination
              </Typography>
              <Typography variant="caption" color="text.secondary">
                Tenant: {destination?.destinationTenant || 'N/A'}
              </Typography>
              <Typography variant="caption" color="text.secondary">
                Cluster: {destination?.destinationCluster || 'N/A'}
              </Typography>
            </Box>
          )

          return (
            <Tooltip title={tooltipTitle} arrow>
              <Box
                sx={{
                  width: '100%',
                  height: '100%',
                  display: 'flex',
                  alignItems: 'center',
                  pl: 1,
                  cursor: 'pointer',
                  position: 'relative',
                  '&:hover': {
                    '& .click-indicator': { opacity: 1 },
                    '& .vm-name-text': { color: 'primary.main' }
                  }
                }}
                onClick={(e) => {
                  e.stopPropagation()
                  params.row.setSelectedMigrationDetail?.(params.row)
                  params.row.setMigrationDetailModalOpen?.(true)
                }}
              >
                <Box
                  className="click-indicator"
                  sx={{
                    position: 'absolute',
                    left: 0,
                    width: '3px',
                    height: '60%',
                    bgcolor: 'primary.main',
                    borderRadius: '0 4px 4px 0',
                    opacity: 0,
                    transition: 'opacity 0.2s'
                  }}
                />
                <Typography variant="body2" className="vm-name-text" sx={{ transition: 'color 0.2s' }}>
                  {name}
                </Typography>
              </Box>
            </Tooltip>
          )
        }
      },
      {
        field: 'status',
        headerName: 'Status',
        valueGetter: (_, row) => row?.status?.phase || 'Pending',
        flex: 0.5,
        sortComparator: (v1, v2) => {
          const order1 = STATUS_ORDER[v1] ?? Number.MAX_SAFE_INTEGER
          const order2 = STATUS_ORDER[v2] ?? Number.MAX_SAFE_INTEGER
          return order1 - order2
        }
      },
      {
        field: 'agent',
        headerName: 'Agent',
        valueGetter: (_, row) => row.status?.agentName,
        flex: 1
      },
      {
        field: 'timeElapsed',
        headerName: 'Time Elapsed',
        valueGetter: (_, row) => calculateTimeElapsed(row.metadata?.creationTimestamp, row.status),
        flex: 0.8,
        renderCell: (params) => {
          const createdAt = formatCreatedAt(params.row.metadata?.creationTimestamp)
          const tooltip = createdAt === '-' ? 'Created at: N/A' : `Created at: ${createdAt}`
          return (
            <Tooltip title={tooltip} arrow>
              <Typography
                variant="body2"
                sx={{
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                  whiteSpace: 'nowrap',
                  maxWidth: '100%'
                }}
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
        valueGetter: (_, row) => formatCreatedAt(row.metadata?.creationTimestamp),
        flex: 1
      },
      {
        field: 'status.conditions',
        headerName: 'Progress',
        valueGetter: (_, row) => getProgressText(row.status?.phase, row.status?.conditions),
        flex: 2,
        renderCell: (params) => {
          const phase = params.row?.status?.phase
          const conditions = params.row?.status?.conditions
          return conditions ? (
            <MigrationProgress phase={phase} progressText={getProgressText(phase, conditions)} />
          ) : null
        }
      },
      {
        field: 'actions',
        headerName: 'Actions',
        flex: 1,
        renderCell: (params) => {
          const phase = params.row?.status?.phase
          const initiateCutover = params.row?.spec?.initiateCutover
          const migrationName = params.row?.metadata?.name
          const namespace = params.row?.metadata?.namespace
          const showRetryButton = phase === Phase.Failed

          const handleRetry = async () => {
            if (!migrationName || !namespace) {
              console.error('Cannot retry: migration name or namespace is missing.')
              return
            }
            try {
              await deleteMigration(migrationName, namespace)
              params.row.refetchMigrations?.()
            } catch (error) {
              console.error(`Failed to delete migration '${migrationName}' for retry:`, error)
            }
          }

          const showAdminCutover = initiateCutover && phase === Phase.AwaitingAdminCutOver

          return (
            <Box sx={{ display: 'flex', gap: 1, alignItems: 'center' }}>
              {params.row.spec?.podRef && (
                <Tooltip title="View pod logs">
                  <IconButton
                    onClick={(e) => {
                      e.stopPropagation()
                      params.row.setSelectedPod({
                        name: params.row.spec.podRef,
                        namespace: params.row.metadata?.namespace || '',
                        migrationName: params.row.metadata?.name || ''
                      })
                      params.row.setLogsDrawerOpen(true)
                    }}
                    size="small"
                    sx={{
                      cursor: 'pointer',
                      position: 'relative'
                    }}
                  >
                    <ListAltIcon />
                  </IconButton>
                </Tooltip>
              )}

              {showAdminCutover && (
                <TriggerAdminCutoverButton
                  migrationName={migrationName}
                  onSuccess={() => {
                    params.row.refetchMigrations?.()
                  }}
                  onError={(error) => {
                    console.error('Failed to trigger cutover:', error)
                  }}
                />
              )}

              {showRetryButton && (
                <Tooltip title="Retry migration">
                  <IconButton
                    onClick={(e) => {
                      e.stopPropagation()
                      handleRetry()
                    }}
                    size="small"
                    sx={{
                      cursor: 'pointer',
                      position: 'relative'
                    }}
                  >
                    <ReplayIcon />
                  </IconButton>
                </Tooltip>
              )}

              <Tooltip title={'Delete migration'}>
                <IconButton
                  onClick={(e) => {
                    e.stopPropagation()
                    params.row.onDelete(params.row.metadata?.name)
                  }}
                  size="small"
                  sx={{
                    cursor: 'pointer',
                    position: 'relative'
                  }}
                >
                  <DeleteIcon />
                </IconButton>
              </Tooltip>
            </Box>
          )
        }
      }
    ]
  }, [destinationByPlan])

  const selectedMigrations =
    migrations?.filter((m) => selectedRows.includes(m.metadata?.name)) || []
  const eligibleForCutover = selectedMigrations.filter(
    (migration) => migration.status?.phase === Phase.AwaitingAdminCutOver
  )

  const handleBulkAdminCutover = async () => {
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
  }

  const handleCloseBulkCutoverDialog = () => {
    if (!isBulkCutoverLoading) {
      setBulkCutoverDialogOpen(false)
      setBulkCutoverError(null)
    }
  }

  const migrationsWithActions =
    filteredMigrations?.map((migration) => ({
      ...migration,
      onDelete: onDeleteMigration,
      refetchMigrations,
      setSelectedPod,
      setLogsDrawerOpen,
      setMigrationDetailModalOpen,
      setSelectedMigrationDetail
    })) || []

  return (
    <>
      <CommonDataGrid
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
        pageSizeOptions={[25, 50, 100]}
        localeText={{ noRowsLabel: 'No Migrations Available' }}
        emptyMessage="No migrations available"
        getRowId={(row) => row.metadata?.name}
        checkboxSelection={onDeleteSelected !== undefined && onDeleteMigration !== undefined}
        onRowSelectionModelChange={handleSelectionChange}
        rowSelectionModel={selectedRows}
        disableRowSelectionOnClick
        loading={isBulkCutoverLoading}
        slots={{
          toolbar:
            onDeleteSelected !== undefined && onDeleteMigration !== undefined
              ? () => (
                  <CustomToolbar
                    numSelected={selectedRows.length}
                    numEligibleForCutover={eligibleForCutover.length}
                    onDeleteSelected={() => {
                      const selectedMigrations = migrations?.filter((m) =>
                        selectedRows.includes(m.metadata?.name)
                      )
                      if (onDeleteSelected) {
                        onDeleteSelected(selectedMigrations || [])
                      }
                    }}
                    onBulkAdminCutover={() => setBulkCutoverDialogOpen(true)}
                    refetchMigrations={refetchMigrations}
                    onStatusFilterChange={setStatusFilter}
                    currentStatusFilter={statusFilter}
                    onDateFilterChange={setDateFilter}
                    currentDateFilter={dateFilter}
                  />
                )
              : undefined
        }}
      />

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

      <LogsDrawer
        open={logsDrawerOpen}
        onClose={() => setLogsDrawerOpen(false)}
        podName={selectedPod?.name || ''}
        namespace={selectedPod?.namespace || ''}
        migrationName={selectedPod?.migrationName || ''}
      />

      <MigrationDetailModal
        open={migrationDetailModalOpen}
        migration={selectedMigrationDetail}
        onClose={() => setMigrationDetailModalOpen(false)}
      />
    </>
  )
}
