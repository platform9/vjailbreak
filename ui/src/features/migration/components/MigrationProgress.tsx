import { useMemo } from 'react'
import { Box, Tooltip, Typography } from '@mui/material'
import { keyframes } from '@mui/material/styles'
import CheckCircleOutlineIcon from '@mui/icons-material/CheckCircleOutline'
import ErrorOutlineIcon from '@mui/icons-material/ErrorOutline'
import HourglassBottomIcon from '@mui/icons-material/HourglassBottom'
import PauseCircleOutlineIcon from '@mui/icons-material/PauseCircleOutline'
import WarningAmberIcon from '@mui/icons-material/WarningAmber'
import CircularProgress from '@mui/material/CircularProgress'
import { Condition, Phase } from 'src/api/migrations/model'
import { getPhaseLabel } from '../utils/phaseUtils'
import {
  getLatestCondition,
  getProgressText,
  getSegmentStates,
  getStepNumber,
  getStepPercent,
  SegmentStatus,
  TOTAL_STEPS
} from '../utils/migrationTableUtils'

const pulse = keyframes`
  0% { opacity: 1; }
  50% { opacity: 0.4; }
  100% { opacity: 1; }
`

const SEGMENT_COLOR: Record<SegmentStatus, string> = {
  done: 'success.main',
  active: 'primary.main',
  ready: 'warning.main',
  failed: 'error.main',
  pending: 'grey.300'
}

const IN_PROGRESS_PHASES = [
  Phase.Validating,
  Phase.AwaitingDataCopyStart,
  Phase.CopyingBlocks,
  Phase.CopyingChangedBlocks,
  Phase.SnapshottingSourceVM,
  Phase.AttachingDisksToProxy,
  Phase.IdentifyingBlockDevices,
  Phase.HotAddTransferInProgress,
  Phase.HotAddCleanup,
  Phase.ConvertingDisk,
  Phase.AwaitingCutOverStartTime
]

function SegmentedTrack({ segments }: { segments: SegmentStatus[] }) {
  return (
    <Box sx={{ display: 'flex', gap: '3px', width: '100%' }}>
      {segments.map((status, i) => (
        <Box
          key={i}
          data-testid="progress-segment"
          data-status={status}
          sx={{
            flex: 1,
            height: 5,
            borderRadius: 1,
            bgcolor: SEGMENT_COLOR[status],
            animation: status === 'active' ? `${pulse} 1.3s ease-in-out infinite` : 'none'
          }}
        />
      ))}
    </Box>
  )
}

interface MigrationProgressProps {
  phase: Phase
  conditions?: Condition[]
  currentDisk?: string
  totalDisks?: number
  syncWarningMessage?: string
}

