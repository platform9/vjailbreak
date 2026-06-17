import { useCallback, useMemo, useState } from 'react'
import { Box, Button, Chip, CircularProgress, IconButton, Tooltip } from '@mui/material'
import AddIcon from '@mui/icons-material/Add'
import DeleteIcon from '@mui/icons-material/DeleteOutlined'
import RefreshIcon from '@mui/icons-material/Refresh'
import WarningIcon from '@mui/icons-material/Warning'
import DnsIcon from '@mui/icons-material/Dns'
import { GridColDef, GridRowSelectionModel } from '@mui/x-data-grid'
import { useQueryClient } from '@tanstack/react-query'
import { CommonDataGrid, CustomSearchToolbar, ListingToolbar } from 'src/components/grid'
import { ClickableTableCell } from 'src/components'
import { ConfirmationDialog } from 'src/components/dialogs'
import { deleteProxyVM, retryProxyVMVerification } from 'src/api/proxyvms/proxyVMs'
import { deleteSecret } from 'src/api/secrets/secrets'
import { ProxyVM, ProxyVMValidationStatus } from 'src/api/proxyvms/model'
import { useProxyVMsQuery, PROXY_VMS_QUERY_KEY } from 'src/hooks/api/useProxyVMsQuery'
import { VJAILBREAK_DEFAULT_NAMESPACE } from 'src/api/constants'
import AddProxyVMDrawer from './AddProxyVMDrawer'
import ProxyVMDetailDrawer from './ProxyVMDetailDrawer'

function statusColor(
  status: ProxyVMValidationStatus | undefined
): 'default' | 'warning' | 'success' | 'error' {
  switch (status) {
    case 'Ready':
      return 'success'
    case 'Verifying':
      return 'warning'
    case 'VerificationFailed':
      return 'error'
    default:
      return 'default'
  }
}

function formatAge(creationTimestamp: string): string {
  if (!creationTimestamp) return '-'
  try {
    const diffMs = Date.now() - new Date(creationTimestamp).getTime()
    const diffMin = Math.floor(diffMs / 60000)
    if (diffMin < 60) return `${diffMin}m`
    const diffHr = Math.floor(diffMin / 60)
    if (diffHr < 24) return `${diffHr}h`
    return `${Math.floor(diffHr / 24)}d`
  } catch {
    return '-'
  }
}

const getColumns = (
  onDeleteClick: (vm: ProxyVM) => void,
  onRetryClick: (vm: ProxyVM) => void,
  retryingNames: Set<string>,
  onDetailClick: (vm: ProxyVM) => void
): GridColDef[] => [
  {
    field: 'name',
    headerName: 'Name',
    flex: 1.2,
    minWidth: 120,
    renderCell: (params) => (
      <ClickableTableCell
        tooltipTitle="View details"
        onClick={() => onDetailClick(params.row.rawObject)}
      >
        {params.value}
      </ClickableTableCell>
    )
  },
  { field: 'vmName', headerName: 'VM Name', flex: 1.2, minWidth: 120 },
  {
    field: 'status',
    headerName: 'Status',
    flex: 0.8,
    minWidth: 100,
    renderCell: (params) => (
      <Chip
        label={params.value ?? 'Pending'}
        size="small"
        color={statusColor(params.value)}
        variant="outlined"
        sx={{ borderRadius: '4px' }}
      />
    )
  },
  {
    field: 'ipAddress',
    headerName: 'IP Address',
    flex: 0.9,
    minWidth: 110,
    renderCell: (params) => params.value || '-'
  },
  {
    field: 'attachedDiskCount',
    headerName: 'Attached Disks',
    flex: 0.7,
    minWidth: 100,
    renderCell: (params) => (params.value != null ? params.value : '-')
  },
  {
    field: 'age',
    headerName: 'Age',
    flex: 0.5,
    minWidth: 60
  },
  {
    field: 'lastValidated',
    headerName: 'Last Validated',
    flex: 1.1,
    minWidth: 150,
    renderCell: (params) => {
      if (!params.value) return '-'
      try {
        return new Date(params.value).toLocaleString()
      } catch {
        return params.value
      }
    }
  },
  {
    field: 'actions',
    headerName: 'Actions',
    width: 100,
    sortable: false,
    disableColumnMenu: true,
    renderCell: (params) => {
      const vm: ProxyVM = params.row.rawObject
      const isFailed = vm.status?.validationStatus === 'VerificationFailed'
      const isRetrying = retryingNames.has(vm.metadata.name)
      return (
        <Box sx={{ display: 'flex', gap: 0.5, alignItems: 'center' }}>
          {isFailed && (
            <Tooltip title="Retry verification">
              <span>
                <IconButton
                  size="small"
                  onClick={(e) => {
                    e.stopPropagation()
                    onRetryClick(vm)
                  }}
                  disabled={isRetrying}
                  aria-label="retry verification"
                >
                  {isRetrying ? (
                    <CircularProgress size={16} />
                  ) : (
                    <RefreshIcon fontSize="small" />
                  )}
                </IconButton>
              </span>
            </Tooltip>
          )}
          <Tooltip title="Delete">
            <IconButton
              size="small"
              onClick={(e) => {
                e.stopPropagation()
                onDeleteClick(vm)
              }}
              aria-label="delete proxy vm"
            >
              <DeleteIcon fontSize="small" />
            </IconButton>
          </Tooltip>
        </Box>
      )
    }
  }
]

