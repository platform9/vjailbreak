import { GridColDef, GridRowSelectionModel } from '@mui/x-data-grid'
import { Button, Box, IconButton, Tooltip, Chip } from '@mui/material'
import DeleteIcon from '@mui/icons-material/DeleteOutlined'
import EditIcon from '@mui/icons-material/Edit'
import WarningIcon from '@mui/icons-material/Warning'
import AddIcon from '@mui/icons-material/Add'
import SdStorageIcon from '@mui/icons-material/SdStorage'
import { CustomSearchToolbar, ListingToolbar } from 'src/components/grid'
import { CommonDataGrid } from 'src/components/grid'
import { useState, useCallback, useEffect, useMemo } from 'react'
import {
  useArrayCredentialsQuery,
  ARRAY_CREDS_QUERY_KEY
} from 'src/hooks/api/useArrayCredentialsQuery'
import { ArrayCreds, ARRAY_VENDOR_TYPES } from 'src/api/array-creds/model'
import { ConfirmationDialog } from 'src/components/dialogs'
import { useQueryClient } from '@tanstack/react-query'
import { deleteArrayCredsWithSecretFlow } from 'src/api/helpers'
import { useErrorHandler } from 'src/hooks/useErrorHandler'
import AddArrayCredentialsDrawer from './AddArrayCredentialsDrawer'
import EditArrayCredentialsDrawer from './EditArrayCredentialsDrawer'

interface ArrayCredentialRow {
  id: string
  name: string
  vendor: string
  vendorLabel: string
  volumeType: string
  backendName: string
  source: string
  credentialsStatus: string
  credObject: ArrayCreds
}

const getCredentialsStatusColor = (status: string): 'success' | 'warning' | 'default' => {
  if (!status) return 'default'
  const normalizedStatus = status.toLowerCase()
  if (normalizedStatus === 'configured' || normalizedStatus === 'succeeded') {
    return 'success'
  }
  if (normalizedStatus === 'pending' || normalizedStatus === 'validating') {
    return 'warning'
  }
  return 'default'
}

const getSourceColor = (source: string): 'info' | 'default' => {
  if (source === 'Auto-discovered') return 'info'
  return 'default'
}

const getVendorLabel = (vendorType: string): string => {
  const vendor = ARRAY_VENDOR_TYPES.find((v) => v.value === vendorType)
  return vendor?.label || vendorType || 'Unknown'
}

const getColumns = (
  onEditClick: (row: ArrayCredentialRow) => void,
  onDeleteClick: (row: ArrayCredentialRow) => void
): GridColDef[] => [
  {
    field: 'name',
    headerName: 'Name',
    flex: 1.5,
    minWidth: 200
  },
  {
    field: 'vendorLabel',
    headerName: 'Vendor',
    flex: 1,
    minWidth: 120,
    renderCell: (params) => (
      <Chip
        label={params.value || 'unsupported'}
        variant="outlined"
        size="small"
        sx={{ borderRadius: '4px' }}
      />
    )
  },
  {
    field: 'volumeType',
    headerName: 'Volume Type',
    flex: 1,
    minWidth: 120
  },
  {
    field: 'backendName',
    headerName: 'Backend Name',
    flex: 1,
    minWidth: 120
  },
  {
    field: 'source',
    headerName: 'Source',
    flex: 1,
    minWidth: 120,
    renderCell: (params) => (
      <Chip
        label={params.value}
        variant="outlined"
        color={getSourceColor(params.value)}
        size="small"
        sx={{ borderRadius: '4px' }}
      />
    )
  },
  {
    field: 'credentialsStatus',
    headerName: 'Credentials',
    flex: 1,
    minWidth: 120,
    renderCell: (params) => (
      <Chip
        label={params.value || 'Pending'}
        variant="outlined"
        color={getCredentialsStatusColor(params.value)}
        size="small"
        sx={{ borderRadius: '4px' }}
      />
    )
  },
  {
    field: 'actions',
    headerName: 'Actions',
    width: 100,
    sortable: false,
    renderCell: (params) => (
      <Box>
        <Tooltip title="Edit credentials">
          <IconButton
            onClick={(e) => {
              e.stopPropagation()
              onEditClick(params.row)
            }}
            aria-label="edit credential"
            size="small"
          >
            <EditIcon fontSize="small" />
          </IconButton>
        </Tooltip>
        <Tooltip title="Delete">
          <IconButton
            onClick={(e) => {
              e.stopPropagation()
              onDeleteClick(params.row)
            }}
            aria-label="delete credential"
            size="small"
          >
            <DeleteIcon fontSize="small" />
          </IconButton>
        </Tooltip>
      </Box>
    )
  }
]

