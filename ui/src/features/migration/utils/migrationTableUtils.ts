import type { DATE_FILTER_OPTIONS } from 'src/components/grid'
import { Condition, Phase } from '../api/migrations'
import { getDesignIndex } from './phaseUtils'

export const TOTAL_STEPS = 9

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

// Conditions carry no ordering guarantee from the API — always read the most recent one.
export function getLatestCondition(conditions: Condition[] | undefined): Condition | undefined {
  return conditions
    ?.slice()
    .sort((a, b) => new Date(b.lastTransitionTime).getTime() - new Date(a.lastTransitionTime).getTime())[0]
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
  const totalSteps = TOTAL_STEPS

  const latestCondition = getLatestCondition(conditions)
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

// Maps a Failed/ValidationFailed migration to the fine-grained (1-9) step it failed at,
// by reusing phaseUtils' condition-based fail-point reasoning (design index 0-5) and
// projecting it onto the same step numbers PHASE_STEPS assigns to the equivalent live phase.
const DESIGN_INDEX_TO_STEP: Record<number, number> = {
  0: PHASE_STEPS[Phase.Pending],
  1: PHASE_STEPS[Phase.Validating],
  2: PHASE_STEPS[Phase.CopyingBlocks],
  3: PHASE_STEPS[Phase.AwaitingCutOverStartTime],
  4: PHASE_STEPS[Phase.ConvertingDisk],
  5: PHASE_STEPS[Phase.Succeeded]
}

function getFailedStepNumber(phase: Phase, conditions: Condition[] = []): number {
  const designIndex = getDesignIndex(phase, conditions)
  return DESIGN_INDEX_TO_STEP[designIndex] ?? PHASE_STEPS[Phase.Validating]
}

// Current step out of TOTAL_STEPS: 0 while queued, TOTAL_STEPS once succeeded, the
// fail-point step for a failed migration, otherwise PHASE_STEPS' step for the live phase.
export function getStepNumber(phase: Phase | undefined, conditions: Condition[] = []): number {
  if (!phase || phase === Phase.Unknown || phase === Phase.Pending) return 0
  if (phase === Phase.Succeeded) return TOTAL_STEPS
  if (phase === Phase.Failed || phase === Phase.ValidationFailed) {
    return getFailedStepNumber(phase, conditions)
  }
  return PHASE_STEPS[phase] ?? 0
}

// Disk-count-based completion percentage (e.g. 1 of 2 disks done -> 50), the only
// quantitative progress signal the backend exposes for copy/convert phases.
function getDiskPercent(currentDisk?: string, totalDisks?: number): number | null {
  if (!currentDisk || !totalDisks) return null
  const parsed = parseInt(currentDisk, 10)
  if (Number.isNaN(parsed)) return null
  return Math.min(100, Math.round((parsed / totalDisks) * 100))
}

// Percent shown in the Progress column. Prefers real disk-count progress for copy/convert
// phases (the only phases the backend reports per-disk counters for); otherwise falls back
// to a step-based estimate so every in-progress row still shows a monotonically increasing
// number instead of fabricating a byte-level rate we have no data for.
export function getStepPercent(
  phase: Phase | undefined,
  stepNumber: number,
  currentDisk?: string,
  totalDisks?: number
): number {
  if (phase === Phase.Succeeded) return 100
  const diskPercent = getDiskPercent(currentDisk, totalDisks)
  if (
    diskPercent !== null &&
    (phase === Phase.CopyingBlocks || phase === Phase.CopyingChangedBlocks || phase === Phase.ConvertingDisk)
  ) {
    return diskPercent
  }
  return Math.round((stepNumber / TOTAL_STEPS) * 100)
}

export type SegmentStatus = 'done' | 'active' | 'ready' | 'failed' | 'pending'

const AWAITING_CUTOVER_PHASES: Phase[] = [Phase.AwaitingCutOverStartTime, Phase.AwaitingAdminCutOver]

// Builds the 9-segment pipeline track state for the Progress column. Segments before the
// current step are 'done'; the current step is 'active' (copy/convert in flight), 'ready'
// (awaiting a human cutover action) or 'failed'; everything after is 'pending'.
export function getSegmentStates(
  phase: Phase | undefined,
  conditions: Condition[] = [],
  totalSteps: number = TOTAL_STEPS
): SegmentStatus[] {
  if (!phase || phase === Phase.Unknown || phase === Phase.Pending) {
    return Array(totalSteps).fill('pending')
  }
  if (phase === Phase.Succeeded) {
    return Array(totalSteps).fill('done')
  }

  const isFailed = phase === Phase.Failed || phase === Phase.ValidationFailed
  const step = isFailed ? getFailedStepNumber(phase, conditions) : (PHASE_STEPS[phase] ?? 0)
  const currentSegmentStatus: SegmentStatus = isFailed
    ? 'failed'
    : AWAITING_CUTOVER_PHASES.includes(phase)
      ? 'ready'
      : 'active'

  return Array.from({ length: totalSteps }, (_, i) => {
    const idx = i + 1
    if (idx < step) return 'done'
    if (idx === step) return currentSegmentStatus
    return 'pending'
  })
}
