import React, { SyntheticEvent, useCallback, useEffect, useState } from 'react'
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
  Tab,
  Tabs,
  TextField,
  Tooltip,
  Typography,
  styled,
  useTheme
} from '@mui/material'
import RefreshIcon from '@mui/icons-material/Refresh'
import InfoOutlinedIcon from '@mui/icons-material/InfoOutlined'
import SettingsOutlinedIcon from '@mui/icons-material/SettingsOutlined'
import HistoryToggleOffOutlinedIcon from '@mui/icons-material/HistoryToggleOffOutlined'
import TuneOutlinedIcon from '@mui/icons-material/TuneOutlined'
import CloudUploadIcon from '@mui/icons-material/CloudUpload'
import {
  getSettingsConfigMap,
  updateSettingsConfigMap,
  VERSION_CONFIG_MAP_NAME,
  VERSION_NAMESPACE
} from 'src/api/settings/settings'
import VDDKUpload from 'src/components/VDDKUpload'

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

const SettingsGrid = styled(Box)(({ theme }) => ({
  display: 'grid',
  gridTemplateColumns: 'repeat(auto-fit, minmax(320px, 1fr))',
  gap: theme.spacing(2)
}))

const FieldLabel = ({ label, tooltip }: { label: string; tooltip?: string }) => (
  <Box display="flex" alignItems="center" gap={0.75}>
    <Typography variant="body2" fontWeight={500} color="text.primary">
      {label}
    </Typography>
    {tooltip ? (
      <Tooltip
        placement="top"
        arrow
        componentsProps={{
          tooltip: {
            sx: {
              fontSize: '12px',
              lineHeight: 1.5,
              letterSpacing: 0
            }
          }
        }}
        title={
          <Typography variant="caption" sx={{ fontSize: '12px', lineHeight: 1, letterSpacing: 0 }}>
            {tooltip}
          </Typography>
        }
      >
        <InfoOutlinedIcon sx={{ fontSize: 18, color: 'text.secondary', cursor: 'pointer' }} />
      </Tooltip>
    ) : null}
  </Box>
)

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
  CLEANUP_PORTS_AFTER_MIGRATION_FAILURE: boolean
  POPULATE_VMWARE_MACHINE_FLAVORS: boolean
  VOLUME_AVAILABLE_WAIT_INTERVAL_SECONDS: number
  VOLUME_AVAILABLE_WAIT_RETRY_LIMIT: number
  VCENTER_LOGIN_RETRY_LIMIT: number
  OPENSTACK_CREDS_REQUEUE_AFTER_MINUTES: number
  VMWARE_CREDS_REQUEUE_AFTER_MINUTES: number
  VALIDATE_RDM_OWNER_VMS: boolean
  AUTO_FSTAB_UPDATE: boolean
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
  CLEANUP_PORTS_AFTER_MIGRATION_FAILURE: false,
  POPULATE_VMWARE_MACHINE_FLAVORS: true,
  VOLUME_AVAILABLE_WAIT_INTERVAL_SECONDS: 10,
  VOLUME_AVAILABLE_WAIT_RETRY_LIMIT: 15,
  VCENTER_LOGIN_RETRY_LIMIT: 5,
  OPENSTACK_CREDS_REQUEUE_AFTER_MINUTES: 60,
  VMWARE_CREDS_REQUEUE_AFTER_MINUTES: 60,
  VALIDATE_RDM_OWNER_VMS: true,
  AUTO_FSTAB_UPDATE: false,
  DEPLOYMENT_NAME: 'vJailbreak'
}

type FormUpdater = (prev: SettingsForm) => SettingsForm
type TabKey = 'general' | 'retry' | 'advanced' | 'vddk'

