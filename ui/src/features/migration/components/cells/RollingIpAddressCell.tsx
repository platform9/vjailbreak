import { Box, Typography, Tooltip } from '@mui/material'
import type { VM } from '../../types'

export interface RollingIpAddressCellProps {
  vm: VM
  isSelected: boolean
}

export function RollingIpAddressCell({ vm, isSelected }: RollingIpAddressCellProps) {
  const powerState = vm.powerState

  if (powerState === 'powered-off') {
    let ipDisplay = ''
    let tooltipMessage = ''
    if (vm.networkInterfaces && vm.networkInterfaces.length > 1) {
      ipDisplay = vm.networkInterfaces.map((nic: any) => nic.ipAddress || '—').join(', ')
      tooltipMessage =
        "Use 'Assign IP' button in toolbar to edit IP addresses for multiple network interfaces"
    } else {
      ipDisplay = vm.ip || '—'
      tooltipMessage = "Use 'Assign IP' button in toolbar to assign IP address"
    }

    const content = (
      <Box
        sx={{
          display: 'flex',
          alignItems: 'center',
          width: '100%',
          height: '100%',
          gap: 1,
          minWidth: 0,
        }}
      >
        <Typography
          variant="body2"
          sx={{
            fontSize: '0.875rem',
            flex: 1,
            minWidth: 0,
            whiteSpace: 'nowrap',
            overflow: 'hidden',
            textOverflow: 'ellipsis',
          }}
        >
          {ipDisplay}
        </Typography>
      </Box>
    )

    return isSelected ? (
      <Tooltip title={tooltipMessage} arrow placement="top">
        {content}
      </Tooltip>
    ) : (
      content
    )
  }

  const currentIp = vm.ip || '—'

  if (powerState === 'powered-on') {
    return (
      <Tooltip title="IP assignment is only available for powered-off VMs" arrow>
        <Box sx={{ display: 'flex', alignItems: 'center', width: '100%', height: '100%' }}>
          <Typography
            variant="body2"
            sx={{
              flex: 1,
              minWidth: 0,
              whiteSpace: 'nowrap',
              overflow: 'hidden',
              textOverflow: 'ellipsis',
            }}
          >
            {currentIp}
          </Typography>
        </Box>
      </Tooltip>
    )
  }

  return (
    <Box sx={{ display: 'flex', alignItems: 'center', width: '100%', height: '100%' }}>
      <Typography variant="body2">{currentIp}</Typography>
    </Box>
  )
}
