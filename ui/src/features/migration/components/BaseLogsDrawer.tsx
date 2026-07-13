import React, { useState, useEffect, useRef, useCallback, useMemo } from 'react'
import Fuse from 'fuse.js'
import {
  Box,
  Typography,
  IconButton,
  Switch,
  CircularProgress,
  Alert,
  Tooltip,
  FormControl,
  MenuItem,
  Select,
  SelectChangeEvent
} from '@mui/material'
import ContentCopyIcon from '@mui/icons-material/ContentCopy'
import DownloadIcon from '@mui/icons-material/Download'
import ReplayIcon from '@mui/icons-material/Replay'
import { DrawerShell, DrawerHeader } from 'src/components'
import DarkLogLine, { LOG_BG, LOG_TS, extractLevel, normalizeLevel } from './DarkLogLine'
import { ToolbarDivider, LogsSearchField, LiveToggle, LogsMetaBar } from './LogsToolbarControls'

const LOG_LEVELS = ['ALL', 'ERROR', 'WARN', 'INFO', 'DEBUG', 'TRACE', 'SUCCESS']

export interface BaseLogsDrawerProps {
  open: boolean
  onClose: () => void
  title: string
  subtitle?: string
  logs: string[]
  isLoading: boolean
  error: string | null
  isPaused: boolean
  onPausedChange: (paused: boolean) => void
  onReconnect: () => void
  /** Custom download handler. Receives filteredLogs. If omitted, downloads filteredLogs as plain text. */
  onDownload?: (filteredLogs: string[]) => Promise<void>
  'data-testid'?: string
}

