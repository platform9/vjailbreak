import {
  DataGrid,
  GridColDef,
  GridToolbarContainer,
  GridRowSelectionModel,
} from "@mui/x-data-grid"
import {
  Button,
  Typography,
  Box,
  IconButton,
  Tooltip,
  Chip,
  Alert,
} from "@mui/material"
import DeleteIcon from '@mui/icons-material/DeleteOutlined'
import EditIcon from '@mui/icons-material/Edit'
import AddIcon from '@mui/icons-material/Add'
import StorageIcon from '@mui/icons-material/Storage'
// import CustomSearchToolbar from "src/components/grid/CustomSearchToolbar"
import { useState, useCallback } from "react"
import { useArrayCredsQuery, useDeleteArrayCredsMutation } from "src/hooks/api/useArrayCredsQuery"
import { ArrayCreds } from "src/api/array-creds"
import ConfirmationDialog from "src/components/dialogs/ConfirmationDialog"
import { useErrorHandler } from "src/hooks/useErrorHandler"
import ArrayCredsDrawer from "src/components/drawers/ArrayCredsDrawer"
import CustomLoadingOverlay from "src/components/grid/CustomLoadingOverlay"

interface ArrayCredsRow {
  id: string
  name: string
  vendorType: string
  volumeType: string
  backendName: string
  autoDiscovered: boolean
  status: string
  hasCredentials: boolean
  credentialsStatus: 'configured' | 'pending' | 'failed'
  credObject: ArrayCreds
  onEdit: (cred: ArrayCreds) => void
  onDelete: (id: string) => void
}

const getVendorTypeColor = (vendor: string): "primary" | "secondary" | "success" | "warning" => {
  switch (vendor.toLowerCase()) {
    case 'pure':
      return 'primary'
    case 'ontap':
    case 'netapp':
      return 'success'
    case 'hpalletra':
      return 'warning'
    default:
      return 'secondary'
  }
}

const columns: GridColDef[] = [
  {
    field: "name",
    headerName: "Name",
    flex: 1.5,
    minWidth: 200,
  },
  {
    field: "vendorType",
    headerName: "Vendor",
    flex: 1,
    minWidth: 120,
    renderCell: (params) => (
      <Chip
        label={params.value}
        color={getVendorTypeColor(params.value)}
        variant="outlined"
        size="small"
      />
    ),
  },
  {
    field: "volumeType",
    headerName: "Volume Type",
    flex: 1.2,
    minWidth: 150,
  },
  {
    field: "backendName",
    headerName: "Backend Name",
    flex: 1.2,
    minWidth: 150,
  },
  {
    field: "autoDiscovered",
    headerName: "Source",
    flex: 0.8,
    minWidth: 120,
    renderCell: (params) => (
      <Chip
        label={params.value ? 'Auto-discovered' : 'Manual'}
        color={params.value ? 'info' : 'default'}
        variant="outlined"
        size="small"
      />
    ),
  },
  {
    field: "credentialsStatus",
    headerName: "Credentials",
    flex: 0.8,
    minWidth: 120,
    renderCell: (params) => {
      const status = params.value as 'configured' | 'pending' | 'failed'
      const label = status === 'configured' ? 'Configured' : status === 'failed' ? 'Failed' : 'Pending'
      const color = status === 'configured' ? 'success' : status === 'failed' ? 'error' : 'warning'
      const variant = status === 'configured' ? 'filled' : 'outlined'
      
      return (
        <Chip
          label={label}
          color={color}
          variant={variant}
          size="small"
        />
      )
    },
  },
  {
    field: 'actions',
    headerName: 'Actions',
    flex: 0.8,
    minWidth: 100,
    sortable: false,
    renderCell: (params) => (
      <Box sx={{ display: 'flex', gap: 0.5 }}>
        <Tooltip title="Edit array credentials">
          <IconButton
            size="small"
            onClick={(e) => {
              e.stopPropagation()
              if (params.row.onEdit) {
                params.row.onEdit(params.row.credObject)
              }
            }}
            aria-label="edit array credentials"
          >
            <EditIcon fontSize="small" />
          </IconButton>
        </Tooltip>
        <Tooltip title="Delete array credentials">
          <IconButton
            size="small"
            onClick={(e) => {
              e.stopPropagation()
              if (params.row.onDelete) {
                params.row.onDelete(params.row.id)
              }
            }}
            aria-label="delete array credentials"
          >
            <DeleteIcon fontSize="small" />
          </IconButton>
        </Tooltip>
      </Box>
    ),
  },
]

interface CustomToolbarProps {
  numSelected: number
  onDeleteSelected: () => void
  onAddNew: () => void
}

