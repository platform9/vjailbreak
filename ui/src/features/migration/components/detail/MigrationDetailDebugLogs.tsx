import { useCallback, useMemo, useRef, useEffect, useState } from 'react'
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  FormControl,
  IconButton,
  InputAdornment,
  MenuItem,
  Select,
  SelectChangeEvent,
  Switch,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material'
import SearchIcon from '@mui/icons-material/Search'
import ClearIcon from '@mui/icons-material/Clear'
import ContentCopyIcon from '@mui/icons-material/ContentCopy'
import FileDownloadOutlinedIcon from '@mui/icons-material/FileDownloadOutlined'
import SyncIcon from '@mui/icons-material/Sync'
import { Migration, Phase } from '../../api/migrations'
import { useDirectPodLogs } from 'src/hooks/useDirectPodLogs'
import { VJAILBREAK_DEFAULT_NAMESPACE } from 'src/api/constants'

const TERMINAL_PHASES: Phase[] = [Phase.Succeeded, Phase.Failed, Phase.ValidationFailed]
const LOG_LEVELS = ['ALL', 'ERROR', 'WARN', 'INFO', 'DEBUG', 'SUCCESS'] as const
type LogLevel = (typeof LOG_LEVELS)[number]

// ─── Dark log theme ───────────────────────────────────────────────────────────
const LOG_BG = '#0d1117'
const LOG_TEXT = '#c9d1d9'
const LOG_NUM = '#484f58'
const LOG_TS = '#8b949e'

// ─── Log parsing ─────────────────────────────────────────────────────────────

const LOG_RE =
  /^(\d{2}:\d{2}:\d{2}[.\d]*)\s+\[?(\w[\w-]*)\]?\s+(ERROR|FATAL|WARN|WARNING|INFO|DEBUG|TRACE|SUCCESS|SUCCEEDED|FAILED)\s+(.*)$/i

function extractLevel(line: string): string | null {
  const m = line.match(
    /\b(ERROR|FATAL|WARN|WARNING|INFO|DEBUG|TRACE|SUCCESS|SUCCEEDED|FAILED)\b/i
  )
  return m ? m[1].toUpperCase() : null
}

function normalizeLevel(raw: string): LogLevel | 'OTHER' {
  if (/ERROR|FATAL|FAIL/.test(raw)) return 'ERROR'
  if (/WARN/.test(raw)) return 'WARN'
  if (raw === 'INFO') return 'INFO'
  if (raw === 'DEBUG') return 'DEBUG'
  if (/SUCCESS|SUCCEED/.test(raw)) return 'SUCCESS'
  return 'OTHER'
}

function extractSource(line: string): string | null {
  const m = line.match(/^\d{2}:\d{2}:\d{2}[.\d]*\s+\[?(\w[\w-]*)\]?/)
  return m ? m[1] : null
}

// Level color for text (no badge, just colored text)
function levelTextColor(lvl: string): string {
  if (/ERROR|FATAL|FAIL/.test(lvl)) return '#f85149'
  if (/WARN/.test(lvl)) return '#e3b341'
  if (lvl === 'INFO') return '#79c0ff'
  if (lvl === 'DEBUG') return '#8b949e'
  if (/SUCCESS|SUCCEED/.test(lvl)) return '#3fb950'
  return '#c9d1d9'
}

// ─── Single log line ─────────────────────────────────────────────────────────