export default function BaseLogsDrawer({
  open,
  onClose,
  title,
  subtitle,
  logs,
  isLoading,
  error,
  isPaused,
  onPausedChange,
  onReconnect,
  onDownload,
  'data-testid': dataTestId
}: BaseLogsDrawerProps) {
  const [follow, setFollow] = useState(true)
  const [searchTerm, setSearchTerm] = useState('')
  const [logLevelFilter, setLogLevelFilter] = useState<string>('ALL')
  const [copySuccess, setCopySuccess] = useState(false)
  const [downloadSuccess, setDownloadSuccess] = useState(false)
  const [isDownloading, setIsDownloading] = useState(false)
  const logsEndRef = useRef<HTMLDivElement>(null)
  const logsLengthRef = useRef(0)

  useEffect(() => {
    if (follow && logsEndRef.current && logs.length > 0) {
      if (logsLengthRef.current !== logs.length) {
        logsLengthRef.current = logs.length
        setTimeout(() => {
          logsEndRef.current?.scrollIntoView({ behavior: 'smooth', block: 'end' })
        }, 0)
      }
    }
  }, [logs.length, follow])

  const handleFollowToggle = useCallback((event: React.ChangeEvent<HTMLInputElement>) => {
    const checked = event.target.checked
    setFollow(checked)
    if (checked && logsEndRef.current) {
      setTimeout(() => {
        logsEndRef.current?.scrollIntoView({ behavior: 'smooth', block: 'end' })
      }, 0)
    }
  }, [])

  const filteredLogs = useMemo(() => {
    let filtered = logs

    if (logLevelFilter !== 'ALL') {
      filtered = filtered.filter((log) => {
        const structuredMatch = new RegExp(`level=${logLevelFilter}\\b`, 'i')
        if (structuredMatch.test(log)) return true
        const cleanLog = log.replace(
          /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})\s*/,
          ''
        )
        return cleanLog.toUpperCase().startsWith(logLevelFilter)
      })
    }

    if (searchTerm.trim()) {
      const isExactSearch = searchTerm.startsWith('"') && searchTerm.endsWith('"')
      if (isExactSearch) {
        const exactTerm = searchTerm.slice(1, -1).toLowerCase()
        filtered = filtered.filter((log) => log.toLowerCase().includes(exactTerm))
      } else {
        const fuse = new Fuse(filtered, { threshold: 0.4, ignoreLocation: true, isCaseSensitive: false })
        filtered = fuse.search(searchTerm).map((r) => r.item)
      }
    }

    return filtered
  }, [logs, searchTerm, logLevelFilter])

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

  const handleCopyLogs = useCallback(() => {
    navigator.clipboard.writeText(filteredLogs.join('\n')).then(
      () => {
        setCopySuccess(true)
        setTimeout(() => setCopySuccess(false), 2000)
      },
      (err) => console.error('Failed to copy logs:', err)
    )
  }, [filteredLogs])

  const handleDownloadLogs = useCallback(async () => {
    setIsDownloading(true)
    try {
      if (onDownload) {
        await onDownload(filteredLogs)
      } else {
        const timestamp = new Date().toISOString().replace(/[:.]/g, '-')
        const blob = new Blob([filteredLogs.join('\n')], { type: 'text/plain' })
        const url = URL.createObjectURL(blob)
        const link = document.createElement('a')
        link.href = url
        link.download = `${title.toLowerCase().replace(/\s+/g, '-')}-${timestamp}.txt`
        document.body.appendChild(link)
        link.click()
        document.body.removeChild(link)
        URL.revokeObjectURL(url)
      }
      setDownloadSuccess(true)
      setTimeout(() => setDownloadSuccess(false), 2000)
    } catch (err) {
      console.error('Failed to download logs:', err)
    } finally {
      setIsDownloading(false)
    }
  }, [filteredLogs, onDownload, title])

  const handleClose = useCallback(() => {
    setFollow(true)
    setSearchTerm('')
    setLogLevelFilter('ALL')
    onClose()
  }, [onClose])

  return (
    <DrawerShell
      open={open}
      onClose={handleClose}
      requireCloseConfirmation={false}
      data-testid={dataTestId}
      header={
        <DrawerHeader
          title={title}
          subtitle={subtitle}
          onClose={handleClose}
        />
      }
    >
      <Box
        data-testid="logs-drawer-body"
        sx={{ display: 'flex', flexDirection: 'column', height: '100%' }}
      >
        <Box
          sx={{
            position: 'sticky',
            top: 0,
            zIndex: 2,
            backgroundColor: (t) => t.palette.background.paper,
            pt: 1,
            mx: -4,
            px: 4
          }}
        >
          {/* Toolbar */}
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
              overflowX: 'auto'
            }}
          >
            <LogsSearchField data-testid="logs-search-input" value={searchTerm} onChange={setSearchTerm} />

            <ToolbarDivider />

            {/* Level */}
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
                  value={logLevelFilter}
                  onChange={(e: SelectChangeEvent) => setLogLevelFilter(e.target.value)}
                  variant="outlined"
                  sx={{
                    fontSize: '0.8rem',
                    '& .MuiOutlinedInput-notchedOutline': { border: 'none' },
                    '& .MuiSelect-select': { py: 0.5, px: 1 }
                  }}
                >
                  {LOG_LEVELS.map((level) => (
                    <MenuItem key={level} value={level} sx={{ fontSize: '0.8rem' }}>
                      {level === 'ALL' ? 'All' : level}
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>
            </Box>

            <ToolbarDivider />

            <LiveToggle live={!isPaused} onToggle={() => onPausedChange(!isPaused)} />

            {/* Follow switch */}
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.25, flexShrink: 0 }}>
              <Switch
                size="small"
                checked={follow}
                onChange={handleFollowToggle}
                disabled={isPaused}
                data-testid="logs-follow-toggle"
                sx={{ mr: -0.5 }}
              />
              <Typography variant="caption" fontWeight={600} sx={{ fontSize: '0.8rem', color: 'text.secondary' }}>
                Follow
              </Typography>
            </Box>

            <ToolbarDivider />

            {/* Actions */}
            <Tooltip title={copySuccess ? 'Copied!' : 'Copy visible logs'}>
              <IconButton
                size="small"
                onClick={handleCopyLogs}
                disabled={filteredLogs.length === 0}
                sx={{ color: copySuccess ? 'success.main' : 'text.secondary' }}
              >
                <ContentCopyIcon fontSize="small" />
              </IconButton>
            </Tooltip>
            <Tooltip title={downloadSuccess ? 'Downloaded!' : isDownloading ? 'Downloading…' : 'Download logs'}>
              <span>
                <IconButton
                  data-testid="logs-download-button"
                  size="small"
                  onClick={handleDownloadLogs}
                  disabled={filteredLogs.length === 0 || isDownloading}
                  sx={{ color: downloadSuccess ? 'success.main' : 'text.secondary' }}
                >
                  {isDownloading ? <CircularProgress size={16} /> : <DownloadIcon fontSize="small" />}
                </IconButton>
              </span>
            </Tooltip>
            <Tooltip title="Reconnect">
              <IconButton size="small" onClick={onReconnect} sx={{ color: 'text.secondary' }}>
                <ReplayIcon fontSize="small" />
              </IconButton>
            </Tooltip>
          </Box>

          <LogsMetaBar
            shownCount={filteredLogs.length}
            totalCount={logs.length}
            counts={counts}
            isLoading={isLoading}
          />

          {isPaused && logs.length > 0 && (
            <Alert severity="info" sx={{ mt: 1 }}>
              Logs are paused. Showing {logs.length} lines captured before pause. Turn Live ON to
              resume streaming.
            </Alert>
          )}
        </Box>

        {error && (
          <Alert
            severity="error"
            sx={{ my: 1, border: '1px solid #30363d' }}
            action={
              <Tooltip title="Retry connection">
                <IconButton color="inherit" size="small" onClick={onReconnect}>
                  <ReplayIcon fontSize="small" />
                </IconButton>
              </Tooltip>
            }
          >
            Failed to connect to log stream: {error}
          </Alert>
        )}

        {(logs.length > 0 || isLoading || !error) && (
          <Box
            sx={{
              flex: 1,
              overflow: 'auto',
              bgcolor: LOG_BG,
              py: 0.5,
              border: '1px solid #30363d',
              borderTop: 'none',
              borderRadius: '0 0 8px 8px'
            }}
          >
            {isLoading && logs.length === 0 && (
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, p: 2 }}>
                <CircularProgress size={14} sx={{ color: '#58a6ff' }} />
                <Typography variant="caption" sx={{ color: LOG_TS, fontFamily: 'monospace' }}>
                  Connecting to log stream…
                </Typography>
              </Box>
            )}
            {!isLoading && filteredLogs.length === 0 && (
              <Box sx={{ p: 2 }}>
                <Typography variant="caption" sx={{ color: LOG_TS, fontFamily: 'monospace' }}>
                  {logs.length === 0
                    ? isPaused
                      ? 'No logs captured yet. Turn Live ON to start streaming.'
                      : 'No logs available'
                    : 'No lines match the current filters.'}
                </Typography>
              </Box>
            )}
            {filteredLogs.map((log, index) => (
              <DarkLogLine key={index} line={log} index={index} />
            ))}
            <div ref={logsEndRef} />
          </Box>
        )}
      </Box>
    </DrawerShell>
  )
}
