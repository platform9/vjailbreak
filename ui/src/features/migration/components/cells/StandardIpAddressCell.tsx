import { Box, Typography, Tooltip } from '@mui/material'
import type { VmDataWithFlavor } from '../../types'
import { parseIpList } from '../../utils/ipValidation'

export interface StandardIpAddressCellProps {
  vm: VmDataWithFlavor
  isSelected: boolean
  originalIPsPerVM: Record<string, Record<number, string>>
}

export function StandardIpAddressCell({ vm, isSelected, originalIPsPerVM }: StandardIpAddressCellProps) {
  const vmId = vm.id
  const networkInterfaces = Array.isArray(vm.networkInterfaces) ? vm.networkInterfaces : []
  const hasMultipleInterfaces = networkInterfaces.length > 1

  const formatNicIps = (ips?: string[]) => {
    const cleaned = (Array.isArray(ips) ? ips : []).filter((ip) => ip && ip.trim() !== '')
    return cleaned.length > 0 ? cleaned.join(', ') : '—'
  }

  const getNicIpDisplay = (nic: any, index: number) => {
    const preserveIP =
      (vm as any)?.preserveIp?.[index] !== undefined
        ? (vm as any).preserveIp[index] !== false
        : nic?.preserveIP !== false
    if (preserveIP) {
      const original = originalIPsPerVM?.[vmId]?.[index] || ''
      if (original.trim() !== '') return formatNicIps(parseIpList(original))
    }
    return formatNicIps(nic?.ipAddress)
  }

  const ipDisplay = hasMultipleInterfaces
    ? networkInterfaces.map((nic, index) => getNicIpDisplay(nic as any, index)).join(', ')
    : getNicIpDisplay(networkInterfaces[0] as any, 0) !== '—'
      ? getNicIpDisplay(networkInterfaces[0] as any, 0)
      : vm.ipAddress || '—'

  const tooltipMessage = hasMultipleInterfaces
    ? "Use 'Assign IP' button in toolbar to edit IP addresses for multiple network interfaces"
    : "Use 'Assign IP' button in toolbar to assign IP address"

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
