import { useState } from 'react'
import {
  Box,
  Button,
  Checkbox,
  Chip,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  FormControlLabel,
  FormGroup,
  FormLabel,
  InputLabel,
  MenuItem,
  RadioGroup,
  Radio,
  Select,
  SelectChangeEvent,
  Step,
  StepLabel,
  Stepper,
  TextField,
  Typography,
  Alert,
  Divider,
  FormHelperText
} from '@mui/material'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useVmwareCredentialsQuery } from 'src/hooks/api/useVmwareCredentialsQuery'
import { useOpenstackCredentialsQuery } from 'src/hooks/api/useOpenstackCredentialsQuery'
import { useBMConfigsQuery } from 'src/hooks/api/useBMConfigQuery'
import { CLUSTER_CONVERSION_BATCHES_QUERY_KEY } from 'src/hooks/api/useClusterConversionBatchesQuery'
import { postClusterConversionBatch } from 'src/api/cluster-conversion-batches/clusterConversionBatches'
import { AutoStartMode } from 'src/api/cluster-conversion-batches/model'
import { getVMwareClusters } from 'src/api/vmware-clusters/vmwareClusters'
import { getVMwareHosts } from 'src/api/vmware-hosts/vmwareHosts'
import { VJAILBREAK_DEFAULT_NAMESPACE } from 'src/api/constants'

interface CreateBatchDialogProps {
  open: boolean
  onClose: () => void
}

const STEPS = ['Infrastructure', 'Select Cluster & Hosts', 'Review']

interface StepOneState {
  vmwareCredsName: string
  openstackCredsName: string
  bmConfigName: string
  autoStartMode: AutoStartMode
  maxRetries: number
}

const INITIAL_STEP_ONE: StepOneState = {
  vmwareCredsName: '',
  openstackCredsName: '',
  bmConfigName: '',
  autoStartMode: 'Auto',
  maxRetries: 3
}