const TAB_FIELD_KEYS: Record<TabKey, Array<keyof SettingsForm>> = {
  general: ['DEPLOYMENT_NAME', 'CHANGED_BLOCKS_COPY_ITERATION_THRESHOLD', 'PERIODIC_SYNC_INTERVAL'],
  retry: [
    'VM_ACTIVE_WAIT_INTERVAL_SECONDS',
    'VM_ACTIVE_WAIT_RETRY_LIMIT',
    'VOLUME_AVAILABLE_WAIT_INTERVAL_SECONDS',
    'VOLUME_AVAILABLE_WAIT_RETRY_LIMIT',
    'VCENTER_LOGIN_RETRY_LIMIT',
    'VCENTER_SCAN_CONCURRENCY_LIMIT'
  ],
  advanced: [
    'OPENSTACK_CREDS_REQUEUE_AFTER_MINUTES',
    'VMWARE_CREDS_REQUEUE_AFTER_MINUTES',
    'DEFAULT_MIGRATION_METHOD',
    'CLEANUP_VOLUMES_AFTER_CONVERT_FAILURE',
    'CLEANUP_PORTS_AFTER_MIGRATION_FAILURE',
    'POPULATE_VMWARE_MACHINE_FLAVORS',
    'VALIDATE_RDM_OWNER_VMS',
    'AUTO_FSTAB_UPDATE'
  ],
  vddk: []
}

const TAB_ORDER: TabKey[] = ['general', 'retry', 'advanced', 'vddk']

const TAB_META: Record<TabKey, { label: string; helper: string; icon: React.ReactNode }> = {
  general: {
    label: 'General',
    helper: 'Keep the deployment identity and cadence consistent across the platform.',
    icon: <SettingsOutlinedIcon fontSize="small" />
  },
  retry: {
    label: 'Intervals',
    helper:
      'Control wait intervals, retry tolerances, and concurrency to balance speed vs. safety.',
    icon: <HistoryToggleOffOutlinedIcon fontSize="small" />
  },
  advanced: {
    label: 'Advanced',
    helper: 'Tune integration defaults and automation flags for OpenStack and VMware flows.',
    icon: <TuneOutlinedIcon fontSize="small" />
  },
  vddk: {
    label: 'VDDK Upload',
    helper: 'Upload VDDK tar files to be extracted to /home/ubuntu on the server.',
    icon: <CloudUploadIcon fontSize="small" />
  }
}

const TabLabel = ({
  label,
  showError,
  icon
}: {
  label: string
  showError: boolean
  icon: React.ReactNode
}) => (
  <Box display="flex" alignItems="center" gap={0.75}>
    <Box component="span" sx={{ display: 'flex', alignItems: 'center', color: 'text.secondary' }}>
      {icon}
    </Box>
    <Typography variant="body2" fontWeight={600}>
      {label}
    </Typography>
    {showError ? (
      <Box
        component="span"
        sx={{ width: 8, height: 8, bgcolor: 'error.main', borderRadius: '50%' }}
      />
    ) : null}
  </Box>
)

const TabPanel = ({
  children,
  current,
  value
}: {
  children: React.ReactNode
  current: TabKey
  value: TabKey
}) => {
  if (current !== value) return null
  return (
    <Box
      role="tabpanel"
      id={`settings-tabpanel-${value}`}
      aria-labelledby={`settings-tab-${value}`}
      sx={{ pt: 3 }}
    >
      {children}
    </Box>
  )
}

