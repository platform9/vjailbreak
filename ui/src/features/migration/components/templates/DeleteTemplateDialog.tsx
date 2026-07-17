import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle
} from '@mui/material'
import WarningAmberIcon from '@mui/icons-material/WarningAmber'
import type { SavedTemplate } from '../../mock-templates/types'

export interface DeleteTemplateDialogProps {
  open: boolean
  template: SavedTemplate | null
  onClose: () => void
  onConfirm: () => void
  isDeleting?: boolean
}

export default function DeleteTemplateDialog({
  open,
  template,
  onClose,
  onConfirm,
  isDeleting = false
}: DeleteTemplateDialogProps) {
  return (
    <Dialog open={open} onClose={isDeleting ? undefined : onClose} maxWidth="xs" fullWidth>
      <DialogTitle sx={{ px: 3, pt: 3, pb: 1, display: 'flex', alignItems: 'center', gap: 1 }}>
        <WarningAmberIcon color="warning" fontSize="small" />
        Delete template?
      </DialogTitle>
      <DialogContent sx={{ px: 3, pb: 2 }}>
        <DialogContentText>
          This will permanently delete <strong>{template?.displayName}</strong>. Migrations already
          created from it are not affected. This action cannot be undone.
        </DialogContentText>
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 3, gap: 1 }}>
        <Button onClick={onClose} disabled={isDeleting}>
          Cancel
        </Button>
        <Button
          variant="outlined"
          color="error"
          onClick={onConfirm}
          disabled={isDeleting}
          data-testid="confirm-delete-template-button"
        >
          {isDeleting ? 'Deleting…' : 'Delete'}
        </Button>
      </DialogActions>
    </Dialog>
  )
}
