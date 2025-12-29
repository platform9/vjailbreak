import React, { useState, useCallback } from 'react'
import {
  Box,
  Button,
  Typography,
  LinearProgress,
  Alert,
  Paper,
  styled
} from '@mui/material'
import CloudUploadIcon from '@mui/icons-material/CloudUpload'
import CheckCircleIcon from '@mui/icons-material/CheckCircle'
import ErrorIcon from '@mui/icons-material/Error'
import FieldLabel from 'src/components/design-system/ui/FieldLabel'

const UploadContainer = styled(Paper)(({ theme }) => ({
  padding: theme.spacing(3),
  border: `2px dashed ${theme.palette.divider}`,
  borderRadius: theme.shape.borderRadius,
  textAlign: 'center',
  cursor: 'pointer',
  transition: 'all 0.2s ease',
  '&:hover': {
    borderColor: theme.palette.primary.main,
    backgroundColor: theme.palette.action.hover
  },
  '&.dragging': {
    borderColor: theme.palette.primary.main,
    backgroundColor: theme.palette.action.selected
  }
}))

const HiddenInput = styled('input')({
  display: 'none'
})

type UploadStatus = 'idle' | 'uploading' | 'success' | 'error'

interface UploadResponse {
  success: boolean
  message: string
  file_path?: string
  extracted_path?: string
}

