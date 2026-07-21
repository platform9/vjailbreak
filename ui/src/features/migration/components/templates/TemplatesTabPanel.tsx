import { useMemo, useState } from 'react'
import {
  Box,
  CircularProgress,
  Grid,
  IconButton,
  InputAdornment,
  Menu,
  MenuItem,
  Select,
  TextField,
  ToggleButton,
  ToggleButtonGroup,
  Tooltip,
  Typography
} from '@mui/material'
import SearchIcon from '@mui/icons-material/Search'
import FilterListIcon from '@mui/icons-material/FilterList'
import CloseIcon from '@mui/icons-material/Close'
import GridViewIcon from '@mui/icons-material/GridView'
import ViewListIcon from '@mui/icons-material/ViewList'
import CloudSyncIcon from '@mui/icons-material/CloudSync'
import { useMigrationTemplatesQuery } from '../../hooks/useMigrationTemplatesQuery'
import {
  filterTemplates,
  sortTemplates,
  type TemplateCopyMethodFilter,
  type TemplateSortKey
} from '../../utils/templateFilters'
import { DATA_COPY_METHOD_LABEL } from '../../utils/templateLabels'
import type { SavedTemplate } from '../../api/migration-blueprints/types'
import TemplateCard from './TemplateCard'
import TemplatesTable from './TemplatesTable'
import TemplateDetailDrawer from './TemplateDetailDrawer'

export interface TemplatesTabPanelProps {
  onUseTemplate: (template: SavedTemplate) => void
  onEditTemplate: (template: SavedTemplate) => void
}

const SORT_OPTIONS: Array<{ value: TemplateSortKey; label: string }> = [
  { value: 'created', label: 'Newest' },
  { value: 'name', label: 'Name' }
]

const COPY_METHOD_FILTER_OPTIONS: Array<{ value: TemplateCopyMethodFilter; label: string }> = [
  { value: 'all', label: 'All types' },
  { value: 'hot', label: DATA_COPY_METHOD_LABEL.hot },
  { value: 'cold', label: DATA_COPY_METHOD_LABEL.cold },
  { value: 'mock', label: DATA_COPY_METHOD_LABEL.mock }
]

