import { useState } from 'react'
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
import CheckCircleIcon from '@mui/icons-material/CheckCircle'
import ErrorIcon from '@mui/icons-material/Error'
import { useForm, Controller } from 'react-hook-form'
import { useQueryClient } from '@tanstack/react-query'
import { ARRAY_CREDS_QUERY_KEY } from 'src/hooks/api/useArrayCredentialsQuery'
import { createArrayCredsWithSecretFlow, deleteArrayCredsWithSecretFlow } from 'src/api/helpers'
import { getArrayCredentials } from 'src/api/array-creds/arrayCreds'
import { ARRAY_VENDOR_TYPES } from 'src/api/array-creds/model'
import { useErrorHandler } from 'src/hooks/useErrorHandler'
import { ConfirmationDialog } from 'src/components/dialogs'

interface AddArrayCredentialsDrawerProps {
  open: boolean
  onClose: () => void
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

export default function AddArrayCredentialsDrawer({
  open,
  onClose
}: AddArrayCredentialsDrawerProps) {
  const { reportError } = useErrorHandler({ component: 'AddArrayCredentialsDrawer' })
  const queryClient = useQueryClient()
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [isValidating, setIsValidating] = useState(false)
  const [validationStatus, setValidationStatus] = useState<ValidationStatus>('idle')
  const [validationMessage, setValidationMessage] = useState<string>('')
  const [submitError, setSubmitError] = useState<string | null>(null)
  const [showDiscardDialog, setShowDiscardDialog] = useState(false)

  const {
    control,
    handleSubmit,
    formState: { errors },
    reset,
    getValues
  } = useForm<FormData>({
    defaultValues: {
      name: '',
      vendorType: 'pure',
      volumeType: '',
      backendName: '',
      managementEndpoint: '',
      username: '',
      password: '',
      skipSslVerification: false
    }
  })

  const handleClose = () => {
    if (isValidating) return // Don't allow closing while validating
    
    // Check if form has been touched (has any values)
    const formValues = getValues()
    const hasChanges = formValues.name || formValues.managementEndpoint || formValues.username || formValues.password
    
    if (hasChanges && !isSubmitting) {
      setShowDiscardDialog(true)
      return
    }
    
    setSubmitError(null)
    setValidationStatus('idle')
    setValidationMessage('')
    onClose()
  }

  const handleDiscardChanges = async () => {
    setSubmitError(null)
    setValidationStatus('idle')
    setValidationMessage('')
    onClose()
  }

  const handleCancelDiscard = () => {
    setShowDiscardDialog(false)
  }

  // Poll for validation status after creating credentials
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

  // Cleanup failed creation by deleting the ArrayCreds resource
  const cleanupFailedCreation = async (name: string) => {
    try {
      console.log(`Starting cleanup for failed creation: ${name}`)
      await deleteArrayCredsWithSecretFlow(name)
      console.log(`Cleanup completed for: ${name}`)
      queryClient.invalidateQueries({ queryKey: ARRAY_CREDS_QUERY_KEY })
    } catch (error) {
      console.error('Error cleaning up failed creation:', error)
      throw error
    }
  }

  const onSubmit = async (data: FormData) => {
    setIsSubmitting(true)
    setSubmitError(null)
    setValidationStatus('idle')
    setValidationMessage('')

    try {
      await createArrayCredsWithSecretFlow(data.name, {
        ARRAY_HOSTNAME: data.managementEndpoint,
        ARRAY_USERNAME: data.username,
        ARRAY_PASSWORD: data.password,
        ARRAY_SKIP_SSL_VERIFICATION: data.skipSslVerification,
        VENDOR_TYPE: data.vendorType,
        OPENSTACK_MAPPING: {
          volumeType: data.volumeType,
          cinderBackendName: data.backendName
        }
      })

      // Start validation polling
      setIsSubmitting(false)
      setIsValidating(true)
      setValidationStatus('validating')
      setValidationMessage('Validating credentials with storage array...')

      const validationResult = await pollForValidation(data.name)
      setIsValidating(false)

      if (!validationResult.success) {
        // Validation failed - show error and cleanup
        setValidationStatus('failed')
        setValidationMessage(validationResult.message || 'Validation failed')

        try {
          await cleanupFailedCreation(data.name)
        } catch (cleanupError) {
          console.error('Cleanup failed:', cleanupError)
          setValidationMessage(
            `${validationResult.message}. Warning: Failed to clean up - please delete manually.`
          )
        }
        return // Stay in the form
      }

      // Validation succeeded
      setValidationStatus('success')
      setValidationMessage('Credentials validated successfully!')
      queryClient.invalidateQueries({ queryKey: ARRAY_CREDS_QUERY_KEY })

      // Wait a moment to show success message before closing
      await new Promise((resolve) => setTimeout(resolve, 1500))

      reset()
      setValidationStatus('idle')
      setValidationMessage('')
      onClose()
    } catch (error: any) {
      console.error('Error creating array credentials:', error)
      setIsValidating(false)
      setValidationStatus('idle')
      const errorMessage =
        error?.response?.data?.message || error?.message || 'Failed to create array credentials'
      setSubmitError(errorMessage)
      reportError(error, {
        context: 'add-array-credentials',
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
          width: { xs: '100%', sm: 700 },
          p: 3,
          backgroundColor: 'background.paper'
        }
      }}
    >
      <Box sx={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
        <Typography variant="h5" fontWeight={600} sx={{ mb: 3 }}>
          Add Array Credentials
        </Typography>

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
        <Box sx={{ mb: 3 }}>
          <Typography variant="subtitle1" fontWeight={600} sx={{ mb: 0.5 }}>
            Basic Information
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            Provide the name and vendor type for the storage array
          </Typography>

          <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr' }, gap: 2 }}>
            <Controller
              name="name"
              control={control}
              rules={{
                required: 'Name is required',
                pattern: {
                  value: /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/,
                  message: 'Name must be lowercase alphanumeric with hyphens'
                }
              }}
              render={({ field }) => (
                <TextField
                  {...field}
                  label="Name"
                  required
                  fullWidth
                  error={!!errors.name}
                  helperText={errors.name?.message || 'Unique identifier for this array'}
                />
              )}
            />

            <Controller
              name="vendorType"
              control={control}
              rules={{ required: 'Vendor type is required' }}
              render={({ field }) => (
                <FormControl fullWidth>
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
          </Box>
        </Box>

        <Divider sx={{ my: 3 }} />

        {/* OpenStack Mapping Section */}
        <Box sx={{ mb: 3 }}>
          <Typography variant="subtitle1" fontWeight={600} sx={{ mb: 0.5 }}>
            OpenStack Mapping
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            Map this array to OpenStack Cinder backend configuration
          </Typography>

          <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr' }, gap: 2 }}>
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
                />
              )}
            />
          </Box>
        </Box>

        <Divider sx={{ my: 3 }} />

        {/* Storage Array Credentials Section */}
        <Box sx={{ mb: 3 }}>
          <Typography variant="subtitle1" fontWeight={600} sx={{ mb: 0.5 }}>
            Storage Array Credentials
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            These credentials need permission to manage volumes and read array configuration
          </Typography>

          <Box sx={{ display: 'grid', gridTemplateColumns: '1fr', gap: 2, mb: 2 }}>
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
                />
              )}
            />
          </Box>

          <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr' }, gap: 2, mb: 2 }}>
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
                  helperText={errors.username?.message}
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
                  helperText={errors.password?.message}
                />
              )}
            />
          </Box>
        </Box>

        <Divider sx={{ my: 3 }} />

        {/* Connection Options Section */}
        <Box sx={{ mb: 3 }}>
          <Typography variant="subtitle1" fontWeight={600} sx={{ mb: 0.5 }}>
            Connection Options
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            Prefer TLS-secure connections. Only disable SSL verification if your environment requires it.
          </Typography>

          <Box
            sx={{
              p: 2,
              border: '1px solid',
              borderColor: 'divider',
              borderRadius: 1,
              backgroundColor: 'background.default'
            }}
          >
            <Controller
              name="skipSslVerification"
              control={control}
              render={({ field }) => (
                <Box>
                  <FormControlLabel
                    control={<Checkbox {...field} checked={field.value} />}
                    label="Allow insecure connection"
                    sx={{ mb: 0.5 }}
                  />
                  <Typography variant="caption" color="text.secondary" display="block" sx={{ ml: 4 }}>
                    Disabling verification may expose credentials in transit.
                  </Typography>
                  <Typography variant="caption" color="text.secondary" display="block" sx={{ ml: 4 }}>
                    Skip SSL verification for self-signed or lab environments.
                  </Typography>
                </Box>
              )}
            />
          </Box>
        </Box>

        {/* Action Buttons - Sticky Footer */}
        <Box
          component="form"
          onSubmit={handleSubmit(onSubmit)}
          sx={{
            mt: 'auto',
            pt: 3,
            pb: 2,
            px: 3,
            mx: -3,
            mb: -3,
            borderTop: '1px solid',
            borderColor: 'divider',
            backgroundColor: 'background.paper',
            display: 'flex',
            gap: 2,
            justifyContent: 'flex-end'
          }}
        >
          <Button
            variant="outlined"
            onClick={handleClose}
            disabled={isSubmitting || isValidating}
          >
            Cancel
          </Button>
          <Button
            type="submit"
            variant="contained"
            disabled={isSubmitting || isValidating || validationStatus === 'success'}
            startIcon={isSubmitting || isValidating ? <CircularProgress size={20} /> : null}
          >
            {isSubmitting
              ? 'Creating...'
              : isValidating
                ? 'Validating...'
                : validationStatus === 'success'
                  ? 'Success'
                  : 'Save Credentials'}
          </Button>
        </Box>
      </Box>

      {/* Discard Changes Dialog */}
      <ConfirmationDialog
        open={showDiscardDialog}
        onClose={handleCancelDiscard}
        title="Discard Changes?"
        message="Are you sure you want to leave? Any unsaved changes will be lost."
        actionLabel="Leave"
        actionColor="warning"
        actionVariant="outlined"
        onConfirm={handleDiscardChanges}
      />
    </Drawer>
  )
}
