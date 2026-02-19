export interface ESXiSSHCreds {
  apiVersion: string
  kind: string
  metadata: {
    name: string
    namespace: string
    creationTimestamp?: string
    generation?: number
    resourceVersion?: string
    uid?: string
  }
  spec: {
    secretRef: {
      name: string
      namespace?: string
    }
    username?: string
  }
  status?: {
    validationStatus?: string
    validationMessage?: string
    totalHosts?: number
    successfulHosts?: number
    failedHosts?: number
    lastValidationTime?: string
  }
}

export interface ESXiSSHCredsList {
  apiVersion: string
  kind: string
  metadata: {
    continue?: string
    resourceVersion?: string
  }
  items: ESXiSSHCreds[]
}
