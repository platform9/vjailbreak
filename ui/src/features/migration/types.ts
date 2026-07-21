import type { Dispatch, SetStateAction } from 'react'
import { VmData } from './api/migration-templates/model'
import type { RetryMigrationConfig } from './context/MigrationFormContext'
import type { SavedTemplate } from './api/migration-blueprints/types'
import { OpenStackFlavor, OpenstackCreds, PCDNetworkInfo } from 'src/api/openstack-creds/model'
import type { VMwareDiskEntry } from 'src/api/vmware-machines/model'
import { Migration } from './api/migrations'
import { RefetchOptions, QueryObserverResult } from '@tanstack/react-query'
import type { GridRowSelectionModel, GridToolbarProps } from '@mui/x-data-grid'
import type { ErrorContext } from 'src/services/errorReporting'

// ---------------------------------------------------------------------------
// MigrationForm types
// ---------------------------------------------------------------------------

export interface FormValues extends Record<string, unknown> {
  vmwareCreds?: {
    vcenterHost: string
    datacenter: string
    username: string
    password: string
    existingCredName?: string
    credentialName?: string
  }
  openstackCreds?: {
    OS_AUTH_URL: string
    OS_DOMAIN_NAME: string
    OS_USERNAME: string
    OS_PASSWORD: string
    OS_REGION_NAME: string
    OS_TENANT_NAME: string
    existingCredName?: string
    credentialName?: string
    OS_INSECURE?: boolean
  }
  vms?: VmData[]
  rdmConfigurations?: Array<{
    uuid: string
    diskName: string
    cinderBackendPool: string
    volumeType: string
    source: Record<string, string>
  }>
  networkMappings?: { source: string; target: string }[]
  storageMappings?: { source: string; target: string }[]
  arrayCredsMappings?: { source: string; target: string }[]
  storageCopyMethod?: StorageCopyMethod
  proxyVMRef?: string
  // Cluster selection fields
  vmwareCluster?: string // Format: "credName:datacenter:clusterName"
  pcdCluster?: string // PCD cluster ID
  // Optional Params
  dataCopyMethod?: string
  dataCopyStartTime?: string
  cutoverOption?: string
  cutoverStartTime?: string
  cutoverEndTime?: string
  postMigrationScript?: string
  osFamily?: string
  // Add postMigrationAction with optional properties
  postMigrationAction?: {
    suffix?: string
    folderName?: string
    renameVm?: boolean
    moveToFolder?: boolean
  }
  disconnectSourceNetwork?: boolean
  securityGroups?: string[]
  serverGroup?: string
  fallbackToDHCP?: boolean
  useGPU?: boolean
  networkPersistence?: boolean
  removeVMwareTools?: boolean
  imageProfiles?: string[]
  periodicSyncInterval?: string
  acknowledgeNetworkConflictRisk?: boolean
}

export interface SelectedMigrationOptionsType {
  dataCopyMethod: boolean
  dataCopyStartTime: boolean
  cutoverOption: boolean
  cutoverStartTime: boolean
  cutoverEndTime: boolean
  postMigrationScript: boolean
  useGPU?: boolean
  periodicSyncEnabled?: boolean
  postMigrationAction?: {
    suffix?: boolean
    folderName?: boolean
    renameVm?: boolean
    moveToFolder?: boolean
  }
  acknowledgeNetworkConflictRisk?: boolean
  [key: string]: unknown
}

export type FieldErrors = { [formId: string]: string }

export type MigrationDrawerRHFValues = {
  securityGroups: string[]
  serverGroup: string
  dataCopyStartTime: string
  cutoverStartTime: string
  cutoverEndTime: string
  postMigrationActionSuffix: string
  postMigrationActionFolderName: string
}

export interface MigrationFormDrawerProps {
  open: boolean
  onClose: () => void
  reloadMigrations?: () => void
  onSuccess?: (message: string) => void
  retryConfig?: RetryMigrationConfig
  templatePrefill?: SavedTemplate
}

// ---------------------------------------------------------------------------
// NetworkAndStorageMappingStep types
// ---------------------------------------------------------------------------

export interface ResourceMap {
  source: string
  target: string
}

export type StorageCopyMethod = 'normal' | 'StorageAcceleratedCopy' | 'HotAdd'

