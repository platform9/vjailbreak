import { useMemo, useState } from 'react'
import {
  Box,
  CircularProgress,
  Grid,
  InputAdornment,
  MenuItem,
  Select,
  TextField,
  ToggleButton,
  ToggleButtonGroup,
  Typography
} from '@mui/material'
import SearchIcon from '@mui/icons-material/Search'
import GridViewIcon from '@mui/icons-material/GridView'
import ViewListIcon from '@mui/icons-material/ViewList'
import CloudSyncIcon from '@mui/icons-material/CloudSync'
import { useMigrationTemplatesQuery } from '../../hooks/useMigrationTemplatesQuery'
import { filterTemplates, sortTemplates, type TemplateSortKey } from '../../utils/templateFilters'
import type { SavedTemplate } from '../../mock-templates/types'
import TemplateCard from './TemplateCard'
import TemplateDetailDrawer from './TemplateDetailDrawer'

export interface TemplatesTabPanelProps {
  onUseTemplate: (template: SavedTemplate) => void
}

const SORT_OPTIONS: Array<{ value: TemplateSortKey; label: string }> = [
  { value: 'lastUsed', label: 'Last used' },
  { value: 'created', label: 'Newest' },
  { value: 'name', label: 'Name' },
  { value: 'timesUsed', label: 'Most used' }
]

export default function TemplatesTabPanel({ onUseTemplate }: TemplatesTabPanelProps) {
  const { data: templates = [], isLoading } = useMigrationTemplatesQuery()
  const [query, setQuery] = useState('')
  const [sortKey, setSortKey] = useState<TemplateSortKey>('lastUsed')
  const [view, setView] = useState<'grid' | 'list'>('grid')
  const [selectedTemplate, setSelectedTemplate] = useState<SavedTemplate | null>(null)

  const visibleTemplates = useMemo(
    () => sortTemplates(filterTemplates(templates, query), sortKey),
    [templates, query, sortKey]
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
          gap: 2,
          justifyContent: 'space-between'
        }}
      >
        <TextField
          size="small"
          placeholder="Search templates…"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          data-testid="templates-search"
          sx={{ minWidth: 280 }}
          InputProps={{
            startAdornment: (
              <InputAdornment position="start">
                <SearchIcon fontSize="small" color="action" />
              </InputAdornment>
            )
          }}
        />

        <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
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
      ) : (
        <Grid container spacing={2} direction={view === 'list' ? 'column' : 'row'}>
          {visibleTemplates.map((template) => (
            <Grid item xs={12} md={view === 'list' ? 12 : 6} key={template.name}>
              <TemplateCard
                template={template}
                onOpenDetail={setSelectedTemplate}
                onUse={onUseTemplate}
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
      />
    </Box>
  )
}
