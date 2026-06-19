import {
  Box,
  LinearProgress,
  List,
  ListItem,
  Typography,
} from '@mui/material'
import CheckCircleIcon from '@mui/icons-material/CheckCircle'
import { Migration, Phase } from '../../api/migrations'
import { TriggerAdminCutoverButton } from '../TriggerAdminCutover/TriggerAdminCutoverButton'
import { VJAILBREAK_DEFAULT_NAMESPACE } from 'src/api/constants'

// ─── Copying Disk Blocks ─────────────────────────────────────────────────────

interface DiskRow {
  label: string
  state: 'done' | 'active' | 'pending'
}

function buildDiskRows(currentDiskStr: string | undefined, totalDisks: number | undefined): DiskRow[] {
  const total = totalDisks ?? 1
  const current = currentDiskStr !== undefined ? parseInt(currentDiskStr, 10) : 0

  return Array.from({ length: total }, (_, i) => {
    const state: DiskRow['state'] =
      i < current ? 'done' :
      i === current ? 'active' :
      'pending'
    return { label: `Disk ${i + 1} of ${total}`, state }
  })
}

function CopyingPhaseDetail({ migration }: { migration: Migration }) {
  const status = migration.status
  const diskRows = buildDiskRows(status?.currentDisk, status?.totalDisks)

  return (
    <Box
      sx={{
        p: 3,
        bgcolor: 'background.paper',
        borderRadius: 2,
        border: '1px solid',
        borderColor: 'divider',
        mb: 2,
      }}
    >
      <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 0.5 }}>
        Currently · Copying Disk Blocks
      </Typography>
      <Typography variant="h6" fontWeight={700} sx={{ mb: 0.5 }}>
        Transferring Disk Data
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
        Streaming source VMDK blocks through nbdkit into target Cinder volumes.
      </Typography>

      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
        {diskRows.map((disk) => (
          <Box key={disk.label}>
            <Box sx={{ display: 'flex', justifyContent: 'space-between', mb: 0.5 }}>
              <Typography variant="body2" fontWeight={500}>
                {disk.label}
              </Typography>
              <Typography
                variant="caption"
                color={
                  disk.state === 'done'   ? 'success.main' :
                  disk.state === 'active' ? 'primary.main' :
                  'text.disabled'
                }
              >
                {disk.state === 'done'   ? 'Complete' :
                 disk.state === 'active' ? 'Copying…' :
                 'Pending'}
              </Typography>
            </Box>
            {disk.state === 'done' ? (
              <LinearProgress
                variant="determinate"
                value={100}
                sx={{
                  height: 6,
                  borderRadius: 3,
                  bgcolor: 'grey.100',
                  '& .MuiLinearProgress-bar': { bgcolor: 'success.main' },
                }}
              />
            ) : disk.state === 'active' ? (
              <LinearProgress
                variant="indeterminate"
                sx={{ height: 6, borderRadius: 3 }}
              />
            ) : (
              <LinearProgress
                variant="determinate"
                value={0}
                sx={{ height: 6, borderRadius: 3, bgcolor: 'grey.100' }}
              />
            )}
          </Box>
        ))}
      </Box>
    </Box>
  )
}

// ─── Awaiting Cutover ─────────────────────────────────────────────────────────

const CUTOVER_CHECKLIST = [
  'Quiesce and power off the source VM in vCenter',
  'Run a final CBT delta sync to capture changed blocks',
  'Detach volumes from worker, attach to target instance',
  'Boot target VM in PCD and run guest health checks',
  'Disconnect source network on the original VM',
]

