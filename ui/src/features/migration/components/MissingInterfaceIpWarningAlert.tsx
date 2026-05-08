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

  const visibleWarnings = warnings.slice(0, 5)
  const hiddenWarningCount = warnings.length - visibleWarnings.length

  return (
    <Alert severity="warning" sx={sx} data-testid="missing-interface-ip-warning">
      <Box sx={{ display: 'grid', gap: 0.75 }}>
        <Typography variant="body2" sx={{ fontWeight: 600 }}>
          Some selected VM interfaces do not have IP addresses from vCenter.
        </Typography>
        <Typography variant="body2">
          If these interfaces have IPs on the source VMs, wait until they appear in the vCenter UI,
          then revalidate the VMware credential so the latest IP details sync before migration.
        </Typography>
        <Typography variant="body2">IP addresses are not found for the following:</Typography>
        <Box component="ul" sx={{ m: 0, pl: 3, listStyleType: 'disc' }}>
          {visibleWarnings.map((warning) => (
            <Typography component="li" variant="body2" key={warning.key} sx={{ display: 'list-item' }}>
              Interface with MAC {warning.macAddress} for VM {warning.vmName}
            </Typography>
          ))}
          {hiddenWarningCount > 0 && (
            <Typography component="li" variant="body2" sx={{ display: 'list-item' }}>
              {hiddenWarningCount} more interface{hiddenWarningCount === 1 ? '' : 's'} with missing
              IP address information.
            </Typography>
          )}
        </Box>
      </Box>
    </Alert>
  )
}
