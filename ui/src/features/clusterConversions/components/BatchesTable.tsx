import { useState } from 'react'
import {
  Box,
  Typography,
  Chip,
  Button,
  IconButton,
  LinearProgress,
  Tooltip
} from '@mui/material'
import { GridColDef, GridRowSelectionModel } from '@mui/x-data-grid'
import ClusterIcon from '@mui/icons-material/Hub'
import DeleteIcon from '@mui/icons-material/Delete'
import VisibilityIcon from '@mui/icons-material/Visibility'
import AddIcon from '@mui/icons-material/Add'
import WarningIcon from '@mui/icons-material/Warning'
import { QueryObserverResult, RefetchOptions } from '@tanstack/react-query'
import { useQueryClient } from '@tanstack/react-query'
import { ReactElement } from 'react'
import { ClusterConversionBatch, ClusterConversionBatchPhase } from 'src/api/cluster-conversion-batches/model'
import { deleteClusterConversionBatch } from 'src/api/cluster-conversion-batches/clusterConversionBatches'
import { CLUSTER_CONVERSION_BATCHES_QUERY_KEY } from 'src/hooks/api/useClusterConversionBatchesQuery'
import { CommonDataGrid, ListingToolbar, CustomSearchToolbar } from 'src/components/grid'
import { ConfirmationDialog } from 'src/components/dialogs'

// ---------------------------------------------------------------------------
// BatchStatusChip — inline status chip for ClusterConversionBatchPhase
// ---------------------------------------------------------------------------

function BatchStatusChip({ phase }: { phase: string }) {
  type ChipColor = 'default' | 'warning' | 'info' | 'success' | 'error'

  const COLOR_MAP: Record<string, ChipColor> = {
    Pending: 'default',
    Running: 'info',
    Succeeded: 'success',
    PartialFail: 'warning',
    Failed: 'error'
  }

  const icon: ReactElement | undefined = undefined
  const color: ChipColor = COLOR_MAP[phase] ?? 'default'

  return (
    <Chip
      size="small"
      label={phase}
      variant="outlined"
      color={color}
      icon={icon}
      sx={{ borderRadius: '4px', height: '24px' }}
    />
  )
}

// ---------------------------------------------------------------------------
// Toolbar
// ---------------------------------------------------------------------------

interface BatchesToolbarProps {
  selectedCount: number
  onDeleteSelected: () => void
  onCreateBatch: () => void
  refetchBatches: (options?: RefetchOptions) => Promise<QueryObserverResult<ClusterConversionBatch[], Error>>
}

function BatchesToolbar({
  selectedCount,
  onDeleteSelected,
  onCreateBatch,
  refetchBatches
}: BatchesToolbarProps) {
  const search = (
    <Box sx={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 2 }}>
      {selectedCount > 0 && (
        <Button
          variant="outlined"
          color="error"
          startIcon={<DeleteIcon />}
          onClick={onDeleteSelected}
          size="small"
        >
          Delete Selected
        </Button>
      )}
      <CustomSearchToolbar
        placeholder="Search by Cluster Name or Status"
        onRefresh={refetchBatches}
      />
    </Box>
  )

  const actions = (
    <Button
      variant="contained"
      color="primary"
      startIcon={<AddIcon />}
      onClick={onCreateBatch}
      sx={{ height: 40 }}
    >
      New Batch
    </Button>
  )

  return (
    <ListingToolbar
      title="Cluster Conversion Batches"
      icon={<ClusterIcon />}
      subtitle={
        selectedCount > 0 ? (
          <span>
            {selectedCount} {selectedCount === 1 ? 'row' : 'rows'} selected
          </span>
        ) : null
      }
      search={search}
      actions={actions}
    />
  )
}

// ---------------------------------------------------------------------------
// BatchesTableProps
// ---------------------------------------------------------------------------

export interface BatchesTableProps {
  batches: ClusterConversionBatch[]
  refetchBatches: (options?: RefetchOptions) => Promise<QueryObserverResult<ClusterConversionBatch[], Error>>
  onCreateBatch: () => void
  onViewDetails: (batch: ClusterConversionBatch) => void
}

// ---------------------------------------------------------------------------
// BatchesTable
// ---------------------------------------------------------------------------