function CustomToolbar({ numSelected, onDeleteSelected, onAddNew }: CustomToolbarProps) {
  return (
    <GridToolbarContainer sx={{ p: 2, justifyContent: 'space-between' }}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
        <StorageIcon color="primary" />
        <Typography variant="h6">Storage Array Credentials</Typography>
      </Box>
      <Box sx={{ display: 'flex', gap: 1 }}>
        {numSelected > 0 && (
          <Button
            variant="outlined"
            color="error"
            startIcon={<DeleteIcon />}
            onClick={onDeleteSelected}
          >
            Delete Selected ({numSelected})
          </Button>
        )}
        <Button
          variant="contained"
          startIcon={<AddIcon />}
          onClick={onAddNew}
        >
          Add Array Credentials
        </Button>
      </Box>
    </GridToolbarContainer>
  )
}

export default function StorageManagementTable() {
  const { data: arrayCreds, isLoading, error } = useArrayCredsQuery()
  const deleteMutation = useDeleteArrayCredsMutation()
  const { reportError } = useErrorHandler()

  const [selectedRows, setSelectedRows] = useState<GridRowSelectionModel>([])
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
  const [itemToDelete, setItemToDelete] = useState<string | null>(null)
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [editingCred, setEditingCred] = useState<ArrayCreds | null>(null)

  const handleDelete = useCallback((id: string) => {
    setItemToDelete(id)
    setDeleteDialogOpen(true)
  }, [])

  const handleEdit = useCallback((cred: ArrayCreds) => {
    setEditingCred(cred)
    setDrawerOpen(true)
  }, [])

  const handleAddNew = useCallback(() => {
    setEditingCred(null)
    setDrawerOpen(true)
  }, [])

  const confirmDelete = useCallback(async () => {
    if (itemToDelete) {
      try {
        await deleteMutation.mutateAsync(itemToDelete)
        setDeleteDialogOpen(false)
        setItemToDelete(null)
      } catch (error) {
        reportError(error as Error, { context: 'Failed to delete array credentials' })
      }
    }
  }, [itemToDelete, deleteMutation, reportError])

  const handleDeleteSelected = useCallback(() => {
    // Implement bulk delete if needed
    console.log('Delete selected:', selectedRows)
  }, [selectedRows])

  const rows: ArrayCredsRow[] = (arrayCreds || []).map((cred) => {
    const isAutoDiscovered = cred.metadata.labels?.['vjailbreak.k8s.pf9.io/auto-discovered'] === 'true'
    const hasSecretRef = !!cred.spec.secretRef?.name
    const validationStatus = cred.status?.arrayValidationStatus
    
    // Determine credentials status
    let credentialsStatus: 'configured' | 'pending' | 'failed' = 'pending'
    if (hasSecretRef) {
      if (validationStatus === 'Succeeded') {
        credentialsStatus = 'configured'
      } else if (validationStatus === 'Failed') {
        credentialsStatus = 'failed'
      } else {
        // Has secret but validation is pending/in-progress
        credentialsStatus = 'pending'
      }
    }
    
    return {
      id: cred.metadata.name,
      name: cred.metadata.name,
      vendorType: cred.spec.vendorType || 'Unknown',
      volumeType: cred.spec.openstackMapping?.volumeType || 'N/A',
      backendName: cred.spec.openstackMapping?.cinderBackendName || 'N/A',
      autoDiscovered: isAutoDiscovered,
      status: validationStatus || 'Unknown',
      hasCredentials: hasSecretRef,
      credentialsStatus,
      credObject: cred,
      onEdit: handleEdit,
      onDelete: handleDelete,
    }
  })

  if (error) {
    return (
      <Box sx={{ p: 3 }}>
        <Alert severity="error">
          Failed to load array credentials: {error.message}
        </Alert>
      </Box>
    )
  }

  return (
    <Box sx={{ height: '100%', width: '100%', display: 'flex', flexDirection: 'column' }}>
      <DataGrid
        rows={rows}
        columns={columns}
        loading={isLoading}
        checkboxSelection
        disableRowSelectionOnClick
        onRowSelectionModelChange={setSelectedRows}
        rowSelectionModel={selectedRows}
        slots={{
          toolbar: () => (
            <CustomToolbar
              numSelected={selectedRows.length}
              onDeleteSelected={handleDeleteSelected}
              onAddNew={handleAddNew}
            />
          ),
          loadingOverlay: CustomLoadingOverlay,
        }}
        initialState={{
          pagination: {
            paginationModel: { pageSize: 25 },
          },
        }}
        pageSizeOptions={[10, 25, 50, 100]}
        sx={{
          '& .MuiDataGrid-cell:focus': {
            outline: 'none',
          },
          '& .MuiDataGrid-row:hover': {
            cursor: 'pointer',
          },
        }}
      />

      <ArrayCredsDrawer
        open={drawerOpen}
        onClose={() => {
          setDrawerOpen(false)
          setEditingCred(null)
        }}
        arrayCreds={editingCred}
      />

      <ConfirmationDialog
        open={deleteDialogOpen}
        onClose={() => {
          setDeleteDialogOpen(false)
          setItemToDelete(null)
        }}
        title="Delete Array Credentials"
        message={`Are you sure you want to delete "${itemToDelete}"? This action cannot be undone.`}
        onConfirm={confirmDelete}
        actionLabel="Delete"
        cancelLabel="Cancel"
        actionColor="error"
      />
    </Box>
  )
}
