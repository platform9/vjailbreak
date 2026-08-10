import React, { useState } from 'react'
import {
  IconButton,
  Tooltip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  Button,
  Alert,
  AlertTitle,
  Box,
  Typography,
  CircularProgress
} from '@mui/material'
import PlayArrowIcon from '@mui/icons-material/PlayArrow'
import { setLDMBootStatus, type LDMBootStatus } from '../../api/migrations'

interface LDMBootGateButtonProps {
  migrationName: string
  namespace?: string
  onSuccess?: () => void
  onError?: (error: string) => void
}

/**
 * Answers the WaitingForLDMBootSuccess gate.
 *
 * Only reached by a guest whose system volume is on a Windows Dynamic Disk (LDM).
 * virt-v2v cannot convert one, so the VM is created on an emulated SATA bus with a
 * scratch virtio volume attached, and Windows installs the virtio storage driver
 * against that device on first boot. The admin confirms whether that worked.
 *
 * Deliberately three explicit actions rather than a yes/no confirm: "Keep on SATA"
 * is a successful outcome and "Fail migration" destroys a working VM, and that
 * difference cannot survive being collapsed into a single confirm button.
 */
export const LDMBootGateButton: React.FC<LDMBootGateButtonProps> = ({
  migrationName,
  namespace,
  onSuccess,
  onError
}) => {
  const [open, setOpen] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [pending, setPending] = useState<LDMBootStatus | null>(null)
  const [confirmingFailure, setConfirmingFailure] = useState(false)

  const answer = async (status: LDMBootStatus) => {
    setError(null)
    setPending(status)

    try {
      const result = await setLDMBootStatus(namespace || 'migration-system', migrationName, status)

      if (result.success) {
        setOpen(false)
        setConfirmingFailure(false)
        onSuccess?.()
      } else {
        setError(result.message || 'Failed to record the answer')
        onError?.(result.message || 'Failed to record the answer')
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Unknown error occurred'
      setError(message)
      onError?.(message)
    } finally {
      setPending(null)
    }
  }

  const busy = pending !== null

  return (
    <>
      <Tooltip title="Confirm virtio driver installation">
        <IconButton
          data-testid="ldm-boot-gate-button"
          onClick={(e) => {
            e.stopPropagation()
            setError(null)
            setConfirmingFailure(false)
            setOpen(true)
          }}
          size="small"
          disabled={busy}
          // Same icon and colour as TriggerAdminCutoverButton: both mean "this
          // migration is paused and needs you".
          sx={{ cursor: 'pointer', color: 'primary.main' }}
        >
          {busy ? <CircularProgress size={16} /> : <PlayArrowIcon />}
        </IconButton>
      </Tooltip>

      <Dialog
        open={open}
        onClose={() => !busy && setOpen(false)}
        maxWidth="sm"
        fullWidth
      >
        <DialogTitle sx={{ px: 3, pt: 3, pb: 1 }}>Confirm virtio driver installation</DialogTitle>
        <DialogContent sx={{ px: 3, pb: 2 }}>
          <DialogContentText component="div">
            <Typography variant="body2" gutterBottom>
              The system volume of <strong>{migrationName}</strong> is on a Windows Dynamic Disk
              (LDM), which virt-v2v cannot convert. The VM was created on an emulated SATA bus with
              a scratch virtio disk attached so Windows would install the virtio storage driver on
              first boot.
            </Typography>

            <Typography variant="body2" sx={{ mt: 2 }} gutterBottom>
              Log into the guest and confirm the driver is running:
            </Typography>
            <Box
              component="pre"
              sx={{
                mt: 1,
                p: 1.5,
                borderRadius: 1,
                bgcolor: 'action.hover',
                fontSize: '0.75rem',
                overflowX: 'auto'
              }}
            >
              {`sc.exe query viostor    # STATE: RUNNING`}
            </Box>
            <Typography variant="caption" color="text.secondary">
              Leave the VM running. “Move to virtio” shuts it down cleanly first, then deletes and
              recreates it with the root disk on the virtio bus.
            </Typography>
          </DialogContentText>

          {error && (
            <Alert severity="error" sx={{ mt: 2 }}>
              {error}
            </Alert>
          )}

          {confirmingFailure && (
            <Alert severity="error" sx={{ mt: 2 }}>
              <AlertTitle>This deletes the migrated VM</AlertTitle>
              The VM and its volumes will be removed. If the guest boots at all, choose “Keep on
              SATA” instead — that completes the migration successfully.
            </Alert>
          )}
        </DialogContent>

        <DialogActions sx={{ px: 3, pb: 3, gap: 1, flexWrap: 'wrap' }}>
          <Button
            data-testid="ldm-gate-fail-button"
            onClick={() => (confirmingFailure ? answer('failed') : setConfirmingFailure(true))}
            color="error"
            disabled={busy}
            sx={{ mr: 'auto' }}
          >
            {confirmingFailure ? 'Yes, fail and clean up' : 'Fail migration'}
          </Button>
          <Button onClick={() => setOpen(false)} disabled={busy}>
            Cancel
          </Button>
          <Button
            data-testid="ldm-gate-finish-button"
            onClick={() => answer('finish')}
            variant="outlined"
            disabled={busy}
          >
            Keep on SATA
          </Button>
          <Button
            data-testid="ldm-gate-success-button"
            onClick={() => answer('success')}
            variant="contained"
            color="primary"
            disabled={busy}
          >
            Move to virtio
          </Button>
        </DialogActions>
      </Dialog>
    </>
  )
}
