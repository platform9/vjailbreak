import React, { useState, useEffect, useRef, useCallback, useMemo } from 'react'
import Fuse from 'fuse.js'
import {
  Box,
  Typography,
  IconButton,
  Switch,
  FormControlLabel,
  CircularProgress,
  Alert,
  Paper,
  useTheme,
  TextField,
  Tooltip,
  Chip,
  MenuItem,
  Menu,
  Badge
} from '@mui/material'
import CloseIcon from '@mui/icons-material/Close'
import ContentCopyIcon from '@mui/icons-material/ContentCopy'
import DownloadIcon from '@mui/icons-material/Download'
import SearchIcon from '@mui/icons-material/Search'
import ClearIcon from '@mui/icons-material/Clear'
import FilterListIcon from '@mui/icons-material/FilterList'
import ReplayIcon from '@mui/icons-material/Replay'
import { DrawerShell, DrawerHeader } from 'src/components'
import { useDirectPodLogs } from 'src/hooks/useDirectPodLogs'
import { fetchPodDebugLogs } from 'src/api/kubernetes/pods'
import LogLine from './LogLine'
import {
  DARK_BG_PAPER,
  DARK_TEXT_PRIMARY,
  DARK_DIVIDER,
  LIGHT_BG_PAPER,
  LIGHT_TEXT_PRIMARY,
  LIGHT_DIVIDER,
  DARK_TEXT_SECONDARY,
  LIGHT_TEXT_SECONDARY
} from 'src/theme/colors'

interface LogsDrawerProps {
  open: boolean
  onClose: () => void
  podName: string
  namespace: string
  migrationName?: string
}

