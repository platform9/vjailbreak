import { useState } from 'react'
import {
  Box,
  Button,
  Collapse,
  Link,
  Paper,
  Tooltip,
  Typography,
} from '@mui/material'
import WarningAmberIcon from '@mui/icons-material/WarningAmber'
import ContentCopyIcon from '@mui/icons-material/ContentCopy'
import ExpandMoreIcon from '@mui/icons-material/ExpandMore'
import ExpandLessIcon from '@mui/icons-material/ExpandLess'
import OpenInNewIcon from '@mui/icons-material/OpenInNew'
import { Migration, Condition } from '../../api/migrations'

const TROUBLESHOOTING_URL =
  'https://platform9.github.io/vjailbreak/guides/troubleshooting/troubleshooting/'


const GENERIC_STEPS = [
  {
    title: 'Review the error message and pod logs',
    body: 'Go to the Pod logs tab and filter by ERROR or FATAL to find the root cause.',
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

  const failedConditionCount = conditions.filter(
    (c) => c.status === 'False' || c.type === 'Failed'
  ).length

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
        mb: 2,
        overflow: 'hidden',
      }}
    >
      {/* Header */}
      <Box
        sx={{
          px: 3,
          pt: 2.5,
          pb: 2,
          borderBottom: '1px solid',
          borderColor: 'divider',
          bgcolor: (theme) =>
            theme.palette.mode === 'dark'
              ? 'rgba(211, 47, 47, 0.08)'
              : 'rgba(211, 47, 47, 0.04)',
        }}
      >
        {/* Phase + timestamp row */}
        <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 1 }}>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
            <WarningAmberIcon sx={{ color: 'error.main', fontSize: 18 }} />
            <Typography
              variant="caption"
              sx={{
                bgcolor: (theme) =>
                  theme.palette.mode === 'dark' ? 'rgba(211,47,47,0.2)' : 'rgba(211,47,47,0.1)',
                color: 'error.main',
                fontWeight: 700,
                px: 0.75,
                py: 0.25,
                borderRadius: 0.5,
                letterSpacing: 0.5,
              }}
            >
              {phase}
            </Typography>
            <Typography variant="caption" color="text.disabled">·</Typography>
            <Typography variant="caption" color="text.secondary">
              {errorTimestamp}
            </Typography>
          </Box>

          <Tooltip title={copied ? 'Copied!' : 'Copy diagnostic bundle'}>
            <Button
              size="small"
              variant="outlined"
              color="inherit"
              startIcon={<ContentCopyIcon fontSize="small" />}
              onClick={handleCopy}
              sx={{ fontSize: '0.75rem', py: 0.5 }}
            >
              {copied ? 'Copied!' : 'Copy diagnostic bundle'}
            </Button>
          </Tooltip>
        </Box>

        {/* Error title */}
        <Typography
          variant="body1"
          fontWeight={700}
          color="error.main"
          sx={{ wordBreak: 'break-word', lineHeight: 1.5 }}
        >
          {errorTitle}
        </Typography>
      </Box>

      {/* Body */}
      <Box sx={{ px: 3, py: 2.5, display: 'grid', gap: 2.5 }}>
        {/* Recommended resolution */}
        <Box>
          <Typography
            variant="overline"
            color="text.secondary"
            sx={{ display: 'block', mb: 1.5, letterSpacing: 1 }}
          >
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

        {/* Troubleshooting link */}
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
          <Typography variant="body2" color="text.secondary">
            Need more help?
          </Typography>
          <Link
            href={TROUBLESHOOTING_URL}
            target="_blank"
            rel="noopener noreferrer"
            variant="body2"
            sx={{ display: 'inline-flex', alignItems: 'center', gap: 0.25 }}
          >
            Troubleshooting guide
            <OpenInNewIcon sx={{ fontSize: 13 }} />
          </Link>
        </Box>

        {/* Collapsible raw log lines */}
        {conditions.length > 0 && (
          <Box sx={{ borderTop: '1px solid', borderColor: 'divider', pt: 1.5 }}>
            <Box
              sx={{ display: 'flex', alignItems: 'center', cursor: 'pointer', py: 0.5 }}
              onClick={() => setLogsExpanded((v) => !v)}
            >
              <Typography variant="caption" color="text.secondary" sx={{ flex: 1 }}>
                Show raw log lines from the failure ({failedConditionCount})
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
                      color:
                        c.type === 'Failed' || c.status === 'False'
                          ? 'error.main'
                          : 'text.primary',
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
