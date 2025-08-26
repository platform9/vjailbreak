import { useState, useEffect } from 'react';
import { useQuery, useMutation } from '@tanstack/react-query';
import {
  getAvailableTags,
  initiateUpgrade,
  getUpgradeProgress,
  cleanupStepApiCall,
} from '../api/version';
import {
  UpgradeResponse,
  ValidationResult,
  UpgradeProgressResponse,
} from '../api/version/model';
import Dialog from '@mui/material/Dialog';
import DialogTitle from '@mui/material/DialogTitle';
import Tooltip from '@mui/material/Tooltip';
import DialogContent from '@mui/material/DialogContent';
import DialogActions from '@mui/material/DialogActions';
import Box from '@mui/material/Box';
import Typography from '@mui/material/Typography';
import Button from '@mui/material/Button';
import Select from '@mui/material/Select';
import MenuItem from '@mui/material/MenuItem';
import Alert from '@mui/material/Alert';
import CircularProgress from '@mui/material/CircularProgress';
import CheckCircleIcon from '@mui/icons-material/CheckCircle';
import CancelIcon from '@mui/icons-material/Cancel';
import RadioButtonUncheckedIcon from '@mui/icons-material/RadioButtonUnchecked';
import { useTheme } from '@mui/material/styles';
import React from 'react';

export const UpgradeModal = ({ show, onClose }) => {
  const [selectedVersion, setSelectedVersion] = useState('');
  const [checkResults, setCheckResults] = useState<ValidationResult | null>(null);
  const [errorMsg, setErrorMsg] = useState('');
  const [successMsg, setSuccessMsg] = useState('');
  const [upgradeInProgress, setUpgradeInProgress] = useState(false);
  const [progressData, setProgressData] = useState<UpgradeProgressResponse | null>(null);
  const [cleanUpInProgress, setCleanUpInProgress] = useState(false);
  const theme = useTheme();

  const stepKeys = [
    'no_migrationplans',
    'no_rollingmigrationplans',
    'agent_scaled_down',
    'vmware_creds_deleted',
    'openstack_creds_deleted',
    'no_custom_resources',
  ];
  const stepLabels = [
    'Delete MigrationPlans',
    'Delete RollingMigrationPlans',
    'Scale down Agents',
    'Delete VMware credentials',
    'Delete OpenStack credentials',
    'Delete Custom Resources',
  ];
  const [stepStates, setStepStates] = useState(stepLabels.map(label => ({ label, state: 'pending' })));

  const { data: updates, isLoading: areVersionsLoading } = useQuery({
    queryKey: ['availableTags'],
    queryFn: getAvailableTags,
    enabled: show,
  });

  const upgradeMutation = useMutation<UpgradeResponse, Error, void>({
    mutationFn: () => initiateUpgrade(selectedVersion, false),
    onSuccess: (data) => {
      if (data.upgradeStarted) {
        setUpgradeInProgress(true);
        setErrorMsg('');
        setCheckResults(null);
      } else {
        setCheckResults(data.checks);
        setErrorMsg('Pre-upgrade checks failed. Please resolve the issues below.');
        setSuccessMsg('');
      }
    },
    onError: (error) => {
      setErrorMsg(`An error occurred: ${error.message}`);
      setSuccessMsg('');
    },
  });

  useEffect(() => {
    if (!upgradeInProgress) return;
    
    const interval = setInterval(async () => {
      try {
        const progress = await getUpgradeProgress();
        setProgressData(progress);

        if (progress.status === 'deploying') {
          setSuccessMsg('');
          setErrorMsg('');
        } else if (progress.status === 'server_restarting') {
          setUpgradeInProgress(false); 
          setSuccessMsg('Upgrade completed successfully');
          clearInterval(interval);
          
          setTimeout(() => {
            sessionStorage.setItem('showUpgradeSuccess', 'true');
            sessionStorage.setItem('upgradedVersion', selectedVersion);
            onClose();
            window.location.href = '/dashboard/migrations';
          }, 5000);
        } else if (progress.status === 'failed' || progress.status === 'rolled_back' || progress.status === 'rollback_failed') {
          setUpgradeInProgress(false);
          setErrorMsg('Upgrade failed: Rolling back'); 
          clearInterval(interval);

          setTimeout(() => {
            window.location.reload();
          }, 3000);
        }
      } catch (err) {
        setUpgradeInProgress(false);
        setErrorMsg('Failed to fetch upgrade progress.');
        clearInterval(interval);
      }
    }, 3000);

    return () => clearInterval(interval);
  }, [upgradeInProgress, onClose, selectedVersion]);

  const runStepwiseCleanup = async () => {
    setCleanUpInProgress(true);
    setErrorMsg('');
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
    setCleanUpInProgress(false);
  };

  const allChecksPassed = checkResults
    ? Object.values(checkResults).every(Boolean)
    : stepStates.every(step => step.state === 'success');

  if (!show) return null;

  const checkList = checkResults ? [
    { label: 'No MigrationPlans', value: checkResults.noMigrationPlans },
    { label: 'No RollingMigrationPlans', value: checkResults.noRollingMigrationPlans },
    { label: 'VMware credentials deleted', value: checkResults.vmwareCredsDeleted },
    { label: 'OpenStack credentials deleted', value: checkResults.openstackCredsDeleted },
    { label: 'Agent scaled down', value: checkResults.agentsScaledDown },
    { label: 'No Custom Resources (CRs) deleted', value: checkResults.noCustomResources },
  ] : [];

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
              disabled={areVersionsLoading || upgradeMutation.isPending}
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
            color: theme.palette.text.primary,
          }}>
            <Typography variant="subtitle1" color="primary.main" fontWeight={600} gutterBottom>
              Pre-Upgrade Checklist
            </Typography>
            <Typography variant="body2" mb={1} sx={{ color: theme.palette.text.secondary }}>
              The following needs to be cleaned up before upgrading:
            </Typography>
            <ul style={{ margin: 0, paddingLeft: 20, color: theme.palette.text.primary, fontWeight: 500, fontSize: '1rem' }}>
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
          {upgradeInProgress && (
            <Box display="flex" flexDirection="column" alignItems="center" mb={2}>
              <CircularProgress size={32} />
              <Typography variant="body2" mt={2}>
              {progressData?.currentStep.startsWith('Waiting') 
                ? 'Waiting for deployments to be ready' 
                : progressData?.currentStep || 'Upgrading'}
              </Typography>
            </Box>
          )}

