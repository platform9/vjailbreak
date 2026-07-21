import { useState } from 'react'
import {
  Alert,
  Box,
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
import { DrawerShell, DrawerHeader, KeyValueGrid, StatusChip, SurfaceCard } from 'src/components'
import type { StatusChipTone } from 'src/components'
import { getSecret } from 'src/api/secrets/secrets'
import { ProxyVM, ProxyVMValidationStatus } from 'src/api/proxyvms/model'
import { VJAILBREAK_DEFAULT_NAMESPACE } from 'src/api/constants'

interface ProxyVMDetailDrawerProps {
  open: boolean
  proxyVM: ProxyVM | null
  onClose: () => void
}

function statusTone(status: ProxyVMValidationStatus | undefined): StatusChipTone {
  switch (status) {
    case 'Ready':
      return 'success'
    case 'Deploying':
      return 'info'
    case 'Verifying':
      return 'warning'
    case 'DeployFailed':
    case 'VerificationFailed':
      return 'error'
    default:
      return 'default'
  }
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
    : '—'

  const lastValidated = status?.lastValidationTime
    ? new Date(status.lastValidationTime).toLocaleString()
    : '—'

  const validationStatus = status?.validationStatus ?? 'Pending'
  const isInProgress = validationStatus === 'Deploying' || validationStatus === 'Verifying'

  const isOVADeployed = !spec.sshKeyPairRef && !spec.sshKeySecretRef

  const generalItems = [
    { label: 'VM name', value: spec.vmName },
    { label: 'VMware credentials', value: spec.vmwareCredsRef.name },
    { label: 'IP address', value: status?.ipAddress },
    {
      label: 'Attached disks',
      value: status?.attachedDiskCount != null ? String(status.attachedDiskCount) : undefined
    },
    { label: 'Last validated', value: lastValidated },
    ...(status?.componentsVerified?.length
      ? [
          {
            label: 'Components verified',
            value: status.componentsVerified
              .map((c) => `${c.name} ${c.present ? '✓' : '✗'}`)
              .join(', ')
          }
        ]
      : [
          {
            label: 'Components verified',
            value: '—'
          }
        ]),
    { label: 'Created', value: createdAt }
  ]

  return (
    <DrawerShell
      open={open}
      onClose={onClose}
      width={760}
      requireCloseConfirmation={false}
      header={
        <DrawerHeader
          icon={<DnsIcon color="primary" />}
          title={metadata.name}
          subtitle="vJailbreak Proxy VM details"
          onClose={onClose}
        />
      }
    >
      <Box sx={{ display: 'grid', gap: 2, p: 3 }}>
        <SurfaceCard
          variant="card"
          title="Status"
          actions={
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
              {isInProgress && <CircularProgress size={14} thickness={5} />}
              <StatusChip
                label={validationStatus}
                tone={statusTone(status?.validationStatus)}
                size="small"
                variant="filled"
              />
            </Box>
          }
        >
          {status?.validationMessage && (
            <Typography
              variant="body2"
              color={
                validationStatus === 'DeployFailed' || validationStatus === 'VerificationFailed'
                  ? 'error.main'
                  : 'text.secondary'
              }
            >
              {status.validationMessage}
            </Typography>
          )}
        </SurfaceCard>

        <SurfaceCard variant="card" title="General">
          <KeyValueGrid items={generalItems} />
        </SurfaceCard>

        <SurfaceCard
          variant="card"
          title="SSH Access"
          subtitle={
            isOVADeployed ? undefined : 'SSH public key used by vJailbreak to access this Proxy VM.'
          }
        >
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
                label=""
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
              This vJailbreak Proxy VM uses a manually uploaded private key — no public key is
              stored.
            </Alert>
          ) : isOVADeployed ? (
            <Typography variant="body2" color="text.secondary">
              SSH access is configured automatically during OVA deployment.
            </Typography>
          ) : validationStatus === 'Ready' ? (
            <Alert severity="warning">No SSH key configured for this vJailbreak Proxy VM.</Alert>
          ) : (
            <Typography variant="body2" color="text.secondary">
              SSH key will be available once the VM is ready.
            </Typography>
          )}
        </SurfaceCard>
      </Box>
    </DrawerShell>
  )
}
