import { Box, Typography } from '@mui/material'
import CheckCircleIcon from '@mui/icons-material/CheckCircle'
import RadioButtonUncheckedIcon from '@mui/icons-material/RadioButtonUnchecked'
import ErrorIcon from '@mui/icons-material/Error'
import { Migration, Condition } from '../../api/migrations'

function conditionIcon(condition: Condition) {
  if (condition.type === 'Failed' || (condition.status === 'False' && condition.type !== 'Migrating')) {
    return <ErrorIcon sx={{ fontSize: 16, color: 'error.main' }} />
  }
  if (condition.status === 'True') {
    return <CheckCircleIcon sx={{ fontSize: 16, color: 'success.main' }} />
  }
  return <RadioButtonUncheckedIcon sx={{ fontSize: 16, color: 'text.disabled' }} />
}

function formatConditionTime(ts: Date | string | undefined): string {
  if (!ts) return '—'
  return new Date(ts).toLocaleTimeString(undefined, {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
}

function conditionSummary(condition: Condition): string {
  const parts: string[] = []
  if (condition.type) parts.push(String(condition.type))
  if (condition.message) parts.push(String(condition.message))
  return parts.join(' — ')
}

interface MigrationActivityTimelineProps {
  migration: Migration
}

export default function MigrationActivityTimeline({ migration }: MigrationActivityTimelineProps) {
  const conditions = migration.status?.conditions ?? []

  if (conditions.length === 0) {
    return (
      <Box
        sx={{
          p: 2.5,
          bgcolor: 'background.paper',
          borderRadius: 2,
          border: '1px solid',
          borderColor: 'divider',
          minWidth: 0,
        }}
      >
        <Typography variant="body2" color="text.secondary">
          No activity recorded yet.
        </Typography>
      </Box>
    )
  }

  // Sort by lastTransitionTime ascending
  const sorted = [...conditions].sort((a, b) => {
    const ta = a.lastTransitionTime ? new Date(a.lastTransitionTime).getTime() : 0
    const tb = b.lastTransitionTime ? new Date(b.lastTransitionTime).getTime() : 0
    return ta - tb
  })

  return (
    <Box
      sx={{
        p: 2.5,
        bgcolor: 'background.paper',
        borderRadius: 2,
        border: '1px solid',
        borderColor: 'divider',
        minWidth: 0,
      }}
    >
      <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 2 }}>
        <Typography variant="subtitle2" fontWeight={600}>Activity timeline</Typography>
        <Typography variant="body2" color="primary.main" sx={{ opacity: 0.5, cursor: 'default', fontSize: '0.8rem' }}>
          View full history
        </Typography>
      </Box>
      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0 }}>
        {sorted.map((condition, idx) => {
          const isLast = idx === sorted.length - 1
          return (
            <Box key={idx} sx={{ display: 'flex', gap: 1, position: 'relative' }}>
              {/* Timestamp — fixed-width left column */}
              <Typography
                variant="caption"
                color="text.disabled"
                sx={{
                  fontFamily: '"Fira Code", monospace',
                  flexShrink: 0,
                  width: 60,
                  pt: 0.25,
                  lineHeight: '16px',
                }}
              >
                {formatConditionTime(condition.lastTransitionTime)}
              </Typography>

              {/* Icon + vertical connecting line */}
              <Box
                sx={{
                  display: 'flex',
                  flexDirection: 'column',
                  alignItems: 'center',
                  flexShrink: 0,
                  pt: 0.25,
                }}
              >
                {conditionIcon(condition)}
                {!isLast && (
                  <Box
                    sx={{
                      width: '2px',
                      flex: 1,
                      bgcolor: 'divider',
                      my: 0.5,
                      minHeight: 16,
                    }}
                  />
                )}
              </Box>

              {/* Content */}
              <Box sx={{ pb: isLast ? 0 : 1.5, minWidth: 0, flex: 1, pl: 0.5 }}>
                <Typography
                  variant="body2"
                  fontWeight={500}
                  sx={{ lineHeight: '16px', pt: 0.25 }}
                  noWrap
                  title={conditionSummary(condition)}
                >
                  {conditionSummary(condition)}
                </Typography>
                {condition.reason && (
                  <Typography variant="caption" color="text.secondary" sx={{ display: 'block' }}>
                    {String(condition.reason)}
                  </Typography>
                )}
              </Box>
            </Box>
          )
        })}
      </Box>
    </Box>
  )
}
