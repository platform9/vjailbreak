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
import { DrawerShell, DrawerHeader } from 'src/components'
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

interface ControllerLogsDrawerProps {
  open: boolean
  onClose: () => void
}

export default function ControllerLogsDrawer({ open, onClose }: ControllerLogsDrawerProps) {
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

  const { logs, isLoading, error, reconnect } = useDeploymentLogs({
    deploymentName: 'migration-controller-manager',
    namespace: 'migration-system',
    labelSelector: 'control-plane=controller-manager',
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

  const filteredLogs = useMemo(() => {
    let filtered = logs

    if (logLevelFilter !== 'ALL') {
      filtered = filtered.filter((log) => {
        if (new RegExp(`level=${logLevelFilter}\\b`, 'i').test(log)) return true
        const clean = log.replace(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})\s*/, '')
        return clean.toUpperCase().startsWith(logLevelFilter)
      })
    }

    if (searchTerm.trim()) {
      if (searchTerm.startsWith('"') && searchTerm.endsWith('"')) {
        const exact = searchTerm.slice(1, -1).toLowerCase()
        filtered = filtered.filter((l) => l.toLowerCase().includes(exact))
      } else {
        const fuse = new Fuse(filtered, { threshold: 0.4, ignoreLocation: true, isCaseSensitive: false })
        filtered = fuse.search(searchTerm).map((r) => r.item)
      }
    }

    return filtered
  }, [logs, searchTerm, logLevelFilter])

  const handleCopy = useCallback(() => {
    navigator.clipboard.writeText(filteredLogs.join('\n')).then(() => {
      setCopySuccess(true)
      setTimeout(() => setCopySuccess(false), 2000)
    })
  }, [filteredLogs])

  const handleDownload = useCallback(() => {
    const ts = new Date().toISOString().replace(/[:.]/g, '-')
    const blob = new Blob([filteredLogs.join('\n')], { type: 'text/plain' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `controller-logs-${ts}.txt`
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
    setDownloadSuccess(true)
    setTimeout(() => setDownloadSuccess(false), 2000)
  }, [filteredLogs])

  const handleReconnect = useCallback(() => {
    setSessionKey((prev) => prev + 1)
    reconnect()
  }, [reconnect])

  const handleFollowToggle = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    setFollow(e.target.checked)
    if (e.target.checked && logsEndRef.current) {
      setTimeout(() => logsEndRef.current?.scrollIntoView({ behavior: 'smooth', block: 'end' }), 0)
    }
  }, [])

  return (
    <DrawerShell
      open={open}
      onClose={onClose}
      requireCloseConfirmation={false}
      header={<DrawerHeader title="Controller Logs" onClose={onClose} />}
    >
      <Box sx={{ display: 'flex', flexDirection: 'column', height: '100%', overflow: 'hidden' }}>
        {/* Controls */}
        <Box sx={{ flexShrink: 0, pb: 1.5, borderBottom: 1, borderColor: 'divider', mb: 1.5 }}>
          <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 1 }}>
            <Box sx={{ display: 'flex', gap: 1, alignItems: 'center' }}>
              <FormControlLabel
                control={<Switch checked={!isPaused} onChange={(e) => setIsPaused(!e.target.checked)} size="small" />}
                label="Live"
              />
              <FormControlLabel
                control={<Switch checked={follow} onChange={handleFollowToggle} size="small" disabled={isPaused} />}
                label="Follow"
              />
            </Box>

            <Box sx={{ display: 'flex', gap: 0.5, alignItems: 'center' }}>
              <Tooltip title={`Filter: ${logLevelFilter}`}>
                <IconButton size="small" onClick={(e) => setFilterMenuAnchor(e.currentTarget)} color={logLevelFilter !== 'ALL' ? 'primary' : 'default'}>
                  <Badge variant="dot" color="primary" invisible={logLevelFilter === 'ALL'}>
                    <FilterListIcon fontSize="small" />
                  </Badge>
                </IconButton>
              </Tooltip>
              <Menu anchorEl={filterMenuAnchor} open={Boolean(filterMenuAnchor)} onClose={() => setFilterMenuAnchor(null)}
                anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
                transformOrigin={{ vertical: 'top', horizontal: 'right' }}
              >
                {LOG_LEVELS.map((level) => (
                  <MenuItem key={level} selected={logLevelFilter === level}
                    onClick={() => { setLogLevelFilter(level); setFilterMenuAnchor(null) }}>
                    {level === 'ALL' ? 'All Levels' : level}
                  </MenuItem>
                ))}
              </Menu>
              <Tooltip title={copySuccess ? 'Copied!' : 'Copy logs'}>
                <IconButton size="small" onClick={handleCopy} color={copySuccess ? 'success' : 'default'} disabled={filteredLogs.length === 0}>
                  <ContentCopyIcon fontSize="small" />
                </IconButton>
              </Tooltip>
              <Tooltip title={downloadSuccess ? 'Downloaded!' : 'Download logs'}>
                <IconButton size="small" onClick={handleDownload} color={downloadSuccess ? 'success' : 'default'} disabled={filteredLogs.length === 0}>
                  <DownloadIcon fontSize="small" />
                </IconButton>
              </Tooltip>
              <Chip label={`${filteredLogs.length} / ${logs.length}`} size="small" variant="outlined" />
            </Box>
          </Box>

          <TextField
            fullWidth
            size="small"
            placeholder='Search logs... (wrap in "quotes" for exact match)'
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            slotProps={{
              input: {
                startAdornment: <SearchIcon sx={{ mr: 1, color: 'text.secondary' }} fontSize="small" />,
                endAdornment: searchTerm && (
                  <IconButton size="small" onClick={() => setSearchTerm('')}>
                    <ClearIcon fontSize="small" />
                  </IconButton>
                )
              }
            }}
          />
        </Box>

        {isPaused && logs.length > 0 && (
          <Alert severity="info" sx={{ mb: 1.5, flexShrink: 0 }}>
            Logs paused. Turn Live ON to resume streaming.
          </Alert>
        )}

        {isLoading && (
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, py: 1, flexShrink: 0 }}>
            <CircularProgress size={16} />
            <Typography variant="body2" color="text.secondary">Connecting...</Typography>
          </Box>
        )}

        {error && (
          <Alert severity="error" sx={{ mb: 1.5, flexShrink: 0 }}
            action={
              <Tooltip title="Retry">
                <IconButton color="inherit" size="small" onClick={handleReconnect}>
                  <ReplayIcon fontSize="small" />
                </IconButton>
              </Tooltip>
            }
          >
            {error}
          </Alert>
        )}

        {/* Log content */}
        <Box
          sx={{
            flex: 1,
            minHeight: 0,
            overflow: 'auto',
            p: 1.5,
            border: 1,
            borderColor: isDarkMode ? DARK_DIVIDER : LIGHT_DIVIDER,
            borderRadius: 1,
            bgcolor: isDarkMode ? DARK_BG_PAPER : LIGHT_BG_PAPER,
            color: isDarkMode ? DARK_TEXT_PRIMARY : LIGHT_TEXT_PRIMARY,
            fontFamily: 'monospace',
            fontSize: '0.8125rem',
            lineHeight: 1.4
          }}
        >
          {logs.length === 0 && !isLoading && !error && (
            <Typography variant="body2" sx={{ fontFamily: 'monospace', color: isDarkMode ? DARK_TEXT_SECONDARY : LIGHT_TEXT_SECONDARY }}>
              {isPaused ? 'No logs captured. Turn Live ON to start streaming.' : 'No logs available'}
            </Typography>
          )}
          {filteredLogs.map((log, index) => (
            <Box key={index} sx={{ display: 'flex' }}>
              <Box sx={{
                minWidth: '44px', pr: 1.5, textAlign: 'right',
                color: isDarkMode ? DARK_TEXT_SECONDARY : LIGHT_TEXT_SECONDARY,
                userSelect: 'none', fontSize: '0.7rem', fontFamily: 'monospace', lineHeight: 1.6, flexShrink: 0
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
      </Box>
    </DrawerShell>
  )
}
