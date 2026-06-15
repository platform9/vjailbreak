import { useCallback, useEffect, useState } from 'react'
import {
  Alert,
  Box,
  Chip,
  Divider,
  InputAdornment,
  TextField,
  Typography
} from '@mui/material'
import AutorenewIcon from '@mui/icons-material/Autorenew'
import EditNoteIcon from '@mui/icons-material/EditNote'
import { useForm } from 'react-hook-form'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { DrawerShell, DrawerHeader, DrawerFooter, ActionButton } from 'src/components/design-system'
import { DesignSystemForm, RHFTextField } from 'src/shared/components/forms'
import { generateSSHKeyPair, createManualSSHKeyPair } from 'src/api/sshKeyPairs/sshKeyPairs'
import { SSH_KEY_PAIRS_QUERY_KEY } from 'src/hooks/api/useSSHKeyPairsQuery'
import { useErrorHandler } from 'src/hooks/useErrorHandler'
import { isValidName, validateSshPrivateKey } from 'src/utils'

type AddMode = 'generate' | 'manual'

interface AddSSHKeyPairDrawerProps {
  open: boolean
  onClose: () => void
}

type FormData = {
  name: string
  privateKey: string
  publicKey: string
}

export default function AddSSHKeyPairDrawer({ open, onClose }: AddSSHKeyPairDrawerProps) {
  const { reportError } = useErrorHandler({ component: 'AddSSHKeyPairDrawer' })
  const queryClient = useQueryClient()
  const [mode, setMode] = useState<AddMode>('generate')
  const [error, setError] = useState<string | null>(null)
  const [generatedPublicKey, setGeneratedPublicKey] = useState<string | null>(null)

  const form = useForm<FormData>({
    defaultValues: { name: '', privateKey: '', publicKey: '' },
    mode: 'onChange',
    reValidateMode: 'onChange'
  })

  const {
    reset,
    formState: { isValid }
  } = form

  useEffect(() => {
    if (!open) return
    reset({ name: '', privateKey: '', publicKey: '' })
    setError(null)
    setGeneratedPublicKey(null)
    setMode('generate')
  }, [open, reset])

  const handleClose = useCallback(() => {
    reset()
    setError(null)
    setGeneratedPublicKey(null)
    onClose()
  }, [onClose, reset])

  const { mutateAsync: save, isPending } = useMutation({
    mutationFn: async (data: FormData) => {
      const name = data.name.trim()
      if (mode === 'generate') {
        const result = await generateSSHKeyPair(name)
        setGeneratedPublicKey(result.publicKey)
        return result
      } else {
        return createManualSSHKeyPair(name, data.privateKey, data.publicKey)
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: SSH_KEY_PAIRS_QUERY_KEY })
    }
  })

  const onSubmit = async (data: FormData) => {
    const name = data.name.trim()
    if (!isValidName(name)) {
      setError('Invalid name. Use lowercase letters, numbers, and hyphens only.')
      return
    }

    if (mode === 'manual') {
      const keyErr = validateSshPrivateKey(data.privateKey)
      if (keyErr) {
        setError(keyErr)
        return
      }
      if (!data.publicKey.trim()) {
        setError('Public key is required.')
        return
      }
    }

    try {
      setError(null)
      await save(data)
      if (mode === 'manual') {
        handleClose()
      }
    } catch (e: any) {
      const status = e?.response?.status
      const message =
        status === 409
          ? 'A key pair with this name already exists. Choose a different name.'
          : e?.response?.data?.message || e?.message || 'Failed to save SSH key pair'
      setError(message)
      reportError(e as Error, { context: 'create-ssh-keypair' })
    }
  }

  const isGenerateMode = mode === 'generate'
  const keyPairSaved = isGenerateMode && !!generatedPublicKey

  return (
    <DrawerShell
      open={open}
      onClose={handleClose}
      requireCloseConfirmation={true}
      width={820}
      header={
        <DrawerHeader
          title="Add SSH Key Pair"
          subtitle="Generate a new RSA-4096 key pair or provide your own."
          onClose={handleClose}
        />
      }
      footer={
        <DrawerFooter>
          <ActionButton tone="secondary" onClick={handleClose} disabled={isPending}>
            {keyPairSaved ? 'Close' : 'Cancel'}
          </ActionButton>
          {!keyPairSaved && (
            <ActionButton
              tone="primary"
              type="submit"
              form="add-ssh-keypair-form"
              loading={isPending}
              disabled={!isValid}
            >
              {isGenerateMode ? 'Generate' : 'Save'}
            </ActionButton>
          )}
        </DrawerFooter>
      }
      data-testid="add-ssh-keypair-drawer"
    >
      <Box sx={{ display: 'flex', gap: 1, mb: 3 }}>
        <Chip
          icon={<AutorenewIcon />}
          label="Generate"
          onClick={() => {
            setMode('generate')
            setError(null)
            setGeneratedPublicKey(null)
          }}
          color={isGenerateMode ? 'primary' : 'default'}
          variant={isGenerateMode ? 'filled' : 'outlined'}
          clickable
        />
        <Chip
          icon={<EditNoteIcon />}
          label="Manual"
          onClick={() => {
            setMode('manual')
            setError(null)
            setGeneratedPublicKey(null)
          }}
          color={!isGenerateMode ? 'primary' : 'default'}
          variant={!isGenerateMode ? 'filled' : 'outlined'}
          clickable
        />
      </Box>

      <DesignSystemForm
        id="add-ssh-keypair-form"
        form={form}
        onSubmit={onSubmit}
        keyboardSubmitProps={{
          open,
          onClose: handleClose,
          isSubmitDisabled: isPending || !isValid || keyPairSaved
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
          label="Key Pair Name"
          placeholder="my-proxy-vm-key"
          required
          disabled={isPending || keyPairSaved}
          rules={{
            required: 'Key pair name is required',
            validate: (val: string) =>
              isValidName(val.trim()) ? true : 'Use lowercase letters, numbers, and hyphens only'
          }}
          onValueChange={() => setError(null)}
        />

        {isGenerateMode && !generatedPublicKey && (
          <Typography variant="body2" color="text.secondary">
            vJailbreak will generate an RSA-4096 key pair. The private key is stored securely and
            never exposed. You will receive the public key to add to your Proxy VM&apos;s
            authorized_keys.
          </Typography>
        )}

        {isGenerateMode && generatedPublicKey && (
          <>
            <Alert severity="success">
              Key pair generated successfully. Copy the public key below and add it to your Proxy
              VM&apos;s <code>~/.ssh/authorized_keys</code>.
            </Alert>
            <TextField
              label="Public Key"
              value={generatedPublicKey}
              multiline
              minRows={4}
              fullWidth
              InputProps={{
                readOnly: true,
                endAdornment: (
                  <InputAdornment position="end">
                    <ActionButton
                      tone="secondary"
                      size="small"
                      onClick={() => navigator.clipboard?.writeText(generatedPublicKey)}
                    >
                      Copy
                    </ActionButton>
                  </InputAdornment>
                )
              }}
            />
          </>
        )}

        {!isGenerateMode && (
          <>
            <Divider />
            <RHFTextField
              name="privateKey"
              label="SSH Private Key"
              placeholder="-----BEGIN RSA PRIVATE KEY-----&#10;...&#10;-----END RSA PRIVATE KEY-----"
              required
              multiline
              minRows={8}
              disabled={isPending}
              rules={{
                required: 'SSH private key is required',
                validate: (val: string) => validateSshPrivateKey(val) || true
              }}
              onValueChange={() => setError(null)}
            />
            <RHFTextField
              name="publicKey"
              label="SSH Public Key"
              placeholder="ssh-rsa AAAA..."
              required
              multiline
              minRows={3}
              disabled={isPending}
              rules={{ required: 'SSH public key is required' }}
              onValueChange={() => setError(null)}
            />
          </>
        )}
      </DesignSystemForm>
    </DrawerShell>
  )
}
