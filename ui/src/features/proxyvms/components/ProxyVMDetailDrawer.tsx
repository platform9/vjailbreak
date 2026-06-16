import { useState } from 'react'
import {
  Alert,
  Box,
  Chip,
  CircularProgress,
  IconButton,
  InputAdornment,
  TextField,
  Tooltip,
  Typography
} from '@mui/material'
import ContentCopyIcon from '@mui/icons-material/ContentCopy'
import CheckIcon from '@mui/icons-material/Check'
import DnsIcon from '@mui/icons-material/Dns'
import { useQuery } from '@tanstack/react-query'
import { DrawerShell, DrawerHeader, Section, SectionHeader, SurfaceCard } from 'src/components'
import { getSecret } from 'src/api/secrets/secrets'
import { ProxyVM, ProxyVMValidationStatus } from 'src/api/proxyvms/model'
import { VJAILBREAK_DEFAULT_NAMESPACE } from 'src/api/constants'

interface ProxyVMDetailDrawerProps {
  open: boolean
  proxyVM: ProxyVM | null
  onClose: () => void
}

function statusColor(
  status: ProxyVMValidationStatus | undefined
): 'default' | 'warning' | 'success' | 'error' {
  switch (status) {
    case 'Ready':
      return 'success'
    case 'Verifying':
      return 'warning'
    case 'VerificationFailed':
      return 'error'
    default:
      return 'default'
  }
}

function InfoRow({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <Box sx={{ display: 'flex', gap: 2, alignItems: 'flex-start', py: 0.5 }}>
      <Typography variant="body2" color="text.secondary" sx={{ minWidth: 160, flexShrink: 0 }}>
        {label}
      </Typography>
      <Typography variant="body2" sx={{ wordBreak: 'break-all' }}>
        {value ?? '-'}
      </Typography>
    </Box>
  )
}

export default function ProxyVMDetailDrawer({ open, proxyVM, onClose }: ProxyVMDetailDrawerProps) {
  const [copied, setCopied] = useState(false)

  const keyPairSecretName = proxyVM?.spec?.sshKeyPairRef?.name ?? null

  const {
    data: publicKey,
    isLoading: keyLoading,
    error: keyError
  } = useQuery({
    queryKey: ['proxy-vm-pubkey', keyPairSecretName],
    queryFn: async () => {
      const secret = await getSecret(keyPairSecretName!, VJAILBREAK_DEFAULT_NAMESPACE)
      return (secret as any).data?.['ssh-publickey'] as string | undefined
    },
    enabled: open && Boolean(keyPairSecretName),
    staleTime: 60_000
  })

  const handleCopy = () => {
    if (!publicKey) return
    navigator.clipboard.writeText(publicKey.trim()).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    })
  }

  if (!proxyVM) return null

  const { metadata, spec, status } = proxyVM

  const createdAt = metadata.creationTimestamp
    ? new Date(metadata.creationTimestamp).toLocaleString()
    : '-'

  const lastValidated = status?.lastValidationTime
    ? new Date(status.lastValidationTime).toLocaleString()
    : '-'

  return (
    <DrawerShell
      open={open}
      onClose={onClose}
      width={640}
      requireCloseConfirmation={false}
      header={
        <DrawerHeader
          icon={<DnsIcon color="primary" />}
          title={metadata.name}
          subtitle="Proxy VM details"
          onClose={onClose}
        />
      }
    >
      <Box sx={{ display: 'grid', gap: 2, p: 3 }}>
        {/* Status */}
        <SurfaceCard>
          <Section>
            <SectionHeader title="Status" />
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, flexWrap: 'wrap' }}>
              <Chip
                label={status?.validationStatus ?? 'Pending'}
                color={statusColor(status?.validationStatus)}
                variant="outlined"
                size="small"
                sx={{ borderRadius: '4px' }}
              />
              {status?.validationMessage && (
                <Typography variant="body2" color="text.secondary">
                  {status.validationMessage}
                </Typography>
              )}
            </Box>
          </Section>
        </SurfaceCard>

        {/* General info */}
        <SurfaceCard>
          <Section>
            <SectionHeader title="General" />
            <Box sx={{ display: 'grid', gap: 0.5 }}>
              <InfoRow label="Proxy VM name" value={metadata.name} />
              <InfoRow label="VM name" value={spec.vmName} />
              <InfoRow label="VMware credentials" value={spec.vmwareCredsRef.name} />
              <InfoRow label="IP address" value={status?.ipAddress} />
              <InfoRow
                label="Attached disks"
                value={status?.attachedDiskCount != null ? status.attachedDiskCount : undefined}
              />
              <InfoRow label="Last validated" value={lastValidated} />
              <InfoRow label="Created" value={createdAt} />
              {status?.componentsVerified && status.componentsVerified.length > 0 && (
                <InfoRow
                  label="Components verified"
                  value={status.componentsVerified.join(', ')}
                />
              )}
            </Box>
          </Section>
        </SurfaceCard>

        {/* SSH / Public key */}
        <SurfaceCard>
          <Section>
            <SectionHeader
              title="SSH Access"
              subtitle="Public key to add to /root/.ssh/authorized_keys on the Proxy VM."
            />

            {keyPairSecretName ? (
              keyLoading ? (
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                  <CircularProgress size={16} />
                  <Typography variant="body2" color="text.secondary">
                    Loading public key…
                  </Typography>
                </Box>
              ) : keyError ? (
                <Alert severity="warning">
                  Could not load the public key from secret &ldquo;{keyPairSecretName}&rdquo;.
                </Alert>
              ) : publicKey ? (
                <TextField
                  label="Public Key"
                  value={publicKey.trim()}
                  multiline
                  minRows={4}
                  fullWidth
                  InputProps={{
                    readOnly: true,
                    sx: { fontFamily: 'monospace', fontSize: '0.75rem' },
                    endAdornment: (
                      <InputAdornment position="end" sx={{ alignSelf: 'flex-start', mt: 1 }}>
                        <Tooltip title={copied ? 'Copied!' : 'Copy public key'}>
                          <IconButton onClick={handleCopy} size="small" edge="end">
                            {copied ? (
                              <CheckIcon fontSize="small" color="success" />
                            ) : (
                              <ContentCopyIcon fontSize="small" />
                            )}
                          </IconButton>
                        </Tooltip>
                      </InputAdornment>
                    )
                  }}
                />
              ) : (
                <Alert severity="info">
                  Secret &ldquo;{keyPairSecretName}&rdquo; exists but contains no public key.
                </Alert>
              )
            ) : spec.sshKeySecretRef ? (
              <Alert severity="info">
                This Proxy VM uses a manually uploaded private key — no public key is stored.
              </Alert>
            ) : (
              <Alert severity="warning">No SSH key configured for this Proxy VM.</Alert>
            )}
          </Section>
        </SurfaceCard>
      </Box>
    </DrawerShell>
  )
}