export default function BatchesTable({
  batches,
  refetchBatches,
  onCreateBatch,
  onViewDetails
}: BatchesTableProps) {
  const queryClient = useQueryClient()
  const [selectedRows, setSelectedRows] = useState<GridRowSelectionModel>([])
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
  const [deleteError, setDeleteError] = useState<string | null>(null)

  const handleDeleteSelected = () => {
    setDeleteDialogOpen(true)
  }

  const handleDeleteClose = () => {
    setDeleteDialogOpen(false)
    setDeleteError(null)
  }

  const handleConfirmDelete = async () => {
    try {
      const toDelete = batches.filter((b) => selectedRows.includes(b.metadata?.name || ''))
      await Promise.all(toDelete.map((b) => deleteClusterConversionBatch(b.metadata?.name || '')))
      queryClient.invalidateQueries({ queryKey: CLUSTER_CONVERSION_BATCHES_QUERY_KEY })
      setSelectedRows([])
    } catch (error) {
      console.error('Failed to delete cluster conversion batches:', error)
      setDeleteError(
        error instanceof Error ? error.message : 'Failed to delete cluster conversion batches'
      )
      throw error
    }
  }

  const handleSelectionChange = (newSelection: GridRowSelectionModel) => {
    setSelectedRows(newSelection)
  }

  const columns: GridColDef[] = [
    {
      field: 'clusterName',
      headerName: 'Cluster Name',
      display: 'flex',
      flex: 1,
      renderCell: (params) => {
        const batch = params.row as ClusterConversionBatch
        return (
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
            <ClusterIcon fontSize="small" color="action" />
            <Typography variant="body2">{batch.spec?.vmwareClusterName || '—'}</Typography>
          </Box>
        )
      }
    },
    {
      field: 'status',
      headerName: 'Status',
      flex: 0.6,
      renderCell: (params) => {
        const batch = params.row as ClusterConversionBatch
        const phase: ClusterConversionBatchPhase | string = batch.status?.phase || 'Pending'
        return <BatchStatusChip phase={phase} />
      }
    },
    {
      field: 'autoStart',
      headerName: 'Auto Start',
      flex: 0.5,
      renderCell: (params) => {
        const batch = params.row as ClusterConversionBatch
        const mode = batch.spec?.autoStart
        return (
          <Chip
            size="small"
            label={mode}
            variant="outlined"
            color={mode === 'Auto' ? 'info' : 'default'}
            sx={{ borderRadius: '4px', height: '24px' }}
          />
        )
      }
    },
    {
      field: 'hosts',
      headerName: 'Hosts',
      flex: 0.6,
      renderCell: (params) => {
        const batch = params.row as ClusterConversionBatch
        const total = batch.status?.totalHosts ?? batch.spec?.hosts?.length ?? 0
        const succeeded = batch.status?.succeededHosts ?? 0
        return (
          <Box>
            <Typography variant="body2">
              {succeeded} succeeded / {total} total
            </Typography>
          </Box>
        )
      }
    },
    {
      field: 'progress',
      headerName: 'Progress',
      flex: 1,
      renderCell: (params) => {
        const batch = params.row as ClusterConversionBatch
        const total = batch.status?.totalHosts ?? batch.spec?.hosts?.length ?? 0
        const succeeded = batch.status?.succeededHosts ?? 0
        const needsAttention = batch.status?.needsAttentionHosts ?? 0
        const progressValue = total > 0 ? (succeeded / total) * 100 : 0

        return (
          <Box sx={{ width: '100%' }}>
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
              <Box sx={{ flex: 1 }}>
                <LinearProgress
                  variant="determinate"
                  value={progressValue}
                  sx={{
                    height: 6,
                    borderRadius: 3,
                    backgroundColor: 'rgba(0, 0, 0, 0.08)'
                  }}
                />
              </Box>
              <Typography variant="caption" sx={{ minWidth: 40, textAlign: 'right' }}>
                {succeeded}/{total}
              </Typography>
            </Box>
            {needsAttention > 0 && (
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5, mt: 0.5 }}>
                <Tooltip title="Hosts needing attention">
                  <Typography variant="caption" color="warning.main">
                    {needsAttention} needs attention
                  </Typography>
                </Tooltip>
              </Box>
            )}
          </Box>
        )
      }
    },
    {
      field: 'actions',
      headerName: 'Actions',
      flex: 0.7,
      sortable: false,
      renderCell: (params) => {
        const batch = params.row as ClusterConversionBatch
        return (
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
            <Button
              variant="text"
              size="small"
              startIcon={<VisibilityIcon />}
              onClick={() => onViewDetails(batch)}
            >
              Details
            </Button>
            <Tooltip title="Delete batch">
              <IconButton
                size="small"
                color="error"
                onClick={() => {
                  setSelectedRows([batch.metadata?.name || ''])
                  setDeleteDialogOpen(true)
                }}
              >
                <DeleteIcon fontSize="small" />
              </IconButton>
            </Tooltip>
          </Box>
        )
      }
    }
  ]

  return (
    <Box sx={{ width: '100%', height: '100%' }}>
      <CommonDataGrid
        rows={batches}
        columns={columns}
        getRowId={(row: ClusterConversionBatch) => row.metadata?.name || ''}
        initialState={{
          pagination: { paginationModel: { pageSize: 25 } },
          sorting: {
            sortModel: [{ field: 'status', sort: 'asc' }]
          }
        }}
        pageSizeOptions={[5, 10, 25]}
        emptyMessage="No cluster conversion batches available"
        checkboxSelection
        onRowSelectionModelChange={handleSelectionChange}
        rowSelectionModel={selectedRows}
        disableRowSelectionOnClick
        slots={{
          toolbar: () => (
            <BatchesToolbar
              selectedCount={selectedRows.length}
              onDeleteSelected={handleDeleteSelected}
              onCreateBatch={onCreateBatch}
              refetchBatches={refetchBatches}
            />
          )
        }}
      />

      <ConfirmationDialog
        open={deleteDialogOpen}
        onClose={handleDeleteClose}
        title="Confirm Delete"
        icon={<WarningIcon color="warning" />}
        message={
          selectedRows.length > 1
            ? 'Are you sure you want to delete these cluster conversion batches?'
            : 'Are you sure you want to delete the selected cluster conversion batch?'
        }
        items={batches
          .filter((b) => selectedRows.includes(b.metadata?.name || ''))
          .map((b) => ({
            id: b.metadata?.name || '',
            name: b.metadata?.name || ''
          }))}
        actionLabel="Delete"
        actionColor="error"
        actionVariant="outlined"
        onConfirm={handleConfirmDelete}
        errorMessage={deleteError}
        onErrorChange={setDeleteError}
      />
    </Box>
  )
}
