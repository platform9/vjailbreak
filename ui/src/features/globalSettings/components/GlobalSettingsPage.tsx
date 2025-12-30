import React, { SyntheticEvent, useCallback, useEffect, useMemo, useState } from 'react'
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  FormControl,
  FormHelperText,
  MenuItem,
  Select,
  SelectChangeEvent,
  Snackbar,
  Tab,
  Tabs,
  TextField,
  Typography,
  styled,
  useTheme
} from '@mui/material'
import type { SnackbarCloseReason } from '@mui/material'
import RefreshIcon from '@mui/icons-material/Refresh'
import SettingsOutlinedIcon from '@mui/icons-material/SettingsOutlined'
import HistoryToggleOffOutlinedIcon from '@mui/icons-material/HistoryToggleOffOutlined'
import TuneOutlinedIcon from '@mui/icons-material/TuneOutlined'
import LanOutlinedIcon from '@mui/icons-material/LanOutlined'
import FieldLabel from 'src/components/design-system/ui/FieldLabel'
import FormGrid from 'src/components/design-system/ui/FormGrid'
import ToggleField from 'src/components/design-system/ui/ToggleField'
import { IntervalField as SharedIntervalField } from 'src/shared/components/forms'
import { getGlobalSettingsHelpers } from 'src/features/globalSettings/helpers'
import {
  getSettingsConfigMap,
  updateSettingsConfigMap,
  VERSION_CONFIG_MAP_NAME,
  VERSION_NAMESPACE
} from 'src/api/settings/settings'
import { getPf9EnvConfig, injectEnvVariables } from 'src/api/helpers'

const StyledPaper = styled(Box)(({ theme }) => ({
  width: '100%',
  height: '100%',
  minHeight: 0,
  padding: theme.spacing(4),
  boxSizing: 'border-box',
  display: 'flex',
  flexDirection: 'column'
}))

const Footer = styled(Box)(({ theme }) => ({
  display: 'flex',
  justifyContent: 'flex-end',
  gap: theme.spacing(2),
  marginTop: theme.spacing(3),
  paddingTop: theme.spacing(2),
  borderTop: `1px solid ${theme.palette.divider}`
}))

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
  // Proxy-related fields are UI-only and handled via injectEnvVariables
  PROXY_ENABLED: boolean
  PROXY_HTTP_HOST: string
  PROXY_HTTP_PORT: string
  PROXY_HTTPS_HOST: string
  PROXY_HTTPS_PORT: string
  NO_PROXY: string
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
  DEPLOYMENT_NAME: 'vJailbreak',
  PROXY_ENABLED: false,
  PROXY_HTTP_HOST: '',
  PROXY_HTTP_PORT: '',
  PROXY_HTTPS_HOST: '',
  PROXY_HTTPS_PORT: '',
  NO_PROXY: 'localhost,127.0.0.1'
}

const helpers = getGlobalSettingsHelpers(DEFAULTS)

type FormUpdater = (prev: SettingsForm) => SettingsForm
type TabKey = 'general' | 'retry' | 'network' | 'advanced'

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
  network: [],
  advanced: [
    'OPENSTACK_CREDS_REQUEUE_AFTER_MINUTES',
    'VMWARE_CREDS_REQUEUE_AFTER_MINUTES',
    'DEFAULT_MIGRATION_METHOD',
    'CLEANUP_VOLUMES_AFTER_CONVERT_FAILURE',
    'CLEANUP_PORTS_AFTER_MIGRATION_FAILURE',
    'POPULATE_VMWARE_MACHINE_FLAVORS',
    'VALIDATE_RDM_OWNER_VMS',
    'AUTO_FSTAB_UPDATE'
  ]
}

const TAB_ORDER: TabKey[] = ['general', 'retry', 'network', 'advanced']

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
  network: {
    label: 'Network',
    helper: 'Configure proxy used by the migration system components.',
    icon: <LanOutlinedIcon fontSize="small" />
  },
  advanced: {
    label: 'Advanced',
    helper: 'Tune integration defaults and automation flags for PCD and VMware flows.',
    icon: <TuneOutlinedIcon fontSize="small" />
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
  activeTab,
  value
}: {
  children: React.ReactNode
  activeTab: TabKey
  value: TabKey
}) => {
  if (activeTab !== value) return null
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
    'Time before failed PCD credentials are re-queued for another attempt.',
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
  AUTO_FSTAB_UPDATE: 'Automatically update fstab entries during VM migration.',
  PROXY_ENABLED: 'Turn on to route outbound HTTP/HTTPS traffic via the configured proxy.',
  PROXY_HTTP_HOST:
    'FQDN or IP of the HTTP proxy server (e.g. proxy.example.com). Do not include http://.',
  PROXY_HTTP_PORT: 'TCP port of the HTTP proxy server (e.g. 3128).',
  PROXY_HTTPS_HOST:
    'FQDN or IP of the HTTPS proxy server (e.g. proxy.example.com). Do not include https://.',
  PROXY_HTTPS_PORT: 'TCP port of the HTTPS proxy server (e.g. 3129).',
  NO_PROXY:
    'Comma-separated hosts or CIDRs that should bypass the proxy (e.g. localhost,127.0.0.1).'
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
    description:
      'Automatically update fstab entries to ensure proper disk mounting after migration.'
  }
]

