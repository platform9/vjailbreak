import {
  Alert,
  Box,
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
import { ActionButton, Section, SectionHeader } from 'src/components'
import { RHFTextField } from 'src/shared/components/forms'
import { validateSshPrivateKey } from 'src/utils'
import type { SSHKeySource, GeneratedKey } from './types'

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
  onGenerate: () => void
  onRegenerateKey: () => void
  onCopy: () => void
  onKeyFileUpload: (file: File | null) => void
  onSubmitErrorChange: (e: string | null) => void
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
                <Alert severity="success">
                  Key pair generated. Copy the public key below and add it to{' '}
                  <strong>/root/.ssh/authorized_keys</strong> on the vJailbreak Proxy VM before
                  registering.
                </Alert>
                <TextField
                  label="Public Key (copy this to authorized_keys)"
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
              </>
            )}
          </Box>
        ) : (
          <Box sx={{ display: 'grid', gap: 2 }}>
            <Alert severity="info">
              Add the public key corresponding to your private key to the vJailbreak Proxy VM&apos;s{' '}
              <strong>/root/.ssh/authorized_keys</strong> before submitting.
            </Alert>
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
          </Box>
        )}
      </Box>
    </Section>
  )
}