// Renders the Progress column cell: phase name + step count, a 9-segment pipeline
// track (done/active/ready/failed/pending), and a detail line — everything derived
// from real CR status fields (phase/conditions/currentDisk/totalDisks), no fabricated
// throughput or ETA since the backend doesn't report those.
export default function MigrationProgress({
  phase,
  conditions = [],
  currentDisk,
  totalDisks,
  syncWarningMessage
}: MigrationProgressProps) {
  const stepNumber = useMemo(() => getStepNumber(phase, conditions), [phase, conditions])
  const segments = useMemo(() => getSegmentStates(phase, conditions), [phase, conditions])
  const percent = useMemo(
    () => getStepPercent(phase, stepNumber, currentDisk, totalDisks),
    [phase, stepNumber, currentDisk, totalDisks]
  )
  const latestCondition = useMemo(() => getLatestCondition(conditions), [conditions])
  const tooltipText = useMemo(
    () => getProgressText(phase, conditions, currentDisk, totalDisks),
    [phase, conditions, currentDisk, totalDisks]
  )

  const isFailed = phase === Phase.Failed || phase === Phase.ValidationFailed
  const isSucceeded = phase === Phase.Succeeded
  const isPending = !phase || phase === Phase.Pending || phase === Phase.Unknown
  const isAwaitingCutover =
    phase === Phase.AwaitingAdminCutOver || phase === Phase.AwaitingCutOverStartTime

  const statusWord = isFailed
    ? 'Error'
    : isSucceeded
      ? 'Done'
      : isAwaitingCutover
        ? 'Ready'
        : isPending
          ? 'Queued'
          : `${percent}%`

  const statusColor = isFailed
    ? 'error.main'
    : isSucceeded
      ? 'success.main'
      : isAwaitingCutover
        ? 'warning.main'
        : isPending
          ? 'text.disabled'
          : syncWarningMessage
            ? 'warning.main'
            : 'primary.main'

  const stepLabel = isFailed
    ? `at Step ${stepNumber} of ${TOTAL_STEPS}`
    : `Step ${stepNumber} of ${TOTAL_STEPS}`

  const diskLabel =
    currentDisk != null &&
    totalDisks &&
    (isAwaitingCutover ? false : IN_PROGRESS_PHASES.includes(phase))
      ? (() => {
          const parsed = parseInt(currentDisk, 10)
          const diskNum = Number.isNaN(parsed) ? currentDisk : parsed + 1
          return `Disk ${diskNum} of ${totalDisks}`
        })()
      : null

  const detail = isFailed
    ? [latestCondition?.reason, latestCondition?.message].filter(Boolean).join(' — ') ||
      'Migration failed.'
    : syncWarningMessage
      ? syncWarningMessage
      : isPending
        ? 'In queue — waiting for available agent'
        : isSucceeded
          ? latestCondition?.message || 'Migration completed successfully.'
          : isAwaitingCutover
            ? latestCondition?.message || 'Data copy complete — awaiting admin cutover'
            : [diskLabel, latestCondition?.message].filter(Boolean).join(' — ') ||
              getPhaseLabel(phase)

  const statusIcon = useMemo(() => {
    if (syncWarningMessage)
      return <WarningAmberIcon fontSize="small" sx={{ color: 'warning.main' }} />
    if (isSucceeded)
      return <CheckCircleOutlineIcon fontSize="small" sx={{ color: 'success.main' }} />
    if (phase === Phase.AwaitingAdminCutOver) {
      return <PauseCircleOutlineIcon fontSize="small" sx={{ color: 'warning.main' }} />
    }
    if (IN_PROGRESS_PHASES.includes(phase)) {
      return <CircularProgress size={14} thickness={5} />
    }
    if (isFailed) return <ErrorOutlineIcon fontSize="small" sx={{ color: 'error.main' }} />
    return <HourglassBottomIcon fontSize="small" sx={{ color: 'text.disabled' }} />
  }, [phase, syncWarningMessage, isSucceeded, isFailed])

  return (
    <Tooltip title={tooltipText} arrow>
      <Box data-testid="migration-progress-cell" sx={{ width: '100%', py: 0.5 }}>
        <Box
          sx={{ display: 'flex', alignItems: 'baseline', justifyContent: 'space-between', gap: 1 }}
        >
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.75, minWidth: 0 }}>
            {statusIcon}
            <Typography
              variant="body2"
              fontWeight={700}
              color={isFailed ? 'error.main' : isPending ? 'text.disabled' : 'text.primary'}
              noWrap
            >
              {getPhaseLabel(phase)}
            </Typography>
            <Typography variant="caption" color="text.secondary" noWrap>
              {stepLabel}
            </Typography>
          </Box>
          <Typography variant="body2" fontWeight={700} color={statusColor} sx={{ flexShrink: 0 }}>
            {statusWord}
          </Typography>
        </Box>
        <Box sx={{ mt: 0.5 }}>
          <SegmentedTrack segments={segments} />
        </Box>
        <Typography
          variant="caption"
          color={isFailed ? 'error.main' : 'text.secondary'}
          sx={{
            display: 'block',
            mt: 0.5,
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap'
          }}
        >
          {detail}
        </Typography>
      </Box>
    </Tooltip>
  )
}
