import {
  Alert,
  Box,
  Collapse,
  Divider,
  FormLabel,
  IconButton,
  InputAdornment,
  Tooltip,
  Typography
} from '@mui/material'
import { useCallback, useEffect, useState } from 'react'
import { DrawerShell, DrawerHeader, DrawerFooter, ActionButton, FormGrid } from 'src/design-system'
import { createVMwareCredsWithSecretFlow, deleteVMwareCredsWithSecretFlow } from 'src/api/helpers'
import axios from 'axios'
import { useVmwareCredentialsQuery } from 'src/hooks/api/useVmwareCredentialsQuery'
import { CircularProgress } from '@mui/material'
import { Visibility, VisibilityOff } from '@mui/icons-material'
import CheckIcon from '@mui/icons-material/Check'
import { isValidName } from 'src/utils'
import { getVmwareCredentials } from 'src/api/vmware-creds/vmwareCreds'
import { useInterval } from 'src/hooks/useInterval'
import { THREE_SECONDS } from 'src/constants'
import { useErrorHandler } from 'src/hooks/useErrorHandler'
import { useAmplitude } from 'src/hooks/useAmplitude'
import { AMPLITUDE_EVENTS } from 'src/types/amplitude'
import InfoOutlined from '@mui/icons-material/InfoOutlined'
import { useForm } from 'react-hook-form'
import DesignSystemForm from 'src/components/forms/rhf/DesignSystemForm'
import RHFTextField from 'src/components/forms/rhf/RHFTextField'
import RHFToggleField from 'src/components/forms/rhf/RHFToggleField'

interface VMwareCredentialsDrawerProps {
  open: boolean
  onClose: () => void
}

interface VMwareCredentialFormValues {
  credentialName: string
  vcenterHost: string
  datacenter: string
  username: string
  password: string
  insecure: boolean
}

const defaultValues: VMwareCredentialFormValues = {
  credentialName: '',
  vcenterHost: '',
  datacenter: '',
  username: '',
  password: '',
  insecure: false
}

