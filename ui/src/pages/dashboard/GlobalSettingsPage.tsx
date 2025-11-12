import React, { useCallback, useEffect, useState } from 'react'
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  FormControl,
  FormHelperText,
  InputLabel,
  MenuItem,
  Paper,
  Select,
  SelectChangeEvent,
  Snackbar,
  Switch,
  TextField,
  Typography,
  styled,
  useTheme
} from '@mui/material'
import RefreshIcon from '@mui/icons-material/Refresh'
import {
  getSettingsConfigMap,
  updateSettingsConfigMap,
  VERSION_CONFIG_MAP_NAME,
  VERSION_NAMESPACE
} from 'src/api/settings/settings'

// Styled components
const StyledPaper = styled(Paper)({
  width: '100%',
  padding: 24,
  boxSizing: 'border-box'
})

const Footer = styled(Box)(({ theme }) => ({
  display: 'flex',
  justifyContent: 'flex-end',
  gap: theme.spacing(2),
  marginTop: theme.spacing(1),
  paddingTop: theme.spacing(2),
  borderTop: `1px solid ${theme.palette.divider}`
}))

/* ------------------------
   Types & Defaults
   -----------------------*/
type SettingsForm = {
  CHANGED_BLOCKS_COPY_ITERATION_THRESHOLD: number
  PERIODIC_SYNC_INTERVAL: string
  VM_ACTIVE_WAIT_INTERVAL_SECONDS: number
  VM_ACTIVE_WAIT_RETRY_LIMIT: number
  DEFAULT_MIGRATION_METHOD: 'hot' | 'cold'
  VCENTER_SCAN_CONCURRENCY_LIMIT: number
  CLEANUP_VOLUMES_AFTER_CONVERT_FAILURE: boolean
  POPULATE_VMWARE_MACHINE_FLAVORS: boolean
  VOLUME_AVAILABLE_WAIT_INTERVAL_SECONDS: number
  VOLUME_AVAILABLE_WAIT_RETRY_LIMIT: number
  VCENTER_LOGIN_RETRY_LIMIT: number
  OPENSTACK_CREDS_REQUEUE_AFTER_MINUTES: number
  VMWARE_CREDS_REQUEUE_AFTER_MINUTES: number
  VALIDATE_RDM_OWNER_VMS: boolean
  DEPLOYMENT_NAME: string
}

const DEFAULTS: SettingsForm = {
  CHANGED_BLOCKS_COPY_ITERATION_THRESHOLD: 20,
  PERIODIC_SYNC_INTERVAL: '1h',
  VM_ACTIVE_WAIT_INTERVAL_SECONDS: 20,
  VM_ACTIVE_WAIT_RETRY_LIMIT: 15,
  DEFAULT_MIGRATION_METHOD: 'hot',
  VCENTER_SCAN_CONCURRENCY_LIMIT: 10,
  CLEANUP_VOLUMES_AFTER_CONVERT_FAILURE: false,
  POPULATE_VMWARE_MACHINE_FLAVORS: true,
  VOLUME_AVAILABLE_WAIT_INTERVAL_SECONDS: 10,
  VOLUME_AVAILABLE_WAIT_RETRY_LIMIT: 15,
  VCENTER_LOGIN_RETRY_LIMIT: 5,
  OPENSTACK_CREDS_REQUEUE_AFTER_MINUTES: 60,
  VMWARE_CREDS_REQUEUE_AFTER_MINUTES: 60,
  VALIDATE_RDM_OWNER_VMS: true,
  DEPLOYMENT_NAME: 'vJailbreak'
}

/* ------------------------
   Helpers (parsing, IO)
   -----------------------*/
const INTERVAL_REGEX = /^\s*\d+\s*(s|m|h|d)\s*$/i

const parseBool = (v: unknown, fallback: boolean) =>
  typeof v === 'string' ? v.toLowerCase() === 'true' : typeof v === 'boolean' ? v : fallback

const parseNum = (v: unknown, fallback: number) => {
  const n = typeof v === 'string' ? Number(v) : typeof v === 'number' ? v : NaN
  return Number.isFinite(n) ? n : fallback
}

