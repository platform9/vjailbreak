import type { MigrationBlueprintSpec } from 'src/api/migration-blueprints/model'

// Mirrors FormValues.dataCopyMethod ('hot' | 'cold' | 'mock') — the "Hot" / "Cold"
// / "Mock" migration-mode tag shown on each template card.
export type DataCopyMethod = 'hot' | 'cold' | 'mock'

export interface SavedTemplateMapping {
  source: string
  target: string
}

export interface SavedTemplatePostMigrationAction {
  suffix?: string
  folderName?: string
  renameVm?: boolean
  moveToFolder?: boolean
}

export type SavedTemplateStorageCopyMethod = 'normal' | 'StorageAcceleratedCopy' | 'HotAdd'

// UI-facing, flattened view of a MigrationBlueprint for the Templates tab
// card/drawer. `spec` carries the full backend spec so clone/delete can
// round-trip fields this flattened shape doesn't surface.
export interface SavedTemplate {
  name: string // k8s object name (sanitized display name)
  resourceVersion: string
  displayName: string
  description?: string
  createdAt: string
  sourceVCenter: string
  sourceCluster: string
  destination: string
  targetCluster: string
  networkMappings: SavedTemplateMapping[]
  storageMappings: SavedTemplateMapping[]
  arrayCredsMappings: SavedTemplateMapping[]
  dataCopyMethod: DataCopyMethod
  dataCopyStartTime: string
  storageCopyMethod: SavedTemplateStorageCopyMethod
  proxyVMRef: string
  cutoverOption: string // CUTOVER_TYPES value ('0' | '1' | '2')
  disconnectSourceNetwork: boolean
  fallbackToDHCP: boolean
  securityGroups: string[]
  serverGroup: string
  firstBootScript: string
  networkPersistence: boolean
  removeVMwareTools: boolean
  imageProfiles: string[]
  periodicSyncInterval: string
  periodicSyncEnabled: boolean
  acknowledgeNetworkConflictRisk: boolean
  postMigrationAction?: SavedTemplatePostMigrationAction
  osFamily?: string
  useGPU?: boolean
  spec: MigrationBlueprintSpec
}

export interface SaveAsTemplateInput {
  displayName: string
  description?: string
  sourceVCenter: string
  sourceCluster?: string
  destination: string
  targetCluster: string
  networkMappings: SavedTemplateMapping[]
  storageMappings: SavedTemplateMapping[]
  arrayCredsMappings?: SavedTemplateMapping[]
  dataCopyMethod: DataCopyMethod
  dataCopyStartTime?: string
  storageCopyMethod?: SavedTemplateStorageCopyMethod
  proxyVMRef?: string
  cutoverOption: string // CUTOVER_TYPES value ('0' | '1' | '2')
  disconnectSourceNetwork?: boolean
  fallbackToDHCP?: boolean
  securityGroups?: string[]
  serverGroup?: string
  firstBootScript?: string
  networkPersistence?: boolean
  removeVMwareTools?: boolean
  imageProfiles?: string[]
  periodicSyncInterval?: string
  periodicSyncEnabled?: boolean
  acknowledgeNetworkConflictRisk?: boolean
  postMigrationAction?: SavedTemplatePostMigrationAction
  osFamily?: string
  useGPU?: boolean
}
