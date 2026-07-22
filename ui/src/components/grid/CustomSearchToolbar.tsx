import { Box, IconButton, InputAdornment, TextField, Tooltip, Typography, Menu, MenuItem } from '@mui/material'
import { keyframes } from '@mui/material/styles'
import { GridToolbarQuickFilter } from '@mui/x-data-grid'
import {
  Search as SearchIcon,
  RefreshRounded,
  FilterList as FilterListIcon,
  CalendarToday as CalendarIcon,
  Close as CloseIcon
} from '@mui/icons-material'
import { useState } from 'react'

const spinAnimation = keyframes`
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
`

// Exported so consumers that implement their own date-range filtering (the actual
// "is this row within N days" logic lives with the data, not here) share the exact
// same set of option labels instead of re-typing them.
// eslint-disable-next-line react-refresh/only-export-components
export const DATE_FILTER_OPTIONS = ['All Time', 'Last 24 hours', 'Last 7 days', 'Last 30 days'] as const

interface CustomSearchToolbarProps {
  title?: string
  onRefresh?: () => void
  disableRefresh?: boolean
  isRefreshing?: boolean
  placeholder?: string
  maxSearchWidth?: number | string
  onStatusFilterChange?: (filter: string) => void
  currentStatusFilter?: string
  statusFilterOptions?: string[]
  onDateFilterChange?: (filter: string) => void
  currentDateFilter?: string
  // When set, the search field is a plain controlled TextField instead of
  // GridToolbarQuickFilter — lets this toolbar be used outside a DataGrid (e.g. sitting
  // inline with page tabs), with the caller owning the filtering.
  searchValue?: string
  onSearchChange?: (value: string) => void
}

const CustomSearchToolbar = ({
  title,
  onRefresh,
  disableRefresh = false,
  isRefreshing = false,
  placeholder = 'Search',
  maxSearchWidth = 360,
  onStatusFilterChange,
  currentStatusFilter = 'All',
  statusFilterOptions = ['All', 'In Progress', 'Succeeded', 'Failed'],
  onDateFilterChange,
  currentDateFilter = 'All Time',
  searchValue,
  onSearchChange
}: CustomSearchToolbarProps) => {
  const isStandaloneSearch = Boolean(onSearchChange)
  const [statusAnchorEl, setStatusAnchorEl] = useState<null | HTMLElement>(null)
  const [dateAnchorEl, setDateAnchorEl] = useState<null | HTMLElement>(null)

  const statusMenuOpen = Boolean(statusAnchorEl)
  const dateMenuOpen = Boolean(dateAnchorEl)

  const isDateFilterActive = currentDateFilter !== 'All Time'
  const isStatusFilterActive = currentStatusFilter !== 'All'

  const handleMenuClick = (event: React.MouseEvent<HTMLElement>, menu: 'status' | 'date') => {
    if (menu === 'status') setStatusAnchorEl(event.currentTarget)
    if (menu === 'date') setDateAnchorEl(event.currentTarget)
  }

  const handleMenuClose = () => {
    setStatusAnchorEl(null)
    setDateAnchorEl(null)
  }

  const handleFilterSelect = (filter: string, menu: 'status' | 'date') => {
    if (menu === 'status') onStatusFilterChange?.(filter)
    if (menu === 'date') onDateFilterChange?.(filter)
    handleMenuClose()
  }

  return (
    <Box
      sx={{
        p: 1,
        display: 'flex',
        alignItems: 'center'
      }}
    >
      {title && <Typography variant="h6">{title}</Typography>}
      <Box sx={{ marginLeft: 'auto', display: 'flex', gap: 1, alignItems: 'center' }}>
        <Box sx={{ maxWidth: maxSearchWidth, width: '100%' }}>
          {isStandaloneSearch ? (
            <TextField
              placeholder={placeholder}
              value={searchValue}
              onChange={(event) => onSearchChange?.(event.target.value)}
              size="small"
              variant="standard"
              fullWidth
              InputProps={{
                sx: { '& .MuiInputBase-input': { textOverflow: 'ellipsis' } },
                startAdornment: (
                  <InputAdornment position="start">
                    <SearchIcon fontSize="small" color="action" />
                  </InputAdornment>
                )
              }}
            />
          ) : (
            <div>
              <GridToolbarQuickFilter
                placeholder={placeholder}
                sx={{
                  '& .MuiInputBase-input': {
                    textOverflow: 'ellipsis'
                  }
                }}
              />
            </div>
          )}
        </Box>
        <Box sx={{ display: 'flex', alignItems: 'center' }}>
          {onRefresh && (
            <Tooltip title={isRefreshing ? 'Revalidating VMware credentials...' : 'Refresh'}>
              <span>
                <IconButton data-testid="vm-list-refresh-button" onClick={onRefresh} disabled={disableRefresh || isRefreshing} size="small">
                  <RefreshRounded
                    sx={isRefreshing ? { animation: `${spinAnimation} 1s linear infinite` } : undefined}
                  />
                </IconButton>
              </span>
            </Tooltip>
          )}
          {onDateFilterChange && (
            <>
              <Tooltip title="Filter by creation date">
                <IconButton data-testid="date-filter" onClick={(e) => handleMenuClick(e, 'date')} size="small">
                  <CalendarIcon
                    fontSize="small"
                    color={isDateFilterActive ? 'primary' : 'inherit'}
                  />
                </IconButton>
              </Tooltip>
              <Menu anchorEl={dateAnchorEl} open={dateMenuOpen} onClose={handleMenuClose}>
                {isDateFilterActive && (
                  <MenuItem onClick={() => handleFilterSelect('All Time', 'date')}>
                    <CloseIcon fontSize="small" sx={{ mr: 1 }} />
                    Clear filter
                  </MenuItem>
                )}
                {DATE_FILTER_OPTIONS.map((option) => (
                  <MenuItem
                    key={option}
                    selected={option === currentDateFilter}
                    onClick={() => handleFilterSelect(option, 'date')}
                  >
                    {option}
                  </MenuItem>
                ))}
              </Menu>
            </>
          )}
          {onStatusFilterChange && (
            <>
              <Tooltip title="Filter by status">
                <IconButton data-testid="status-filter" onClick={(e) => handleMenuClick(e, 'status')} size="small">
                  <FilterListIcon color={isStatusFilterActive ? 'primary' : 'inherit'} />
                </IconButton>
              </Tooltip>
              <Menu anchorEl={statusAnchorEl} open={statusMenuOpen} onClose={handleMenuClose}>
                {isStatusFilterActive && (
                  <MenuItem onClick={() => handleFilterSelect('All', 'status')}>
                    <CloseIcon fontSize="small" sx={{ mr: 1 }} />
                    Clear filter
                  </MenuItem>
                )}
                {statusFilterOptions.map((option) => (
                  <MenuItem
                    key={option}
                    selected={option === currentStatusFilter}
                    onClick={() => handleFilterSelect(option, 'status')}
                  >
                    {option}
                  </MenuItem>
                ))}
              </Menu>
            </>
          )}
        </Box>
      </Box>
    </Box>
  )
}

export default CustomSearchToolbar
