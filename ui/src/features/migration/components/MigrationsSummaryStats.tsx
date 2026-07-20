import { Box, Typography } from '@mui/material'
import type { MigrationStatusCategory } from '../utils/migrationTableUtils'

export interface MigrationsSummaryStatsCounts {
  inProgress: number
  awaitingAction: number
  pending: number
  succeeded: number
  failed: number
}

interface StatCardDef {
  key: MigrationStatusCategory
  label: string
  filterValue: string
  dotColor: string
}

const STAT_CARD_DEFS: StatCardDef[] = [
  { key: 'inProgress', label: 'In Progress', filterValue: 'In Progress', dotColor: 'info.main' },
  {
    key: 'awaitingAction',
    label: 'Awaiting Action',
    filterValue: 'Awaiting Action',
    dotColor: 'warning.main'
  },
  { key: 'pending', label: 'Pending', filterValue: 'Pending', dotColor: 'text.disabled' },
  { key: 'succeeded', label: 'Succeeded', filterValue: 'Succeeded', dotColor: 'success.main' },
  { key: 'failed', label: 'Failed', filterValue: 'Failed', dotColor: 'error.main' }
]

interface MigrationsSummaryStatsProps {
  counts: MigrationsSummaryStatsCounts
  activeFilter: string
  onFilterChange: (filter: string) => void
}

export default function MigrationsSummaryStats({
  counts,
  activeFilter,
  onFilterChange
}: MigrationsSummaryStatsProps) {
  return (
    <Box
      data-testid="migrations-summary-stats"
      sx={{
        display: 'grid',
        gridTemplateColumns: { xs: '1fr 1fr', sm: `repeat(${STAT_CARD_DEFS.length}, 1fr)` },
        gap: 2,
        mb: 2
      }}
    >
      {STAT_CARD_DEFS.map((def) => {
        const isActive = activeFilter === def.filterValue
        return (
          <Box
            key={def.key}
            data-testid={`migrations-summary-stat-${def.key}`}
            role="button"
            tabIndex={0}
            onClick={() => onFilterChange(isActive ? 'All' : def.filterValue)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault()
                onFilterChange(isActive ? 'All' : def.filterValue)
              }
            }}
            sx={{
              cursor: 'pointer',
              px: 2,
              py: 1.5,
              borderRadius: 1,
              border: '1px solid',
              borderColor: isActive ? 'primary.main' : 'divider',
              backgroundColor: (theme) =>
                isActive
                  ? theme.palette.mode === 'dark'
                    ? 'rgba(0, 137, 199, 0.12)'
                    : 'rgba(0, 137, 199, 0.06)'
                  : 'background.paper',
              transition: 'border-color 0.15s ease, background-color 0.15s ease',
              '&:hover': { borderColor: 'primary.main' }
            }}
          >
            <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
              <Typography
                variant="caption"
                color="text.secondary"
                sx={{ fontWeight: 600, letterSpacing: 0.6 }}
              >
                {def.label.toUpperCase()}
              </Typography>
              <Box
                sx={{
                  width: 8,
                  height: 8,
                  borderRadius: '50%',
                  backgroundColor: def.dotColor
                }}
              />
            </Box>
            <Typography variant="h4" sx={{ mt: 0.5 }}>
              {counts[def.key]}
            </Typography>
            <Typography
              variant="caption"
              color={isActive ? 'primary.main' : 'text.secondary'}
              sx={{ fontSize: '0.7rem' }}
            >
              Click to filter
            </Typography>
          </Box>
        )
      })}
    </Box>
  )
}