type NotificationSeverity = 'error' | 'info' | 'success' | 'warning'

type NotificationState = {
  open: boolean
  message: string
  severity: NotificationSeverity
}

type FieldErrorMap = Record<string, string>

const DEFAULT_NOTIFICATION: NotificationState = {
  open: false,
  message: '',
  severity: 'info'
}

const EMPTY_ERRORS: FieldErrorMap = {}

const { parseInterval, validateProxyUrl, deriveProxyState, applyProxyState } = helpers
const { toConfigMapData, fromConfigMapData, buildEnvPayload } = helpers

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

type UseGlobalSettingsControllerReturn = {
  form: SettingsForm
  errors: FieldErrorMap
  loading: boolean
  saving: boolean
  activeTab: TabKey
  notification: NotificationState
  onText: (e: React.ChangeEvent<HTMLInputElement>) => void
  onNumber: (e: React.ChangeEvent<HTMLInputElement>) => void
  onBool: (e: React.ChangeEvent<HTMLInputElement>) => void
  onSelect: (e: SelectChangeEvent<string>) => void
  numberError: (key: keyof SettingsForm) => string | undefined
  tabHasError: (tab: TabKey) => boolean
  handleTabChange: (_: SyntheticEvent, value: string | number) => void
  onResetDefaults: () => void
  onCancel: () => void
  onSave: (e: React.FormEvent) => Promise<void>
  handleNotificationClose: (_: SyntheticEvent | Event, reason?: SnackbarCloseReason) => void
}

