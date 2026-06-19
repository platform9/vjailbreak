import { Box, Divider, Typography } from '@mui/material'
import { Migration } from '../../api/migrations'
import { MigrationDetailResources } from 'src/hooks/api/useMigrationDetailResourcesQuery'
import { calculateTimeElapsed } from 'src/utils'
import { VMwareCreds } from 'src/api/vmware-creds/model'
import { OpenstackCreds } from 'src/api/openstack-creds/model'

function KpiCell({
  label,
  value,
  sub,
  mono = false,
}: {
  label: string
  value: string
  sub?: string
  mono?: boolean
}) {
  return (
    <Box sx={{ px: 2.5, py: 1.5, flex: 1, minWidth: 0 }}>
      <Typography
        variant="caption"
        color="text.secondary"
        sx={{
          display: 'block',
          textTransform: 'uppercase',
          letterSpacing: 0.8,
          fontSize: '0.7rem',
          fontWeight: 600,
          mb: 0.25,
        }}
      >
        {label}
      </Typography>
      <Typography
        variant="body2"
        fontWeight={600}
        sx={{
          display: 'block',
          ...(mono && { fontFamily: '"Fira Code", monospace', fontSize: '0.8rem' }),
        }}
        noWrap
        title={value}
      >
        {value}
      </Typography>
      {sub && (
        <Typography
          variant="caption"
          color="text.secondary"
          sx={{ display: 'block', ...(mono && { fontFamily: '"Fira Code", monospace' }) }}
          noWrap
        >
          {sub}
        </Typography>
      )}
    </Box>
  )
}

interface MigrationKpiStripProps {
  migration: Migration
  resources?: MigrationDetailResources | null
}

export default function MigrationKpiStrip({ migration, resources }: MigrationKpiStripProps) {
  const creationTs = migration.metadata?.creationTimestamp
  const status = migration.status

  const startedAt = creationTs
    ? new Date(creationTs).toLocaleString(undefined, {
        month: 'short',
        day: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
      })
    : '—'

  const elapsed = creationTs ? calculateTimeElapsed(creationTs.toString(), status) : '—'

  const isFailed = status?.phase === 'Failed' || status?.phase === 'ValidationFailed'
  const remaining = status?.phase === 'Succeeded' ? 'Completed' : isFailed ? 'Halted' : '—'

  const vmwareCreds = resources?.vmwareCreds as VMwareCreds | null | undefined
  const sourceValue =
    vmwareCreds?.spec?.hostName ||
    vmwareCreds?.spec?.datacenter ||
    resources?.vmwareCredsRef ||
    '—'

  const openstackCreds = resources?.openstackCreds as OpenstackCreds | null | undefined
  const destValue =
    openstackCreds?.spec?.projectName ||
    resources?.openstackCredsRef ||
    '—'

  const agentValue = status?.agentName || '—'

  const cells = [
    { label: 'Started',       value: startedAt,   mono: false },
    { label: 'Total Elapsed', value: elapsed,      mono: false },
    { label: 'Remaining',     value: remaining,    mono: false },
    { label: 'Source',        value: sourceValue,  mono: true  },
    { label: 'Destination',   value: destValue,    mono: true  },
    { label: 'Agent',         value: agentValue,   mono: true  },
  ]

  return (
    <Box
      sx={{
        display: 'flex',
        bgcolor: 'background.paper',
        borderRadius: 2,
        border: '1px solid',
        borderColor: 'divider',
        mb: 2,
        overflow: 'hidden',
      }}
    >
      {cells.map((cell, idx) => (
        <Box key={cell.label} sx={{ display: 'flex', flex: 1, minWidth: 0 }}>
          {idx > 0 && <Divider orientation="vertical" flexItem />}
          <KpiCell label={cell.label} value={cell.value} mono={cell.mono} />
        </Box>
      ))}
    </Box>
  )
}
