import { useState, useCallback, useMemo, useRef } from 'react'
import {
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  IconButton,
  Box,
  Typography,
  TextField,
  Chip,
  Tooltip,
  Paper,
  useTheme,
  Button
} from '@mui/material'
import CloseIcon from '@mui/icons-material/Close'
import ContentCopyIcon from '@mui/icons-material/ContentCopy'
import DownloadIcon from '@mui/icons-material/Download'
import SearchIcon from '@mui/icons-material/Search'
import ClearIcon from '@mui/icons-material/Clear'
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

interface YamlViewerModalProps {
  open: boolean
  onClose: () => void
  title: string
  subtitle?: string
  data: unknown
}

function jsonToYaml(obj: unknown, indent = 0): string {
  const pad = '  '.repeat(indent)

  if (obj === null) return 'null'
  if (obj === undefined) return ''
  if (typeof obj === 'boolean') return String(obj)
  if (typeof obj === 'number') return String(obj)
  if (typeof obj === 'string') {
    if (
      obj.includes('\n') ||
      obj.includes(':') ||
      obj.includes('#') ||
      obj.includes('{') ||
      obj.includes('[') ||
      obj.startsWith(' ') ||
      obj.endsWith(' ') ||
      obj === ''
    ) {
      // Multi-line or special strings: use block scalar or quoted
      if (obj.includes('\n')) {
        const lines = obj.split('\n')
        return `|\n${lines.map((l) => pad + '  ' + l).join('\n')}`
      }
      return JSON.stringify(obj)
    }
    return obj
  }

  if (Array.isArray(obj)) {
    if (obj.length === 0) return '[]'
    return obj
      .map((item) => {
        const rendered = jsonToYaml(item, indent + 1)
        if (typeof item === 'object' && item !== null && !Array.isArray(item)) {
          // Object items in an array: first key on the same line as `-`
          const lines = rendered.split('\n')
          return `${pad}- ${lines[0].trimStart()}\n${lines
            .slice(1)
            .map((l) => `${pad}  ${l.trimStart()}`)
            .join('\n')}`
            .trimEnd()
        }
        return `${pad}- ${rendered}`
      })
      .join('\n')
  }

  if (typeof obj === 'object') {
    const entries = Object.entries(obj as Record<string, unknown>)
    if (entries.length === 0) return '{}'
    return entries
      .map(([key, val]) => {
        if (typeof val === 'object' && val !== null) {
          const rendered = jsonToYaml(val, indent + 1)
          if (Array.isArray(val) && val.length === 0) return `${pad}${key}: []`
          if (!Array.isArray(val) && Object.keys(val as object).length === 0) {
            return `${pad}${key}: {}`
          }
          return `${pad}${key}:\n${rendered}`
        }
        return `${pad}${key}: ${jsonToYaml(val, indent + 1)}`
      })
      .join('\n')
  }

  return String(obj)
}

