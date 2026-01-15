import React, { useCallback, useEffect, useState } from 'react'
import {
  Alert,
  Box,
  Collapse,
  IconButton,
  InputAdornment,
  MenuItem,
  Tooltip
} from '@mui/material'
import { Visibility, VisibilityOff } from '@mui/icons-material'
import CheckIcon from '@mui/icons-material/Check'
import InfoOutlined from '@mui/icons-material/InfoOutlined'

import { useForm } from 'react-hook-form'

import {
  DrawerShell,
  DrawerHeader,
  DrawerFooter,
  ActionButton,
  FormGrid,
  OperationStatus,
  Section,
  SectionHeader,
  SurfaceCard
} from 'src/components'
import { DesignSystemForm, RHFTextField, RHFToggleField } from 'src/shared/components/forms'

import {
  createArrayCredsWithSecretFlow,
  deleteArrayCredsWithSecretFlow
} from 'src/api/helpers'
import { getArrayCredentials } from 'src/api/array-creds/arrayCreds'
import { ARRAY_VENDOR_TYPES } from 'src/api/array-creds/model'

import axios from 'axios'

import { useArrayCredentialsQuery } from 'src/hooks/api/useArrayCredentialsQuery'
import { useInterval } from 'src/hooks/useInterval'
import { useErrorHandler } from 'src/hooks/useErrorHandler'
import { useAmplitude } from 'src/hooks/useAmplitude'

import { isValidName } from 'src/utils'
import { THREE_SECONDS } from 'src/constants'
import { AMPLITUDE_EVENTS } from 'src/types/amplitude'

interface ArrayCredentialsDrawerProps {
  open: boolean
  onClose: () => void
}

interface ArrayCredentialFormValues {
  credentialName: string
  vendorType: string
  hostname: string
  username: string
  password: string
  skipSSLVerification: boolean
  volumeType: string
  cinderBackendName: string
  cinderBackendPool: string
  cinderHost: string
}

const defaultValues: ArrayCredentialFormValues = {
  credentialName: '',
  vendorType: 'pure',
  hostname: '',
  username: '',
  password: '',
  skipSSLVerification: false,
  volumeType: '',
  cinderBackendName: '',
  cinderBackendPool: '',
  cinderHost: ''
}

