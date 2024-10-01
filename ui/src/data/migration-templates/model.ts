export interface GetMigrationTemplatesList {
  apiVersion: string
  items: MigrationTemplate[]
  kind: string
  metadata: GetMigrationTemplatesMetadata
}

export interface MigrationTemplate {
  apiVersion: string
  kind: string
  metadata: MigrationTemplateMetadata
  spec: MigrationTemplateSpec
  status: MigrationTemplateStatus
}

export interface MigrationTemplateMetadata {
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
  "f:kubectl.kubernetes.io/last-applied-configuration": FNetworkMapping
}

export type FNetworkMapping = object

export interface FSpec {
  "f:destination": FDestination
  "f:networkMapping": FNetworkMapping
  "f:source": FSource
  "f:storageMapping": FNetworkMapping
}

export interface FDestination {
  "f:openstackRef": FNetworkMapping
}

export interface FSource {
  "f:datacenter": FNetworkMapping
  "f:vmwareRef": FNetworkMapping
}

export interface FStatus {
  ".": FNetworkMapping
  "f:openstack": FOpenstack
  "f:vmware": FNetworkMapping
}

export interface FOpenstack {
  ".": FNetworkMapping
  "f:networks": FNetworkMapping
  "f:volumeTypes": FNetworkMapping
}

export interface MigrationTemplateSpec {
  destination: Destination
  networkMapping: string
  source: Source
  storageMapping: string
}

export interface Destination {
  openstackRef: string
}

export interface Source {
  datacenter: string
  vmwareRef: string
}

export interface MigrationTemplateStatus {
  openstack: Openstack
  vmware: VmData[]
}

export interface Openstack {
  networks: string[]
  volumeTypes: string[]
}

export interface VmData {
  datastores: string[]
  name: string
  networks?: string[]
}

export interface GetMigrationTemplatesMetadata {
  continue: string
  resourceVersion: string
}
