// Wire/CRD model for the MigrationBucket resource (planned CRD — see
// specs/004-migration-planner/DESIGN.md §8). The UI builds against this shape so the
// data layer can flip from mock → real k8s API (T047) with no component changes.

import type { FormValues, SelectedMigrationOptionsType } from 'src/features/migration/types'

/** Lifecycle status of a bucket (derived from its execution objects). */
export type BucketStatus = 'NotMigrated' | 'Scheduled' | 'InProgress' | 'Migrated'

export const BucketStatusValues = {
  NotMigrated: 'NotMigrated',
  Scheduled: 'Scheduled',
  InProgress: 'InProgress',
  Migrated: 'Migrated'
} as const

/** A single source → target mapping entry (network or storage). */
export interface BucketMapping {
  source: string
  target: string
}

/**
 * Migration configuration carried by a bucket. Mirrors the output of the existing
 * Migration Form. Kept intentionally close to the migration CRDs so the trigger
 * step can compile it into MigrationPlan/RollingMigrationPlan without translation.
 */
export interface MigrationBucketConfig {
  /** VMware source cluster (auto-detected from the bucket's VMs). */
  sourceCluster?: string
  /** Destination PCD cluster (defaults to first PCDHostConfig entry). */
  pcdCluster?: string
  networkMappings?: BucketMapping[]
  storageMappings?: BucketMapping[]
  securityGroups?: string[]
  serverGroup?: string
  dataCopyMethod?: string
  /** Remaining advanced options. */
  advancedOptions?: Record<string, unknown>
  /**
   * Full migration-form values, for exact round-trip editing in the bucket editor (which
   * reuses the Migration Form). Mirrors the inputs a MigrationPlan is built from.
   */
  formValues?: Partial<FormValues>
  /** Which optional migration-options checkboxes were enabled. */
  selectedOptions?: SelectedMigrationOptionsType
}

export interface MigrationBucketSpec {
  /** Source VMware credential this bucket belongs to (keys multi-cred future work). */
  vmwareCredsRef: { name: string }
  /** Member VM names (bucket membership). */
  vms: string[]
  /** True for the auto-created default bucket (non-deletable). */
  isDefault: boolean
  /** Optional future schedule time (RFC3339). */
  schedule?: string
  /** Embedded migration configuration. */
  config: MigrationBucketConfig
}

export interface MigrationBucketStatus {
  phase: BucketStatus
  message?: string
}

export interface MigrationBucket {
  apiVersion: string
  kind: string
  metadata: {
    name: string
    namespace: string
    creationTimestamp?: string
    uid?: string
    resourceVersion?: string
    labels?: Record<string, string>
  }
  spec: MigrationBucketSpec
  status?: MigrationBucketStatus
}

export interface MigrationBucketList {
  apiVersion: string
  kind: string
  metadata: { resourceVersion: string }
  items: MigrationBucket[]
}

export const MIGRATION_BUCKET_API_VERSION = 'vjailbreak.k8s.pf9.io/v1alpha1'
export const MIGRATION_BUCKET_KIND = 'MigrationBucket'
