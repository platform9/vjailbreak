import { Chip, Typography } from '@mui/material'
import { Phase } from '../api/migrations'
import { getPhaseColorKey, getPhaseLabel } from '../utils/phaseUtils'

interface MigrationStatusChipProps {
  phase: Phase | string | undefined
}

// Renders the Status column value: a colored pill for active/terminal/pending
// phases, or plain muted text only for the truly neutral "Unknown" phase.
export default function MigrationStatusChip({ phase }: MigrationStatusChipProps) {
  const colorKey = getPhaseColorKey(phase as Phase)
  const label = getPhaseLabel(phase as Phase)

  if (colorKey === 'default' && phase !== Phase.Pending) {
    return (
      <Typography variant="body2" color="text.secondary">
        {label}
      </Typography>
    )
  }

  return <Chip label={label} color={colorKey} size="small" variant="outlined" />
}
