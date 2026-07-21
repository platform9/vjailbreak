import {
  Alert,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
} from '@mui/material'
import WarningAmberIcon from '@mui/icons-material/WarningAmber'
import { Migration } from '../api/migrations'
import { useDeleteMigrations } from '../hooks/useDeleteMigrations'

interface DeleteMigrationDialogProps {
  open: boolean
  onClose: () => void
  migrations: Migration[]
  onSuccess?: () => void
}

export default function DeleteMigrationDialog({
  open,
  onClose,
  migrations,
  onSuccess,
}: DeleteMigrationDialogProps) {
  const { deleteMigrations, isDeleting, error, setError } = useDeleteMigrations()

  const handleClose = () => {
    if (isDeleting) return
    setError(null)
    onClose()
  }

  const handleConfirm = async () => {
    const success = await deleteMigrations(migrations)
    if (success) {
      setError(null)
      onClose()
      onSuccess?.()
    }
  }

  const isBulk = migrations.length > 1
  const vmName = migrations[0]?.spec?.vmName || migrations[0]?.metadata?.name || ''

  return (
    <Dialog open={open} onClose={handleClose} maxWidth="xs" fullWidth>
      <DialogTitle sx={{ px: 3, pt: 3, pb: 1, display: 'flex', alignItems: 'center', gap: 1 }}>
        <WarningAmberIcon color="warning" fontSize="small" />
        {isBulk ? `Delete ${migrations.length} migrations?` : 'Delete migration?'}
      </DialogTitle>
      <DialogContent sx={{ px: 3, pb: 2 }}>
        <DialogContentText>
          {isBulk
            ? `This will delete ${migrations.length} migration objects. Source VMs will not be modified. This action cannot be undone.`
            : <>This will delete the migration object for <strong>{vmName}</strong>. The source VM will not be modified. This action cannot be undone.</>
          }
        </DialogContentText>
        {error && (
          <Alert severity="error" sx={{ mt: 2 }} onClose={() => setError(null)}>
            {error}
          </Alert>
        )}
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 3, gap: 1 }}>
        <Button onClick={handleClose} disabled={isDeleting}>
          Cancel
        </Button>
        <Button
          variant="outlined"
          color="error"
          onClick={handleConfirm}
          disabled={isDeleting}
          data-testid="confirm-delete-button"
        >
          {isDeleting ? 'Deleting…' : 'Delete'}
        </Button>
      </DialogActions>
    </Dialog>
  )
}