export default function TemplatesTabPanel({ onUseTemplate, onEditTemplate }: TemplatesTabPanelProps) {
  const { data: templates = [], isLoading } = useMigrationTemplatesQuery()
  const [query, setQuery] = useState('')
  const [sortKey, setSortKey] = useState<TemplateSortKey>('created')
  const [copyMethodFilter, setCopyMethodFilter] = useState<TemplateCopyMethodFilter>('all')
  const [view, setView] = useState<'grid' | 'list'>('grid')
  const [selectedTemplate, setSelectedTemplate] = useState<SavedTemplate | null>(null)
  const [filterAnchorEl, setFilterAnchorEl] = useState<null | HTMLElement>(null)

  const isTypeFilterActive = copyMethodFilter !== 'all'

  const handleFilterSelect = (value: TemplateCopyMethodFilter) => {
    setCopyMethodFilter(value)
    setFilterAnchorEl(null)
  }

  const visibleTemplates = useMemo(
    () => sortTemplates(filterTemplates(templates, query, copyMethodFilter), sortKey),
    [templates, query, copyMethodFilter, sortKey]
  )

  if (isLoading) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', py: 6 }}>
        <CircularProgress size={28} />
      </Box>
    )
  }

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
      <Box
        sx={{
          display: 'flex',
          flexWrap: 'wrap',
          alignItems: 'center',
          justifyContent: 'flex-end',
          gap: 2
        }}
      >
        <TextField
          placeholder="Search templates..."
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          data-testid="templates-search"
          size="small"
          variant="standard"
          sx={{ width: '100%', maxWidth: 360 }}
          InputProps={{
            sx: { '& .MuiInputBase-input': { textOverflow: 'ellipsis' } },
            startAdornment: (
              <InputAdornment position="start">
                <SearchIcon fontSize="small" color="action" />
              </InputAdornment>
            )
          }}
        />

        <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
          <Tooltip title="Filter by migration type">
            <IconButton
              size="small"
              onClick={(event) => setFilterAnchorEl(event.currentTarget)}
              data-testid="templates-type-filter"
            >
              <FilterListIcon color={isTypeFilterActive ? 'primary' : 'inherit'} />
            </IconButton>
          </Tooltip>
          <Menu
            anchorEl={filterAnchorEl}
            open={Boolean(filterAnchorEl)}
            onClose={() => setFilterAnchorEl(null)}
          >
            {isTypeFilterActive && (
              <MenuItem onClick={() => handleFilterSelect('all')}>
                <CloseIcon fontSize="small" sx={{ mr: 1 }} />
                Clear filter
              </MenuItem>
            )}
            {COPY_METHOD_FILTER_OPTIONS.filter((option) => option.value !== 'all').map(
              (option) => (
                <MenuItem
                  key={option.value}
                  selected={option.value === copyMethodFilter}
                  onClick={() => handleFilterSelect(option.value)}
                  data-testid={`templates-type-filter-${option.value}`}
                >
                  {option.label}
                </MenuItem>
              )
            )}
          </Menu>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
            <Typography variant="body2" color="text.secondary">
              Sort
            </Typography>
            <Select
              size="small"
              value={sortKey}
              onChange={(event) => setSortKey(event.target.value as TemplateSortKey)}
              data-testid="templates-sort"
            >
              {SORT_OPTIONS.map((option) => (
                <MenuItem key={option.value} value={option.value}>
                  {option.label}
                </MenuItem>
              ))}
            </Select>
          </Box>

          <ToggleButtonGroup
            size="small"
            exclusive
            value={view}
            onChange={(_event, next) => next && setView(next)}
          >
            <ToggleButton value="grid" data-testid="templates-view-grid">
              <GridViewIcon fontSize="small" />
            </ToggleButton>
            <ToggleButton value="list" data-testid="templates-view-list">
              <ViewListIcon fontSize="small" />
            </ToggleButton>
          </ToggleButtonGroup>
        </Box>
      </Box>

      {templates.length === 0 ? (
        <Box sx={{ textAlign: 'center', py: 8 }}>
          <CloudSyncIcon sx={{ fontSize: 40, color: 'text.secondary', mb: 1 }} />
          <Typography variant="h6" component="p">
            No templates yet
          </Typography>
          <Typography variant="body2" color="text.secondary">
            Configure a migration, then use &ldquo;Save as template&rdquo; in the New Migration
            drawer to create your first one.
          </Typography>
        </Box>
      ) : visibleTemplates.length === 0 ? (
        <Box sx={{ textAlign: 'center', py: 8 }}>
          <Typography variant="h6" component="p">
            No templates match
          </Typography>
          <Typography variant="body2" color="text.secondary">
            Try a different search term or filter.
          </Typography>
        </Box>
      ) : view === 'list' ? (
        <TemplatesTable
          templates={visibleTemplates}
          onOpenDetail={setSelectedTemplate}
          onUse={onUseTemplate}
          onEdit={onEditTemplate}
        />
      ) : (
        <Grid container spacing={2}>
          {visibleTemplates.map((template) => (
            <Grid item xs={12} sm={6} md={4} key={template.name}>
              <TemplateCard
                template={template}
                onOpenDetail={setSelectedTemplate}
                onUse={onUseTemplate}
                onEdit={onEditTemplate}
              />
            </Grid>
          ))}
        </Grid>
      )}

      <TemplateDetailDrawer
        open={Boolean(selectedTemplate)}
        template={selectedTemplate}
        onClose={() => setSelectedTemplate(null)}
        onUse={onUseTemplate}
        onEdit={onEditTemplate}
      />
    </Box>
  )
}