const useGlobalSettingsController = (): UseGlobalSettingsControllerReturn => {
  const [form, setForm] = useState<SettingsForm>(DEFAULTS)
  const [initial, setInitial] = useState<SettingsForm>(DEFAULTS)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [errors, setErrors] = useState<FieldErrorMap>(EMPTY_ERRORS)
  const [activeTab, setActiveTab] = useState<TabKey>('general')
  const [notification, setNotification] = useState<NotificationState>(DEFAULT_NOTIFICATION)

  const buildErrors = useCallback((state: SettingsForm): FieldErrorMap => {
    const e: FieldErrorMap = {}

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

    const proxyEnabled = state.PROXY_ENABLED
    const httpHost = (state.PROXY_HTTP_HOST ?? '').trim()
    const httpPort = (state.PROXY_HTTP_PORT ?? '').trim()
    const httpsHost = (state.PROXY_HTTPS_HOST ?? '').trim()
    const httpsPort = (state.PROXY_HTTPS_PORT ?? '').trim()

    const fqdnRegex =
      /^(?:[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)(?:\.(?:[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?))+$/
    const ipv4Regex =
      /^(?:25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(?:\.(?:25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}$/

    const isValidHost = (value: string) => fqdnRegex.test(value) || ipv4Regex.test(value)

    if (proxyEnabled) {
      const validateHostPort = (
        hostKey: keyof SettingsForm,
        portKey: keyof SettingsForm,
        hostVal: string,
        portVal: string,
        scheme: 'http' | 'https'
      ) => {
        if (!hostVal && !portVal) {
          return
        }

        if (!hostVal) {
          e[String(hostKey)] =
            `${scheme.toUpperCase()} proxy server is required when proxy is enabled.`
        } else if (/^https?:\/\//i.test(hostVal)) {
          e[String(hostKey)] =
            scheme === 'http'
              ? 'Enter only the FQDN or IPv4 address for HTTP proxy (without http://).'
              : 'Enter only the FQDN or IPv4 address for HTTPS proxy (without https://).'
        } else if (!isValidHost(hostVal)) {
          e[String(hostKey)] = 'Enter a valid FQDN (with at least one dot) or IPv4 address.'
        }

        if (!portVal) {
          e[String(portKey)] =
            `${scheme.toUpperCase()} proxy port is required when proxy is enabled.`
        } else {
          const portNum = Number(portVal)
          if (!Number.isInteger(portNum) || portNum < 1 || portNum > 65535) {
            e[String(portKey)] = 'Enter a valid TCP port between 1 and 65535.'
          }
        }

        if (!e[String(hostKey)] && !e[String(portKey)]) {
          const proxyUrl = `${scheme}://${hostVal}:${portVal}`
          const proxyError = validateProxyUrl(proxyUrl)
          if (proxyError) {
            e[String(hostKey)] = proxyError
          }
        }
      }

      validateHostPort('PROXY_HTTP_HOST', 'PROXY_HTTP_PORT', httpHost, httpPort, 'http')
      validateHostPort('PROXY_HTTPS_HOST', 'PROXY_HTTPS_PORT', httpsHost, httpsPort, 'https')
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

  const fetchSettings = useCallback(async () => {
    setLoading(true)
    try {
      const [settingsCm, pf9Env] = await Promise.all([
        getSettingsConfigMap(),
        getPf9EnvConfig().catch((err) => {
          console.error('Failed to fetch pf9-env config:', err)
          return undefined
        })
      ])

      const base = fromConfigMapData(
        settingsCm?.data as Record<string, string | number | undefined> | undefined
      )
      const proxyState = deriveProxyState(base, pf9Env?.data)
      const merged = applyProxyState(base, proxyState)

      setForm(merged)
      setInitial(merged)
      setErrors(buildErrors(merged))
    } catch (err) {
      console.error('Failed to load Global Settings:', err)
    } finally {
      setLoading(false)
    }
  }, [buildErrors])

  useEffect(() => {
    fetchSettings()
  }, [fetchSettings])

  const show = useCallback((message: string, severity: NotificationSeverity = 'info') => {
    setNotification({ open: true, message, severity })
  }, [])

  const handleNotificationClose = useCallback(
    (_: SyntheticEvent | Event, reason?: SnackbarCloseReason) => {
      if (reason === 'clickaway') return
      setNotification((prev) => ({ ...prev, open: false }))
    },
    []
  )

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
    updateForm({ ...DEFAULTS })
  }, [updateForm])

  const onCancel = useCallback(() => {
    updateForm({ ...initial })
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
        await updateSettingsConfigMap({
          apiVersion: 'v1',
          kind: 'ConfigMap',
          metadata: { name: VERSION_CONFIG_MAP_NAME, namespace: VERSION_NAMESPACE },
          data: toConfigMapData(form)
        } as any)

        let envInjectionFailed = false

        try {
          await injectEnvVariables(buildEnvPayload(form))
        } catch (envErr) {
          envInjectionFailed = true
          console.error('Failed to inject proxy env variables:', envErr)
        }

        setInitial(form)
        setErrors(buildErrors(form))

        if (envInjectionFailed) {
          show(
            'Settings saved, but applying proxy environment variables failed. Please verify connectivity and try again.',
            'warning'
          )
        } else {
          show('Global Settings saved successfully.', 'success')
        }
      } catch (err) {
        console.error('Failed to save Global Settings ConfigMap:', err)
        show('Failed to save Global Settings. No changes were applied.', 'error')
      } finally {
        setSaving(false)
      }
    },
    [form, validateForm, show, buildErrors]
  )

  const numberError = useCallback((key: keyof SettingsForm) => errors[String(key)], [errors])

  const tabErrorFlags = useMemo(
    () =>
      TAB_ORDER.reduce<Record<TabKey, boolean>>(
        (acc, tab) => {
          acc[tab] = TAB_FIELD_KEYS[tab].some((key) => Boolean(errors[String(key)]))
          return acc
        },
        {} as Record<TabKey, boolean>
      ),
    [errors]
  )

  const tabHasError = useCallback((tab: TabKey) => tabErrorFlags[tab], [tabErrorFlags])

  const handleTabChange = useCallback((_: SyntheticEvent, value: string | number) => {
    setActiveTab(value as TabKey)
  }, [])

  return {
    form,
    errors,
    loading,
    saving,
    activeTab,
    notification,
    onText,
    onNumber,
    onBool,
    onSelect,
    numberError,
    tabHasError,
    handleTabChange,
    onResetDefaults,
    onCancel,
    onSave,
    handleNotificationClose
  }
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
      data-testid={`global-settings-field-${String(name)}`}
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
  required?: boolean
}

const CustomTextField = ({
  label,
  name,
  value,
  helper,
  error,
  onChange,
  tooltip,
  required
}: TextFieldProps) => (
  <Box display="flex" flexDirection="column" gap={0.5}>
    <FieldLabel label={label} tooltip={tooltip} required={required} />
    <TextField
      fullWidth
      size="small"
      name={String(name)}
      value={value}
      onChange={onChange}
      error={!!error}
      helperText={error || helper}
      data-testid={`global-settings-field-${String(name)}`}
    />
  </Box>
)

export default function GlobalSettingsPage() {
  const theme = useTheme()
  const {
    form,
    errors,
    loading,
    saving,
    activeTab,
    notification,
    onText,
    onNumber,
    onBool,
    onSelect,
    numberError,
    tabHasError,
    handleTabChange,
    onResetDefaults,
    onCancel,
    onSave,
    handleNotificationClose
  } = useGlobalSettingsController()

  const tabProps = (value: TabKey) => ({
    id: `settings-tab-${value}`,
    'aria-controls': `settings-tabpanel-${value}`
  })

  if (loading) {
    return (
      <StyledPaper>
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
    <StyledPaper>
      <Box
        component="form"
        onSubmit={onSave}
        data-testid="global-settings-form"
        sx={{
          display: 'flex',
          flexDirection: 'column',
          flex: 1,
          minHeight: 0
        }}
      >
        <Tabs
          value={activeTab}
          onChange={handleTabChange}
          variant="scrollable"
          allowScrollButtonsMobile
          data-testid="global-settings-tabs"
          sx={{ borderBottom: (theme) => `1px solid ${theme.palette.divider}` }}
        >
          {TAB_ORDER.map((tab) => (
            <Tab
              key={tab}
              value={tab}
              data-testid={`global-settings-tab-${tab}`}
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

        <TabPanel activeTab={activeTab} value="general">
          <FormGrid minWidth={320} gap={2}>
            <CustomTextField
              label="Deployment Name"
              name="DEPLOYMENT_NAME"
              value={form.DEPLOYMENT_NAME}
              onChange={onText}
              error={errors.DEPLOYMENT_NAME}
              tooltip={FIELD_TOOLTIPS.DEPLOYMENT_NAME}
              required
            />

            <NumberField
              label="Changed Blocks Copy Iteration Threshold"
              name="CHANGED_BLOCKS_COPY_ITERATION_THRESHOLD"
              value={form.CHANGED_BLOCKS_COPY_ITERATION_THRESHOLD}
              onChange={onNumber}
              error={numberError('CHANGED_BLOCKS_COPY_ITERATION_THRESHOLD')}
              tooltip={FIELD_TOOLTIPS.CHANGED_BLOCKS_COPY_ITERATION_THRESHOLD}
            />

            <Box display="flex" flexDirection="column" gap={0.5}>
              <FieldLabel label="Periodic Sync" tooltip={FIELD_TOOLTIPS.PERIODIC_SYNC_INTERVAL} />
              <SharedIntervalField
                label=""
                name="PERIODIC_SYNC_INTERVAL"
                value={form.PERIODIC_SYNC_INTERVAL}
                onChange={onText}
                error={errors.PERIODIC_SYNC_INTERVAL}
              />
            </Box>
          </FormGrid>
        </TabPanel>

        <TabPanel activeTab={activeTab} value="network">
          <FormGrid minWidth={320} gap={2} sx={{ mb: 2 }}>
            <ToggleField
              label="Use Proxy"
              name="PROXY_ENABLED"
              checked={form.PROXY_ENABLED}
              onChange={onBool}
              tooltip={FIELD_TOOLTIPS.PROXY_ENABLED}
              description="Route outbound HTTP/HTTPS traffic via the configured proxy."
              data-testid="global-settings-toggle-PROXY_ENABLED"
            />
          </FormGrid>
          {form.PROXY_ENABLED && (
            <>
              <Box sx={{ mb: 2 }}>
                <FormGrid minWidth={320} gap={2}>
                  <CustomTextField
                    label="HTTP Proxy Server"
                    name="PROXY_HTTP_HOST"
                    value={form.PROXY_HTTP_HOST}
                    onChange={onText}
                    error={errors.PROXY_HTTP_HOST}
                    tooltip={FIELD_TOOLTIPS.PROXY_HTTP_HOST}
                    required
                  />

                  <CustomTextField
                    label="HTTP Proxy Port"
                    name="PROXY_HTTP_PORT"
                    value={form.PROXY_HTTP_PORT}
                    onChange={onText}
                    error={errors.PROXY_HTTP_PORT}
                    tooltip={FIELD_TOOLTIPS.PROXY_HTTP_PORT}
                    required
                  />
                </FormGrid>
              </Box>

              <Box sx={{ mb: 2 }}>
                <FormGrid minWidth={320} gap={2}>
                  <CustomTextField
                    label="HTTPS Proxy Server"
                    name="PROXY_HTTPS_HOST"
                    value={form.PROXY_HTTPS_HOST}
                    onChange={onText}
                    error={errors.PROXY_HTTPS_HOST}
                    tooltip={FIELD_TOOLTIPS.PROXY_HTTPS_HOST}
                    required
                  />

                  <CustomTextField
                    label="HTTPS Proxy Port"
                    name="PROXY_HTTPS_PORT"
                    value={form.PROXY_HTTPS_PORT}
                    onChange={onText}
                    error={errors.PROXY_HTTPS_PORT}
                    tooltip={FIELD_TOOLTIPS.PROXY_HTTPS_PORT}
                    required
                  />
                </FormGrid>
              </Box>

              <FormGrid minWidth={320} gap={2}>
                <CustomTextField
                  label="No Proxy Hosts"
                  name="NO_PROXY"
                  value={form.NO_PROXY}
                  onChange={onText}
                  error={errors.NO_PROXY}
                  tooltip={FIELD_TOOLTIPS.NO_PROXY}
                />
              </FormGrid>
            </>
          )}
        </TabPanel>

        <TabPanel activeTab={activeTab} value="retry">
          <FormGrid minWidth={320} gap={2}>
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
          </FormGrid>
        </TabPanel>

        <TabPanel activeTab={activeTab} value="advanced">
          <FormGrid minWidth={320} gap={2}>
            <NumberField
              label="PCD Creds Requeue After (minutes)"
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
                data-testid="global-settings-field-DEFAULT_MIGRATION_METHOD"
              >
                <MenuItem value="hot">hot</MenuItem>
                <MenuItem value="cold">cold</MenuItem>
              </Select>
              {errors.DEFAULT_MIGRATION_METHOD && (
                <FormHelperText>{errors.DEFAULT_MIGRATION_METHOD}</FormHelperText>
              )}
            </FormControl>
          </FormGrid>

          <Typography variant="subtitle2" sx={{ mt: 3, mb: 1 }}>
            Automation Flags
          </Typography>
          <FormGrid minWidth={260} gap={2}>
            {TOGGLE_FIELDS.map(({ key, label, description }) => (
              <ToggleField
                key={key}
                label={label}
                name={String(key)}
                checked={form[key] as boolean}
                onChange={onBool}
                tooltip={FIELD_TOOLTIPS[key]}
                description={description}
                data-testid={`global-settings-toggle-${String(key)}`}
              />
            ))}
          </FormGrid>
        </TabPanel>

        <Box sx={{ flexGrow: 1 }} />

        <Footer sx={{ marginTop: 'auto', marginBottom: theme.spacing(3) }}>
          <Button
            variant="outlined"
            color="inherit"
            onClick={onResetDefaults}
            startIcon={<RefreshIcon />}
            data-testid="global-settings-reset-defaults"
          >
            Reset to Defaults
          </Button>
          <Button variant="outlined" onClick={onCancel} data-testid="global-settings-cancel">
            Cancel
          </Button>
          <Button
            variant="contained"
            type="submit"
            color="primary"
            disabled={saving}
            startIcon={saving ? <CircularProgress size={20} color="inherit" /> : null}
            data-testid="global-settings-save"
          >
            {saving ? 'Saving...' : 'Save'}
          </Button>
        </Footer>
      </Box>

      <Snackbar
        open={notification.open}
        autoHideDuration={6000}
        onClose={handleNotificationClose}
        anchorOrigin={{ vertical: 'top', horizontal: 'right' }}
        sx={{
          top: { xs: 80, sm: 90 },
          right: { xs: 16, sm: 24 }
        }}
      >
        <Alert
          onClose={handleNotificationClose}
          severity={notification.severity}
          variant="filled"
          sx={{
            minWidth: '350px',
            fontSize: '1rem',
            fontWeight: 600,
            boxShadow: '0 8px 24px rgba(0, 0, 0, 0.25)',
            '& .MuiAlert-icon': {
              fontSize: '28px'
            },
            '& .MuiAlert-message': {
              fontSize: '1rem',
              fontWeight: 600
            }
          }}
        >
          {notification.message}
        </Alert>
      </Snackbar>
    </StyledPaper>
  )
}