function DarkLogLine({ line, index }: { line: string; index: number }) {
  const m = line.match(LOG_RE)

  if (m) {
    const [, ts, source, level, msg] = m
    const lvl = level.toUpperCase()
    return (
      <Box
        sx={{
          display: 'flex',
          alignItems: 'baseline',
          gap: '8px',
          py: '1px',
          px: 1.5,
          fontFamily: '"Fira Code","SF Mono","Consolas",monospace',
          fontSize: '0.72rem',
          lineHeight: 1.75,
          '&:hover': { bgcolor: 'rgba(255,255,255,0.04)' },
        }}
      >
        <span
          style={{
            color: LOG_NUM,
            userSelect: 'none',
            minWidth: 28,
            textAlign: 'right',
            flexShrink: 0,
          }}
        >
          {String(index + 1).padStart(3, '0')}
        </span>
        <span style={{ color: LOG_TS, flexShrink: 0 }}>{ts}</span>
        <span style={{ color: '#79c0ff', flexShrink: 0 }}>[{source}]</span>
        <span
          style={{
            color: levelTextColor(lvl),
            fontWeight: 700,
            flexShrink: 0,
            minWidth: 52,
          }}
        >
          {lvl}
        </span>
        <span style={{ color: LOG_TEXT, wordBreak: 'break-all', flex: 1 }}>{msg}</span>
      </Box>
    )
  }

  return (
    <Box
      sx={{
        display: 'flex',
        alignItems: 'baseline',
        gap: '8px',
        py: '1px',
        px: 1.5,
        fontFamily: '"Fira Code","SF Mono","Consolas",monospace',
        fontSize: '0.72rem',
        lineHeight: 1.75,
        '&:hover': { bgcolor: 'rgba(255,255,255,0.04)' },
      }}
    >
      <span
        style={{
          color: LOG_NUM,
          userSelect: 'none',
          minWidth: 28,
          textAlign: 'right',
          flexShrink: 0,
        }}
      >
        {String(index + 1).padStart(3, '0')}
      </span>
      <span style={{ color: LOG_TEXT, wordBreak: 'break-all', flex: 1 }}>{line}</span>
    </Box>
  )
}

// ─── Main component ───────────────────────────────────────────────────────────

interface MigrationDetailDebugLogsProps {
  migration: Migration
}