export default function VDDKUploadTab() {
  const [selectedFile, setSelectedFile] = useState<File | null>(null)
  const [uploadStatus, setUploadStatus] = useState<UploadStatus>('idle')
  const [uploadProgress, setUploadProgress] = useState(0)
  const [uploadMessage, setUploadMessage] = useState('')
  const [extractedPath, setExtractedPath] = useState('')
  const [isDragging, setIsDragging] = useState(false)

  const handleFileSelect = useCallback((file: File | null) => {
    if (!file) return

    const validExtensions = ['.tar', '.tar.gz', '.tgz']
    const isValid = validExtensions.some((ext) => file.name.toLowerCase().endsWith(ext))

    if (!isValid) {
      setUploadStatus('error')
      setUploadMessage('Invalid file type. Please select a .tar or .tar.gz file.')
      return
    }

    if (file.size > 500 * 1024 * 1024) {
      setUploadStatus('error')
      setUploadMessage('File size exceeds 500MB limit.')
      return
    }

    setSelectedFile(file)
    setUploadStatus('idle')
    setUploadMessage('')
  }, [])

  const handleFileInputChange = useCallback(
    (event: React.ChangeEvent<HTMLInputElement>) => {
      const file = event.target.files?.[0] || null
      handleFileSelect(file)
    },
    [handleFileSelect]
  )

  const handleDragOver = useCallback((event: React.DragEvent) => {
    event.preventDefault()
    setIsDragging(true)
  }, [])

  const handleDragLeave = useCallback((event: React.DragEvent) => {
    event.preventDefault()
    setIsDragging(false)
  }, [])

  const handleDrop = useCallback(
    (event: React.DragEvent) => {
      event.preventDefault()
      setIsDragging(false)
      const file = event.dataTransfer.files?.[0] || null
      handleFileSelect(file)
    },
    [handleFileSelect]
  )

  const handleUpload = useCallback(async () => {
    if (!selectedFile) return

    setUploadStatus('uploading')
    setUploadProgress(0)
    setUploadMessage('Uploading VDDK file...')

    const formData = new FormData()
    formData.append('vddk_file', selectedFile)

    try {
      const xhr = new XMLHttpRequest()

      xhr.upload.addEventListener('progress', (event) => {
        if (event.lengthComputable) {
          const percentComplete = (event.loaded / event.total) * 100
          setUploadProgress(percentComplete)
        }
      })

      xhr.addEventListener('load', () => {
        if (xhr.status === 200) {
          try {
            const response: UploadResponse = JSON.parse(xhr.responseText)
            setUploadStatus('success')
            setUploadMessage(response.message || 'VDDK file uploaded and extracted successfully!')
            setExtractedPath(response.extracted_path || '')
          } catch (e) {
            setUploadStatus('error')
            setUploadMessage('Upload succeeded but failed to parse response.')
          }
        } else {
          setUploadStatus('error')
          setUploadMessage(`Upload failed: ${xhr.statusText}`)
        }
      })

      xhr.addEventListener('error', () => {
        setUploadStatus('error')
        setUploadMessage('Network error occurred during upload.')
      })

      xhr.open('POST', '/vpw/v1/vddk/upload')
      xhr.send(formData)
    } catch (error) {
      setUploadStatus('error')
      setUploadMessage(`Upload failed: ${error instanceof Error ? error.message : 'Unknown error'}`)
    }
  }, [selectedFile])

  const handleReset = useCallback(() => {
    setSelectedFile(null)
    setUploadStatus('idle')
    setUploadProgress(0)
    setUploadMessage('')
    setExtractedPath('')
  }, [])

  return (
    <Box sx={{ pt: 3 }}>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
        Upload VDDK (Virtual Disk Development Kit) tar files for VMware integration. The file will
        be automatically extracted after upload.
      </Typography>

      <Box sx={{ maxWidth: 600 }}>
        <FieldLabel
          label="VDDK File"
          tooltip="Upload a VDDK tar or tar.gz file (max 500MB)"
        />

        <UploadContainer
          className={isDragging ? 'dragging' : ''}
          onDragOver={handleDragOver}
          onDragLeave={handleDragLeave}
          onDrop={handleDrop}
          onClick={() => document.getElementById('vddk-file-input')?.click()}
          elevation={0}
        >
          <CloudUploadIcon sx={{ fontSize: 48, color: 'text.secondary', mb: 2 }} />
          <Typography variant="body1" gutterBottom>
            {selectedFile ? selectedFile.name : 'Drag and drop VDDK file here'}
          </Typography>
          <Typography variant="body2" color="text.secondary">
            or click to browse
          </Typography>
          <Typography variant="caption" color="text.secondary" sx={{ mt: 1, display: 'block' }}>
            Supported formats: .tar, .tar.gz (max 500MB)
          </Typography>
        </UploadContainer>

        <HiddenInput
          id="vddk-file-input"
          type="file"
          accept=".tar,.tar.gz,.tgz"
          onChange={handleFileInputChange}
        />

        {selectedFile && uploadStatus === 'idle' && (
          <Box sx={{ mt: 2, display: 'flex', gap: 2 }}>
            <Button
              variant="contained"
              color="primary"
              startIcon={<CloudUploadIcon />}
              onClick={handleUpload}
              fullWidth
            >
              Upload and Extract
            </Button>
            <Button variant="outlined" onClick={handleReset}>
              Clear
            </Button>
          </Box>
        )}

        {uploadStatus === 'uploading' && (
          <Box sx={{ mt: 3 }}>
            <Typography variant="body2" color="text.secondary" gutterBottom>
              {uploadMessage}
            </Typography>
            <LinearProgress variant="determinate" value={uploadProgress} sx={{ mt: 1 }} />
            <Typography variant="caption" color="text.secondary" sx={{ mt: 0.5, display: 'block' }}>
              {Math.round(uploadProgress)}% complete
            </Typography>
          </Box>
        )}

        {uploadStatus === 'success' && (
          <Alert
            severity="success"
            icon={<CheckCircleIcon />}
            sx={{ mt: 3 }}
            action={
              <Button color="inherit" size="small" onClick={handleReset}>
                Upload Another
              </Button>
            }
          >
            <Typography variant="body2" fontWeight={600}>
              {uploadMessage}
            </Typography>
            {extractedPath && (
              <Typography variant="caption" sx={{ mt: 1, display: 'block' }}>
                Extracted to: {extractedPath}
              </Typography>
            )}
          </Alert>
        )}

        {uploadStatus === 'error' && (
          <Alert
            severity="error"
            icon={<ErrorIcon />}
            sx={{ mt: 3 }}
            action={
              <Button color="inherit" size="small" onClick={handleReset}>
                Try Again
              </Button>
            }
          >
            <Typography variant="body2" fontWeight={600}>
              {uploadMessage}
            </Typography>
          </Alert>
        )}
      </Box>
    </Box>
  )
}
