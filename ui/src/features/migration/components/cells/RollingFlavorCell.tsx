import { Box, Select, MenuItem, Typography } from '@mui/material'
import type { OpenStackFlavor } from 'src/api/openstack-creds/model'

export interface RollingFlavorCellProps {
  vmId: string
  currentFlavor: string
  isSelected: boolean
  openstackFlavors: OpenStackFlavor[]
  onFlavorChange: (vmId: string, flavorId: string) => void
}

export function RollingFlavorCell({
  vmId,
  currentFlavor,
  isSelected,
  openstackFlavors,
  onFlavorChange,
}: RollingFlavorCellProps) {
  if (isSelected) {
    return (
      <Box sx={{ display: 'flex', alignItems: 'center', height: '100%', width: '100%' }}>
        <Select
          size="small"
          value={(() => {
            if (currentFlavor === 'auto-assign') return 'auto-assign'
            const flavorByName = openstackFlavors.find((f) => f.name === currentFlavor)
            const flavorById = openstackFlavors.find((f) => f.id === currentFlavor)
            return flavorByName?.id || flavorById?.id || currentFlavor
          })()}
          onChange={(e) => onFlavorChange(vmId, e.target.value)}
          displayEmpty
          sx={{
            minWidth: 120,
            width: '100%',
            '& .MuiSelect-select': { padding: '4px 8px', fontSize: '0.875rem' },
          }}
        >
          <MenuItem value="auto-assign">
            <Typography variant="body2">Auto Assign</Typography>
          </MenuItem>
          {openstackFlavors.map((flavor) => (
            <MenuItem key={flavor.id} value={flavor.id}>
              <Typography variant="body2">{flavor.name}</Typography>
            </MenuItem>
          ))}
        </Select>
      </Box>
    )
  }

  return (
    <Box
      sx={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        height: '100%',
      }}
    >
      <Typography variant="body2">{currentFlavor}</Typography>
    </Box>
  )
}
