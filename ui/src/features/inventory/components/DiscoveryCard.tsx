import { Avatar, Box, Divider, Stack, Typography } from '@mui/material'
import Inventory2OutlinedIcon from '@mui/icons-material/Inventory2Outlined'
import DnsOutlinedIcon from '@mui/icons-material/DnsOutlined'
import LayersOutlinedIcon from '@mui/icons-material/LayersOutlined'
import CheckCircleOutlineIcon from '@mui/icons-material/CheckCircleOutline'
import RadioButtonUncheckedIcon from '@mui/icons-material/RadioButtonUnchecked'
import { SurfaceCard } from 'src/components'
import { ReactNode } from 'react'

export interface DiscoveryCardProps {
  vmCount: number
  credName?: string
  bucketCount?: number
  bucketedVmCount?: number
}

function Stat({ icon, value, label }: { icon: ReactNode; value: ReactNode; label: string }) {
  return (
    <Stack direction="row" spacing={1.5} alignItems="center" sx={{ minWidth: 130 }}>
      <Box sx={{ color: 'text.secondary', display: 'flex' }}>{icon}</Box>
      <Box>
        <Typography variant="h6" sx={{ lineHeight: 1.1, fontWeight: 700 }}>
          {value}
        </Typography>
        <Typography variant="caption" color="text.secondary">
          {label}
        </Typography>
      </Box>
    </Stack>
  )
}

/**
 * Discovery summary: a prominent header (icon + VM count + credential) and a stats strip
 * (total VMs / in buckets / unbucketed / buckets). Presentational.
 */
export default function DiscoveryCard({
  vmCount,
  credName,
  bucketCount = 0,
  bucketedVmCount = 0
}: DiscoveryCardProps) {
  const unbucketed = Math.max(0, vmCount - bucketedVmCount)

  return (
    <SurfaceCard
      variant="card"
      data-testid="discovery-card"
      sx={{
        background: (theme) =>
          `linear-gradient(135deg, ${theme.palette.action.hover}, transparent 60%)`
      }}
    >
      <Stack direction="row" spacing={2} alignItems="center">
        <Avatar
          variant="rounded"
          sx={{ bgcolor: 'primary.main', color: 'primary.contrastText', width: 48, height: 48 }}
        >
          <Inventory2OutlinedIcon />
        </Avatar>
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <Typography variant="h5" sx={{ fontWeight: 700, lineHeight: 1.2 }}>
            {vmCount} VM{vmCount === 1 ? '' : 's'} discovered
          </Typography>
          <Typography variant="body2" color="text.secondary">
            {credName ? `from credential "${credName}"` : 'No VMware credential connected'}
          </Typography>
        </Box>
      </Stack>

      <Divider sx={{ my: 0.5 }} />

      <Stack direction="row" spacing={3} flexWrap="wrap" useFlexGap>
        <Stat icon={<DnsOutlinedIcon />} value={vmCount} label="Total VMs" />
        <Stat icon={<CheckCircleOutlineIcon />} value={bucketedVmCount} label="In buckets" />
        <Stat icon={<RadioButtonUncheckedIcon />} value={unbucketed} label="Unbucketed" />
        <Stat icon={<LayersOutlinedIcon />} value={bucketCount} label="Buckets" />
      </Stack>
    </SurfaceCard>
  )
}