export interface RollingFormParams extends Record<string, unknown> {
  vmwareCluster?: string
  pcdCluster?: string
  networkMappings?: ResourceMap[]
  storageMappings?: ResourceMap[]
  arrayCredsMappings?: ResourceMap[]
  securityGroups?: string[]
  serverGroup?: string
  dataCopyMethod?: string
  dataCopyStartTime?: string
  cutoverOption?: string
  cutoverStartTime?: string
  cutoverEndTime?: string
  postMigrationScript?: string
  osFamily?: string
  storageCopyMethod?: StorageCopyMethod
  proxyVMRef?: string
  postMigrationAction?: {
    suffix?: string
    folderName?: string
    renameVm?: boolean
    moveToFolder?: boolean
  }
  useGPU?: boolean
  disconnectSourceNetwork?: boolean
  fallbackToDHCP?: boolean
  networkPersistence?: boolean
}

export interface NetworkAndStorageMappingStepProps {
  vmwareNetworks: string[]
  vmWareStorage: string[]
  openstackNetworks: PCDNetworkInfo[]
  openstackStorage: string[]
  params: {
    networkMappings?: ResourceMap[]
    storageMappings?: ResourceMap[]
    arrayCredsMappings?: ResourceMap[]
    storageCopyMethod?: StorageCopyMethod
    proxyVMRef?: string
  }
  onChange: (key: string) => (value: any) => void
  networkMappingError?: string
  storageMappingError?: string
  stepNumber?: string
  loading?: boolean
  showHeader?: boolean
  subnetWarnings?: Record<string, string>
}

// ---------------------------------------------------------------------------
// RollingMigrationForm types
// ---------------------------------------------------------------------------

export interface VmNetworkInterface {
  mac: string
  network: string
  ipAddress: string[]
  preserveIP?: boolean
  preserveMAC?: boolean
}

export interface ESXHost {
  id: string
  name: string
  ip: string
  bmcIp: string
  maasState: string
  vms: number
  state: string
  pcdHostConfigName?: string
  pcdHostConfigId?: string
}

export interface VM {
  id: string
  name: string
  ip: string
  esxHost: string
  networks?: string[]
  datastores?: string[]
  cpu?: number
  memory?: number
  powerState: string
  osFamily?: string
  flavor?: string
  targetFlavorId?: string
  ipValidationStatus?: 'pending' | 'valid' | 'invalid' | 'validating'
  ipValidationMessage?: string
  networkInterfaces?: VmNetworkInterface[]
  preserveIp?: Record<number, boolean>
  preserveMac?: Record<number, boolean>
}

export type RollingMigrationRHFValues = {
  securityGroups: string[]
  serverGroup: string
  dataCopyStartTime: string
  cutoverStartTime: string
  cutoverEndTime: string
  postMigrationActionSuffix: string
  postMigrationActionFolderName: string
}

export interface RollingMigrationFormDrawerProps {
  open: boolean
  onClose: () => void
}

// ---------------------------------------------------------------------------
// VmsSelectionStep types
// ---------------------------------------------------------------------------

export interface RdmConfiguration {
  uuid: string
  diskName: string
  cinderBackendPool: string
  volumeType: string
  source: Record<string, string>
}

export interface VmDataWithFlavor extends VmData {
  isMigrated?: boolean
  flavor?: string
  flavorName?: string
  flavorNotFound?: boolean
  ipValidationStatus?: 'pending' | 'valid' | 'invalid' | 'validating'
  ipValidationMessage?: string
  powerState?: string
}

/**
 * Unified VM shape used by the merged VmsSelectionStep.
 * Bridges VmDataWithFlavor (standard migration) and VM (rolling migration).
 * Canonical power state: 'powered-on' | 'powered-off'
 * Canonical ip field: ip (not ipAddress)
 * Canonical cpu field: cpu (not cpuCount)
 */
export interface CanonicalVM {
  id: string
  name: string
  powerState: 'powered-on' | 'powered-off'
  ip: string
  cpu?: number
  memory?: number
  osFamily?: string
  flavor?: string
  targetFlavorId?: string
  networkInterfaces?: VmNetworkInterface[]
  esxHost?: string
  networks?: string[]
  datastores?: string[]
  preserveIp?: Record<number, boolean>
  preserveMac?: Record<number, boolean>
  ipValidationStatus?: 'pending' | 'valid' | 'invalid' | 'validating'
  ipValidationMessage?: string
  // Standard-migration-only (absent in rolling mode)
  isMigrated?: boolean
  flavorNotFound?: boolean
  hasSharedRdm?: boolean
  vmKey?: string
  vmid?: string
  labels?: Record<string, string>
  disks?: VMwareDiskEntry[]
  vmWareMachineName?: string
}

