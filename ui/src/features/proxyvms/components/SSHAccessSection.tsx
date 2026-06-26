import {
  Alert,
  Box,
  Checkbox,
  FormControl,
  FormControlLabel,
  FormHelperText,
  IconButton,
  InputAdornment,
  TextField,
  ToggleButton,
  ToggleButtonGroup,
  Tooltip,
  Typography
} from '@mui/material'
import ContentCopyIcon from '@mui/icons-material/ContentCopy'
import CheckIcon from '@mui/icons-material/Check'
import { Controller, useFormContext } from 'react-hook-form'
import { ActionButton, Section, SectionHeader } from 'src/components'
import { RHFTextField } from 'src/shared/components/forms'
import { validateSshPrivateKey } from 'src/utils'
import type { SSHKeySource, GeneratedKey, SelectFormData } from './types'

interface SSHAccessSectionProps {
  sshKeySource: SSHKeySource
  onSshKeySourceChange: (v: SSHKeySource) => void
  generatedKey: GeneratedKey | null
  isGenerating: boolean
  generateError: string | null
  onClearGenerateError: () => void
  vmSelected: boolean
  isSubmitting: boolean
  copied: boolean
  hasPrivateKey?: boolean
  onGenerate: () => void
  onRegenerateKey: () => void
  onCopy: () => void
  onKeyFileUpload: (file: File | null) => void
  onSubmitErrorChange: (e: string | null) => void
}

function AuthorizedKeysConfirmation() {
  const { control } = useFormContext<SelectFormData>()
  return (
    <Controller
      name="authorizedKeysConfirmed"
      control={control}
      rules={{ validate: (v) => v === true || 'Required' }}
      shouldUnregister
      render={({ field, fieldState }) => (
        <FormControl error={Boolean(fieldState.error)}>
          <Box
            sx={{
              border: '1px solid',
              borderColor: 'divider',
              borderRadius: 1,
              px: 1.5,
              py: 0.5
            }}
          >
            <FormControlLabel
              control={
                <Checkbox
                  {...field}
                  checked={field.value}
                  onChange={(e) => field.onChange(e.target.checked)}
                  size="small"
                />
              }
              label={
                <Typography variant="body2">
                  I&apos;ve added this public key to the proxy VM&apos;s{' '}
                  <code>authorized_keys</code>.
                </Typography>
              }
            />
          </Box>
          {fieldState.error && (
            <FormHelperText sx={{ ml: 0, fontWeight: 600 }}>
              {fieldState.error.message}
            </FormHelperText>
          )}
        </FormControl>
      )}
    />
  )
}

