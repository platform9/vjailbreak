import { GridColDef, GridRowSelectionModel } from '@mui/x-data-grid'
import { Box, Button, IconButton, Tooltip } from '@mui/material'
import AddIcon from '@mui/icons-material/Add'
import VpnKeyIcon from '@mui/icons-material/VpnKey'
import ContentCopyIcon from '@mui/icons-material/ContentCopy'
import EditIcon from '@mui/icons-material/EditOutlined'
import DeleteIcon from '@mui/icons-material/DeleteOutlined'
import WarningIcon from '@mui/icons-material/Warning'
import { useCallback, useMemo, useState } from 'react'
import { useQuery, useMutation } from '@tanstack/react-query'
import { deleteSecret, listSecrets } from 'src/api/secrets/secrets'
import type { Secret } from 'src/api/secrets/model'
import { CommonDataGrid, CustomSearchToolbar, ListingToolbar } from 'src/components/grid'
import { useErrorHandler } from 'src/hooks/useErrorHandler'
import { ConfirmationDialog } from 'src/components/dialogs'
import AddEsxiSshKeyDrawer from '../components/AddEsxiSshKeyDrawer'

export const ESXI_SSH_KEYS_QUERY_KEY = ['secrets', 'migration-system', 'esxi-ssh-keys']

type EsxiSshKeyRow = {
  id: string
  name: string
  secret: Secret
}

