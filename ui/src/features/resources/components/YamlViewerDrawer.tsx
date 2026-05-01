import { useState, useCallback, useMemo } from 'react'
import {
  Box,
  Typography,
  IconButton,
  TextField,
  Tooltip,
  Alert,
  Button,
  CircularProgress,
  useTheme
} from '@mui/material'
import ContentCopyIcon from '@mui/icons-material/ContentCopy'
import DownloadIcon from '@mui/icons-material/Download'
import SearchIcon from '@mui/icons-material/Search'
import ClearIcon from '@mui/icons-material/Clear'
import EditIcon from '@mui/icons-material/Edit'
import CheckIcon from '@mui/icons-material/Check'
import CloseIcon from '@mui/icons-material/Close'
import { DrawerShell, DrawerHeader } from 'src/components'
import CodeMirror from '@uiw/react-codemirror'
import { yaml as yamlLang } from '@codemirror/lang-yaml'
import { EditorView } from '@codemirror/view'
import { parse as parseYaml, stringify as stringifyYaml } from 'yaml'
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

interface YamlViewerDrawerProps {
  open: boolean
  onClose: () => void
  title: string
  subtitle?: string
  data: unknown
  onSave?: (updated: unknown) => Promise<void>
}


export default function YamlViewerDrawer({ open, onClose, title, subtitle, data, onSave }: YamlViewerDrawerProps) {
  const theme = useTheme()
  const isDarkMode = theme.palette.mode === 'dark'
  const [searchTerm, setSearchTerm] = useState('')
  const [copySuccess, setCopySuccess] = useState(false)
  const [downloadSuccess, setDownloadSuccess] = useState(false)
  const [isEditing, setIsEditing] = useState(false)
  const [editContent, setEditContent] = useState('')
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState<string | null>(null)

  // Strip managedFields from display — matches kubectl default behaviour
  const displayData = useMemo(() => {
    if (!data || typeof data !== 'object') return data
    const obj = data as Record<string, unknown>
    if (!obj.metadata || typeof obj.metadata !== 'object') return data
    const { managedFields: _, ...cleanMetadata } = obj.metadata as Record<string, unknown>
    return { ...obj, metadata: cleanMetadata }
  }, [data])

  const yamlContent = useMemo(() => {
    if (!displayData) return ''
    try {
      // schema: 'core' prevents yaml from auto-converting timestamps to Date objects
      return stringifyYaml(displayData, { schema: 'core' })
    } catch {
      return JSON.stringify(displayData, null, 2)
    }
  }, [displayData])

  const lines = useMemo(() => yamlContent.split('\n'), [yamlContent])

  const filteredLines = useMemo(() => {
    if (!searchTerm.trim()) return lines
    return lines.filter((l) => l.toLowerCase().includes(searchTerm.toLowerCase()))
  }, [lines, searchTerm])

  const handleCopy = useCallback(() => {
    navigator.clipboard.writeText(yamlContent).then(() => {
      setCopySuccess(true)
      setTimeout(() => setCopySuccess(false), 2000)
    })
  }, [yamlContent])

  const handleDownload = useCallback(() => {
    const blob = new Blob([yamlContent], { type: 'text/plain' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `${subtitle || title}.yaml`
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
    setDownloadSuccess(true)
    setTimeout(() => setDownloadSuccess(false), 2000)
  }, [yamlContent, title, subtitle])

  const handleStartEdit = useCallback(() => {
    setEditContent(yamlContent)
    setSaveError(null)
    setIsEditing(true)
  }, [yamlContent])

  const handleCancelEdit = useCallback(() => {
    setIsEditing(false)
    setSaveError(null)
  }, [])

  const handleSave = useCallback(async () => {
    if (!onSave) return
    setSaving(true)
    setSaveError(null)
    try {
      // schema: 'core' keeps timestamps as strings — prevents Date conversion that breaks Kubernetes API
      const parsed = parseYaml(editContent, { schema: 'core' })
      await onSave(parsed)
      setIsEditing(false)
    } catch (err) {
      setSaveError(err instanceof Error ? err.message : 'Failed to save changes')
    } finally {
      setSaving(false)
    }
  }, [editContent, onSave])

  const handleClose = useCallback(() => {
    setSearchTerm('')
    setIsEditing(false)
    setSaveError(null)
    onClose()
  }, [onClose])

  const editorExtensions = useMemo(() => [yamlLang(), EditorView.lineWrapping], [])

  return (
    <DrawerShell open={open} onClose={handleClose} requireCloseConfirmation={false}
      header={<DrawerHeader title={title} subtitle={subtitle} onClose={handleClose} />}
    >
      <Box sx={{ display: 'flex', flexDirection: 'column', height: '100%', overflow: 'hidden' }}>
        {/* Controls */}
        <Box
          sx={{
            flexShrink: 0,
            pt: 1,
            pb: 2,
            borderBottom: 1,
            borderColor: 'divider',
            mb: 2
          }}
        >
          <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'flex-end', mb: 1.5 }}>
            {isEditing ? (
              <Box sx={{ display: 'flex', gap: 1, alignItems: 'center' }}>
                <Button
                  size="small"
                  variant="outlined"
                  onClick={handleCancelEdit}
                  startIcon={<CloseIcon fontSize="small" />}
                  disabled={saving}
                >
                  Cancel
                </Button>
                <Button
                  size="small"
                  variant="contained"
                  onClick={handleSave}
                  startIcon={saving ? <CircularProgress size={14} /> : <CheckIcon fontSize="small" />}
                  disabled={saving}
                >
                  Save
                </Button>
              </Box>
            ) : (
              <Box sx={{ display: 'flex', gap: 0.5 }}>
                {onSave && (
                  <Tooltip title="Edit">
                    <IconButton size="small" onClick={handleStartEdit}>
                      <EditIcon fontSize="small" />
                    </IconButton>
                  </Tooltip>
                )}
                <Tooltip title={copySuccess ? 'Copied!' : 'Copy YAML'}>
                  <IconButton size="small" onClick={handleCopy} color={copySuccess ? 'success' : 'default'}>
                    <ContentCopyIcon fontSize="small" />
                  </IconButton>
                </Tooltip>
                <Tooltip title={downloadSuccess ? 'Downloaded!' : 'Download YAML'}>
                  <IconButton size="small" onClick={handleDownload} color={downloadSuccess ? 'success' : 'default'}>
                    <DownloadIcon fontSize="small" />
                  </IconButton>
                </Tooltip>
              </Box>
            )}
          </Box>

          {!isEditing && (
            <TextField
              fullWidth
              size="small"
              placeholder="Search..."
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
          )}
        </Box>

        {saveError && (
          <Alert severity="error" sx={{ mb: 2 }} onClose={() => setSaveError(null)}>
            {saveError}
          </Alert>
        )}

        {isEditing ? (
          <Box
            sx={{
              flex: 1,
              minHeight: 0,
              overflow: 'auto',
              border: 1,
              borderColor: 'divider',
              borderRadius: 1
            }}
          >
            <CodeMirror
              value={editContent}
              extensions={editorExtensions}
              theme={isDarkMode ? 'dark' : 'light'}
              onChange={(value) => setEditContent(value)}
              basicSetup={{
                lineNumbers: true,
                highlightActiveLine: true,
                syntaxHighlighting: true
              }}
            />
          </Box>
        ) : (
          <Box
            sx={{
              flex: 1,
              minHeight: 0,
              overflow: 'auto',
              p: 2,
              border: 1,
              borderColor: isDarkMode ? DARK_DIVIDER : LIGHT_DIVIDER,
              borderRadius: 1,
              bgcolor: isDarkMode ? DARK_BG_PAPER : LIGHT_BG_PAPER,
              color: isDarkMode ? DARK_TEXT_PRIMARY : LIGHT_TEXT_PRIMARY,
              fontFamily: 'monospace',
              fontSize: '0.8125rem',
              lineHeight: 1.5,
              whiteSpace: 'pre',
              wordBreak: 'break-all'
            }}
          >
            {filteredLines.length === 0 ? (
              <Typography variant="body2" sx={{ fontFamily: 'monospace', color: isDarkMode ? DARK_TEXT_SECONDARY : LIGHT_TEXT_SECONDARY }}>
                No matches found
              </Typography>
            ) : (
              filteredLines.map((line, idx) => (
                <Box key={idx} sx={{ display: 'flex', '&:hover': { bgcolor: 'action.hover' } }}>
                  <Box sx={{
                    minWidth: '44px', pr: 2, py: 0.125, textAlign: 'right',
                    color: isDarkMode ? DARK_TEXT_SECONDARY : LIGHT_TEXT_SECONDARY,
                    userSelect: 'none', fontSize: '0.75rem', fontFamily: 'monospace', lineHeight: 1.5, flexShrink: 0
                  }}>
                    {idx + 1}
                  </Box>
                  <Box component="span" sx={{ flex: 1 }}>
                    {line || ' '}
                  </Box>
                </Box>
              ))
            )}
          </Box>
        )}
      </Box>
    </DrawerShell>
  )
}
