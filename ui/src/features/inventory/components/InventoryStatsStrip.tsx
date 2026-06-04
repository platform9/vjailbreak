import { Box, Paper, Typography, Tooltip } from '@mui/material'
import DnsOutlinedIcon from '@mui/icons-material/DnsOutlined'
import CheckCircleOutlineIcon from '@mui/icons-material/CheckCircleOutline'
import RadioButtonUncheckedIcon from '@mui/icons-material/RadioButtonUnchecked'
import LayersOutlinedIcon from '@mui/icons-material/LayersOutlined'
import HubOutlinedIcon from '@mui/icons-material/HubOutlined'
import type { ReactNode } from 'react'

export interface InventoryStatsStripProps {
  totalVms: number
  inBuckets: number
  unbucketed: number
  bucketCount: number
  /** Number of migration agents (VjailbreakNode workers + master). */
  agentCount: number
  credName?: string
}

function Stat({
  icon,
  value,
  label,
  tooltip
}: {
  icon: ReactNode
  value: number
  label: string
  tooltip?: string
}) {
  const content = (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
      <Box sx={{ display: 'flex', color: 'text.secondary' }}>{icon}</Box>
      <Box sx={{ display: 'flex', alignItems: 'baseline', gap: 0.75 }}>
        <Typography variant="subtitle1" sx={{ fontWeight: 700, lineHeight: 1 }}>
          {value}
        </Typography>
        <Typography variant="caption" color="text.secondary" sx={{ whiteSpace: 'nowrap' }}>
          {label}
        </Typography>
      </Box>
    </Box>
  )
  return tooltip ? (
    <Tooltip title={tooltip} arrow>
      {content}
    </Tooltip>
  ) : (
    content
  )
}

/**
 * Slim summary strip shown above the buckets table: discovery + bucketing + agent counts.
 * Compact by design so it reads like a toolbar rather than a card (consistent with the
 * Migrations page density).
 */
export default function InventoryStatsStrip({
  totalVms,
  inBuckets,
  unbucketed,
  bucketCount,
  agentCount,
  credName
}: InventoryStatsStripProps) {
  const iconSx = { fontSize: 20 }
  return (
    <Paper
      variant="outlined"
      sx={{
        px: 2,
        py: 1.25,
        display: 'flex',
        alignItems: 'center',
        flexWrap: 'wrap',
        gap: { xs: 2, md: 4 },
        borderRadius: 1
      }}
    >
      <Stat
        icon={<DnsOutlinedIcon sx={iconSx} />}
        value={totalVms}
        label="Total VMs"
        tooltip={credName ? `Discovered from credential "${credName}"` : undefined}
      />
      <Stat icon={<CheckCircleOutlineIcon sx={iconSx} />} value={inBuckets} label="In buckets" />
      <Stat
        icon={<RadioButtonUncheckedIcon sx={iconSx} />}
        value={unbucketed}
        label="Unbucketed"
      />
      <Stat icon={<LayersOutlinedIcon sx={iconSx} />} value={bucketCount} label="Buckets" />
      <Stat
        icon={<HubOutlinedIcon sx={iconSx} />}
        value={agentCount}
        label="Agents"
        tooltip="Migration agents available (VjailbreakNodes)"
      />
    </Paper>
  )
}
