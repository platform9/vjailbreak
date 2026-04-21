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
  advancedOptions?: AdvancedOptions
}

export interface AdvancedOptions {
  periodicSyncInterval?: string
  periodicSyncEnabled?: boolean
  networkPersistence?: boolean
  removeVMwareTools?: boolean
  acknowledgeNetworkConflictRisk?: boolean
  imageProfiles?: string[]
}

export interface MigrationStrategy {
  type: string
}

export interface Status {
  migrationMessage: string
  migrationStatus: string
}

export interface GetMigrationPlansListMetadata {
  continue: string
  resourceVersion: string
}
