import { useState, useEffect } from 'react'
import { useQuery, useMutation } from '@tanstack/react-query'

import { UpgradeResponse, UpgradeProgressResponse } from 'src/api/version/model'
import Dialog from '@mui/material/Dialog'
import DialogTitle from '@mui/material/DialogTitle'
import Tooltip from '@mui/material/Tooltip'
import DialogContent from '@mui/material/DialogContent'
import DialogActions from '@mui/material/DialogActions'
import Box from '@mui/material/Box'
import Chip from '@mui/material/Chip'
import Typography from '@mui/material/Typography'
import Select from '@mui/material/Select'
import MenuItem from '@mui/material/MenuItem'
import Alert from '@mui/material/Alert'
import CircularProgress from '@mui/material/CircularProgress'
import CheckCircleIcon from '@mui/icons-material/CheckCircle'
import RadioButtonUncheckedIcon from '@mui/icons-material/RadioButtonUnchecked'
import WarningAmberIcon from '@mui/icons-material/WarningAmber'
import { useTheme } from '@mui/material/styles'
import React from 'react'
import { ActionButton, ConfirmationDialog } from 'src/components'
import {
  cleanupApiCall,
  getAvailableTags,
  getUpgradeProgress,
  initiateUpgrade
} from 'src/api/version'

const CLEANUP_DONE_KEY = 'vjailbreak.upgrade.cleanupCompleted'

const CLEANUP_ITEMS = [
  'Delete MigrationPlans',
  'Delete RollingMigrationPlans',
  'Scale down Agents',
  'Delete VMware credentials',
  'Delete PCD credentials',
  'Delete Custom Resources'
]

const getUIStatusMessage = (status: string | undefined): string => {
  switch (status) {
    case 'pending':
      return 'Pending'
    case 'in_progress':
    case 'deploying':
      return 'Upgrading'
    case 'verifying_stability':
      return 'Waiting for services to be ready'
    case 'rolling_back':
      return 'Rolling back'
    case 'completed':
      return 'Upgrade completed'
    case 'rolled_back':
      return 'Rolled back'
    case 'failed':
      return 'Upgrade failed'
    case 'rollback_failed':
      return 'Rollback failed'
    default:
      return 'Processing...'
  }
}

const StepHeader = ({
  index,
  title,
  status,
  statusColor,
  done
}: {
  index: number
  title: string
  status: string
  statusColor: 'default' | 'warning' | 'success'
  done: boolean
}) => (
  <Box display="flex" alignItems="center" gap={1} mb={1.5}>
    <Box
      sx={{
        width: 22,
        height: 22,
        borderRadius: '50%',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        flexShrink: 0,
        fontSize: '0.75rem',
        fontWeight: 600,
        bgcolor: done ? 'success.main' : 'primary.main',
        color: 'common.white'
      }}
    >
      {done ? <CheckCircleIcon sx={{ fontSize: 16 }} /> : index}
    </Box>
    <Typography variant="subtitle2" fontWeight={600} sx={{ flexGrow: 1 }}>
      {title}
    </Typography>
    <Chip size="small" label={status} color={statusColor} variant="outlined" />
  </Box>
)

