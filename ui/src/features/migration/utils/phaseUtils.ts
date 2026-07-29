import { Migration, Phase, Condition } from '../api/migrations'

export type PhaseStatus = 'done' | 'active' | 'paused' | 'pending' | 'failed'

export interface PhaseState {
  status: PhaseStatus
  elapsed: string | null
  detail: string
  eta: string | null
}

export interface DesignPhaseDef {
  key: string
  label: string
  stepLabel: string
}

export const DESIGN_PHASE_DEFS: DesignPhaseDef[] = [
  { key: 'pending',    label: 'Pending',         stepLabel: 'Step 1' },
  { key: 'validating', label: 'Validating',      stepLabel: 'Step 2' },
  { key: 'copying',    label: 'Copying Blocks',  stepLabel: 'Step 3' },
  { key: 'cutover',    label: 'Cutover',         stepLabel: 'Step 4' },
  { key: 'converting', label: 'Converting Disk', stepLabel: 'Step 5' },
  { key: 'done',       label: 'Done',            stepLabel: 'Step 6' },
]

// K8s Phase → design phase index (0–4)
function getDesignIndex(phase: Phase, conditions: Condition[]): number {
  switch (phase) {
    case Phase.Pending:
    case Phase.AwaitingDataCopyStart:
      return 0
    case Phase.Validating:
    case Phase.ValidationFailed:
      return 1
    case Phase.CopyingBlocks:
    case Phase.CopyingChangedBlocks:
    case Phase.SnapshottingSourceVM:
    case Phase.AttachingDisksToProxy:
    case Phase.IdentifyingBlockDevices:
    case Phase.HotAddTransferInProgress:
    case Phase.HotAddCleanup:
      return 2
    case Phase.AwaitingAdminCutOver:
    case Phase.AwaitingCutOverStartTime:
      return 3
    case Phase.ConvertingDisk:
      return 4
    case Phase.Succeeded:
    case Phase.DataCopied:
      return 5
    case Phase.Failed: {
      const validatedOk = conditions.some((c) => c.type === 'Validated' && c.status === 'True')
      const copyStarted = conditions.some((c) => c.type === 'DataCopy')
      // 'Migrating' condition is set when "Converting disk" event fires (first line of ConvertVolumes).
      // This is the only reliable signal that conversion started, for both regular and HotAdd migrations
      // (HotAdd doesn't fire "Copying disk" events, so DataCopy condition is never set for HotAdd).
      const convertStarted = conditions.some((c) => c.type === 'Migrating' && c.status === 'True')
      if (convertStarted) return 4
      if (copyStarted || validatedOk) return 2
      return 1
    }
    default:
      return 0
  }
}

function isFailed(phase: Phase): boolean {
  return phase === Phase.Failed || phase === Phase.ValidationFailed
}

function formatDuration(diffMs: number): string | null {
  if (diffMs < 0) return null
  const s = Math.floor(diffMs / 1000)
  const m = Math.floor(s / 60)
  const h = Math.floor(m / 60)
  if (h > 0) return `${h}h ${m % 60}m`
  if (m > 0) return `${m}m ${s % 60}s`
  return `${s}s`
}

export function durationBetween(
  startTimestamp: string | Date | undefined,
  endTimestamp: string | Date | undefined
): string | null {
  if (!startTimestamp || !endTimestamp) return null
  const start = new Date(startTimestamp).getTime()
  const end = new Date(endTimestamp).getTime()
  return formatDuration(end - start)
}

// Extract elapsed duration for a condition type (time from creation to condition transition).
// Used for the terminal "Done" step, which intentionally shows total migration time rather
// than a single step's own duration.
function conditionElapsed(
  creationTimestamp: string | Date | undefined,
  conditions: Condition[],
  type: string
): string | null {
  if (!creationTimestamp) return null
  const cond = conditions.find((c) => c.type === type)
  return durationBetween(creationTimestamp, cond?.lastTransitionTime)
}

