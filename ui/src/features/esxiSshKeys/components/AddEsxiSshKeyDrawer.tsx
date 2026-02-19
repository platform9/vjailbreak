import { useCallback, useEffect, useMemo, useState } from 'react'
import { Alert, Box, Typography } from '@mui/material'
import { useForm } from 'react-hook-form'
import { useMutation } from '@tanstack/react-query'
import { DrawerShell, DrawerHeader, DrawerFooter, ActionButton } from 'src/components/design-system'
import { DesignSystemForm, RHFTextField } from 'src/shared/components/forms'
import { createSecret, replaceSecret } from 'src/api/secrets/secrets'
import { useErrorHandler } from 'src/hooks/useErrorHandler'
import { isValidName } from 'src/utils'

interface AddEsxiSshKeyDrawerProps {
  open: boolean
  onClose: () => void
  mode?: 'add' | 'edit'
  initialValues?: {
    name: string
    sshPrivateKey: string
  }
}

type FormData = {
  name: string
  sshPrivateKey: string
}

const validateOpenSshPrivateKey = (value: string): string | null => {
  const trimmed = value.trim()
  if (!trimmed) return 'SSH private key is required'
  if (/^ssh-privatekey\s*:/m.test(trimmed)) {
    return 'Paste only the key content (do not include "ssh-privatekey:")'
  }
  const hasBegin = /-----BEGIN OPENSSH PRIVATE KEY-----/.test(trimmed)
  const hasEnd = /-----END OPENSSH PRIVATE KEY-----/.test(trimmed)
  if (!hasBegin || !hasEnd) {
    return 'Invalid key format. Expected OpenSSH private key (-----BEGIN OPENSSH PRIVATE KEY-----)'
  }
  return null
}

export default function AddEsxiSshKeyDrawer({
  open,
  onClose,
  mode = 'add',
  initialValues
}: AddEsxiSshKeyDrawerProps) {
  const { reportError } = useErrorHandler({ component: 'AddEsxiSshKeyDrawer' })
  const [error, setError] = useState<string | null>(null)

  const defaultValues = useMemo(
    () => ({
      name: initialValues?.name ?? '',
      sshPrivateKey: initialValues?.sshPrivateKey ?? ''
    }),
    [initialValues?.name, initialValues?.sshPrivateKey]
  )

  const form = useForm<FormData>({
    defaultValues,
    mode: 'onChange',
    reValidateMode: 'onChange'
  })

  const {
    reset,
    setValue,
    formState: { isValid }
  } = form

  useEffect(() => {
    if (!open) return
    reset(defaultValues)
    setError(null)
  }, [defaultValues, open, reset])

  const { mutateAsync: saveKey, isPending } = useMutation({
    mutationFn: async (data: FormData) => {
      if (mode === 'edit') {
        return replaceSecret(
          data.name.trim(),
          { 'ssh-privatekey': data.sshPrivateKey.trim() },
          'migration-system'
        )
      }

      return createSecret(
        data.name.trim(),
        { 'ssh-privatekey': data.sshPrivateKey.trim() },
        'migration-system'
      )
    }
  })

  const handleClose = useCallback(() => {
    if (isPending) return
    reset()
    setError(null)
    onClose()
  }, [isPending, onClose, reset])

  const handleKeyFileChange = useCallback(
    async (file: File | null) => {
      if (!file) return
      const MAX_KEY_FILE_SIZE = 1024 * 1024
      if (file.size > MAX_KEY_FILE_SIZE) {
        setError('File is too large. SSH private key files should be less than 1 MB.')
        return
      }
      try {
        const text = await file.text()
        setValue('sshPrivateKey', text, { shouldDirty: true, shouldValidate: true })
        setError(null)
      } catch (e) {
        setError('Failed to read file')
      }
    },
    [setValue]
  )

  const onSubmit = async (data: FormData) => {
    const name = data.name.trim()
    if (!name) {
      setError('SSH key name is required')
      return
    }
    if (!isValidName(name)) {
      setError('Invalid name. Use a DNS-compatible name (lowercase letters, numbers, and hyphens).')
      return
    }

    const keyErr = validateOpenSshPrivateKey(data.sshPrivateKey)
    if (keyErr) {
      setError(keyErr)
      return
    }

    try {
      setError(null)
      await saveKey({ name, sshPrivateKey: data.sshPrivateKey })
      handleClose()
    } catch (e: any) {
      const status = e?.response?.status
      const message =
        status === 409 && mode === 'add'
          ? 'A secret with this name already exists. Choose a different name.'
          : e?.response?.data?.message || e?.message || 'Failed to save ESXi SSH key'
      setError(message)
      reportError(e as Error, {
        context: mode === 'edit' ? 'edit-esxi-ssh-key' : 'create-esxi-ssh-key'
      })
    }
  }

  return (
    <DrawerShell
      open={open}
      onClose={handleClose}
      requireCloseConfirmation={true}
      width={820}
      header={
        <DrawerHeader
          title={mode === 'edit' ? 'Edit ESXi SSH Key' : 'Add ESXi SSH Key'}
          subtitle="Provide a name and an OpenSSH private key."
          onClose={handleClose}
        />
      }
      footer={
        <DrawerFooter>
          <ActionButton tone="secondary" onClick={handleClose} disabled={isPending}>
            Cancel
          </ActionButton>
          <ActionButton
            tone="primary"
            type="submit"
            form="add-esxi-ssh-key-form"
            loading={isPending}
            disabled={!isValid}
          >
            Save
          </ActionButton>
        </DrawerFooter>
      }
      data-testid="add-esxi-ssh-key-drawer"
    >
      <DesignSystemForm
        id="add-esxi-ssh-key-form"
        form={form}
        onSubmit={onSubmit}
        keyboardSubmitProps={{
          open,
          onClose: handleClose,
          isSubmitDisabled: isPending || !isValid
        }}
        sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}
      >
        {error && (
          <Alert severity="error" sx={{ mb: 1 }} onClose={() => setError(null)}>
            {error}
          </Alert>
        )}

        <RHFTextField
          name="name"
          label="SSH Key Name"
          placeholder="esxi-ssh-key-1"
          disabled={isPending || mode === 'edit'}
          rules={{
            required: 'SSH key name is required',
            validate: (val: string) => (isValidName(val.trim()) ? true : 'Invalid name')
          }}
          onValueChange={() => setError(null)}
        />

        <Box sx={{ display: 'flex', gap: 2, alignItems: 'center' }}>
          <ActionButton tone="secondary" component="label" disabled={isPending}>
            Upload key file
            <input
              type="file"
              hidden
              onChange={(e) => handleKeyFileChange(e.target.files?.[0] ?? null)}
            />
          </ActionButton>
          <Typography variant="body2" color="text.secondary">
            Only the key content will be stored (do not include a field name).
          </Typography>
        </Box>

        <RHFTextField
          name="sshPrivateKey"
          label="SSH Private Key"
          placeholder="-----BEGIN OPENSSH PRIVATE KEY-----\n...\n-----END OPENSSH PRIVATE KEY-----"
          multiline
          minRows={12}
          disabled={isPending}
          rules={{
            validate: (val: string) => validateOpenSshPrivateKey(val) || true
          }}
          onValueChange={() => setError(null)}
        />
      </DesignSystemForm>
    </DrawerShell>
  )
}
