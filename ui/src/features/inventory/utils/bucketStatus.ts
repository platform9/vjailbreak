import type { StatusChipTone } from 'src/components'
import type { BucketStatus, MigrationBucket } from '../types'

/**
 * Resolve a bucket's lifecycle status.
 *
 * Phase 2: reads the bucket's own `status.phase` (mock/CRD field). In Phase 10 (T049) this
 * will instead be derived from the bucket's MigrationPlan/Migration execution objects.
 */
export const getBucketStatus = (bucket: MigrationBucket): BucketStatus =>
  bucket.status?.phase ?? 'NotMigrated'

/** Human-readable label for a bucket status. */
export const bucketStatusLabel = (status: BucketStatus): string => {
  switch (status) {
    case 'Scheduled':
      return 'Scheduled'
    case 'InProgress':
      return 'In progress'
    case 'Migrated':
      return 'Migrated'
    case 'NotMigrated':
    default:
      return 'Not migrated'
  }
}

const RUNNING_HINTS = ['running', 'progress', 'copying', 'converting', 'migrating', 'cutover']
const DONE_HINTS = ['succeeded', 'completed']
const SCHEDULED_HINTS = ['pending', 'await', 'scheduled', 'validating', 'queued']

const classify = (phase: string): 'done' | 'running' | 'scheduled' | 'other' => {
  const p = phase.toLowerCase()
  if (RUNNING_HINTS.some((h) => p.includes(h))) return 'running'
  if (DONE_HINTS.some((h) => p.includes(h))) return 'done'
  if (SCHEDULED_HINTS.some((h) => p.includes(h))) return 'scheduled'
  return 'other'
}

/**
 * Derive a bucket's live status from the real Migration objects of its member VMs (T049 /
 * FR-017). `phaseByVmName` maps a VM name to its Migration `status.phase`. Falls back to the
 * bucket's own `status.phase` when none of its VMs have a migration yet.
 *
 * Best-effort: matches on exact VM name; a bucket VM with no matching Migration is treated as
 * not-yet-started.
 */
export const deriveBucketStatus = (
  bucket: MigrationBucket,
  phaseByVmName: Record<string, string>
): BucketStatus => {
  const phases = bucket.spec.vms
    .map((vm) => phaseByVmName[vm])
    .filter((p): p is string => Boolean(p))
    .map(classify)

  if (phases.length === 0) return getBucketStatus(bucket)
  if (phases.some((c) => c === 'running')) return 'InProgress'
  if (phases.length === bucket.spec.vms.length && phases.every((c) => c === 'done')) {
    return 'Migrated'
  }
  if (phases.some((c) => c === 'done')) return 'InProgress' // partially migrated
  if (phases.some((c) => c === 'scheduled')) return 'Scheduled'
  return getBucketStatus(bucket)
}

/** StatusChip tone for a bucket status, reusing the design-system chip palette. */
export const bucketStatusTone = (status: BucketStatus): StatusChipTone => {
  switch (status) {
    case 'Scheduled':
      return 'info'
    case 'InProgress':
      return 'warning'
    case 'Migrated':
      return 'success'
    case 'NotMigrated':
    default:
      return 'default'
  }
}