// The condition whose lastTransitionTime marks the END of each design-phase step (0-4).
// Step 0 (Pending) ends on 'PodRunning' - read from the pod's own Status.StartTime - so
// Pending's real wait (queued for agent capacity) can be measured separately from
// Validating's real work, instead of Pending always showing nothing and Validating silently
// absorbing the wait. Step 4 (Converting Disk) ends on 'Migrated' (or 'DataCopied' for
// DataOnly migrations) - the next condition to fire after 'Migrating' - rather than reusing
// 'Migrating' itself (which only marks when conversion *started*).
function stepEndType(step: number, dataOnly: boolean): string | undefined {
  switch (step) {
    case 0: return 'PodRunning'
    case 1: return 'Validated'
    case 2: return 'DataCopy'
    case 3: return 'Migrating'
    case 4: return dataOnly ? 'DataCopied' : 'Migrated'
    default: return undefined
  }
}

// A condition type can have more than one entry - e.g. 'DataCopy' gets one entry per disk
// (matched by (type, reason) with reason "Copying disk N"), and the array's storage order
// is not guaranteed to be chronological. Taking the max lastTransitionTime across all
// matching entries (rather than the first array match) ensures a multi-disk copy's end
// boundary is really the last disk to finish, not whichever disk happened to be recorded
// first.
function stepEndTimestamp(conditions: Condition[], step: number, dataOnly: boolean): string | undefined {
  const type = stepEndType(step, dataOnly)
  if (!type) return undefined
  const matches = conditions.filter((c) => c.type === type && c.lastTransitionTime)
  if (matches.length === 0) return undefined
  const latest = matches.reduce((a, b) =>
    new Date(String(a.lastTransitionTime)).getTime() > new Date(String(b.lastTransitionTime)).getTime() ? a : b
  )
  return String(latest.lastTransitionTime)
}

function stepStartTimestamp(
  step: number,
  creationTimestamp: string | Date | undefined,
  conditions: Condition[],
  dataOnly: boolean
): string | undefined {
  if (step <= 0) return creationTimestamp?.toString()
  if (step === 3) {
    // Cutover's real start is when an admin actually triggered it, not when data copy
    // finished - there can be an arbitrary "awaiting admin" wait in between. Falls back to
    // DataCopy's end for migrations where cutover isn't admin-gated (no CutoverTriggered
    // condition ever fires there), or for migrations from before this condition existed.
    const triggered = conditions.find((c) => c.type === 'CutoverTriggered')
    if (triggered?.lastTransitionTime) return String(triggered.lastTransitionTime)
  }
  return stepEndTimestamp(conditions, step - 1, dataOnly) ?? creationTimestamp?.toString()
}

// Duration of a single design-phase step: end-of-this-step minus end-of-previous-step
// (or migration creation time for the first tracked step), not cumulative-since-creation.
function stepElapsed(
  step: number,
  creationTimestamp: string | Date | undefined,
  conditions: Condition[],
  dataOnly: boolean
): string | null {
  const end = stepEndTimestamp(conditions, step, dataOnly)
  const start = stepStartTimestamp(step, creationTimestamp, conditions, dataOnly)
  return durationBetween(start, end)
}

// Duration of the step currently in progress (active/paused), or the step where a failure
// occurred: from this step's own start boundary to `until` (now, or the failure time) - not
// cumulative since the whole migration began. Mirrors stepElapsed's start-boundary logic but
// takes an explicit end instead of a condition, since the step isn't done yet.
function stepElapsedUntil(
  step: number,
  creationTimestamp: string | Date | undefined,
  conditions: Condition[],
  dataOnly: boolean,
  until: string
): string | null {
  const start = stepStartTimestamp(step, creationTimestamp, conditions, dataOnly)
  return durationBetween(start, until)
}

function doneDetail(designIndex: number, conditions: Condition[]): string {
  switch (designIndex) {
    case 0: return 'Picked up by agent.'
    case 1: {
      const c = conditions.find((c) => c.type === 'Validated')
      return c?.message ? String(c.message) : 'All checks passed.'
    }
    case 2: {
      const c = conditions.find((c) => c.type === 'DataCopy')
      return c?.message ? String(c.message) : 'Disk transfer complete.'
    }
    case 3: return 'Cutover complete.'
    case 4: return 'Disk conversion complete.'
    case 5: return 'Migration completed successfully.'
    default: return 'Complete.'
  }
}