export default function YamlViewerModal({
  open,
  onClose,
  title,
  subtitle,
  data
}: YamlViewerModalProps) {
  const theme = useTheme()
  const isDarkMode = theme.palette.mode === 'dark'
  const [searchTerm, setSearchTerm] = useState('')
  const [copySuccess, setCopySuccess] = useState(false)
  const [downloadSuccess, setDownloadSuccess] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)

  const yamlContent = useMemo(() => {
    if (!data) return ''
    try {
      return jsonToYaml(data)
    } catch {
      return JSON.stringify(data, null, 2)
    }
  }, [data])

  const lines = useMemo(() => yamlContent.split('\n'), [yamlContent])

  const filteredLines = useMemo(() => {
    if (!searchTerm.trim()) return lines
    const lower = searchTerm.toLowerCase()
    return lines.filter((l) => l.toLowerCase().includes(lower))
  }, [lines, searchTerm])

  const handleCopy = useCallback(() => {
    navigator.clipboard.writeText(yamlContent).then(() => {
      setCopySuccess(true)
      setTimeout(() => setCopySuccess(false), 2000)
    })
  }, [yamlContent])

  const handleDownload = useCallback(() => {
    const fileName = `${subtitle || title}-resource.yaml`
    const blob = new Blob([yamlContent], { type: 'text/plain' })
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
  }, [yamlContent, title, subtitle])

  const handleClose = useCallback(() => {
    setSearchTerm('')
    onClose()
  }, [onClose])

  return (
    <Dialog
      open={open}
      onClose={handleClose}
      maxWidth="md"
      fullWidth
      PaperProps={{
        sx: { height: '80vh', display: 'flex', flexDirection: 'column' }
      }}
    >
      <DialogTitle
        sx={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          pb: 1,
          borderBottom: 1,
          borderColor: 'divider'
        }}
      >
        <Box>
          <Typography variant="h6" component="span">
            {title}
          </Typography>
          {subtitle && (
            <Typography
              variant="body2"
              color="text.secondary"
              component="div"
              sx={{ mt: 0.25 }}
            >
              {subtitle}
            </Typography>
          )}
        </Box>
        <IconButton onClick={handleClose} size="small">
          <CloseIcon fontSize="small" />
        </IconButton>
      </DialogTitle>

      <DialogContent
        sx={{ flex: 1, overflow: 'hidden', display: 'flex', flexDirection: 'column', pt: 2 }}
      >
        {/* Controls row */}
        <Box
          sx={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            mb: 1.5,
            gap: 1
          }}
        >
          <TextField
            size="small"
            placeholder="Search..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            sx={{ flex: 1 }}
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

          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
            <Chip
              label={`${filteredLines.length} / ${lines.length} lines`}
              size="small"
              variant="outlined"
            />

            <Tooltip title={copySuccess ? 'Copied!' : 'Copy YAML'}>
              <IconButton
                size="small"
                onClick={handleCopy}
                color={copySuccess ? 'success' : 'default'}
              >
                <ContentCopyIcon fontSize="small" />
              </IconButton>
            </Tooltip>

            <Tooltip title={downloadSuccess ? 'Downloaded!' : 'Download YAML'}>
              <IconButton
                size="small"
                onClick={handleDownload}
                color={downloadSuccess ? 'success' : 'default'}
              >
                <DownloadIcon fontSize="small" />
              </IconButton>
            </Tooltip>
          </Box>
        </Box>

        {/* YAML content */}
        <Paper
          variant="outlined"
          sx={{
            flex: 1,
            overflow: 'hidden',
            backgroundColor: isDarkMode ? DARK_BG_PAPER : LIGHT_BG_PAPER,
            borderColor: isDarkMode ? DARK_DIVIDER : LIGHT_DIVIDER
          }}
        >
          <Box
            ref={containerRef}
            sx={{
              height: '100%',
              overflow: 'auto',
              p: 2,
              color: isDarkMode ? DARK_TEXT_PRIMARY : LIGHT_TEXT_PRIMARY,
              fontFamily: 'monospace',
              fontSize: '0.8125rem',
              lineHeight: 1.5,
              whiteSpace: 'pre',
              wordBreak: 'break-all'
            }}
          >
            {filteredLines.length === 0 ? (
              <Typography
                variant="body2"
                sx={{
                  fontFamily: 'monospace',
                  color: isDarkMode ? DARK_TEXT_SECONDARY : LIGHT_TEXT_SECONDARY
                }}
              >
                No matches found
              </Typography>
            ) : (
              filteredLines.map((line, idx) => {
                const lineNumber = idx + 1

                return (
                  <Box
                    key={idx}
                    sx={{ display: 'flex', '&:hover': { bgcolor: 'action.hover' } }}
                  >
                    <Box
                      sx={{
                        minWidth: '44px',
                        pr: 2,
                        py: 0.125,
                        textAlign: 'right',
                        color: isDarkMode ? DARK_TEXT_SECONDARY : LIGHT_TEXT_SECONDARY,
                        userSelect: 'none',
                        fontSize: '0.75rem',
                        fontFamily: 'monospace',
                        lineHeight: 1.5,
                        flexShrink: 0
                      }}
                    >
                      {lineNumber}
                    </Box>
                    <Box
                      component="span"
                      sx={{ flex: 1 }}
                      dangerouslySetInnerHTML={{
                        __html: highlightYamlLine(line, searchTerm, isDarkMode)
                      }}
                    />
                  </Box>
                )
              })
            )}
          </Box>
        </Paper>
      </DialogContent>

      <DialogActions sx={{ borderTop: 1, borderColor: 'divider', px: 3, py: 1.5 }}>
        <Button variant="outlined" size="small" onClick={handleClose}>
          Close
        </Button>
      </DialogActions>
    </Dialog>
  )
}

