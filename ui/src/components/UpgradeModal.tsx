import React, { useState, useEffect } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { getAvailableTags, initiateUpgrade, getUpgradeProgress, confirmCleanupAndUpgrade, cleanupStepApiCall } from '../api/version';
import { ValidationResult, UpgradeProgressResponse } from '../api/version/model';
import {
    Dialog, DialogTitle, DialogContent, DialogActions, Box, Typography, Button, Select, MenuItem,
    Alert, CircularProgress, useTheme
} from '@mui/material';
import CheckCircleIcon from '@mui/icons-material/CheckCircle';
import CancelIcon from '@mui/icons-material/Cancel';
import RadioButtonUncheckedIcon from '@mui/icons-material/RadioButtonUnchecked';

export const UpgradeModal = ({ show, onClose }) => {
  const [selectedVersion, setSelectedVersion] = useState('');
  const [checkResults, setCheckResults] = useState<ValidationResult | null>(null);
  const [errorMsg, setErrorMsg] = useState('');
  const [successMsg, setSuccessMsg] = useState('');
  const [upgradeInProgress, setUpgradeInProgress] = useState(false);
  const [progressData, setProgressData] = useState<UpgradeProgressResponse | null>(null);
  const [crList, setCrList] = useState<string[]>([]);
  const [showCRWarning, setShowCRWarning] = useState(false);
  const theme = useTheme();
  const navigate = useNavigate();

  const stepLabels = [
    'Delete MigrationPlans',
    'Delete RollingMigrationPlans',
    'Scale down Agents',
    'Delete VMware credentials',
    'Delete OpenStack credentials',
    'Delete Custom Resources',
  ];
  const stepKeys = [
    'no_migrationplans',
    'no_rollingmigrationplans',
    'agent_scaled_down',
    'vmware_creds_deleted',
    'openstack_creds_deleted',
    'no_custom_resources',
  ];
  const [stepStates, setStepStates] = useState(stepLabels.map(label => ({ label, state: 'pending' })));

  // Fetch available updates
  const { data: updates, isLoading: areVersionsLoading } = useQuery({
    queryKey: ['availableTags'],
    queryFn: getAvailableTags,
    enabled: show,
  });

  useEffect(() => {
    let interval: NodeJS.Timeout;
    if (upgradeInProgress) {
      const pollProgress = async () => {
        try {
          const progress = await getUpgradeProgress();
          setProgressData(progress);

          if (progress.status === 'completed') {
            clearInterval(interval);
            setSuccessMsg('Upgrade completed successfully!');
            setTimeout(() => {
              onClose();
              navigate('/dashboard/migrations');
              window.location.reload();
            }, 2000);
          } else if (['failed', 'rolled_back', 'rollback_failed'].includes(progress.status)) {
            clearInterval(interval);
            setUpgradeInProgress(false);
            setErrorMsg(`Upgrade failed: ${progress.error || 'An unknown error occurred.'}`);
          }
        } catch (error) {
          clearInterval(interval);
          setUpgradeInProgress(false);
          setErrorMsg('Failed to get upgrade progress.');
          console.error('Failed to fetch upgrade progress:', error);
        }
      };
      interval = setInterval(pollProgress, 5000);
    }
    return () => {
      if (interval) clearInterval(interval);
    };
  }, [upgradeInProgress, onClose, navigate]);

  const handleUpgradeClick = async () => {
    setErrorMsg('');
    setSuccessMsg('');

    try {
      const data = await initiateUpgrade(selectedVersion, false);
      if (data.upgradeStarted) {
        setUpgradeInProgress(true);
        setCheckResults(null);
      } else if (data.cleanupRequired && data.customResourceList) {
        setCrList(data.customResourceList || []); 
        setShowCRWarning(true);
      } else {
        setCheckResults(data.checks);
        setErrorMsg('Pre-upgrade checks failed. Please resolve the issues below.');
      }
    } catch (error: any) {
      setErrorMsg(`An error occurred: ${error.message}`);
      setUpgradeInProgress(false);
    }
  };

  const handleConfirmCleanup = async () => {
    setShowCRWarning(false);
    setErrorMsg('');
    setSuccessMsg('');
    try {
        const data = await confirmCleanupAndUpgrade(selectedVersion, true);
        if (data.upgradeStarted) {
            setUpgradeInProgress(true);
            setCheckResults(null);
        } else {
            setCheckResults(data.checks);
            setErrorMsg('Checks failed even after cleanup.');
        }
    } catch (error: any) {
        setErrorMsg(`An error occurred: ${error.message}`);
    }
  };

  const handleCancelCleanup = () => {
    setShowCRWarning(false);
    setCrList([]);
    onClose();
  };

  const runStepwiseCleanup = async () => {
    let newStates = stepLabels.map(label => ({ label, state: 'pending' }));
    setStepStates(newStates);

    for (let i = 0; i < stepKeys.length; i++) {
      newStates[i].state = 'in_progress';
      setStepStates([...newStates]);

      try {
        const res = await cleanupStepApiCall(stepKeys[i]);
        newStates[i].state = res.success ? 'success' : 'error';
      } catch (e) {
        newStates[i].state = 'error';
      }
      setStepStates([...newStates]);
      if (newStates[i].state === 'error') break;
    }
  };

  const allChecksPassed = stepStates.every(step => step.state === 'success');

  const checkList = checkResults ? [
    { label: 'No MigrationPlans', value: checkResults.noMigrationPlans },
    { label: 'No RollingMigrationPlans', value: checkResults.noRollingMigrationPlans },
    { label: 'Agent scaled down', value: checkResults.agentsScaledDown },
    { label: 'VMware credentials deleted', value: checkResults.vmwareCredsDeleted },
    { label: 'OpenStack credentials deleted', value: checkResults.openstackCredsDeleted },
    { label: 'No Custom Resources (CRs) deleted', value: checkResults.noCustomResources },
  ] : [];


  if (!show) return null;

  return (
    <React.Fragment>
      <Dialog open={show} onClose={upgradeInProgress ? undefined : onClose} maxWidth="xs" fullWidth>
        <DialogTitle>Upgrade vJailbreak</DialogTitle>
        <DialogContent>
          <Box mb={2}>
            <Select
              fullWidth
              value={selectedVersion}
              onChange={e => setSelectedVersion(e.target.value)}
              disabled={areVersionsLoading || upgradeInProgress}
              displayEmpty
              size="small"
            >
              <MenuItem value="">
                {areVersionsLoading ? 'Loading versions...' : 'Select a version...'}
              </MenuItem>
              {Array.isArray(updates?.updates) && updates.updates.map(update => (
                <MenuItem key={update.version} value={update.version}>
                  {update.version}
                </MenuItem>
              ))}
            </Select>
          </Box>
          <Box mb={2} p={2} sx={{
            background: theme.palette.background.paper,
            border: `1px solid ${theme.palette.divider}`,
            borderRadius: 1,
          }}>
            <Typography variant="subtitle1" color="warning.main" fontWeight={600} gutterBottom>
              Pre-Upgrade Checklist
            </Typography>
            <Typography variant="body2" mb={1} sx={{ color: theme.palette.text.secondary }}>
              The following needs to be cleaned up before upgrading:
            </Typography>
            <ul style={{ margin: 0, paddingLeft: 20, fontSize: '1rem' }}>
              {stepStates.map((item) => (
                <li key={item.label} style={{ display: 'flex', alignItems: 'center', marginBottom: 2 }}>
                  {item.state === 'in_progress' && <CircularProgress size={16} sx={{ mr: 1 }} />}
                  {item.state === 'success' && <CheckCircleIcon color="success" sx={{ mr: 1 }} />}
                  {item.state === 'error' && <CancelIcon color="error" sx={{ mr: 1 }} />}
                  {item.state === 'pending' && <RadioButtonUncheckedIcon color="disabled" sx={{ mr: 1 }} />}
                  {item.label}
                </li>
              ))}
            </ul>
          </Box>

          {upgradeInProgress && !successMsg && (
            <Box display="flex" alignItems="center" justifyContent="center" my={2} p={2}>
              <CircularProgress size={24} sx={{ mr: 2 }} />
              <Typography variant="body1">
                Upgrading... {progressData?.currentStep && `(${progressData.currentStep})`}
              </Typography>
            </Box>
          )}

          {successMsg && (
            <Box display="flex" alignItems="center" justifyContent="center" my={2} p={2}>
              <CheckCircleIcon color="success" sx={{ mr: 1, fontSize: 30 }} />
              <Typography variant="h6" color="success.main">
                {successMsg}
              </Typography>
            </Box>
          )}

          {errorMsg && <Alert severity="error" sx={{ mb: 2 }}>{errorMsg}</Alert>}
          
          {checkResults && (
            <Box mb={2} p={2} sx={{ background: theme.palette.background.default, borderRadius: 1 }}>
              <Typography variant="subtitle2" color="primary" fontWeight={600} gutterBottom>
                Pre-flight Check Results
              </Typography>
              <ul style={{ listStyle: 'none', padding: 0, margin: 0 }}>
                {checkList.map((item) => (
                  <li key={item.label} style={{ display: 'flex', alignItems: 'center', color: item.value ? theme.palette.success.main : theme.palette.error.main, marginBottom: 2 }}>
                    {item.value ? <CheckCircleIcon fontSize="small" color="success" sx={{ mr: 1 }} /> : <CancelIcon fontSize="small" color="error" sx={{ mr: 1 }} />} {item.label}
                  </li>
                ))}
              </ul>
            </Box>
          )}
        </DialogContent>
        <DialogActions sx={{ p: '16px 24px', display: 'flex', gap: 1 }}>
          <Button
            onClick={handleUpgradeClick}
            disabled={upgradeInProgress || areVersionsLoading || !allChecksPassed || !selectedVersion}
            variant="contained"
            color="primary"
            sx={{ flex: 1 }}
          >
            {upgradeInProgress ? 'Upgrading...' : 'Upgrade Now'}
          </Button>
          <Button
            onClick={runStepwiseCleanup}
            variant="contained"
            color="secondary"
            disabled={upgradeInProgress}
            sx={{ flex: 1 }}
          >
            Run Stepwise Cleanup
          </Button>
          <Button
            onClick={onClose}
            variant="outlined"
            disabled={upgradeInProgress}
            sx={{ flex: 1 }}
          >
            Cancel
          </Button>
        </DialogActions>
      </Dialog>
      
      <Dialog open={showCRWarning} onClose={handleCancelCleanup} maxWidth="sm" fullWidth>
        <DialogTitle>Custom Resources Detected</DialogTitle>
        <DialogContent>
          <Alert severity="warning" sx={{ mb: 2 }}>
            <Typography variant="subtitle1" fontWeight={600} gutterBottom>
              The following Custom Resources must be deleted to proceed. This is a destructive operation.
            </Typography>
            <ul style={{ margin: 0, paddingLeft: 20 }}>
              {crList.map(cr => (
                <li key={cr}>{cr}</li>
              ))}
            </ul>
            <Typography variant="body2" color="error" mt={2}>
              Are you sure you want to delete all these CRs and continue?
            </Typography>
          </Alert>
        </DialogContent>
        <DialogActions>
          <Button onClick={handleConfirmCleanup} color="error" variant="contained">OK, Delete and Continue</Button>
          <Button onClick={handleCancelCleanup} variant="outlined">Cancel</Button>
        </DialogActions>
      </Dialog>
    </React.Fragment>
  );
};