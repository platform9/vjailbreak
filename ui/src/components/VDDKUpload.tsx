import React, { useState } from 'react'
import {
  Box,
  Button,
  LinearProgress,
  Paper,
  Typography,
  Alert,
  CircularProgress
} from '@mui/material'
import CloudUploadIcon from '@mui/icons-material/CloudUpload'
import CheckCircleIcon from '@mui/icons-material/CheckCircle'
import ErrorIcon from '@mui/icons-material/Error'
import axios from 'axios'

interface UploadResponse {
  upload_id: string
  status: string
  message: string
  extract_dir: string
  bytes_received: number
  progress_percentage: number
}

export default function VDDKUpload() {
  const [file, setFile] = useState<File | null>(null)
  const [uploading, setUploading] = useState(false)
  const [progress, setProgress] = useState(0)
  const [uploadResult, setUploadResult] = useState<UploadResponse | null>(null)
  const [error, setError] = useState<string | null>(null)

  const handleFileChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    const selectedFile = event.target.files?.[0]
    if (selectedFile) {
      setFile(selectedFile)
      setUploadResult(null)
      setError(null)
    }
  }

  const uploadFile = async () => {
    if (!file) return

    setUploading(true)
    setProgress(0)
    setError(null)
    setUploadResult(null)

    try {
      const reader = new FileReader()
      
      reader.onprogress = (e) => {
        if (e.lengthComputable) {
          const percentComplete = (e.loaded / e.total) * 50
          setProgress(percentComplete)
        }
      }

      reader.onload = async () => {
        try {
          const base64 = (reader.result as string).split(',')[1]
          
          setProgress(50)

          const response = await axios.post<UploadResponse>(
            '/dev-api/sdk/vpw/v1/upload_vddk',
            {
              upload_id: `ui_upload_${Date.now()}`,
              filename: file.name,
              file_chunk: base64,
              chunk_index: 0,
              total_chunks: 1
            },
            {
              headers: {
                'Content-Type': 'application/json'
              },
              onUploadProgress: (progressEvent) => {
                if (progressEvent.total) {
                  const percentComplete = 50 + (progressEvent.loaded / progressEvent.total) * 50
                  setProgress(percentComplete)
                }
              }
            }
          )

          setProgress(100)
          setUploadResult(response.data)
        } catch (err: any) {
          setError(err.response?.data?.message || err.message || 'Upload failed')
        } finally {
          setUploading(false)
        }
      }

      reader.onerror = () => {
        setError('Failed to read file')
        setUploading(false)
      }

      reader.readAsDataURL(file)
    } catch (err: any) {
      setError(err.message || 'Upload failed')
      setUploading(false)
    }
  }

  return (
    <Paper variant="outlined" sx={{ p: 3 }}>
      <Box display="flex" flexDirection="column" gap={2}>
        <Box display="flex" alignItems="center" gap={1}>
          <CloudUploadIcon color="primary" />
          <Typography variant="h6">VDDK Upload</Typography>
        </Box>

        <Typography variant="body2" color="text.secondary">
          Upload VDDK tar file (.tar.gz or .tar) to be extracted to /home/ubuntu on the server.
        </Typography>

        <Box display="flex" gap={2} alignItems="center">
          <Button
            variant="outlined"
            component="label"
            disabled={uploading}
          >
            Choose File
            <input
              type="file"
              hidden
              accept=".tar,.tar.gz,.tgz"
              onChange={handleFileChange}
            />
          </Button>
          
          {file && (
            <Typography variant="body2" color="text.secondary">
              {file.name} ({(file.size / (1024 * 1024)).toFixed(2)} MB)
            </Typography>
          )}
        </Box>

        {file && !uploading && !uploadResult && (
          <Button
            variant="contained"
            onClick={uploadFile}
            startIcon={<CloudUploadIcon />}
          >
            Upload VDDK
          </Button>
        )}

        {uploading && (
          <Box>
            <Box display="flex" alignItems="center" gap={2} mb={1}>
              <CircularProgress size={20} />
              <Typography variant="body2">
                Uploading... {Math.round(progress)}%
              </Typography>
            </Box>
            <LinearProgress variant="determinate" value={progress} />
          </Box>
        )}

        {uploadResult && (
          <Alert
            severity="success"
            icon={<CheckCircleIcon />}
            sx={{ mt: 1 }}
          >
            <Typography variant="body2" fontWeight={600}>
              {uploadResult.message}
            </Typography>
            <Typography variant="caption" display="block" sx={{ mt: 0.5 }}>
              Upload ID: {uploadResult.upload_id}
            </Typography>
            <Typography variant="caption" display="block">
              Extract Directory: {uploadResult.extract_dir}
            </Typography>
            <Typography variant="caption" display="block">
              Bytes Received: {(uploadResult.bytes_received / (1024 * 1024)).toFixed(2)} MB
            </Typography>
          </Alert>
        )}

        {error && (
          <Alert severity="error" icon={<ErrorIcon />}>
            <Typography variant="body2">{error}</Typography>
          </Alert>
        )}
      </Box>
    </Paper>
  )
}
