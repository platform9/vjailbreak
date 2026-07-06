import { Chip } from '@mui/material'
import { HostConversionPhase } from 'src/api/cluster-conversion-batches/model'

interface HostStatusChipProps {
  phase: HostConversionPhase
  size?: 'small' | 'medium'
}

type ChipColor = 'default' | 'warning' | 'info' | 'success' | 'error'

interface PhaseConfig {
  color: ChipColor
  label: string
}

const PHASE_CONFIG: Record<HostConversionPhase, PhaseConfig> = {
  CheckingEligibility: { color: 'default', label: 'Checking Eligibility' },
  NotReady: { color: 'warning', label: 'Not Ready' },
  Ready: { color: 'info', label: 'Ready' },
  Converting: { color: 'info', label: 'Converting' },
  Succeeded: { color: 'success', label: 'Succeeded' },
  Failed: { color: 'error', label: 'Failed' },
  NeedsAttention: { color: 'error', label: 'Needs Attention' },
  Skipped: { color: 'default', label: 'Skipped' }
}

export default function HostStatusChip({ phase, size = 'small' }: HostStatusChipProps) {
  const config = PHASE_CONFIG[phase] ?? { color: 'default' as ChipColor, label: phase }

  return (
    <Chip
      size={size}
      label={config.label}
      variant="outlined"
      color={config.color}
      sx={
        size === 'small'
          ? {
              borderRadius: '4px',
              height: '24px'
            }
          : undefined
      }
    />
  )
}
