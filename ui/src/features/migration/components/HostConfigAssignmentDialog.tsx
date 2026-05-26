import {
  Box,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Select,
  MenuItem,
  Typography,
  SelectChangeEvent
} from '@mui/material'
import { ActionButton } from 'src/components'
import { OpenstackCreds } from 'src/api/openstack-creds/model'

interface HostConfigAssignmentDialogProps {
  open: boolean
  onClose: () => void
  openstackCredData: OpenstackCreds | null
  selectedPcdHostConfig: string
  onChange: (event: SelectChangeEvent<string>) => void
  onApply: () => void
  loading: boolean
}

export default function HostConfigAssignmentDialog({
  open,
  onClose,
  openstackCredData,
  selectedPcdHostConfig,
  onChange,
  onApply,
  loading
}: HostConfigAssignmentDialogProps) {
  return (
    <Dialog
      open={open}
      onClose={onClose}
      fullWidth
      maxWidth="sm"
      data-testid="rolling-migration-form-host-config-dialog"
    >
      <DialogTitle>Assign Host Config To All ESXi Hosts</DialogTitle>
      <DialogContent>
        <Box sx={{ my: 2 }}>
          <Typography variant="body2" gutterBottom>
            Select Host Configuration
          </Typography>
          <Select
            fullWidth
            value={selectedPcdHostConfig}
            onChange={onChange}
            size="small"
            sx={{ mt: 1 }}
            displayEmpty
            data-testid="rolling-migration-form-host-config-select"
          >
            <MenuItem value="">
              <em>Select a host configuration</em>
            </MenuItem>
            {(openstackCredData?.spec?.pcdHostConfig || []).map((config) => (
              <MenuItem key={config.id} value={config.id}>
                <Box sx={{ display: 'flex', flexDirection: 'column' }}>
                  <Typography variant="body1">{config.name}</Typography>
                  <Typography variant="caption" color="text.secondary">
                    Management Interface: {config.mgmtInterface}
                  </Typography>
                </Box>
              </MenuItem>
            ))}
          </Select>
        </Box>
      </DialogContent>
      <DialogActions sx={{ gap: 1, p: 2 }}>
        <ActionButton
          tone="secondary"
          onClick={onClose}
          data-testid="rolling-migration-form-host-config-cancel"
        >
          Cancel
        </ActionButton>
        <ActionButton
          tone="primary"
          onClick={onApply}
          disabled={!selectedPcdHostConfig || loading}
          loading={loading}
          data-testid="rolling-migration-form-host-config-apply"
        >
          Apply To All Hosts
        </ActionButton>
      </DialogActions>
    </Dialog>
  )
}
