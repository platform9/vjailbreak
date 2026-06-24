import { useMemo, useState } from 'react'
import {
  Box,
  Chip,
  InputAdornment,
  TextField,
  ToggleButton,
  ToggleButtonGroup,
  Typography,
} from '@mui/material'
import CheckCircleIcon from '@mui/icons-material/CheckCircle'
import ErrorIcon from '@mui/icons-material/Error'
import RadioButtonUncheckedIcon from '@mui/icons-material/RadioButtonUnchecked'
import SearchIcon from '@mui/icons-material/Search'
import { Condition, Migration } from '../../api/migrations'

type StatusFilter = 'all' | 'success' | 'error' | 'pending'
type SortOrder = 'oldest' | 'newest'

function conditionStatus(c: Condition): 'success' | 'error' | 'pending' {
  if (c.type === 'Failed' || (c.status === 'False' && c.type !== 'Migrating')) return 'error'
  if (c.status === 'True') return 'success'
  return 'pending'
}

function formatFullTs(ts: Date | string | undefined): string {
  if (!ts) return '—'
  return new Date(ts).toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
}

interface MigrationEventsTabProps {
  migration: Migration
}

export default function MigrationEventsTab({ migration }: MigrationEventsTabProps) {
  const [filter, setFilter] = useState<StatusFilter>('all')
  const [sort, setSort] = useState<SortOrder>('oldest')
  const [search, setSearch] = useState('')

  const conditions = migration.status?.conditions ?? []

  const counts = useMemo(
    () => ({
      success: conditions.filter((c) => conditionStatus(c) === 'success').length,
      error: conditions.filter((c) => conditionStatus(c) === 'error').length,
      pending: conditions.filter((c) => conditionStatus(c) === 'pending').length,
    }),
    [conditions]
  )

  const filtered = useMemo(() => {
    let result = [...conditions].sort((a, b) => {
      const ta = a.lastTransitionTime ? new Date(a.lastTransitionTime).getTime() : 0
      const tb = b.lastTransitionTime ? new Date(b.lastTransitionTime).getTime() : 0
      return sort === 'oldest' ? ta - tb : tb - ta
    })
    if (filter !== 'all') result = result.filter((c) => conditionStatus(c) === filter)
    if (search.trim()) {
      const q = search.toLowerCase()
      result = result.filter(
        (c) =>
          String(c.type ?? '').toLowerCase().includes(q) ||
          String(c.message ?? '').toLowerCase().includes(q) ||
          String(c.reason ?? '').toLowerCase().includes(q)
      )
    }
    return result
  }, [conditions, sort, filter, search])

  if (conditions.length === 0) {
    return (
      <Box sx={{ py: 8, textAlign: 'center' }}>
        <Typography variant="body2" color="text.secondary">
          No events recorded for this migration yet.
        </Typography>
      </Box>
    )
  }

  return (
    <Box>
      {/* Toolbar */}
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, mb: 3, flexWrap: 'wrap' }}>
        <TextField
          size="small"
          placeholder="Search events…"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          slotProps={{
            input: {
              startAdornment: (
                <InputAdornment position="start">
                  <SearchIcon fontSize="small" />
                </InputAdornment>
              ),
            },
          }}
          sx={{ minWidth: 240 }}
        />

        <ToggleButtonGroup
          size="small"
          exclusive
          value={filter}
          onChange={(_, v) => {
            if (v) setFilter(v)
          }}
        >
          <ToggleButton value="all">All ({conditions.length})</ToggleButton>
          <ToggleButton value="success" sx={{ gap: 0.5 }}>
            <CheckCircleIcon sx={{ fontSize: 14, color: 'success.main' }} />
            {counts.success}
          </ToggleButton>
          <ToggleButton value="error" sx={{ gap: 0.5 }}>
            <ErrorIcon sx={{ fontSize: 14, color: 'error.main' }} />
            {counts.error}
          </ToggleButton>
          <ToggleButton value="pending" sx={{ gap: 0.5 }}>
            <RadioButtonUncheckedIcon sx={{ fontSize: 14, color: 'text.disabled' }} />
            {counts.pending}
          </ToggleButton>
        </ToggleButtonGroup>

        <ToggleButtonGroup
          size="small"
          exclusive
          value={sort}
          onChange={(_, v) => {
            if (v) setSort(v)
          }}
          sx={{ ml: 'auto' }}
        >
          <ToggleButton value="oldest">Oldest first</ToggleButton>
          <ToggleButton value="newest">Newest first</ToggleButton>
        </ToggleButtonGroup>
      </Box>

      <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 2 }}>
        {filtered.length === conditions.length
          ? `${conditions.length} events`
          : `${filtered.length} of ${conditions.length} events`}
      </Typography>

      {filtered.length === 0 ? (
        <Box sx={{ py: 4, textAlign: 'center' }}>
          <Typography variant="body2" color="text.secondary">
            No events match the current filters.
          </Typography>
        </Box>
      ) : (
        <Box sx={{ display: 'flex', flexDirection: 'column' }}>
          {filtered.map((condition, idx) => {
            const status = conditionStatus(condition)
            const isLast = idx === filtered.length - 1
            const accentColor =
              status === 'success'
                ? 'success.main'
                : status === 'error'
                  ? 'error.main'
                  : 'divider'
            const chipColor: 'success' | 'error' | 'default' =
              status === 'success' ? 'success' : status === 'error' ? 'error' : 'default'

            return (
              <Box key={idx} sx={{ display: 'flex', gap: 2.5 }}>
                {/* Timeline icon + connector */}
                <Box
                  sx={{
                    display: 'flex',
                    flexDirection: 'column',
                    alignItems: 'center',
                    flexShrink: 0,
                    pt: 1.25,
                  }}
                >
                  {status === 'error' ? (
                    <ErrorIcon sx={{ fontSize: 20, color: 'error.main' }} />
                  ) : status === 'success' ? (
                    <CheckCircleIcon sx={{ fontSize: 20, color: 'success.main' }} />
                  ) : (
                    <RadioButtonUncheckedIcon sx={{ fontSize: 20, color: 'text.disabled' }} />
                  )}
                  {!isLast && (
                    <Box
                      sx={{ width: '2px', flex: 1, bgcolor: 'divider', my: 0.5, minHeight: 20 }}
                    />
                  )}
                </Box>

                {/* Event card */}
                <Box
                  sx={{
                    flex: 1,
                    mb: isLast ? 0 : 2,
                    p: 1.75,
                    border: '1px solid',
                    borderLeft: '3px solid',
                    borderColor: 'divider',
                    borderLeftColor: accentColor,
                    borderRadius: 1.5,
                    bgcolor: 'background.paper',
                    minWidth: 0,
                  }}
                >
                  <Box
                    sx={{
                      display: 'flex',
                      alignItems: 'flex-start',
                      gap: 1,
                      flexWrap: 'wrap',
                      mb: 0.25,
                    }}
                  >
                    <Typography variant="body2" fontWeight={600} sx={{ flex: 1 }}>
                      {condition.type ?? '—'}
                    </Typography>
                    <Chip
                      label={condition.status ?? 'Unknown'}
                      size="small"
                      color={chipColor}
                      variant="outlined"
                      sx={{ height: 20, fontSize: '0.7rem', flexShrink: 0 }}
                    />
                    <Typography
                      variant="caption"
                      color="text.disabled"
                      sx={{
                        fontFamily: '"Fira Code", monospace',
                        flexShrink: 0,
                        fontSize: '0.72rem',
                      }}
                    >
                      {formatFullTs(condition.lastTransitionTime)}
                    </Typography>
                  </Box>

                  {condition.message && (
                    <Typography
                      variant="body2"
                      color="text.secondary"
                      sx={{ wordBreak: 'break-word', mt: 0.25 }}
                    >
                      {condition.message}
                    </Typography>
                  )}
                  {condition.reason && (
                    <Typography
                      variant="caption"
                      color="text.disabled"
                      sx={{ display: 'block', mt: 0.5 }}
                    >
                      Reason: {condition.reason}
                    </Typography>
                  )}
                </Box>
              </Box>
            )
          })}
        </Box>
      )}
    </Box>
  )
}
