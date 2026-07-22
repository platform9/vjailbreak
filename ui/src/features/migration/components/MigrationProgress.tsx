import { Box, LinearProgress, Typography } from '@mui/material'
import type { ProgressDisplay } from '../utils/migrationTableUtils'

interface MigrationProgressProps {
  display: ProgressDisplay
}

export default function MigrationProgress({ display }: MigrationProgressProps) {
  const { primaryText, secondaryText, barValue, barVariant, barColor } = display
  const isNeutral = barColor === 'neutral'
  const isError = barColor === 'error'

  return (
    <Box data-testid="migration-progress-cell" sx={{ width: '100%', minWidth: 0 }}>
      <Box sx={{ display: 'flex', alignItems: 'baseline', justifyContent: 'space-between', gap: 1 }}>
        <Typography
          variant="body2"
          noWrap
          title={primaryText}
          sx={{
            minWidth: 0,
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            ...(isError && { color: 'error.main', fontWeight: 600 })
          }}
        >
          {primaryText}
        </Typography>
        {secondaryText && (
          <Typography variant="caption" color="text.secondary" noWrap sx={{ flexShrink: 0 }}>
            {secondaryText}
          </Typography>
        )}
      </Box>
      <LinearProgress
        variant={barVariant}
        value={barValue}
        color={isNeutral ? 'inherit' : barColor}
        sx={{
          mt: 0.5,
          height: 4,
          borderRadius: 1,
          ...(isNeutral && { color: 'action.disabled' })
        }}
      />
    </Box>
  )
}
