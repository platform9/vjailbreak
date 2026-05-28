import { useMemo, useState } from 'react'
import { Box, Chip, IconButton, Tooltip, Typography } from '@mui/material'
import DeleteIcon from '@mui/icons-material/Delete'
import { GridColDef } from '@mui/x-data-grid'
import { useQueryClient } from '@tanstack/react-query'
import { CommonDataGrid } from 'src/components/grid'
import ConfirmationDialog from 'src/components/dialogs/ConfirmationDialog'
import { deleteProxyVM } from 'src/api/proxyvms/proxyVMs'
import { ProxyVM, ProxyVMValidationStatus } from 'src/api/proxyvms/model'
import { PROXY_VMS_QUERY_KEY } from 'src/hooks/api/useProxyVMsQuery'
interface Props {
  proxyVMs: ProxyVM[]
  loading?: boolean
  toolbar: React.ReactNode
}

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

export default function ProxyVMsTable({ proxyVMs, loading, toolbar }: Props) {
  const queryClient = useQueryClient()
  const [deleteTarget, setDeleteTarget] = useState<ProxyVM | null>(null)
  const [deleteError, setDeleteError] = useState<string | null>(null)

  const columns: GridColDef[] = useMemo(
    () => [
      { field: 'name', headerName: 'Name', flex: 1.2, minWidth: 140 },
      { field: 'vmName', headerName: 'VM Name', flex: 1.2, minWidth: 140 },
      {
        field: 'status',
        headerName: 'Status',
        flex: 1,
        minWidth: 150,
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
        flex: 1,
        minWidth: 130,
        renderCell: (params) => params.value || '-'
      },
      {
        field: 'attachedDiskCount',
        headerName: 'Attached Disks',
        flex: 0.8,
        minWidth: 120,
        renderCell: (params) => (params.value != null ? params.value : '-')
      },
      {
        field: 'age',
        headerName: 'Age',
        flex: 0.7,
        minWidth: 80
      },
      {
        field: 'lastValidated',
        headerName: 'Last Validated',
        flex: 1.2,
        minWidth: 160,
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
        headerName: '',
        width: 60,
        sortable: false,
        disableColumnMenu: true,
        renderCell: (params) => (
          <Tooltip title="Delete">
            <IconButton
              size="small"
              color="error"
              onClick={() => setDeleteTarget(params.row.rawObject)}
            >
              <DeleteIcon fontSize="small" />
            </IconButton>
          </Tooltip>
        )
      }
    ],
    []
  )

  const rows = useMemo(
    () =>
      proxyVMs.map((vm) => ({
        id: vm.metadata.name,
        name: vm.metadata.name,
        vmName: vm.spec.vmName,
        status: vm.status?.validationStatus,
        ipAddress: vm.status?.ipAddress,
        attachedDiskCount: vm.status?.attachedDiskCount,
        age: formatAge(vm.metadata.creationTimestamp),
        lastValidated: vm.status?.lastValidationTime,
        rawObject: vm
      })),
    [proxyVMs]
  )

  const handleDelete = async () => {
    if (!deleteTarget) return
    setDeleteError(null)
    await deleteProxyVM(deleteTarget.metadata.name)
    queryClient.invalidateQueries({ queryKey: PROXY_VMS_QUERY_KEY })
    setDeleteTarget(null)
  }

  return (
    <Box sx={{ height: '100%', width: '100%' }}>
      <CommonDataGrid
        rows={rows}
        columns={columns}
        loading={loading}
        disableRowSelectionOnClick
        slots={{ toolbar: () => toolbar }}
        initialState={{
          pagination: { paginationModel: { pageSize: 25 } }
        }}
        pageSizeOptions={[10, 25, 50, 100]}
        emptyMessage="No Proxy VMs configured"
      />

      {deleteTarget && (
        <ConfirmationDialog
          open
          title="Delete Proxy VM"
          message={
            <Typography>
              Delete Proxy VM <strong>{deleteTarget.metadata.name}</strong>? This action cannot be
              undone.
            </Typography>
          }
          actionLabel="Delete"
          actionColor="error"
          onConfirm={handleDelete}
          onClose={() => {
            setDeleteTarget(null)
            setDeleteError(null)
          }}
          errorMessage={deleteError}
          onErrorChange={setDeleteError}
        />
      )}
    </Box>
  )
}
