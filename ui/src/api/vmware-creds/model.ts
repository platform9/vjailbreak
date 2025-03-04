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
  name: string
  namespace: string
  resourceVersion: string
  uid: string
}

export interface Annotations {
  "kubectl.kubernetes.io/last-applied-configuration": string
}

export type Empty = object

export interface VMwareCredsSpec {
  // For credentials using secretRef (new approach)
  secretRef?: {
    name: string
  }
}

export interface GetVmwareCredsListMetadata {
  continue: string
  resourceVersion: string
}

export interface VMwareCredsStatus {
  vmwareValidationMessage: string
  vmwareValidationStatus: string
}
