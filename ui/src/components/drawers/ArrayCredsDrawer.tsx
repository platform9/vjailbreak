import { useState, useEffect } from 'react'
import {
  Box,
  Typography,
  Alert,
  MenuItem,
  FormControlLabel,
  Checkbox,
  Divider,
} from '@mui/material'
import { StyledDrawer } from '../forms/StyledDrawer'
import TextField from '../forms/TextField'
import Footer from '../forms/Footer'
import Header from '../forms/Header'
import { ArrayCreds, ArrayCredsFormData } from '../../api/array-creds'
import {
  useCreateArrayCredsMutation,
  useUpdateArrayCredsMutation,
  ARRAY_CREDS_QUERY_KEY,
} from '../../hooks/api/useArrayCredsQuery'
import { useErrorHandler } from '../../hooks/useErrorHandler'
import { useQueryClient } from '@tanstack/react-query'

interface ArrayCredsDrawerProps {
  open: boolean
  onClose: () => void
  arrayCreds?: ArrayCreds | null
}

const VENDOR_TYPES = [
  { value: 'pure', label: 'Pure Storage' },
  { value: 'ontap', label: 'NetApp ONTAP' },
  { value: 'hpalletra', label: 'HPE Alletra' },
]

// Map display names to backend values
const normalizeVendorType = (vendorType: string): string => {
  const mapping: Record<string, string> = {
    'Pure Storage': 'pure'
  }
  return mapping[vendorType] || vendorType.toLowerCase()
}