const getColumns = (handlers: {
  onEdit: (row: EsxiSshKeyRow) => void
  onDelete: (row: EsxiSshKeyRow) => void
}): GridColDef[] => [
  {
    field: 'name',
    headerName: 'SSH Key Name',
    flex: 1,
    minWidth: 240
  },
  {
    field: 'actions',
    headerName: 'Actions',
    sortable: false,
    filterable: false,
    width: 180,
    renderCell: (params) => {
      const keyContent = params.row.secret.data?.['ssh-privatekey']
      const canCopy = typeof keyContent === 'string' && keyContent.trim() !== ''

      return (
        <Box sx={{ display: 'flex', gap: 0.5 }}>
          <Tooltip title={canCopy ? 'Copy key content' : 'No key content'}>
            <span>
              <IconButton
                size="small"
                aria-label="copy ssh key"
                disabled={!canCopy}
                onClick={async (e) => {
                  e.stopPropagation()
                  if (!canCopy) return
                  await navigator.clipboard.writeText(keyContent)
                }}
              >
                <ContentCopyIcon fontSize="small" />
              </IconButton>
            </span>
          </Tooltip>

          <Tooltip title="Edit">
            <IconButton
              size="small"
              aria-label="edit ssh key"
              onClick={(e) => {
                e.stopPropagation()
                handlers.onEdit(params.row)
              }}
            >
              <EditIcon fontSize="small" />
            </IconButton>
          </Tooltip>

          <Tooltip title="Delete">
            <IconButton
              size="small"
              aria-label="delete ssh key"
              onClick={(e) => {
                e.stopPropagation()
                handlers.onDelete(params.row)
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

export default function EsxiSshKeysPage() {
  const { reportError } = useErrorHandler({ component: 'EsxiSshKeysPage' })
  const [drawerState, setDrawerState] = useState<
    | { open: false }
    | { open: true; mode: 'add' | 'edit'; initialValues?: { name: string; sshPrivateKey: string } }
  >({ open: false })

  const [rowSelectionModel, setRowSelectionModel] = useState<GridRowSelectionModel>([])

  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
  const [selectedForDeletion, setSelectedForDeletion] = useState<EsxiSshKeyRow | null>(null)
  const [deleteError, setDeleteError] = useState<string | null>(null)

  const [bulkDeleteDialogOpen, setBulkDeleteDialogOpen] = useState(false)
  const [bulkDeleteError, setBulkDeleteError] = useState<string | null>(null)

  const {
    data: secrets,
    isLoading,
    refetch
  } = useQuery({
    queryKey: ESXI_SSH_KEYS_QUERY_KEY,
    queryFn: async () => {
      try {
        return await listSecrets('migration-system')
      } catch (error) {
        reportError(error as Error, { context: 'list-esxi-ssh-key-secrets' })
        throw error
      }
    },
    staleTime: 0,
    refetchOnMount: true
  })

  const rows: EsxiSshKeyRow[] = useMemo(() => {
    const items = Array.isArray(secrets) ? secrets : []
    return items
      .filter((secret) => {
        const data = secret.data
        return data && Object.prototype.hasOwnProperty.call(data, 'ssh-privatekey')
      })
      .map((secret) => ({
        id: secret.metadata.name,
        name: secret.metadata.name,
        secret
      }))
  }, [secrets])

  const handleRefresh = useCallback(() => {
    refetch()
  }, [refetch])

  const { mutateAsync: deleteKeys, isPending: deleting } = useMutation({
    mutationFn: async (names: string[]) => {
      await Promise.all(names.map((name) => deleteSecret(name, 'migration-system')))
    },
    onSuccess: () => {
      setRowSelectionModel([])
      refetch()
    },
    onError: (error: any) => {
      reportError(error as Error, { context: 'delete-esxi-ssh-keys' })
    }
  })

  const handleOpenAdd = () => setDrawerState({ open: true, mode: 'add' })

  const handleEdit = (row: EsxiSshKeyRow) => {
    setDrawerState({
      open: true,
      mode: 'edit',
      initialValues: {
        name: row.secret.metadata.name,
        sshPrivateKey: row.secret.data?.['ssh-privatekey'] || ''
      }
    })
  }

  const handleDelete = (row: EsxiSshKeyRow) => {
    setSelectedForDeletion(row)
    setDeleteError(null)
    setDeleteDialogOpen(true)
  }

  const handleDeleteClose = () => {
    setDeleteDialogOpen(false)
    setSelectedForDeletion(null)
    setDeleteError(null)
  }

  const handleConfirmDelete = async () => {
    if (!selectedForDeletion) return
    await deleteKeys([selectedForDeletion.secret.metadata.name])
  }

  const selectedRows = useMemo(() => {
    const selected = new Set(rowSelectionModel as string[])
    return rows.filter((r) => selected.has(r.id))
  }, [rowSelectionModel, rows])

  const handleBulkDelete = () => {
    if (selectedRows.length === 0) return
    setBulkDeleteError(null)
    setBulkDeleteDialogOpen(true)
  }

  const handleBulkDeleteClose = () => {
    setBulkDeleteDialogOpen(false)
    setBulkDeleteError(null)
  }

  const handleConfirmBulkDelete = async () => {
    if (selectedRows.length === 0) return
    try {
      await deleteKeys(selectedRows.map((r) => r.secret.metadata.name))
    } catch (e: any) {
      const message = e?.response?.data?.message || e?.message || 'Failed to delete SSH keys'
      setBulkDeleteError(message)
      throw e
    }
  }

  const search = (
    <Box sx={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 2 }}>
      {rowSelectionModel.length > 0 && (
        <Button
          variant="outlined"
          color="error"
          startIcon={<DeleteIcon />}
          onClick={handleBulkDelete}
          disabled={deleting}
          sx={{ height: 40 }}
        >
          Delete Selected ({rowSelectionModel.length})
        </Button>
      )}

      <CustomSearchToolbar
        placeholder="Search by SSH key name"
        onRefresh={handleRefresh}
        disableRefresh={isLoading}
      />
    </Box>
  )

  const actions = (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
      <Button
        variant="contained"
        color="primary"
        startIcon={<AddIcon />}
        onClick={handleOpenAdd}
        disabled={deleting}
        sx={{ height: 40 }}
      >
        Add SSH Key
      </Button>
    </Box>
  )

  const toolbar = (
    <ListingToolbar title="ESXi SSH Keys" icon={<VpnKeyIcon />} search={search} actions={actions} />
  )

  const columns = useMemo(
    () =>
      getColumns({
        onEdit: handleEdit,
        onDelete: handleDelete
      }),
    [handleEdit, handleDelete]
  )

  return (
    <div style={{ height: '100%', width: '100%', overflow: 'hidden' }}>
      <CommonDataGrid
        rows={rows}
        columns={columns}
        checkboxSelection
        disableRowSelectionOnClick
        rowSelectionModel={rowSelectionModel}
        onRowSelectionModelChange={setRowSelectionModel}
        initialState={{
          sorting: {
            sortModel: [{ field: 'name', sort: 'asc' }]
          },
          pagination: {
            paginationModel: {
              pageSize: 25
            }
          }
        }}
        slots={{
          toolbar: () => toolbar
        }}
        pageSizeOptions={[10, 25, 50, 100]}
        loading={isLoading || deleting}
        emptyMessage="No ESXi SSH keys configured"
      />

      {drawerState.open && (
        <AddEsxiSshKeyDrawer
          open
          requireCloseConfirmation={true}
          mode={drawerState.mode}
          initialValues={drawerState.initialValues}
          onClose={() => {
            setDrawerState({ open: false })
            refetch()
          }}
        />
      )}

      <ConfirmationDialog
        open={deleteDialogOpen}
        onClose={handleDeleteClose}
        title="Confirm Delete"
        icon={<WarningIcon color="warning" />}
        message={
          selectedForDeletion
            ? `Are you sure you want to delete SSH key "${selectedForDeletion.secret.metadata.name}"?`
            : 'Are you sure you want to delete this SSH key?'
        }
        actionLabel="Delete"
        actionColor="error"
        actionVariant="outlined"
        onConfirm={handleConfirmDelete}
        errorMessage={deleteError}
        onErrorChange={setDeleteError}
      />

      <ConfirmationDialog
        open={bulkDeleteDialogOpen}
        onClose={handleBulkDeleteClose}
        title="Confirm Delete"
        icon={<WarningIcon color="warning" />}
        message={
          selectedRows.length > 1
            ? 'Are you sure you want to delete these SSH keys?'
            : 'Are you sure you want to delete this SSH key?'
        }
        items={selectedRows.map((r) => ({ id: r.id, name: r.name }))}
        actionLabel="Delete"
        actionColor="error"
        actionVariant="outlined"
        onConfirm={handleConfirmBulkDelete}
        errorMessage={bulkDeleteError}
        onErrorChange={setBulkDeleteError}
      />
    </div>
  )
}
