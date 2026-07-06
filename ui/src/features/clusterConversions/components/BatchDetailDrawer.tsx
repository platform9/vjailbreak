import { useState } from 'react'
import {
  Box,
  Button,
  Chip,
  Drawer,
  IconButton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Tooltip,
  Typography,
  styled
} from '@mui/material'
import CloseIcon from '@mui/icons-material/Close'
import { ClusterConversionBatch, HostConversionStatus } from 'src/api/cluster-conversion-batches/model'
import { patchClusterConversionBatch } from 'src/api/cluster-conversion-batches/clusterConversionBatches'
import HostStatusChip from './HostStatusChip'

// ── Styled components ──────────────────────────────────────────────────────

const StyledDrawer = styled(Drawer)(() => ({
  '& .MuiDrawer-paper': {
    display: 'grid',
    gridTemplateRows: 'max-content 1fr max-content',
    width: '900px',
    maxWidth: '90vw'
  }
}))

const DrawerHeader = styled('div')(({ theme }) => ({
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
  padding: theme.spacing(2, 4),
  borderBottom: `1px solid ${theme.palette.divider}`
}))

const DrawerContent = styled('div')(({ theme }) => ({
  overflow: 'auto',
  padding: theme.spacing(3, 4)
}))

const DrawerFooter = styled('div')(({ theme }) => ({
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'flex-end',
  padding: theme.spacing(2, 4),
  borderTop: `1px solid ${theme.palette.divider}`
}))

// ── Props ──────────────────────────────────────────────────────────────────

export interface BatchDetailDrawerProps {
  open: boolean
  onClose: () => void
  batch: ClusterConversionBatch | null
  onRefresh: () => void
}

// ── Eligibility chip colors ────────────────────────────────────────────────

type EligibilityStatus = 'Ready' | 'NotReady' | 'Unknown'

const ELIGIBILITY_COLOR: Record<EligibilityStatus, 'success' | 'warning' | 'default'> = {
  Ready: 'success',
  NotReady: 'warning',
  Unknown: 'default'
}

// ── Component ──────────────────────────────────────────────────────────────

function BatchInfoSection({ batch }: { batch: ClusterConversionBatch }) {
  const { spec, status } = batch

  const formatDate = (iso?: string) => {
    if (!iso) return '—'
    return new Date(iso).toLocaleString()
  }

  const fields: { label: string; value: string }[] = [
    { label: 'VMware Cluster', value: spec.vmwareClusterName },
    { label: 'VMware Credentials', value: spec.vmwareCredsRef.name },
    { label: 'OpenStack Credentials', value: spec.openstackCredsRef.name },
    { label: 'BMConfig', value: spec.bmConfigRef.name },
    { label: 'Auto Start', value: spec.autoStart },
    { label: 'Max Retries', value: spec.maxRetries !== undefined ? String(spec.maxRetries) : '—' },
    { label: 'Started At', value: formatDate(status?.startedAt) },
    { label: 'Completed At', value: formatDate(status?.completedAt) },
    ...(status?.message ? [{ label: 'Status Message', value: status.message }] : [])
  ]

  return (
    <Box>
      <Typography variant="subtitle1" fontWeight="bold" gutterBottom>
        Batch Info
      </Typography>
      <Box
        sx={{
          display: 'grid',
          gridTemplateColumns: '1fr 1fr',
          gap: 2,
          p: 2,
          border: '1px solid',
          borderColor: 'divider',
          borderRadius: 1
        }}
      >
        {fields.map(({ label, value }) => (
          <Box key={label}>
            <Typography variant="caption" color="text.secondary">
              {label}
            </Typography>
            <Typography variant="body2" fontWeight={500}>
              {value}
            </Typography>
          </Box>
        ))}
      </Box>
    </Box>
  )
}

interface HostRowProps {
  hostStatus: HostConversionStatus
  batch: ClusterConversionBatch
  onRefresh: () => void
}

function HostActionsCell({ hostStatus, batch, onRefresh }: HostRowProps) {
  const [loading, setLoading] = useState<string | null>(null)

  const patch = async (annotationKey: string, value: string) => {
    setLoading(annotationKey)
    try {
      await patchClusterConversionBatch(batch.metadata.name, {
        metadata: {
          annotations: {
            [annotationKey]: value
          }
        }
      })
      onRefresh()
    } finally {
      setLoading(null)
    }
  }

  const handleRetry = () =>
    patch('vjailbreak.k8s.pf9.io/retry-host', hostStatus.esxiName)

  const handleSkip = () =>
    patch('vjailbreak.k8s.pf9.io/skip-host', hostStatus.esxiName)

  const handleTrigger = () =>
    patch('vjailbreak.k8s.pf9.io/trigger-host', hostStatus.esxiName)

  const { phase } = hostStatus
  const isManual = batch.spec.autoStart === 'Manual'

  if (phase === 'Ready' && isManual) {
    return (
      <Button
        size="small"
        variant="outlined"
        color="success"
        disabled={loading === 'vjailbreak.k8s.pf9.io/trigger-host'}
        onClick={handleTrigger}
      >
        Trigger
      </Button>
    )
  }

  if (phase === 'NeedsAttention' || phase === 'Failed') {
    return (
      <Box sx={{ display: 'flex', gap: 1 }}>
        <Button
          size="small"
          variant="outlined"
          color="warning"
          disabled={loading === 'vjailbreak.k8s.pf9.io/retry-host'}
          onClick={handleRetry}
        >
          Retry
        </Button>
        <Button
          size="small"
          variant="outlined"
          color="inherit"
          disabled={loading === 'vjailbreak.k8s.pf9.io/skip-host'}
          onClick={handleSkip}
        >
          Skip
        </Button>
      </Box>
    )
  }

  return null
}