export default function CreateBatchDialog({ open, onClose }: CreateBatchDialogProps) {
  const queryClient = useQueryClient()

  const [activeStep, setActiveStep] = useState(0)
  const [stepOne, setStepOne] = useState<StepOneState>(INITIAL_STEP_ONE)
  const [clusterName, setClusterName] = useState('')
  const [selectedHosts, setSelectedHosts] = useState<string[]>([])
  const [submitError, setSubmitError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  // ── Data queries ──────────────────────────────────────────────────────────
  const { data: vmwareCreds = [], isLoading: loadingVmwareCreds } = useVmwareCredentialsQuery()
  const { data: allOpenstackCreds = [], isLoading: loadingOpenstackCreds } =
    useOpenstackCredentialsQuery()
  const { data: allBmConfigs = [], isLoading: loadingBmConfigs } = useBMConfigsQuery()

  const openstackCreds = allOpenstackCreds.filter(
    (cred) => cred.metadata?.labels?.['vjailbreak.k8s.pf9.io/is-pcd'] === 'true'
  )
  const bmConfigs = allBmConfigs.filter(
    (bmc) => bmc.status?.validationStatus === 'Succeeded'
  )

  const { data: clustersData, isLoading: loadingClusters } = useQuery({
    queryKey: ['vmware-clusters', stepOne.vmwareCredsName],
    queryFn: () => getVMwareClusters(undefined, stepOne.vmwareCredsName),
    enabled: !!stepOne.vmwareCredsName
  })
  const clusters = clustersData?.items ?? []

  const { data: hostsData, isLoading: loadingHosts } = useQuery({
    queryKey: ['vmware-hosts', stepOne.vmwareCredsName, clusterName],
    queryFn: () => getVMwareHosts(undefined, stepOne.vmwareCredsName, clusterName),
    enabled: !!stepOne.vmwareCredsName && !!clusterName
  })
  const hosts = hostsData?.items ?? []

  // ── Helpers ───────────────────────────────────────────────────────────────
  const resetDialog = () => {
    setActiveStep(0)
    setStepOne(INITIAL_STEP_ONE)
    setClusterName('')
    setSelectedHosts([])
    setSubmitError(null)
    setSubmitting(false)
  }

  const handleClose = () => {
    resetDialog()
    onClose()
  }

  // ── Step 1 handlers ───────────────────────────────────────────────────────
  const handleVmwareCredsChange = (e: SelectChangeEvent<string>) => {
    setStepOne((prev) => ({ ...prev, vmwareCredsName: e.target.value }))
    setClusterName('')
    setSelectedHosts([])
  }

  const handleOpenstackCredsChange = (e: SelectChangeEvent<string>) => {
    setStepOne((prev) => ({ ...prev, openstackCredsName: e.target.value }))
  }

  const handleBmConfigChange = (e: SelectChangeEvent<string>) => {
    setStepOne((prev) => ({ ...prev, bmConfigName: e.target.value }))
  }

  const handleAutoStartChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setStepOne((prev) => ({ ...prev, autoStartMode: e.target.value as AutoStartMode }))
  }

  const handleMaxRetriesChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const val = Math.min(10, Math.max(0, parseInt(e.target.value, 10) || 0))
    setStepOne((prev) => ({ ...prev, maxRetries: val }))
  }

  // ── Step 2 handlers ───────────────────────────────────────────────────────
  const handleClusterChange = (e: SelectChangeEvent<string>) => {
    setClusterName(e.target.value)
    setSelectedHosts([])
  }

  const handleHostToggle = (hostName: string) => {
    setSelectedHosts((prev) =>
      prev.includes(hostName) ? prev.filter((h) => h !== hostName) : [...prev, hostName]
    )
  }

  const allHostNames = hosts.map((h) => h.spec.name)
  const allSelected = allHostNames.length > 0 && allHostNames.every((n) => selectedHosts.includes(n))
  const someSelected = selectedHosts.length > 0 && !allSelected

  const handleSelectAll = () => {
    if (allSelected) {
      setSelectedHosts([])
    } else {
      setSelectedHosts(allHostNames)
    }
  }

  // ── Navigation ────────────────────────────────────────────────────────────
  const isStepOneValid =
    !!stepOne.vmwareCredsName && !!stepOne.openstackCredsName && !!stepOne.bmConfigName

  const isStepTwoValid = !!clusterName && selectedHosts.length > 0

  const handleNext = () => setActiveStep((prev) => prev + 1)
  const handleBack = () => setActiveStep((prev) => prev - 1)

  // ── Submit ────────────────────────────────────────────────────────────────
  const handleSubmit = async () => {
    setSubmitError(null)
    setSubmitting(true)
    try {
      await postClusterConversionBatch({
        apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1',
        kind: 'ClusterConversionBatch',
        metadata: {
          generateName: `batch-${clusterName}-`,
          namespace: VJAILBREAK_DEFAULT_NAMESPACE
        },
        spec: {
          vmwareClusterName: clusterName,
          vmwareCredsRef: { name: stepOne.vmwareCredsName },
          openstackCredsRef: { name: stepOne.openstackCredsName },
          bmConfigRef: { name: stepOne.bmConfigName },
          hosts: selectedHosts.map((h) => ({ esxiName: h })),
          autoStart: stepOne.autoStartMode,
          maxRetries: stepOne.maxRetries
        }
      })
      await queryClient.invalidateQueries({ queryKey: CLUSTER_CONVERSION_BATCHES_QUERY_KEY })
      handleClose()
    } catch (err) {
      setSubmitError(err instanceof Error ? err.message : 'Failed to create batch')
    } finally {
      setSubmitting(false)
    }
  }

  // ── Renderers ─────────────────────────────────────────────────────────────
  const renderStepOne = () => (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 3, mt: 1 }}>
      <FormControl fullWidth size="small" required>
        <InputLabel>VMware Credentials</InputLabel>
        <Select
          label="VMware Credentials"
          value={stepOne.vmwareCredsName}
          onChange={handleVmwareCredsChange}
          disabled={loadingVmwareCreds}
          displayEmpty
        >
          {vmwareCreds.map((cred) => (
            <MenuItem key={cred.metadata.name} value={cred.metadata.name}>
              {cred.metadata.name}
            </MenuItem>
          ))}
        </Select>
        {loadingVmwareCreds && <FormHelperText>Loading...</FormHelperText>}
      </FormControl>

      <FormControl fullWidth size="small" required>
        <InputLabel>PCD / OpenStack Credentials</InputLabel>
        <Select
          label="PCD / OpenStack Credentials"
          value={stepOne.openstackCredsName}
          onChange={handleOpenstackCredsChange}
          disabled={loadingOpenstackCreds}
          displayEmpty
        >
          {openstackCreds.map((cred) => (
            <MenuItem key={cred.metadata.name} value={cred.metadata.name}>
              {cred.metadata.name}
            </MenuItem>
          ))}
        </Select>
        {loadingOpenstackCreds && <FormHelperText>Loading...</FormHelperText>}
        {!loadingOpenstackCreds && openstackCreds.length === 0 && (
          <FormHelperText error>No PCD credentials found</FormHelperText>
        )}
      </FormControl>

      <FormControl fullWidth size="small" required>
        <InputLabel>BMConfig</InputLabel>
        <Select
          label="BMConfig"
          value={stepOne.bmConfigName}
          onChange={handleBmConfigChange}
          disabled={loadingBmConfigs}
          displayEmpty
        >
          {bmConfigs.map((bmc) => (
            <MenuItem key={bmc.metadata.name} value={bmc.metadata.name}>
              {bmc.metadata.name}
            </MenuItem>
          ))}
        </Select>
        {loadingBmConfigs && <FormHelperText>Loading...</FormHelperText>}
        {!loadingBmConfigs && bmConfigs.length === 0 && (
          <FormHelperText error>No validated BMConfigs found</FormHelperText>
        )}
      </FormControl>

      <FormControl component="fieldset">
        <FormLabel component="legend">Auto Start Mode</FormLabel>
        <RadioGroup row value={stepOne.autoStartMode} onChange={handleAutoStartChange}>
          <FormControlLabel value="Auto" control={<Radio />} label="Auto" />
          <FormControlLabel value="Manual" control={<Radio />} label="Manual" />
        </RadioGroup>
      </FormControl>

      <TextField
        label="Max Retries"
        type="number"
        size="small"
        value={stepOne.maxRetries}
        onChange={handleMaxRetriesChange}
        inputProps={{ min: 0, max: 10 }}
        helperText="Number of retries per host (0–10)"
        sx={{ width: 200 }}
      />
    </Box>
  )

  const renderStepTwo = () => (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 3, mt: 1 }}>
      <FormControl fullWidth size="small" required>
        <InputLabel>VMware Cluster</InputLabel>
        <Select
          label="VMware Cluster"
          value={clusterName}
          onChange={handleClusterChange}
          disabled={loadingClusters || !stepOne.vmwareCredsName}
          displayEmpty
        >
          {clusters.map((cluster) => {
            const name = cluster.spec?.name || cluster.metadata.name
            return (
              <MenuItem key={cluster.metadata.name} value={name}>
                {name}
              </MenuItem>
            )
          })}
        </Select>
        {loadingClusters && <FormHelperText>Loading clusters...</FormHelperText>}
        {!loadingClusters && stepOne.vmwareCredsName && clusters.length === 0 && (
          <FormHelperText error>No clusters found for this credential</FormHelperText>
        )}
      </FormControl>

      {clusterName && (
        <Box>
          <Typography variant="subtitle2" gutterBottom>
            Hosts
          </Typography>
          {loadingHosts ? (
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
              <CircularProgress size={16} />
              <Typography variant="body2" color="text.secondary">
                Loading hosts...
              </Typography>
            </Box>
          ) : hosts.length === 0 ? (
            <Typography variant="body2" color="text.secondary">
              No hosts found for this cluster.
            </Typography>
          ) : (
            <FormGroup>
              <FormControlLabel
                control={
                  <Checkbox
                    checked={allSelected}
                    indeterminate={someSelected}
                    onChange={handleSelectAll}
                  />
                }
                label={
                  <Typography variant="body2" fontWeight="bold">
                    Select All ({hosts.length})
                  </Typography>
                }
              />
              <Divider sx={{ my: 0.5 }} />
              {hosts.map((host) => (
                <FormControlLabel
                  key={host.metadata.name}
                  control={
                    <Checkbox
                      checked={selectedHosts.includes(host.spec.name)}
                      onChange={() => handleHostToggle(host.spec.name)}
                    />
                  }
                  label={host.spec.name}
                />
              ))}
            </FormGroup>
          )}
        </Box>
      )}
    </Box>
  )

  const renderReview = () => (
    <Box sx={{ mt: 1 }}>
      <Typography variant="subtitle2" gutterBottom>
        Review your configuration before creating the batch.
      </Typography>
      <Box
        sx={{
          display: 'grid',
          gridTemplateColumns: '1fr 1fr',
          gap: 2,
          mt: 2,
          p: 2,
          border: '1px solid',
          borderColor: 'divider',
          borderRadius: 1
        }}
      >
        {[
          { label: 'VMware Credentials', value: stepOne.vmwareCredsName },
          { label: 'PCD / OpenStack Credentials', value: stepOne.openstackCredsName },
          { label: 'BMConfig', value: stepOne.bmConfigName },
          { label: 'Auto Start Mode', value: stepOne.autoStartMode },
          { label: 'Max Retries', value: String(stepOne.maxRetries) },
          { label: 'VMware Cluster', value: clusterName }
        ].map(({ label, value }) => (
          <Box key={label}>
            <Typography variant="caption" color="text.secondary">
              {label}
            </Typography>
            <Typography variant="body2" fontWeight={500}>
              {value}
            </Typography>
          </Box>
        ))}
      </Box>

      <Box sx={{ mt: 2 }}>
        <Typography variant="caption" color="text.secondary">
          Selected Hosts ({selectedHosts.length})
        </Typography>
        <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 0.5, mt: 0.5 }}>
          {selectedHosts.map((h) => (
            <Chip key={h} label={h} size="small" variant="outlined" />
          ))}
        </Box>
      </Box>

      {submitError && (
        <Alert severity="error" sx={{ mt: 2 }}>
          {submitError}
        </Alert>
      )}
    </Box>
  )

  const stepContent = [renderStepOne(), renderStepTwo(), renderReview()]

  return (
    <Dialog open={open} onClose={handleClose} fullWidth maxWidth="sm">
      <DialogTitle>Create Cluster Conversion Batch</DialogTitle>

      <DialogContent>
        <Stepper activeStep={activeStep} sx={{ mb: 3 }}>
          {STEPS.map((label) => (
            <Step key={label}>
              <StepLabel>{label}</StepLabel>
            </Step>
          ))}
        </Stepper>

        {stepContent[activeStep]}
      </DialogContent>

      <DialogActions sx={{ px: 3, pb: 2, gap: 1 }}>
        <Button onClick={handleClose} disabled={submitting}>
          Cancel
        </Button>
        {activeStep > 0 && (
          <Button variant="outlined" onClick={handleBack} disabled={submitting}>
            Back
          </Button>
        )}
        {activeStep < STEPS.length - 1 ? (
          <Button
            variant="contained"
            onClick={handleNext}
            disabled={activeStep === 0 ? !isStepOneValid : !isStepTwoValid}
          >
            Next
          </Button>
        ) : (
          <Button
            variant="contained"
            onClick={handleSubmit}
            disabled={submitting}
            startIcon={submitting ? <CircularProgress size={16} color="inherit" /> : undefined}
          >
            {submitting ? 'Creating...' : 'Create'}
          </Button>
        )}
      </DialogActions>
    </Dialog>
  )
}
