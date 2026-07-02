import { describe, expect, it } from 'vitest'
import { derivePhaseStates, getActivePhasIndex, isMigrationFailed } from './phaseUtils'
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
  condition('Migrating', 40), // set when "Converting disk" fires = cutover complete
  condition('Migrated', 60)
]

describe('derivePhaseStates — succeeded', () => {
  const states = derivePhaseStates(buildMigration(Phase.Succeeded, fullConditions))

  it('marks every step done', () => {
    expect(states.map((s) => s.status)).toEqual(['done', 'done', 'done', 'done', 'done', 'done'])
  })

  it('maps each step to its completion condition', () => {
    expect(states[1].elapsed).toBe('2m 0s') // Validated
    expect(states[2].elapsed).toBe('30m 0s') // DataCopy
    expect(states[3].elapsed).toBe('40m 0s') // Migrating = conversion started = cutover done
    // Step 4 (converting) also uses Migrating — no condition marks the end of
    // conversion, so its start is the closest available signal (PR #2092 review).
    expect(states[4].elapsed).toBe('40m 0s')
    expect(states[5].elapsed).toBe('1h 0m') // Migrated
  })

  it('shows the done step completing after conversion started', () => {
    expect(states[5].elapsed).not.toBe(states[4].elapsed)
  })
})

describe('derivePhaseStates — active migration', () => {
  it('shows elapsed for the completed cutover step while converting', () => {
    const states = derivePhaseStates(buildMigration(Phase.ConvertingDisk, fullConditions))

    expect(states[3].status).toBe('done')
    // Regression: cutover previously mapped to '' and always showed null.
    expect(states[3].elapsed).toBe('40m 0s')
    expect(states[1].elapsed).toBe('2m 0s')
    expect(states[2].elapsed).toBe('30m 0s')
    expect(states[4].status).toBe('active')
    expect(states[5].status).toBe('pending')
  })

  it('pauses at cutover while awaiting admin', () => {
    const states = derivePhaseStates(
      buildMigration(Phase.AwaitingAdminCutOver, [condition('Validated', 2), condition('DataCopy', 30)])
    )

    expect(states[3].status).toBe('paused')
    expect(states[4].status).toBe('pending')
  })

  it('treats cutoverTriggered as active, not paused', () => {
    const states = derivePhaseStates(
      buildMigration(Phase.AwaitingAdminCutOver, [condition('Validated', 2)]),
      { cutoverTriggered: true }
    )

    expect(states[3].status).toBe('active')
  })
})

describe('derivePhaseStates — failed migration', () => {
  it('fails at conversion when Migrating condition is set', () => {
    const conditions = [...fullConditions.slice(0, 3), condition('Failed', 45)]
    const states = derivePhaseStates(buildMigration(Phase.Failed, conditions))

    expect(states[3].status).toBe('done')
    expect(states[3].elapsed).toBe('40m 0s') // Migrating
    expect(states[4].status).toBe('failed')
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
