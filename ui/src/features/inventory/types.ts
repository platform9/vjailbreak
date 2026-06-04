// Inventory / Migration Planner shared types.

import type { MigrationBucket } from './api/migration-buckets/model'

export type { MigrationBucket, BucketStatus } from './api/migration-buckets/model'
export { BucketStatusValues } from './api/migration-buckets/model'

/** Normalized VM power state used across the planner. */
export type VmPowerState = 'powered-on' | 'powered-off' | 'unknown'

/**
 * Read-only, planner-facing view of a discovered VM, derived from the existing
 * VMwareMachine / VmData model via useInventoryVms (T008).
 *
 * Note: the UI VM model does not expose per-disk capacity (GB), so size-based
 * ordering is a backend concern; `diskCount` is the FE proxy for "size".
 */
export interface InventoryVm {
  /** Stable id (VmData.id). */
  id: string
  /** VM display name — used as the bucket-membership key. */
  name: string
  /** Underlying VMwareMachine CR name, when known. */
  vmwareMachineName?: string
  powerState: VmPowerState
  /** Number of network interfaces (drives single-NIC selection + ordering). */
  nicCount: number
  /** Source VMware cluster, when known. */
  clusterName?: string
  /** Disk count — FE proxy for size (no capacityGB in the UI model). */
  diskCount: number
  /** Source network names (for default network mapping). */
  networks: string[]
  /** Source datastore names (for default storage mapping). */
  datastores: string[]
}

/** Map of VM name → owning bucket name (for the uniqueness/greying rules). */
export type BucketIdByVm = Record<string, string>

export interface InventoryData {
  vms: InventoryVm[]
  byName: Record<string, InventoryVm>
  bucketIdByVm: BucketIdByVm
  buckets: MigrationBucket[]
  credName?: string
  /** Datacenter of the (single) VMware credential — needed to build the source-cluster id. */
  vmwareDatacenter?: string
  isLoading: boolean
  isError: boolean
}
