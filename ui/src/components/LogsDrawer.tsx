import React, { useState, useEffect, useRef, useCallback, useMemo } from 'react'
import {
  Box,
  Typography,
  IconButton,
  Switch,
  FormControlLabel,
  CircularProgress,
  Alert,
  Paper,
  ToggleButton,
  ToggleButtonGroup,
  useTheme,
  TextField,
  Tooltip,
  Chip
} from '@mui/material'
import CloseIcon from '@mui/icons-material/Close'
import ContentCopyIcon from '@mui/icons-material/ContentCopy'
import SearchIcon from '@mui/icons-material/Search'
import ClearIcon from '@mui/icons-material/Clear'
import { StyledDrawer, DrawerContent } from 'src/components/forms/StyledDrawer'
import { useDirectPodLogs } from 'src/hooks/useDirectPodLogs'
import { useDeploymentLogs } from 'src/hooks/useDeploymentLogs'
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
  const [logSource, setLogSource] = useState<'pod' | 'controller'>('pod')
  const [isTransitioning, setIsTransitioning] = useState(false)
  const [searchTerm, setSearchTerm] = useState('')
  const [copySuccess, setCopySuccess] = useState(false)
  const logsEndRef = useRef<HTMLDivElement>(null)
  const logsContainerRef = useRef<HTMLDivElement>(null)

  const {
    logs: directPodLogs,
    isLoading: directPodIsLoading,
    error: directPodError,
    reconnect: directPodReconnect
  } = useDirectPodLogs({
    podName,
    namespace,
    enabled: open && logSource === 'pod' && !isPaused
  })

  const controllerLogs = useDeploymentLogs({
    deploymentName: 'migration-controller-manager',
    namespace: 'migration-system',
    labelSelector: 'control-plane=controller-manager',
    enabled: open && logSource === 'controller' && !isPaused
  })

  // Get current logs and states based on log source
  const currentLogs = logSource === 'pod' ? directPodLogs : controllerLogs.logs
  const currentIsLoading = logSource === 'pod' ? directPodIsLoading : controllerLogs.isLoading
  const currentError = logSource === 'pod' ? directPodError : controllerLogs.error
  const currentReconnect = logSource === 'pod' ? directPodReconnect : controllerLogs.reconnect

  // Auto-scroll to bottom when new logs arrive and follow is enabled
  useEffect(() => {
    if (follow && logsEndRef.current && !isTransitioning && currentLogs.length > 0) {
      setTimeout(() => {
        logsEndRef.current?.scrollIntoView({ behavior: 'smooth', block: 'end' })
      }, 0)
    }
  }, [currentLogs.length, follow, isTransitioning])

  const handleFollowToggle = useCallback((event: React.ChangeEvent<HTMLInputElement>) => {
    const checked = event.target.checked
    setFollow(checked)
    // If enabling follow, scroll to bottom immediately
    if (checked && logsEndRef.current) {
      setTimeout(() => {
        logsEndRef.current?.scrollIntoView({ behavior: 'smooth', block: 'end' })
      }, 0)
    }
  }, [])

  const handleLogSourceChange = useCallback(
    (_event: React.MouseEvent<HTMLElement>, newLogSource: 'pod' | 'controller' | null) => {
      if (newLogSource !== null && newLogSource !== logSource) {
        setIsTransitioning(true)
        setLogSource(newLogSource)

        // Reset transition state after a brief delay to show loading state
        setTimeout(() => {
          setIsTransitioning(false)
        }, 100)
      }
    },
    [logSource]
  )

  const handleClose = useCallback(() => {
    setFollow(true) // Reset follow state when closing
    setIsPaused(false) // Reset pause state when closing
    setLogSource('pod') // Reset log source when closing
    setIsTransitioning(false) // Reset transition state when closing
    setSearchTerm('') // Reset search term
    onClose()
  }, [onClose])

  // Filter logs based on search term
  const filteredLogs = useMemo(() => {
    if (!searchTerm.trim()) return currentLogs
    const searchLower = searchTerm.toLowerCase()
    return currentLogs.filter((log) => log.toLowerCase().includes(searchLower))
  }, [currentLogs, searchTerm])

  // Copy filtered logs to clipboard
  const handleCopyLogs = useCallback(() => {
    const logsText = filteredLogs.join('\n')
    navigator.clipboard.writeText(logsText).then(
      () => {
        setCopySuccess(true)
        setTimeout(() => setCopySuccess(false), 2000)
      },
      (err) => {
        console.error('Failed to copy logs:', err)
      }
    )
  }, [filteredLogs])

  return (
    <StyledDrawer anchor="right" open={open} onClose={handleClose}>
      {/* Header */}
      <Box
        sx={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          px: 3,
          py: 2,
          borderBottom: 1,
          borderColor: 'divider',
          backgroundColor: 'background.paper'
        }}
      >
        <Box>
          <Typography variant="h6" component="h2">
            {logSource === 'pod' ? 'Migration Pod Logs' : 'Controller Logs'}
          </Typography>
          <Typography variant="body2" color="text.secondary">
            {logSource === 'pod'
              ? migrationName
                ? migrationName.replace(/^(migration-|basic-migration-)/, '')
                : null
              : 'migration-controller-manager'}
          </Typography>
        </Box>
        <IconButton onClick={handleClose} aria-label="close logs drawer" size="small">
          <CloseIcon />
        </IconButton>
      </Box>

      <DrawerContent>
        <Box sx={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
          {/* Log Source Toggle */}
          <Box
            sx={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              mb: 2,
              pb: 1,
              borderBottom: 1,
              borderColor: 'divider'
            }}
          >
            <ToggleButtonGroup
              value={logSource}
              exclusive
              onChange={handleLogSourceChange}
              aria-label="log source"
              size="small"
            >
              <ToggleButton value="pod" aria-label="migration pod logs">
                Migration
              </ToggleButton>
              <ToggleButton value="controller" aria-label="controller logs">
                Controller
              </ToggleButton>
            </ToggleButtonGroup>
          </Box>

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
                {currentError && (
                  <Tooltip title="Reconnect to log stream">
                    <IconButton
                      onClick={currentReconnect}
                      size="small"
                      color="error"
                      sx={{
                        border: '1px solid',
                        borderColor: 'error.main'
                      }}
                    >
                      <Typography variant="caption" sx={{ fontWeight: 'bold' }}>
                        Retry
                      </Typography>
                    </IconButton>
                  </Tooltip>
                )}
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
                <Chip
                  label={`${filteredLogs.length} / ${currentLogs.length} lines`}
                  size="small"
                  variant="outlined"
                />
              </Box>
            </Box>

            <TextField
              fullWidth
              size="small"
              placeholder="Search logs..."
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              InputProps={{
                startAdornment: (
                  <SearchIcon sx={{ mr: 1, color: 'text.secondary' }} fontSize="small" />
                ),
                endAdornment: searchTerm && (
                  <IconButton size="small" onClick={() => setSearchTerm('')}>
                    <ClearIcon fontSize="small" />
                  </IconButton>
                )
              }}
            />
          </Box>

          {isPaused && currentLogs.length > 0 && (
            <Alert severity="info" sx={{ mb: 2 }}>
              Logs are paused. Showing {currentLogs.length} lines captured before pause. Turn Live ON to resume streaming.
            </Alert>
          )}

          {/* Loading State */}
          {(currentIsLoading || isTransitioning) && (
            <Box
              sx={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                py: 4
              }}
            >
              <CircularProgress size={24} sx={{ mr: 2 }} />
              <Typography variant="body2" color="text.secondary">
                {isTransitioning
                  ? `Switching to ${logSource === 'pod' ? 'pod' : 'controller'} logs...`
                  : `Connecting to ${logSource === 'pod' ? 'pod' : 'controller'} log stream...`}
              </Typography>
            </Box>
          )}

          {/* Error State */}
          {currentError && (
            <Alert
              severity="error"
              sx={{ mb: 2 }}
              action={
                <IconButton color="inherit" size="small" onClick={currentReconnect}>
                  <Typography variant="caption">Retry</Typography>
                </IconButton>
              }
            >
              Failed to connect to {logSource === 'pod' ? 'pod' : 'controller'} log stream:{' '}
              {currentError}
            </Alert>
          )}

          {/* Logs Display */}
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
              {currentLogs.length === 0 &&
                !currentIsLoading &&
                !currentError &&
                !isTransitioning && (
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
                      textAlign: 'right',
                      color: isDarkMode ? DARK_TEXT_SECONDARY : LIGHT_TEXT_SECONDARY,
                      userSelect: 'none',
                      fontSize: '0.75rem',
                      fontFamily: 'monospace'
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
      </DrawerContent>
    </StyledDrawer>
  )
}
