import { useState, useEffect } from 'react'
import {
  Drawer,
  Box,
  Typography,
  TextField,
  Button,
  FormControl,
  InputLabel,
  Select,
  MenuItem,
  FormControlLabel,
  Checkbox,
  Alert,
  CircularProgress,
  Divider
} from '@mui/material'
import InfoIcon from '@mui/icons-material/Info'
import CheckCircleIcon from '@mui/icons-material/CheckCircle'
import ErrorIcon from '@mui/icons-material/Error'
import { useForm, Controller } from 'react-hook-form'
import { useQueryClient } from '@tanstack/react-query'
import { ARRAY_CREDS_QUERY_KEY } from 'src/hooks/api/useArrayCredentialsQuery'
import { ArrayCreds, ARRAY_VENDOR_TYPES } from 'src/api/array-creds/model'
import { updateArrayCredsWithSecret, getArrayCredentials } from 'src/api/array-creds/arrayCreds'
import { createArrayCredsSecret } from 'src/api/secrets/secrets'
import { useErrorHandler } from 'src/hooks/useErrorHandler'
import { VJAILBREAK_DEFAULT_NAMESPACE } from 'src/api/constants'

interface EditArrayCredentialsDrawerProps {
  open: boolean
  onClose: () => void
  credential: ArrayCreds
}

interface FormData {
  name: string
  vendorType: string
  volumeType: string
  backendName: string
  managementEndpoint: string
  username: string
  password: string
  skipSslVerification: boolean
}

type ValidationStatus = 'idle' | 'validating' | 'success' | 'failed'