function AwaitingCutoverDetail({
  migration,
  onSuccess,
}: {
  migration: Migration
  onSuccess?: () => void
}) {
  const migrationName = migration.metadata?.name ?? ''
  const namespace =
    (migration.metadata?.namespace as string | undefined) ?? VJAILBREAK_DEFAULT_NAMESPACE

  return (
    <Box
      sx={{
        p: 3,
        bgcolor: 'background.paper',
        borderRadius: 2,
        border: '1px solid',
        borderColor: 'warning.light',
        mb: 2,
      }}
    >
      <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 0.5 }}>
        Currently · Awaiting Admin Cutover
      </Typography>
      <Typography variant="h6" fontWeight={700} sx={{ mb: 0.5 }}>
        Ready for Final Cutover
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
        All disk data has been copied. The source VM is still running. An administrator must
        initiate the final cutover to complete the migration.
      </Typography>

      <Box
        sx={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          p: 2,
          bgcolor: 'warning.50',
          borderRadius: 1,
          border: '1px solid',
          borderColor: 'warning.200',
          mb: 2,
        }}
      >
        <Box>
          <Typography variant="body2" fontWeight={600}>
            Ready to cut over
          </Typography>
          <Typography variant="caption" color="text.secondary">
            Final delta sync will copy any blocks changed since the last sweep.
          </Typography>
        </Box>
        <TriggerAdminCutoverButton
          migrationName={migrationName}
          namespace={namespace}
          onSuccess={onSuccess}
        />
      </Box>

      <Typography variant="caption" color="text.secondary" fontWeight={600} sx={{ display: 'block', mb: 1 }}>
        Cutover will:
      </Typography>
      <List dense disablePadding>
        {CUTOVER_CHECKLIST.map((item) => (
          <ListItem key={item} disableGutters sx={{ py: 0.25 }}>
            <CheckCircleIcon sx={{ fontSize: 14, color: 'success.main', mr: 1, flexShrink: 0 }} />
            <Typography variant="caption" color="text.secondary">
              {item}
            </Typography>
          </ListItem>
        ))}
      </List>
    </Box>
  )
}

// ─── Success ──────────────────────────────────────────────────────────────────

function SuccessDetail({ migration }: { migration: Migration }) {
  const vmName = migration.spec?.vmName || migration.metadata?.name || '—'
  return (
    <Box
      sx={{
        p: 3,
        bgcolor: 'background.paper',
        borderRadius: 2,
        border: '1px solid',
        borderColor: 'success.light',
        mb: 2,
      }}
    >
      <Typography variant="caption" color="success.main" sx={{ display: 'block', mb: 0.5 }}>
        Migration complete
      </Typography>
      <Typography variant="h6" fontWeight={700} sx={{ mb: 0.5 }}>
        {vmName} is running in PCD
      </Typography>
      <Typography variant="body2" color="text.secondary">
        The target VM passed all post-boot health checks. The original VM has been powered off and
        disconnected from its source network.
      </Typography>
    </Box>
  )
}

// ─── Generic in-progress detail (Pending, Validating) ────────────────────────

function GenericActiveDetail({ migration }: { migration: Migration }) {
  const phase = migration.status?.phase ?? '—'
  return (
    <Box
      sx={{
        p: 3,
        bgcolor: 'background.paper',
        borderRadius: 2,
        border: '1px solid',
        borderColor: 'divider',
        mb: 2,
      }}
    >
      <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 0.5 }}>
        Currently active
      </Typography>
      <Typography variant="body2" color="text.secondary">
        Phase: <strong>{phase}</strong>
      </Typography>
    </Box>
  )
}

// ─── Router ───────────────────────────────────────────────────────────────────

interface MigrationPhaseDetailProps {
  migration: Migration
  onCutoverSuccess?: () => void
}

export default function MigrationPhaseDetail({
  migration,
  onCutoverSuccess,
}: MigrationPhaseDetailProps) {
  const phase = migration.status?.phase as Phase | undefined

  if (!phase) return null

  switch (phase) {
    case Phase.CopyingBlocks:
    case Phase.CopyingChangedBlocks:
    case Phase.ConvertingDisk:
    case Phase.AwaitingDataCopyStart:
      return <CopyingPhaseDetail migration={migration} />

    case Phase.AwaitingAdminCutOver:
    case Phase.AwaitingCutOverStartTime:
      return <AwaitingCutoverDetail migration={migration} onSuccess={onCutoverSuccess} />

    case Phase.Succeeded:
      return <SuccessDetail migration={migration} />

    case Phase.Failed:
    case Phase.ValidationFailed:
      // ErrorCard shown instead; return null here
      return null

    default:
      return <GenericActiveDetail migration={migration} />
  }
}
