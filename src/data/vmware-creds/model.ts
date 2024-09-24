export interface GetVMWareCredsList {
  apiVersion: string
  items: VMwareCreds[]
  kind: string
  metadata: GetVmwareCredsListMetadata
}

export interface VMwareCreds {
  apiVersion: string
  kind: string
  metadata: ItemMetadata
  spec: VMwareCredsSpec
  status: VMwareCredsStatus
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
  time: Date
}

export interface FieldsV1 {
  "f:metadata": FMetadata
  "f:spec": FSpec
}

export interface FMetadata {
  "f:annotations": FAnnotations
}

export interface FAnnotations {
  ".": Empty
  "f:kubectl.kubernetes.io/last-applied-configuration": Empty
}

export type Empty = object

export interface FSpec {
  ".": Empty
  "f:VCENTER_HOST": Empty
  "f:VCENTER_INSECURE": Empty
  "f:VCENTER_PASSWORD": Empty
  "f:VCENTER_USERNAME": Empty
}

export interface VMwareCredsSpec {
  VCENTER_HOST: string
  VCENTER_INSECURE: boolean
  VCENTER_PASSWORD: string
  VCENTER_USERNAME: string
}

export interface GetVmwareCredsListMetadata {
  continue: string
  resourceVersion: string
}

export interface VMwareCredsStatus {
  vmwareValidationMessage: string
  vmwareValidationStatus: string
}
