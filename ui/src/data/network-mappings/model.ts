export interface GetNetworkMappingsList {
  apiVersion: string
  items: NetworkMapping[]
  kind: string
  metadata: GetNetworkMappingsListMetadata
}

export interface NetworkMapping {
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
  "f:kubectl.kubernetes.io/last-applied-configuration": FNetworks
}

export type FNetworks = object

export interface FSpec {
  "f:networks": FNetworks
}

export interface FStatus {
  ".": FNetworks
  "f:networkMappingValidationMessage": FNetworks
  "f:networkMappingValidationStatus": FNetworks
}

export interface Spec {
  networks: Network[]
}

export interface Network {
  source: string
  target: string
}

export interface Status {
  networkMappingValidationMessage: string
  networkMappingValidationStatus: string
}

export interface GetNetworkMappingsListMetadata {
  continue: string
  resourceVersion: string
}
