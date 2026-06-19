import { Migration, Phase, Condition } from '../api/migrations'
import { calculateTimeElapsed } from 'src/utils'

export type PhaseStatus = 'done' | 'active' | 'pending' | 'failed'

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
  { key: 'pending',    label: 'Pending',        stepLabel: 'Step 1' },
  { key: 'validating', label: 'Validating',     stepLabel: 'Step 2' },
  { key: 'copying',    label: 'Copying Blocks', stepLabel: 'Step 3' },
  { key: 'cutover',    label: 'Cutover',        stepLabel: 'Step 4' },
  { key: 'done',       label: 'Done',           stepLabel: 'Step 5' },
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
    case Phase.ConvertingDisk:
      return 2
    case Phase.AwaitingAdminCutOver:
    case Phase.AwaitingCutOverStartTime:
      return 3
    case Phase.Succeeded:
      return 4
    case Phase.Failed: {
      const validatedOk = conditions.some((c) => c.type === 'Validated' && c.status === 'True')
      const copyStarted = conditions.some((c) => c.type === 'DataCopy')
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

// Extract elapsed duration for a condition type (time from creation to condition transition)
function conditionElapsed(
  creationTimestamp: string | Date | undefined,
  conditions: Condition[],
  type: string
): string | null {
  if (!creationTimestamp) return null
  const cond = conditions.find((c) => c.type === type)
  if (!cond?.lastTransitionTime) return null
  const start = new Date(creationTimestamp).getTime()
  const end = new Date(cond.lastTransitionTime).getTime()
  const diffMs = end - start
  if (diffMs < 0) return null
  const s = Math.floor(diffMs / 1000)
  const m = Math.floor(s / 60)
  const h = Math.floor(m / 60)
  if (h > 0) return `${h}h ${m % 60}m`
  if (m > 0) return `${m}m ${s % 60}s`
  return `${s}s`
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
    case 4: return 'Target VM is healthy.'
    default: return 'Complete.'
  }
}

function activeDetail(migration: Migration, designIndex: number): string {
  const status = migration.status
  switch (designIndex) {
    case 0: return 'Waiting for available agent.'
    case 1: return 'Running pre-flight checks…'
    case 2: {
      if (status?.currentDisk && status?.totalDisks) {
        return `Disk ${status.currentDisk} of ${status.totalDisks}`
      }
      return 'Transferring disk data…'
    }
    case 3: return 'Waiting for admin to initiate cutover.'
    default: return 'In progress…'
  }
}

function pendingDetail(designIndex: number): string {
  switch (designIndex) {
    case 0: return 'Queued.'
    case 1: return 'Will start after agent picks up task.'
    case 2: return 'Will start when validation completes.'
    case 3: return 'Will start when copy completes.'
    case 4: return 'Pending cutover.'
    default: return 'Pending.'
  }
}

function failedDetail(migration: Migration, designIndex: number): string {
  const conditions = migration.status?.conditions || []
  const failCond = conditions.find((c) => c.type === 'Failed')
  if (failCond?.message) return String(failCond.message)
  switch (designIndex) {
    case 1: return 'Validation check failed.'
    case 2: return 'Disk copy failed.'
    case 3: return 'Cutover failed.'
    default: return 'Failed.'
  }
}

/**
 * Maps a Migration CRD to the 5 design-phase states used by MigrationPhaseStepper.
 * Returns an array of exactly 5 PhaseState items.
 */
export function derivePhaseStates(migration: Migration): PhaseState[] {
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

  const currentIndex = getDesignIndex(phase as Phase, conditions)
  const failed = isFailed(phase as Phase)
  const succeeded = phase === Phase.Succeeded

  return DESIGN_PHASE_DEFS.map((_, i): PhaseState => {
    if (succeeded) {
      const elapsed =
        conditionElapsed(creationTs?.toString(), conditions,
          i === 1 ? 'Validated' : i === 2 ? 'DataCopy' : i === 4 ? 'Migrated' : ''
        ) ?? null
      return { status: 'done', elapsed, detail: doneDetail(i, conditions), eta: null }
    }

    if (failed) {
      if (i < currentIndex) {
        return {
          status: 'done',
          elapsed: conditionElapsed(creationTs?.toString(), conditions,
            i === 1 ? 'Validated' : i === 2 ? 'DataCopy' : '') ?? null,
          detail: doneDetail(i, conditions),
          eta: null,
        }
      }
      if (i === currentIndex) {
        const cond = conditions.find((c) => c.type === 'Failed')
        const elapsed = cond?.lastTransitionTime && creationTs
          ? calculateTimeElapsed(creationTs.toString(), migration.status)
          : null
        return { status: 'failed', elapsed, detail: failedDetail(migration, i), eta: null }
      }
      return { status: 'pending', elapsed: null, detail: 'Blocked by failure.', eta: null }
    }

    // Active migration
    if (i < currentIndex) {
      return {
        status: 'done',
        elapsed: conditionElapsed(creationTs?.toString(), conditions,
          i === 1 ? 'Validated' : i === 2 ? 'DataCopy' : '') ?? null,
        detail: doneDetail(i, conditions),
        eta: null,
      }
    }
    if (i === currentIndex) {
      return {
        status: 'active',
        elapsed: creationTs ? calculateTimeElapsed(creationTs.toString(), migration.status) : null,
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
  if (!phase || phase === Phase.Succeeded) return -1
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
  switch (phase) {
    case Phase.Pending:               return 'Pending'
    case Phase.Validating:            return 'Validating'
    case Phase.ValidationFailed:      return 'Validation Failed'
    case Phase.AwaitingDataCopyStart: return 'Awaiting Copy Start'
    case Phase.CopyingBlocks:         return 'Copying Blocks'
    case Phase.CopyingChangedBlocks:  return 'Copying Changed Blocks'
    case Phase.ConvertingDisk:        return 'Converting Disk'
    case Phase.AwaitingAdminCutOver:  return 'Awaiting Admin Cutover'
    case Phase.AwaitingCutOverStartTime: return 'Awaiting Cutover Window'
    case Phase.Succeeded:             return 'Succeeded'
    case Phase.Failed:                return 'Failed'
    case Phase.Unknown:               return 'Unknown'
    default:                          return String(phase ?? 'Unknown')
  }
}

export type PhaseColorKey = 'info' | 'success' | 'error' | 'warning' | 'default'

export function getPhaseColorKey(phase: Phase | string | undefined): PhaseColorKey {
  switch (phase) {
    case Phase.Succeeded:             return 'success'
    case Phase.Failed:
    case Phase.ValidationFailed:      return 'error'
    case Phase.AwaitingAdminCutOver:
    case Phase.AwaitingCutOverStartTime: return 'warning'
    case Phase.Pending:
    case Phase.Unknown:               return 'default'
    default:                          return 'info'
  }
}
