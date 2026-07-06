import { ItemMetadata } from 'src/api/rolling-migration-plans/model'

export interface NameReference {
  name: string
}

export type AutoStartMode = 'Auto' | 'Manual'

export type ClusterConversionBatchPhase =
  | 'Pending'
  | 'Running'
  | 'Succeeded'
  | 'PartialFail'
  | 'Failed'

export type HostConversionPhase =
  | 'CheckingEligibility'
  | 'NotReady'
  | 'Ready'
  | 'Converting'
  | 'Succeeded'
  | 'Failed'
  | 'NeedsAttention'
  | 'Skipped'

export interface HostEntry {
  esxiName: string
}

export interface HostConversionStatus {
  esxiName: string
  phase: HostConversionPhase
  eligibilityStatus?: 'Ready' | 'NotReady' | 'Unknown'
  eligibilityReason?: string
  retryCount?: number
  nextRetryAt?: string
  esxiMigrationRef?: NameReference
  message?: string
  startedAt?: string
  completedAt?: string
  skippedAt?: string
}

export interface ClusterConversionBatchSpec {
  vmwareClusterName: string
  vmwareCredsRef: NameReference
  openstackCredsRef: NameReference
  bmConfigRef: NameReference
  cloudInitConfigRef?: { name: string; namespace: string }
  hosts: HostEntry[]
  autoStart: AutoStartMode
  maxRetries?: number
  retryBackoffSeconds?: number
}

export interface ClusterConversionBatchStatus {
  phase?: ClusterConversionBatchPhase
  hosts?: HostConversionStatus[]
  totalHosts?: number
  succeededHosts?: number
  needsAttentionHosts?: number
  skippedHosts?: number
  runningHosts?: number
  pendingHosts?: number
  startedAt?: string
  completedAt?: string
  message?: string
}

export interface ClusterConversionBatch {
  apiVersion: string
  kind: string
  metadata: ItemMetadata
  spec: ClusterConversionBatchSpec
  status?: ClusterConversionBatchStatus
}

export interface ClusterConversionBatchList {
  apiVersion: string
  kind: string
  metadata: { resourceVersion: string }
  items: ClusterConversionBatch[]
}