export default function ArrayCredsDrawer({
  open,
  onClose,
  arrayCreds,
}: ArrayCredsDrawerProps) {
  const isEditMode = !!arrayCreds
  const createMutation = useCreateArrayCredsMutation()
  const updateMutation = useUpdateArrayCredsMutation()
  const { reportError } = useErrorHandler()
  const queryClient = useQueryClient()

  const [formData, setFormData] = useState<ArrayCredsFormData>({
    name: '',
    vendorType: 'pure',
    volumeType: '',
    cinderBackendName: '',
    cinderBackendPool: '',
    managementEndpoint: '',
    username: '',
    password: '',
    skipSSLVerification: false,
  })

  const [errors, setErrors] = useState<Record<string, string>>({})
  const [isValidating, setIsValidating] = useState(false)
  const [validationStatus, setValidationStatus] = useState<{
    status: 'validating' | 'success' | 'failed' | null
    message?: string
  }>({ status: null })

  useEffect(() => {
    if (arrayCreds) {
      setFormData({
        name: arrayCreds.metadata.name,
        vendorType: normalizeVendorType(arrayCreds.spec.vendorType),
        volumeType: arrayCreds.spec.openstackMapping?.volumeType || '',
        cinderBackendName: arrayCreds.spec.openstackMapping?.cinderBackendName || '',
        cinderBackendPool: arrayCreds.spec.openstackMapping?.cinderBackendPool || '',
        managementEndpoint: '',
        username: '',
        password: '',
        skipSSLVerification: false,
      })
      // Credentials section is always visible
    } else {
      setFormData({
        name: '',
        vendorType: 'pure',
        volumeType: '',
        cinderBackendName: '',
        cinderBackendPool: '',
        managementEndpoint: '',
        username: '',
        password: '',
        skipSSLVerification: false,
      })
      setValidationStatus({ status: null })
    }
    setErrors({})
    setValidationStatus({ status: null })
  }, [arrayCreds, open])

  const handleChange = (field: keyof ArrayCredsFormData) => (
    event: React.ChangeEvent<HTMLInputElement>
  ) => {
    setFormData((prev) => ({
      ...prev,
      [field]: event.target.value,
    }))
    // Clear error for this field
    if (errors[field]) {
      setErrors((prev) => {
        const newErrors = { ...prev }
        delete newErrors[field]
        return newErrors
      })
    }
  }

  const validate = (): boolean => {
    const newErrors: Record<string, string> = {}

    if (!formData.name.trim()) {
      newErrors.name = 'Name is required'
    } else if (!/^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/.test(formData.name)) {
      newErrors.name = 'Name must be lowercase alphanumeric with hyphens'
    }

    if (!formData.vendorType) {
      newErrors.vendorType = 'Vendor type is required'
    }

    if (!formData.volumeType.trim()) {
      newErrors.volumeType = 'Volume type is required'
    }

    if (!formData.cinderBackendName.trim()) {
      newErrors.cinderBackendName = 'Backend name is required'
    }

    // Validate credential fields
    if (!formData.managementEndpoint?.trim()) {
      newErrors.managementEndpoint = 'Management endpoint is required'
    }
    if (!formData.username?.trim()) {
      newErrors.username = 'Username is required'
    }
    if (!formData.password?.trim()) {
      newErrors.password = 'Password is required'
    }

    setErrors(newErrors)
    return Object.keys(newErrors).length === 0
  }

  // Check if form is valid for enabling/disabling submit button
  const isFormValid = (): boolean => {
    // Basic required fields
    const hasBasicFields = 
      formData.name.trim() !== '' &&
      /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/.test(formData.name) &&
      formData.vendorType !== '' &&
      formData.volumeType.trim() !== '' &&
      formData.cinderBackendName.trim() !== ''

    // Check credential fields too
    const hasCredentialFields = 
      formData.managementEndpoint?.trim() !== '' &&
      formData.username?.trim() !== '' &&
      formData.password?.trim() !== ''
    
    return hasBasicFields && hasCredentialFields
  }

  const handleSubmit = async () => {
    if (!validate()) {
      return
    }

    setValidationStatus({ status: null })

    try {
      let result: ArrayCreds
      if (isEditMode) {
        // Use the original name from arrayCreds, not the form data
        const originalName = arrayCreds!.metadata.name
        result = await updateMutation.mutateAsync({
          name: originalName,
          data: formData,
        })
      } else {
        result = await createMutation.mutateAsync(formData)
      }

      // Wait for validation (credentials are always required now)
      if (formData.managementEndpoint || formData.username || formData.password) {
        setIsValidating(true)
        setValidationStatus({ status: 'validating', message: 'Validating credentials...' })
        
        const validationResult = await pollForValidation(result.metadata.name)
        setIsValidating(false)
        
        // If validation failed, clean up and stay in form
        if (!validationResult.success) {
          setValidationStatus({ status: 'failed', message: validationResult.message })
          try {
            if (isEditMode) {
              // For edit mode, just clean up the secret and secretRef
              await cleanupFailedCredentials(result.metadata.name)
            } else {
              // For create mode, delete the entire ArrayCreds resource
              await cleanupFailedCreation(result.metadata.name)
            }
          } catch (cleanupError) {
            console.error('Cleanup failed, but continuing:', cleanupError)
            // Update status to show cleanup also failed
            setValidationStatus({ 
              status: 'failed', 
              message: `${validationResult.message}. Warning: Failed to clean up - please delete manually.` 
            })
          }
          return // Stay in the form
        }
        
        setValidationStatus({ status: 'success', message: 'Credentials validated successfully!' })
        // Invalidate queries to refresh the table with new validation status
        queryClient.invalidateQueries({ queryKey: [ARRAY_CREDS_QUERY_KEY] })
        // Wait a moment to show success message before closing
        await new Promise(resolve => setTimeout(resolve, 1000))
      }

      // Invalidate queries to refresh the table
      queryClient.invalidateQueries({ queryKey: [ARRAY_CREDS_QUERY_KEY] })
      onClose()
    } catch (error) {
      setIsValidating(false)
      setValidationStatus({ status: null })
      reportError(
        error as Error,
        {
          context: isEditMode
            ? 'Failed to update array credentials'
            : 'Failed to create array credentials'
        }
      )
    }
  }

  const pollForValidation = async (name: string, maxAttempts = 30, intervalMs = 2000): Promise<{ success: boolean; message?: string }> => {
    const { getArrayCredsById } = await import('../../api/array-creds')
    
    for (let attempt = 0; attempt < maxAttempts; attempt++) {
      await new Promise(resolve => setTimeout(resolve, intervalMs))
      
      try {
        const updated = await getArrayCredsById(name)
        const status = updated.status?.arrayValidationStatus
        
        // Check if validation succeeded
        if (status === 'Succeeded') {
          return { success: true }
        }
        
        // Check if validation failed
        if (status && status !== 'AwaitingCredentials' && status !== 'Validating') {
          return { 
            success: false, 
            message: updated.status?.arrayValidationMessage || `Validation failed with status: ${status}` 
          }
        }
      } catch (error) {
        console.error('Error polling for validation:', error)
      }
    }
    
    // Timeout - validation took too long
    return { success: false, message: 'Validation timed out after 60 seconds' }
  }

  const cleanupFailedCreation = async (name: string) => {
    try {
      const { deleteArrayCreds, deleteArrayCredsSecret } = await import('../../api/array-creds')
      
      console.log(`Starting cleanup for failed creation: ${name}`)
      
      // Delete the secret first
      const secretName = `${name}-secret`
      await deleteArrayCredsSecret(secretName)
      console.log(`Secret deletion completed for: ${secretName}`)
      
      // Delete the entire ArrayCreds resource
      await deleteArrayCreds(name)
      console.log(`ArrayCreds deletion completed for: ${name}`)
      
      // Invalidate queries to refresh the table
      queryClient.invalidateQueries({ queryKey: [ARRAY_CREDS_QUERY_KEY] })
    } catch (error) {
      console.error('Error cleaning up failed creation:', error)
      // Re-throw to let the caller know cleanup failed
      throw error
    }
  }

  const cleanupFailedCredentials = async (name: string) => {
    try {
      const { deleteArrayCredsSecret, getArrayCredsById } = await import('../../api/array-creds')
      const axios = (await import('axios')).default
      
      console.log(`Starting cleanup for failed credentials: ${name}`)
      
      // Delete the secret
      const secretName = `${name}-secret`
      await deleteArrayCredsSecret(secretName)
      console.log(`Secret deletion completed for: ${secretName}`)
      
      // Get the current ArrayCreds to update it properly
      const existing = await getArrayCredsById(name)
      console.log(`Current secretRef:`, existing.spec.secretRef)
      
      // Remove secretRef by setting name to empty string
      if (!existing.spec.secretRef) {
        existing.spec.secretRef = {}
      }
      existing.spec.secretRef.name = ''
      existing.spec.secretRef.namespace = ''
      
      // Update the ArrayCreds resource
      const NAMESPACE = 'migration-system'
      const ARRAY_CREDS_API_PATH = `/apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/${NAMESPACE}/arraycreds`
      
      const authToken = import.meta.env.VITE_API_TOKEN
      const axiosInstance = axios.create({
        headers: {
          'Content-Type': 'application/json;charset=UTF-8',
          ...(authToken && { Authorization: `Bearer ${authToken}` }),
        },
      })
      
      await axiosInstance.put(`${ARRAY_CREDS_API_PATH}/${name}`, existing)
      console.log(`Successfully cleared secretRef for: ${name}`)
    } catch (error) {
      console.error('Error cleaning up failed credentials:', error)
      // Re-throw to let the caller know cleanup failed
      throw error
    }
  }

  const isLoading = createMutation.isPending || updateMutation.isPending || isValidating

  const isAutoDiscovered = arrayCreds?.metadata.labels?.['vjailbreak.k8s.pf9.io/auto-discovered'] === 'true'

  return (
    <StyledDrawer open={open} onClose={onClose} anchor="right">
      <Header
        title={isEditMode ? 'Edit Array Credentials' : 'Add Array Credentials'}
      />

      <Box sx={{ p: 3, flexGrow: 1, overflow: 'auto' }}>
        {isAutoDiscovered && (
          <Alert severity="info" sx={{ mb: 3 }}>
            This array was auto-discovered from OpenStack Cinder. You can update the
            credentials and vendor type as needed.
          </Alert>
        )}

        <Typography variant="h6" sx={{ mb: 2 }}>
          Basic Information
        </Typography>

        <TextField
          label="Name"
          value={formData.name}
          onChange={handleChange('name')}
          error={!!errors.name}
          helperText={errors.name || (isEditMode ? 'Name cannot be changed after creation' : 'Unique identifier for this array')}
          required
          disabled={isEditMode}
          fullWidth
          sx={{ mb: 2 }}
        />

        <TextField
          select
          label="Vendor Type"
          value={formData.vendorType}
          onChange={handleChange('vendorType')}
          error={!!errors.vendorType}
          helperText={errors.vendorType}
          required
          fullWidth
          sx={{ mb: 2 }}
        >
          {VENDOR_TYPES.map((option) => (
            <MenuItem key={option.value} value={option.value}>
              {option.label}
            </MenuItem>
          ))}
        </TextField>

        <Divider sx={{ my: 3 }} />

        <Typography variant="h6" sx={{ mb: 2 }}>
          OpenStack Mapping
        </Typography>

        <TextField
          label="Volume Type"
          value={formData.volumeType}
          onChange={handleChange('volumeType')}
          error={!!errors.volumeType}
          helperText={errors.volumeType || 'Cinder volume type name'}
          required
          fullWidth
          sx={{ mb: 2 }}
        />

        <TextField
          label="Backend Name"
          value={formData.cinderBackendName}
          onChange={handleChange('cinderBackendName')}
          error={!!errors.cinderBackendName}
          helperText={errors.cinderBackendName || 'Cinder backend name'}
          required
          fullWidth
          sx={{ mb: 2 }}
        />

        <Divider sx={{ my: 3 }} />

        <Typography variant="h6" sx={{ mb: 2 }}>
          Storage Array Credentials
        </Typography>

        {validationStatus.status === 'validating' && (
          <Alert severity="info" sx={{ mb: 2 }}>
            {validationStatus.message}
          </Alert>
        )}

        {validationStatus.status === 'success' && (
          <Alert severity="success" sx={{ mb: 2 }}>
            {validationStatus.message}
          </Alert>
        )}

        {validationStatus.status === 'failed' && (
          <Alert severity="error" sx={{ mb: 2 }}>
            {validationStatus.message}
          </Alert>
        )}

        {!validationStatus.status && (
          <Alert severity="info" sx={{ mb: 2 }}>
            Provide credentials to connect to the storage array management interface.
            These are used for advanced operations like volume management.
          </Alert>
        )}

        <TextField
          label="Management Endpoint"
          value={formData.managementEndpoint}
          onChange={handleChange('managementEndpoint')}
          error={!!errors.managementEndpoint}
          helperText={errors.managementEndpoint || 'Storage array management IP or hostname'}
          required
          fullWidth
          sx={{ mb: 2 }}
        />

        <TextField
          label="Username"
          value={formData.username}
          onChange={handleChange('username')}
          error={!!errors.username}
          helperText={errors.username || 'Storage array username'}
          required
          fullWidth
          sx={{ mb: 2 }}
        />

        <TextField
          label="Password"
          type="password"
          value={formData.password}
          onChange={handleChange('password')}
          error={!!errors.password}
          helperText={errors.password || 'Storage array password'}
          required
          fullWidth
          sx={{ mb: 2 }}
        />

        <FormControlLabel
          control={
            <Checkbox
              checked={formData.skipSSLVerification}
              onChange={(e) => setFormData(prev => ({ ...prev, skipSSLVerification: e.target.checked }))}
            />
          }
          label="Skip SSL Certificate Verification"
          sx={{ mb: 2 }}
        />
      </Box>

      <Footer
        onClose={onClose}
        onSubmit={handleSubmit}
        submitButtonLabel={isEditMode ? 'Update' : 'Create'}
        submitting={isLoading}
        disableSubmit={!isFormValid()}
      />
    </StyledDrawer>
  )
}
