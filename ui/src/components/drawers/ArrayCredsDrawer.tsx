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
} from '../../hooks/api/useArrayCredsQuery'
import { useErrorHandler } from '../../hooks/useErrorHandler'

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

export default function ArrayCredsDrawer({
  open,
  onClose,
  arrayCreds,
}: ArrayCredsDrawerProps) {
  const isEditMode = !!arrayCreds
  const createMutation = useCreateArrayCredsMutation()
  const updateMutation = useUpdateArrayCredsMutation()
  const { reportError } = useErrorHandler()

  const [formData, setFormData] = useState<ArrayCredsFormData>({
    name: '',
    vendorType: 'pure',
    volumeType: '',
    cinderBackendName: '',
    cinderBackendPool: '',
    managementEndpoint: '',
    username: '',
    password: '',
    apiToken: '',
  })

  const [errors, setErrors] = useState<Record<string, string>>({})
  const [showCredentials, setShowCredentials] = useState(false)

  useEffect(() => {
    if (arrayCreds) {
      setFormData({
        name: arrayCreds.metadata.name,
        vendorType: arrayCreds.spec.vendorType,
        volumeType: arrayCreds.spec.openStackMapping.volumeType,
        cinderBackendName: arrayCreds.spec.openStackMapping.cinderBackendName,
        cinderBackendPool: arrayCreds.spec.openStackMapping.cinderBackendPool || '',
        managementEndpoint: '',
        username: '',
        password: '',
        apiToken: '',
      })
      setShowCredentials(!!arrayCreds.spec.secretRef.name)
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
        apiToken: '',
      })
      setShowCredentials(false)
    }
    setErrors({})
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

    setErrors(newErrors)
    return Object.keys(newErrors).length === 0
  }

  const handleSubmit = async () => {
    if (!validate()) {
      return
    }

    try {
      if (isEditMode) {
        await updateMutation.mutateAsync({
          name: formData.name,
          data: formData,
        })
      } else {
        await createMutation.mutateAsync(formData)
      }
      onClose()
    } catch (error) {
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

  const isLoading = createMutation.isPending || updateMutation.isPending

  return (
    <StyledDrawer open={open} onClose={onClose}>
      <Header
        title={isEditMode ? 'Edit Array Credentials' : 'Add Array Credentials'}
      />

      <Box sx={{ p: 3, flexGrow: 1, overflow: 'auto' }}>
        {arrayCreds?.spec.autoDiscovered && (
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
          helperText={errors.name || 'Unique identifier for this array'}
          disabled={isEditMode}
          required
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

        <TextField
          label="Backend Pool (Optional)"
          value={formData.cinderBackendPool}
          onChange={handleChange('cinderBackendPool')}
          helperText="Cinder backend pool name (if applicable)"
          fullWidth
          sx={{ mb: 2 }}
        />

        <Divider sx={{ my: 3 }} />

        <Box sx={{ mb: 2 }}>
          <FormControlLabel
            control={
              <Checkbox
                checked={showCredentials}
                onChange={(e) => setShowCredentials(e.target.checked)}
              />
            }
            label="Configure Storage Array Credentials"
          />
        </Box>

        {showCredentials && (
          <>
            <Typography variant="h6" sx={{ mb: 2 }}>
              Storage Array Credentials
            </Typography>

            <Alert severity="info" sx={{ mb: 2 }}>
              Provide credentials to connect to the storage array management interface.
              These are used for advanced operations like snapshot management.
            </Alert>

            <TextField
              label="Management Endpoint"
              value={formData.managementEndpoint}
              onChange={handleChange('managementEndpoint')}
              helperText="Storage array management IP or hostname"
              fullWidth
              sx={{ mb: 2 }}
            />

            <TextField
              label="Username"
              value={formData.username}
              onChange={handleChange('username')}
              helperText="Storage array username"
              fullWidth
              sx={{ mb: 2 }}
            />

            <TextField
              label="Password"
              type="password"
              value={formData.password}
              onChange={handleChange('password')}
              helperText="Storage array password"
              fullWidth
              sx={{ mb: 2 }}
            />

            <TextField
              label="API Token (Optional)"
              value={formData.apiToken}
              onChange={handleChange('apiToken')}
              helperText="API token for token-based authentication"
              fullWidth
              sx={{ mb: 2 }}
            />
          </>
        )}
      </Box>

      <Footer
        onClose={onClose}
        onSubmit={handleSubmit}
        submitButtonLabel={isEditMode ? 'Update' : 'Create'}
        submitting={isLoading}
      />
    </StyledDrawer>
  )
}
