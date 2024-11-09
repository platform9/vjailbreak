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
  managedFields: ManagedField[]
  name: string
  namespace: string
  resourceVersion: string
  uid: string
}

export interface Annotations {
  "kubectl.kubernetes.io/last-applied-configuration": string
}

export interface ManagedField {
  apiVersion: string
  fieldsType: string
  fieldsV1: FieldsV1
  manager: string
  operation: string
  time?: Date
  subresource?: string
}

export interface FieldsV1 {
  "f:metadata"?: FMetadata
  "f:spec"?: FSpec
  "f:status"?: FStatus
}

export interface FMetadata {
  "f:annotations": FAnnotations
}

export interface FAnnotations {
  "f:kubectl.kubernetes.io/last-applied-configuration": FMigrationTemplate
}

export type FMigrationTemplate = object

export interface FSpec {
  "f:migrationStrategy": FMigrationStrategy
  "f:migrationTemplate": FMigrationTemplate
  "f:retry": FMigrationTemplate
  "f:virtualmachines": FMigrationTemplate
}

export interface FMigrationStrategy {
  "f:type": FMigrationTemplate
}

export interface FStatus {
  ".": FMigrationTemplate
  "f:migrationMessage": FMigrationTemplate
  "f:migrationStatus": FMigrationTemplate
}

export interface Spec {
  migrationStrategy: MigrationStrategy
  migrationTemplate: string
  retry: boolean
  virtualmachines: Array<string[]>
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
