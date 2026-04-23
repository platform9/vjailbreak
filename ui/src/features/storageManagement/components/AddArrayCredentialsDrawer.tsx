import { useState } from 'react'
import { Alert, Box, CircularProgress, MenuItem, Select, Typography } from '@mui/material'
import CheckCircleIcon from '@mui/icons-material/CheckCircle'
import ErrorIcon from '@mui/icons-material/Error'
import { useForm } from 'react-hook-form'
import { useQueryClient } from '@tanstack/react-query'
import { ARRAY_CREDS_QUERY_KEY } from 'src/hooks/api/useArrayCredentialsQuery'
import { createArrayCredsWithSecretFlow, deleteArrayCredsWithSecretFlow } from 'src/api/helpers'
import {
  getArrayCredentials,
  patchNetAppConfig
} from 'src/api/array-creds/arrayCreds'
import type { ArrayCreds, BackendTargetGroup } from 'src/api/array-creds/model'
import { ARRAY_CREDS_PHASE_NEEDS_BACKEND_SELECTION } from 'src/api/array-creds/model'
import { useErrorHandler } from 'src/hooks/useErrorHandler'
import { useAmplitude } from 'src/hooks/useAmplitude'
import { AMPLITUDE_EVENTS } from 'src/types/amplitude'
import { ConfirmationDialog } from 'src/components/dialogs'
import { DesignSystemForm } from 'src/shared/components/forms'
import { ActionButton, DrawerFooter, DrawerHeader, DrawerShell } from 'src/components/design-system'
import ArrayCredentialsFormFields from './ArrayCredentialsFormFields'

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
  netAppSvm?: string
  netAppFlexVol?: string
}

type ValidationStatus =
  | 'idle'
  | 'validating'
  | 'success'
  | 'failed'
  | 'needsBackendSelection'

