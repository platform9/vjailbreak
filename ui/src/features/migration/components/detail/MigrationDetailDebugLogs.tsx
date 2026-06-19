import { useCallback, useMemo, useState } from 'react'
import {
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
  FormControl,
  IconButton,
  InputAdornment,
  MenuItem,
  Paper,
  Select,
  SelectChangeEvent,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material'
import SearchIcon from '@mui/icons-material/Search'
import ClearIcon from '@mui/icons-material/Clear'
import ContentCopyIcon from '@mui/icons-material/ContentCopy'
import DownloadIcon from '@mui/icons-material/Download'
import RefreshIcon from '@mui/icons-material/Refresh'
import { Migration, Phase } from '../../api/migrations'
import { useDirectPodLogs } from 'src/hooks/useDirectPodLogs'
import { VJAILBREAK_DEFAULT_NAMESPACE } from 'src/api/constants'

const TERMINAL_PHASES: Phase[] = [Phase.Succeeded, Phase.Failed, Phase.ValidationFailed]
const LOG_LEVELS = ['ALL', 'ERROR', 'WARN', 'INFO', 'DEBUG', 'SUCCESS'] as const
type LogLevel = typeof LOG_LEVELS[number]

function extractLevel(line: string): string | null {
  const m = line.match(/\b(ERROR|FATAL|WARN|WARNING|INFO|DEBUG|TRACE|SUCCESS|SUCCEEDED|FAILED)\b/i)
  return m ? m[1].toUpperCase() : null
}

function normalizeLevel(raw: string): LogLevel | 'OTHER' {
  if (/ERROR|FATAL|FAIL/.test(raw)) return 'ERROR'
  if (/WARN/.test(raw))             return 'WARN'
  if (raw === 'INFO')               return 'INFO'
  if (raw === 'DEBUG')              return 'DEBUG'
  if (/SUCCESS|SUCCEED/.test(raw))  return 'SUCCESS'
  return 'OTHER'
}

function levelColor(level: string): string {
  switch (level) {
    case 'ERROR': case 'FATAL': case 'FAILED': return '#f44336'
    case 'WARN':  case 'WARNING':               return '#ff9800'
    case 'INFO':                                 return '#2196f3'
    case 'DEBUG':                                return '#9e9e9e'
    case 'SUCCESS': case 'SUCCEEDED':           return '#4caf50'
    default:                                     return 'inherit'
  }
}

function extractSource(line: string): string | null {
  const m = line.match(/^\d{2}:\d{2}:\d{2}[.\d]*\s+(\w[\w-]*)/)
  return m ? m[1] : null
}

interface LogLineProps {
  line: string
  index: number
}

function LogLine({ line, index }: LogLineProps) {
  const m = line.match(
    /^(\d{2}:\d{2}:\d{2}[.\d]*)\s+(\w[\w-]*)\s+(ERROR|FATAL|WARN|WARNING|INFO|DEBUG|TRACE|SUCCESS|SUCCEEDED|FAILED)\s+(.*)$/i
  )

  if (m) {
    const [, ts, source, level, msg] = m
    const lvl = level.toUpperCase()
    return (
      <Box
        sx={{
          display: 'flex',
          gap: 1,
          py: 0.1,
          fontFamily: 'monospace',
          fontSize: '0.72rem',
          lineHeight: 1.6,
          color: /ERROR|FATAL|FAIL/.test(lvl) ? 'error.main' : 'text.primary',
          '&:hover': { bgcolor: 'action.hover' },
          px: 1,
        }}
      >
        <Typography
          component="span"
          sx={{ color: 'text.disabled', fontFamily: 'monospace', fontSize: 'inherit', flexShrink: 0 }}
        >
          {String(index + 1).padStart(4, '0')}
        </Typography>
        <Typography
          component="span"
          sx={{ color: 'text.secondary', fontFamily: 'monospace', fontSize: 'inherit', flexShrink: 0 }}
        >
          {ts}
        </Typography>
        <Typography
          component="span"
          sx={{ color: 'text.secondary', fontFamily: 'monospace', fontSize: 'inherit', flexShrink: 0, minWidth: 80 }}
        >
          [{source}]
        </Typography>
        <Typography
          component="span"
          sx={{
            fontFamily: 'monospace',
            fontSize: 'inherit',
            color: levelColor(lvl),
            flexShrink: 0,
            minWidth: 60,
            fontWeight: 600,
          }}
        >
          {lvl}
        </Typography>
        <Typography
          component="span"
          sx={{ fontFamily: 'monospace', fontSize: 'inherit', flex: 1, wordBreak: 'break-all' }}
        >
          {msg}
        </Typography>
      </Box>
    )
  }

  return (
    <Box
      sx={{
        display: 'flex',
        gap: 1,
        py: 0.1,
        fontFamily: 'monospace',
        fontSize: '0.72rem',
        lineHeight: 1.6,
        px: 1,
        '&:hover': { bgcolor: 'action.hover' },
      }}
    >
      <Typography
        component="span"
        sx={{ color: 'text.disabled', fontFamily: 'monospace', fontSize: 'inherit', flexShrink: 0 }}
      >
        {String(index + 1).padStart(4, '0')}
      </Typography>
      <Typography
        component="span"
        sx={{ fontFamily: 'monospace', fontSize: 'inherit', flex: 1, wordBreak: 'break-all' }}
      >
        {line}
      </Typography>
    </Box>
  )
}

interface MigrationDetailDebugLogsProps {
  migration: Migration
}

export default function MigrationDetailDebugLogs({ migration }: MigrationDetailDebugLogsProps) {
  const [search, setSearch] = useState('')
  const [levelFilter, setLevelFilter] = useState<LogLevel>('ALL')
  const [sourceFilter, setSourceFilter] = useState('ALL')
  const [follow] = useState(true)
  const [sessionKey, setSessionKey] = useState(0)
  const [copied, setCopied] = useState(false)

  const phase = migration.status?.phase as Phase | undefined
  const podName = (migration.spec?.podRef as string | undefined) ?? ''
  const namespace =
    (migration.metadata?.namespace as string | undefined) ?? VJAILBREAK_DEFAULT_NAMESPACE

  const isTerminal = phase ? TERMINAL_PHASES.includes(phase) : false
  const useLiveFollow = !isTerminal && follow

  const { logs, isLoading, error, reconnect } = useDirectPodLogs({
    podName,
    namespace,
    enabled: !!podName,
    follow: useLiveFollow,
    sessionKey,
  })

  // Derive unique sources from log lines
  const sources = useMemo(() => {
    const s = new Set<string>(['ALL'])
    logs.forEach((line) => {
      const src = extractSource(line)
      if (src) s.add(src)
    })
    return Array.from(s)
  }, [logs])

  // Filter lines
  const filtered = useMemo(() => {
    return logs.filter((line) => {
      if (levelFilter !== 'ALL') {
        const raw = extractLevel(line)
        if (!raw || normalizeLevel(raw) !== levelFilter) return false
      }
      if (sourceFilter !== 'ALL') {
        const src = extractSource(line)
        if (src !== sourceFilter) return false
      }
      if (search.trim()) {
        if (!line.toLowerCase().includes(search.toLowerCase())) return false
      }
      return true
    })
  }, [logs, levelFilter, sourceFilter, search])

  // Level counts for badge
  const counts = useMemo(() => {
    const c = { ERROR: 0, WARN: 0, INFO: 0, DEBUG: 0 }
    logs.forEach((line) => {
      const raw = extractLevel(line)
      if (!raw) return
      const norm = normalizeLevel(raw)
      if (norm === 'ERROR') c.ERROR++
      else if (norm === 'WARN') c.WARN++
      else if (norm === 'INFO') c.INFO++
      else if (norm === 'DEBUG') c.DEBUG++
    })
    return c
  }, [logs])

  const handleCopy = useCallback(async () => {
    await navigator.clipboard.writeText(filtered.join('\n')).catch(() => undefined)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }, [filtered])

  const handleDownload = useCallback(() => {
    const blob = new Blob([filtered.join('\n')], { type: 'text/plain' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `${podName || 'migration'}-logs.txt`
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
  }, [filtered, podName])

  const handleReconnect = useCallback(() => {
    setSessionKey((k) => k + 1)
    reconnect()
  }, [reconnect])

  if (!podName) {
    return (
      <Alert severity="warning">No pod reference found for this migration. Logs unavailable.</Alert>
    )
  }

  return (
    <Box>
      {/* Controls */}
      <Box
        sx={{
          display: 'flex',
          gap: 1,
          alignItems: 'center',
          mb: 1,
          flexWrap: 'wrap',
        }}
      >
        {/* Search */}
        <TextField
          size="small"
          placeholder='Search logs…'
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          sx={{ minWidth: 220, flex: 1 }}
          InputProps={{
            startAdornment: (
              <InputAdornment position="start">
                <SearchIcon fontSize="small" />
              </InputAdornment>
            ),
            endAdornment: search ? (
              <InputAdornment position="end">
                <IconButton size="small" onClick={() => setSearch('')}>
                  <ClearIcon fontSize="small" />
                </IconButton>
              </InputAdornment>
            ) : undefined,
          }}
        />

        {/* Level filter */}
        <FormControl size="small" sx={{ minWidth: 90 }}>
          <Select
            value={levelFilter}
            onChange={(e: SelectChangeEvent) => setLevelFilter(e.target.value as LogLevel)}
            displayEmpty
          >
            {LOG_LEVELS.map((lv) => (
              <MenuItem key={lv} value={lv}>
                {lv === 'ALL' ? 'All levels' : lv}
              </MenuItem>
            ))}
          </Select>
        </FormControl>

        {/* Source filter */}
        <FormControl size="small" sx={{ minWidth: 100 }}>
          <Select
            value={sourceFilter}
            onChange={(e: SelectChangeEvent) => setSourceFilter(e.target.value)}
          >
            {sources.map((s) => (
              <MenuItem key={s} value={s}>
                {s === 'ALL' ? 'All sources' : s}
              </MenuItem>
            ))}
          </Select>
        </FormControl>

        <Box sx={{ flex: 1 }} />

        {/* Copy */}
        <Tooltip title={copied ? 'Copied!' : 'Copy visible logs'}>
          <IconButton size="small" onClick={handleCopy}>
            <ContentCopyIcon fontSize="small" />
          </IconButton>
        </Tooltip>

        {/* Download */}
        <Tooltip title="Download logs">
          <IconButton size="small" onClick={handleDownload}>
            <DownloadIcon fontSize="small" />
          </IconButton>
        </Tooltip>

        {/* Reconnect */}
        <Tooltip title="Reconnect">
          <IconButton size="small" onClick={handleReconnect}>
            <RefreshIcon fontSize="small" />
          </IconButton>
        </Tooltip>
      </Box>

      {/* Meta bar */}
      <Box sx={{ display: 'flex', gap: 1.5, alignItems: 'center', mb: 1 }}>
        <Typography variant="caption" color="text.secondary">
          {filtered.length} / {logs.length} lines
        </Typography>
        {counts.ERROR > 0 && (
          <Chip
            label={`${counts.ERROR} errors`}
            size="small"
            color="error"
            variant="outlined"
            sx={{ height: 18, fontSize: '0.65rem' }}
          />
        )}
        {counts.WARN > 0 && (
          <Chip
            label={`${counts.WARN} warn`}
            size="small"
            color="warning"
            variant="outlined"
            sx={{ height: 18, fontSize: '0.65rem' }}
          />
        )}
        {isLoading && <CircularProgress size={12} sx={{ ml: 0.5 }} />}
        {useLiveFollow && !isLoading && (
          <Typography variant="caption" color="success.main" sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
            <Box
              component="span"
              sx={{
                width: 6,
                height: 6,
                borderRadius: '50%',
                bgcolor: 'success.main',
                display: 'inline-block',
                animation: 'pulse 1.5s ease-in-out infinite',
                '@keyframes pulse': {
                  '0%, 100%': { opacity: 1 },
                  '50%': { opacity: 0.3 },
                },
              }}
            />
            Live
          </Typography>
        )}
        <Box sx={{ flex: 1 }} />
        <Typography variant="caption" color="text.disabled">
          Logs are a debug aid. Use Overview tab for status.
        </Typography>
      </Box>

      {/* Error */}
      {error && (
        <Alert
          severity="error"
          sx={{ mb: 1 }}
          action={
            <Button size="small" color="inherit" onClick={handleReconnect}>
              Reconnect
            </Button>
          }
        >
          {error}
        </Alert>
      )}

      {/* Log stream */}
      <Paper
        variant="outlined"
        sx={{
          bgcolor: 'grey.50',
          overflow: 'auto',
          maxHeight: 480,
          minHeight: 160,
          py: 0.5,
        }}
      >
        {isLoading && logs.length === 0 && (
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, p: 2 }}>
            <CircularProgress size={16} />
            <Typography variant="caption" color="text.secondary">
              Connecting to pod logs…
            </Typography>
          </Box>
        )}
        {!isLoading && filtered.length === 0 && (
          <Box sx={{ p: 2 }}>
            <Typography variant="caption" color="text.secondary">
              {logs.length === 0 ? 'No log lines received yet.' : 'No lines match the current filters.'}
            </Typography>
          </Box>
        )}
        {filtered.map((line, idx) => (
          <LogLine key={idx} line={line} index={idx} />
        ))}
        {useLiveFollow && filtered.length > 0 && (
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5, p: 1, opacity: 0.5 }}>
            <Box
              sx={{
                width: 6,
                height: 6,
                borderRadius: '50%',
                bgcolor: 'success.main',
              }}
            />
            <Typography variant="caption" color="text.secondary">
              streaming…
            </Typography>
          </Box>
        )}
      </Paper>
    </Box>
  )
}