const FIELD_TOOLTIPS: Record<keyof SettingsForm, string> = {
  DEPLOYMENT_NAME: 'Display name shown across dashboards and exported workflows.',
  CHANGED_BLOCKS_COPY_ITERATION_THRESHOLD: 'Number of iterations to copy changed blocks.',
  PERIODIC_SYNC_INTERVAL: 'Frequency for background periodic sync jobs (minimum 5 minutes).',
  VM_ACTIVE_WAIT_INTERVAL_SECONDS: 'Interval to wait for VM to become active (in seconds).',
  VM_ACTIVE_WAIT_RETRY_LIMIT: 'Number of retries to wait for VM to become active.',
  VOLUME_AVAILABLE_WAIT_INTERVAL_SECONDS:
    'Delay between retries while tracking volume availability.',
  VOLUME_AVAILABLE_WAIT_RETRY_LIMIT: 'Maximum attempts to wait for volumes before failing the job.',
  VCENTER_LOGIN_RETRY_LIMIT:
    'Number of login retries before the workflow surfaces an authentication error.',
  VCENTER_SCAN_CONCURRENCY_LIMIT: 'Maximum number of vCenter VMs to scan concurrently.',
  OPENSTACK_CREDS_REQUEUE_AFTER_MINUTES:
    'Time before failed OpenStack credentials are re-queued for another attempt.',
  VMWARE_CREDS_REQUEUE_AFTER_MINUTES: 'Time before VMware credential rotations are retried.',
  DEFAULT_MIGRATION_METHOD:
    'Default method for VM migration (placeholder for future releases, not currently used).',
  CLEANUP_VOLUMES_AFTER_CONVERT_FAILURE:
    'Automatically delete intermediate volumes when a conversion fails.',
  CLEANUP_PORTS_AFTER_MIGRATION_FAILURE:
    'Automatically delete network ports when a migration fails.',
  POPULATE_VMWARE_MACHINE_FLAVORS:
    'Fetch VMware hardware flavors to enrich instance sizing details.',
  VALIDATE_RDM_OWNER_VMS: 'Ensure Raw Device Mapping owners are validated before migration.',
  AUTO_FSTAB_UPDATE: 'Automatically update fstab entries during VM migration.'
}

type ToggleKey = Extract<
  keyof SettingsForm,
  | 'CLEANUP_VOLUMES_AFTER_CONVERT_FAILURE'
  | 'CLEANUP_PORTS_AFTER_MIGRATION_FAILURE'
  | 'POPULATE_VMWARE_MACHINE_FLAVORS'
  | 'VALIDATE_RDM_OWNER_VMS'
  | 'AUTO_FSTAB_UPDATE'
>

const TOGGLE_FIELDS: Array<{ key: ToggleKey; label: string; description: string }> = [
  {
    key: 'CLEANUP_VOLUMES_AFTER_CONVERT_FAILURE',
    label: 'Cleanup Volumes After Conversion Failure',
    description: 'Remove orphaned storage artifacts after a failed conversion run.'
  },
  {
    key: 'CLEANUP_PORTS_AFTER_MIGRATION_FAILURE',
    label: 'Cleanup Ports After Migration Failure',
    description: 'Remove orphaned network ports after a failed migration run.'
  },
  {
    key: 'POPULATE_VMWARE_MACHINE_FLAVORS',
    label: 'Populate VMware Machine Flavors',
    description: 'Sync VMware flavor data to pre-fill CPU, memory, and disk sizing hints.'
  },
  {
    key: 'VALIDATE_RDM_OWNER_VMS',
    label: 'Validate MigrationPlan for RDM VMs',
    description: 'Adds guard rails to ensure RDM devices still belong to the reported VM owner.'
  },
  {
    key: 'AUTO_FSTAB_UPDATE',
    label: 'Auto Fstab Update',
    description: 'Automatically update fstab entries to ensure proper disk mounting after migration.'
  }
]

