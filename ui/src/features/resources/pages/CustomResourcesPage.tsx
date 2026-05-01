import { useState, useCallback, useMemo } from 'react'
import { useQuery, useQueries, useQueryClient } from '@tanstack/react-query'
import {
  Box,
  Typography,
  IconButton,
  Tooltip,
  CircularProgress,
  Button,
  Checkbox
} from '@mui/material'
import { GridColDef, GridRowParams, GridSortModel } from '@mui/x-data-grid'
import LayersIcon from '@mui/icons-material/Layers'
import ExpandMoreIcon from '@mui/icons-material/ExpandMore'
import ChevronRightIcon from '@mui/icons-material/ChevronRight'
import DeleteIcon from '@mui/icons-material/DeleteOutlined'
import WarningIcon from '@mui/icons-material/Warning'
import { CommonDataGrid, ListingToolbar, CustomSearchToolbar } from 'src/components/grid'
import { ClickableTableCell } from 'src/components/design-system/ui'
import { ConfirmationDialog } from 'src/components/dialogs'
import {
  fetchVjailbreakResourceTypes,
  fetchCustomResources,
  fetchCustomResource,
  deleteCustomResource,
  updateCustomResource,
  type KubernetesResource
} from 'src/api/kubernetes/resources'
import YamlViewerDrawer from '../components/YamlViewerDrawer'

interface CRTableRow {
  id: string
  rowType: 'group' | 'child' | 'loading' | 'empty'
  kind: string
  resourceName: string
  isExpanded: boolean
  name: string
  createdAt: string
  raw: KubernetesResource | null
}

function CustomToolbar({
  numSelected,
  onDeleteSelected,
  onRefresh
}: {
  numSelected: number
  onDeleteSelected: () => void
  onRefresh: () => void
}) {
  const search = (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
      {numSelected > 0 && (
        <Button
          variant="outlined"
          color="error"
          size="small"
          startIcon={<DeleteIcon />}
          onClick={onDeleteSelected}
        >
          Delete Selected ({numSelected})
        </Button>
      )}
      <CustomSearchToolbar placeholder="Search resource types..." onRefresh={onRefresh} />
    </Box>
  )
  return <ListingToolbar title="Custom Resources" icon={<LayersIcon />} search={search} />
}

