export type ProxyVMValidationStatus = 'Pending' | 'Verifying' | 'Ready' | 'VerificationFailed'

export interface ProxyVMSpec {
  vmName: string
  vmwareCredsRef: {
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
    creationTimestamp: string
    uid: string
    resourceVersion?: string
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