/* ------------------------
   Helpers (parsing, IO)
   -----------------------*/

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
  CLEANUP_PORTS_AFTER_MIGRATION_FAILURE: String(f.CLEANUP_PORTS_AFTER_MIGRATION_FAILURE),
  POPULATE_VMWARE_MACHINE_FLAVORS: String(f.POPULATE_VMWARE_MACHINE_FLAVORS),
  VOLUME_AVAILABLE_WAIT_INTERVAL_SECONDS: String(f.VOLUME_AVAILABLE_WAIT_INTERVAL_SECONDS),
  VOLUME_AVAILABLE_WAIT_RETRY_LIMIT: String(f.VOLUME_AVAILABLE_WAIT_RETRY_LIMIT),
  VCENTER_LOGIN_RETRY_LIMIT: String(f.VCENTER_LOGIN_RETRY_LIMIT),
  OPENSTACK_CREDS_REQUEUE_AFTER_MINUTES: String(f.OPENSTACK_CREDS_REQUEUE_AFTER_MINUTES),
  VMWARE_CREDS_REQUEUE_AFTER_MINUTES: String(f.VMWARE_CREDS_REQUEUE_AFTER_MINUTES),
  VALIDATE_RDM_OWNER_VMS: String(f.VALIDATE_RDM_OWNER_VMS),
  AUTO_FSTAB_UPDATE: String(f.AUTO_FSTAB_UPDATE),
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
  CLEANUP_PORTS_AFTER_MIGRATION_FAILURE: parseBool(
    data?.CLEANUP_PORTS_AFTER_MIGRATION_FAILURE,
    DEFAULTS.CLEANUP_PORTS_AFTER_MIGRATION_FAILURE
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
  AUTO_FSTAB_UPDATE: parseBool(data?.AUTO_FSTAB_UPDATE, DEFAULTS.AUTO_FSTAB_UPDATE),
  DEPLOYMENT_NAME: data?.DEPLOYMENT_NAME ?? DEFAULTS.DEPLOYMENT_NAME
})

/**
 * Parse a Go duration string (like "5m", "30s", "1h30m") and return milliseconds.
 * Returns NaN if parsing fails.
 */
