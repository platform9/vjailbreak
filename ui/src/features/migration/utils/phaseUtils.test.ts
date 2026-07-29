import { describe, expect, it, vi } from 'vitest'
import {
  derivePhaseStates,
  getActivePhasIndex,
  getPhaseColorKey,
  getPhaseLabel,
  isMigrationFailed,
} from './phaseUtils'
import { Migration, Phase } from '../api/migrations'

// Timeline: creation at T0, then each condition marks the end of a step.
const T0 = '2026-01-01T00:00:00Z'
const at = (minutes: number) => new Date(`2026-01-01T00:00:00Z`).getTime() + minutes * 60_000

const condition = (type: string, minutesAfterCreation: number, status = 'True') => ({
  type,
  status,
  lastTransitionTime: new Date(at(minutesAfterCreation)).toISOString(),
  message: '',
  reason: ''
})

const buildMigration = (phase: string | undefined, conditions: unknown[]): Migration =>
  ({
    metadata: { creationTimestamp: T0 },
    status: phase ? { phase, conditions } : undefined
  }) as unknown as Migration

const fullConditions = [
  condition('Validated', 2),
  condition('DataCopy', 30),
  condition('Migrating', 40), // set when "Converting disk" fires = cutover complete, conversion starts
  condition('Migrated', 60) // set when "VM created successfully" fires = conversion + VM creation complete
]

describe('derivePhaseStates — succeeded', () => {
  const states = derivePhaseStates(buildMigration(Phase.Succeeded, fullConditions))

  it('marks every step done', () => {
    expect(states.map((s) => s.status)).toEqual(['done', 'done', 'done', 'done', 'done', 'done'])
  })

  it('shows each step\'s own duration, not cumulative time since creation (GH #2195)', () => {
    expect(states[0].elapsed).toBeNull() // Pending: no PodRunning condition (pre-fix migration) -> "-"
    expect(states[1].elapsed).toBe('2m 0s') // Validating: creation -> Validated (falls back since no PodRunning)
    expect(states[2].elapsed).toBe('28m 0s') // CopyingBlocks: Validated -> DataCopy
    expect(states[3].elapsed).toBe('10m 0s') // Cutover: DataCopy -> Migrating
    expect(states[4].elapsed).toBe('20m 0s') // ConvertingDisk: Migrating -> Migrated
    expect(states[5].elapsed).toBe('1h 0m') // Done: total time, creation -> Migrated
  })

  it('shows the converting-disk step with its own distinct duration from cutover', () => {
    expect(states[4].elapsed).not.toBe(states[3].elapsed)
  })
})

describe('derivePhaseStates — Pending/Validating split via PodRunning condition', () => {
  it('measures real Pending wait and real Validating work separately, not bundled together', () => {
    // Pod object existed from creation, but didn't actually start running until t=50
    // (e.g. queued behind another migration on the same agent) - that's the real Pending
    // wait. Validation itself only took 2 more minutes after that.
    const conditions = [
      condition('PodRunning', 50),
      condition('Validated', 52),
      { ...condition('DataCopy', 60), reason: 'Copying disk 0' },
      condition('Migrating', 65),
      condition('Migrated', 70)
    ]
    const states = derivePhaseStates(buildMigration(Phase.Succeeded, conditions))

    expect(states[0].elapsed).toBe('50m 0s') // Pending: creation -> PodRunning
    expect(states[1].elapsed).toBe('2m 0s') // Validating: PodRunning -> Validated, NOT creation -> Validated (52m)
  })

  it('excludes the Pending wait from Done\'s total - "how long the migration actually took", not since object creation', () => {
    const conditions = [
      condition('PodRunning', 50),
      condition('Validated', 52),
      { ...condition('DataCopy', 60), reason: 'Copying disk 0' },
      condition('Migrating', 65),
      condition('Migrated', 70)
    ]
    const states = derivePhaseStates(buildMigration(Phase.Succeeded, conditions))

    expect(states[5].elapsed).toBe('20m 0s') // Done: PodRunning(50) -> Migrated(70), NOT creation(0) -> Migrated(70) (70m)
  })
})

describe('derivePhaseStates — admin-initiated cutover via CutoverTriggered condition', () => {
  it('measures Cutover from when admin actually triggered it, not from DataCopy end', () => {
    // DataCopy finishes at t=13, but admin doesn't click cutover until t=250 (a long
    // "awaiting admin" wait). The real cutover work (VM power-off, final changed-block
    // copy) only takes 1 minute after that, until conversion starts at t=251.
    const conditions = [
      condition('Validated', 2),
      { ...condition('DataCopy', 13), reason: 'Copying disk 0' },
      condition('CutoverTriggered', 250),
      condition('Migrating', 251),
      condition('Migrated', 260)
    ]
    const states = derivePhaseStates(buildMigration(Phase.Succeeded, conditions))

    expect(states[2].elapsed).toBe('11m 0s') // CopyingBlocks: Validated(2) -> DataCopy(13), unaffected
    expect(states[3].elapsed).toBe('1m 0s') // Cutover: CutoverTriggered(250) -> Migrating(251), NOT DataCopy(13) -> Migrating(251) (238m)
  })

  it('falls back to DataCopy end when cutover is not admin-gated (no CutoverTriggered condition)', () => {
    const states = derivePhaseStates(buildMigration(Phase.Succeeded, fullConditions))
    // fullConditions has no CutoverTriggered - same as a cold/automatic-cutover migration
    expect(states[3].elapsed).toBe('10m 0s') // Cutover: DataCopy(30) -> Migrating(40), the existing fallback
  })
})

