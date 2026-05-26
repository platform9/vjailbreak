import {
  Autocomplete,
  Box,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormLabel,
  Typography
} from '@mui/material'
import { TextField as SharedTextField } from 'src/shared/components/forms'
import { ActionButton } from 'src/components'
import { OpenStackFlavor } from 'src/api/openstack-creds/model'

export interface FlavorAssignmentDialogProps {
  open: boolean
  selectedVMCount: number
  flavors: OpenStackFlavor[]
  selectedFlavor: string
  updating: boolean
  onClose: () => void
  onApply: () => void
  onFlavorChange: (flavorId: string) => void
}

const AUTO_ASSIGN_OPTION: OpenStackFlavor = {
  id: 'auto-assign',
  name: 'Auto-assign',
  vcpus: 0,
  ram: 0,
  disk: 0
}

export function FlavorAssignmentDialog({
  open,
  selectedVMCount,
  flavors,
  selectedFlavor,
  updating,
  onClose,
  onApply,
  onFlavorChange
}: FlavorAssignmentDialogProps) {
  const allOptions = [AUTO_ASSIGN_OPTION, ...flavors]
  const value = selectedFlavor
    ? (allOptions.find((f) => f.id === selectedFlavor) ?? null)
    : null

  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="sm">
      <DialogTitle>
        Assign Flavor to {selectedVMCount} {selectedVMCount === 1 ? 'VM' : 'VMs'}
      </DialogTitle>
      <DialogContent>
        <Box sx={{ my: 2 }}>
          <FormLabel>Select Flavor</FormLabel>
          <Autocomplete
            sx={{ mt: 1 }}
            size="small"
            options={allOptions}
            value={value}
            onChange={(_e, option) => onFlavorChange(option?.id ?? '')}
            getOptionLabel={(option) => {
              if (option.id === 'auto-assign') return option.name
              return `${option.name} (${option.vcpus} vCPU, ${option.ram}MB RAM, ${option.disk}GB Disk)`
            }}
            isOptionEqualToValue={(option, val) => option.id === val.id}
            renderOption={(props, option) => (
              <li {...props} key={option.id}>
                <Box sx={{ display: 'flex', flexDirection: 'column' }}>
                  <Typography variant="body1">{option.name}</Typography>
                  {option.id !== 'auto-assign' && (
                    <Typography variant="caption" color="text.secondary">
                      {option.vcpus} vCPU, {option.ram / 1024}GB RAM, {option.disk}GB Storage
                    </Typography>
                  )}
                  {option.id === 'auto-assign' && (
                    <Typography variant="caption" color="text.secondary">
                      Let OpenStack automatically assign the most suitable flavor
                    </Typography>
                  )}
                </Box>
              </li>
            )}
            renderInput={(params) => (
              <SharedTextField {...params} placeholder="Search flavors" fullWidth />
            )}
          />
        </Box>
      </DialogContent>
      <DialogActions
        sx={{ justifyContent: 'flex-end', alignItems: 'center', gap: 1, px: 3, py: 2 }}
      >
        <ActionButton tone="secondary" onClick={onClose} disabled={updating}>
          Cancel
        </ActionButton>
        <ActionButton
          tone="primary"
          onClick={onApply}
          disabled={!selectedFlavor || updating}
          loading={updating}
        >
          Apply to selected VMs
        </ActionButton>
      </DialogActions>
    </Dialog>
  )
}
