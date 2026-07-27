import { describe, expect, it } from 'vitest'
import { Condition, Phase } from '../api/migrations'
import {
  getLatestCondition,
  getSegmentStates,
  getStepNumber,
  getStepPercent,
  matchesDateFilter,
  TOTAL_STEPS
} from './migrationTableUtils'

const condition = (type: string, minutesAfterEpoch: number, status = 'True'): Condition =>
  ({
    type,
    status,
    lastTransitionTime: new Date(minutesAfterEpoch * 60_000).toISOString(),
    message: '',
    reason: ''
  }) as unknown as Condition

const NOW = new Date('2026-01-31T12:00:00Z').getTime()
const hoursAgo = (h: number) => new Date(NOW - h * 60 * 60 * 1000).toISOString()

describe('matchesDateFilter', () => {
  it('matches everything for "All Time"', () => {
    expect(matchesDateFilter(hoursAgo(10000), 'All Time', NOW)).toBe(true)
    expect(matchesDateFilter(undefined, 'All Time', NOW)).toBe(true)
  })

  it('matches everything for an unrecognized filter', () => {
    expect(matchesDateFilter(hoursAgo(10000), 'nonsense', NOW)).toBe(true)
  })

  it('includes items within the "Last 24 hours" window', () => {
    expect(matchesDateFilter(hoursAgo(1), 'Last 24 hours', NOW)).toBe(true)
    expect(matchesDateFilter(hoursAgo(24), 'Last 24 hours', NOW)).toBe(true)
  })

  it('excludes items outside the "Last 24 hours" window', () => {
    expect(matchesDateFilter(hoursAgo(25), 'Last 24 hours', NOW)).toBe(false)
  })

  it('includes items within the "Last 7 days" window and excludes items outside it', () => {
    expect(matchesDateFilter(hoursAgo(24 * 6), 'Last 7 days', NOW)).toBe(true)
    expect(matchesDateFilter(hoursAgo(24 * 8), 'Last 7 days', NOW)).toBe(false)
  })

  it('includes items within the "Last 30 days" window and excludes items outside it', () => {
    expect(matchesDateFilter(hoursAgo(24 * 29), 'Last 30 days', NOW)).toBe(true)
    expect(matchesDateFilter(hoursAgo(24 * 31), 'Last 30 days', NOW)).toBe(false)
  })

  it('treats a missing creation timestamp as not matching a specific window', () => {
    expect(matchesDateFilter(undefined, 'Last 24 hours', NOW)).toBe(false)
  })

  it('treats an unparsable creation timestamp as not matching a specific window', () => {
    expect(matchesDateFilter('not-a-date', 'Last 24 hours', NOW)).toBe(false)
  })
})

describe('getLatestCondition', () => {
  it('returns the condition with the most recent transition time', () => {
    const conditions = [condition('Validated', 2), condition('DataCopy', 30), condition('Migrating', 10)]
    expect(getLatestCondition(conditions)?.type).toBe('DataCopy')
  })

  it('returns undefined for missing/empty conditions', () => {
    expect(getLatestCondition(undefined)).toBeUndefined()
    expect(getLatestCondition([])).toBeUndefined()
  })
})

describe('getStepNumber', () => {
  it('returns 0 for Pending, undefined, and Unknown', () => {
    expect(getStepNumber(Phase.Pending, [])).toBe(0)
    expect(getStepNumber(undefined, [])).toBe(0)
    expect(getStepNumber(Phase.Unknown, [])).toBe(0)
  })

  it('returns TOTAL_STEPS for Succeeded', () => {
    expect(getStepNumber(Phase.Succeeded, [])).toBe(TOTAL_STEPS)
  })

  it('maps live phases to their PHASE_STEPS value', () => {
    expect(getStepNumber(Phase.CopyingBlocks, [])).toBe(4)
    expect(getStepNumber(Phase.ConvertingDisk, [])).toBe(6)
    expect(getStepNumber(Phase.AwaitingCutOverStartTime, [])).toBe(7)
  })

  describe('for Failed/ValidationFailed, derives the fail point from conditions', () => {
    it('falls back to the validation step when nothing progressed', () => {
      expect(getStepNumber(Phase.ValidationFailed, [])).toBe(2)
    })

    it('reports the copy step once validation passed or copy started', () => {
      expect(getStepNumber(Phase.Failed, [condition('Validated', 1)])).toBe(4)
      expect(getStepNumber(Phase.Failed, [condition('DataCopy', 1, 'Unknown')])).toBe(4)
    })

    it('reports the convert step once conversion started', () => {
      expect(getStepNumber(Phase.Failed, [condition('Migrating', 1, 'True')])).toBe(6)
    })
  })
})

describe('getStepPercent', () => {
  it('is always 100 for Succeeded', () => {
    expect(getStepPercent(Phase.Succeeded, TOTAL_STEPS, '0', 5)).toBe(100)
  })

  it('uses disk-count progress for copy/convert phases when available', () => {
    expect(getStepPercent(Phase.CopyingBlocks, 4, '1', 2)).toBe(50)
    expect(getStepPercent(Phase.ConvertingDisk, 6, '2', 4)).toBe(50)
  })

  it('falls back to a step-based estimate without disk info', () => {
    expect(getStepPercent(Phase.Validating, 2, undefined, undefined)).toBe(Math.round((2 / 9) * 100))
  })

  it('does not use disk info for phases without per-disk semantics', () => {
    expect(getStepPercent(Phase.AwaitingCutOverStartTime, 7, '1', 2)).toBe(Math.round((7 / 9) * 100))
  })
})

describe('getSegmentStates', () => {
  it('marks every segment pending while queued', () => {
    expect(getSegmentStates(Phase.Pending, [])).toEqual(Array(9).fill('pending'))
    expect(getSegmentStates(undefined, [])).toEqual(Array(9).fill('pending'))
  })

  it('marks every segment done once succeeded', () => {
    expect(getSegmentStates(Phase.Succeeded, [])).toEqual(Array(9).fill('done'))
  })

  it('marks completed steps done and the current step active for a live phase', () => {
    expect(getSegmentStates(Phase.CopyingBlocks, [])).toEqual([
      'done', 'done', 'done', 'active', 'pending', 'pending', 'pending', 'pending', 'pending'
    ])
  })

  it('marks the current step "ready" (not "active") while awaiting cutover', () => {
    const states = getSegmentStates(Phase.AwaitingCutOverStartTime, [])
    expect(states[6]).toBe('ready')
    expect(states.slice(0, 6)).toEqual(Array(6).fill('done'))
    expect(states.slice(7)).toEqual(Array(2).fill('pending'))
  })

  it('marks the fail point "failed" and leaves the rest pending', () => {
    const states = getSegmentStates(Phase.Failed, [condition('Validated', 1)])
    expect(states).toEqual([
      'done', 'done', 'done', 'failed', 'pending', 'pending', 'pending', 'pending', 'pending'
    ])
  })
})