export default function AddArrayCredentialsDrawer({
  open,
  onClose
}: AddArrayCredentialsDrawerProps) {
  const { reportError } = useErrorHandler({ component: 'AddArrayCredentialsDrawer' })
  const { track } = useAmplitude({ component: 'AddArrayCredentialsDrawer' })
  const queryClient = useQueryClient()
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [isValidating, setIsValidating] = useState(false)
  const [validationStatus, setValidationStatus] = useState<ValidationStatus>('idle')
  const [validationMessage, setValidationMessage] = useState<string>('')
  const [submitError, setSubmitError] = useState<string | null>(null)
  const [showDiscardDialog, setShowDiscardDialog] = useState(false)
  const [createdCredentialName, setCreatedCredentialName] = useState<string | null>(null)

  const form = useForm<FormData>({
    defaultValues: {
      name: '',
      vendorType: 'pure',
      volumeType: '',
      backendName: '',
      managementEndpoint: '',
      username: '',
      password: '',
      skipSslVerification: false,
      netAppSvm: '',
      netAppFlexVol: ''
    }
  })

  // Backend-target picker state. Populated when the controller reports
  // NeedsBackendSelection after validation.
  const [backendTargets, setBackendTargets] = useState<BackendTargetGroup[]>([])
  const [selectedSvm, setSelectedSvm] = useState<string>('')
  const [selectedFlexVol, setSelectedFlexVol] = useState<string>('')
  const [isPatchingSelection, setIsPatchingSelection] = useState(false)

  const {
    formState: { errors },
    reset,
    getValues
  } = form

  const handleClose = () => {
    if (isValidating) return // Don't allow closing while validating

    // Check if form has been touched (has any values)
    const formValues = getValues()
    const hasChanges =
      formValues.name || formValues.managementEndpoint || formValues.username || formValues.password

    if (hasChanges && !isSubmitting) {
      setShowDiscardDialog(true)
      return
    }

    // Cleanup any created credential if user is closing before validation completes
    if (createdCredentialName && validationStatus !== 'success') {
      deleteArrayCredsWithSecretFlow(createdCredentialName).catch((err) => {
        console.error(`Error deleting cancelled credential: ${createdCredentialName}`, err)
      })
    }

    setSubmitError(null)
    setValidationStatus('idle')
    setValidationMessage('')
    setCreatedCredentialName(null)
    onClose()
  }

  const handleDiscardChanges = async () => {
    // Cleanup any created credential when user discards changes
    if (createdCredentialName && validationStatus !== 'success') {
      deleteArrayCredsWithSecretFlow(createdCredentialName).catch((err) => {
        console.error(`Error deleting discarded credential: ${createdCredentialName}`, err)
      })
    }

    setSubmitError(null)
    setValidationStatus('idle')
    setValidationMessage('')
    setCreatedCredentialName(null)
    onClose()
  }

  const handleCancelDiscard = () => {
    setShowDiscardDialog(false)
  }

  // Poll for validation status after creating credentials.
  // Returns the latest ArrayCreds so callers can inspect phase + backendTargets
  // (needed for the NetApp NeedsBackendSelection flow).
  const pollForValidation = async (
    name: string,
    maxAttempts = 30,
    intervalMs = 2000
  ): Promise<{ success: boolean; message?: string; arrayCreds?: ArrayCreds }> => {
    for (let attempt = 0; attempt < maxAttempts; attempt++) {
      await new Promise((resolve) => setTimeout(resolve, intervalMs))

      try {
        const updated = await getArrayCredentials(name)
        const status = updated.status?.arrayValidationStatus

        // Check if validation succeeded
        if (status === 'Succeeded') {
          return { success: true, arrayCreds: updated }
        }

        // Check if validation failed
        if (status && status !== 'AwaitingCredentials' && status !== 'Validating') {
          return {
            success: false,
            message:
              updated.status?.arrayValidationMessage || `Validation failed with status: ${status}`,
            arrayCreds: updated
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

    track(AMPLITUDE_EVENTS.STORAGE_ARRAY_CREDENTIALS_ADDED, {
      credentialName: data.name,
      vendorType: data.vendorType,
      volumeType: data.volumeType,
      backendName: data.backendName,
      managementEndpoint: data.managementEndpoint,
      stage: 'creation_start'
    })

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
        },
        NETAPP_CONFIG:
          data.vendorType === 'netapp' && (data.netAppSvm || data.netAppFlexVol)
            ? { svm: data.netAppSvm, flexVol: data.netAppFlexVol }
            : undefined
      })

      // Track the created credential name for cleanup if needed
      setCreatedCredentialName(data.name)

      // Start validation polling
      setIsSubmitting(false)
      setIsValidating(true)
      setValidationStatus('validating')
      setValidationMessage('Validating credentials with storage array...')

      const validationResult = await pollForValidation(data.name)
      setIsValidating(false)

      // NetApp: credentials are valid but the controller needs an SVM/FlexVol
      // selection before the ArrayCreds is usable for migrations. Surface the
      // discovered targets so the user can pick.
      const phase = validationResult.arrayCreds?.status?.phase
      if (
        !validationResult.success &&
        phase === ARRAY_CREDS_PHASE_NEEDS_BACKEND_SELECTION
      ) {
        const targets = validationResult.arrayCreds?.status?.backendTargets || []
        setBackendTargets(targets)
        // Preselect if there's only one SVM / one FlexVol.
        if (targets.length === 1) {
          setSelectedSvm(targets[0].name)
          if ((targets[0].children?.length || 0) === 1) {
            setSelectedFlexVol(targets[0].children![0].name)
          }
        }
        setValidationStatus('needsBackendSelection')
        setValidationMessage(
          validationResult.arrayCreds?.status?.arrayValidationMessage ||
            'Select an SVM and FlexVol to complete setup.'
        )
        return // Stay in the form; user completes selection below
      }

      if (!validationResult.success) {
        track(AMPLITUDE_EVENTS.STORAGE_ARRAY_CREDENTIALS_FAILED, {
          credentialName: data.name,
          vendorType: data.vendorType,
          stage: 'validation',
          errorMessage: validationResult.message || 'Validation failed'
        })

        // Validation failed - show error and cleanup
        setValidationStatus('failed')
        setValidationMessage(validationResult.message || 'Validation failed')

        try {
          await cleanupFailedCreation(data.name)
          setCreatedCredentialName(null) // Clear after cleanup
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
      setCreatedCredentialName(null)
      onClose()
    } catch (error: any) {
      console.error('Error creating array credentials:', error)
      setIsValidating(false)
      setValidationStatus('idle')
      const errorMessage =
        error?.response?.data?.message || error?.message || 'Failed to create array credentials'

      track(AMPLITUDE_EVENTS.STORAGE_ARRAY_CREDENTIALS_FAILED, {
        credentialName: data.name,
        vendorType: data.vendorType,
        stage: 'creation',
        errorMessage
      })

      setSubmitError(errorMessage)
      reportError(error, {
        context: 'add-array-credentials',
        metadata: { credentialName: data.name }
      })
    } finally {
      setIsSubmitting(false)
    }
  }

  // Applies the chosen SVM/FlexVol to spec.netAppConfig and re-polls validation
  // so the ArrayCreds reaches Succeeded phase before the drawer closes.
  const handleBackendSelectionSubmit = async () => {
    if (!createdCredentialName || !selectedSvm || !selectedFlexVol) return
    setIsPatchingSelection(true)
    try {
      await patchNetAppConfig(createdCredentialName, {
        svm: selectedSvm,
        flexVol: selectedFlexVol
      })
      setIsPatchingSelection(false)
      setIsValidating(true)
      setValidationStatus('validating')
      setValidationMessage('Verifying selection against the NetApp array...')

      const validationResult = await pollForValidation(createdCredentialName)
      setIsValidating(false)

      if (!validationResult.success) {
        setValidationStatus('failed')
        setValidationMessage(
          validationResult.message || 'Selection could not be validated against the array.'
        )
        return
      }

      setValidationStatus('success')
      setValidationMessage('Credentials validated successfully!')
      queryClient.invalidateQueries({ queryKey: ARRAY_CREDS_QUERY_KEY })
      await new Promise((resolve) => setTimeout(resolve, 1500))
      reset()
      setValidationStatus('idle')
      setValidationMessage('')
      setCreatedCredentialName(null)
      setBackendTargets([])
      setSelectedSvm('')
      setSelectedFlexVol('')
      onClose()
    } catch (error) {
      setIsPatchingSelection(false)
      const asObj = error as { response?: { data?: { message?: string } }; message?: string }
      const errorMessage =
        asObj?.response?.data?.message ||
        asObj?.message ||
        'Failed to apply SVM/FlexVol selection'
      setSubmitError(errorMessage)
      reportError(error as Error, {
        context: 'patch-netapp-config',
        metadata: { credentialName: createdCredentialName }
      })
    }
  }

  const flexVolOptions =
    backendTargets.find((g) => g.name === selectedSvm)?.children || []

  return (
    <DrawerShell
      open={open}
      onClose={handleClose}
      requireCloseConfirmation={false}
      header={
        <DrawerHeader
          title="Add Array Credentials"
          onClose={handleClose}
          data-testid="add-array-credentials-drawer-header"
        />
      }
      footer={
        <DrawerFooter>
          <ActionButton
            tone="secondary"
            onClick={handleClose}
            disabled={isSubmitting || isValidating || isPatchingSelection}
          >
            Cancel
          </ActionButton>
          {validationStatus === 'needsBackendSelection' ? (
            <ActionButton
              onClick={handleBackendSelectionSubmit}
              loading={isPatchingSelection || isValidating}
              disabled={!selectedSvm || !selectedFlexVol}
            >
              {isPatchingSelection || isValidating ? 'Applying...' : 'Apply Selection'}
            </ActionButton>
          ) : (
            <ActionButton
              type="submit"
              form="add-array-credentials-form"
              loading={isSubmitting || isValidating}
              disabled={validationStatus === 'success'}
            >
              {isSubmitting
                ? 'Creating...'
                : isValidating
                  ? 'Validating...'
                  : validationStatus === 'success'
                    ? 'Success'
                    : 'Save Credentials'}
            </ActionButton>
          )}
        </DrawerFooter>
      }
      data-testid="add-array-credentials-drawer"
    >
      <DesignSystemForm
        id="add-array-credentials-form"
        form={form}
        onSubmit={onSubmit}
        keyboardSubmitProps={{
          open,
          onClose: handleClose,
          isSubmitDisabled: isSubmitting || isValidating || validationStatus === 'success'
        }}
        sx={{ height: '100%', display: 'flex', flexDirection: 'column' }}
      >
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

        {validationStatus === 'needsBackendSelection' && (
          <Box sx={{ mb: 3 }}>
            <Alert severity="info" sx={{ mb: 2 }}>
              {validationMessage ||
                'Credentials validated. Select an SVM and FlexVol to complete setup.'}
            </Alert>
            <Typography variant="subtitle1" fontWeight={600} sx={{ mb: 0.5 }}>
              Select NetApp Target
            </Typography>
            <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
              Pick the SVM and FlexVol where migration LUNs will be created.
            </Typography>
            <Box sx={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 2 }}>
              <Box>
                <Typography variant="caption" color="text.secondary">
                  SVM
                </Typography>
                <Select
                  fullWidth
                  size="small"
                  value={selectedSvm}
                  onChange={(e) => {
                    setSelectedSvm(e.target.value as string)
                    setSelectedFlexVol('')
                  }}
                  displayEmpty
                >
                  <MenuItem value="" disabled>
                    Select an SVM
                  </MenuItem>
                  {backendTargets.map((g) => (
                    <MenuItem key={g.name} value={g.name}>
                      {g.name}
                    </MenuItem>
                  ))}
                </Select>
              </Box>
              <Box>
                <Typography variant="caption" color="text.secondary">
                  FlexVol
                </Typography>
                <Select
                  fullWidth
                  size="small"
                  value={selectedFlexVol}
                  onChange={(e) => setSelectedFlexVol(e.target.value as string)}
                  displayEmpty
                  disabled={!selectedSvm}
                >
                  <MenuItem value="" disabled>
                    {selectedSvm ? 'Select a FlexVol' : 'Select an SVM first'}
                  </MenuItem>
                  {flexVolOptions.map((c) => (
                    <MenuItem key={c.name} value={c.name}>
                      {c.name}
                    </MenuItem>
                  ))}
                </Select>
              </Box>
            </Box>
          </Box>
        )}

        {validationStatus !== 'needsBackendSelection' && (
          <ArrayCredentialsFormFields mode="add" errors={errors} />
        )}
      </DesignSystemForm>

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
    </DrawerShell>
  )
}
