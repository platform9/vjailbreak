export interface GetMigrationsList {
  apiVersion: APIVersion
  items: Migration[]
  kind: string
  metadata: GetMigrationsListMetadata
}

export enum APIVersion {
  VjailbreakK8SPf9IoV1Alpha1 = "vjailbreak.k8s.pf9.io/v1alpha1",
}

export interface Migration {
  apiVersion: APIVersion
  kind: Kind
  metadata: ItemMetadata
  spec: Spec
  status: StatusClass
}

export enum Kind {
  Migration = "Migration",
}

export interface ItemMetadata {
  annotations: Annotations
  creationTimestamp: Date
  generation: number
  managedFields: ManagedField[]
  name: string
  namespace: Namespace
  resourceVersion: string
  uid: string
}

export interface Annotations {
  "kubectl.kubernetes.io/last-applied-configuration": string
}

export interface ManagedField {
  apiVersion: APIVersion
  fieldsType: FieldsType
  fieldsV1: FieldsV1
  manager: Manager
  operation: Operation
  time?: Date
  subresource?: Subresource
}

export enum FieldsType {
  FieldsV1 = "FieldsV1",
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
  "f:kubectl.kubernetes.io/last-applied-configuration": FMigrationPlan
}

export type FMigrationPlan = object

export interface FSpec {
  "f:migrationPlan": FMigrationPlan
  "f:podRef": FMigrationPlan
  "f:vmName": FMigrationPlan
}

export interface FStatus {
  ".": FMigrationPlan
  "f:conditions": FMigrationPlan
  "f:phase": FMigrationPlan
}

export enum Manager {
  Kubectl = "kubectl",
  KubectlEdit = "kubectl-edit",
  KubectlLastApplied = "kubectl-last-applied",
}

export enum Operation {
  Apply = "Apply",
  Update = "Update",
}

export enum Subresource {
  Status = "status",
}

export enum Namespace {
  MigrationSystem = "migration-system",
}

export interface Spec {
  migrationPlan: MigrationPlan
  podRef: PodRef
  vmName: VMName
}

export enum MigrationPlan {
  VMMigrationU22 = "vm-migration-u22",
}

export enum PodRef {
  V2VHelperMigTestCbtBakClone0 = "v2v-helper-mig-test-cbt-bak-clone-0",
  V2VHelperMigTestCbtBakClone1 = "v2v-helper-mig-test-cbt-bak-clone-1",
}

export enum VMName {
  MigTestCbtBakClone0 = "mig_test_cbt_bak-clone-0",
  MigTestCbtBakClone1 = "mig_test_cbt_bak-clone-1",
}

export interface StatusClass {
  conditions: Condition[]
  phase: Phase
}

export interface Condition {
  lastTransitionTime: Date
  message: Message
  reason: Kind
  status: StatusEnum
  type: Type
}

export enum Message {
  CopyingDisk0 = "Copying disk 0",
  CopyingDisk1 = "Copying disk 1",
  MigratingVMFromVMwareToOpenstack = "Migrating VM from VMware to Openstack",
  MigrationValidatedSuccessfully = "Migration validated successfully",
}

export enum StatusEnum {
  False = "False",
  True = "True",
  Unknown = "Unknown",
}

export enum Type {
  DataCopy = "DataCopy",
  Migrated = "Migrated",
  Validated = "Validated",
}

export enum Phase {
  Failed = "Failed",
  Running = "Running",
  Succeeded = "Succeeded",
}

export interface GetMigrationsListMetadata {
  continue: string
  resourceVersion: string
}
