import { useState } from 'react'
import {
  Alert,
  Box,
  Collapse,
  IconButton,
  Paper,
  Tooltip,
  Typography,
} from '@mui/material'
import ErrorOutlineIcon from '@mui/icons-material/ErrorOutline'
import ContentCopyIcon from '@mui/icons-material/ContentCopy'
import ExpandMoreIcon from '@mui/icons-material/ExpandMore'
import ExpandLessIcon from '@mui/icons-material/ExpandLess'
import { Migration, Condition } from '../../api/migrations'

const GENERIC_STEPS = [
  {
    title: 'Review the error message and pod logs',
    body: 'The Debug Logs tab contains the full pod output. Filter by ERROR or FATAL to find the root cause.',
  },
  {
    title: 'Check source VM accessibility',
    body: 'Verify the vCenter host, ESXi host, and datastore are reachable from the vJailbreak appliance.',
  },
  {
    title: 'Verify target capacity',
    body: 'Confirm the OpenStack Cinder pool has enough free capacity for all disks in the migration.',
  },
  {
    title: 'Retry the migration',
    body: 'After addressing the issue, use the Retry button. vJailbreak will resume from the last checkpoint.',
  },
]

function findErrorCondition(conditions: Condition[]): Condition | undefined {
  return (
    conditions.find((c) => c.type === 'Failed') ||
    conditions.find((c) => c.status === 'False')
  )
}

interface MigrationErrorCardProps {
  migration: Migration
}

export default function MigrationErrorCard({ migration }: MigrationErrorCardProps) {
  const [logsExpanded, setLogsExpanded] = useState(false)
  const [copied, setCopied] = useState(false)

  const conditions = migration.status?.conditions ?? []
  const errorCondition = findErrorCondition(conditions)
  const phase = migration.status?.phase ?? '—'

  const errorTitle =
    errorCondition?.message
      ? String(errorCondition.message)
      : phase === 'ValidationFailed'
      ? 'Validation failed'
      : 'Migration failed'

  const errorTimestamp = errorCondition?.lastTransitionTime
    ? new Date(errorCondition.lastTransitionTime).toLocaleString(undefined, {
        month: 'short',
        day: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
      })
    : '—'

  const failedConditions = conditions.filter(
    (c) => c.status === 'False' || c.type === 'Failed'
  )

  const handleCopy = async () => {
    const text = [
      `Phase: ${phase}`,
      `Time: ${errorTimestamp}`,
      `Error: ${errorTitle}`,
      '',
      'Conditions:',
      ...conditions.map(
        (c) => `  [${c.type}] status=${c.status} reason=${c.reason} msg=${c.message}`
      ),
    ].join('\n')
    await navigator.clipboard.writeText(text).catch(() => undefined)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <Box
      sx={{
        bgcolor: 'background.paper',
        borderRadius: 2,
        border: '1px solid',
        borderColor: 'divider',
        borderLeftWidth: 4,
        borderLeftColor: 'error.main',
        mb: 2,
        overflow: 'hidden',
      }}
    >
      {/* Header */}
      <Box
        sx={{
          display: 'flex',
          alignItems: 'flex-start',
          gap: 1.5,
          px: 3,
          pt: 2.5,
          pb: 2,
          borderBottom: '1px solid',
          borderColor: 'divider',
        }}
      >
        <ErrorOutlineIcon sx={{ color: 'error.main', mt: 0.25, flexShrink: 0 }} />
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <Box sx={{ display: 'flex', gap: 1, mb: 0.25 }}>
            <Typography variant="caption" color="text.secondary">
              {phase}
            </Typography>
            <Typography variant="caption" color="text.disabled">
              ·
            </Typography>
            <Typography variant="caption" color="text.secondary">
              {errorTimestamp}
            </Typography>
          </Box>
          <Typography variant="subtitle1" fontWeight={700} color="error.main" sx={{ wordBreak: 'break-word' }}>
            {errorTitle}
          </Typography>
        </Box>
        <Tooltip title={copied ? 'Copied!' : 'Copy diagnostic info'}>
          <IconButton size="small" onClick={handleCopy} sx={{ flexShrink: 0 }}>
            <ContentCopyIcon fontSize="small" />
          </IconButton>
        </Tooltip>
      </Box>

      {/* Body */}
      <Box sx={{ px: 3, py: 2.5 }}>
        {/* Conditions summary */}
        {failedConditions.length > 0 && (
          <Box sx={{ mb: 3 }}>
            <Typography variant="overline" color="text.secondary" sx={{ display: 'block', mb: 1 }}>
              What happened
            </Typography>
            {failedConditions.map((c, idx) => (
              <Alert key={idx} severity="error" sx={{ mb: 1, py: 0.5 }}>
                <Typography variant="body2">
                  <strong>{String(c.type)}</strong>
                  {c.message ? ` — ${String(c.message)}` : ''}
                </Typography>
              </Alert>
            ))}
          </Box>
        )}

        {/* Resolution steps */}
        <Box sx={{ mb: 2 }}>
          <Typography variant="overline" color="text.secondary" sx={{ display: 'block', mb: 1.5 }}>
            Recommended resolution
          </Typography>
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
            {GENERIC_STEPS.map((step, idx) => (
              <Box key={idx} sx={{ display: 'flex', gap: 1.5 }}>
                <Box
                  sx={{
                    width: 24,
                    height: 24,
                    borderRadius: '50%',
                    bgcolor: 'primary.main',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    flexShrink: 0,
                    mt: 0.1,
                  }}
                >
                  <Typography variant="caption" fontWeight={700} sx={{ color: 'white' }}>
                    {idx + 1}
                  </Typography>
                </Box>
                <Box>
                  <Typography variant="body2" fontWeight={600}>
                    {step.title}
                  </Typography>
                  <Typography variant="body2" color="text.secondary">
                    {step.body}
                  </Typography>
                </Box>
              </Box>
            ))}
          </Box>
        </Box>

        {/* Collapsible raw conditions */}
        {conditions.length > 0 && (
          <Box>
            <Box
              sx={{
                display: 'flex',
                alignItems: 'center',
                cursor: 'pointer',
                py: 0.5,
                borderTop: '1px solid',
                borderColor: 'divider',
                pt: 1.5,
              }}
              onClick={() => setLogsExpanded((v) => !v)}
            >
              <Typography variant="caption" color="text.secondary" sx={{ flex: 1 }}>
                Raw conditions ({conditions.length})
              </Typography>
              {logsExpanded ? (
                <ExpandLessIcon fontSize="small" sx={{ color: 'text.secondary' }} />
              ) : (
                <ExpandMoreIcon fontSize="small" sx={{ color: 'text.secondary' }} />
              )}
            </Box>
            <Collapse in={logsExpanded}>
              <Paper
                variant="outlined"
                sx={{
                  mt: 1,
                  p: 1.5,
                  bgcolor: 'grey.50',
                  fontFamily: 'monospace',
                  fontSize: '0.72rem',
                  overflow: 'auto',
                  maxHeight: 200,
                }}
              >
                {conditions.map((c, idx) => (
                  <Box
                    key={idx}
                    sx={{
                      color: c.type === 'Failed' || c.status === 'False' ? 'error.main' : 'text.primary',
                      mb: 0.25,
                    }}
                  >
                    [{String(c.type)}] status={String(c.status)} reason={String(c.reason)}{' '}
                    {c.message ? `— ${String(c.message)}` : ''}
                  </Box>
                ))}
              </Paper>
            </Collapse>
          </Box>
        )}
      </Box>
    </Box>
  )
}
