export interface GetOpenstackCredsList {
  apiVersion: string
  items: OpenstackCreds[]
  kind: string
  metadata: GetOpenstackCredsListMetadata
}

export interface OpenstackCreds {
  apiVersion: string
  kind: string
  metadata: OpenstackCredsMetadata
  spec: OpenstackCredsSpec
  status: OpenstackCredsStatus
}

export interface OpenstackCredsMetadata {
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
  "f:OS_AUTH_URL": Empty
  "f:OS_DOMAIN_NAME": Empty
  "f:OS_PASSWORD": Empty
  "f:OS_REGION_NAME": Empty
  "f:OS_TENANT_NAME": Empty
  "f:OS_USERNAME": Empty
}

export interface OpenstackCredsSpec {
  // For credentials using secretRef (new approach)
  secretRef?: {
    name: string
  }
  // For legacy direct credential storage (deprecated)
  OS_AUTH_URL?: string
  OS_DOMAIN_NAME?: string
  OS_USERNAME?: string
  OS_PASSWORD?: string
  OS_REGION_NAME?: string
  OS_TENANT_NAME?: string
  OS_INSECURE?: boolean
}

export interface GetOpenstackCredsListMetadata {
  continue: string
  resourceVersion: string
}

export interface OpenstackCredsStatus {
  openstackValidationMessage: string
  openstackValidationStatus: string
}

export interface OpenstackImage {
  name: string
  id: string
  status: string
  virtual_size: number
}

export interface OpenstackImagesResponse {
  images: OpenstackImage[]
}