function activeDetail(migration: Migration, designIndex: number): string {
  const status = migration.status
  switch (designIndex) {
    case 0: return 'Waiting for available agent.'
    case 1: return 'Running pre-flight checks…'
    case 2: {
      if (status?.currentDisk != null && status?.totalDisks) {
        const diskNum = parseInt(String(status.currentDisk), 10)
        const display = Number.isNaN(diskNum) ? status.currentDisk : diskNum + 1
        return `Disk ${display} of ${status.totalDisks}`
      }
      return 'Transferring disk data…'
    }
    case 3: return 'Waiting for admin to initiate cutover.'
    case 4: return 'Converting disk.'
    default: return 'In progress…'
  }
}

function pendingDetail(designIndex: number): string {
  switch (designIndex) {
    case 0: return 'Queued.'
    case 1: return 'Will start after agent picks up task.'
    case 2: return 'Will start when validation completes.'
    case 3: return 'Will start when copy completes.'
    case 4: return 'Will start after cutover.'
    case 5: return 'Pending.'
    default: return 'Pending.'
  }
}

function failedDetail(_migration: Migration, designIndex: number): string {
  switch (designIndex) {
    case 1: return 'Validation check failed. See error details below.'
    case 2: return 'Disk copy failed. See error details below.'
    case 3: return 'Cutover failed. See error details below.'
    case 4: return 'Disk conversion failed. See error details below.'
    default: return 'Migration halted. See error details below.'
  }
}

/**
 * Maps a Migration CRD to the design-phase states used by MigrationPhaseStepper.
 * Returns an array of PhaseState items matching DESIGN_PHASE_DEFS length.
 * @param options.minDesignIndex - clamp the active index to at least this value (used to
 *   prevent visual step-back during final delta sync after admin cutover is triggered)
 */
export function derivePhaseStates(
  migration: Migration,
  options?: { minDesignIndex?: number; cutoverTriggered?: boolean }
): PhaseState[] {
  const phase = migration.status?.phase
  const conditions = migration.status?.conditions || []
  const creationTs = migration.metadata?.creationTimestamp

  if (!phase) {
    return DESIGN_PHASE_DEFS.map((_, i) =>
      i === 0
        ? { status: 'active', elapsed: null, detail: 'Queued for agent.', eta: null }
        : { status: 'pending', elapsed: null, detail: pendingDetail(i), eta: null }
    )
  }

  const rawIndex = getDesignIndex(phase as Phase, conditions)
  const currentIndex = options?.minDesignIndex != null
    ? Math.max(rawIndex, options.minDesignIndex)
    : rawIndex
  const failed = isFailed(phase as Phase)
  const succeeded = phase === Phase.Succeeded || phase === Phase.DataCopied
  const dataOnly = migration.spec?.dataOnly === true

  return DESIGN_PHASE_DEFS.map((_, i): PhaseState => {
    if (succeeded) {
      // DataOnly migrations skip cutover: step 3 was never executed.
      if (dataOnly && i === 3) {
        return { status: 'pending', elapsed: null, detail: 'Skipped — no cutover in data-only migration.', eta: null }
      }
      const elapsed = i >= 0 && i <= 4
        ? stepElapsed(i, creationTs, conditions, dataOnly)
        : i === 5
          ? conditionElapsed(creationTs?.toString(), conditions, dataOnly ? 'DataCopied' : 'Migrated')
          : null
      const detail = (dataOnly && i === 5) ? 'Disk copy and conversion complete.' : doneDetail(i, conditions)
      return { status: 'done', elapsed, detail, eta: null }
    }

    if (failed) {
      if (i < currentIndex) {
        return {
          status: 'done',
          elapsed: stepElapsed(i, creationTs, conditions, dataOnly),
          detail: doneDetail(i, conditions),
          eta: null,
        }
      }
      if (i === currentIndex) {
        const cond = conditions.find((c) => c.type === 'Failed')
        const elapsed = cond?.lastTransitionTime
          ? stepElapsedUntil(i, creationTs, conditions, dataOnly, String(cond.lastTransitionTime))
          : null
        return { status: 'failed', elapsed, detail: failedDetail(migration, i), eta: null }
      }
      return { status: 'pending', elapsed: null, detail: 'Blocked by failure.', eta: null }
    }

    // Active migration
    if (i < currentIndex) {
      return {
        status: 'done',
        elapsed: stepElapsed(i, creationTs, conditions, dataOnly),
        detail: doneDetail(i, conditions),
        eta: null,
      }
    }
    if (i === currentIndex) {
      const isPaused =
        (phase === Phase.AwaitingAdminCutOver || phase === Phase.AwaitingCutOverStartTime) &&
        !options?.cutoverTriggered
      const now = new Date().toISOString()
      // While paused awaiting admin cutover, keep showing total time elapsed since the
      // migration began (not just since data copy finished) - this is the "how long has
      // this whole migration been running" figure users expect to watch tick up while
      // waiting, and it's what's been shown here all along. Once cutover is actually
      // triggered (no longer paused), switch to this step's own elapsed like every other
      // active step.
      const elapsed = isPaused
        ? durationBetween(creationTs?.toString(), now)
        : stepElapsedUntil(i, creationTs, conditions, dataOnly, now)
      return {
        status: isPaused ? 'paused' : 'active',
        elapsed,
        detail: activeDetail(migration, i),
        eta: null,
      }
    }
    return { status: 'pending', elapsed: null, detail: pendingDetail(i), eta: null }
  })
}

