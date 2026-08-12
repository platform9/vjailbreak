import React, { useState } from 'react'
import {
  IconButton,
  Tooltip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  Box,
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
            <Typography variant="body2" gutterBottom>
              <strong>{migrationName}</strong> has migrated and is running, but on an emulated SATA
              disk controller. Moving it to virtio gives noticeably better disk performance and
              lifts the six-disk limit SATA imposes — but only if Windows has picked up the virtio
              storage driver.
            </Typography>

            <Typography variant="body2" sx={{ mt: 2 }} gutterBottom>
              <strong>Step 1 — check inside the guest.</strong> Log in and run:
            </Typography>
            <Box
              component="pre"
              sx={{
                mt: 1,
                mb: 1.5,
                p: 1.5,
                borderRadius: 1,
                bgcolor: 'action.hover',
                fontSize: '0.75rem',
                overflowX: 'auto'
              }}
            >
              {`sc.exe query viostor`}
            </Box>

            <Typography variant="body2" gutterBottom>
              <strong>Step 2 — choose based on what it says.</strong>
            </Typography>
            <Box component="ul" sx={{ pl: 2.5, mt: 0.5, mb: 1.5 }}>
              <li>
                <Typography variant="body2">
                  <code>STATE: 4 RUNNING</code> → <strong>Move to virtio</strong>
                </Typography>
              </li>
              <li>
                <Typography variant="body2">
                  Anything else, or the service does not exist → <strong>Keep on SATA</strong>. The
                  migration still completes successfully; the VM simply stays as it is.
                </Typography>
              </li>
            </Box>

            <InlineHelp tone="default" icon="info" sx={{ mt: 1 }}>
              <strong>Move to virtio restarts the VM.</strong> It is shut down cleanly, deleted and
              recreated with the same name, IP and MAC — expect a short outage. Leave the VM running
              now; the shutdown is handled for you. There is no time limit — the migration waits here
              until you answer, so you can come back during a maintenance window.
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
              title="This destroys the migrated VM"
              message={
                <>
                  The VM and its volumes are deleted and the migration is marked failed. You would
                  need to migrate this VM again from the beginning.
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
            {confirmingFailure ? 'Yes, fail and clean up' : 'Fail migration'}
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
