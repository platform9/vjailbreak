import { useMemo } from 'react'
import { Box, Button, Chip, IconButton, Tooltip, Typography } from '@mui/material'
import { GridColDef } from '@mui/x-data-grid'
import DeleteIcon from '@mui/icons-material/DeleteOutlined'
import InventoryIcon from '@mui/icons-material/Inventory2'
import PlayArrowIcon from '@mui/icons-material/PlayArrow'
import ScheduleOutlinedIcon from '@mui/icons-material/ScheduleOutlined'
import dayjs from 'dayjs'
import { CommonDataGrid, CustomSearchToolbar, ListingToolbar } from 'src/components/grid'
import { ClickableTableCell, StatusChip } from 'src/components'
import { DEFAULT_BUCKET_LABEL } from '../constants'
import { bucketStatusLabel, bucketStatusTone, getBucketStatus } from '../utils/bucketStatus'
import type { BucketStatus, MigrationBucket } from '../types'

export interface BucketsTableProps {
  buckets: MigrationBucket[]
  statusByBucket?: Record<string, BucketStatus>
  loading?: boolean
  onRefresh?: () => void
  onOpenDetails: (bucket: MigrationBucket) => void
  onDelete: (bucket: MigrationBucket) => void
  onTrigger: () => void
  triggerDisabled?: boolean
  triggerDisabledReason?: string
}

// All optional so the component is assignable to the DataGrid toolbar slot type; real values
// are supplied via slotProps.toolbar.
interface ToolbarProps {
  onRefresh?: () => void
  onTrigger?: () => void
  triggerDisabled?: boolean
  triggerDisabledReason?: string
}

// Module-level toolbar (stable reference) — dynamic data flows via slotProps.toolbar.
const BucketsToolbar = ({
  onRefresh,
  onTrigger,
  triggerDisabled,
  triggerDisabledReason
}: ToolbarProps) => {
  const search = <CustomSearchToolbar placeholder="Search buckets" onRefresh={onRefresh} />
  const actions = (
    <Tooltip title={triggerDisabled ? triggerDisabledReason ?? '' : ''} arrow>
      <span>
        <Button
          variant="contained"
          color="primary"
          startIcon={<PlayArrowIcon />}
          onClick={() => onTrigger?.()}
          disabled={triggerDisabled}
          sx={{ height: 40 }}
          data-testid="trigger-migrations-button"
        >
          Trigger migrations
        </Button>
      </span>
    </Tooltip>
  )
  return (
    <ListingToolbar
      title="Inventory"
      subtitle="Discover VMs and organize them into migration buckets."
      icon={<InventoryIcon />}
      search={search}
      actions={actions}
    />
  )
}

/**
 * Buckets listing table — same shell as the Migrations page (CommonDataGrid + ListingToolbar).
 * Clicking a bucket name opens its details drawer (where Edit / Duplicate / Schedule live);
 * Delete is inline in the row (the default bucket is non-deletable).
 */
export default function BucketsTable({
  buckets,
  statusByBucket,
  loading = false,
  onRefresh,
  onOpenDetails,
  onDelete,
  onTrigger,
  triggerDisabled,
  triggerDisabledReason
}: BucketsTableProps) {
  // Default bucket first, then alphabetical.
  const rows = useMemo(
    () =>
      [...buckets].sort((a, b) => {
        if (a.spec.isDefault !== b.spec.isDefault) return a.spec.isDefault ? -1 : 1
        return a.metadata.name.localeCompare(b.metadata.name)
      }),
    [buckets]
  )

  const columns: GridColDef<MigrationBucket>[] = useMemo(
    () => [
      {
        field: 'name',
        headerName: 'Name',
        flex: 1.4,
        valueGetter: (_, row) =>
          row.spec.isDefault ? DEFAULT_BUCKET_LABEL : row.metadata.name,
        renderCell: (params) => {
          const isDefault = params.row.spec.isDefault
          const label = isDefault ? DEFAULT_BUCKET_LABEL : params.row.metadata.name
          return (
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, minWidth: 0 }}>
              <ClickableTableCell onClick={() => onOpenDetails(params.row)}>
                {label}
              </ClickableTableCell>
              {isDefault ? (
                <Chip label="Default" size="small" color="primary" variant="outlined" />
              ) : null}
            </Box>
          )
        }
      },
      {
        field: 'status',
        headerName: 'Status',
        flex: 0.8,
        valueGetter: (_, row) =>
          bucketStatusLabel(statusByBucket?.[row.metadata.name] ?? getBucketStatus(row)),
        renderCell: (params) => {
          const status = statusByBucket?.[params.row.metadata.name] ?? getBucketStatus(params.row)
          return (
            <StatusChip
              label={bucketStatusLabel(status)}
              tone={bucketStatusTone(status)}
              size="small"
              variant="filled"
            />
          )
        }
      },
      {
        field: 'vms',
        headerName: 'VMs',
        flex: 0.4,
        valueGetter: (_, row) => row.spec.vms.length
      },
      {
        field: 'schedule',
        headerName: 'Schedule',
        flex: 0.9,
        valueGetter: (_, row) => row.spec.schedule ?? '',
        renderCell: (params) => {
          const schedule = params.row.spec.schedule
          if (!schedule) {
            return (
              <Typography variant="body2" color="text.secondary">
                —
              </Typography>
            )
          }
          return (
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
              <ScheduleOutlinedIcon sx={{ fontSize: 16, color: 'text.secondary' }} />
              <Typography variant="body2">{dayjs(schedule).format('MMM D, HH:mm')}</Typography>
            </Box>
          )
        }
      },
      {
        field: 'actions',
        headerName: 'Actions',
        flex: 0.5,
        sortable: false,
        filterable: false,
        renderCell: (params) => {
          if (params.row.spec.isDefault) {
            return (
              <Tooltip title="The default bucket cannot be deleted">
                <span>
                  <IconButton size="small" disabled>
                    <DeleteIcon />
                  </IconButton>
                </span>
              </Tooltip>
            )
          }
          return (
            <Tooltip title="Delete bucket">
              <IconButton
                size="small"
                onClick={(e) => {
                  e.stopPropagation()
                  onDelete(params.row)
                }}
              >
                <DeleteIcon />
              </IconButton>
            </Tooltip>
          )
        }
      }
    ],
    [statusByBucket, onOpenDetails, onDelete]
  )

  return (
    <CommonDataGrid<MigrationBucket>
      data-testid="buckets-table"
      rows={rows}
      columns={columns}
      getRowId={(row) => row.metadata.name}
      loading={loading}
      sx={{ height: '100%' }}
      emptyMessage="No buckets yet. A default bucket is created automatically once eligible VMs are discovered."
      initialState={{
        pagination: { paginationModel: { page: 0, pageSize: 25 } }
      }}
      pageSizeOptions={[10, 25, 50, 100]}
      disableRowSelectionOnClick
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      slots={{ toolbar: BucketsToolbar as any }}
      slotProps={{
        toolbar: {
          onRefresh,
          onTrigger,
          triggerDisabled,
          triggerDisabledReason
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
        } as any
      }}
    />
  )
}