const toConfigMapData = (f: SettingsForm): Record<string, string> => ({
  CHANGED_BLOCKS_COPY_ITERATION_THRESHOLD: String(f.CHANGED_BLOCKS_COPY_ITERATION_THRESHOLD),
  PERIODIC_SYNC_INTERVAL: f.PERIODIC_SYNC_INTERVAL,
  VM_ACTIVE_WAIT_INTERVAL_SECONDS: String(f.VM_ACTIVE_WAIT_INTERVAL_SECONDS),
  VM_ACTIVE_WAIT_RETRY_LIMIT: String(f.VM_ACTIVE_WAIT_RETRY_LIMIT),
  DEFAULT_MIGRATION_METHOD: f.DEFAULT_MIGRATION_METHOD,
  VCENTER_SCAN_CONCURRENCY_LIMIT: String(f.VCENTER_SCAN_CONCURRENCY_LIMIT),
  CLEANUP_VOLUMES_AFTER_CONVERT_FAILURE: String(f.CLEANUP_VOLUMES_AFTER_CONVERT_FAILURE),
  POPULATE_VMWARE_MACHINE_FLAVORS: String(f.POPULATE_VMWARE_MACHINE_FLAVORS),
  VOLUME_AVAILABLE_WAIT_INTERVAL_SECONDS: String(f.VOLUME_AVAILABLE_WAIT_INTERVAL_SECONDS),
  VOLUME_AVAILABLE_WAIT_RETRY_LIMIT: String(f.VOLUME_AVAILABLE_WAIT_RETRY_LIMIT),
  VCENTER_LOGIN_RETRY_LIMIT: String(f.VCENTER_LOGIN_RETRY_LIMIT),
  OPENSTACK_CREDS_REQUEUE_AFTER_MINUTES: String(f.OPENSTACK_CREDS_REQUEUE_AFTER_MINUTES),
  VMWARE_CREDS_REQUEUE_AFTER_MINUTES: String(f.VMWARE_CREDS_REQUEUE_AFTER_MINUTES),
  VALIDATE_RDM_OWNER_VMS: String(f.VALIDATE_RDM_OWNER_VMS),
  DEPLOYMENT_NAME: f.DEPLOYMENT_NAME
})

