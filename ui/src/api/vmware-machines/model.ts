export interface VMwareVM {
  cpu: number
  datastores: string[]
  disks: string[]
  memory: number
  name: string
  networks?: string[]
  vmState: string
  ipAddress?: string
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
