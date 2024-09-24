export interface GetStorageMappingsList {
  apiVersion: string
  items: StorageMapping[]
  kind: string
  metadata: GetStorageMappingsListMetadata
}

export interface StorageMapping {
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
  "f:kubectl.kubernetes.io/last-applied-configuration": FStorages
}

export type FStorages = object

export interface FSpec {
  "f:storages": FStorages
}

export interface FStatus {
  ".": FStorages
  "f:storageMappingValidationMessage": FStorages
  "f:storageMappingValidationStatus": FStorages
}

export interface Spec {
  storages: Storage[]
}

export interface Storage {
  source: string
  target: string
}

export interface Status {
  storageMappingValidationMessage: string
  storageMappingValidationStatus: string
}

export interface GetStorageMappingsListMetadata {
  continue: string
  resourceVersion: string
}
