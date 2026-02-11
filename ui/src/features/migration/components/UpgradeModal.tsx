import { useState, useEffect } from 'react'
import { useQuery, useMutation } from '@tanstack/react-query'

import { UpgradeResponse, UpgradeProgressResponse } from 'src/api/version/model'
import Dialog from '@mui/material/Dialog'
import DialogTitle from '@mui/material/DialogTitle'
import DialogContent from '@mui/material/DialogContent'
import DialogActions from '@mui/material/DialogActions'
import Box from '@mui/material/Box'
import Typography from '@mui/material/Typography'
import Select from '@mui/material/Select'
import MenuItem from '@mui/material/MenuItem'
import Alert from '@mui/material/Alert'
import CircularProgress from '@mui/material/CircularProgress'
import { useTheme } from '@mui/material/styles'
import React from 'react'
import { ActionButton } from 'src/components'
import {
  getAvailableTags,
  getUpgradeProgress,
  initiateUpgrade
} from 'src/api/version'

// Map backend status to clean UI messages
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

export const UpgradeModal = ({ show, onClose }) => {
  const [selectedVersion, setSelectedVersion] = useState('')
  const [errorMsg, setErrorMsg] = useState('')
  const [successMsg, setSuccessMsg] = useState('')
  const [upgradeInProgress, setUpgradeInProgress] = useState(false)
  const [progressData, setProgressData] = useState<UpgradeProgressResponse | null>(null)
  const theme = useTheme()

  const { data: updates, isLoading: areVersionsLoading } = useQuery({
    queryKey: ['availableTags'],
    queryFn: getAvailableTags,
    enabled: show
  })

  // Upgrade with autoCleanup=true - job handles all cleanup, validation, backup internally
  const upgradeMutation = useMutation<UpgradeResponse, Error, void>({
    mutationFn: () => initiateUpgrade(selectedVersion, true),
    onSuccess: (data) => {
      if (data.upgradeStarted) {
        setUpgradeInProgress(true)
        setErrorMsg('')
      } else {
        // Job will handle cleanup automatically, but show error if immediate failure
        setErrorMsg('Failed to start upgrade. Please try again.')
      }
    },
    onError: (error) => {
      setErrorMsg(`An error occurred: ${error.message}`)
      setSuccessMsg('')
    }
  })

  // Poll upgrade progress
  useEffect(() => {
    if (!upgradeInProgress) return

    const interval = setInterval(async () => {
      try {
        const progress = await getUpgradeProgress()
        setProgressData(progress)

        // Clear messages during active upgrade
        if (progress.status === 'deploying' || progress.status === 'in_progress' || progress.status === 'verifying_stability') {
          setSuccessMsg('')
          setErrorMsg('')
        } else if (progress.status === 'completed') {
          // Upgrade completed successfully
          setUpgradeInProgress(false)
          setSuccessMsg('Upgrade completed successfully')
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
      } catch (err) {
        setUpgradeInProgress(false)
        setErrorMsg('Failed to fetch upgrade progress.')
        clearInterval(interval)
      }
    }, 3000)

    return () => clearInterval(interval)
  }, [upgradeInProgress, onClose, selectedVersion])

  if (!show) return null

  return (
    <React.Fragment>
      <Dialog open={show} onClose={upgradeInProgress ? undefined : onClose} maxWidth="xs" fullWidth>
        <DialogTitle>Upgrade vJailbreak</DialogTitle>
        <DialogContent>
          <Box mb={2}>
            <Select
              fullWidth
              value={selectedVersion}
              onChange={(e) => setSelectedVersion(e.target.value)}
              disabled={areVersionsLoading || upgradeMutation.isPending || upgradeInProgress}
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

          {/* Info box about what upgrade does */}
          <Box
            mb={2}
            p={2}
            sx={{
              background: theme.palette.background.paper,
              border: `1px solid ${theme.palette.divider}`,
              borderRadius: 1,
              color: theme.palette.text.primary
            }}
          >
            <Typography variant="subtitle1" color="primary.main" fontWeight={600} gutterBottom>
              Upgrade Process
            </Typography>
            <Typography variant="body2" sx={{ color: theme.palette.text.secondary }}>
              The upgrade will automatically:
            </Typography>
            <ul style={{ margin: '8px 0 0 0', paddingLeft: 20, color: theme.palette.text.secondary, fontSize: '0.875rem' }}>
              <li>Clean up existing resources</li>
              <li>Backup current state</li>
              <li>Apply new version</li>
              <li>Verify services are ready</li>
            </ul>
          </Box>

          {/* Upgrade progress */}
          {upgradeInProgress && (
            <Box display="flex" flexDirection="column" alignItems="center" mb={2}>
              <CircularProgress size={32} />
              <Typography variant="body2" mt={2}>
                {getUIStatusMessage(progressData?.status)}
              </Typography>
            </Box>
          )}

          {upgradeInProgress && (
            <Alert severity="warning" sx={{ mt: 2 }}>
              Processing. Please do not close or refresh this page.
            </Alert>
          )}

          {errorMsg && (
            <Box display="flex" justifyContent="center" mb={2}>
              <Alert severity="error" sx={{ mb: 2, display: 'flex', alignItems: 'center' }}>
                {errorMsg}
              </Alert>
            </Box>
          )}

          {successMsg && (
            <Box display="flex" justifyContent="center" alignItems="center" mb={2}>
              <Alert
                severity="success"
                sx={{
                  mb: 2,
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  textAlign: 'center',
                  width: '100%'
                }}
              >
                {successMsg}
              </Alert>
            </Box>
          )}

          {upgradeMutation.isPending && !upgradeInProgress && (
            <Box display="flex" justifyContent="center" mb={2}>
              <CircularProgress size={24} />
            </Box>
          )}
        </DialogContent>
        <DialogActions sx={{ gap: 1, p: 2 }}>
          <ActionButton
            onClick={() => upgradeMutation.mutate()}
            disabled={
              !selectedVersion ||
              upgradeInProgress ||
              areVersionsLoading ||
              upgradeMutation.isPending
            }
            tone="primary"
            fullWidth
          >
            Upgrade
          </ActionButton>
          <ActionButton
            onClick={onClose}
            tone="secondary"
            fullWidth
            disabled={upgradeInProgress}
          >
            Cancel
          </ActionButton>
        </DialogActions>
      </Dialog>
    </React.Fragment>
  )
}