export default function CustomResourcesPage() {
  const queryClient = useQueryClient()
  const [expandedGroups, setExpandedGroups] = useState<Set<string>>(new Set())
  const [sortModel, setSortModel] = useState<GridSortModel>([])
  const [selectedIds, setSelectedIds] = useState<string[]>([])
  const [searchTerm, setSearchTerm] = useState('')
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
  const [pendingDeleteItems, setPendingDeleteItems] = useState<{ id: string; name: string }[]>([])
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [selectedResource, setSelectedResource] = useState<KubernetesResource | null>(null)
  const [selectedKind, setSelectedKind] = useState('')
  const [selectedResourceName, setSelectedResourceName] = useState('')
  const [fetchingYaml, setFetchingYaml] = useState(false)

  const { data: resourceTypes, isLoading: typesLoading } = useQuery({
    queryKey: ['vjailbreak-resource-types'],
    queryFn: fetchVjailbreakResourceTypes,
    staleTime: 60_000
  })

  // Only fetch instances for expanded groups
  const instanceQueries = useQueries({
    queries: (resourceTypes || []).map((rt) => ({
      queryKey: ['custom-resources', rt.name],
      queryFn: () => fetchCustomResources(rt.name),
      staleTime: 30_000,
      enabled: expandedGroups.has(rt.name)
    }))
  })

  // Filter and sort resource types alphabetically by kind
  const filteredResourceTypes = useMemo(() => {
    if (!resourceTypes) return []
    const filtered = searchTerm.trim()
      ? resourceTypes.filter((rt) => rt.kind.toLowerCase().includes(searchTerm.toLowerCase()))
      : resourceTypes
    return [...filtered].sort((a, b) => a.kind.localeCompare(b.kind))
  }, [resourceTypes, searchTerm])

  // Build flat row list: group rows + child rows (when expanded)
  const rows = useMemo((): CRTableRow[] => {
    const result: CRTableRow[] = []
    filteredResourceTypes.forEach((rt, idx) => {
      const globalIdx = resourceTypes?.findIndex((r) => r.name === rt.name) ?? idx
      const isExpanded = expandedGroups.has(rt.name)
      const query = instanceQueries[globalIdx]
      const isLoading = isExpanded && (query?.isLoading ?? false)
      const instances = query?.data || []

      result.push({
        id: `group:${rt.name}`,
        rowType: 'group',
        kind: rt.kind,
        resourceName: rt.name,
        isExpanded,
        name: rt.kind,
        createdAt: '',
        raw: null
      })

      if (isExpanded) {
        if (isLoading) {
          result.push({
            id: `loading:${rt.name}`,
            rowType: 'loading',
            kind: rt.kind,
            resourceName: rt.name,
            isExpanded: false,
            name: '',
            createdAt: '',
            raw: null
          })
        } else if (instances.length === 0) {
          result.push({
            id: `empty:${rt.name}`,
            rowType: 'empty',
            kind: rt.kind,
            resourceName: rt.name,
            isExpanded: false,
            name: '',
            createdAt: '',
            raw: null
          })
        } else {
          instances.forEach((cr) => {
            result.push({
              id: `child:${rt.name}:${cr.metadata.name}`,
              rowType: 'child',
              kind: rt.kind,
              resourceName: rt.name,
              isExpanded: false,
              name: cr.metadata.name,
              createdAt: cr.metadata.creationTimestamp
                ? new Date(cr.metadata.creationTimestamp).toLocaleString()
                : '—',
              raw: cr
            })
          })
        }
      }
    })
    return result
  }, [filteredResourceTypes, resourceTypes, expandedGroups, instanceQueries])

  const handleToggleGroup = useCallback((resourceName: string) => {
    setExpandedGroups((prev) => {
      const next = new Set(prev)
      if (next.has(resourceName)) {
        next.delete(resourceName)
      } else {
        next.add(resourceName)
        setSortModel([]) // clear sort when opening a group to preserve hierarchy
      }
      return next
    })
  }, [])

  const handleViewYaml = useCallback(async (row: CRTableRow) => {
    if (!row.raw) return
    setFetchingYaml(true)
    setSelectedKind(row.kind)
    setSelectedResourceName(row.resourceName)
    try {
      const full = await fetchCustomResource(row.resourceName, row.raw.metadata.name)
      setSelectedResource(full)
    } catch {
      setSelectedResource(row.raw)
    } finally {
      setFetchingYaml(false)
      setDrawerOpen(true)
    }
  }, [])

  const handleSaveCR = useCallback(async (updated: unknown) => {
    if (!selectedResource || !selectedResourceName) return
    const saved = await updateCustomResource(
      selectedResourceName,
      selectedResource.metadata.name,
      updated as KubernetesResource
    )
    setSelectedResource(saved)
    queryClient.invalidateQueries({ queryKey: ['custom-resources', selectedResourceName] })
  }, [selectedResource, selectedResourceName, queryClient])

  const handleDeleteSingle = useCallback((row: CRTableRow) => {
    setPendingDeleteItems([{ id: row.id, name: row.name }])
    setDeleteDialogOpen(true)
  }, [])

  const handleDeleteSelected = useCallback(() => {
    const items = selectedIds
      .map((id) => rows.find((r) => r.id === id))
      .filter((r): r is CRTableRow => r?.rowType === 'child')
      .map((r) => ({ id: r.id, name: r.name }))
    setPendingDeleteItems(items)
    setDeleteDialogOpen(true)
  }, [selectedIds, rows])

  const handleConfirmDelete = useCallback(async () => {
    await Promise.all(
      pendingDeleteItems.map((item) => {
        const row = rows.find((r) => r.id === item.id)
        return row ? deleteCustomResource(row.resourceName, row.name) : Promise.resolve()
      })
    )
    const affectedTypes = new Set(
      pendingDeleteItems
        .map((item) => rows.find((r) => r.id === item.id)?.resourceName)
        .filter(Boolean) as string[]
    )
    affectedTypes.forEach((rt) =>
      queryClient.invalidateQueries({ queryKey: ['custom-resources', rt] })
    )
    setSelectedIds([])
  }, [pendingDeleteItems, rows, queryClient])

  const handleRefresh = useCallback(() => {
    queryClient.invalidateQueries({ queryKey: ['vjailbreak-resource-types'] })
    queryClient.invalidateQueries({ queryKey: ['custom-resources'] })
    setExpandedGroups(new Set())
  }, [queryClient])

  const anyExpanded = expandedGroups.size > 0
  const childRows = rows.filter((r) => r.rowType === 'child')
  const allChildSelected = childRows.length > 0 && childRows.every((r) => selectedIds.includes(r.id))
  const someChildSelected = childRows.some((r) => selectedIds.includes(r.id))

  const handleToggleSelectAll = useCallback(() => {
    setSelectedIds(allChildSelected ? [] : childRows.map((r) => r.id))
  }, [allChildSelected, childRows])

  const handleToggleSelectRow = useCallback((id: string) => {
    setSelectedIds((prev) =>
      prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]
    )
  }, [])

  const columns: GridColDef<CRTableRow>[] = [
    {
      field: '__toggle',
      headerName: '',
      width: 52,
      sortable: false,
      filterable: false,
      renderHeader: () => (
        <Checkbox
          size="small"
          checked={allChildSelected}
          indeterminate={someChildSelected && !allChildSelected}
          onChange={handleToggleSelectAll}
          onClick={(e) => e.stopPropagation()}
        />
      ),
      renderCell: (params) => {
        const row = params.row
        if (row.rowType === 'group') {
          return (
            <IconButton size="small" sx={{ p: 0.25 }} onClick={(e) => { e.stopPropagation(); handleToggleGroup(row.resourceName) }}>
              {row.isExpanded ? <ExpandMoreIcon fontSize="small" /> : <ChevronRightIcon fontSize="small" />}
            </IconButton>
          )
        }
        if (row.rowType === 'child') {
          return (
            <Checkbox
              size="small"
              checked={selectedIds.includes(row.id)}
              onChange={() => handleToggleSelectRow(row.id)}
              onClick={(e) => e.stopPropagation()}
            />
          )
        }
        return null
      }
    },
    {
      field: 'name',
      headerName: 'Name',
      flex: 3,
      sortable: !anyExpanded,
      filterable: false,
      renderCell: (params) => {
        const row = params.row

        if (row.rowType === 'loading') {
          return (
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
              <CircularProgress size={14} />
              <Typography variant="body2" color="text.secondary">Loading...</Typography>
            </Box>
          )
        }

        if (row.rowType === 'empty') {
          return (
            <Typography variant="body2" color="text.secondary" sx={{ fontStyle: 'italic', pl: 2 }}>
              No resources created yet
            </Typography>
          )
        }

        if (row.rowType === 'group') {
          return (
            <Typography variant="body2" sx={{ fontWeight: 600 }}>
              {row.kind}
            </Typography>
          )
        }

        return (
          <Box sx={{ pl: 4 }}>
            <ClickableTableCell onClick={() => handleViewYaml(row)}>
              {row.name}
            </ClickableTableCell>
          </Box>
        )
      }
    },
    {
      field: 'createdAt',
      headerName: 'Created',
      flex: 1,
      sortable: false,
      filterable: false,
      renderCell: (params) =>
        params.row.rowType === 'child' ? params.row.createdAt : null
    },
    {
      field: 'actions',
      headerName: 'Actions',
      width: 80,
      sortable: false,
      filterable: false,
      renderCell: (params) => {
        if (params.row.rowType !== 'child') return null
        return (
          <Tooltip title="Delete">
            <span>
              <IconButton
                size="small"
                onClick={(e) => {
                  e.stopPropagation()
                  handleDeleteSingle(params.row)
                }}
              >
                <DeleteIcon />
              </IconButton>
            </span>
          </Tooltip>
        )
      }
    }
  ]

  return (
    <Box sx={{ height: '100%', display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
      <CommonDataGrid
        rows={rows}
        columns={columns}
        loading={typesLoading || fetchingYaml}
        loadingMessage="Loading resource types..."
        emptyMessage="No resource types found"
        disableRowSelectionOnClick
        sortModel={sortModel}
        onSortModelChange={(model) => !anyExpanded && setSortModel(model)}
        onRowClick={(params: GridRowParams) => {
          if (params.row.rowType === 'group') handleToggleGroup(params.row.resourceName)
        }}
        onFilterModelChange={(model) =>
          setSearchTerm((model.quickFilterValues || []).join(' '))
        }
        slots={{
          toolbar: () => (
            <CustomToolbar
              numSelected={selectedIds.length}
              onDeleteSelected={handleDeleteSelected}
              onRefresh={handleRefresh}
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
            ? `Are you sure you want to delete ${pendingDeleteItems.length} resources?`
            : `Are you sure you want to delete "${pendingDeleteItems[0]?.name}"?`
        }
        items={pendingDeleteItems}
        actionLabel="Delete"
        actionColor="error"
        actionVariant="outlined"
        onConfirm={handleConfirmDelete}
      />

      {drawerOpen && selectedResource && (
        <YamlViewerDrawer
          open={drawerOpen}
          onClose={() => { setDrawerOpen(false); setSelectedResource(null) }}
          title={selectedKind}
          subtitle={selectedResource.metadata?.name}
          data={selectedResource}
          onSave={handleSaveCR}
        />
      )}
    </Box>
  )
}