/**
 * Returns the index of the currently active or failed design phase.
 * Returns -1 for succeeded migrations.
 */
export function getActivePhasIndex(migration: Migration): number {
  const phase = migration.status?.phase
  if (!phase || phase === Phase.Succeeded || phase === Phase.DataCopied) return -1
  const conditions = migration.status?.conditions || []
  return getDesignIndex(phase as Phase, conditions)
}

/**
 * Returns true if the migration is in a terminal failed state.
 */
export function isMigrationFailed(migration: Migration): boolean {
  const phase = migration.status?.phase
  return !!phase && isFailed(phase as Phase)
}

/**
 * Maps K8s Phase to a human-readable status label and semantic color key.
 */
export function getPhaseLabel(phase: Phase | string | undefined): string {
  // A Migration's Status.Phase is set to "Pending" the instant it's created (before any
  // worker pod exists) - the backend never actually assigns Phase.Unknown. A falsy phase
  // here just means the frontend hasn't received status yet, not a distinct backend state,
  // so it should read the same as a fresh migration: "Pending".
  if (!phase) return 'Pending'
  switch (phase) {
    case Phase.Pending:               return 'Pending'
    case Phase.Validating:            return 'Validating'
    case Phase.ValidationFailed:      return 'Validation Failed'
    case Phase.AwaitingDataCopyStart: return 'Awaiting Copy Start'
    case Phase.CopyingBlocks:             return 'Copying Blocks'
    case Phase.CopyingChangedBlocks:      return 'Copying Changed Blocks'
    case Phase.SnapshottingSourceVM:      return 'Snapshotting Source VM'
    case Phase.AttachingDisksToProxy:     return 'Attaching Disks to Proxy'
    case Phase.IdentifyingBlockDevices:   return 'Identifying Block Devices'
    case Phase.HotAddTransferInProgress:  return 'HotAdd Transfer In Progress'
    case Phase.HotAddCleanup:             return 'HotAdd Cleanup'
    case Phase.ConvertingDisk:            return 'Converting Disk'
    case Phase.AwaitingAdminCutOver:  return 'Awaiting Admin Cutover'
    case Phase.AwaitingCutOverStartTime: return 'Awaiting Cutover Window'
    case Phase.Succeeded:             return 'Succeeded'
    case Phase.DataCopied:            return 'Data Copied'
    case Phase.Failed:                return 'Failed'
    case Phase.Unknown:               return 'Unknown'
    default:                          return String(phase)
  }
}

export type PhaseColorKey = 'info' | 'success' | 'error' | 'warning' | 'default'

export function getPhaseColorKey(phase: Phase | string | undefined): PhaseColorKey {
  if (!phase) return 'default'
  switch (phase) {
    case Phase.Succeeded:
    case Phase.DataCopied:            return 'success'
    case Phase.Failed:
    case Phase.ValidationFailed:      return 'error'
    case Phase.AwaitingAdminCutOver:
    case Phase.AwaitingCutOverStartTime: return 'warning'
    case Phase.Pending:
    case Phase.Unknown:               return 'default'
    default:                          return 'info'
  }
}
