import type { VmData } from 'src/features/migration/api/migration-templates/model'

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
  storageCopyMethod?: 'normal' | 'StorageAcceleratedCopy'
  vmwareCluster?: string
  pcdCluster?: string
  dataCopyMethod?: string
  dataCopyStartTime?: string
  cutoverOption?: string
  cutoverStartTime?: string
  cutoverEndTime?: string
  postMigrationScript?: string
  osFamily?: string
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
  useFlavorless?: boolean
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
  useFlavorless?: boolean
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
