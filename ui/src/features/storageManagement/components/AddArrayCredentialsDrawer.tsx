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
import InfoIcon from '@mui/icons-material/Info'
import { useForm, Controller } from 'react-hook-form'
import { useQueryClient } from '@tanstack/react-query'
import { ARRAY_CREDS_QUERY_KEY } from 'src/hooks/api/useArrayCredentialsQuery'
import { createArrayCredsWithSecretFlow } from 'src/api/helpers'
import { ARRAY_VENDOR_TYPES } from 'src/api/array-creds/model'
import { useErrorHandler } from 'src/hooks/useErrorHandler'

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

export default function AddArrayCredentialsDrawer({
  open,
  onClose
}: AddArrayCredentialsDrawerProps) {
  const { reportError } = useErrorHandler({ component: 'AddArrayCredentialsDrawer' })
  const queryClient = useQueryClient()
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [submitError, setSubmitError] = useState<string | null>(null)

  const {
    control,
    handleSubmit,
    formState: { errors },
    reset
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
    reset()
    setSubmitError(null)
    onClose()
  }

  const onSubmit = async (data: FormData) => {
    setIsSubmitting(true)
    setSubmitError(null)

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

      queryClient.invalidateQueries({ queryKey: ARRAY_CREDS_QUERY_KEY })
      handleClose()
    } catch (error: any) {
      console.error('Error creating array credentials:', error)
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
          width: { xs: '100%', sm: 500 },
          p: 3,
          backgroundColor: 'background.paper'
        }
      }}
    >
      <Box component="form" onSubmit={handleSubmit(onSubmit)} sx={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
        <Typography variant="h5" fontWeight={600} sx={{ mb: 3 }}>
          Add Array Credentials
        </Typography>

        {submitError && (
          <Alert severity="error" sx={{ mb: 2 }} onClose={() => setSubmitError(null)}>
            {submitError}
          </Alert>
        )}

        {/* Basic Information Section */}
        <Typography variant="subtitle1" fontWeight={600} sx={{ mb: 2 }}>
          Basic Information
        </Typography>

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
          <Button variant="outlined" onClick={handleClose} disabled={isSubmitting}>
            CANCEL
          </Button>
          <Button
            type="submit"
            variant="contained"
            disabled={isSubmitting}
            startIcon={isSubmitting ? <CircularProgress size={20} /> : null}
          >
            {isSubmitting ? 'CREATING...' : 'CREATE'}
          </Button>
        </Box>
      </Box>
    </Drawer>
  )
}
