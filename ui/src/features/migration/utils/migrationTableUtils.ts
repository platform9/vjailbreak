import type { DATE_FILTER_OPTIONS } from 'src/components/grid'
import { Condition, Phase } from '../api/migrations'

export const PHASE_STEPS: Record<string, number> = {
  [Phase.Pending]: 1,
  [Phase.Validating]: 2,
  [Phase.AwaitingDataCopyStart]: 3,
  [Phase.CopyingBlocks]: 4,
  [Phase.SnapshottingSourceVM]: 4,
  [Phase.AttachingDisksToProxy]: 4,
  [Phase.IdentifyingBlockDevices]: 4,
  [Phase.CopyingChangedBlocks]: 5,
  [Phase.HotAddTransferInProgress]: 5,
  [Phase.HotAddCleanup]: 5,
  [Phase.ConvertingDisk]: 6,
  [Phase.AwaitingCutOverStartTime]: 7,
  [Phase.AwaitingAdminCutOver]: 8,
  [Phase.Succeeded]: 9,
  [Phase.DataCopied]: 9,
  [Phase.Failed]: 10,
  [Phase.ValidationFailed]: 11
}

export const AWAITING_ACTION_PHASES: Phase[] = [
  Phase.AwaitingAdminCutOver,
  Phase.AwaitingCutOverStartTime
]

export const IN_PROGRESS_PHASES: Phase[] = [
  Phase.Validating,
  Phase.AwaitingDataCopyStart,
  Phase.CopyingBlocks,
  Phase.CopyingChangedBlocks,
  Phase.SnapshottingSourceVM,
  Phase.AttachingDisksToProxy,
  Phase.IdentifyingBlockDevices,
  Phase.HotAddTransferInProgress,
  Phase.HotAddCleanup,
  Phase.ConvertingDisk
]

export type MigrationStatusCategory =
  | 'inProgress'
  | 'awaitingAction'
  | 'pending'
  | 'succeeded'
  | 'failed'

// Buckets a migration's phase into the 5 summary categories shown on the Migrations
// page stat cards; also drives the "click to filter" status filter on the table.
export function getMigrationStatusCategory(phase: Phase | undefined): MigrationStatusCategory {
  if (!phase || phase === Phase.Pending) return 'pending'
  if (phase === Phase.Succeeded || phase === Phase.DataCopied) return 'succeeded'
  if (phase === Phase.Failed || phase === Phase.ValidationFailed) return 'failed'
  if (AWAITING_ACTION_PHASES.includes(phase)) return 'awaitingAction'
  return 'inProgress'
}

export const STATUS_FILTER_OPTIONS = [
  'All',
  'In Progress',
  'Awaiting Action',
  'Pending',
  'Succeeded',
  'Failed'
] as const

export const STATUS_FILTER_TO_CATEGORY: Record<string, MigrationStatusCategory> = {
  'In Progress': 'inProgress',
  'Awaiting Action': 'awaitingAction',
  Pending: 'pending',
  Succeeded: 'succeeded',
  Failed: 'failed'
}

// Keyed off CustomSearchToolbar's DATE_FILTER_OPTIONS (type-only import — no runtime
// dependency) so the option labels can't drift out of sync between the toolbar UI and
// the actual filtering logic. 'All Time' is intentionally absent: it means "no filter".
type DateFilterOption = Exclude<(typeof DATE_FILTER_OPTIONS)[number], 'All Time'>

const DATE_FILTER_WINDOW_MS: Record<DateFilterOption, number> = {
  'Last 24 hours': 24 * 60 * 60 * 1000,
  'Last 7 days': 7 * 24 * 60 * 60 * 1000,
  'Last 30 days': 30 * 24 * 60 * 60 * 1000
}

// Drives the migrations table's "filter by creation date" toolbar control.
// `now` is injectable so callers/tests don't depend on the real clock. The API client
// types creationTimestamp as `Date`, but it's really JSON — accept both.
export function matchesDateFilter(
  creationTimestamp: string | Date | undefined,
  filter: string,
  now: number = Date.now()
): boolean {
  const windowMs = (DATE_FILTER_WINDOW_MS as Record<string, number>)[filter]
  if (!windowMs) return true // 'All Time' (or an unrecognized filter) — no filtering

  if (!creationTimestamp) return false
  const createdAt = new Date(creationTimestamp).getTime()
  if (Number.isNaN(createdAt)) return false

  return now - createdAt <= windowMs
}

export const getProgressText = (
  phase: Phase | undefined,
  conditions: Condition[] | undefined,
  currentDisk?: string,
  totalDisks?: number
): string => {
  if (!phase || phase === Phase.Unknown) {
    return 'Unknown Status'
  }

  const stepNumber = PHASE_STEPS[phase] || 0
  const totalSteps = 9

  const latestCondition = conditions?.sort(
    (a, b) => new Date(b.lastTransitionTime).getTime() - new Date(a.lastTransitionTime).getTime()
  )[0]

  const message = latestCondition?.message || phase

  if (phase === Phase.Failed || phase === Phase.ValidationFailed || phase === Phase.Succeeded || phase === Phase.DataCopied) {
    return `${phase} - ${message}`
  }

  let diskInfo = ''
  if (
    currentDisk &&
    totalDisks &&
    (phase === Phase.CopyingBlocks || phase === Phase.CopyingChangedBlocks)
  ) {
    const parsedDisk = parseInt(currentDisk, 10)
    const currentDiskNum = Number.isNaN(parsedDisk) ? 1 : parsedDisk + 1
    diskInfo = ` (disk ${currentDiskNum}/${totalDisks})`
  }

  return `STEP ${stepNumber}/${totalSteps}: ${phase}${diskInfo} - ${message}`
}
