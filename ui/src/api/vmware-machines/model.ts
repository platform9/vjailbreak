export interface VMwareDisk {
  name?: string
  capacityGB?: number
  datastore?: string
  datastoreId?: string
}

export type VMwareDiskEntry = string | VMwareDisk

export interface VMwareVM {
  cpu: number
  datastores: string[]
  disks: VMwareDiskEntry[]
  memory: number
  name: string
  vmid?: string
  networks?: string[]
  vmState: string
  ipAddress?: string
  assignedIp?: string
  osFamily?: string
  networkInterfaces?: VmNetworkInterface[]
  rdmDisks?: string[]
  clusterName?: string
  useGPU?: boolean
  // vSphere tag category name -> comma-separated tag names (e.g. "env" -> "production")
  tags?: Record<string, string>
  // vSphere custom attribute name -> value (e.g. "Owner" -> "alice@corp.com")
  customAttributes?: Record<string, string>
}

export interface VmNetworkInterface {
  mac: string
  network: string
  ipAddress: string[]
}

export interface VMwareMachine {
  apiVersion: string
  kind: string
  metadata: {
    name: string
    namespace: string
    creationTimestamp: string
    labels: {
      [key: string]: string
    }
  }
  spec: {
    vms: VMwareVM
    targetFlavorId?: string
  }
  status: {
    migrated: boolean
    powerState: string
    conditions?: Array<{
      type: string
      status: string
      reason: string
      message: string
      lastTransitionTime: string
    }>
  }
}

export interface VMwareMachineList {
  apiVersion: string
  kind: string
  metadata: {
    continue: string
    resourceVersion: string
  }
  items: VMwareMachine[]
}