export default function ArrayCredentialsDrawer({ open, onClose }: ArrayCredentialsDrawerProps) {
  const { reportError } = useErrorHandler({ component: 'ArrayCredentialsDrawer' })
  const { track } = useAmplitude({ component: 'ArrayCredentialsDrawer' })
  const { refetch: refetchArrayCreds } = useArrayCredentialsQuery()

  const form = useForm<ArrayCredentialFormValues>({
    defaultValues,
    mode: 'onChange',
    reValidateMode: 'onChange'
  })

  const {
    watch,
    reset,
    formState: { isValid }
  } = form

  const values = watch()
  const {
    credentialName: credentialNameValue,
    vendorType: vendorTypeValue,
    hostname: hostnameValue,
    username: usernameValue,
    password: passwordValue,
    skipSSLVerification: skipSSLVerificationValue
  } = values

  const formId = 'array-credentials-form'

  const [showPassword, setShowPassword] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [validatingArrayCreds, setValidatingArrayCreds] = useState(false)
  const [arrayCredsValidated, setArrayCredsValidated] = useState<boolean | null>(null)
  const [formError, setFormError] = useState<string | null>(null)
  const [createdCredentialName, setCreatedCredentialName] = useState<string | null>(null)

  const shouldPollArrayCreds = Boolean(createdCredentialName && validatingArrayCreds)

  const handleClickShowPassword = useCallback(() => setShowPassword((p) => !p), [])
  const handleMouseDownPassword = useCallback((e: React.MouseEvent<HTMLButtonElement>) => {
    e.preventDefault()
  }, [])

  const resetDrawerState = useCallback(() => {
    reset(defaultValues)
    setCreatedCredentialName(null)
    setValidatingArrayCreds(false)
    setArrayCredsValidated(null)
    setFormError(null)
    setSubmitting(false)
    setShowPassword(false)

    onClose()
  }, [onClose, reset])

  const closeDrawer = useCallback(() => {
    if (createdCredentialName) {
      deleteArrayCredsWithSecretFlow(createdCredentialName).catch((err) => {
        console.error(`Error deleting cancelled credential: ${createdCredentialName}`, err)
      })
    }

    resetDrawerState()
  }, [createdCredentialName, resetDrawerState])

  useEffect(() => {
    setArrayCredsValidated(null)
    setFormError(null)
  }, [
    credentialNameValue,
    vendorTypeValue,
    hostnameValue,
    usernameValue,
    passwordValue,
    skipSSLVerificationValue
  ])

  const handleValidationStatus = useCallback(
    (status: string, message?: string) => {
      if (status === 'Succeeded') {
        setArrayCredsValidated(true)
        setValidatingArrayCreds(false)

        track(AMPLITUDE_EVENTS.CREDENTIALS_ADDED, {
          credentialType: 'array',
          credentialName: createdCredentialName,
          stage: 'validation_success'
        })

        setTimeout(() => {
          refetchArrayCreds()
          resetDrawerState()
        }, 1500)
      } else if (status === 'Failed') {
        setArrayCredsValidated(false)
        setValidatingArrayCreds(false)
        setFormError(message || 'Validation failed')

        track(AMPLITUDE_EVENTS.CREDENTIALS_FAILED, {
          credentialType: 'array',
          credentialName: createdCredentialName,
          errorMessage: message || 'Validation failed',
          stage: 'validation'
        })

        reportError(
          new Error(`Storage array credential validation failed: ${message || 'Unknown reason'}`),
          {
            context: 'array-validation-failure',
            metadata: {
              credentialName: createdCredentialName,
              validationMessage: message,
              action: 'array-validation-failed'
            }
          }
        )

        if (createdCredentialName) {
          deleteArrayCredsWithSecretFlow(createdCredentialName).catch((deleteErr) => {
            console.error(`Error deleting failed credential: ${createdCredentialName}`, deleteErr)
            reportError(deleteErr as Error, {
              context: 'array-credential-deletion',
              metadata: {
                credentialName: createdCredentialName,
                action: 'delete-failed-credential'
              }
            })
          })
        }
      }

      setSubmitting(false)
    },
    [createdCredentialName, refetchArrayCreds, reportError, resetDrawerState, track]
  )

  useInterval(
    async () => {
      if (!createdCredentialName) return
      try {
        const response = await getArrayCredentials(createdCredentialName)
        if (response?.status?.arrayValidationStatus) {
          handleValidationStatus(
            response.status.arrayValidationStatus,
            response.status.arrayValidationMessage
          )
        }
      } catch (err) {
        console.error('Error validating storage array credentials', err)
        reportError(err as Error, {
          context: 'array-validation-polling',
          metadata: {
            credentialName: createdCredentialName,
            action: 'array-validation-status-polling'
          }
        })
        setFormError('Error validating storage array credentials')
        setValidatingArrayCreds(false)
        setSubmitting(false)
      }
    },
    THREE_SECONDS,
    shouldPollArrayCreds
  )

  const onSubmit = useCallback(
    async (vals: ArrayCredentialFormValues) => {
      try {
        setSubmitting(true)
        setValidatingArrayCreds(true)
        setFormError(null)

        track(AMPLITUDE_EVENTS.CREDENTIALS_ADDED, {
          credentialType: 'array',
          credentialName: vals.credentialName,
          vendorType: vals.vendorType,
          stage: 'creation_start'
        })

        const openstackMapping =
          vals.volumeType || vals.cinderBackendName
            ? {
                volumeType: vals.volumeType,
                cinderBackendName: vals.cinderBackendName,
                cinderBackendPool: vals.cinderBackendPool || undefined,
                cinderHost: vals.cinderHost || undefined
              }
            : undefined

        const response = await createArrayCredsWithSecretFlow(vals.credentialName, {
          ARRAY_HOSTNAME: vals.hostname,
          ARRAY_USERNAME: vals.username,
          ARRAY_PASSWORD: vals.password,
          ARRAY_SKIP_SSL_VERIFICATION: vals.skipSSLVerification,
          VENDOR_TYPE: vals.vendorType,
          ...(openstackMapping && { OPENSTACK_MAPPING: openstackMapping })
        })

        setCreatedCredentialName(response.metadata.name)

        track(AMPLITUDE_EVENTS.CREDENTIALS_ADDED, {
          credentialType: 'array',
          credentialName: vals.credentialName,
          vendorType: vals.vendorType,
          stage: 'creation_success'
        })
      } catch (error) {
        console.error('Error creating storage array credentials:', error)

        track(AMPLITUDE_EVENTS.CREDENTIALS_FAILED, {
          credentialType: 'array',
          credentialName: vals.credentialName,
          vendorType: vals.vendorType,
          errorMessage: error instanceof Error ? error.message : String(error),
          stage: 'creation'
        })

        reportError(error as Error, {
          context: 'array-credential-creation',
          metadata: {
            credentialName: vals.credentialName,
            vendorType: vals.vendorType,
            hostname: vals.hostname,
            action: 'create-array-credential'
          }
        })

        setArrayCredsValidated(false)
        setValidatingArrayCreds(false)
        setFormError(
          'Error creating storage array credentials: ' +
            (axios.isAxiosError(error) ? error?.response?.data?.message : String(error))
        )
        setSubmitting(false)
      }
    },
    [reportError, track]
  )

  const isSubmitDisabled = submitting || validatingArrayCreds || !isValid

  const renderBasicsSection = () => (
    <Section>
      <SectionHeader
        title="Connection Details"
        subtitle="Provide the name and endpoint details for the storage array you want to connect."
      />

      <FormGrid gap={2} minWidth={300}>
        <RHFTextField
          name="credentialName"
          label="Credential name"
          placeholder="pure-array-prod"
          required
          fullWidth
          size="small"
          rules={{
            required: 'Credential name is required',
            validate: (value: string) =>
              isValidName(value) ||
              'Credential name must start with a lowercase letter/number and use only lowercase letters, numbers, or hyphens.'
          }}
        />

        <RHFTextField
          name="vendorType"
          label="Storage vendor"
          select
          required
          fullWidth
          size="small"
          rules={{ required: 'Storage vendor is required' }}
        >
          {ARRAY_VENDOR_TYPES.map((vendor) => (
            <MenuItem key={vendor.value} value={vendor.value}>
              {vendor.label}
            </MenuItem>
          ))}
        </RHFTextField>

        <RHFTextField
          name="hostname"
          label="Array hostname / IP"
          placeholder="192.168.1.100 or array.example.com"
          required
          fullWidth
          size="small"
          rules={{ required: 'Array hostname is required' }}
          InputProps={{
            endAdornment: (
              <InputAdornment position="end">
                <Tooltip
                  title="Enter the IP address or FQDN of your storage array management interface."
                  arrow
                  placement="left"
                >
                  <IconButton size="small" tabIndex={-1}>
                    <InfoOutlined fontSize="small" />
                  </IconButton>
                </Tooltip>
              </InputAdornment>
            )
          }}
        />
      </FormGrid>
    </Section>
  )

  const renderAuthSection = () => (
    <Section>
      <SectionHeader
        title="Authentication"
        subtitle="Provide credentials with administrative access to the storage array."
      />

      <FormGrid gap={2} minWidth={300}>
        <RHFTextField
          name="username"
          label="Username"
          placeholder="pureuser"
          required
          fullWidth
          size="small"
          rules={{ required: 'Username is required' }}
        />

        <RHFTextField
          name="password"
          label="Password"
          type={showPassword ? 'text' : 'password'}
          required
          fullWidth
          size="small"
          rules={{ required: 'Password is required' }}
          InputProps={{
            endAdornment: (
              <InputAdornment position="end">
                <IconButton
                  onClick={handleClickShowPassword}
                  onMouseDown={handleMouseDownPassword}
                  edge="end"
                  size="small"
                >
                  {showPassword ? <VisibilityOff /> : <Visibility />}
                </IconButton>
              </InputAdornment>
            )
          }}
        />
      </FormGrid>
    </Section>
  )

  const renderSecuritySection = () => (
    <Section>
      <SectionHeader
        title="Connection Options"
        subtitle="Configure SSL/TLS verification settings for the storage array connection."
      />

      <RHFToggleField
        name="skipSSLVerification"
        label="Skip SSL verification"
        description="Disable SSL certificate verification for self-signed certificates."
        helperText="Only use this in trusted environments."
      />

      <Collapse in={Boolean(skipSSLVerificationValue)} unmountOnExit sx={{ mt: 1.5 }}>
        <Alert severity="warning" variant="outlined">
          Disabling SSL verification may expose credentials in transit. Use only when you fully
          trust the network between vJailbreak and the storage array.
        </Alert>
      </Collapse>
    </Section>
  )

  const renderOpenstackMappingSection = () => (
    <Section>
      <SectionHeader
        title="OpenStack Mapping (Optional)"
        subtitle="Configure the Cinder backend mapping for storage accelerated copy operations."
      />

      <FormGrid gap={2} minWidth={300}>
        <RHFTextField
          name="volumeType"
          label="Cinder volume type"
          placeholder="pure-iscsi"
          fullWidth
          size="small"
          InputProps={{
            endAdornment: (
              <InputAdornment position="end">
                <Tooltip
                  title="The Cinder volume type associated with this storage array."
                  arrow
                  placement="left"
                >
                  <IconButton size="small" tabIndex={-1}>
                    <InfoOutlined fontSize="small" />
                  </IconButton>
                </Tooltip>
              </InputAdornment>
            )
          }}
        />

        <RHFTextField
          name="cinderBackendName"
          label="Cinder backend name"
          placeholder="pure-iscsi-1"
          fullWidth
          size="small"
          InputProps={{
            endAdornment: (
              <InputAdornment position="end">
                <Tooltip
                  title="The backend name configured in cinder.conf (e.g., pure-01)."
                  arrow
                  placement="left"
                >
                  <IconButton size="small" tabIndex={-1}>
                    <InfoOutlined fontSize="small" />
                  </IconButton>
                </Tooltip>
              </InputAdornment>
            )
          }}
        />

        <RHFTextField
          name="cinderBackendPool"
          label="Cinder backend pool"
          placeholder="vt-pure-iscsi"
          fullWidth
          size="small"
          InputProps={{
            endAdornment: (
              <InputAdornment position="end">
                <Tooltip
                  title="Optional pool name within the backend."
                  arrow
                  placement="left"
                >
                  <IconButton size="small" tabIndex={-1}>
                    <InfoOutlined fontSize="small" />
                  </IconButton>
                </Tooltip>
              </InputAdornment>
            )
          }}
        />

        <RHFTextField
          name="cinderHost"
          label="Cinder host"
          placeholder="pcd-ce@pure-iscsi-1#vt-pure-iscsi"
          fullWidth
          size="small"
          InputProps={{
            endAdornment: (
              <InputAdornment position="end">
                <Tooltip
                  title="Full Cinder host string for manage API (format: hostname@backend or hostname@backend#pool)."
                  arrow
                  placement="left"
                >
                  <IconButton size="small" tabIndex={-1}>
                    <InfoOutlined fontSize="small" />
                  </IconButton>
                </Tooltip>
              </InputAdornment>
            )
          }}
        />
      </FormGrid>
    </Section>
  )

  const renderStatusRow = () => (
    <OperationStatus
      display="flex"
      flexDirection="column"
      gap={2}
      loading={validatingArrayCreds}
      loadingMessage="Validating storage array credentials…"
      success={arrayCredsValidated === true && Boolean(credentialNameValue)}
      successMessage="Storage array credentials created and validated."
      successIcon={<CheckIcon color="success" fontSize="small" />}
      error={formError}
    />
  )

  return (
    <DrawerShell
      open={open}
      onClose={closeDrawer}
      header={
        <DrawerHeader
          title="Add Storage Array Credentials"
          subtitle="Provide storage array connection details for accelerated copy operations"
          onClose={closeDrawer}
        />
      }
      footer={
        <DrawerFooter>
          <ActionButton tone="secondary" onClick={closeDrawer} data-testid="array-cred-cancel">
            Cancel
          </ActionButton>
          <ActionButton
            tone="primary"
            type="submit"
            form={formId}
            loading={submitting}
            disabled={isSubmitDisabled}
            data-testid="array-cred-submit"
          >
            Save Credential
          </ActionButton>
        </DrawerFooter>
      }
    >
      <DesignSystemForm
        form={form}
        onSubmit={onSubmit}
        keyboardSubmitProps={{ open, onClose: closeDrawer, isSubmitDisabled }}
        data-testid="array-cred-form"
        id={formId}
      >
        <SurfaceCard>
          <Box sx={{ display: 'grid', gap: 2 }}>
            <Box
              sx={{
                display: 'grid',
                gridTemplateColumns: { xs: '1fr', md: '1fr 1fr' },
                gap: 2,
                alignItems: 'start'
              }}
            >
              {renderBasicsSection()}
              {renderAuthSection()}
            </Box>

            {renderSecuritySection()}

            {renderOpenstackMappingSection()}

            {renderStatusRow()}
          </Box>
        </SurfaceCard>
      </DesignSystemForm>
    </DrawerShell>
  )
}