export default function MigrationDetailDebugLogs({ migration }: MigrationDetailDebugLogsProps) {
  const [search, setSearch] = useState('')
  const [levelFilter, setLevelFilter] = useState<LogLevel>('ALL')
  const [sourceFilter, setSourceFilter] = useState('ALL')
  const [follow, setFollow] = useState(true)
  const [isPaused, setIsPaused] = useState(false)
  const [sessionKey, setSessionKey] = useState(0)
  const [copied, setCopied] = useState(false)
  const bottomRef = useRef<HTMLDivElement>(null)

  const phase = migration.status?.phase as Phase | undefined
  const podName = (migration.spec?.podRef as string | undefined) ?? ''
  const namespace =
    (migration.metadata?.namespace as string | undefined) ?? VJAILBREAK_DEFAULT_NAMESPACE

  const isTerminal = phase ? TERMINAL_PHASES.includes(phase) : false
  const isLive = !isTerminal && !isPaused

  const { logs, isLoading, error, reconnect } = useDirectPodLogs({
    podName,
    namespace,
    enabled: !!podName && !isPaused,
    follow: isLive,
    sessionKey,
  })

  useEffect(() => {
    if (follow && bottomRef.current && logs.length > 0) {
      bottomRef.current.scrollIntoView({ behavior: 'smooth' })
    }
  }, [logs.length, follow])

  const sources = useMemo(() => {
    const s = new Set<string>(['ALL'])
    logs.forEach((line) => {
      const src = extractSource(line)
      if (src) s.add(src)
    })
    return Array.from(s)
  }, [logs])

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
    a.download = `${podName || 'pod'}-logs.txt`
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

  // ── Inline label + select helper ──────────────────────────────────────────
  const inlineSelectSx = {
    fontSize: '0.8rem',
    '& .MuiOutlinedInput-notchedOutline': { border: 'none' },
    '& .MuiSelect-select': { py: 0.5, px: 1 },
  }

  return (
    <Box sx={{ maxWidth: '100%' }}>
      {/* ── Toolbar ──────────────────────────────────────────────────────── */}
      <Box
        sx={{
          display: 'flex',
          gap: 1,
          alignItems: 'center',
          px: 1.5,
          py: 1,
          bgcolor: 'background.paper',
          border: '1px solid',
          borderColor: 'divider',
          borderRadius: '8px 8px 0 0',
          borderBottom: 'none',
          flexWrap: 'nowrap',
          overflowX: 'auto',
        }}
      >
        {/* Search */}
        <TextField
          size="small"
          placeholder='Search logs… ("exact match" for literal)'
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          sx={{ flex: 1, minWidth: 0 }}
          InputProps={{
            startAdornment: (
              <InputAdornment position="start">
                <SearchIcon fontSize="small" sx={{ color: 'text.disabled' }} />
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

        {/* Divider */}
        <Box sx={{ width: '1px', height: 24, bgcolor: 'divider', flexShrink: 0 }} />

        {/* LEVEL */}
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.25, flexShrink: 0 }}>
          <Typography
            variant="caption"
            fontWeight={700}
            sx={{ textTransform: 'uppercase', letterSpacing: 0.6, fontSize: '0.65rem', color: 'text.secondary' }}
          >
            Level
          </Typography>
          <FormControl size="small">
            <Select
              value={levelFilter}
              onChange={(e: SelectChangeEvent) => setLevelFilter(e.target.value as LogLevel)}
              variant="outlined"
              sx={inlineSelectSx}
            >
              {LOG_LEVELS.map((lv) => (
                <MenuItem key={lv} value={lv} sx={{ fontSize: '0.8rem' }}>
                  {lv === 'ALL' ? 'All' : lv}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
        </Box>

        {/* Divider */}
        <Box sx={{ width: '1px', height: 24, bgcolor: 'divider', flexShrink: 0 }} />

        {/* SOURCE */}
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.25, flexShrink: 0 }}>
          <Typography
            variant="caption"
            fontWeight={700}
            sx={{ textTransform: 'uppercase', letterSpacing: 0.6, fontSize: '0.65rem', color: 'text.secondary' }}
          >
            Source
          </Typography>
          <FormControl size="small">
            <Select
              value={sourceFilter}
              onChange={(e: SelectChangeEvent) => setSourceFilter(e.target.value)}
              variant="outlined"
              sx={inlineSelectSx}
            >
              {sources.map((s) => (
                <MenuItem key={s} value={s} sx={{ fontSize: '0.8rem' }}>
                  {s === 'ALL' ? 'All sources' : s}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
        </Box>

        {/* Divider */}
        <Box sx={{ width: '1px', height: 24, bgcolor: 'divider', flexShrink: 0 }} />

        {/* Live — clickable toggle */}
        <Tooltip title={isTerminal ? 'Migration ended — no live stream' : isPaused ? 'Click to resume live stream' : 'Click to pause live stream'}>
          <Box
            component="button"
            onClick={() => !isTerminal && setIsPaused((p) => !p)}
            sx={{
              display: 'flex',
              alignItems: 'center',
              gap: 0.5,
              cursor: isTerminal ? 'default' : 'pointer',
              border: 'none',
              bgcolor: 'transparent',
              p: 0.5,
              borderRadius: 1,
              '&:hover': !isTerminal ? { bgcolor: 'action.hover' } : {},
            }}
          >
            <Box
              sx={{
                width: 8,
                height: 8,
                borderRadius: '50%',
                bgcolor: isLive ? '#3fb950' : 'text.disabled',
                flexShrink: 0,
                ...(isLive && {
                  animation: 'livePulse 1.5s ease-in-out infinite',
                  '@keyframes livePulse': {
                    '0%,100%': { opacity: 1, transform: 'scale(1)' },
                    '50%': { opacity: 0.5, transform: 'scale(0.85)' },
                  },
                }),
              }}
            />
            <Typography
              variant="caption"
              fontWeight={600}
              sx={{ color: isLive ? '#3fb950' : 'text.disabled', fontSize: '0.8rem' }}
            >
              Live
            </Typography>
          </Box>
        </Tooltip>

        {/* Follow switch */}
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.25, flexShrink: 0 }}>
          <Switch
            size="small"
            checked={follow}
            onChange={(e) => setFollow(e.target.checked)}
            sx={{ mr: -0.5 }}
          />
          <Typography variant="caption" fontWeight={600} sx={{ fontSize: '0.8rem', color: 'text.secondary' }}>
            Follow
          </Typography>
        </Box>

        {/* Divider */}
        <Box sx={{ width: '1px', height: 24, bgcolor: 'divider', flexShrink: 0 }} />

        {/* Actions */}
        <Tooltip title={copied ? 'Copied!' : 'Copy visible logs'}>
          <IconButton size="small" onClick={handleCopy} sx={{ color: 'text.secondary' }}>
            <ContentCopyIcon sx={{ fontSize: 17 }} />
          </IconButton>
        </Tooltip>
        <Tooltip title="Download logs">
          <IconButton size="small" onClick={handleDownload} sx={{ color: 'text.secondary' }}>
            <FileDownloadOutlinedIcon sx={{ fontSize: 18 }} />
          </IconButton>
        </Tooltip>
        <Tooltip title="Reconnect">
          <IconButton size="small" onClick={handleReconnect} sx={{ color: 'text.secondary' }}>
            <SyncIcon sx={{ fontSize: 17 }} />
          </IconButton>
        </Tooltip>
      </Box>

      {/* ── Meta bar ─────────────────────────────────────────────────────── */}
      <Box
        sx={{
          display: 'flex',
          alignItems: 'center',
          gap: 0,
          px: 1.5,
          py: '5px',
          bgcolor: '#161b22',
          borderLeft: '1px solid #30363d',
          borderRight: '1px solid #30363d',
        }}
      >
        <Typography
          variant="caption"
          sx={{ color: '#8b949e', fontFamily: 'monospace', fontSize: '0.7rem' }}
        >
          {filtered.length} / {logs.length} lines
        </Typography>
        <Box sx={{ mx: 1, color: '#30363d' }}>·</Box>
        {[
          { label: 'errors', count: counts.ERROR, activeColor: '#f85149' },
          { label: 'warnings', count: counts.WARN, activeColor: '#e3b341' },
          { label: 'info', count: counts.INFO, activeColor: '#79c0ff' },
          { label: 'debug', count: counts.DEBUG, activeColor: '#8b949e' },
        ].map(({ label, count, activeColor }, i) => (
          <Box key={label} sx={{ display: 'flex', alignItems: 'center', gap: 0 }}>
            {i > 0 && <Box sx={{ mx: 1, color: '#21262d' }}> </Box>}
            <Typography
              variant="caption"
              sx={{
                color: count > 0 ? activeColor : '#484f58',
                fontFamily: 'monospace',
                fontSize: '0.7rem',
              }}
            >
              {count} {label}
            </Typography>
          </Box>
        ))}
        {isLoading && <CircularProgress size={10} sx={{ ml: 1.5, color: '#58a6ff' }} />}
        <Box sx={{ flex: 1 }} />
        <Typography
          variant="caption"
          sx={{ color: '#484f58', fontFamily: 'monospace', fontSize: '0.65rem', fontStyle: 'italic' }}
        >
          Logs are a debug aid. Use Overview tab for status.
        </Typography>
      </Box>

      {/* Error */}
      {error && (
        <Alert
          severity="error"
          sx={{ borderRadius: 0, border: '1px solid #30363d', borderTop: 'none' }}
          action={
            <Button size="small" color="inherit" onClick={handleReconnect}>
              Reconnect
            </Button>
          }
        >
          {error}
        </Alert>
      )}

      {/* ── Log stream ────────────────────────────────────────────────────── */}
      <Box
        sx={{
          bgcolor: LOG_BG,
          overflow: 'auto',
          maxHeight: 540,
          minHeight: 200,
          py: 0.5,
          border: '1px solid #30363d',
          borderTop: 'none',
          borderRadius: '0 0 8px 8px',
        }}
      >
        {isLoading && logs.length === 0 && (
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, p: 2 }}>
            <CircularProgress size={14} sx={{ color: '#58a6ff' }} />
            <Typography variant="caption" sx={{ color: LOG_TS, fontFamily: 'monospace' }}>
              Connecting to pod logs…
            </Typography>
          </Box>
        )}
        {!isLoading && filtered.length === 0 && (
          <Box sx={{ p: 2 }}>
            <Typography variant="caption" sx={{ color: LOG_TS, fontFamily: 'monospace' }}>
              {logs.length === 0
                ? 'No log lines received yet.'
                : 'No lines match the current filters.'}
            </Typography>
          </Box>
        )}
        {filtered.map((line, idx) => (
          <DarkLogLine key={idx} line={line} index={idx} />
        ))}
        {isLive && filtered.length > 0 && (
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5, px: 1.5, py: 0.5 }}>
            <Box sx={{ width: 5, height: 5, borderRadius: '50%', bgcolor: '#3fb950', opacity: 0.6 }} />
            <Typography
              variant="caption"
              sx={{ color: '#484f58', fontFamily: 'monospace', fontSize: '0.65rem' }}
            >
              streaming…
            </Typography>
          </Box>
        )}
        <div ref={bottomRef} />
      </Box>
    </Box>
  )
}
