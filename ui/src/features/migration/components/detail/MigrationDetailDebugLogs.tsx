import { useCallback, useMemo, useRef, useEffect, useState } from 'react'
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  FormControl,
  IconButton,
  MenuItem,
  Select,
  SelectChangeEvent,
  Snackbar,
  Switch,
  Tooltip,
  Typography,
} from '@mui/material'
import ContentCopyIcon from '@mui/icons-material/ContentCopy'
import FileDownloadOutlinedIcon from '@mui/icons-material/FileDownloadOutlined'
import SyncIcon from '@mui/icons-material/Sync'
import { Migration, Phase } from '../../api/migrations'
import { useDirectPodLogs } from 'src/hooks/useDirectPodLogs'
import { VJAILBREAK_DEFAULT_NAMESPACE } from 'src/api/constants'
import { downloadDebugBundle } from 'src/api/migrations/debugBundle'
import { useToast } from 'src/features/migration/hooks/useToast'
import DarkLogLine, {
  LOG_BG,
  LOG_TS,
  extractLevel,
  normalizeLevel,
  extractSource
} from '../DarkLogLine'
import { ToolbarDivider, LogsSearchField, LiveToggle, LogsMetaBar } from '../LogsToolbarControls'

const TERMINAL_PHASES: Phase[] = [Phase.Succeeded, Phase.Failed, Phase.ValidationFailed]
const LOG_LEVELS = ['ALL', 'ERROR', 'WARN', 'INFO', 'DEBUG', 'SUCCESS'] as const
type LogLevel = (typeof LOG_LEVELS)[number]

// ─── Main component ───────────────────────────────────────────────────────────

interface MigrationDetailDebugLogsProps {
  migration: Migration
}

export default function MigrationDetailDebugLogs({ migration }: MigrationDetailDebugLogsProps) {
  const { toastOpen, toastMessage, toastSeverity, showToast, handleCloseToast } = useToast()
  const [search, setSearch] = useState('')
  const [levelFilter, setLevelFilter] = useState<LogLevel>('ALL')
  const [sourceFilter, setSourceFilter] = useState('ALL')
  const [follow, setFollow] = useState(true)
  const [isPaused, setIsPaused] = useState(false)
  const [sessionKey, setSessionKey] = useState(0)
  const [copied, setCopied] = useState(false)
  const [isDownloading, setIsDownloading] = useState(false)
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

  const handleDownload = useCallback(async () => {
    setIsDownloading(true)
    try {
      await downloadDebugBundle(migration.metadata?.name ?? '', namespace)
    } catch (err) {
      console.error('Debug bundle download failed:', err)
      showToast('Failed to download debug bundle. The bundle may be too large.', 'error')
    } finally {
      setIsDownloading(false)
    }
  }, [migration.metadata?.name, namespace, showToast])

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
        <LogsSearchField value={search} onChange={setSearch} />

        <ToolbarDivider />

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

        <ToolbarDivider />

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

        <ToolbarDivider />

        <LiveToggle
          live={isLive}
          onToggle={() => setIsPaused((p) => !p)}
          disabled={isTerminal}
          disabledTooltip="Migration ended — no live stream"
        />

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

        <ToolbarDivider />

        {/* Actions */}
        <Tooltip title={copied ? 'Copied!' : 'Copy visible logs'}>
          <IconButton size="small" onClick={handleCopy} sx={{ color: 'text.secondary' }}>
            <ContentCopyIcon sx={{ fontSize: 17 }} />
          </IconButton>
        </Tooltip>
        <Tooltip title={isDownloading ? 'Downloading…' : 'Download debug bundle'}>
          <span>
            <IconButton
              size="small"
              onClick={handleDownload}
              disabled={isDownloading}
              sx={{ color: 'text.secondary' }}
            >
              {isDownloading
                ? <CircularProgress size={18} sx={{ color: 'text.secondary' }} />
                : <FileDownloadOutlinedIcon sx={{ fontSize: 18 }} />}
            </IconButton>
          </span>
        </Tooltip>
        <Tooltip title="Reconnect">
          <IconButton size="small" onClick={handleReconnect} sx={{ color: 'text.secondary' }}>
            <SyncIcon sx={{ fontSize: 17 }} />
          </IconButton>
        </Tooltip>
      </Box>

      <LogsMetaBar
        shownCount={filtered.length}
        totalCount={logs.length}
        counts={counts}
        isLoading={isLoading}
        note="Logs are a debug aid. Use Overview tab for status."
      />

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

      <Snackbar open={toastOpen} autoHideDuration={6000} onClose={handleCloseToast} anchorOrigin={{ vertical: 'top', horizontal: 'right' }}>
        <Alert onClose={handleCloseToast} severity={toastSeverity} sx={{ width: '100%' }}>
          {toastMessage}
        </Alert>
      </Snackbar>
    </Box>
  )
}