describe('derivePhaseStates — multi-disk DataCopy conditions', () => {
  it('uses the latest disk-copy completion, not whichever DataCopy entry is first in the array', () => {
    // 'DataCopy' has one condition entry per disk (reason "Copying disk N"); the array's
    // storage order is not guaranteed to be chronological. Disk 0 finishes at t=8, disk 1
    // finishes later at t=13 - here they're stored in chronological (insertion) order, the
    // case that broke the old `.find()`-first-match code (confirmed live on a real
    // migration where this exact ordering understated Copying Blocks by 5+ minutes).
    const conditions = [
      condition('Validated', 2),
      { ...condition('DataCopy', 8), reason: 'Copying disk 0' },
      { ...condition('DataCopy', 13), reason: 'Copying disk 1' },
      condition('Migrating', 14)
    ]
    const states = derivePhaseStates(buildMigration(Phase.ConvertingDisk, conditions))

    expect(states[2].elapsed).toBe('11m 0s') // CopyingBlocks: Validated(2) -> latest DataCopy(13), not the first entry(8)
    expect(states[3].elapsed).toBe('1m 0s') // Cutover: latest DataCopy(13) -> Migrating(14)
  })
})

describe('derivePhaseStates — succeeded DataOnly migration', () => {
  it('bounds the converting-disk step with DataCopied instead of Migrated, and skips cutover', () => {
    const dataOnlyConditions = [
      condition('Validated', 2),
      condition('DataCopy', 30),
      condition('Migrating', 40),
      condition('DataCopied', 50)
    ]
    const migration = {
      metadata: { creationTimestamp: T0 },
      spec: { dataOnly: true },
      status: { phase: Phase.DataCopied, conditions: dataOnlyConditions }
    } as unknown as Migration
    const states = derivePhaseStates(migration)

    expect(states[3].status).toBe('pending') // cutover skipped for DataOnly
    expect(states[4].elapsed).toBe('10m 0s') // ConvertingDisk: Migrating -> DataCopied
    expect(states[5].elapsed).toBe('50m 0s') // Done: total time, creation -> DataCopied
  })
})

describe('derivePhaseStates — active migration', () => {
  it('shows elapsed for the completed cutover step while converting', () => {
    const states = derivePhaseStates(buildMigration(Phase.ConvertingDisk, fullConditions))

    expect(states[3].status).toBe('done')
    // Regression: cutover previously mapped to '' and always showed null.
    expect(states[3].elapsed).toBe('10m 0s') // DataCopy -> Migrating
    expect(states[1].elapsed).toBe('2m 0s')
    expect(states[2].elapsed).toBe('28m 0s') // Validated -> DataCopy
    expect(states[4].status).toBe('active')
    expect(states[5].status).toBe('pending')
  })

  it('does not reveal disk count while converting (GHI #2194)', () => {
    const migration = {
      metadata: { creationTimestamp: T0 },
      status: { phase: Phase.ConvertingDisk, conditions: fullConditions, currentDisk: 1, totalDisks: 2 }
    } as unknown as Migration
    const states = derivePhaseStates(migration)

    expect(states[4].detail).toBe('Converting disk format…')
  })

  it('pauses at cutover while awaiting admin', () => {
    const states = derivePhaseStates(
      buildMigration(Phase.AwaitingAdminCutOver, [condition('Validated', 2), condition('DataCopy', 30)])
    )

    expect(states[3].status).toBe('paused')
    expect(states[4].status).toBe('pending')
  })

  it('shows time elapsed since data copy finished while paused awaiting admin, not cumulative since creation', () => {
    vi.useFakeTimers()
    try {
      vi.setSystemTime(new Date(at(45))) // "now" = 45 minutes after creation
      const states = derivePhaseStates(
        buildMigration(Phase.AwaitingAdminCutOver, [condition('Validated', 2), { ...condition('DataCopy', 30), reason: 'Copying disk 0' }])
      )

      expect(states[3].status).toBe('paused')
      expect(states[3].elapsed).toBe('15m 0s') // DataCopy(30) -> now(45): how long cutover has been waiting, NOT creation(0) -> now (45m)
    } finally {
      vi.useRealTimers()
    }
  })

  it('treats cutoverTriggered as active, not paused', () => {
    const states = derivePhaseStates(
      buildMigration(Phase.AwaitingAdminCutOver, [condition('Validated', 2)]),
      { cutoverTriggered: true }
    )

    expect(states[3].status).toBe('active')
  })

  it('measures the currently-active step from its own start boundary, not cumulative since creation', () => {
    vi.useFakeTimers()
    try {
      vi.setSystemTime(new Date(at(45))) // "now" = 45 minutes after creation
      const conditions = [
        condition('Validated', 2),
        { ...condition('DataCopy', 30), reason: 'Copying disk 0' },
        condition('Migrating', 40)
      ]
      const states = derivePhaseStates(buildMigration(Phase.ConvertingDisk, conditions))

      expect(states[4].status).toBe('active')
      expect(states[4].elapsed).toBe('5m 0s') // ConvertingDisk: Migrating(40) -> now(45), not creation(0) -> now(45) (45m)
    } finally {
      vi.useRealTimers()
    }
  })
})