export default function VMwareCredentialsDrawer({ open, onClose }: VMwareCredentialsDrawerProps) {
  const { reportError } = useErrorHandler({ component: 'VMwareCredentialsDrawer' })
  const { track } = useAmplitude({ component: 'VMwareCredentialsDrawer' })
  const [validatingVmwareCreds, setValidatingVmwareCreds] = useState(false)
  const [vmwareCredsValidated, setVmwareCredsValidated] = useState<boolean | null>(null)
  const [formError, setFormError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [showPassword, setShowPassword] = useState(false)
  const [createdCredentialName, setCreatedCredentialName] = useState<string | null>(null)

  const { refetch: refetchVmwareCreds } = useVmwareCredentialsQuery()

  const form = useForm<VMwareCredentialFormValues>({
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
    vcenterHost: vcenterHostValue,
    datacenter: datacenterValue,
    username: usernameValue,
    password: passwordValue,
    insecure: insecureValue
  } = values
  const formId = 'vmware-credentials-form'

  const closeDrawer = useCallback(() => {
    // Check if we have a created credential that hasn't been fully validated (succeeded)
    if (createdCredentialName) {
      console.log(`Cleaning up VMware credential on drawer close: ${createdCredentialName}`)
      try {
        deleteVMwareCredsWithSecretFlow(createdCredentialName)
          .then(() =>
            console.log(`Cancelled credential ${createdCredentialName} deleted successfully`)
          )
          .catch((err) =>
            console.error(`Error deleting cancelled credential: ${createdCredentialName}`, err)
          )
      } catch (err) {
        console.error(
          `Error initiating deletion of cancelled credential: ${createdCredentialName}`,
          err
        )
      }
    }

    // Reset state
    reset(defaultValues)
    setCreatedCredentialName(null)
    setValidatingVmwareCreds(false)
    setVmwareCredsValidated(null)
    setFormError(null)
    setSubmitting(false)
    setShowPassword(false)

    onClose()
  }, [createdCredentialName, onClose, reset])

  const shouldPollVmwareCreds = !!createdCredentialName && validatingVmwareCreds

  const handleClickShowPassword = () => {
    setShowPassword((prev) => !prev)
  }

  const handleMouseDownPassword = (event: React.MouseEvent<HTMLButtonElement>) => {
    event.preventDefault()
  }

  useEffect(() => {
    setVmwareCredsValidated(null)
    setFormError(null)
  }, [
    credentialNameValue,
    vcenterHostValue,
    datacenterValue,
    usernameValue,
    passwordValue,
    insecureValue
  ])

  const handleValidationStatus = (status: string, message?: string) => {
    if (status === 'Succeeded') {
      setVmwareCredsValidated(true)
      setValidatingVmwareCreds(false)

      // Track successful credential validation
      track(AMPLITUDE_EVENTS.CREDENTIALS_ADDED, {
        credentialType: 'vmware',
        credentialName: createdCredentialName,
        stage: 'validation_success'
      })

      // Close the drawer after a short delay to show success state
      setTimeout(() => {
        refetchVmwareCreds()
        onClose()
      }, 1500)
    } else if (status === 'Failed') {
      setVmwareCredsValidated(false)
      setValidatingVmwareCreds(false)
      setFormError(message || 'Validation failed')

      // Track credential validation failure
      track(AMPLITUDE_EVENTS.CREDENTIALS_FAILED, {
        credentialType: 'vmware',
        credentialName: createdCredentialName,
        errorMessage: message || 'Validation failed',
        stage: 'validation'
      })

      reportError(
        new Error(`VMware credential validation failed: ${message || 'Unknown reason'}`),
        {
          context: 'vmware-validation-failure',
          metadata: {
            credentialName: createdCredentialName,
            validationMessage: message,
            action: 'vmware-validation-failed'
          }
        }
      )

      // Try to delete the failed credential to clean up
      if (createdCredentialName) {
        try {
          deleteVMwareCredsWithSecretFlow(createdCredentialName)
            .then(() => console.log(`Failed credential ${createdCredentialName} deleted`))
            .catch((deleteErr) =>
              console.error(`Error deleting failed credential: ${createdCredentialName}`, deleteErr)
            )
        } catch (deleteErr) {
          console.error(`Error deleting failed credential: ${createdCredentialName}`, deleteErr)
          reportError(deleteErr as Error, {
            context: 'vmware-credential-deletion',
            metadata: {
              credentialName: createdCredentialName,
              action: 'delete-failed-credential'
            }
          })
        }
      }
    }
    setSubmitting(false)
  }

  useInterval(
    async () => {
      try {
        const response = await getVmwareCredentials(createdCredentialName)
        if (response?.status?.vmwareValidationStatus) {
          handleValidationStatus(
            response.status.vmwareValidationStatus,
            response.status.vmwareValidationMessage
          )
        }
      } catch (err) {
        console.error('Error validating VMware credentials', err)
        reportError(err as Error, {
          context: 'vmware-validation-polling',
          metadata: {
            credentialName: createdCredentialName,
            action: 'vmware-validation-status-polling'
          }
        })
        setFormError('Error validating VMware credentials')
        setValidatingVmwareCreds(false)
        setSubmitting(false)
      }
    },
    THREE_SECONDS,
    shouldPollVmwareCreds
  )

  const onSubmit = useCallback(
    async (values: VMwareCredentialFormValues) => {
      setFormError(null)
      setSubmitting(true)
      setValidatingVmwareCreds(true)

      try {
        const credentialData = {
          VCENTER_HOST: values.vcenterHost,
          VCENTER_DATACENTER: values.datacenter,
          VCENTER_USERNAME: values.username,
          VCENTER_PASSWORD: values.password,
          ...(values.insecure && { VCENTER_INSECURE: true })
        }

        const response = await createVMwareCredsWithSecretFlow(
          values.credentialName,
          credentialData
        )

        setCreatedCredentialName(response.metadata.name)

        track(AMPLITUDE_EVENTS.CREDENTIALS_ADDED, {
          credentialType: 'vmware',
          credentialName: values.credentialName,
          vcenterHost: values.vcenterHost,
          namespace: response.metadata.namespace
        })
      } catch (error) {
        console.error('Error creating VMware credentials:', error)

        track(AMPLITUDE_EVENTS.CREDENTIALS_FAILED, {
          credentialType: 'vmware',
          credentialName: values.credentialName,
          vcenterHost: values.vcenterHost,
          errorMessage: error instanceof Error ? error.message : String(error),
          stage: 'creation'
        })

        reportError(error as Error, {
          context: 'vmware-credential-creation',
          metadata: {
            credentialName: values.credentialName,
            vcenterHost: values.vcenterHost,
            username: values.username,
            action: 'create-vmware-credential'
          }
        })

        setVmwareCredsValidated(false)
        setValidatingVmwareCreds(false)
        setFormError(
          'Error creating VMware credentials: ' +
            (axios.isAxiosError(error) ? error?.response?.data?.message : error)
        )
        setSubmitting(false)
      }
    },
    [reportError, track]
  )

  const isSubmitDisabled = submitting || validatingVmwareCreds || !isValid

  return (
    <DrawerShell
      open={open}
      onClose={closeDrawer}
      header={<DrawerHeader title="Add VMware Credentials" onClose={closeDrawer} />}
      footer={
        <DrawerFooter>
          <ActionButton tone="secondary" onClick={closeDrawer} data-testid="vmware-cred-cancel">
            Cancel
          </ActionButton>
          <ActionButton
            tone="primary"
            type="submit"
            form={formId}
            loading={submitting}
            disabled={isSubmitDisabled}
            data-testid="vmware-cred-submit"
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
        data-testid="vmware-cred-form"
        id={formId}
      >
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 3, py: 2.5 }}>
          <Box>
            <Typography variant="overline" color="text.secondary">
              Credential Basics
            </Typography>
            <Typography variant="h6" sx={{ mt: 0.25 }}>
              Connection Details
            </Typography>
            <Typography variant="body2" color="text.secondary" sx={{ mt: 0.25, mb: 2 }}>
              Provide the name and endpoint details for the VMware environment you want to connect.
            </Typography>
            <FormGrid gap={2} minWidth={300}>
              <RHFTextField
                name="credentialName"
                label="Credential name"
                placeholder="production-vcenter"
                required
                fullWidth
                size="small"
                rules={{
                  required: 'Credential name is required',
                  validate: (value) =>
                    isValidName(value) ||
                    'Credential name must start with a letter/number and use only letters, numbers, or hyphens.'
                }}
                //  labelHelperText="Used to reference this credential across the platform."
              />

              <RHFTextField
                name="vcenterHost"
                label="vCenter server"
                placeholder="https://vcenter.example.com"
                required
                fullWidth
                size="small"
                rules={{ required: 'vCenter server is required' }}
                InputProps={{
                  endAdornment: (
                    <InputAdornment position="end">
                      <Tooltip
                        title="Enter the FQDN or full URL for the vCenter endpoint."
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
                name="datacenter"
                label="Datacenter name"
                placeholder="Primary-DC"
                required
                fullWidth
                size="small"
                rules={{ required: 'Datacenter name is required' }}
              />
            </FormGrid>
          </Box>

          <Divider />

          <Box>
            <Typography variant="overline" color="text.secondary">
              Authentication
            </Typography>
            <Typography variant="h6" sx={{ mt: 0.25 }}>
              User Credentials
            </Typography>
            <Typography variant="body2" color="text.secondary" sx={{ mt: 0.25, mb: 2 }}>
              These credentials need permission to manage inventory and read cluster configuration.
            </Typography>
            <FormGrid gap={2} minWidth={300}>
              <RHFTextField
                name="username"
                label="Username"
                placeholder="administrator@vsphere.local"
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
          </Box>

          <Divider />

          <Box>
            <Typography variant="overline" color="text.secondary">
              Security
            </Typography>
            <Typography variant="h6" sx={{ mt: 0.25 }}>
              Connection Options
            </Typography>
            <Typography variant="body2" color="text.secondary" sx={{ mt: 0.25, mb: 1.5 }}>
              Prefer TLS-secure connections. Only disable SSL verification if your environment
              requires it.
            </Typography>

            <RHFToggleField
              name="insecure"
              label="Allow insecure connection"
              description="Skip SSL verification for self-signed or lab environments."
              helperText="Disabling verification may expose credentials in transit."
            />

            <Collapse in={Boolean(insecureValue)} unmountOnExit sx={{ mt: 1.5 }}>
              <Alert severity="warning" variant="outlined">
                Use this option only when you fully trust the network between Platform9 and the
                vCenter host.
              </Alert>
            </Collapse>
          </Box>

          <Box sx={{ display: 'flex', gap: 2, alignItems: 'center' }}>
            {validatingVmwareCreds && (
              <>
                <CircularProgress size={24} />
                <FormLabel>Validating VMware credentials...</FormLabel>
              </>
            )}
            {vmwareCredsValidated === true && credentialNameValue && (
              <>
                <CheckIcon color="success" fontSize="small" />
                <FormLabel>VMware credentials created</FormLabel>
              </>
            )}
          </Box>

          {formError && (
            <Alert severity="error" variant="outlined">
              {formError}
            </Alert>
          )}
        </Box>
      </DesignSystemForm>
    </DrawerShell>
  )
}