const MAX_MESSAGE_LENGTH = 60

function TruncatedMessage({ message }: { message?: string }) {
  if (!message) return <Typography variant="body2">—</Typography>
  const truncated = message.length > MAX_MESSAGE_LENGTH
    ? `${message.slice(0, MAX_MESSAGE_LENGTH)}…`
    : message

  return truncated !== message ? (
    <Tooltip title={message} arrow>
      <Typography variant="body2" sx={{ cursor: 'default' }}>
        {truncated}
      </Typography>
    </Tooltip>
  ) : (
    <Typography variant="body2">{message}</Typography>
  )
}

function HostConversionTable({
  batch,
  onRefresh
}: {
  batch: ClusterConversionBatch
  onRefresh: () => void
}) {
  const hosts: HostConversionStatus[] = batch.status?.hosts ?? []

  return (
    <Box>
      <Typography variant="subtitle1" fontWeight="bold" gutterBottom>
        Host Conversion Status
      </Typography>
      <TableContainer
        sx={{
          border: '1px solid',
          borderColor: 'divider',
          borderRadius: 1
        }}
      >
        <Table size="small" stickyHeader>
          <TableHead>
            <TableRow>
              <TableCell>Host Name</TableCell>
              <TableCell>Phase</TableCell>
              <TableCell>Eligibility</TableCell>
              <TableCell align="center">Retry Count</TableCell>
              <TableCell>Message</TableCell>
              <TableCell>Actions</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {hosts.length === 0 ? (
              <TableRow>
                <TableCell colSpan={6} align="center">
                  <Typography variant="body2" color="text.secondary" sx={{ py: 2 }}>
                    No host status data available.
                  </Typography>
                </TableCell>
              </TableRow>
            ) : (
              hosts.map((host) => {
                const eligStatus = host.eligibilityStatus ?? 'Unknown'
                const eligColor = ELIGIBILITY_COLOR[eligStatus as EligibilityStatus] ?? 'default'
                return (
                  <TableRow key={host.esxiName} hover>
                    <TableCell>
                      <Typography variant="body2" fontWeight={500}>
                        {host.esxiName}
                      </Typography>
                    </TableCell>
                    <TableCell>
                      <HostStatusChip phase={host.phase} />
                    </TableCell>
                    <TableCell>
                      {host.eligibilityStatus ? (
                        <Tooltip title={host.eligibilityReason ?? ''} arrow placement="top">
                          <Chip
                            size="small"
                            label={eligStatus}
                            color={eligColor}
                            variant="outlined"
                            sx={{ borderRadius: '4px', height: '24px', cursor: 'default' }}
                          />
                        </Tooltip>
                      ) : (
                        <Typography variant="body2" color="text.secondary">
                          —
                        </Typography>
                      )}
                    </TableCell>
                    <TableCell align="center">
                      <Typography variant="body2">
                        {host.retryCount !== undefined ? host.retryCount : '—'}
                      </Typography>
                    </TableCell>
                    <TableCell>
                      <TruncatedMessage message={host.message} />
                    </TableCell>
                    <TableCell>
                      <HostActionsCell
                        hostStatus={host}
                        batch={batch}
                        onRefresh={onRefresh}
                      />
                    </TableCell>
                  </TableRow>
                )
              })
            )}
          </TableBody>
        </Table>
      </TableContainer>
    </Box>
  )
}

// ── Main export ────────────────────────────────────────────────────────────

export default function BatchDetailDrawer({
  open,
  onClose,
  batch,
  onRefresh
}: BatchDetailDrawerProps) {
  if (!batch) return null

  const batchPhase = batch.status?.phase

  return (
    <StyledDrawer anchor="right" open={open} onClose={onClose}>
      <DrawerHeader>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
          <Typography variant="h6">{batch.metadata.name}</Typography>
          {batchPhase && (
            <Chip
              size="small"
              label={batchPhase}
              variant="outlined"
              color={
                batchPhase === 'Succeeded'
                  ? 'success'
                  : batchPhase === 'Failed'
                  ? 'error'
                  : batchPhase === 'Running'
                  ? 'info'
                  : batchPhase === 'PartialFail'
                  ? 'warning'
                  : 'default'
              }
              sx={{ borderRadius: '4px', height: '24px' }}
            />
          )}
        </Box>
        <IconButton onClick={onClose} aria-label="close drawer">
          <CloseIcon />
        </IconButton>
      </DrawerHeader>

      <DrawerContent>
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
          <BatchInfoSection batch={batch} />
          <HostConversionTable batch={batch} onRefresh={onRefresh} />
        </Box>
      </DrawerContent>

      <DrawerFooter>
        <Button variant="outlined" onClick={onClose}>
          Close
        </Button>
      </DrawerFooter>
    </StyledDrawer>
  )
}