function escapeHtml(str: string): string {
  return str
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
}

function highlightYamlLine(line: string, searchTerm: string, isDarkMode: boolean): string {
  const keyColor = isDarkMode ? '#79b8ff' : '#005cc5'
  const stringColor = isDarkMode ? '#9ecbff' : '#032f62'
  const numberColor = isDarkMode ? '#79c0ff' : '#e36209'
  const boolColor = isDarkMode ? '#ff7b72' : '#d73a49'
  const listMarkerColor = isDarkMode ? '#f97583' : '#e36209'

  // Tokenize the line into segments
  let html = ''

  // Match YAML key-value or list item patterns
  const keyValueMatch = line.match(/^(\s*)([\w\-./]+)(\s*:\s*)(.*)$/)
  const listItemMatch = line.match(/^(\s*)(- )(.*)$/)
  const bareValueMatch = line.match(/^(\s*)(.+)$/)

  if (keyValueMatch) {
    const [, spaces, key, colon, value] = keyValueMatch
    html += escapeHtml(spaces)
    html += `<span style="color:${keyColor}">${escapeHtml(key)}</span>`
    html += escapeHtml(colon)
    html += colorValue(value, isDarkMode, stringColor, numberColor, boolColor)
  } else if (listItemMatch) {
    const [, spaces, marker, rest] = listItemMatch
    html += escapeHtml(spaces)
    html += `<span style="color:${listMarkerColor}">${escapeHtml(marker)}</span>`
    html += colorValue(rest, isDarkMode, stringColor, numberColor, boolColor)
  } else if (bareValueMatch) {
    const [, spaces, rest] = bareValueMatch
    html += escapeHtml(spaces)
    html += colorValue(rest, isDarkMode, stringColor, numberColor, boolColor)
  } else {
    html = escapeHtml(line)
  }

  // Highlight search term
  if (searchTerm.trim()) {
    const escaped = searchTerm.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
    html = html.replace(
      new RegExp(`(${escaped})`, 'gi'),
      '<mark style="background:rgba(255,213,0,0.4);border-radius:2px">$1</mark>'
    )
  }

  return html || '&nbsp;'
}

function colorValue(
  value: string,
  _isDarkMode: boolean,
  stringColor: string,
  numberColor: string,
  boolColor: string
): string {
  if (!value) return ''
  const trimmed = value.trim()
  if (trimmed === 'true' || trimmed === 'false' || trimmed === 'null') {
    return `<span style="color:${boolColor}">${escapeHtml(value)}</span>`
  }
  if (/^-?\d+(\.\d+)?$/.test(trimmed)) {
    return `<span style="color:${numberColor}">${escapeHtml(value)}</span>`
  }
  if (trimmed.startsWith('"') || trimmed.startsWith("'") || trimmed.startsWith('|')) {
    return `<span style="color:${stringColor}">${escapeHtml(value)}</span>`
  }
  // Plain scalar string
  return `<span style="color:${stringColor}">${escapeHtml(value)}</span>`
}
