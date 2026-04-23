import { Box, Typography, Alert } from '@mui/material'
import InfoIcon from '@mui/icons-material/Info'
import { FieldErrors, useWatch } from 'react-hook-form'
import { ARRAY_VENDOR_TYPES } from 'src/api/array-creds/model'
import { RHFSelect, RHFToggleField, RHFTextField } from 'src/shared/components/forms'
import { FormGrid } from 'src/components/design-system'

export type ArrayCredentialsFormData = {
  name: string
  vendorType: string
  volumeType: string
  backendName: string
  managementEndpoint: string
  username: string
  password: string
  skipSslVerification: boolean
  netAppSvm?: string
  netAppFlexVol?: string
}

export type ArrayCredentialsFormFieldsProps = {
  mode: 'add' | 'edit'
  errors: FieldErrors<ArrayCredentialsFormData>
  isAutoDiscovered?: boolean
}

export default function ArrayCredentialsFormFields({
  mode,
  errors,
  isAutoDiscovered
}: ArrayCredentialsFormFieldsProps) {
  const isAdd = mode === 'add'
  const vendorType = useWatch<ArrayCredentialsFormData>({ name: 'vendorType' }) as string | undefined
  const isNetApp = vendorType === 'netapp'

  return (
    <>
      {mode === 'edit' && isAutoDiscovered && (
        <Alert
          severity="info"
          icon={<InfoIcon />}
          sx={{
            mb: 3,
            backgroundColor: 'rgba(33, 150, 243, 0.1)',
            '& .MuiAlert-message': { fontSize: '0.875rem' }
          }}
        >
          This array was auto-discovered from PCD. You can update the credentials and vendor type as
          needed.
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

        <FormGrid minWidth={280}>
          <RHFTextField
            name="name"
            label="Name"
            disabled={!isAdd}
            required={isAdd}
            rules={
              isAdd
                ? {
                    required: 'Name is required',
                    pattern: {
                      value: /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/,
                      message: 'Name must be lowercase alphanumeric with hyphens'
                    }
                  }
                : undefined
            }
            labelProps={{
              tooltip: isAdd
                ? 'Unique identifier for this array'
                : 'Name cannot be changed after creation'
            }}
            helperText={errors.name?.message}
            error={!!errors.name}
          />

          <RHFSelect
            name="vendorType"
            label="Vendor Type"
            rules={{ required: 'Vendor type is required' }}
            options={ARRAY_VENDOR_TYPES.filter((vendor) => vendor.value !== 'unsupported').map(
              (vendor) => ({
                label: vendor.label,
                value: vendor.value
              })
            )}
          />
        </FormGrid>
      </Box>

      <Box sx={{ my: 1 }} />

      {/* OpenStack Mapping Section */}
      <Box sx={{ mb: 3 }}>
        <Typography variant="subtitle1" fontWeight={600} sx={{ mb: 0.5 }}>
          OpenStack Mapping
        </Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          Map this array to PCD backend configuration
        </Typography>

        <FormGrid minWidth={280}>
          <RHFTextField
            name="volumeType"
            label="Volume Type"
            required
            rules={{ required: 'Volume type is required' }}
            labelProps={{ tooltip: 'Cinder volume type name' }}
            helperText={errors.volumeType?.message}
            error={!!errors.volumeType}
          />

          <RHFTextField
            name="backendName"
            label="Backend Name"
            required
            rules={{ required: 'Backend name is required' }}
            labelProps={{ tooltip: 'Cinder backend name' }}
            helperText={errors.backendName?.message}
            error={!!errors.backendName}
          />
        </FormGrid>
      </Box>

      <Box sx={{ my: 1 }} />

      {/* NetApp-specific targeting (optional at create time) */}
      {isNetApp && (
        <>
          <Box sx={{ mb: 3 }}>
            <Typography variant="subtitle1" fontWeight={600} sx={{ mb: 0.5 }}>
              NetApp Target (Optional)
            </Typography>
            <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
              Specify the SVM and FlexVol where migration LUNs will be created. Leave blank to let
              vJailbreak discover the available SVMs and FlexVols; you will then pick from a list
              once credentials are validated.
            </Typography>

            <FormGrid minWidth={280}>
              <RHFTextField
                name="netAppSvm"
                label="SVM"
                labelProps={{ tooltip: 'NetApp Storage Virtual Machine name' }}
                helperText={errors.netAppSvm?.message}
                error={!!errors.netAppSvm}
              />

              <RHFTextField
                name="netAppFlexVol"
                label="FlexVol"
                labelProps={{ tooltip: 'FlexVol name within the SVM where LUNs will be created' }}
                helperText={errors.netAppFlexVol?.message}
                error={!!errors.netAppFlexVol}
              />
            </FormGrid>
          </Box>

          <Box sx={{ my: 1 }} />
        </>
      )}

      {/* Storage Array Credentials Section */}
      <Box sx={{ mb: 3 }}>
        <Typography variant="subtitle1" fontWeight={600} sx={{ mb: 0.5 }}>
          Storage Array Credentials
        </Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          These credentials need permission to manage volumes and read array configuration
        </Typography>

        {mode === 'edit' && (
          <Alert
            severity="info"
            icon={<InfoIcon />}
            sx={{
              mb: 2,
              backgroundColor: 'rgba(33, 150, 243, 0.1)',
              '& .MuiAlert-message': { fontSize: '0.875rem' }
            }}
          >
            Leave fields empty to keep existing credentials.
          </Alert>
        )}

        <Box sx={{ display: 'grid', gridTemplateColumns: '1fr', gap: 2, mb: 2 }}>
          <RHFTextField
            name="managementEndpoint"
            label="Management Endpoint"
            required={isAdd}
            rules={isAdd ? { required: 'Management endpoint is required' } : undefined}
            labelProps={{ tooltip: 'Storage array management IP or hostname' }}
            helperText={isAdd ? errors.managementEndpoint?.message : undefined}
            error={isAdd ? !!errors.managementEndpoint : undefined}
          />
        </Box>

        <FormGrid minWidth={280}>
          <RHFTextField
            name="username"
            label="Username"
            required={isAdd}
            rules={isAdd ? { required: 'Username is required' } : undefined}
            helperText={isAdd ? errors.username?.message : undefined}
            error={isAdd ? !!errors.username : undefined}
            labelProps={isAdd ? undefined : { tooltip: 'Storage array username' }}
          />

          <RHFTextField
            name="password"
            label="Password"
            type="password"
            required={isAdd}
            rules={isAdd ? { required: 'Password is required' } : undefined}
            helperText={isAdd ? errors.password?.message : undefined}
            error={isAdd ? !!errors.password : undefined}
            labelProps={isAdd ? undefined : { tooltip: 'Storage array password' }}
          />
        </FormGrid>
      </Box>

      <Box sx={{ my: 1 }} />

      {/* Connection Options Section */}
      <Box sx={{ mb: 3 }}>
        <Typography variant="subtitle1" fontWeight={600} sx={{ mb: 0.5 }}>
          Connection Options
        </Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          Prefer TLS-secure connections. Only disable SSL verification if your environment requires
          it.
        </Typography>

        <RHFToggleField
          name="skipSslVerification"
          label="Allow insecure connection"
          helperText="Disabling verification may expose credentials in transit."
          description="Skip SSL verification for self-signed or lab environments."
          containerProps={{ sx: { mb: 5 } }}
        />
      </Box>
    </>
  )
}
