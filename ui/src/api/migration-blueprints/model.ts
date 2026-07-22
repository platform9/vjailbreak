import type { Network } from 'src/api/network-mapping/model'
import type { Storage } from 'src/api/storage-mappings/model'

export interface MigrationBlueprintStrategy {
  type: 'hot' | 'cold' | 'mock'
  dataCopyStart?: string
  vmCutoverStart?: string
  vmCutoverEnd?: string
  adminInitiatedCutOver?: boolean
  performHealthChecks?: boolean
  healthCheckPort?: string
  disconnectSourceNetwork?: boolean
  arrayOffload?: boolean
}

export interface MigrationBlueprintAdvancedOptions {
  granularVolumeTypes?: string[]
  granularNetworks?: string[]
  granularPorts?: string[]
  periodicSyncInterval?: string
  periodicSyncEnabled?: boolean
  networkPersistence?: boolean
  removeVMwareTools?: boolean
  acknowledgeNetworkConflictRisk?: boolean
  imageProfiles?: string[]
}

export interface MigrationBlueprintPostMigrationAction {
  renameVm?: boolean
  suffix?: string
  moveToFolder?: boolean
  folderName?: string
}

export interface DatastoreArrayCredsMapping {
  source: string
  target: string
}

export interface MigrationBlueprintSpec {
  displayName: string
  description?: string
  vmwareRef?: string
  pcdRef?: string
  vmwareClusterName?: string
  noVMwareClusterFilter?: boolean
  targetPCDClusterName?: string
  networkMappings?: Network[]
  storageMappings?: Storage[]
  arrayCredsMappings?: DatastoreArrayCredsMapping[]
  proxyVMRef?: { name: string }
  migrationStrategy?: MigrationBlueprintStrategy
  advancedOptions?: MigrationBlueprintAdvancedOptions
  postMigrationAction?: MigrationBlueprintPostMigrationAction
  firstBootScript?: string
  securityGroups?: string[]
  serverGroup?: string
  fallbackToDHCP?: boolean
  preserveSourceTags?: boolean
  customMetadata?: Record<string, string>
  useGPUFlavor?: boolean
  storageCopyMethod?: 'normal' | 'StorageAcceleratedCopy' | 'HotAdd'
  osFamily?: 'windowsGuest' | 'linuxGuest'
  virtioWinDriver?: string
}

export interface MigrationBlueprint {
  apiVersion: string
  kind: string
  metadata: {
    name: string
    namespace?: string
    creationTimestamp?: string
    uid?: string
    resourceVersion?: string
  }
  spec: MigrationBlueprintSpec
}

export interface MigrationBlueprintList {
  apiVersion: string
  kind: string
  metadata: {
    resourceVersion: string
  }
  items: MigrationBlueprint[]
}
