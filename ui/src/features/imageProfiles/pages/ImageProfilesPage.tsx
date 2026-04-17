import { useState, useCallback } from 'react'
import { Box, Button, Chip, IconButton, Tooltip, Typography } from '@mui/material'
import AddIcon from '@mui/icons-material/Add'
import DeleteOutlineIcon from '@mui/icons-material/DeleteOutlined'
import EditOutlinedIcon from '@mui/icons-material/EditOutlined'
import LockIcon from '@mui/icons-material/Lock'
import TuneIcon from '@mui/icons-material/Tune'
import WarningIcon from '@mui/icons-material/Warning'
import { GridColDef, GridRowSelectionModel } from '@mui/x-data-grid'
import { useQueryClient, useMutation } from '@tanstack/react-query'
import { CommonDataGrid, ListingToolbar, CustomSearchToolbar } from 'src/components/grid'
import { ConfirmationDialog } from 'src/components/dialogs'
import {
  useVolumeImageProfilesQuery,
  VOLUME_IMAGE_PROFILES_QUERY_KEY
} from 'src/hooks/api/useVolumeImageProfilesQuery'
import { deleteVolumeImageProfile } from 'src/api/volume-image-profiles/volumeImageProfiles'
import { VolumeImageProfile, DEFAULT_PROFILE_NAMES } from 'src/api/volume-image-profiles/model'
import VolumeImageProfileDrawer from '../components/VolumeImageProfileDrawer'

const OS_FAMILY_COLOR: Record<string, 'primary' | 'success' | 'default'> = {
  windows: 'primary',
  linux: 'success',
  any: 'default'
}

interface ProfileToolbarProps {
  numSelected: number
  onDeleteSelected: () => void
  loading: boolean
  onRefresh: () => void
  onAddProfile: () => void
}

function ProfileToolbar({
  numSelected,
  onDeleteSelected,
  loading,
  onRefresh,
  onAddProfile
}: ProfileToolbarProps) {
  const search = (
    <Box sx={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 2 }}>
      {numSelected > 0 && (
        <Button
          variant="outlined"
          color="error"
          startIcon={<DeleteOutlineIcon />}
          onClick={onDeleteSelected}
          disabled={loading}
          sx={{ height: 40 }}
        >
          Delete Selected ({numSelected})
        </Button>
      )}
      <CustomSearchToolbar placeholder="Search by Name or OS Family" onRefresh={onRefresh} />
    </Box>
  )

  const actions = (
    <Button
      variant="contained"
      color="primary"
      startIcon={<AddIcon />}
      onClick={onAddProfile}
      sx={{ height: 40 }}
      data-tour="add-image-profile"
    >
      Add Profile
    </Button>
  )

  return (
    <ListingToolbar title="Image Profiles" icon={<TuneIcon />} search={search} actions={actions} />
  )
}