export const UpgradeModal = ({ show, onClose }) => {
  const [selectedVersion, setSelectedVersion] = useState('')
  const [errorMsg, setErrorMsg] = useState('')
  const [successMsg, setSuccessMsg] = useState('')
  const [upgradeInProgress, setUpgradeInProgress] = useState(false)
  const [cleanUpInProgress, setCleanUpInProgress] = useState(false)
  // Persisted so a page refresh mid-upgrade does not silently drop the cleanup gate.
  // sessionStorage (not localStorage): a brand new tab/session re-requires cleanup.
  const [cleanupCompleted, setCleanupCompleted] = useState(
    () => sessionStorage.getItem(CLEANUP_DONE_KEY) === 'true'
  )
  const [confirmCleanupOpen, setConfirmCleanupOpen] = useState(false)
  const [progressData, setProgressData] = useState<UpgradeProgressResponse | null>(null)
  const theme = useTheme()

  const busy = upgradeInProgress || cleanUpInProgress

  const { data: updates, isLoading: areVersionsLoading } = useQuery({
    queryKey: ['availableTags'],
    queryFn: getAvailableTags,
    enabled: show
  })

  const cleanupMutation = useMutation({ mutationFn: cleanupApiCall })

  // Runs after the confirmation dialog has already closed, so progress is shown on the
  // Cleanup button in this dialog instead of freezing the confirmation dialog.
  const runCleanup = async () => {
    setCleanUpInProgress(true)
    setSuccessMsg('')
    setErrorMsg('')
    try {
      const data = await cleanupMutation.mutateAsync()
      if (!data.success) {
        throw new Error(data.message || 'Cleanup failed')
      }
      sessionStorage.setItem(CLEANUP_DONE_KEY, 'true')
      setCleanupCompleted(true)
      setSuccessMsg('Cleanup completed successfully')
    } catch (error) {
      setErrorMsg(`Cleanup failed: ${error instanceof Error ? error.message : String(error)}`)
    } finally {
      setCleanUpInProgress(false)
    }
  }

  const upgradeMutation = useMutation<UpgradeResponse, Error, void>({
    mutationFn: () => initiateUpgrade(selectedVersion, true),
    onSuccess: (data) => {
      if (data.upgradeStarted) {
        setUpgradeInProgress(true)
        setErrorMsg('')
      } else {
        setErrorMsg('Failed to start upgrade. Please try again.')
      }
    },
    onError: (error) => {
      setErrorMsg(`An error occurred: ${error.message}`)
      setSuccessMsg('')
    }
  })

  useEffect(() => {
    if (!upgradeInProgress) return

    const interval = setInterval(async () => {
      try {
        const progress = await getUpgradeProgress()
        setProgressData(progress)

        if (progress.status === 'deploying' || progress.status === 'in_progress' || progress.status === 'verifying_stability') {
          setSuccessMsg('')
          setErrorMsg('')
        } else if (progress.status === 'completed') {
          setUpgradeInProgress(false)
          setSuccessMsg('Upgrade completed successfully')
          // Next upgrade needs its own cleanup.
          sessionStorage.removeItem(CLEANUP_DONE_KEY)
          clearInterval(interval)

          setTimeout(() => {
            sessionStorage.setItem('showUpgradeSuccess', 'true')
            sessionStorage.setItem('upgradedVersion', selectedVersion)
            onClose()
            window.location.href = '/dashboard/migrations'
          }, 3000)
        } else if (
          progress.status === 'failed' ||
          progress.status === 'rolled_back' ||
          progress.status === 'rollback_failed'
        ) {
          setUpgradeInProgress(false)
          setErrorMsg(progress.status === 'rolled_back' ? 'Upgrade failed: Rolled back to previous version' : 'Upgrade failed')
          clearInterval(interval)

          setTimeout(() => {
            window.location.reload()
          }, 2000)
        }
      } catch {
        setUpgradeInProgress(false)
        setErrorMsg('Failed to fetch upgrade progress.')
        clearInterval(interval)
      }
    }, 3000)

    return () => clearInterval(interval)
  }, [upgradeInProgress, onClose, selectedVersion])

  if (!show) return null

  const upgradeDisabledReason = !cleanupCompleted
    ? 'Run the mandatory cleanup before upgrading'
    : !selectedVersion
      ? 'Select a version to upgrade to'
      : ''

  const cardSx = {
    p: 2,
    mb: 2,
    background: theme.palette.background.paper,
    border: `1px solid ${theme.palette.divider}`,
    borderRadius: 1,
    color: theme.palette.text.primary
  }

  const alertSx = { mb: 2, justifyContent: 'center' }

  return (
    <React.Fragment>
      <Dialog open={show} onClose={busy ? undefined : onClose} maxWidth="sm" fullWidth>
        <DialogTitle>Upgrade vJailbreak</DialogTitle>
        <DialogContent sx={{ pb: 0 }}>
          {/* Step 1 — mandatory cleanup */}
          <Box sx={cardSx}>
            <StepHeader
              index={1}
              title="Clean up existing resources"
              status={
                cleanUpInProgress ? 'In progress' : cleanupCompleted ? 'Completed' : 'Required'
              }
              statusColor={cleanupCompleted && !cleanUpInProgress ? 'success' : 'warning'}
              done={cleanupCompleted}
            />

            <Typography variant="body2" mb={1} sx={{ color: theme.palette.text.secondary }}>
              The following will be cleaned up:
            </Typography>
            <Box
              component="ul"
              sx={{ margin: 0, paddingLeft: 0, listStyle: 'none', fontSize: '0.875rem' }}
            >
              {CLEANUP_ITEMS.map((item) => (
                <Box
                  component="li"
                  key={item}
                  sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 0.5 }}
                >
                  {cleanupCompleted ? (
                    <CheckCircleIcon color="success" sx={{ fontSize: 18 }} />
                  ) : (
                    <RadioButtonUncheckedIcon
                      sx={{ fontSize: 18, color: theme.palette.text.disabled }}
                    />
                  )}
                  <Typography variant="body2" sx={{ color: theme.palette.text.secondary }}>
                    {item}
                  </Typography>
                </Box>
              ))}
            </Box>

          </Box>

          {/* Step 2 — target version */}
          <Box sx={cardSx} data-testid="version-step-card">
            <StepHeader
              index={2}
              title="Select target version"
              status={selectedVersion ? selectedVersion : 'Pending'}
              statusColor={selectedVersion ? 'success' : 'default'}
              done={!!selectedVersion && cleanupCompleted}
            />
            <Select
              fullWidth
              value={selectedVersion}
              onChange={(e) => setSelectedVersion(e.target.value)}
              disabled={areVersionsLoading || upgradeMutation.isPending || busy}
              displayEmpty
              size="small"
            >
              <MenuItem value="">
                {areVersionsLoading ? 'Loading versions...' : 'Select a version...'}
              </MenuItem>
              {Array.isArray(updates?.updates) &&
                updates.updates.map((update) => (
                  <MenuItem key={update.version} value={update.version}>
                    {update.version}
                  </MenuItem>
                ))}
            </Select>
          </Box>
          {upgradeInProgress && (
            <Box
              display="flex"
              justifyContent="center"
              alignItems="center"
              gap={1.5}
              mb={2}
              data-testid="upgrade-progress"
            >
              <CircularProgress size={18} />
              <Typography variant="body2">{getUIStatusMessage(progressData?.status)}</Typography>
            </Box>
          )}

          {(busy || upgradeMutation.isPending || cleanupMutation.isPending) && (
            <Alert severity="warning" sx={alertSx}>
              Processing. Please do not close or refresh this page.
            </Alert>
          )}

          {errorMsg && (
            <Alert severity="error" sx={alertSx}>
              {errorMsg}
            </Alert>
          )}

          {successMsg && (
            <Alert severity="success" sx={alertSx}>
              {successMsg}
            </Alert>
          )}

          {upgradeMutation.isPending && !upgradeInProgress && (
            <Box display="flex" justifyContent="center" mb={2}>
              <CircularProgress size={24} />
            </Box>
          )}
        </DialogContent>
        <DialogActions sx={{ gap: 1, p: 2 }}>
          <ActionButton onClick={onClose} tone="secondary" fullWidth disabled={busy}>
            Cancel
          </ActionButton>
          <Tooltip
            title={
              <Typography sx={{ fontSize: '0.875rem' }}>
                {cleanupCompleted
                  ? 'Cleanup already ran. Running it again deletes any resources recreated since.'
                  : 'Deletes all resources listed above. Required before upgrading.'}
              </Typography>
            }
            arrow
          >
            <span style={{ width: '100%' }}>
              <ActionButton
                onClick={() => setConfirmCleanupOpen(true)}
                tone="primary"
                fullWidth
                loading={cleanUpInProgress}
                disabled={busy}
                data-testid="cleanup-button"
              >
                {cleanUpInProgress ? 'Cleaning up...' : cleanupCompleted ? 'Re-run Cleanup' : 'Cleanup'}
              </ActionButton>
            </span>
          </Tooltip>
          <Tooltip
            title={
              upgradeDisabledReason ? (
                <Typography sx={{ fontSize: '0.875rem' }}>{upgradeDisabledReason}</Typography>
              ) : (
                ''
              )
            }
            arrow
          >
            <span style={{ width: '100%' }}>
              <ActionButton
                onClick={() => {
                  setSuccessMsg('')
                  upgradeMutation.mutate()
                }}
                disabled={
                  !cleanupCompleted ||
                  !selectedVersion ||
                  busy ||
                  areVersionsLoading ||
                  upgradeMutation.isPending
                }
                tone="primary"
                fullWidth
                data-testid="upgrade-button"
              >
                Upgrade
              </ActionButton>
            </span>
          </Tooltip>
        </DialogActions>
      </Dialog>

      <ConfirmationDialog
        open={confirmCleanupOpen}
        onClose={() => setConfirmCleanupOpen(false)}
        title="Clean up before upgrade?"
        icon={<WarningAmberIcon color="warning" />}
        actionLabel="Yes, clean up"
        actionColor="primary"
        cancelLabel="Cancel"
        confirmButtonTestId="confirm-cleanup-button"
        // Close first, then kick off cleanup: progress belongs on the Cleanup button below,
        // not inside a confirmation dialog that would look like a frozen screen.
        onConfirm={async () => {
          setConfirmCleanupOpen(false)
          void runCleanup()
        }}
        message={
          <>
            This deletes all custom resources in this vJailbreak appliance and{' '}
            <strong>cannot be undone</strong>. Any migration that is still running will be lost.
          </>
        }
      />
    </React.Fragment>
  )
}
