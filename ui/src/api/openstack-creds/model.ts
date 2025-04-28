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
  name: string
  namespace: string
  resourceVersion: string
  uid: string
}

export interface Annotations {
  "kubectl.kubernetes.io/last-applied-configuration": string
}

export interface OpenstackFlavor {
  id: string
  name: string
  vcpus: number
  ram: number
  disk: number
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
  flavors?: OpenStackFlavor[]
}

export interface GetOpenstackCredsListMetadata {
  continue: string
  resourceVersion: string
}

export interface OpenstackCredsStatus {
  openstackValidationMessage: string
  openstackValidationStatus: string
  openstack?: {
    networks?: string[]
    volumeTypes?: string[]
  }
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

export interface OpenStackFlavor {
  "OS-FLV-EXT-DATA:ephemeral"?: number
  description?: string
  disk: number
  id: string
  name: string
  "os-flavor-access:is_public"?: boolean
  ram: number
  rxtx_factor?: number
  vcpus: number
}