describe('derivePhaseStates — failed migration', () => {
  it('fails at conversion when Migrating condition is set', () => {
    const conditions = [...fullConditions.slice(0, 3), condition('Failed', 45)]
    const states = derivePhaseStates(buildMigration(Phase.Failed, conditions))

    expect(states[3].status).toBe('done')
    expect(states[3].elapsed).toBe('10m 0s') // DataCopy -> Migrating
    expect(states[4].status).toBe('failed')
    expect(states[4].elapsed).toBe('5m 0s') // ConvertingDisk: Migrating(40) -> Failed(45), not creation(0) -> Failed(45) (45m)
    expect(states[5].status).toBe('pending')
  })

  it('falls back to copy step when conversion never started', () => {
    const conditions = [condition('Validated', 2), condition('DataCopy', 30), condition('Failed', 35)]
    const states = derivePhaseStates(buildMigration(Phase.Failed, conditions))

    expect(states[2].status).toBe('failed')
    expect(states[1].status).toBe('done')
    expect(states[1].elapsed).toBe('2m 0s')
  })
})

describe('derivePhaseStates — no phase yet', () => {
  it('activates the first step and leaves the rest pending', () => {
    const states = derivePhaseStates(buildMigration(undefined, []))

    expect(states[0].status).toBe('active')
    expect(states.slice(1).every((s) => s.status === 'pending')).toBe(true)
  })

  it('shows live elapsed time for the queued Pending step, not a blank dash', () => {
    vi.useFakeTimers()
    try {
      vi.setSystemTime(new Date(at(29))) // "now" = 29 minutes after creation
      const states = derivePhaseStates(buildMigration(undefined, []))

      expect(states[0].elapsed).toBe('29m 0s') // creation(0) -> now(29), same as a running migration's active step
    } finally {
      vi.useRealTimers()
    }
  })
})

describe('getPhaseLabel / getPhaseColorKey', () => {
  it('reads a falsy phase as Pending, not Unknown (backend never assigns Phase.Unknown)', () => {
    expect(getPhaseLabel(undefined)).toBe('Pending')
    expect(getPhaseLabel('')).toBe('Pending')
    expect(getPhaseColorKey(undefined)).toBe('default')
    expect(getPhaseColorKey('')).toBe('default')
  })

  it('still surfaces the real Phase.Unknown value distinctly from a missing phase', () => {
    expect(getPhaseLabel(Phase.Unknown)).toBe('Unknown')
    expect(getPhaseColorKey(Phase.Unknown)).toBe('default')
  })

  it('maps known phases to their labels and colors', () => {
    expect(getPhaseLabel(Phase.Pending)).toBe('Pending')
    expect(getPhaseLabel(Phase.Succeeded)).toBe('Succeeded')
    expect(getPhaseColorKey(Phase.Succeeded)).toBe('success')
    expect(getPhaseColorKey(Phase.Failed)).toBe('error')
  })

  it('falls back to showing an unrecognized non-empty phase string as-is', () => {
    expect(getPhaseLabel('SomeFuturePhase')).toBe('SomeFuturePhase')
    expect(getPhaseColorKey('SomeFuturePhase')).toBe('info')
  })
})

describe('helpers', () => {
  it('getActivePhasIndex returns -1 for succeeded', () => {
    expect(getActivePhasIndex(buildMigration(Phase.Succeeded, fullConditions))).toBe(-1)
  })

  it('isMigrationFailed covers Failed and ValidationFailed', () => {
    expect(isMigrationFailed(buildMigration(Phase.Failed, []))).toBe(true)
    expect(isMigrationFailed(buildMigration(Phase.ValidationFailed, []))).toBe(true)
    expect(isMigrationFailed(buildMigration(Phase.CopyingBlocks, []))).toBe(false)
  })
})
