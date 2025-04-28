export interface GetRollingMigrationPlansList {
  apiVersion: string
  items: RollingMigrationPlan[]
  kind: string
  metadata: ListMetadata
}

export interface RollingMigrationPlan {
  apiVersion: string
  kind: string
  metadata: ItemMetadata
  spec: RollingMigrationPlanSpec
  status?: RollingMigrationPlanStatus
}

export interface ListMetadata {
  continue: string
  resourceVersion: string
}

export interface ItemMetadata {
  annotations?: Record<string, string>
  creationTimestamp?: Date
  generation?: number
  name: string
  namespace?: string
  resourceVersion?: string
  uid?: string
  labels?: Record<string, string>
}

export interface RollingMigrationPlanSpec {
  clusterSequence: ClusterSequence[]
  vmwareCredsRef: ResourceRef
  openstackCredsRef: ResourceRef
  bmConfigRef: ResourceRef
}

export interface ClusterSequence {
  clusterName: string
  vmSequence: VMSequence[]
}

export interface VMSequence {
  vmName: string
  esxiName: string
}

export interface ResourceRef {
  name: string
  namespace?: string
}

export interface NetworkMapping {
  source: string
  destination: string
}

export interface StorageMapping {
  source: string
  destination: string
}

export interface RollingMigrationPlanStatus {
  migrationStatus?: string
  migrationMessage?: string
  phase?: string
  startTime?: string
  completionTime?: string
}