export default function EditArrayCredentialsDrawer({
  open,
  onClose,
  credential
}: EditArrayCredentialsDrawerProps) {
  const { reportError } = useErrorHandler({ component: 'EditArrayCredentialsDrawer' })
  const queryClient = useQueryClient()
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [isValidating, setIsValidating] = useState(false)
  const [validationStatus, setValidationStatus] = useState<ValidationStatus>('idle')
  const [validationMessage, setValidationMessage] = useState<string>('')
  const [submitError, setSubmitError] = useState<string | null>(null)

  const isAutoDiscovered = credential.spec?.autoDiscovered

  const {
    control,
    handleSubmit,
    formState: { errors },
    reset
  } = useForm<FormData>({
    defaultValues: {
      name: credential.metadata.name,
      vendorType: credential.spec?.vendorType || 'pure',
      volumeType: credential.spec?.openstackMapping?.volumeType || '',
      backendName: credential.spec?.openstackMapping?.cinderBackendName || '',
      managementEndpoint: '',
      username: '',
      password: '',
      skipSslVerification: false
    }
  })

  useEffect(() => {
    reset({
      name: credential.metadata.name,
      vendorType: credential.spec?.vendorType || 'pure',
      volumeType: credential.spec?.openstackMapping?.volumeType || '',
      backendName: credential.spec?.openstackMapping?.cinderBackendName || '',
      managementEndpoint: '',
      username: '',
      password: '',
      skipSslVerification: false
    })
  }, [credential, reset])

  const handleClose = () => {
    if (isValidating) return // Don't allow closing while validating
    setSubmitError(null)
    setValidationStatus('idle')
    setValidationMessage('')
    onClose()
  }

  // Poll for validation status after updating credentials
  const pollForValidation = async (
    name: string,
    maxAttempts = 30,
    intervalMs = 2000
  ): Promise<{ success: boolean; message?: string }> => {
    for (let attempt = 0; attempt < maxAttempts; attempt++) {
      await new Promise((resolve) => setTimeout(resolve, intervalMs))

      try {
        const updated = await getArrayCredentials(name)
        const status = updated.status?.arrayValidationStatus

        // Check if validation succeeded
        if (status === 'Succeeded') {
          return { success: true }
        }

        // Check if validation failed
        if (status && status !== 'AwaitingCredentials' && status !== 'Validating') {
          return {
            success: false,
            message:
              updated.status?.arrayValidationMessage || `Validation failed with status: ${status}`
          }
        }
      } catch (error) {
        console.error('Error polling for validation:', error)
      }
    }

    // Timeout - validation took too long
    return { success: false, message: 'Validation timed out after 60 seconds' }
  }

  const onSubmit = async (data: FormData) => {
    setIsSubmitting(true)
    setSubmitError(null)
    setValidationStatus('idle')
    setValidationMessage('')

    try {
      const namespace = credential.metadata.namespace || VJAILBREAK_DEFAULT_NAMESPACE
      const secretName = `${data.name}-array-secret`

      // Create or update the secret if credentials are provided
      if (data.managementEndpoint && data.username && data.password) {
        await createArrayCredsSecret(
          secretName,
          {
            ARRAY_HOSTNAME: data.managementEndpoint,
            ARRAY_USERNAME: data.username,
            ARRAY_PASSWORD: data.password,
            ARRAY_SKIP_SSL_VERIFICATION: data.skipSslVerification
          },
          namespace
        )
      }

      // Update the ArrayCreds resource
      await updateArrayCredsWithSecret(
        data.name,
        secretName,
        data.vendorType,
        {
          volumeType: data.volumeType,
          cinderBackendName: data.backendName
        },
        namespace
      )

      // Start validation polling if credentials were provided
      if (data.managementEndpoint && data.username && data.password) {
        setIsSubmitting(false)
        setIsValidating(true)
        setValidationStatus('validating')
        setValidationMessage('Validating credentials with storage array...')

        const validationResult = await pollForValidation(data.name)
        setIsValidating(false)

        if (!validationResult.success) {
          // Validation failed - show error but keep the resource
          setValidationStatus('failed')
          setValidationMessage(validationResult.message || 'Validation failed')
          queryClient.invalidateQueries({ queryKey: ARRAY_CREDS_QUERY_KEY })
          return // Stay in the form
        }

        // Validation succeeded
        setValidationStatus('success')
        setValidationMessage('Credentials validated successfully!')
        queryClient.invalidateQueries({ queryKey: ARRAY_CREDS_QUERY_KEY })

        // Wait a moment to show success message before closing
        await new Promise((resolve) => setTimeout(resolve, 1500))
      } else {
        queryClient.invalidateQueries({ queryKey: ARRAY_CREDS_QUERY_KEY })
      }

      setValidationStatus('idle')
      setValidationMessage('')

      onClose()
    } catch (error: any) {
      console.error('Error updating array credentials:', error)
      setIsValidating(false)
      setValidationStatus('idle')
      const errorMessage =
        error?.response?.data?.message || error?.message || 'Failed to update array credentials'
      setSubmitError(errorMessage)
      reportError(error, {
        context: 'edit-array-credentials',
        metadata: { credentialName: data.name }
      })
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Drawer
      anchor="right"
      open={open}
      onClose={handleClose}
      PaperProps={{
        sx: {
          width: { xs: '100%', sm: 500 },
          p: 3,
          backgroundColor: 'background.paper'
        }
      }}
    >
      <Box component="form" onSubmit={handleSubmit(onSubmit)} sx={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
        <Typography variant="h5" fontWeight={600} sx={{ mb: 3 }}>
          Edit Array Credentials
        </Typography>

        {isAutoDiscovered && (
          <Alert
            severity="info"
            icon={<InfoIcon />}
            sx={{
              mb: 3,
              backgroundColor: 'rgba(33, 150, 243, 0.1)',
              '& .MuiAlert-message': { fontSize: '0.875rem' }
            }}
          >
            This array was auto-discovered from OpenStack Cinder. You can update the credentials and
            vendor type as needed.
          </Alert>
        )}

        {submitError && (
          <Alert severity="error" sx={{ mb: 2 }} onClose={() => setSubmitError(null)}>
            {submitError}
          </Alert>
        )}

        {validationStatus === 'validating' && (
          <Alert severity="info" icon={<CircularProgress size={20} />} sx={{ mb: 2 }}>
            {validationMessage}
          </Alert>
        )}

        {validationStatus === 'success' && (
          <Alert severity="success" icon={<CheckCircleIcon />} sx={{ mb: 2 }}>
            {validationMessage}
          </Alert>
        )}

        {validationStatus === 'failed' && (
          <Alert severity="error" icon={<ErrorIcon />} sx={{ mb: 2 }}>
            {validationMessage}
          </Alert>
        )}

        {/* Basic Information Section */}
        <Typography variant="subtitle1" fontWeight={600} sx={{ mb: 2 }}>
          Basic Information
        </Typography>

        <Controller
          name="name"
          control={control}
          render={({ field }) => (
            <TextField
              {...field}
              label="Name"
              required
              fullWidth
              disabled
              helperText="Name cannot be changed after creation"
              sx={{ mb: 2 }}
            />
          )}
        />

        <Controller
          name="vendorType"
          control={control}
          rules={{ required: 'Vendor type is required' }}
          render={({ field }) => (
            <FormControl fullWidth sx={{ mb: 3 }}>
              <InputLabel>Vendor Type *</InputLabel>
              <Select {...field} label="Vendor Type *">
                {ARRAY_VENDOR_TYPES.map((vendor) => (
                  <MenuItem key={vendor.value} value={vendor.value}>
                    {vendor.label}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
          )}
        />

        <Divider sx={{ my: 2 }} />

        {/* OpenStack Mapping Section */}
        <Typography variant="subtitle1" fontWeight={600} sx={{ mb: 2 }}>
          OpenStack Mapping
        </Typography>

        <Controller
          name="volumeType"
          control={control}
          rules={{ required: 'Volume type is required' }}
          render={({ field }) => (
            <TextField
              {...field}
              label="Volume Type"
              required
              fullWidth
              error={!!errors.volumeType}
              helperText={errors.volumeType?.message || 'Cinder volume type name'}
              sx={{ mb: 2 }}
            />
          )}
        />

        <Controller
          name="backendName"
          control={control}
          rules={{ required: 'Backend name is required' }}
          render={({ field }) => (
            <TextField
              {...field}
              label="Backend Name"
              required
              fullWidth
              error={!!errors.backendName}
              helperText={errors.backendName?.message || 'Cinder backend name'}
              sx={{ mb: 3 }}
            />
          )}
        />

        <Divider sx={{ my: 2 }} />

        {/* Storage Array Credentials Section */}
        <Typography variant="subtitle1" fontWeight={600} sx={{ mb: 2 }}>
          Storage Array Credentials
        </Typography>

        <Alert
          severity="info"
          icon={<InfoIcon />}
          sx={{
            mb: 2,
            backgroundColor: 'rgba(33, 150, 243, 0.1)',
            '& .MuiAlert-message': { fontSize: '0.875rem' }
          }}
        >
          Provide credentials to connect to the storage array management interface. These are used
          for advanced operations like volume management.
        </Alert>

        <Controller
          name="managementEndpoint"
          control={control}
          rules={{ required: 'Management endpoint is required' }}
          render={({ field }) => (
            <TextField
              {...field}
              label="Management Endpoint"
              required
              fullWidth
              error={!!errors.managementEndpoint}
              helperText={errors.managementEndpoint?.message || 'Storage array management IP or hostname'}
              sx={{ mb: 2 }}
            />
          )}
        />

        <Controller
          name="username"
          control={control}
          rules={{ required: 'Username is required' }}
          render={({ field }) => (
            <TextField
              {...field}
              label="Username"
              required
              fullWidth
              error={!!errors.username}
              helperText={errors.username?.message || 'Storage array username'}
              sx={{ mb: 2 }}
            />
          )}
        />

        <Controller
          name="password"
          control={control}
          rules={{ required: 'Password is required' }}
          render={({ field }) => (
            <TextField
              {...field}
              label="Password"
              type="password"
              required
              fullWidth
              error={!!errors.password}
              helperText={errors.password?.message || 'Storage array password'}
              sx={{ mb: 2 }}
            />
          )}
        />

        <Controller
          name="skipSslVerification"
          control={control}
          render={({ field }) => (
            <FormControlLabel
              control={<Checkbox {...field} checked={field.value} />}
              label="Skip SSL Certificate Verification"
              sx={{ mb: 2 }}
            />
          )}
        />

        {/* Action Buttons */}
        <Box sx={{ mt: 'auto', display: 'flex', gap: 2, justifyContent: 'flex-end', pt: 3 }}>
          <Button
            variant="outlined"
            onClick={handleClose}
            disabled={isSubmitting || isValidating}
          >
            CANCEL
          </Button>
          <Button
            type="submit"
            variant="contained"
            disabled={isSubmitting || isValidating || validationStatus === 'success'}
            startIcon={isSubmitting || isValidating ? <CircularProgress size={20} /> : null}
          >
            {isSubmitting
              ? 'UPDATING...'
              : isValidating
                ? 'VALIDATING...'
                : validationStatus === 'success'
                  ? 'SUCCESS'
                  : 'UPDATE'}
          </Button>
        </Box>
      </Box>
    </Drawer>
  )
}
