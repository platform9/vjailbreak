import { useMemo } from 'react'
import { CircularProgress, styled, Typography, Box, Tooltip } from '@mui/material'
import CheckCircleOutlineIcon from '@mui/icons-material/CheckCircleOutline'
import ErrorOutlineIcon from '@mui/icons-material/ErrorOutline'
import HourglassBottomIcon from '@mui/icons-material/HourglassBottom'
import PauseCircleOutlineIcon from '@mui/icons-material/PauseCircleOutline'
import WarningAmberIcon from '@mui/icons-material/WarningAmber'
import { Phase } from 'src/api/migrations/model'

const ProgressContainer = styled(Box)({
  display: 'flex',
  alignItems: 'center',
  height: '100%',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap',
  maxWidth: '100%',
  overflow: 'hidden'
})

interface MigrationProgressProps {
  phase: Phase
  progressText: string
  syncWarningMessage?: string
}

export default function MigrationProgress({ progressText, phase, syncWarningMessage }: MigrationProgressProps) {
  const statusIcon = useMemo(() => {
    // Show warning icon if sync is in warning state (non-empty warning message)
    if (syncWarningMessage) {
      return <WarningAmberIcon sx={{ color: 'warning.main' }} />
    }
    if (phase === Phase.Succeeded) {
      return <CheckCircleOutlineIcon sx={{ color: 'success.main' }} />
    } else if (phase === Phase.AwaitingAdminCutOver) {
      return <PauseCircleOutlineIcon sx={{ color: 'warning.main' }} />
    } else if (
      [
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
      ].includes(phase as Phase)
    ) {
      return <CircularProgress size={20} sx={{ mr: '3px' }} />
    } else if (phase === Phase.Failed || phase === Phase.ValidationFailed) {
      return <ErrorOutlineIcon sx={{ color: 'error.main' }} />
    } else {
      return <HourglassBottomIcon sx={{ color: 'text.disabled' }} />
    }
  }, [phase, syncWarningMessage])

  return (
    <>
      <ProgressContainer data-testid="migration-progress-cell">
        {statusIcon}
        <Tooltip title={progressText} arrow>
          <Typography
            variant="body2"
            sx={{
              ml: 2,
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              whiteSpace: 'nowrap',
              maxWidth: '100%'
            }}
          >
            {progressText}
          </Typography>
        </Tooltip>
      </ProgressContainer>
    </>
  )
}