interface CustomToolbarProps {
  onRefresh: () => void
  onAddCredential: () => void
  selectedCount: number
  onDeleteSelected: () => void
}

const CustomToolbar = ({
  onRefresh,
  onAddCredential,
  selectedCount,
  onDeleteSelected
}: CustomToolbarProps) => {
  const search = (
    <Box sx={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 2 }}>
      {selectedCount > 0 && (
        <Button
          variant="outlined"
          color="error"
          startIcon={<DeleteIcon />}
          onClick={onDeleteSelected}
          sx={{ height: 40 }}
        >
          Delete Selected ({selectedCount})
        </Button>
      )}
      <CustomSearchToolbar placeholder="Search by Name" onRefresh={onRefresh} />
    </Box>
  )

  const actions = (
    <Button
      variant="contained"
      color="primary"
      startIcon={<AddIcon />}
      onClick={onAddCredential}
      sx={{ height: 40 }}
    >
      Add Array Credentials
    </Button>
  )

  return (
    <ListingToolbar
      title="Storage Array Credentials"
      icon={<SdStorageIcon />}
      search={search}
      actions={actions}
    />
  )
}

export default function StorageArrayTable() {
  const { reportError } = useErrorHandler({ component: 'StorageArrayTable' })
  const queryClient = useQueryClient()

  const {
    data: arrayCredentials,
    isLoading,
    refetch
  } = useArrayCredentialsQuery(undefined, {
    staleTime: 0,
    refetchOnMount: true
  })

  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
  const [selectedForDeletion, setSelectedForDeletion] = useState<ArrayCredentialRow | null>(null)
  const [deleteError, setDeleteError] = useState<string | null>(null)
  const [deleting, setDeleting] = useState(false)
  const [addDrawerOpen, setAddDrawerOpen] = useState(false)
  const [editDrawerOpen, setEditDrawerOpen] = useState(false)
  const [selectedForEdit, setSelectedForEdit] = useState<ArrayCredentialRow | null>(null)
  const [rowSelectionModel, setRowSelectionModel] = useState<GridRowSelectionModel>([])
  const [bulkDeleteDialogOpen, setBulkDeleteDialogOpen] = useState(false)

  useEffect(() => {
    refetch()
  }, [refetch])

  const rows: ArrayCredentialRow[] =
    arrayCredentials?.map((cred: ArrayCreds) => {
      const hasSecret = !!cred.spec?.secretRef?.name
      const credentialsStatus = hasSecret ? 'Configured' : 'Pending'
      const source = cred.spec?.autoDiscovered ? 'Auto-discovered' : 'Manual'

      return {
        id: cred.metadata.name,
        name: cred.metadata.name,
        vendor: cred.spec?.vendorType || '',
        vendorLabel: getVendorLabel(cred.spec?.vendorType),
        volumeType: cred.spec?.openstackMapping?.volumeType || '',
        backendName: cred.spec?.openstackMapping?.cinderBackendName || '',
        source,
        credentialsStatus,
        credObject: cred
      }
    }) || []

  const handleRefresh = useCallback(() => {
    refetch()
  }, [refetch])

  const handleEditClick = (row: ArrayCredentialRow) => {
    setSelectedForEdit(row)
    setEditDrawerOpen(true)
  }

  const handleDeleteClick = (row: ArrayCredentialRow) => {
    setSelectedForDeletion(row)
    setDeleteDialogOpen(true)
  }

  const handleDeleteClose = () => {
    setDeleteDialogOpen(false)
    setSelectedForDeletion(null)
    setDeleteError(null)
  }

  const handleConfirmDelete = async () => {
    if (!selectedForDeletion) return

    setDeleting(true)
    try {
      await deleteArrayCredsWithSecretFlow(selectedForDeletion.name)
      queryClient.invalidateQueries({ queryKey: ARRAY_CREDS_QUERY_KEY })
      handleDeleteClose()
    } catch (error) {
      console.error('Error deleting array credential:', error)
      reportError(error as Error, {
        context: 'array-credentials-deletion',
        metadata: {
          credentialName: selectedForDeletion.name
        }
      })
      setDeleteError(error instanceof Error ? error.message : 'Unknown error occurred')
    } finally {
      setDeleting(false)
    }
  }

  const handleBulkDeleteClick = () => {
    setBulkDeleteDialogOpen(true)
  }

  const handleBulkDeleteClose = () => {
    setBulkDeleteDialogOpen(false)
    setDeleteError(null)
  }

  const handleConfirmBulkDelete = async () => {
    setDeleting(true)
    try {
      await Promise.all(rowSelectionModel.map((id) => deleteArrayCredsWithSecretFlow(id as string)))
      queryClient.invalidateQueries({ queryKey: ARRAY_CREDS_QUERY_KEY })
      setRowSelectionModel([])
      handleBulkDeleteClose()
    } catch (error) {
      console.error('Error deleting array credentials:', error)
      reportError(error as Error, {
        context: 'array-credentials-bulk-deletion'
      })
      setDeleteError(error instanceof Error ? error.message : 'Unknown error occurred')
    } finally {
      setDeleting(false)
    }
  }

  const selectedItems = useMemo(() => {
    return rows.filter((row) => rowSelectionModel.includes(row.id))
  }, [rows, rowSelectionModel])

  const handleOpenAddDrawer = () => {
    setAddDrawerOpen(true)
  }

  const handleCloseAddDrawer = () => {
    setAddDrawerOpen(false)
    refetch()
  }

  const handleCloseEditDrawer = () => {
    setEditDrawerOpen(false)
    setSelectedForEdit(null)
    refetch()
  }

  const getCustomErrorMessage = useCallback((error: Error | string) => {
    const baseMessage = 'Failed to delete array credential'
    if (error instanceof Error) {
      return `${baseMessage}: ${error.message}`
    }
    return `${baseMessage}: ${error}`
  }, [])

  const tableColumns = getColumns(handleEditClick, handleDeleteClick)

  return (
    <div style={{ height: '100%', width: '100%', overflow: 'hidden' }}>
      <CommonDataGrid
        rows={rows}
        columns={tableColumns}
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
          toolbar: () => (
            <CustomToolbar
              onRefresh={handleRefresh}
              onAddCredential={handleOpenAddDrawer}
              selectedCount={rowSelectionModel.length}
              onDeleteSelected={handleBulkDeleteClick}
            />
          )
        }}
        pageSizeOptions={[10, 25, 50, 100]}
        loading={isLoading || deleting}
        emptyMessage="No storage array credentials available"
        sx={{
          '& .MuiDataGrid-cell:focus': {
            outline: 'none'
          }
        }}
      />

      <ConfirmationDialog
        open={deleteDialogOpen}
        onClose={handleDeleteClose}
        title="Confirm Delete"
        icon={<WarningIcon color="warning" />}
        message={`Are you sure you want to delete storage array credential "${selectedForDeletion?.name}"?`}
        actionLabel="Delete"
        actionColor="error"
        actionVariant="outlined"
        onConfirm={handleConfirmDelete}
        customErrorMessage={getCustomErrorMessage}
        errorMessage={deleteError}
        onErrorChange={setDeleteError}
      />

      <ConfirmationDialog
        open={bulkDeleteDialogOpen}
        onClose={handleBulkDeleteClose}
        title="Confirm Delete"
        icon={<WarningIcon color="warning" />}
        message={`Are you sure you want to delete ${rowSelectionModel.length} storage array credential${rowSelectionModel.length > 1 ? 's' : ''}?`}
        items={selectedItems.map((item) => ({ id: item.id, name: item.name }))}
        actionLabel="Delete"
        actionColor="error"
        actionVariant="outlined"
        onConfirm={handleConfirmBulkDelete}
        customErrorMessage={getCustomErrorMessage}
        errorMessage={deleteError}
        onErrorChange={setDeleteError}
      />

      {addDrawerOpen && (
        <AddArrayCredentialsDrawer open={addDrawerOpen} onClose={handleCloseAddDrawer} />
      )}

      {editDrawerOpen && selectedForEdit && (
        <EditArrayCredentialsDrawer
          open={editDrawerOpen}
          onClose={handleCloseEditDrawer}
          credential={selectedForEdit.credObject}
        />
      )}
    </div>
  )
}
