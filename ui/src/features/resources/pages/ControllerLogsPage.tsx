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
import ContentCopyIcon from '@mui/icons-material/ContentCopy'
import DownloadIcon from '@mui/icons-material/Download'
import SearchIcon from '@mui/icons-material/Search'
import ClearIcon from '@mui/icons-material/Clear'
import FilterListIcon from '@mui/icons-material/FilterList'
import ReplayIcon from '@mui/icons-material/Replay'
import TerminalIcon from '@mui/icons-material/Terminal'
import { useDeploymentLogs } from 'src/hooks/useDeploymentLogs'
import LogLine from 'src/features/migration/components/LogLine'
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

const LOG_LEVELS = ['ALL', 'ERROR', 'WARN', 'INFO', 'DEBUG', 'TRACE', 'SUCCESS'] as const

export default function ControllerLogsPage() {
  const theme = useTheme()
  const isDarkMode = theme.palette.mode === 'dark'

  const [follow, setFollow] = useState(true)
  const [isPaused, setIsPaused] = useState(false)
  const [searchTerm, setSearchTerm] = useState('')
  const [logLevelFilter, setLogLevelFilter] = useState<string>('ALL')
  const [copySuccess, setCopySuccess] = useState(false)
  const [downloadSuccess, setDownloadSuccess] = useState(false)
  const [filterMenuAnchor, setFilterMenuAnchor] = useState<null | HTMLElement>(null)
  const [sessionKey, setSessionKey] = useState(0)
  const logsEndRef = useRef<HTMLDivElement>(null)
  const logsLengthRef = useRef(0)

  const {
    logs,
    isLoading,
    error,
    reconnect
  } = useDeploymentLogs({
    deploymentName: 'migration-controller-manager',
    namespace: 'migration-system',
    labelSelector: 'control-plane=controller-manager',
    enabled: !isPaused,
    sessionKey
  })

  // Auto-scroll to bottom when new logs arrive and follow is enabled
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
    []
  )

  const handleReconnect = useCallback(() => {
    setSessionKey((prev) => prev + 1)
    reconnect()
  }, [reconnect])

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

  const handleCopyLogs = useCallback(() => {
    navigator.clipboard.writeText(filteredLogs.join('\n')).then(() => {
      setCopySuccess(true)
      setTimeout(() => setCopySuccess(false), 2000)
    })
  }, [filteredLogs])

  const handleDownloadLogs = useCallback(() => {
    const timestamp = new Date().toISOString().replace(/[:.]/g, '-')
    const fileName = `controller-logs-${timestamp}.txt`
    const blob = new Blob([filteredLogs.join('\n')], { type: 'text/plain' })
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
  }, [filteredLogs])

  const controls = (
    <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 1.5 }}>
      <Box sx={{ display: 'flex', gap: 1, alignItems: 'center' }}>
        <FormControlLabel
          control={
            <Switch
              checked={!isPaused}
              onChange={(e) => setIsPaused(!e.target.checked)}
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
          {LOG_LEVELS.map((level) => (
            <MenuItem
              key={level}
              selected={logLevelFilter === level}
              onClick={() => {
                setLogLevelFilter(level)
                setFilterMenuAnchor(null)
              }}
            >
              {level === 'ALL' ? 'All Levels' : level}
            </MenuItem>
          ))}
        </Menu>

        <Tooltip title={copySuccess ? 'Copied!' : 'Copy visible logs'}>
          <IconButton
            onClick={handleCopyLogs}
            size="small"
            color={copySuccess ? 'success' : 'default'}
            disabled={filteredLogs.length === 0}
          >
            <ContentCopyIcon fontSize="small" />
          </IconButton>
        </Tooltip>

        <Tooltip title={downloadSuccess ? 'Downloaded!' : 'Download logs'}>
          <IconButton
            onClick={handleDownloadLogs}
            size="small"
            color={downloadSuccess ? 'success' : 'default'}
            disabled={filteredLogs.length === 0}
          >
            <DownloadIcon fontSize="small" />
          </IconButton>
        </Tooltip>

        <Chip
          label={`${filteredLogs.length} / ${logs.length} lines`}
          size="small"
          variant="outlined"
        />
      </Box>
    </Box>
  )

  const toolbar = (
    <Box
      sx={{
        p: 2,
        display: 'flex',
        alignItems: 'center',
        gap: 1.25,
        borderBottom: 1,
        borderColor: 'divider'
      }}
    >
      <TerminalIcon />
      <Typography variant="h6" component="h2">
        Controller Logs
      </Typography>
    </Box>
  )

  return (
    <Box sx={{ height: '100%', display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
      <Box sx={{ flexShrink: 0 }}>{toolbar}</Box>

      <Box sx={{ flex: 1, overflow: 'hidden', display: 'flex', flexDirection: 'column', p: 2 }}>
        {controls}

        <TextField
          fullWidth
          size="small"
          placeholder="Search logs..."
          value={searchTerm}
          onChange={(e) => setSearchTerm(e.target.value)}
          sx={{ mb: 1.5 }}
          slotProps={{
            input: {
              startAdornment: (
                <SearchIcon sx={{ mr: 1, color: 'text.secondary' }} fontSize="small" />
              ),
              endAdornment: searchTerm && (
                <IconButton size="small" onClick={() => setSearchTerm('')}>
                  <ClearIcon fontSize="small" />
                </IconButton>
              )
            }
          }}
        />

        {isPaused && logs.length > 0 && (
          <Alert severity="info" sx={{ mb: 1.5 }}>
            Logs are paused. Showing {logs.length} lines captured before pause. Turn Live ON to
            resume streaming.
          </Alert>
        )}

        {isLoading && (
          <Box sx={{ display: 'flex', alignItems: 'center', py: 2, gap: 1 }}>
            <CircularProgress size={20} />
            <Typography variant="body2" color="text.secondary">
              Connecting to controller log stream...
            </Typography>
          </Box>
        )}

        {error && (
          <Alert
            severity="error"
            sx={{ mb: 1.5 }}
            action={
              <Tooltip title="Retry connection">
                <IconButton color="inherit" size="small" onClick={handleReconnect}>
                  <ReplayIcon fontSize="small" />
                </IconButton>
              </Tooltip>
            }
          >
            Failed to connect to controller log stream: {error}
          </Alert>
        )}

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
              <Typography
                variant="body2"
                sx={{
                  fontFamily: 'monospace',
                  color: isDarkMode ? DARK_TEXT_SECONDARY : LIGHT_TEXT_SECONDARY
                }}
              >
                {isPaused
                  ? 'No logs captured yet. Turn Live ON to start streaming.'
                  : 'No logs available'}
              </Typography>
            )}

            {filteredLogs.map((log, index) => (
              <Box key={index} sx={{ display: 'flex' }}>
                <Box
                  sx={{
                    minWidth: '50px',
                    pr: 2,
                    py: 0.5,
                    textAlign: 'right',
                    color: isDarkMode ? DARK_TEXT_SECONDARY : LIGHT_TEXT_SECONDARY,
                    userSelect: 'none',
                    fontSize: '0.75rem',
                    fontFamily: 'monospace',
                    lineHeight: 1.6
                  }}
                >
                  {index + 1}
                </Box>
                <Box sx={{ flex: 1 }}>
                  <LogLine
                    log={log}
                    index={index}
                    showBorder={index < filteredLogs.length - 1}
                    isDarkMode={isDarkMode}
                  />
                </Box>
              </Box>
            ))}
            <div ref={logsEndRef} />
          </Box>
        </Paper>
      </Box>
    </Box>
  )
}