const fromConfigMapData = (data: Record<string, string> | undefined): SettingsForm => ({
  CHANGED_BLOCKS_COPY_ITERATION_THRESHOLD: parseNum(
    data?.CHANGED_BLOCKS_COPY_ITERATION_THRESHOLD,
    DEFAULTS.CHANGED_BLOCKS_COPY_ITERATION_THRESHOLD
  ),
  PERIODIC_SYNC_INTERVAL: data?.PERIODIC_SYNC_INTERVAL ?? DEFAULTS.PERIODIC_SYNC_INTERVAL,
  VM_ACTIVE_WAIT_INTERVAL_SECONDS: parseNum(
    data?.VM_ACTIVE_WAIT_INTERVAL_SECONDS,
    DEFAULTS.VM_ACTIVE_WAIT_INTERVAL_SECONDS
  ),
  VM_ACTIVE_WAIT_RETRY_LIMIT: parseNum(
    data?.VM_ACTIVE_WAIT_RETRY_LIMIT,
    DEFAULTS.VM_ACTIVE_WAIT_RETRY_LIMIT
  ),
  DEFAULT_MIGRATION_METHOD: (data?.DEFAULT_MIGRATION_METHOD === 'cold' ? 'cold' : 'hot') as
    | 'hot'
    | 'cold',
  VCENTER_SCAN_CONCURRENCY_LIMIT: parseNum(
    data?.VCENTER_SCAN_CONCURRENCY_LIMIT,
    DEFAULTS.VCENTER_SCAN_CONCURRENCY_LIMIT
  ),
  CLEANUP_VOLUMES_AFTER_CONVERT_FAILURE: parseBool(
    data?.CLEANUP_VOLUMES_AFTER_CONVERT_FAILURE,
    DEFAULTS.CLEANUP_VOLUMES_AFTER_CONVERT_FAILURE
  ),
  POPULATE_VMWARE_MACHINE_FLAVORS: parseBool(
    data?.POPULATE_VMWARE_MACHINE_FLAVORS,
    DEFAULTS.POPULATE_VMWARE_MACHINE_FLAVORS
  ),
  VOLUME_AVAILABLE_WAIT_INTERVAL_SECONDS: parseNum(
    data?.VOLUME_AVAILABLE_WAIT_INTERVAL_SECONDS,
    DEFAULTS.VOLUME_AVAILABLE_WAIT_INTERVAL_SECONDS
  ),
  VOLUME_AVAILABLE_WAIT_RETRY_LIMIT: parseNum(
    data?.VOLUME_AVAILABLE_WAIT_RETRY_LIMIT,
    DEFAULTS.VOLUME_AVAILABLE_WAIT_RETRY_LIMIT
  ),
  VCENTER_LOGIN_RETRY_LIMIT: parseNum(
    data?.VCENTER_LOGIN_RETRY_LIMIT,
    DEFAULTS.VCENTER_LOGIN_RETRY_LIMIT
  ),
  OPENSTACK_CREDS_REQUEUE_AFTER_MINUTES: parseNum(
    data?.OPENSTACK_CREDS_REQUEUE_AFTER_MINUTES,
    DEFAULTS.OPENSTACK_CREDS_REQUEUE_AFTER_MINUTES
  ),
  VMWARE_CREDS_REQUEUE_AFTER_MINUTES: parseNum(
    data?.VMWARE_CREDS_REQUEUE_AFTER_MINUTES,
    DEFAULTS.VMWARE_CREDS_REQUEUE_AFTER_MINUTES
  ),
  VALIDATE_RDM_OWNER_VMS: parseBool(data?.VALIDATE_RDM_OWNER_VMS, DEFAULTS.VALIDATE_RDM_OWNER_VMS),
  DEPLOYMENT_NAME: data?.DEPLOYMENT_NAME ?? DEFAULTS.DEPLOYMENT_NAME
})

type NumberFieldProps = {
  label: string
  name: keyof SettingsForm
  value: number
  helper?: string
  min?: number
  error?: string
  onChange: (e: React.ChangeEvent<HTMLInputElement>) => void
}
const NumberField = ({
  label,
  name,
  value,
  helper,
  min = 0,
  error,
  onChange
}: NumberFieldProps) => (
  <TextField
    fullWidth
    size="small"
    type="number"
    inputProps={{ min }}
    label={label}
    name={String(name)}
    value={Number.isFinite(value) ? value : ''}
    onChange={onChange}
    error={!!error}
    helperText={error || helper}
  />
)

type IntervalFieldProps = {
  label: string
  name: keyof SettingsForm
  value: string
  helper?: string
  error?: string
  onChange: (e: React.ChangeEvent<HTMLInputElement>) => void
}
const IntervalField = ({ label, name, value, helper, error, onChange }: IntervalFieldProps) => (
  <TextField
    fullWidth
    size="small"
    label={label}
    name={String(name)}
    value={value}
    onChange={onChange}
    error={!!error}
    helperText={error || helper || 'e.g. 30s, 5m, 1h, 2d'}
  />
)

/* ------------------------
   Main component
   -----------------------*/