export default function ImageProfilesPage() {
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [editProfile, setEditProfile] = useState<VolumeImageProfile | null>(null)
  const [selectedIds, setSelectedIds] = useState<string[]>([])
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
  const [selectedForDeletion, setSelectedForDeletion] = useState<VolumeImageProfile[]>([])
  const [deleteError, setDeleteError] = useState<string | null>(null)
  const [deleting, setDeleting] = useState(false)

  const {
    data: profiles = [],
    isLoading: loadingProfiles,
    refetch
  } = useVolumeImageProfilesQuery(undefined, { staleTime: 0, refetchOnMount: true })
  const queryClient = useQueryClient()

  const { mutateAsync: doDelete } = useMutation({
    mutationFn: (name: string) => deleteVolumeImageProfile(name),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: VOLUME_IMAGE_PROFILES_QUERY_KEY })
  })

  const isDefault = (name: string) => DEFAULT_PROFILE_NAMES.includes(name)

  const handleOpenCreate = useCallback(() => {
    setEditProfile(null)
    setDrawerOpen(true)
  }, [])

  const handleOpenEdit = useCallback((profile: VolumeImageProfile) => {
    setEditProfile(profile)
    setDrawerOpen(true)
  }, [])

  const handleCloseDrawer = useCallback(() => {
    setDrawerOpen(false)
    setEditProfile(null)
    refetch()
  }, [refetch])

  const handleSelectionChange = (model: GridRowSelectionModel) => {
    setSelectedIds(model as string[])
  }

  const handleDeleteSelected = () => {
    const selected = profiles.filter((p) => selectedIds.includes(p.metadata.name))
    setSelectedForDeletion(selected)
    setDeleteDialogOpen(true)
  }

  const handleSingleDelete = (profile: VolumeImageProfile) => {
    setSelectedForDeletion([profile])
    setDeleteDialogOpen(true)
  }

  const handleDeleteClose = () => {
    setDeleteDialogOpen(false)
    setSelectedForDeletion([])
    setDeleteError(null)
  }

  const handleConfirmDelete = async () => {
    setDeleting(true)
    try {
      await Promise.all(
        selectedForDeletion.map((p) => doDelete(p.metadata.name))
      )
      setSelectedIds([])
      handleDeleteClose()
    } catch (error) {
      setDeleteError(error instanceof Error ? error.message : 'Unknown error occurred')
    } finally {
      setDeleting(false)
    }
  }

  const columns: GridColDef<VolumeImageProfile>[] = [
    {
      field: 'name',
      headerName: 'Name',
      flex: 1.5,
      valueGetter: (_value, row) => row.metadata.name,
      renderCell: ({ row }) => (
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
          <Typography variant="body2">{row.metadata.name}</Typography>
          {isDefault(row.metadata.name) && (
            <Tooltip title="System default profile">
              <LockIcon sx={{ fontSize: 14, color: 'text.secondary' }} />
            </Tooltip>
          )}
        </Box>
      )
    },
    {
      field: 'osFamily',
      headerName: 'OS Family',
      flex: 1,
      valueGetter: (_value, row) => row.spec.osFamily,
      renderCell: ({ row }) => (
        <Chip
          label={row.spec.osFamily}
          size="small"
          color={OS_FAMILY_COLOR[row.spec.osFamily] ?? 'default'}
          variant="outlined"
          sx={{ textTransform: 'capitalize' }}
        />
      )
    },
    {
      field: 'properties',
      headerName: 'Properties',
      flex: 1,
      sortable: false,
      renderCell: ({ row }) => {
        const count = Object.keys(row.spec.properties).length
        return (
          <Chip
            label={`${count} prop${count !== 1 ? 's' : ''}`}
            size="small"
            variant="outlined"
            color="default"
          />
        )
      }
    },
    {
      field: 'description',
      headerName: 'Description',
      flex: 2,
      valueGetter: (_value, row) => row.spec.description ?? '—'
    },
    {
      field: 'actions',
      headerName: 'Actions',
      flex: 1,
      sortable: false,
      renderCell: ({ row }) => (
        <Box>
          <Tooltip title="Edit">
            <IconButton
              onClick={(e) => {
                e.stopPropagation()
                handleOpenEdit(row)
              }}
            >
              <EditOutlinedIcon />
            </IconButton>
          </Tooltip>
          <Tooltip
            title={isDefault(row.metadata.name) ? 'Cannot delete system default profiles' : 'Delete'}
          >
            <span>
              <IconButton
                onClick={(e) => {
                  e.stopPropagation()
                  handleSingleDelete(row)
                }}
                disabled={isDefault(row.metadata.name)}
              >
                <DeleteOutlineIcon />
              </IconButton>
            </span>
          </Tooltip>
        </Box>
      )
    }
  ]

  const isLoading = loadingProfiles || deleting

  return (
    <div style={{ height: '100%', width: '100%', overflow: 'hidden' }}>
      <CommonDataGrid
        rows={profiles}
        columns={columns}
        loading={isLoading}
        loadingMessage="Loading image profiles..."
        emptyMessage="No image profiles yet"
        disableRowSelectionOnClick
        checkboxSelection
        rowSelectionModel={selectedIds}
        onRowSelectionModelChange={handleSelectionChange}
        getRowId={(row) => row.metadata.name}
        initialState={{
          sorting: {
            sortModel: [{ field: 'name', sort: 'asc' }]
          },
          pagination: {
            paginationModel: { pageSize: 25 }
          }
        }}
        pageSizeOptions={[10, 25, 50, 100]}
        slots={{
          toolbar: () => (
            <ProfileToolbar
              numSelected={selectedIds.length}
              onDeleteSelected={handleDeleteSelected}
              loading={isLoading}
              onRefresh={refetch}
              onAddProfile={handleOpenCreate}
            />
          )
        }}
        sx={{
          '& .MuiDataGrid-cell:focus': {
            outline: 'none'
          }
        }}
      />

      <VolumeImageProfileDrawer
        open={drawerOpen}
        onClose={handleCloseDrawer}
        editProfile={editProfile}
      />

      <ConfirmationDialog
        open={deleteDialogOpen}
        onClose={handleDeleteClose}
        title="Confirm Delete"
        icon={<WarningIcon color="warning" />}
        message={
          selectedForDeletion.length > 1
            ? 'Are you sure you want to delete these image profiles?'
            : `Are you sure you want to delete profile "${selectedForDeletion[0]?.metadata.name}"?`
        }
        items={selectedForDeletion.map((p) => ({ id: p.metadata.name, name: p.metadata.name }))}
        actionLabel="Delete"
        actionColor="error"
        actionVariant="outlined"
        onConfirm={handleConfirmDelete}
        errorMessage={deleteError}
        onErrorChange={setDeleteError}
      />
    </div>
  )
}
