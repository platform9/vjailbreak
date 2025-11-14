export interface ArrayCreds {
  apiVersion: string
  kind: string
  metadata: {
    name: string
    namespace: string
    labels?: Record<string, string>
    creationTimestamp?: string
    uid?: string
  }
  spec: ArrayCredsSpec
  status?: ArrayCredsStatus
}

export interface ArrayCredsSpec {
  vendorType: string
  autoDiscovered?: boolean
  openstackMapping?: OpenStackMapping
  secretRef?: {
    name?: string
    namespace?: string
  }
}

export interface OpenStackMapping {
  volumeType: string
  cinderBackendName: string
  cinderBackendPool?: string
}

export interface ArrayCredsStatus {
  validationStatus?: string
  validationMessage?: string
}

export interface ArrayCredsFormData {
  name: string
  vendorType: string
  volumeType: string
  cinderBackendName: string
  cinderBackendPool?: string
  // Secret data
  managementEndpoint?: string
  username?: string
  password?: string
  apiToken?: string
}
