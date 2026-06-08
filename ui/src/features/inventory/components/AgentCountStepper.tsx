import { Box, IconButton, Typography } from '@mui/material'
import RemoveIcon from '@mui/icons-material/Remove'
import AddIcon from '@mui/icons-material/Add'

export interface AgentCountStepperProps {
  value: number
  onChange: (value: number) => void
  min?: number
  max: number
  disabled?: boolean
}

/**
 * +/- stepper for the recommended agent count, bounded to [min, max] (FR-019).
 * The recommendation derivation is shown by the host (TriggerPlanDialog).
 */
export default function AgentCountStepper({
  value,
  onChange,
  min = 0,
  max,
  disabled = false
}: AgentCountStepperProps) {
  const dec = () => onChange(Math.max(min, value - 1))
  const inc = () => onChange(Math.min(max, value + 1))

  return (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }} data-testid="agent-count-stepper">
      <IconButton
        aria-label="Decrease agents"
        size="small"
        onClick={dec}
        disabled={disabled || value <= min}
      >
        <RemoveIcon fontSize="small" />
      </IconButton>

      <Typography variant="h5" component="span" sx={{ minWidth: 40, textAlign: 'center' }}>
        {value}
      </Typography>

      <IconButton
        aria-label="Increase agents"
        size="small"
        onClick={inc}
        disabled={disabled || value >= max}
      >
        <AddIcon fontSize="small" />
      </IconButton>

      <Typography variant="body2" color="text.secondary">
        agent{value === 1 ? '' : 's'} to scale up
      </Typography>
    </Box>
  )
}
