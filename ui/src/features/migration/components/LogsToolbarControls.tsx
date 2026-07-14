import { Box, CircularProgress, IconButton, TextField, Tooltip, Typography } from '@mui/material'
import SearchIcon from '@mui/icons-material/Search'
import ClearIcon from '@mui/icons-material/Clear'

export function ToolbarDivider() {
  return <Box sx={{ width: '1px', height: 24, bgcolor: 'divider', flexShrink: 0 }} />
}

interface LogsSearchFieldProps {
  value: string
  onChange: (value: string) => void
  'data-testid'?: string
}

export function LogsSearchField({ value, onChange, 'data-testid': dataTestId }: LogsSearchFieldProps) {
  return (
    <TextField
      data-testid={dataTestId}
      size="small"
      placeholder='Search logs… ("exact match" for literal)'
      value={value}
      onChange={(e) => onChange(e.target.value)}
      sx={{ flex: 1, minWidth: 0 }}
      InputProps={{
        startAdornment: <SearchIcon fontSize="small" sx={{ color: 'text.disabled', mr: 1 }} />,
        endAdornment: value ? (
          <IconButton size="small" onClick={() => onChange('')}>
            <ClearIcon fontSize="small" />
          </IconButton>
        ) : undefined
      }}
    />
  )
}

interface LiveToggleProps {
  live: boolean
  onToggle: () => void
  disabled?: boolean
  disabledTooltip?: string
}

export function LiveToggle({ live, onToggle, disabled, disabledTooltip }: LiveToggleProps) {
  const tooltip = disabled
    ? disabledTooltip
    : live
      ? 'Click to pause live stream'
      : 'Click to resume live stream'

  return (
    <Tooltip title={tooltip}>
      <Box
        component="button"
        onClick={() => !disabled && onToggle()}
        sx={{
          display: 'flex',
          alignItems: 'center',
          gap: 0.5,
          cursor: disabled ? 'default' : 'pointer',
          border: 'none',
          bgcolor: 'transparent',
          p: 0.5,
          borderRadius: 1,
          '&:hover': disabled ? {} : { bgcolor: 'action.hover' }
        }}
      >
        <Box
          sx={{
            width: 8,
            height: 8,
            borderRadius: '50%',
            bgcolor: live ? '#3fb950' : 'text.disabled',
            flexShrink: 0,
            ...(live && {
              animation: 'logsLivePulse 1.5s ease-in-out infinite',
              '@keyframes logsLivePulse': {
                '0%,100%': { opacity: 1, transform: 'scale(1)' },
                '50%': { opacity: 0.5, transform: 'scale(0.85)' }
              }
            })
          }}
        />
        <Typography variant="caption" fontWeight={600} sx={{ color: live ? '#3fb950' : 'text.disabled', fontSize: '0.8rem' }}>
          Live
        </Typography>
      </Box>
    </Tooltip>
  )
}

interface LogsMetaBarProps {
  shownCount: number
  totalCount: number
  counts: { ERROR: number; WARN: number; INFO: number; DEBUG: number }
  isLoading: boolean
  note?: string
}

const COUNT_ITEMS = [
  { key: 'ERROR', label: 'errors', activeColor: '#f85149' },
  { key: 'WARN', label: 'warnings', activeColor: '#e3b341' },
  { key: 'INFO', label: 'info', activeColor: '#79c0ff' },
  { key: 'DEBUG', label: 'debug', activeColor: '#8b949e' }
] as const

export function LogsMetaBar({ shownCount, totalCount, counts, isLoading, note }: LogsMetaBarProps) {
  return (
    <Box
      sx={{
        display: 'flex',
        alignItems: 'center',
        gap: 0,
        px: 1.5,
        py: '5px',
        bgcolor: '#161b22',
        borderLeft: '1px solid #30363d',
        borderRight: '1px solid #30363d'
      }}
    >
      <Typography variant="caption" sx={{ color: '#8b949e', fontFamily: 'monospace', fontSize: '0.7rem' }}>
        {shownCount} / {totalCount} lines
      </Typography>
      <Box sx={{ mx: 1, color: '#30363d' }}>·</Box>
      {COUNT_ITEMS.map(({ key, label, activeColor }, i) => (
        <Box key={key} sx={{ display: 'flex', alignItems: 'center', gap: 0 }}>
          {i > 0 && <Box sx={{ mx: 1, color: '#21262d' }}> </Box>}
          <Typography
            variant="caption"
            sx={{ color: counts[key] > 0 ? activeColor : '#484f58', fontFamily: 'monospace', fontSize: '0.7rem' }}
          >
            {counts[key]} {label}
          </Typography>
        </Box>
      ))}
      {isLoading && <CircularProgress size={10} sx={{ ml: 1.5, color: '#58a6ff' }} />}
      {note && (
        <>
          <Box sx={{ flex: 1 }} />
          <Typography
            variant="caption"
            sx={{ color: '#484f58', fontFamily: 'monospace', fontSize: '0.65rem', fontStyle: 'italic' }}
          >
            {note}
          </Typography>
        </>
      )}
    </Box>
  )
}
