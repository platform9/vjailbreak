export type ProxyVMValidationStatus =
  | 'Deploying'
  | 'DeployFailed'
  | 'Pending'
  | 'Verifying'
  | 'Ready'
  | 'VerificationFailed'

export interface ProxyVMSpec {
  vmName: string
  vmwareCredsRef: {
    name: string
  }
  sshKeySecretRef?: {
    name: string
  }
  sshKeyPairRef?: {
    name: string
  }
}

export interface ProxyVMStatus {
  validationStatus: ProxyVMValidationStatus
  validationMessage?: string
  ipAddress?: string
  attachedDiskCount?: number
  componentsVerified?: string[]
  lastValidationTime?: string
}

export interface ProxyVM {
  apiVersion: string
  kind: string
  metadata: {
    name: string
    namespace: string
    creationTimestamp?: string
    uid?: string
    resourceVersion?: string
    deletionTimestamp?: string
  }
  spec: ProxyVMSpec
  status?: ProxyVMStatus
}

export interface ProxyVMList {
  apiVersion: string
  kind: string
  metadata: {
    resourceVersion: string
  }
  items: ProxyVM[]
}
