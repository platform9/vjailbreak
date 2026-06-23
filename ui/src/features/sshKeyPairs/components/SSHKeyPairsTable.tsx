import { useCallback, useMemo, useState } from 'react'
import { Box, Button, Chip, IconButton, Tooltip } from '@mui/material'
import AddIcon from '@mui/icons-material/Add'
import DeleteIcon from '@mui/icons-material/DeleteOutlined'
import ContentCopyIcon from '@mui/icons-material/ContentCopy'
import WarningIcon from '@mui/icons-material/Warning'
import KeyIcon from '@mui/icons-material/Key'
import { GridColDef } from '@mui/x-data-grid'
import { useQueryClient } from '@tanstack/react-query'
import { CommonDataGrid, CustomSearchToolbar, ListingToolbar } from 'src/components/grid'
import { ConfirmationDialog } from 'src/components/dialogs'
import { deleteSSHKeyPair } from 'src/api/sshKeyPairs/sshKeyPairs'
import { SSHKeyPair } from 'src/api/sshKeyPairs/model'
import { useSSHKeyPairsQuery, SSH_KEY_PAIRS_QUERY_KEY } from 'src/hooks/api/useSSHKeyPairsQuery'
import AddSSHKeyPairDrawer from './AddSSHKeyPairDrawer'

function formatAge(createdAt: string): string {
  if (!createdAt) return '-'
  try {
    const diffMs = Date.now() - new Date(createdAt).getTime()
    const diffMin = Math.floor(diffMs / 60000)
    if (diffMin < 60) return `${diffMin}m`
    const diffHr = Math.floor(diffMin / 60)
    if (diffHr < 24) return `${diffHr}h`
    return `${Math.floor(diffHr / 24)}d`
  } catch {
    return '-'
  }
}

const getColumns = (onDeleteClick: (kp: SSHKeyPair) => void): GridColDef[] => [
  { field: 'name', headerName: 'Name', flex: 1.2, minWidth: 150 },
  {
    field: 'type',
    headerName: 'Type',
    flex: 0.6,
    minWidth: 100,
    renderCell: (params) => (
      <Chip
        label={params.value === 'generated' ? 'Generated' : 'Manual'}
        size="small"
        color={params.value === 'generated' ? 'primary' : 'default'}
        variant="outlined"
        sx={{ borderRadius: '4px' }}
      />
    )
  },
  {
    field: 'publicKey',
    headerName: 'Public Key',
    flex: 2,
    minWidth: 200,
    renderCell: (params) => {
      const val: string = params.value || ''
      const truncated = val.length > 60 ? `${val.slice(0, 57)}...` : val
      return (
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5, overflow: 'hidden' }}>
          <Tooltip title={val}>
            <span
              style={{
                overflow: 'hidden',
                textOverflow: 'ellipsis',
                whiteSpace: 'nowrap',
                fontFamily: 'monospace',
                fontSize: '0.75rem'
              }}
            >
              {truncated || '-'}
            </span>
          </Tooltip>
          {val && (
            <Tooltip title="Copy public key">
              <IconButton
                size="small"
                onClick={(e) => {
                  e.stopPropagation()
                  navigator.clipboard?.writeText(val)
                }}
              >
                <ContentCopyIcon sx={{ fontSize: 14 }} />
              </IconButton>
            </Tooltip>
          )}
        </Box>
      )
    }
  },
  {
    field: 'age',
    headerName: 'Age',
    flex: 0.5,
    minWidth: 60
  },
  {
    field: 'actions',
    headerName: 'Actions',
    width: 70,
    sortable: false,
    disableColumnMenu: true,
    renderCell: (params) => (
      <Tooltip title="Delete">
        <IconButton
          size="small"
          onClick={(e) => {
            e.stopPropagation()
            onDeleteClick(params.row.rawObject)
          }}
          aria-label="delete ssh key pair"
        >
          <DeleteIcon fontSize="small" />
        </IconButton>
      </Tooltip>
    )
  }
]

export default function SSHKeyPairsTable() {
  const queryClient = useQueryClient()
  const { data: keyPairs = [], isLoading, refetch } = useSSHKeyPairsQuery()
  const [addDrawerOpen, setAddDrawerOpen] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<SSHKeyPair | null>(null)
  const [deleteError, setDeleteError] = useState<string | null>(null)
  const [deleting, setDeleting] = useState(false)

  const handleRefresh = useCallback(() => {
    refetch()
  }, [refetch])

  const handleDelete = async () => {
    if (!deleteTarget) return
    setDeleting(true)
    setDeleteError(null)
    try {
      await deleteSSHKeyPair(deleteTarget.name)
      queryClient.invalidateQueries({ queryKey: SSH_KEY_PAIRS_QUERY_KEY })
      refetch()
      setDeleteTarget(null)
    } catch (err: any) {
      const status = err?.response?.status
      if (status === 404) {
        queryClient.invalidateQueries({ queryKey: SSH_KEY_PAIRS_QUERY_KEY })
        refetch()
        setDeleteTarget(null)
      } else {
        setDeleteError(
          err?.response?.data?.message || err?.message || 'Failed to delete SSH key pair.'
        )
      }
    } finally {
      setDeleting(false)
    }
  }

  const rows = useMemo(
    () =>
      keyPairs.map((kp) => ({
        id: kp.name,
        name: kp.name,
        type: kp.type,
        publicKey: kp.publicKey,
        age: formatAge(kp.createdAt),
        rawObject: kp
      })),
    [keyPairs]
  )

  const columns = useMemo(() => getColumns(setDeleteTarget), [])

  const search = (
    <Box sx={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 2 }}>
      <CustomSearchToolbar
        placeholder="Search by name"
        onRefresh={handleRefresh}
        disableRefresh={isLoading}
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
      Add SSH Key Pair
    </Button>
  )

  const toolbar = (
    <ListingToolbar title="SSH Key Pairs" icon={<KeyIcon />} search={search} actions={actions} />
  )

  return (
    <div style={{ height: '100%', width: '100%', overflow: 'hidden' }}>
      <CommonDataGrid
        rows={rows}
        columns={columns}
        loading={isLoading || deleting}
        disableRowSelectionOnClick
        initialState={{
          sorting: { sortModel: [{ field: 'name', sort: 'asc' }] },
          pagination: { paginationModel: { pageSize: 25 } }
        }}
        slots={{ toolbar: () => toolbar }}
        pageSizeOptions={[10, 25, 50, 100]}
        emptyMessage="No SSH key pairs configured"
        sx={{ '& .MuiDataGrid-cell:focus': { outline: 'none' } }}
      />

      {deleteTarget && (
        <ConfirmationDialog
          open
          title="Delete SSH Key Pair"
          icon={<WarningIcon color="warning" />}
          message={`Are you sure you want to delete SSH key pair "${deleteTarget.name}"?`}
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

      {addDrawerOpen && (
        <AddSSHKeyPairDrawer
          open
          onClose={() => {
            setAddDrawerOpen(false)
            refetch()
          }}
        />
      )}
    </div>
  )
}