export type BulkIpEdit = { vmName: string; interfaceIndex: number; ip: string }
export type BulkIpClear = { vmName: string; interfaceIndex: number }

export interface VmsSelectionStepProps {
  mode?: 'standard' | 'rolling'

  // Standard-mode props
  onChange?: (id: string) => (value: unknown) => void
  error?: string
  open?: boolean
  vmwareCredsValidated?: boolean
  openstackCredsValidated?: boolean
  sessionId?: string
  openstackFlavors?: OpenStackFlavor[]
  vmwareCredName?: string
  openstackCredName?: string
  openstackCredentials?: OpenstackCreds
  vmwareCluster?: string
  useGPU?: boolean
  showHeader?: boolean
  // When set, the table shows only this VM (by name) and treats it as not migrated.
  retryVmName?: string
  // Prefilled VmData from useRetryPrefill (carries preserveIp/preserveMac/networkInterfaces
  // from the original plan). Merged into vmsWithFlavor so the "Assign IP" dialog shows
  // the user's previous configuration instead of raw VMware machine data.
  retryPrefillVm?: VmData

  // Rolling-mode props
  vmsWithAssignments?: VM[]
  setVmsWithAssignments?: Dispatch<SetStateAction<VM[]>>
  vmOSAssignments?: Record<string, string>
  setVmOSAssignments?: Dispatch<SetStateAction<Record<string, string>>>
  selectedVMs?: GridRowSelectionModel
  onSelectionChange?: (ids: GridRowSelectionModel) => void
  loadingVMs?: boolean
  vmIpValidationError?: string
  osValidationError?: string
  fetchClusterVMs?: () => Promise<void>
  openstackCredData?: OpenstackCreds | null
  reportError?: (error: Error, additionalContext?: ErrorContext) => void
}

// ---------------------------------------------------------------------------
// MigrationOptionsAlt types
// ---------------------------------------------------------------------------

export interface MigrationOptionsPropsInterface {
  params: FormValues & { useGPU?: boolean }
  onChange: (key: string) => (value: unknown) => void
  openstackCredentials?: OpenstackCreds
  selectedMigrationOptions: SelectedMigrationOptionsType
  updateSelectedMigrationOptions: (
    key:
      | keyof SelectedMigrationOptionsType
      | 'postMigrationAction.suffix'
      | 'postMigrationAction.folderName'
  ) => (value: unknown) => void
  errors: FieldErrors
  getErrorsUpdater: (key: string | number) => (value: string) => void
  stepNumber: string
  showHeader?: boolean
  hasSubnetMismatch?: boolean
  // True while a template/retry prefill is populating the form. Async global-default
  // effects must skip entirely in this window rather than race the prefill on load-order — settings and
  // prefill can each resolve first depending on network timing, so "only seed if unset"
  // isn't reliable when both fire in the same initial commit.
  skipDefaultSeeding?: boolean
}

// ---------------------------------------------------------------------------
// MigrationsTable types
// ---------------------------------------------------------------------------

declare module '@mui/x-data-grid' {
  interface ToolbarPropsOverrides {
    numSelected: number
    onDeleteSelected: () => void
    onBulkAdminCutover: () => void
    numEligibleForCutover: number
    onBulkRetry: () => void
    numEligibleForRetry: number
    isBulkRetryLoading: boolean
    filteredCount: number
    totalCount: number
  }
}

export interface CustomToolbarProps extends GridToolbarProps {
  numSelected: number
  onDeleteSelected: () => void
  onBulkAdminCutover: () => void
  numEligibleForCutover: number
  onBulkRetry: () => void
  numEligibleForRetry: number
  isBulkRetryLoading: boolean
  filteredCount: number
  totalCount: number
}

export interface MigrationsTableProps {
  migrations: Migration[]
  onDeleteMigration?: (name: string) => void
  onDeleteSelected?: (migrations: Migration[]) => void
  refetchMigrations: (options?: RefetchOptions) => Promise<QueryObserverResult<Migration[], Error>>
  loading?: boolean
  statusFilter?: string
}
