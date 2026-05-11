import { Alert, Box, SxProps, Theme, Typography } from '@mui/material'
import type { MissingInterfaceIpWarning } from './missingInterfaceIpWarnings'

interface MissingInterfaceIpWarningAlertProps {
  warnings: MissingInterfaceIpWarning[]
  sx?: SxProps<Theme>
}

export function MissingInterfaceIpWarningAlert({
  warnings,
  sx
}: MissingInterfaceIpWarningAlertProps) {
  if (warnings.length === 0) return null

  const vmNames = warnings.map((warning) => warning.vmName).join(', ')

  return (
    <Alert severity="warning" sx={sx} data-testid="missing-interface-ip-warning">
      <Box sx={{ display: 'grid', gap: 0.75 }}>
        <Typography variant="body2" sx={{ fontWeight: 600 }}>
          Some selected VMs are missing IP addresses from vCenter.
        </Typography>
        <Typography variant="body2">
          If these selected VMs have IPs on the source, wait until they appear in the vCenter UI,
          then revalidate the VMware credential so the latest IP details sync before migration.
        </Typography>
        <Typography variant="body2">
          IP addresses are not found for the following VMs: {vmNames}
        </Typography>
      </Box>
    </Alert>
  )
}
