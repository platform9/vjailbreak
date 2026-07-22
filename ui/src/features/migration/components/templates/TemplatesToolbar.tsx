import { useState } from 'react'
import {
  Box,
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
import { DATA_COPY_METHOD_LABEL } from '../../utils/templateLabels'
import type { TemplateCopyMethodFilter, TemplateSortKey } from '../../utils/templateFilters'

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

export interface TemplatesToolbarProps {
  query: string
  onQueryChange: (value: string) => void
  copyMethodFilter: TemplateCopyMethodFilter
  onCopyMethodFilterChange: (value: TemplateCopyMethodFilter) => void
  sortKey: TemplateSortKey
  onSortKeyChange: (value: TemplateSortKey) => void
  view: 'grid' | 'list'
  onViewChange: (value: 'grid' | 'list') => void
}

export default function TemplatesToolbar({
  query,
  onQueryChange,
  copyMethodFilter,
  onCopyMethodFilterChange,
  sortKey,
  onSortKeyChange,
  view,
  onViewChange
}: TemplatesToolbarProps) {
  const [filterAnchorEl, setFilterAnchorEl] = useState<null | HTMLElement>(null)
  const isTypeFilterActive = copyMethodFilter !== 'all'

  const handleFilterSelect = (value: TemplateCopyMethodFilter) => {
    onCopyMethodFilterChange(value)
    setFilterAnchorEl(null)
  }

  return (
    <Box sx={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 1.5 }}>
      <TextField
        placeholder="Search templates..."
        value={query}
        onChange={(event) => onQueryChange(event.target.value)}
        data-testid="templates-search"
        size="small"
        variant="standard"
        sx={{ width: '100%', maxWidth: 220, minWidth: 0 }}
        InputProps={{
          sx: { '& .MuiInputBase-input': { textOverflow: 'ellipsis' } },
          startAdornment: (
            <InputAdornment position="start">
              <SearchIcon fontSize="small" color="action" />
            </InputAdornment>
          )
        }}
      />

      <Tooltip title="Filter by migration type">
        <IconButton
          size="small"
          onClick={(event) => setFilterAnchorEl(event.currentTarget)}
          data-testid="templates-type-filter"
          sx={{ flexShrink: 0 }}
        >
          <FilterListIcon color={isTypeFilterActive ? 'primary' : 'inherit'} />
        </IconButton>
      </Tooltip>
      <Menu anchorEl={filterAnchorEl} open={Boolean(filterAnchorEl)} onClose={() => setFilterAnchorEl(null)}>
        {isTypeFilterActive && (
          <MenuItem onClick={() => handleFilterSelect('all')}>
            <CloseIcon fontSize="small" sx={{ mr: 1 }} />
            Clear filter
          </MenuItem>
        )}
        {COPY_METHOD_FILTER_OPTIONS.filter((option) => option.value !== 'all').map((option) => (
          <MenuItem
            key={option.value}
            selected={option.value === copyMethodFilter}
            onClick={() => handleFilterSelect(option.value)}
            data-testid={`templates-type-filter-${option.value}`}
          >
            {option.label}
          </MenuItem>
        ))}
      </Menu>

      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, flexShrink: 0 }}>
        <Typography variant="body2" color="text.secondary">
          Sort
        </Typography>
        <Select
          size="small"
          value={sortKey}
          onChange={(event) => onSortKeyChange(event.target.value as TemplateSortKey)}
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
        onChange={(_event, next) => next && onViewChange(next)}
        sx={{ flexShrink: 0 }}
      >
        <ToggleButton value="grid" data-testid="templates-view-grid">
          <GridViewIcon fontSize="small" />
        </ToggleButton>
        <ToggleButton value="list" data-testid="templates-view-list">
          <ViewListIcon fontSize="small" />
        </ToggleButton>
      </ToggleButtonGroup>
    </Box>
  )
}
