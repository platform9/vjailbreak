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
  if (phase === Phase.Succeeded) return 'succeeded'
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

  if (phase === Phase.Failed || phase === Phase.ValidationFailed || phase === Phase.Succeeded) {
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

export type ProgressBarColor = 'primary' | 'success' | 'warning' | 'error' | 'neutral'

export interface ProgressDisplay {
  primaryText: string
  secondaryText?: string
  barValue: number
  barVariant: 'determinate' | 'indeterminate'
  barColor: ProgressBarColor
}

// Human label for phases that don't otherwise get a disk-count-aware label below.
// Only phases with a real, phase-specific story to tell get an entry here — anything
// missing falls back to the raw phase name rather than inventing wording for it.
const PHASE_LABEL: Partial<Record<Phase, string>> = {
  [Phase.AwaitingDataCopyStart]: 'Preparing to copy data',
  [Phase.SnapshottingSourceVM]: 'Creating VM snapshot',
  [Phase.AttachingDisksToProxy]: 'Attaching disks to proxy',
  [Phase.IdentifyingBlockDevices]: 'Identifying block devices',
  [Phase.HotAddTransferInProgress]: 'Transferring via HotAdd',
  [Phase.HotAddCleanup]: 'Cleaning up HotAdd',
  [Phase.ConvertingDisk]: 'Converting disk format',
  [Phase.CopyingChangedBlocks]: 'Final changed-block sync'
}

// Drives the Migrations table's Progress column: a label line (with a disk-count/percent
// meta on the right where that's real, known data) plus a colored progress bar. Every
// value here is derived from actual status fields — no fabricated ETAs, throughput, or
// error codes the API doesn't provide.
export function deriveProgressDisplay(
  phase: Phase | undefined,
  conditions: Condition[] | undefined,
  currentDisk: string | undefined,
  totalDisks: number | undefined,
  syncWarningMessage?: string
): ProgressDisplay {
  const latestCondition = conditions
    ?.slice()
    .sort((a, b) => new Date(b.lastTransitionTime).getTime() - new Date(a.lastTransitionTime).getTime())[0]

  const diskNum = currentDisk != null ? parseInt(currentDisk, 10) : null
  const diskProgress =
    diskNum !== null && !Number.isNaN(diskNum) && totalDisks
      ? Math.round((diskNum / totalDisks) * 100)
      : null

  const display = ((): ProgressDisplay => {
    switch (phase) {
      case Phase.Succeeded:
        return { primaryText: 'Complete', barValue: 100, barVariant: 'determinate', barColor: 'success' }

      case Phase.Failed:
      case Phase.ValidationFailed:
        return {
          primaryText: latestCondition?.message || `${phase} migration`,
          barValue: diskProgress ?? 100,
          barVariant: 'determinate',
          barColor: 'error'
        }

      case Phase.Pending:
        return {
          primaryText: 'Queued',
          secondaryText: 'In queue',
          barValue: 0,
          barVariant: 'determinate',
          barColor: 'neutral'
        }

      case Phase.AwaitingCutOverStartTime:
        return {
          primaryText: 'Data copy complete',
          secondaryText: 'Awaiting cutover window',
          barValue: 100,
          barVariant: 'determinate',
          barColor: 'warning'
        }

      case Phase.AwaitingAdminCutOver:
        return {
          primaryText: 'Data copy complete',
          secondaryText: 'Awaiting admin cutover',
          barValue: 100,
          barVariant: 'determinate',
          barColor: 'warning'
        }

      case Phase.Validating:
        return {
          primaryText: latestCondition?.message || 'Running pre-flight checks',
          barValue: 0,
          barVariant: 'indeterminate',
          barColor: 'primary'
        }

      case Phase.CopyingBlocks: {
        const primaryText =
          diskNum !== null && totalDisks ? `Disk ${diskNum + 1} of ${totalDisks}` : 'Copying blocks'
        return {
          primaryText,
          secondaryText: diskProgress !== null ? `${diskProgress}%` : undefined,
          barValue: diskProgress ?? 0,
          barVariant: diskProgress !== null ? 'determinate' : 'indeterminate',
          barColor: 'primary'
        }
      }

      case Phase.AwaitingDataCopyStart:
      case Phase.SnapshottingSourceVM:
      case Phase.AttachingDisksToProxy:
      case Phase.IdentifyingBlockDevices:
      case Phase.HotAddTransferInProgress:
      case Phase.HotAddCleanup:
      case Phase.ConvertingDisk:
      case Phase.CopyingChangedBlocks:
        return {
          primaryText: PHASE_LABEL[phase] || phase,
          secondaryText: diskProgress !== null ? `${diskProgress}%` : undefined,
          barValue: diskProgress ?? 0,
          barVariant: diskProgress !== null ? 'determinate' : 'indeterminate',
          barColor: 'primary'
        }

      default:
        return {
          primaryText: phase || 'Unknown',
          barValue: 0,
          barVariant: 'indeterminate',
          barColor: 'neutral'
        }
    }
  })()

  // A sync warning can surface mid-copy without the migration having failed — flag it
  // on the bar without touching the label/percent that's already showing real progress.
  if (syncWarningMessage && phase !== Phase.Failed && phase !== Phase.ValidationFailed) {
    return { ...display, barColor: 'warning' }
  }

  return display
}
