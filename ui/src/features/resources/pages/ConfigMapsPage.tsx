import { useState, useCallback } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Box, Button, IconButton, Tooltip } from '@mui/material'
import DeleteIcon from '@mui/icons-material/DeleteOutlined'
import WarningIcon from '@mui/icons-material/Warning'
import MapIcon from '@mui/icons-material/Map'
import { GridColDef, GridRowSelectionModel } from '@mui/x-data-grid'
import { CommonDataGrid, CustomSearchToolbar, ListingToolbar } from 'src/components/grid'
import { ClickableTableCell } from 'src/components/design-system/ui'
import { ConfirmationDialog } from 'src/components/dialogs'
import { fetchConfigMaps, fetchConfigMap, deleteConfigMap, updateConfigMap, type KubernetesResource } from 'src/api/kubernetes/resources'
import YamlViewerDrawer from '../components/YamlViewerDrawer'

const CONFIGMAPS_QUERY_KEY = ['configmaps']

interface ConfigMapRow {
  id: string
  name: string
  createdAt: string
  raw: KubernetesResource
}

interface CustomToolbarProps {
  numSelected: number
  onDeleteSelected: () => void
  onRefresh: () => void
}

function CustomToolbar({ numSelected, onDeleteSelected, onRefresh }: CustomToolbarProps) {
  const search = (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
      {numSelected > 0 && (
        <Button
          variant="outlined"
          color="error"
          startIcon={<DeleteIcon />}
          onClick={onDeleteSelected}
          sx={{ height: 40 }}
        >
          Delete Selected ({numSelected})
        </Button>
      )}
      <CustomSearchToolbar placeholder="Search config maps..." onRefresh={onRefresh} />
    </Box>
  )
  return <ListingToolbar title="Config Maps" icon={<MapIcon />} search={search} />
}

export default function ConfigMapsPage() {
  const queryClient = useQueryClient()
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [selectedConfigMap, setSelectedConfigMap] = useState<KubernetesResource | null>(null)
  const [selectedName, setSelectedName] = useState('')
  const [fetchingYaml, setFetchingYaml] = useState(false)
  const [selectedIds, setSelectedIds] = useState<string[]>([])
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
  const [pendingDeleteItems, setPendingDeleteItems] = useState<{ id: string; name: string }[]>([])

  const { data: configMaps, isLoading, refetch } = useQuery({
    queryKey: CONFIGMAPS_QUERY_KEY,
    queryFn: () => fetchConfigMaps(),
    staleTime: 30_000
  })

  const rows: ConfigMapRow[] = [...(configMaps || [])]
    .sort((a, b) => a.metadata.name.localeCompare(b.metadata.name))
    .map((cm) => ({
      id: cm.metadata.name,
      name: cm.metadata.name,
      createdAt: cm.metadata.creationTimestamp
        ? new Date(cm.metadata.creationTimestamp).toLocaleString()
        : '—',
      raw: cm
    }))

  const handleViewYaml = useCallback(async (cm: KubernetesResource) => {
    setFetchingYaml(true)
    setSelectedName(cm.metadata.name)
    try {
      const full = await fetchConfigMap(cm.metadata.name)
      setSelectedConfigMap(full)
    } catch {
      setSelectedConfigMap(cm)
    } finally {
      setFetchingYaml(false)
      setDrawerOpen(true)
    }
  }, [])

  const handleDeleteSingle = useCallback((name: string) => {
    setPendingDeleteItems([{ id: name, name }])
    setDeleteDialogOpen(true)
  }, [])

  const handleDeleteSelected = useCallback(() => {
    setPendingDeleteItems(selectedIds.map((id) => ({ id, name: id })))
    setDeleteDialogOpen(true)
  }, [selectedIds])

  const handleConfirmDelete = useCallback(async () => {
    await Promise.all(pendingDeleteItems.map((item) => deleteConfigMap(item.name)))
    setSelectedIds([])
    queryClient.invalidateQueries({ queryKey: CONFIGMAPS_QUERY_KEY })
  }, [pendingDeleteItems, queryClient])

  const handleSaveConfigMap = useCallback(async (updated: unknown) => {
    if (!selectedConfigMap) return
    const saved = await updateConfigMap(selectedConfigMap.metadata.name, updated as KubernetesResource)
    setSelectedConfigMap(saved)
    queryClient.invalidateQueries({ queryKey: CONFIGMAPS_QUERY_KEY })
  }, [selectedConfigMap, queryClient])

  const handleSelectionChange = useCallback((model: GridRowSelectionModel) => {
    setSelectedIds(model as string[])
  }, [])

  const columns: GridColDef<ConfigMapRow>[] = [
    {
      field: 'name',
      headerName: 'Name',
      flex: 2,
      renderCell: (params) => (
        <ClickableTableCell onClick={() => handleViewYaml(params.row.raw)}>
          {params.value}
        </ClickableTableCell>
      )
    },
    {
      field: 'createdAt',
      headerName: 'Created',
      flex: 1
    },
    {
      field: 'actions',
      headerName: 'Actions',
      width: 80,
      sortable: false,
      filterable: false,
      renderCell: (params) => (
        <Tooltip title="Delete config map">
          <span>
            <IconButton
              onClick={(e) => {
                e.stopPropagation()
                handleDeleteSingle(params.row.name)
              }}
              aria-label="delete config map"
            >
              <DeleteIcon />
            </IconButton>
          </span>
        </Tooltip>
      )
    }
  ]

  return (
    <Box sx={{ height: '100%', display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
      <CommonDataGrid
        rows={rows}
        columns={columns}
        loading={isLoading || fetchingYaml}
        loadingMessage="Loading config maps..."
        emptyMessage="No config maps found"
        checkboxSelection
        disableRowSelectionOnClick
        rowSelectionModel={selectedIds}
        onRowSelectionModelChange={handleSelectionChange}
        slots={{
          toolbar: () => (
            <CustomToolbar
              numSelected={selectedIds.length}
              onDeleteSelected={handleDeleteSelected}
              onRefresh={refetch}
            />
          )
        }}
        sx={{ flex: 1 }}
      />

      <ConfirmationDialog
        open={deleteDialogOpen}
        onClose={() => setDeleteDialogOpen(false)}
        title="Confirm Delete"
        icon={<WarningIcon color="warning" />}
        message={
          pendingDeleteItems.length > 1
            ? `Are you sure you want to delete ${pendingDeleteItems.length} config maps?`
            : `Are you sure you want to delete config map "${pendingDeleteItems[0]?.name}"?`
        }
        items={pendingDeleteItems}
        actionLabel="Delete"
        actionColor="error"
        actionVariant="outlined"
        onConfirm={handleConfirmDelete}
      />

      {drawerOpen && selectedConfigMap && (
        <YamlViewerDrawer
          open={drawerOpen}
          onClose={() => { setDrawerOpen(false); setSelectedConfigMap(null) }}
          title="ConfigMap"
          subtitle={selectedName}
          data={selectedConfigMap}
          onSave={handleSaveConfigMap}
        />
      )}
    </Box>
  )
}
