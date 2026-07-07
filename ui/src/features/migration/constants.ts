export enum CUTOVER_TYPES {
  'IMMEDIATE' = '0',
  'ADMIN_INITIATED' = '1',
  'TIME_WINDOW' = '2'
}

export enum OS_TYPES {
  'AUTO_DETECT' = 'default',
  'WINDOWS' = 'windowsGuest',
  'LINUX' = 'linuxGuest'
}

export const DATA_COPY_OPTIONS = [
  { value: 'cold', label: 'Power off live VMs, then copy' },
  { value: 'hot', label: 'Copy live VMs, then power off' },
  { value: 'mock', label: 'Do not Turn off the source VM'}
]

export const OS_TYPES_OPTIONS = [
  { value: OS_TYPES.AUTO_DETECT, label: 'Auto-detect' },
  { value: OS_TYPES.WINDOWS, label: 'Windows' },
  { value: OS_TYPES.LINUX, label: 'Linux' }
]

export const VM_CUTOVER_OPTIONS = [
  {
    value: CUTOVER_TYPES.IMMEDIATE,
    label: 'Cutover immediately after data copy'
  },
  { value: CUTOVER_TYPES.ADMIN_INITIATED, label: 'Admin initiated cutover' },
  { value: CUTOVER_TYPES.TIME_WINDOW, label: 'Cutover during time window' }
]

// ---------------------------------------------------------------------------
// NetworkAndStorageMappingStep constants
// ---------------------------------------------------------------------------

export const STORAGE_COPY_METHOD_OPTIONS = [
  { value: 'normal', label: 'Standard Copy' },
  { value: 'StorageAcceleratedCopy', label: 'Storage Accelerated Copy' },
  { value: 'HotAdd', label: 'vJailbreak Accelerated Copy' }
] as const

// ---------------------------------------------------------------------------
// MigrationsTable constants
// ---------------------------------------------------------------------------

export const STATUS_ORDER: Record<string, number> = {
  Running: 0,
  Failed: 1,
  Succeeded: 2,
  Pending: 3
}

// ---------------------------------------------------------------------------
// MigrationForm / RollingMigrationForm defaults
// ---------------------------------------------------------------------------

export const DEFAULT_MIGRATION_OPTIONS = {
  dataCopyMethod: false,
  dataCopyStartTime: false,
  cutoverOption: false,
  cutoverStartTime: false,
  cutoverEndTime: false,
  postMigrationScript: false,
  useGPU: false,
  postMigrationAction: {
    suffix: false,
    folderName: false,
    renameVm: false,
    moveToFolder: false
  }
}

export const DRAWER_WIDTH = 1400

// ---------------------------------------------------------------------------
// VmsSelectionStep constants
// ---------------------------------------------------------------------------

export const MIGRATED_TOOLTIP_MESSAGE = 'This VM is migrating or already has been migrated.'
export const FLAVOR_NOT_FOUND_MESSAGE =
  'Appropriate flavor not found. Please assign a flavor before selecting this VM for migration or create a flavor.'
export const DEFAULT_PAGINATION_MODEL = { page: 0, pageSize: 5 }

// ---------------------------------------------------------------------------
// MigrationOptionsAlt constants
// ---------------------------------------------------------------------------

export const NEXT_SCRIPT_DELIMITER = '### NEXT SCRIPT ###'