{(upgradeInProgress || cleanUpInProgress) && (
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
              <Alert severity="success" sx={{ 
                mb: 2, 
                display: 'flex', 
                alignItems: 'center',
                justifyContent: 'center',
                textAlign: 'center',
                width: '100%'
              }}>    
                {successMsg}
              </Alert>
            </Box>
          )}

          {upgradeMutation.isPending && !upgradeInProgress && (
            <Box display="flex" justifyContent="center" mb={2}>
              <CircularProgress size={24} />
            </Box>
          )}
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
        <DialogActions>
        <Button
          onClick={() => upgradeMutation.mutate()}
          disabled={!selectedVersion || upgradeInProgress || cleanUpInProgress || areVersionsLoading || upgradeMutation.isPending || !allChecksPassed}
          variant="contained"
          color="primary"
          fullWidth
        >
          Upgrade
        </Button>
        <Tooltip
          title={
            <Typography sx={{ fontSize: '0.875rem', whiteSpace: 'nowrap' }}>
              This will clean up the items listed above
            </Typography>
          }
          arrow
        >
          <span>
            <Button onClick={runStepwiseCleanup} variant="contained" color="primary" fullWidth disabled={upgradeInProgress || cleanUpInProgress}>
              Cleanup
            </Button>
          </span>
        </Tooltip>
          <Button onClick={onClose} variant="outlined" fullWidth disabled={upgradeInProgress || cleanUpInProgress}>
            Cancel
          </Button>
        </DialogActions>
      </Dialog>
    </React.Fragment>
  );
};