import React, { useCallback, useEffect, useState } from 'react'
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  FormControl,
  FormHelperText,
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
const StyledPaper = styled(Paper)(({ theme }) => ({
  width: '100%',
  height: 'calc(100vh - 96px)', // leave space for app header
  padding: theme.spacing(4),
  boxSizing: 'border-box',
  display: 'flex',
  flexDirection: 'column'
  //overflowY: 'auto'
}))

const Footer = styled(Box)(({ theme }) => ({
  display: 'flex',
  justifyContent: 'flex-end',
  gap: theme.spacing(2),
  marginTop: theme.spacing(3),
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

// Go duration regex (groups like 30s, 5m, 1h)
const GO_DURATION_FULL_REGEX = /^([0-9]+(ns|us|µs|ms|s|m|h))+$/i
const GO_DURATION_GROUP_RE = /([0-9]+)(ns|us|µs|ms|s|m|h)/gi

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

/**
 * Parse a Go duration string (like "5m", "30s", "1h30m") and return milliseconds.
 * Returns NaN if parsing fails.
 */
const parseInterval = (val: string): string | undefined => {
  const trimmedVal = val?.trim()
  if (!trimmedVal) return 'required'

  // Allow composite formats like 1h30m, 5m30s, etc.
  const regex = /^(?:(\d+)h)?(?:(\d+)m)?(?:(\d+)s)?$/
  const match = trimmedVal.match(regex)

  if (!match || match[0] === '') {
    return 'Use duration format like 30s, 5m, 1h, 1h30m, 5m30s (units: h,m,s).'
  }

  const hours = match[1] ? Number(match[1]) : 0
  const minutes = match[2] ? Number(match[2]) : 0
  const seconds = match[3] ? Number(match[3]) : 0

  // Convert total duration to minutes
  const totalMinutes = hours * 60 + minutes + seconds / 60

  if (isNaN(totalMinutes) || totalMinutes < 5) {
    return 'Interval must be at least 5 minutes'
  }

  return undefined
}
type NumberFieldProps = {
  label: string
  name: keyof SettingsForm
  value: number
  helper?: string
  min?: number
  error?: string
  onChange: (e: React.ChangeEvent<HTMLInputElement>) => void
}
const NumberField = ({ label, name, value, helper, error, onChange }: NumberFieldProps) => (
  <Box display="flex" flexDirection="column" gap={0.5}>
    <Typography variant="body2" fontWeight={500}>
      {label}
    </Typography>
    <TextField
      fullWidth
      size="small"
      type="number"
      name={String(name)}
      value={Number.isFinite(value) ? value : ''}
      onChange={onChange}
      error={!!error}
      helperText={error || helper}
    />
  </Box>
)

type TextFieldProps = {
  label: string
  name: keyof SettingsForm
  value: string
  helper?: string
  error?: string
  onChange: (e: React.ChangeEvent<HTMLInputElement>) => void
}
const CustomTextField = ({ label, name, value, helper, error, onChange }: TextFieldProps) => (
  <Box display="flex" flexDirection="column" gap={0.5}>
    <Typography variant="body2" fontWeight={500}>
      {label}
    </Typography>
    <TextField
      fullWidth
      size="small"
      name={String(name)}
      value={value}
      onChange={onChange}
      error={!!error}
      helperText={error || helper}
    />
  </Box>
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
  <Box display="flex" flexDirection="column" gap={0.5}>
    <Typography variant="body2" fontWeight={500}>
      {label}
    </Typography>
    <TextField
      fullWidth
      size="small"
      name={String(name)}
      value={value}
      onChange={onChange}
      error={!!error}
      helperText={error || helper || 'e.g. 30s, 5m, 1h30m'}
    />
  </Box>
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

  const fetchSettings = async () => {
    setLoading(true)
    try {
      const cm = await getSettingsConfigMap()
      const next = fromConfigMapData(cm?.data as any)
      setForm(next ?? DEFAULTS)
      setInitial(next ?? DEFAULTS)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchSettings()
  }, [])

  const show = useCallback(
    (message: string, severity: 'success' | 'error' | 'info' | 'warning' = 'info') =>
      setNotification({ open: true, message, severity }),
    []
  )

  const validate = useCallback((state: SettingsForm) => {
    const e: Record<string, string> = {}

    // CHANGED_BLOCKS_COPY_ITERATION_THRESHOLD: integer >=1 && <=20
    const cb = state.CHANGED_BLOCKS_COPY_ITERATION_THRESHOLD
    if (!Number.isInteger(cb) || cb < 1 || cb > 20) {
      e.CHANGED_BLOCKS_COPY_ITERATION_THRESHOLD = 'Enter an integer between 1 and 20 (inclusive).'
    }

    // PERIODIC_SYNC_INTERVAL: Go duration string and >= 5m
    const intervalStr = (state.PERIODIC_SYNC_INTERVAL ?? '').trim()
    const intervalError = parseInterval(intervalStr)
    if (intervalError) {
      e.PERIODIC_SYNC_INTERVAL = intervalError
    }

    // DEFAULT_MIGRATION_METHOD: enum hot|cold
    if (state.DEFAULT_MIGRATION_METHOD !== 'hot' && state.DEFAULT_MIGRATION_METHOD !== 'cold') {
      e.DEFAULT_MIGRATION_METHOD = "Must be 'hot' or 'cold'."
    }

    // Integer fields with >= 1 constraint
    const requiredAtLeastOne: Array<keyof SettingsForm> = [
      'VM_ACTIVE_WAIT_INTERVAL_SECONDS',
      'VM_ACTIVE_WAIT_RETRY_LIMIT',
      'VCENTER_SCAN_CONCURRENCY_LIMIT',
      'VOLUME_AVAILABLE_WAIT_INTERVAL_SECONDS',
      'VOLUME_AVAILABLE_WAIT_RETRY_LIMIT',
      'OPENSTACK_CREDS_REQUEUE_AFTER_MINUTES',
      'VMWARE_CREDS_REQUEUE_AFTER_MINUTES'
    ]

    requiredAtLeastOne.forEach((k) => {
      const v = state[k] as unknown as number
      if (!Number.isFinite(v) || !Number.isInteger(v) || v < 1) {
        e[String(k)] = 'Enter an integer >= 1.'
      }
    })

    // VCENTER_LOGIN_RETRY_LIMIT: integer >= 0
    const loginRetry = state.VCENTER_LOGIN_RETRY_LIMIT
    if (!Number.isFinite(loginRetry) || !Number.isInteger(loginRetry) || loginRetry < 0) {
      e.VCENTER_LOGIN_RETRY_LIMIT = 'Enter an integer >= 0.'
    }

    // Boolean flags must be boolean (true/false)
    const bools: Array<keyof SettingsForm> = [
      'CLEANUP_VOLUMES_AFTER_CONVERT_FAILURE',
      'POPULATE_VMWARE_MACHINE_FLAVORS',
      'VALIDATE_RDM_OWNER_VMS'
    ]
    bools.forEach((k) => {
      const val = state[k]
      if (typeof val !== 'boolean') {
        e[String(k)] = 'Must be boolean: true or false.'
      }
    })

    // DEPLOYMENT_NAME: non-empty, max 63 chars, match regex ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$
    const dn = (state.DEPLOYMENT_NAME ?? '').trim()
    //const DEPLOYMENT_NAME_RE = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/
    if (!dn) {
      e.DEPLOYMENT_NAME = 'Required.'
    } else if (dn.length > 63) {
      e.DEPLOYMENT_NAME = 'Must be 63 characters or fewer.'
    }

    // else if (!DEPLOYMENT_NAME_RE.test(dn)) {
    //   e.DEPLOYMENT_NAME =
    //     'Must match /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/ (lowercase letters, numbers and hyphens).'
    // }

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
          minHeight: 'calc(96vh - 92px)'
        }}
      >
        {/* Header */}
        {/* <Box sx={{ mb: 3 }}>
          <Typography variant="h5" sx={{ fontWeight: 700 }} gutterBottom>
            Global Settings
          </Typography>
          <Typography variant="body2" color="text.secondary">
            Configure global cluster & migration related defaults. Changes apply cluster-wide.
          </Typography>
        </Box> */}

        {/* General Settings section (3-columns) */}
        <Box sx={{ mb: 3 }}>
          <Typography variant="h6" sx={{ mb: 1 }}>
            General Settings
          </Typography>

          <Box
            sx={{
              display: 'grid',
              gridTemplateColumns: { xs: '1fr', md: 'repeat(3, 1fr)' },
              gap: 2
            }}
          >
            <CustomTextField
              label="Deployment Name"
              name="DEPLOYMENT_NAME"
              value={form.DEPLOYMENT_NAME}
              onChange={onText}
              error={errors.DEPLOYMENT_NAME}
            />

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
          </Box>
        </Box>

        {/* Retry & Interval Settings */}
        <Box sx={{ mb: 3 }}>
          <Typography variant="h6" sx={{ mb: 1 }}>
            Retry & Interval Settings
          </Typography>

          <Box
            sx={{
              display: 'grid',
              gridTemplateColumns: { xs: '1fr', md: 'repeat(3, 1fr)' },
              gap: 2
            }}
          >
            <NumberField
              label="VM Active Wait Interval (sec)"
              name="VM_ACTIVE_WAIT_INTERVAL_SECONDS"
              value={form.VM_ACTIVE_WAIT_INTERVAL_SECONDS}
              onChange={onNumber}
              error={numberError('VM_ACTIVE_WAIT_INTERVAL_SECONDS')}
            />

            <NumberField
              label="VM Active Retry Limit"
              name="VM_ACTIVE_WAIT_RETRY_LIMIT"
              value={form.VM_ACTIVE_WAIT_RETRY_LIMIT}
              onChange={onNumber}
              error={numberError('VM_ACTIVE_WAIT_RETRY_LIMIT')}
            />

            <NumberField
              label="Volume Wait Interval (sec)"
              name="VOLUME_AVAILABLE_WAIT_INTERVAL_SECONDS"
              value={form.VOLUME_AVAILABLE_WAIT_INTERVAL_SECONDS}
              onChange={onNumber}
              error={numberError('VOLUME_AVAILABLE_WAIT_INTERVAL_SECONDS')}
            />

            <NumberField
              label="Volume Retry Limit"
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
              label="vCenter Concurrency Limit"
              name="VCENTER_SCAN_CONCURRENCY_LIMIT"
              value={form.VCENTER_SCAN_CONCURRENCY_LIMIT}
              onChange={onNumber}
              error={numberError('VCENTER_SCAN_CONCURRENCY_LIMIT')}
            />
          </Box>
        </Box>

        {/* Advanced & Flags */}
        <Box sx={{ mb: 2 }}>
          <Typography variant="h6" sx={{ mb: 1 }}>
            Advanced & Flags
          </Typography>

          <Box
            sx={{
              display: 'grid',
              gridTemplateColumns: { xs: '1fr', md: 'repeat(3, 1fr)' },
              gap: 2,
              alignItems: 'center'
            }}
          >
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

            <FormControl fullWidth size="small" error={!!errors.DEFAULT_MIGRATION_METHOD}>
              <Box display="flex" flexDirection="column" gap={0.5}>
                <Typography variant="body2" fontWeight={500}>
                  Default Migration Method{' '}
                </Typography>
                <Select
                  name="DEFAULT_MIGRATION_METHOD"
                  value={form.DEFAULT_MIGRATION_METHOD}
                  onChange={onSelect}
                >
                  <MenuItem value="hot">hot</MenuItem>
                  <MenuItem value="cold">cold</MenuItem>
                </Select>
                {errors.DEFAULT_MIGRATION_METHOD && (
                  <FormHelperText>{errors.DEFAULT_MIGRATION_METHOD}</FormHelperText>
                )}
              </Box>
            </FormControl>

            {/* Flags - place them across the row to match design */}
            <Box sx={{ display: 'flex', gap: 1, alignItems: 'center', gridColumn: '1 / -1' }}>
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                <Switch
                  name="CLEANUP_VOLUMES_AFTER_CONVERT_FAILURE"
                  checked={form.CLEANUP_VOLUMES_AFTER_CONVERT_FAILURE}
                  onChange={onBool}
                />
                <Typography variant="body2">Cleanup Volumes After Convert Failure</Typography>
              </Box>

              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                <Switch
                  name="POPULATE_VMWARE_MACHINE_FLAVORS"
                  checked={form.POPULATE_VMWARE_MACHINE_FLAVORS}
                  onChange={onBool}
                />
                <Typography variant="body2">Populate VMware Machine Flavors</Typography>
              </Box>

              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                <Switch
                  name="VALIDATE_RDM_OWNER_VMS"
                  checked={form.VALIDATE_RDM_OWNER_VMS}
                  onChange={onBool}
                />
                <Typography variant="body2">Validate RDM Owner VMs</Typography>
              </Box>
            </Box>
          </Box>
        </Box>

        <Box sx={{ flexGrow: 1 }} />

        <Footer sx={{ marginTop: 'auto', marginBottom: theme.spacing(3) }}>
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
