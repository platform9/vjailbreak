import * as React from 'react'
import { Box, Select, MenuItem, Typography, Tooltip } from '@mui/material'
import WarningIcon from '@mui/icons-material/Warning'
import WindowsIcon from 'src/assets/windows_icon.svg'
import LinuxIcon from 'src/assets/linux_icon.svg'

export interface OsFamilyCellProps {
  vmId: string
  powerState: string
  detectedOsFamily?: string
  assignedOsFamily?: string
  /** Standard: pass `isSelected`. Rolling: pass `isSelected && powerState === 'powered-off'`. */
  showSelectWhenSelected: boolean
  /** Rolling uses 'Other'; standard uses 'Unknown' (default). */
  unknownFallbackLabel?: string
  /** Rolling: show warning icon only for powered-off VMs. Standard: always show. */
  showWarningForPoweredOffOnly?: boolean
  /** Standard passes keepMounted=true; rolling omits it. */
  keepSelectMenuMounted?: boolean
  onOSAssignment: (vmId: string, osFamily: string) => void
}

export function OsFamilyCell({
  vmId,
  powerState,
  detectedOsFamily,
  assignedOsFamily,
  showSelectWhenSelected,
  unknownFallbackLabel = 'Unknown',
  showWarningForPoweredOffOnly = false,
  keepSelectMenuMounted = false,
  onOSAssignment,
}: OsFamilyCellProps) {
  const currentOsFamily = assignedOsFamily === undefined ? detectedOsFamily : assignedOsFamily

  if (showSelectWhenSelected) {
    return (
      <Box sx={{ display: 'flex', alignItems: 'center', height: '100%' }}>
        <Select
          size="small"
          value={(() => {
            if (!currentOsFamily || currentOsFamily === 'Unknown') return ''
            const osLower = currentOsFamily.toLowerCase()
            if (osLower.includes('windows')) return 'windowsGuest'
            if (osLower.includes('linux')) return 'linuxGuest'
            return ''
          })()}
          onChange={(e) => onOSAssignment(vmId, e.target.value)}
          displayEmpty
          sx={{
            minWidth: 120,
            '& .MuiSelect-select': { padding: '4px 8px', fontSize: '0.875rem' },
          }}
          MenuProps={keepSelectMenuMounted ? { keepMounted: true } : undefined}
        >
          <MenuItem value="">
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, color: 'text.secondary' }}>
              <WarningIcon sx={{ fontSize: 16 }} />
              <em>Select OS</em>
            </Box>
          </MenuItem>
          <MenuItem value="windowsGuest">
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
              <img src={WindowsIcon} alt="Windows" style={{ width: 16, height: 16 }} />
              Windows
            </Box>
          </MenuItem>
          <MenuItem value="linuxGuest">
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
              <img src={LinuxIcon} alt="Linux" style={{ width: 16, height: 16 }} />
              Linux
            </Box>
          </MenuItem>
        </Select>
      </Box>
    )
  }

  let displayValue = currentOsFamily || 'Unknown'
  let icon: React.ReactNode = null
  if (currentOsFamily && currentOsFamily.toLowerCase().includes('windows')) {
    displayValue = 'Windows'
    icon = <img src={WindowsIcon} alt="Windows" style={{ width: 20, height: 20 }} />
  } else if (currentOsFamily && currentOsFamily.toLowerCase().includes('linux')) {
    displayValue = 'Linux'
    icon = <img src={LinuxIcon} alt="Linux" style={{ width: 20, height: 20 }} />
  } else if (currentOsFamily && currentOsFamily !== 'Unknown') {
    displayValue = unknownFallbackLabel
  }

  const showWarning = showWarningForPoweredOffOnly
    ? powerState === 'powered-off' && (!currentOsFamily || currentOsFamily === 'Unknown')
    : !currentOsFamily || currentOsFamily === 'Unknown'

  return (
    <Tooltip
      title={
        powerState === 'powered-off'
          ? !currentOsFamily || currentOsFamily === 'Unknown'
            ? 'OS assignment required for powered-off VMs'
            : 'Click to change OS selection'
          : displayValue
      }
    >
      <Box sx={{ display: 'flex', alignItems: 'center', height: '100%', gap: 1 }}>
        {icon}
        {showWarning && <WarningIcon sx={{ color: 'warning.main', fontSize: 16 }} />}
        <Typography
          variant="body2"
          sx={{
            color:
              !currentOsFamily || currentOsFamily === 'Unknown' ? 'text.secondary' : 'text.primary',
          }}
        >
          {displayValue}
        </Typography>
      </Box>
    </Tooltip>
  )
}
