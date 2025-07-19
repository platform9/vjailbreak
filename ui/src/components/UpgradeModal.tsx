import { useState, useEffect } from 'react';
import { useQuery, useMutation } from '@tanstack/react-query';
import {
  getAvailableTags,
  initiateUpgrade,
  getUpgradeProgress,
  confirmCleanupAndUpgrade,
  cleanupStepApiCall,
} from '../api/version';
import { useNavigate } from 'react-router-dom';
import {
  UpgradeResponse,
  ValidationResult,
  UpgradeProgressResponse,
} from '../api/version/model';
import Dialog from '@mui/material/Dialog';
import DialogTitle from '@mui/material/DialogTitle';
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
  const [crList, setCrList] = useState<string[]>([]);
  const [showCRWarning, setShowCRWarning] = useState(false);
  const theme = useTheme();
  const navigate = useNavigate();

  const stepKeys = [
    'no_migrationplans',
    'no_rollingmigrationplans',
    'vmware_creds_deleted',
    'openstack_creds_deleted',
    'agent_scaled_down',
    'no_custom_resources',
  ];
  const stepLabels = [
    'Delete MigrationPlans',
    'Delete RollingMigrationPlans',
    'Delete VMware credentials',
    'Delete OpenStack credentials',
    'Scale down Agents',
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
        setSuccessMsg('Upgrade process has been initiated!');
      } else if (data.cleanupRequired && Array.isArray(data.customResourceList) && data.customResourceList.length > 0) {
        setCrList(data.customResourceList);
        setShowCRWarning(true);
        setErrorMsg('');
        setSuccessMsg('');
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

        if (progress.status === 'completed' || progress.status === 'deploying') {
          setSuccessMsg('');
          setErrorMsg('');
        } else if (progress.status === 'deployments_ready') {
          setUpgradeInProgress(false);
          setSuccessMsg(progress.currentStep || 'Upgrade completed successfully');
          clearInterval(interval);
          setTimeout(() => {
            onClose();
            navigate('/dashboard/migrations', {
              state: {
                showUpgradeSuccess: true,
                version: selectedVersion
              }
            });
          }, 3000);
        } else if (progress.status === 'failed') {
          setUpgradeInProgress(false);
          setErrorMsg(`Upgrade failed: ${progress.error}`);
          clearInterval(interval);
        }
      } catch (err) {
        setUpgradeInProgress(false);
        setErrorMsg('Failed to fetch upgrade progress.');
        clearInterval(interval);
      }
    }, 3000);
    return () => clearInterval(interval);
  }, [upgradeInProgress, onClose, selectedVersion, navigate]);

  const handleConfirmCleanup = async () => {
    setShowCRWarning(false);
    setErrorMsg('');
    setSuccessMsg('');
    try {
      const data = await confirmCleanupAndUpgrade(selectedVersion, true);
      if (data.upgradeStarted) {
        setUpgradeInProgress(true);
        setCheckResults(null);
        setSuccessMsg('Upgrade process has been initiated!');
      } else {
        setCheckResults(data.checks);
        setErrorMsg('Pre-upgrade checks failed. Please resolve the issues below.');
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
            <Typography variant="subtitle1" color="warning.main" fontWeight={600} gutterBottom>
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
          disabled={!selectedVersion || upgradeInProgress || areVersionsLoading || upgradeMutation.isPending || !allChecksPassed}
          variant="contained"
          color="primary"
          fullWidth
        >
          {upgradeInProgress || upgradeMutation.isPending ? 'Upgrading...' : 'Upgrade'}
        </Button>
          <Button onClick={runStepwiseCleanup} variant="contained" color="primary" fullWidth disabled={upgradeInProgress}>
            Run Stepwise Cleanup
          </Button>
          <Button onClick={onClose} variant="outlined" fullWidth disabled={upgradeInProgress}>
            Cancel
          </Button>
        </DialogActions>
      </Dialog>
      <Dialog open={showCRWarning} onClose={handleCancelCleanup} maxWidth="sm" fullWidth>
        <DialogTitle sx={{ fontWeight: 600 }}>
            Custom Resources Detected
        </DialogTitle>
        <DialogContent>
            <Alert
                severity="warning"
                variant="outlined" 
                sx={{
                    borderColor: 'warning.main',
                    '& .MuiAlert-icon': {
                        color: 'warning.main',
                    },
                }}
            >
                <Typography fontWeight={600} gutterBottom>
                    The following resources must be deleted to proceed. This is a destructive operation and cannot be undone.
                </Typography>
                <Box component="ul" sx={{ my: 2, pl: 2.5 }}>
                    {crList.map(cr => (
                        <Typography component="li" key={cr} variant="body2">{cr}</Typography>
                    ))}
                </Box>
                <Typography variant="body2" fontWeight={500}>
                    Are you sure you want to delete these resources and continue?
                </Typography>
            </Alert>
        </DialogContent>
        <DialogActions sx={{ p: 2, gap: 1 }}> 
            <Button onClick={handleCancelCleanup} variant="outlined">
                Cancel
            </Button>
            <Button onClick={handleConfirmCleanup} color="error" variant="contained">
                OK, Delete and Continue
            </Button>
        </DialogActions>
    </Dialog>
    </React.Fragment>
  );
};