export default function GlobalSettingsPage() {
  const theme = useTheme()
  const [form, setForm] = useState<SettingsForm>(DEFAULTS)
  const [initial, setInitial] = useState<SettingsForm>(DEFAULTS)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [errors, setErrors] = useState<Record<string, string>>({})

  const [notification, setNotification] = useState<{
    open: boolean
    message: string
    severity: 'error' | 'info' | 'success' | 'warning'
  }>({ open: false, message: '', severity: 'info' })

  // Fetch on mount
  useEffect(() => {
    let cancelled = false
    ;(async () => {
      try {
        setLoading(true)
        const cm = await getSettingsConfigMap()
        if (cancelled) return
        const next = fromConfigMapData(cm?.data as any)
        setForm(next)
        setInitial(next)
      } catch (err) {
        // keep defaults if not found
        setNotification({
          open: true,
          message:
            'No existing Global Settings found. Defaults are prefilled; adjust and Save to create.',
          severity: 'info'
        })
        setForm(DEFAULTS)
        setInitial(DEFAULTS)
      } finally {
        if (!cancelled) setLoading(false)
      }
    })()
    return () => {
      cancelled = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const show = useCallback(
    (message: string, severity: 'success' | 'error' | 'info' | 'warning' = 'info') =>
      setNotification({ open: true, message, severity }),
    []
  )

  const validate = useCallback((state: SettingsForm) => {
    const e: Record<string, string> = {}

    if (!INTERVAL_REGEX.test(state.PERIODIC_SYNC_INTERVAL)) {
      e.PERIODIC_SYNC_INTERVAL = 'Use formats like 30s, 5m, 1h, 2d'
    }

    if (state.DEFAULT_MIGRATION_METHOD !== 'hot' && state.DEFAULT_MIGRATION_METHOD !== 'cold') {
      e.DEFAULT_MIGRATION_METHOD = "Must be 'hot' or 'cold'"
    }

    const positiveInts: Array<keyof SettingsForm> = [
      'CHANGED_BLOCKS_COPY_ITERATION_THRESHOLD',
      'VM_ACTIVE_WAIT_INTERVAL_SECONDS',
      'VM_ACTIVE_WAIT_RETRY_LIMIT',
      'VCENTER_SCAN_CONCURRENCY_LIMIT',
      'VOLUME_AVAILABLE_WAIT_INTERVAL_SECONDS',
      'VOLUME_AVAILABLE_WAIT_RETRY_LIMIT',
      'VCENTER_LOGIN_RETRY_LIMIT',
      'OPENSTACK_CREDS_REQUEUE_AFTER_MINUTES',
      'VMWARE_CREDS_REQUEUE_AFTER_MINUTES'
    ]

    positiveInts.forEach((k) => {
      const v = state[k] as unknown as number
      if (!Number.isFinite(v) || v < 0) e[String(k)] = 'Enter a non-negative number'
    })

    if (!state.DEPLOYMENT_NAME.trim()) e.DEPLOYMENT_NAME = 'Required'

    setErrors(e)
    return Object.keys(e).length === 0
  }, [])

  /* ------------------------
     Event handlers
     -----------------------*/
  const onText = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const { name, value } = e.target
    setForm((prev) => ({ ...prev, [name]: value }))
  }, [])

  const onNumber = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const { name, value } = e.target
    const n = value === '' ? ('' as unknown as number) : Number(value)
    setForm((prev) => ({ ...prev, [name]: n }))
  }, [])

  const onBool = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const { name, checked } = e.target
    setForm((prev) => ({ ...prev, [name]: checked }))
  }, [])

  const onSelect = useCallback((e: SelectChangeEvent<string>) => {
    const { name, value } = e.target
    setForm((prev) => ({ ...prev, [name]: value as 'hot' | 'cold' }))
  }, [])

  const onResetDefaults = useCallback(() => {
    setForm(DEFAULTS)
    setErrors({})
  }, [])

  const onCancel = useCallback(() => {
    setForm(initial)
    setErrors({})
  }, [initial])

  const onSave = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault()
      if (!validate(form)) {
        show('Please fix the validation errors.', 'error')
        return
      }
      setSaving(true)
      try {
        // call save API with the full resource shape expected by the API
        await updateSettingsConfigMap({
          apiVersion: 'v1',
          kind: 'ConfigMap',
          metadata: { name: VERSION_CONFIG_MAP_NAME, namespace: VERSION_NAMESPACE },
          data: toConfigMapData(form)
        } as any)
        setInitial(form)
        show('Global Settings saved successfully.', 'success')
      } catch (err) {
        show('Failed to save Global Settings.', 'error')
      } finally {
        setSaving(false)
      }
    },
    [form, validate, show]
  )

  const numberError = useCallback((key: keyof SettingsForm) => errors[String(key)], [errors])

  if (loading) {
    return (
      <StyledPaper elevation={0}>
        <Box display="flex" justifyContent="center" alignItems="center" height="400px">
          <CircularProgress />
          <Typography variant="body1" sx={{ ml: 2 }}>
            Loading Global Settings...
          </Typography>
        </Box>
      </StyledPaper>
    )
  }

  return (
    <StyledPaper elevation={0}>
      <Box
        component="form"
        onSubmit={onSave}
        sx={{
          display: 'flex',
          flexDirection: 'column',
          // make the form occupy full viewport height so footer can stick to bottom
          minHeight: 'calc(94vh - 92px)', // subtract small padding so it fits within page
          gap: 0
        }}
      >
        <Box>
          <Typography variant="h6" gutterBottom>
            Global Settings
          </Typography>

          <Box sx={{ mt: 2 }}>
            <Box
              sx={{
                display: 'grid',
                gridTemplateColumns: { xs: '1fr', md: 'repeat(3, 1fr)' },
                gap: 2
              }}
            >
              <NumberField
                label="Changed Blocks Copy Iteration Threshold"
                name="CHANGED_BLOCKS_COPY_ITERATION_THRESHOLD"
                value={form.CHANGED_BLOCKS_COPY_ITERATION_THRESHOLD}
                onChange={onNumber}
                error={numberError('CHANGED_BLOCKS_COPY_ITERATION_THRESHOLD')}
              />

              <IntervalField
                label="Periodic Sync Interval"
                name="PERIODIC_SYNC_INTERVAL"
                value={form.PERIODIC_SYNC_INTERVAL}
                onChange={onText}
                error={errors.PERIODIC_SYNC_INTERVAL}
              />

              <FormControl fullWidth size="small" error={!!errors.DEFAULT_MIGRATION_METHOD}>
                <InputLabel>Default Migration Method</InputLabel>
                <Select
                  label="Default Migration Method"
                  name="DEFAULT_MIGRATION_METHOD"
                  value={form.DEFAULT_MIGRATION_METHOD}
                  onChange={onSelect}
                  size="small"
                >
                  <MenuItem value="hot">hot</MenuItem>
                  <MenuItem value="cold">cold</MenuItem>
                </Select>
                {errors.DEFAULT_MIGRATION_METHOD && (
                  <FormHelperText>{errors.DEFAULT_MIGRATION_METHOD}</FormHelperText>
                )}
              </FormControl>

              <NumberField
                label="VM Active Wait Interval (sec)"
                name="VM_ACTIVE_WAIT_INTERVAL_SECONDS"
                value={form.VM_ACTIVE_WAIT_INTERVAL_SECONDS}
                onChange={onNumber}
                error={numberError('VM_ACTIVE_WAIT_INTERVAL_SECONDS')}
              />

              <NumberField
                label="VM Active Wait Retry Limit"
                name="VM_ACTIVE_WAIT_RETRY_LIMIT"
                value={form.VM_ACTIVE_WAIT_RETRY_LIMIT}
                onChange={onNumber}
                error={numberError('VM_ACTIVE_WAIT_RETRY_LIMIT')}
              />

              <NumberField
                label="vCenter Scan Concurrency Limit"
                name="VCENTER_SCAN_CONCURRENCY_LIMIT"
                value={form.VCENTER_SCAN_CONCURRENCY_LIMIT}
                onChange={onNumber}
                error={numberError('VCENTER_SCAN_CONCURRENCY_LIMIT')}
              />

              <Box>
                <Typography variant="subtitle2" gutterBottom>
                  Cleanup Volumes After Convert Failure
                </Typography>
                <Switch
                  name="CLEANUP_VOLUMES_AFTER_CONVERT_FAILURE"
                  checked={form.CLEANUP_VOLUMES_AFTER_CONVERT_FAILURE}
                  onChange={onBool}
                />
              </Box>

              <Box>
                <Typography variant="subtitle2" gutterBottom>
                  Populate VMware Machine Flavors
                </Typography>
                <Switch
                  name="POPULATE_VMWARE_MACHINE_FLAVORS"
                  checked={form.POPULATE_VMWARE_MACHINE_FLAVORS}
                  onChange={onBool}
                />
              </Box>

              <Box>
                <Typography variant="subtitle2" gutterBottom>
                  Validate RDM Owner VMs
                </Typography>
                <Switch
                  name="VALIDATE_RDM_OWNER_VMS"
                  checked={form.VALIDATE_RDM_OWNER_VMS}
                  onChange={onBool}
                />
              </Box>

              <NumberField
                label="Volume Available Wait Interval (sec)"
                name="VOLUME_AVAILABLE_WAIT_INTERVAL_SECONDS"
                value={form.VOLUME_AVAILABLE_WAIT_INTERVAL_SECONDS}
                onChange={onNumber}
                error={numberError('VOLUME_AVAILABLE_WAIT_INTERVAL_SECONDS')}
              />

              <NumberField
                label="Volume Available Wait Retry Limit"
                name="VOLUME_AVAILABLE_WAIT_RETRY_LIMIT"
                value={form.VOLUME_AVAILABLE_WAIT_RETRY_LIMIT}
                onChange={onNumber}
                error={numberError('VOLUME_AVAILABLE_WAIT_RETRY_LIMIT')}
              />

              <NumberField
                label="vCenter Login Retry Limit"
                name="VCENTER_LOGIN_RETRY_LIMIT"
                value={form.VCENTER_LOGIN_RETRY_LIMIT}
                onChange={onNumber}
                error={numberError('VCENTER_LOGIN_RETRY_LIMIT')}
              />

              <NumberField
                label="OpenStack Creds Requeue After (minutes)"
                name="OPENSTACK_CREDS_REQUEUE_AFTER_MINUTES"
                value={form.OPENSTACK_CREDS_REQUEUE_AFTER_MINUTES}
                onChange={onNumber}
                error={numberError('OPENSTACK_CREDS_REQUEUE_AFTER_MINUTES')}
              />

              <NumberField
                label="VMware Creds Requeue After (minutes)"
                name="VMWARE_CREDS_REQUEUE_AFTER_MINUTES"
                value={form.VMWARE_CREDS_REQUEUE_AFTER_MINUTES}
                onChange={onNumber}
                error={numberError('VMWARE_CREDS_REQUEUE_AFTER_MINUTES')}
              />

              <TextField
                fullWidth
                size="small"
                label="Deployment Name"
                name="DEPLOYMENT_NAME"
                value={form.DEPLOYMENT_NAME}
                onChange={onText}
                error={!!errors.DEPLOYMENT_NAME}
                helperText={errors.DEPLOYMENT_NAME}
              />
            </Box>
          </Box>
        </Box>

        {/* spacer pushes footer to bottom via marginTop: auto */}
        <Box sx={{ flexGrow: 1 }} />

        <Footer sx={{ marginTop: 'auto' }}>
          <Button
            variant="outlined"
            color="inherit"
            onClick={onResetDefaults}
            startIcon={<RefreshIcon />}
          >
            Reset to Defaults
          </Button>
          <Button variant="outlined" onClick={onCancel}>
            Cancel
          </Button>
          <Button
            variant="contained"
            type="submit"
            color="primary"
            disabled={saving}
            startIcon={saving ? <CircularProgress size={20} color="inherit" /> : undefined}
          >
            {saving ? 'Saving...' : 'Save'}
          </Button>
        </Footer>
      </Box>

      <Snackbar
        open={notification.open}
        autoHideDuration={6000}
        onClose={() => setNotification((p) => ({ ...p, open: false }))}
      >
        <Alert
          severity={notification.severity}
          onClose={() => setNotification((p) => ({ ...p, open: false }))}
          sx={{ width: '100%' }}
        >
          {notification.message}
        </Alert>
      </Snackbar>
    </StyledPaper>
  )
}
