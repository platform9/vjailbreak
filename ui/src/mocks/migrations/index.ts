import mockClusters from "./mock-clusters.json"
import mockHosts from "./mock-hosts.json"
import mockVms from "./mock-vms.json"
import mockESXiMigrations from "./mock-esxi-migrations.json"

// Define ClusterMigration type
export interface ClusterMigration {
  apiVersion: string
  kind: string
  metadata: {
    name: string
    namespace: string
    creationTimestamp: string
    finalizers: string[]
    generation: number
    resourceVersion: string
    uid: string
    ownerReferences: {
      apiVersion: string
      kind: string
      name: string
      uid: string
    }[]
  }
  spec: {
    clusterName: string
    esxiMigrationSequence: string[]
    openstackCredsRef: { name: string }
    rollingMigrationPlanRef: { name: string }
    vmwareCredsRef: { name: string }
  }
  status: {
    currentESXi: string
    message: string
    phase: string
  }
}

// Define ESXIMigration type
export interface ESXIMigration {
  apiVersion: string
  kind: string
  metadata: {
    name: string
    namespace: string
    creationTimestamp: string
    finalizers: string[]
    generation: number
    resourceVersion: string
    uid: string
    ownerReferences: {
      apiVersion: string
      kind: string
      name: string
      uid: string
    }[]
  }
  spec: {
    esxiName: string
    openstackCredsRef: { name: string }
    rollingMigrationPlanRef: { name: string }
    vmwareCredsRef: { name: string }
  }
}

// VM model
export interface VM {
  id: string
  name: string
  status: string
  cluster: string
  ip: string
  esxHost: string
  networks?: string[]
  datastores?: string[]
  cpu?: number
  memory?: number
  powerState: string
}

// ESX host model
export interface ESXHost {
  id: string
  name: string
  ip: string
  bmcIp: string
  maasState: string
  vms: number
  state: string
}

// Type assertions for the imported JSON data
const clusterMigrations = mockClusters as ClusterMigration[]
const esxiMigrations = mockESXiMigrations as ESXIMigration[]
const esxHostsByCluster: Record<string, ESXHost[]> = mockHosts
const vmsByCluster: Record<string, VM[]> = mockVms

export { clusterMigrations, esxiMigrations, esxHostsByCluster, vmsByCluster }
