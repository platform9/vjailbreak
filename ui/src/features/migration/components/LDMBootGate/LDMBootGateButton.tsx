import React, { useState } from 'react'
import {
  IconButton,
  Tooltip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  Typography,
  CircularProgress
} from '@mui/material'
import PlayArrowIcon from '@mui/icons-material/PlayArrow'
import { ActionButton, Banner, InlineHelp } from 'src/components/design-system'
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
 * is a successful outcome and "Rollback Migration" destroys a working VM, and that
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
      <Tooltip title="Action needed: move this VM to virtio, or keep it on SATA">
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
        <DialogTitle sx={{ px: 3, pt: 3, pb: 1 }}>Move this VM to virtio?</DialogTitle>
        <DialogContent sx={{ px: 3, pb: 2 }}>
          <DialogContentText component="div">
            <Typography variant="body2">
              <strong>{migrationName}</strong> has migrated and is running on an emulated SATA
              controller. Moving it to virtio improves disk performance and lifts SATA&apos;s
              six-disk limit. Keeping it on SATA completes the migration as it is.
            </Typography>

            <InlineHelp tone="default" icon="info" sx={{ mt: 2 }}>
              Moving to virtio shuts the VM down, deletes and recreates it with the same name, IP
              and MAC — expect a short outage. Leave it running; the shutdown is handled for you.
              There is no time limit, so you can come back during a maintenance window.
            </InlineHelp>
          </DialogContentText>

          {error && (
            <InlineHelp tone="critical" icon="danger" sx={{ mt: 2 }}>
              {error}
            </InlineHelp>
          )}

          {confirmingFailure && (
            <Banner
              variant="error"
              title="This rolls back and destroys the migrated VM"
              message={
                <>
                  The VM and its volumes are deleted, and the migration is marked failed. You
                  would need to migrate this VM again from the beginning.
                  <br />
                  <br />
                  Only use this if the guest is genuinely unusable. If it boots at all, choose{' '}
                  <strong>Keep on SATA</strong> — that keeps the VM and completes the migration
                  successfully.
                </>
              }
              sx={{ mt: 2 }}
            />
          )}
        </DialogContent>

        <DialogActions sx={{ px: 3, pb: 3, gap: 1, flexWrap: 'wrap' }}>
          <ActionButton
            data-testid="ldm-gate-fail-button"
            onClick={() => (confirmingFailure ? answer('failed') : setConfirmingFailure(true))}
            tone="danger"
            variant="text"
            loading={pending === 'failed'}
            disabled={busy}
            sx={{ mr: 'auto' }}
          >
            {confirmingFailure ? 'Yes, roll back and clean up' : 'Rollback Migration'}
          </ActionButton>
          <ActionButton tone="secondary" variant="text" onClick={() => setOpen(false)} disabled={busy}>
            Cancel
          </ActionButton>
          <ActionButton
            data-testid="ldm-gate-finish-button"
            onClick={() => answer('finish')}
            tone="secondary"
            variant="outlined"
            loading={pending === 'finish'}
            disabled={busy}
          >
            Keep on SATA
          </ActionButton>
          <ActionButton
            data-testid="ldm-gate-success-button"
            onClick={() => answer('success')}
            tone="primary"
            variant="contained"
            loading={pending === 'success'}
            disabled={busy}
          >
            Move to virtio
          </ActionButton>
        </DialogActions>
      </Dialog>
    </>
  )
}
