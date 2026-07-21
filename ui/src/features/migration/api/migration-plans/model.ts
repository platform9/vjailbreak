export interface GetMigrationPlansList {
  apiVersion: string
  items: MigrationPlan[]
  kind: string
  metadata: GetMigrationPlansListMetadata
}

export interface MigrationPlan {
  apiVersion: string
  kind: string
  metadata: ItemMetadata
  spec: Spec
  status: Status
}

export interface ItemMetadata {
  annotations: Annotations
  creationTimestamp: Date
  generation: number
  name: string
  namespace: string
  resourceVersion: string
  uid: string
}

export interface Annotations {
  'kubectl.kubernetes.io/last-applied-configuration': string
}

export interface Spec {
  migrationStrategy: MigrationStrategy
  migrationTemplate: string
  retry: boolean
  virtualMachines: Array<string[]>
  securityGroups?: string[]
  serverGroup?: string
  fallbackToDHCP?: boolean
  firstBootScript?: string
  postMigrationAction?: {
    renameVm?: boolean
    suffix?: string
    moveToFolder?: boolean
    folderName?: string
  }
  advancedOptions?: {
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
  networkOverridesPerVM?: Record<
    string,
    Array<{
      interfaceIndex: number
      preserveIP: boolean
      preserveMAC: boolean
      UserAssignedIP?: string
    }>
  >
  // Copies each source VM's vSphere tags and custom attributes to the migrated VM
  preserveSourceTags?: boolean
  // Extra instance metadata applied to every migrated VM in the plan
  customMetadata?: Record<string, string>
}

export interface MigrationStrategy {
  type: string
  dataCopyStart?: string
  vmCutoverStart?: string
  vmCutoverEnd?: string
  adminInitiatedCutOver?: boolean
  performHealthChecks?: boolean
  healthCheckPort?: string
  disconnectSourceNetwork?: boolean
  dataOnly?: boolean
}

export interface Status {
  migrationMessage: string
  migrationStatus: string
}

export interface GetMigrationPlansListMetadata {
  continue: string
  resourceVersion: string
}