export default function LogsDrawer({
  open,
  onClose,
  podName,
  namespace,
  migrationName
}: LogsDrawerProps) {
  const theme = useTheme()
  const isDarkMode = theme.palette.mode === 'dark'

  const [follow, setFollow] = useState(true)
  const [isPaused, setIsPaused] = useState(false)
  const [searchTerm, setSearchTerm] = useState('')
  const [logLevelFilter, setLogLevelFilter] = useState<string>('ALL')
  const [copySuccess, setCopySuccess] = useState(false)
  const [downloadSuccess, setDownloadSuccess] = useState(false)
  const [isDownloading, setIsDownloading] = useState(false)
  const [filterMenuAnchor, setFilterMenuAnchor] = useState<null | HTMLElement>(null)
  const [sessionKey, setSessionKey] = useState(0)
  const logsEndRef = useRef<HTMLDivElement>(null)
  const logsContainerRef = useRef<HTMLDivElement>(null)
  const logsLengthRef = useRef(0)

  const {
    logs,
    isLoading,
    error
  } = useDirectPodLogs({
    podName,
    namespace,
    enabled: open && !isPaused,
    sessionKey
  })

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

  const handleFollowToggle = useCallback(
    (event: React.ChangeEvent<HTMLInputElement>) => {
      const checked = event.target.checked
      setFollow(checked)
      if (checked && logsEndRef.current) {
        setTimeout(() => {
          logsEndRef.current?.scrollIntoView({ behavior: 'smooth', block: 'end' })
        }, 0)
      }
    },
    [logsEndRef]
  )

  const handleReconnect = useCallback(() => {
    setSessionKey((prev) => prev + 1)
  }, [])

  const handleClose = useCallback(() => {
    setFollow(true)
    setIsPaused(false)
    setSearchTerm('')
    setLogLevelFilter('ALL')
    onClose()
  }, [onClose])

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
        filtered = fuse.search(searchTerm).map((result) => result.item)
      }
    }

    return filtered
  }, [logs, searchTerm, logLevelFilter])

  const vmDisplayName = useMemo(() => {
    const fromMigration = (() => {
      if (!migrationName) return null
      const withoutPrefix = migrationName.replace(/^migration-/, '')
      const withoutSuffix = withoutPrefix.replace(/-[0-9a-f]{5}$/i, '')
      return withoutSuffix || null
    })()

    if (fromMigration) return fromMigration

    if (!podName) return null
    const withoutPrefix = podName.replace(/^v2v-helper-/, '')
    const parts = withoutPrefix.split('-')
    if (parts.length >= 4) return parts.slice(0, -3).join('-') || withoutPrefix
    if (parts.length >= 3) return parts.slice(0, -2).join('-') || withoutPrefix
    return withoutPrefix
  }, [migrationName, podName])

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
      const timestamp = new Date().toISOString().replace(/[:.]/g, '-')
      const fileName = `${vmDisplayName || podName || 'logs'}-${timestamp}.txt`

      let combinedLogs = '='.repeat(80) + '\nSTDOUT/STDERR LOGS\n' + '='.repeat(80) + '\n\n'
      combinedLogs += filteredLogs.join('\n')

      if (podName && namespace) {
        try {
          const debugLogs = await fetchPodDebugLogs(namespace, podName, migrationName)
          if (debugLogs && debugLogs.trim()) {
            combinedLogs += '\n\n' + '='.repeat(80) + '\nDEBUG LOGS FROM /var/log/pf9\n' + '='.repeat(80) + '\n\n'
            combinedLogs += debugLogs
          }
        } catch {
          combinedLogs += '\n\n' + '='.repeat(80) + '\nDEBUG LOGS FROM /var/log/pf9\n' + '='.repeat(80) + '\n\n'
          combinedLogs += '[Failed to fetch debug logs from pod filesystem]\n'
        }
      }

      const blob = new Blob([combinedLogs], { type: 'text/plain' })
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = fileName
      document.body.appendChild(link)
      link.click()
      document.body.removeChild(link)
      URL.revokeObjectURL(url)
      setDownloadSuccess(true)
      setTimeout(() => setDownloadSuccess(false), 2000)
    } catch (err) {
      console.error('Failed to download logs:', err)
    } finally {
      setIsDownloading(false)
    }
  }, [filteredLogs, vmDisplayName, podName, namespace, migrationName])

  return (
    <DrawerShell
      open={open}
      onClose={handleClose}
      requireCloseConfirmation={false}
      header={
        <DrawerHeader
          title="Migration Pod Logs"
          subtitle={vmDisplayName || ''}
          onClose={handleClose}
          icon={<CloseIcon sx={{ display: 'none' }} />}
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
          {/* Controls */}
          <Box
            sx={{
              display: 'flex',
              flexDirection: 'column',
              gap: 1.5,
              mb: 2,
              pb: 2,
              borderBottom: 1,
              borderColor: 'divider'
            }}
          >
            <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <Box sx={{ display: 'flex', gap: 1, alignItems: 'center' }}>
                <FormControlLabel
                  control={
                    <Switch
                      checked={!isPaused}
                      onChange={(e) => setIsPaused(!e.target.checked)}
                      name="streaming"
                      size="small"
                    />
                  }
                  label="Live"
                />
                <FormControlLabel
                  control={
                    <Switch
                      checked={follow}
                      onChange={handleFollowToggle}
                      name="follow"
                      size="small"
                      disabled={isPaused}
                    />
                  }
                  label="Follow"
                />
              </Box>

              <Box sx={{ display: 'flex', gap: 1, alignItems: 'center' }}>
                <Tooltip title={`Filter by level: ${logLevelFilter}`}>
                  <IconButton
                    onClick={(e) => setFilterMenuAnchor(e.currentTarget)}
                    size="small"
                    color={logLevelFilter !== 'ALL' ? 'primary' : 'default'}
                  >
                    <Badge variant="dot" color="primary" invisible={logLevelFilter === 'ALL'}>
                      <FilterListIcon fontSize="small" />
                    </Badge>
                  </IconButton>
                </Tooltip>

                <Menu
                  anchorEl={filterMenuAnchor}
                  open={Boolean(filterMenuAnchor)}
                  onClose={() => setFilterMenuAnchor(null)}
                  anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
                  transformOrigin={{ vertical: 'top', horizontal: 'right' }}
                >
                  {['ALL', 'ERROR', 'WARN', 'INFO', 'DEBUG', 'TRACE', 'SUCCESS'].map((level) => (
                    <MenuItem key={level} selected={logLevelFilter === level}
                      onClick={() => { setLogLevelFilter(level); setFilterMenuAnchor(null) }}>
                      {level === 'ALL' ? 'All Levels' : level}
                    </MenuItem>
                  ))}
                </Menu>

                <Tooltip title={copySuccess ? 'Copied!' : 'Copy visible logs'}>
                  <IconButton onClick={handleCopyLogs} size="small" color={copySuccess ? 'success' : 'default'} disabled={filteredLogs.length === 0}>
                    <ContentCopyIcon fontSize="small" />
                  </IconButton>
                </Tooltip>

                <Tooltip title={downloadSuccess ? 'Downloaded!' : isDownloading ? 'Downloading...' : 'Download logs'}>
                  <IconButton onClick={handleDownloadLogs} size="small" color={downloadSuccess ? 'success' : 'default'} disabled={filteredLogs.length === 0 || isDownloading}>
                    {isDownloading ? <CircularProgress size={16} /> : <DownloadIcon fontSize="small" />}
                  </IconButton>
                </Tooltip>

                <Chip label={`${filteredLogs.length} / ${logs.length} lines`} size="small" variant="outlined" />
              </Box>
            </Box>

            <TextField
              fullWidth
              size="small"
              placeholder="Search logs..."
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              InputProps={{
                startAdornment: <SearchIcon sx={{ mr: 1, color: 'text.secondary' }} fontSize="small" />,
                endAdornment: searchTerm && (
                  <IconButton size="small" onClick={() => setSearchTerm('')}>
                    <ClearIcon fontSize="small" />
                  </IconButton>
                )
              }}
            />
          </Box>

          {isPaused && logs.length > 0 && (
            <Alert severity="info" sx={{ mb: 2 }}>
              Logs are paused. Showing {logs.length} lines captured before pause. Turn Live ON to resume streaming.
            </Alert>
          )}
        </Box>

        {isLoading && (
          <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'center', py: 4 }}>
            <CircularProgress size={24} sx={{ mr: 2 }} />
            <Typography variant="body2" color="text.secondary">Connecting to pod log stream...</Typography>
          </Box>
        )}

        {error && (
          <Alert severity="error" sx={{ mb: 2 }}
            action={
              <Tooltip title="Retry connection">
                <IconButton color="inherit" size="small" onClick={handleReconnect}>
                  <ReplayIcon fontSize="small" />
                </IconButton>
              </Tooltip>
            }
          >
            Failed to connect to pod log stream: {error}
          </Alert>
        )}

        {(logs.length > 0 || isLoading || !error) && (
          <Paper
            variant="outlined"
            sx={{
              flex: 1,
              overflow: 'hidden',
              display: 'flex',
              flexDirection: 'column',
              backgroundColor: isDarkMode ? DARK_BG_PAPER : LIGHT_BG_PAPER,
              borderColor: isDarkMode ? DARK_DIVIDER : LIGHT_DIVIDER
            }}
          >
            <Box
              ref={logsContainerRef}
              sx={{
                flex: 1,
                overflow: 'auto',
                p: 2,
                backgroundColor: isDarkMode ? DARK_BG_PAPER : LIGHT_BG_PAPER,
                color: isDarkMode ? DARK_TEXT_PRIMARY : LIGHT_TEXT_PRIMARY,
                fontFamily: 'monospace',
                fontSize: '0.875rem',
                lineHeight: 1.4,
                whiteSpace: 'pre-wrap',
                wordBreak: 'break-word'
              }}
            >
              {logs.length === 0 && !isLoading && !error && (
                <Typography variant="body2" sx={{ fontFamily: 'monospace', color: isDarkMode ? DARK_TEXT_SECONDARY : LIGHT_TEXT_SECONDARY }}>
                  {isPaused ? 'No logs captured yet. Turn Live ON to start streaming.' : 'No logs available'}
                </Typography>
              )}
              {filteredLogs.map((log, index) => (
                <Box key={index} sx={{ display: 'flex' }}>
                  <Box sx={{
                    minWidth: '50px', pr: 2, py: 0.5, textAlign: 'right',
                    color: isDarkMode ? DARK_TEXT_SECONDARY : LIGHT_TEXT_SECONDARY,
                    userSelect: 'none', fontSize: '0.75rem', fontFamily: 'monospace', lineHeight: 1.6
                  }}>
                    {index + 1}
                  </Box>
                  <Box sx={{ flex: 1 }}>
                    <LogLine log={log} index={index} showBorder={index < filteredLogs.length - 1} isDarkMode={isDarkMode} />
                  </Box>
                </Box>
              ))}
              <div ref={logsEndRef} />
            </Box>
          </Paper>
        )}
      </Box>
    </DrawerShell>
  )
}