const parseInterval = (val: string): string | undefined => {
  const trimmedVal = val?.trim()
  if (!trimmedVal) return 'Periodic Sync is required'

  // Allow composite formats like 1h30m, 5m30s, etc.
  const regex = /^(?:(\d+)h)?(?:(\d+)m)?(?:(\d+)s)?$/
  const match = trimmedVal.match(regex)

  if (!match || match[0] === '') {
    return 'Use duration format like 5m, 1h30m, 5m30s (units: h,m,s).'
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
  tooltip?: string
}
const NumberField = ({
  label,
  name,
  value,
  helper,
  error,
  onChange,
  tooltip
}: NumberFieldProps) => (
  <Box display="flex" flexDirection="column" gap={0.5}>
    <FieldLabel label={label} tooltip={tooltip} />
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
  tooltip?: string
}
const CustomTextField = ({
  label,
  name,
  value,
  helper,
  error,
  onChange,
  tooltip
}: TextFieldProps) => (
  <Box display="flex" flexDirection="column" gap={0.5}>
    <FieldLabel label={label} tooltip={tooltip} />
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
  tooltip?: string
}
const IntervalField = ({
  label,
  name,
  value,
  helper,
  error,
  onChange,
  tooltip
}: IntervalFieldProps) => (
  <Box display="flex" flexDirection="column" gap={0.5}>
    <FieldLabel label={label} tooltip={tooltip} />
    <TextField
      fullWidth
      size="small"
      name={String(name)}
      value={value}
      onChange={onChange}
      error={!!error}
      helperText={error || helper || 'e.g. 5m, 1h30m, 5m30s (units: h,m,s)'}
    />
  </Box>
)

type ToggleFieldProps = {
  label: string
  name: keyof SettingsForm
  checked: boolean
  onChange: (e: React.ChangeEvent<HTMLInputElement>) => void
  tooltip?: string
  description?: string
}

const ToggleField = ({
  label,
  name,
  checked,
  onChange,
  tooltip,
  description
}: ToggleFieldProps) => (
  <Paper variant="outlined" sx={{ p: 2, display: 'flex', flexDirection: 'column', gap: 1 }}>
    <Box display="flex" alignItems="center" justifyContent="space-between">
      <FieldLabel label={label} tooltip={tooltip} />
      <Switch name={String(name)} checked={checked} onChange={onChange} />
    </Box>
    {description ? (
      <Typography variant="caption" color="text.secondary">
        {description}
      </Typography>
    ) : null}
  </Paper>
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
  const [activeTab, setActiveTab] = useState<TabKey>('general')

  const buildErrors = useCallback((state: SettingsForm) => {
    const e: Record<string, string> = {}

    const cb = state.CHANGED_BLOCKS_COPY_ITERATION_THRESHOLD
    if (!Number.isInteger(cb) || cb < 1 || cb > 20) {
      e.CHANGED_BLOCKS_COPY_ITERATION_THRESHOLD = 'Enter an integer between 1 and 20 (inclusive).'
    }

    const intervalStr = (state.PERIODIC_SYNC_INTERVAL ?? '').trim()
    const intervalError = parseInterval(intervalStr)
    if (intervalError) {
      e.PERIODIC_SYNC_INTERVAL = intervalError
    }

    if (state.DEFAULT_MIGRATION_METHOD !== 'hot' && state.DEFAULT_MIGRATION_METHOD !== 'cold') {
      e.DEFAULT_MIGRATION_METHOD = "Must be 'hot' or 'cold'."
    }

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

    const loginRetry = state.VCENTER_LOGIN_RETRY_LIMIT
    if (!Number.isFinite(loginRetry) || !Number.isInteger(loginRetry) || loginRetry < 0) {
      e.VCENTER_LOGIN_RETRY_LIMIT = 'Enter an integer >= 0.'
    }

    const bools: Array<keyof SettingsForm> = [
      'CLEANUP_VOLUMES_AFTER_CONVERT_FAILURE',
      'CLEANUP_PORTS_AFTER_MIGRATION_FAILURE',
      'POPULATE_VMWARE_MACHINE_FLAVORS',
      'VALIDATE_RDM_OWNER_VMS',
      'AUTO_FSTAB_UPDATE'
    ]
    bools.forEach((k) => {
      const val = state[k]
      if (typeof val !== 'boolean') {
        e[String(k)] = 'Must be boolean: true or false.'
      }
    })

    const dn = (state.DEPLOYMENT_NAME ?? '').trim()
    if (!dn) {
      e.DEPLOYMENT_NAME = 'Required.'
    } else if (dn.length > 63) {
      e.DEPLOYMENT_NAME = 'Must be 63 characters or fewer.'
    }

    return e
  }, [])

  const validateForm = useCallback(
    (state: SettingsForm) => {
      const nextErrors = buildErrors(state)
      setErrors(nextErrors)
      return Object.keys(nextErrors).length === 0
    },
    [buildErrors]
  )

  const updateForm = useCallback(
    (updater: SettingsForm | FormUpdater) => {
      setForm((prev) => {
        const next = typeof updater === 'function' ? (updater as FormUpdater)(prev) : updater
        setErrors(buildErrors(next))
        return next
      })
    },
    [buildErrors]
  )

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
      updateForm(next ?? DEFAULTS)
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

  const tabProps = (value: TabKey) => ({
    id: `settings-tab-${value}`,
    'aria-controls': `settings-tabpanel-${value}`
  })

  const handleTabChange = useCallback((_: SyntheticEvent, value: string | number) => {
    setActiveTab(value as TabKey)
  }, [])

  /* ------------------------
     Event handlers
     -----------------------*/
  const onText = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const { name, value } = e.target
      updateForm((prev) => ({ ...prev, [name]: value }))
    },
    [updateForm]
  )

  const onNumber = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const { name, value } = e.target
      const n = value === '' ? ('' as unknown as number) : Number(value)
      updateForm((prev) => ({ ...prev, [name]: n }))
    },
    [updateForm]
  )

  const onBool = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const { name, checked } = e.target
      updateForm((prev) => ({ ...prev, [name]: checked }))
    },
    [updateForm]
  )

  const onSelect = useCallback(
    (e: SelectChangeEvent<string>) => {
      const { name, value } = e.target
      updateForm((prev) => ({ ...prev, [name]: value as 'hot' | 'cold' }))
    },
    [updateForm]
  )

  const onResetDefaults = useCallback(() => {
    updateForm(DEFAULTS)
    setErrors({})
  }, [updateForm])

  const onCancel = useCallback(() => {
    updateForm(initial)
    setErrors({})
  }, [initial, updateForm])

  const onSave = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault()
      if (!validateForm(form)) {
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
    [form, validateForm, show]
  )

  const numberError = useCallback((key: keyof SettingsForm) => errors[String(key)], [errors])

  const tabHasError = useCallback(
    (tab: TabKey) => TAB_FIELD_KEYS[tab].some((key) => Boolean(errors[String(key)])),
    [errors]
  )

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
        <Tabs
          value={activeTab}
          onChange={handleTabChange}
          variant="scrollable"
          allowScrollButtonsMobile
          sx={{ borderBottom: (theme) => `1px solid ${theme.palette.divider}` }}
        >
          {TAB_ORDER.map((tab) => (
            <Tab
              key={tab}
              value={tab}
              label={
                <TabLabel
                  label={TAB_META[tab].label}
                  showError={tabHasError(tab)}
                  icon={TAB_META[tab].icon}
                />
              }
              {...tabProps(tab)}
            />
          ))}
        </Tabs>

        <Typography variant="body2" color="text.secondary" sx={{ mt: 1 }}>
          {TAB_META[activeTab].helper}
        </Typography>

        <TabPanel current={activeTab} value="general">
          <SettingsGrid>
            <CustomTextField
              label="Deployment Name"
              name="DEPLOYMENT_NAME"
              value={form.DEPLOYMENT_NAME}
              onChange={onText}
              error={errors.DEPLOYMENT_NAME}
              tooltip={FIELD_TOOLTIPS.DEPLOYMENT_NAME}
            />

            <NumberField
              label="Changed Blocks Copy Iteration Threshold"
              name="CHANGED_BLOCKS_COPY_ITERATION_THRESHOLD"
              value={form.CHANGED_BLOCKS_COPY_ITERATION_THRESHOLD}
              onChange={onNumber}
              error={numberError('CHANGED_BLOCKS_COPY_ITERATION_THRESHOLD')}
              tooltip={FIELD_TOOLTIPS.CHANGED_BLOCKS_COPY_ITERATION_THRESHOLD}
            />

            <IntervalField
              label="Periodic Sync"
              name="PERIODIC_SYNC_INTERVAL"
              value={form.PERIODIC_SYNC_INTERVAL}
              onChange={onText}
              error={errors.PERIODIC_SYNC_INTERVAL}
              tooltip={FIELD_TOOLTIPS.PERIODIC_SYNC_INTERVAL}
            />
          </SettingsGrid>
        </TabPanel>

        <TabPanel current={activeTab} value="retry">
          <SettingsGrid>
            <NumberField
              label="VM Active Wait Interval (seconds)"
              name="VM_ACTIVE_WAIT_INTERVAL_SECONDS"
              value={form.VM_ACTIVE_WAIT_INTERVAL_SECONDS}
              onChange={onNumber}
              error={numberError('VM_ACTIVE_WAIT_INTERVAL_SECONDS')}
              tooltip={FIELD_TOOLTIPS.VM_ACTIVE_WAIT_INTERVAL_SECONDS}
            />

            <NumberField
              label="VM Active Retry Limit"
              name="VM_ACTIVE_WAIT_RETRY_LIMIT"
              value={form.VM_ACTIVE_WAIT_RETRY_LIMIT}
              onChange={onNumber}
              error={numberError('VM_ACTIVE_WAIT_RETRY_LIMIT')}
              tooltip={FIELD_TOOLTIPS.VM_ACTIVE_WAIT_RETRY_LIMIT}
            />

            <NumberField
              label="Volume Wait Interval (seconds)"
              name="VOLUME_AVAILABLE_WAIT_INTERVAL_SECONDS"
              value={form.VOLUME_AVAILABLE_WAIT_INTERVAL_SECONDS}
              onChange={onNumber}
              error={numberError('VOLUME_AVAILABLE_WAIT_INTERVAL_SECONDS')}
              tooltip={FIELD_TOOLTIPS.VOLUME_AVAILABLE_WAIT_INTERVAL_SECONDS}
            />

            <NumberField
              label="Volume Retry Limit"
              name="VOLUME_AVAILABLE_WAIT_RETRY_LIMIT"
              value={form.VOLUME_AVAILABLE_WAIT_RETRY_LIMIT}
              onChange={onNumber}
              error={numberError('VOLUME_AVAILABLE_WAIT_RETRY_LIMIT')}
              tooltip={FIELD_TOOLTIPS.VOLUME_AVAILABLE_WAIT_RETRY_LIMIT}
            />

            <NumberField
              label="vCenter Login Retry Limit"
              name="VCENTER_LOGIN_RETRY_LIMIT"
              value={form.VCENTER_LOGIN_RETRY_LIMIT}
              onChange={onNumber}
              error={numberError('VCENTER_LOGIN_RETRY_LIMIT')}
              tooltip={FIELD_TOOLTIPS.VCENTER_LOGIN_RETRY_LIMIT}
            />

            <NumberField
              label="vCenter Concurrency Limit"
              name="VCENTER_SCAN_CONCURRENCY_LIMIT"
              value={form.VCENTER_SCAN_CONCURRENCY_LIMIT}
              onChange={onNumber}
              error={numberError('VCENTER_SCAN_CONCURRENCY_LIMIT')}
              tooltip={FIELD_TOOLTIPS.VCENTER_SCAN_CONCURRENCY_LIMIT}
            />
          </SettingsGrid>
        </TabPanel>

        <TabPanel current={activeTab} value="advanced">
          <SettingsGrid>
            <NumberField
              label="OpenStack Creds Requeue After (minutes)"
              name="OPENSTACK_CREDS_REQUEUE_AFTER_MINUTES"
              value={form.OPENSTACK_CREDS_REQUEUE_AFTER_MINUTES}
              onChange={onNumber}
              error={numberError('OPENSTACK_CREDS_REQUEUE_AFTER_MINUTES')}
              tooltip={FIELD_TOOLTIPS.OPENSTACK_CREDS_REQUEUE_AFTER_MINUTES}
            />

            <NumberField
              label="VMware Creds Requeue After (minutes)"
              name="VMWARE_CREDS_REQUEUE_AFTER_MINUTES"
              value={form.VMWARE_CREDS_REQUEUE_AFTER_MINUTES}
              onChange={onNumber}
              error={numberError('VMWARE_CREDS_REQUEUE_AFTER_MINUTES')}
              tooltip={FIELD_TOOLTIPS.VMWARE_CREDS_REQUEUE_AFTER_MINUTES}
            />

            <FormControl
              fullWidth
              size="small"
              error={!!errors.DEFAULT_MIGRATION_METHOD}
              sx={{ display: 'flex', flexDirection: 'column', gap: 0.5 }}
            >
              <FieldLabel
                label="Default Migration Method"
                tooltip={FIELD_TOOLTIPS.DEFAULT_MIGRATION_METHOD}
              />
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
            </FormControl>
          </SettingsGrid>

          <Typography variant="subtitle2" sx={{ mt: 3, mb: 1 }}>
            Automation Flags
          </Typography>
          <Box
            sx={{
              display: 'grid',
              gridTemplateColumns: { xs: '1fr', sm: 'repeat(auto-fit, minmax(260px, 1fr))' },
              gap: 2
            }}
          >
            {TOGGLE_FIELDS.map(({ key, label, description }) => (
              <ToggleField
                key={key}
                label={label}
                name={key}
                checked={form[key] as boolean}
                onChange={onBool}
                tooltip={FIELD_TOOLTIPS[key]}
                description={description}
              />
            ))}
          </Box>
        </TabPanel>

        <TabPanel current={activeTab} value="vddk">
          <VDDKUpload />
        </TabPanel>

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