export default function SSHAccessSection({
  sshKeySource,
  onSshKeySourceChange,
  generatedKey,
  isGenerating,
  generateError,
  onClearGenerateError,
  vmSelected,
  isSubmitting,
  copied,
  hasPrivateKey = false,
  onGenerate,
  onRegenerateKey,
  onCopy,
  onKeyFileUpload,
  onSubmitErrorChange
}: SSHAccessSectionProps) {
  return (
    <Section>
      <SectionHeader
        title="SSH Access"
        subtitle="vJailbreak needs SSH access to attach disks during Hot-Add migrations."
        sx={{ mb: 1 }}
      />
      <Box sx={{ display: 'grid', gap: 2 }}>
        <ToggleButtonGroup
          value={sshKeySource}
          exclusive
          onChange={(_, v) => v && onSshKeySourceChange(v as SSHKeySource)}
          size="small"
        >
          <ToggleButton value="generated">Generate Key Pair</ToggleButton>
          <ToggleButton value="manual">Upload Private Key</ToggleButton>
        </ToggleButtonGroup>

        {sshKeySource === 'generated' ? (
          <Box sx={{ display: 'grid', gap: 2 }}>
            {!generatedKey ? (
              <>
                <Alert severity="info">
                  Generate a key pair, then add the public key to the VM&apos;s{' '}
                  <strong>/root/.ssh/authorized_keys</strong> before registering.
                </Alert>
                {generateError && (
                  <Alert severity="error" onClose={onClearGenerateError}>
                    {generateError}
                  </Alert>
                )}
                <ActionButton
                  tone="secondary"
                  onClick={onGenerate}
                  loading={isGenerating}
                  disabled={!vmSelected || isGenerating}
                  sx={{ justifySelf: 'start' }}
                >
                  Generate Key Pair
                </ActionButton>
                {!vmSelected && (
                  <Typography variant="caption" color="text.secondary">
                    Select a VM first to generate a key pair.
                  </Typography>
                )}
              </>
            ) : (
              <>
                <Alert severity="warning">
                  Copy the public key below and add it to{' '}
                  <strong>/root/.ssh/authorized_keys</strong> on the vJailbreak Proxy VM before
                  registering.
                </Alert>
                <TextField
                  label=""
                  value={generatedKey.publicKey.trim()}
                  multiline
                  minRows={4}
                  fullWidth
                  slotProps={{
                    input: {
                      readOnly: true,
                      endAdornment: (
                        <InputAdornment position="end" sx={{ alignSelf: 'flex-start', mt: 1 }}>
                          <Tooltip title={copied ? 'Copied!' : 'Copy public key'}>
                            <IconButton onClick={onCopy} size="small" edge="end">
                              {copied ? (
                                <CheckIcon fontSize="small" color="success" />
                              ) : (
                                <ContentCopyIcon fontSize="small" />
                              )}
                            </IconButton>
                          </Tooltip>
                        </InputAdornment>
                      )
                    }
                  }}
                  sx={{ fontFamily: 'monospace', fontSize: '0.75rem' }}
                />
                <ActionButton
                  tone="secondary"
                  onClick={onRegenerateKey}
                  sx={{ justifySelf: 'start' }}
                  disabled={isSubmitting}
                >
                  Regenerate
                </ActionButton>
                <AuthorizedKeysConfirmation />
              </>
            )}
          </Box>
        ) : (
          <Box sx={{ display: 'grid', gap: 2 }}>
            {hasPrivateKey ? (
              <Alert severity="warning">
                Before clicking Register, ensure the public key corresponding to this private key is
                added to <strong>/root/.ssh/authorized_keys</strong> on the vJailbreak Proxy VM.
              </Alert>
            ) : (
              <Alert severity="info">
                Upload or paste the SSH private key for the vJailbreak Proxy VM. Make sure the
                corresponding public key is already added to{' '}
                <strong>/root/.ssh/authorized_keys</strong> on the VM before registering.
              </Alert>
            )}
            <Box sx={{ display: 'flex', gap: 2, alignItems: 'center' }}>
              <ActionButton
                tone="secondary"
                component="label"
                disabled={isSubmitting}
                sx={{ whiteSpace: 'nowrap' }}
              >
                Upload key file
                <input
                  type="file"
                  hidden
                  onChange={(e) => onKeyFileUpload(e.target.files?.[0] ?? null)}
                />
              </ActionButton>
              <Typography variant="body2" color="text.secondary">
                Or paste the private key below (OpenSSH, RSA, EC, PKCS#8).
              </Typography>
            </Box>
            <RHFTextField
              name="sshPrivateKey"
              label="SSH Private Key"
              required
              multiline
              minRows={10}
              disabled={isSubmitting}
              placeholder={
                '-----BEGIN OPENSSH PRIVATE KEY-----\n...\n-----END OPENSSH PRIVATE KEY-----'
              }
              rules={{
                required: 'SSH private key is required',
                validate: (val: string) => validateSshPrivateKey(val) || true
              }}
              onValueChange={() => onSubmitErrorChange(null)}
            />
            {hasPrivateKey && <AuthorizedKeysConfirmation />}
          </Box>
        )}
      </Box>
    </Section>
  )
}