const STATUS_FILTER_OPTIONS = ['All', 'Pending', 'Verifying', 'Ready', 'VerificationFailed']

export default function ProxyVMsTable() {
  const queryClient = useQueryClient()
  const { data: proxyVMs = [], isLoading, refetch } = useProxyVMsQuery()
  const [addDrawerOpen, setAddDrawerOpen] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<ProxyVM | null>(null)
  const [deleteError, setDeleteError] = useState<string | null>(null)
  const [deleting, setDeleting] = useState(false)
  const [statusFilter, setStatusFilter] = useState('All')
  const [rowSelectionModel, setRowSelectionModel] = useState<GridRowSelectionModel>([])
  const [bulkDeleteDialogOpen, setBulkDeleteDialogOpen] = useState(false)
  const [retryingNames, setRetryingNames] = useState<Set<string>>(new Set())
  const [detailVM, setDetailVM] = useState<ProxyVM | null>(null)
  const [detailOpen, setDetailOpen] = useState(false)

  const handleRefresh = useCallback(() => {
    refetch()
  }, [refetch])

  const handleRetry = useCallback(
    async (vm: ProxyVM) => {
      const name = vm.metadata.name
      setRetryingNames((prev) => new Set(prev).add(name))
      try {
        await retryProxyVMVerification(name)
        setTimeout(() => {
          queryClient.invalidateQueries({ queryKey: PROXY_VMS_QUERY_KEY })
        }, 1000)
      } finally {
        setRetryingNames((prev) => {
          const next = new Set(prev)
          next.delete(name)
          return next
        })
      }
    },
    [queryClient]
  )

  const handleBulkDeleteClick = () => {
    setBulkDeleteDialogOpen(true)
  }

  const handleBulkDeleteClose = () => {
    setBulkDeleteDialogOpen(false)
    setDeleteError(null)
  }

  const handleConfirmBulkDelete = async () => {
    const snapshot = [...selectedItems]
    setDeleting(true)
    setDeleteError(null)
    try {
      await Promise.all(
        snapshot.map(async (vm) => {
          try {
            await deleteProxyVM(vm.metadata.name)
          } catch (err: any) {
            if (err?.response?.status !== 404) throw err
          }
          if (vm.spec.sshKeySecretRef?.name) {
            deleteSecret(vm.spec.sshKeySecretRef.name, VJAILBREAK_DEFAULT_NAMESPACE).catch(() => {})
          }
        })
      )
      queryClient.invalidateQueries({ queryKey: PROXY_VMS_QUERY_KEY })
      refetch()
      setRowSelectionModel([])
      setBulkDeleteDialogOpen(false)
    } catch (err: any) {
      setDeleteError(err?.response?.data?.message || err?.message || 'Failed to delete Proxy VMs.')
    } finally {
      setDeleting(false)
    }
  }

  const handleDelete = async () => {
    if (!deleteTarget) return
    setDeleting(true)
    setDeleteError(null)
    try {
      await deleteProxyVM(deleteTarget.metadata.name)
    } catch (err: any) {
      const status = err?.response?.status
      if (status !== 404) {
        setDeleteError(err?.response?.data?.message || err?.message || 'Failed to delete Proxy VM.')
        setDeleting(false)
        return
      }
      // 404 = already gone, proceed with cleanup
    }
    if (deleteTarget.spec.sshKeySecretRef?.name) {
      deleteSecret(deleteTarget.spec.sshKeySecretRef.name, VJAILBREAK_DEFAULT_NAMESPACE).catch(() => {})
    }
    queryClient.invalidateQueries({ queryKey: PROXY_VMS_QUERY_KEY })
    refetch()
    setDeleteTarget(null)
    setDeleting(false)
  }

  const rows = useMemo(
    () =>
      proxyVMs
        .filter((vm) => !vm.metadata.deletionTimestamp)
        .map((vm) => ({
          id: vm.metadata.name,
          name: vm.metadata.name,
          vmName: vm.spec.vmName,
          status: vm.status?.validationStatus,
          message: vm.status?.validationMessage,
          ipAddress: vm.status?.ipAddress,
          attachedDiskCount: vm.status?.attachedDiskCount,
          age: formatAge(vm.metadata?.creationTimestamp || ''),
          lastValidated: vm.status?.lastValidationTime,
          rawObject: vm
        })),
    [proxyVMs]
  )

  const filteredRows = useMemo(() => {
    if (statusFilter === 'All') return rows
    return rows.filter((r) => r.status === statusFilter)
  }, [rows, statusFilter])

  const selectedItems = useMemo(
    () => proxyVMs.filter((vm) => rowSelectionModel.includes(vm.metadata.name)),
    [proxyVMs, rowSelectionModel]
  )

  const handleDetailClick = useCallback((vm: ProxyVM) => {
    setDetailVM(vm)
    setDetailOpen(true)
  }, [])

  const columns = useMemo(
    () => getColumns(setDeleteTarget, handleRetry, retryingNames, handleDetailClick),
    [handleRetry, retryingNames, handleDetailClick]
  )

  const search = (
    <Box sx={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 2 }}>
      {rowSelectionModel.length > 0 && (
        <Button
          variant="outlined"
          color="error"
          startIcon={<DeleteIcon />}
          onClick={handleBulkDeleteClick}
          sx={{ height: 40 }}
        >
          Delete Selected ({rowSelectionModel.length})
        </Button>
      )}
      <CustomSearchToolbar
        placeholder="Search by name or VM name"
        onRefresh={handleRefresh}
        disableRefresh={isLoading}
        onStatusFilterChange={setStatusFilter}
        currentStatusFilter={statusFilter}
        statusFilterOptions={STATUS_FILTER_OPTIONS}
      />
    </Box>
  )

  const actions = (
    <Button
      variant="contained"
      startIcon={<AddIcon />}
      onClick={() => setAddDrawerOpen(true)}
      sx={{ height: 40 }}
    >
      Add Proxy VM
    </Button>
  )

  const toolbar = (
    <ListingToolbar title="Proxy VMs" icon={<DnsIcon />} search={search} actions={actions} />
  )

  return (
    <div style={{ height: '100%', width: '100%', overflow: 'hidden' }}>
      <CommonDataGrid
        rows={filteredRows}
        columns={columns}
        loading={isLoading || deleting}
        checkboxSelection
        disableRowSelectionOnClick
        rowSelectionModel={rowSelectionModel}
        onRowSelectionModelChange={setRowSelectionModel}
        initialState={{
          sorting: { sortModel: [{ field: 'name', sort: 'asc' }] },
          pagination: { paginationModel: { pageSize: 25 } }
        }}
        slots={{ toolbar: () => toolbar }}
        pageSizeOptions={[10, 25, 50, 100]}
        emptyMessage="No Proxy VMs configured"
        sx={{ '& .MuiDataGrid-cell:focus': { outline: 'none' } }}
      />

      {deleteTarget && (
        <ConfirmationDialog
          open
          title="Delete Proxy VM"
          icon={<WarningIcon color="warning" />}
          message={`Are you sure you want to delete Proxy VM "${deleteTarget.metadata.name}"?`}
          actionLabel="Delete"
          actionColor="error"
          actionVariant="outlined"
          onConfirm={handleDelete}
          onClose={() => {
            setDeleteTarget(null)
            setDeleteError(null)
          }}
          errorMessage={deleteError}
          onErrorChange={setDeleteError}
        />
      )}

      {bulkDeleteDialogOpen && (
        <ConfirmationDialog
          open
          title="Delete Proxy VMs"
          icon={<WarningIcon color="warning" />}
          message={`Are you sure you want to delete ${rowSelectionModel.length} Proxy VM${rowSelectionModel.length > 1 ? 's' : ''}?`}
          items={selectedItems.map((vm) => ({ id: vm.metadata.name, name: vm.metadata.name }))}
          actionLabel="Delete"
          actionColor="error"
          actionVariant="outlined"
          onConfirm={handleConfirmBulkDelete}
          onClose={handleBulkDeleteClose}
          errorMessage={deleteError}
          onErrorChange={setDeleteError}
        />
      )}

      {addDrawerOpen && <AddProxyVMDrawer open onClose={() => setAddDrawerOpen(false)} />}

      <ProxyVMDetailDrawer
        open={detailOpen}
        proxyVM={detailVM}
        onClose={() => setDetailOpen(false)}
      />
    </div>
  )
}
