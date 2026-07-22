import { describe, expect, it } from 'vitest'
import { Phase, type Condition } from '../api/migrations'
import { deriveProgressDisplay, matchesDateFilter } from './migrationTableUtils'

const condition = (message: string, lastTransitionTime: string): Condition[] =>
  [{ message, lastTransitionTime, reason: 'Migration', status: 'True', type: 'Migrating' }] as unknown as Condition[]

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

describe('deriveProgressDisplay', () => {
  it('shows a full success bar on Succeeded', () => {
    expect(deriveProgressDisplay(Phase.Succeeded, undefined, undefined, undefined)).toEqual({
      primaryText: 'Complete',
      barValue: 100,
      barVariant: 'determinate',
      barColor: 'success'
    })
  })

  it('shows the latest condition message on Failed, with disk progress if known', () => {
    const conditions = condition('virt-v2v could not inject VirtIO drivers.', '2026-01-01T00:00:00Z')
    expect(deriveProgressDisplay(Phase.Failed, conditions, '1', 4)).toEqual({
      primaryText: 'virt-v2v could not inject VirtIO drivers.',
      barValue: 25,
      barVariant: 'determinate',
      barColor: 'error'
    })
  })

  it('falls back to a generic label on Failed with no condition message', () => {
    expect(deriveProgressDisplay(Phase.Failed, undefined, undefined, undefined)).toEqual({
      primaryText: 'Failed migration',
      barValue: 100,
      barVariant: 'determinate',
      barColor: 'error'
    })
  })

  it('shows a queued state on Pending', () => {
    expect(deriveProgressDisplay(Phase.Pending, undefined, undefined, undefined)).toEqual({
      primaryText: 'Queued',
      secondaryText: 'In queue',
      barValue: 0,
      barVariant: 'determinate',
      barColor: 'neutral'
    })
  })

  it('distinguishes admin cutover from scheduled cutover-window waits', () => {
    expect(deriveProgressDisplay(Phase.AwaitingAdminCutOver, undefined, undefined, undefined).secondaryText).toBe(
      'Awaiting admin cutover'
    )
    expect(
      deriveProgressDisplay(Phase.AwaitingCutOverStartTime, undefined, undefined, undefined).secondaryText
    ).toBe('Awaiting cutover window')
  })

  it('shows the latest condition message on Validating, else a generic label', () => {
    expect(deriveProgressDisplay(Phase.Validating, undefined, undefined, undefined).primaryText).toBe(
      'Running pre-flight checks'
    )
    const conditions = condition('Migration validated successfully', '2026-01-01T00:00:00Z')
    expect(deriveProgressDisplay(Phase.Validating, conditions, undefined, undefined).primaryText).toBe(
      'Migration validated successfully'
    )
  })

  it('shows the disk count and percent on CopyingBlocks when known', () => {
    expect(deriveProgressDisplay(Phase.CopyingBlocks, undefined, '1', 4)).toEqual({
      primaryText: 'Disk 2 of 4',
      secondaryText: '25%',
      barValue: 25,
      barVariant: 'determinate',
      barColor: 'primary'
    })
  })

  it('falls back to an indeterminate bar on CopyingBlocks with no disk info', () => {
    expect(deriveProgressDisplay(Phase.CopyingBlocks, undefined, undefined, undefined)).toEqual({
      primaryText: 'Copying blocks',
      secondaryText: undefined,
      barValue: 0,
      barVariant: 'indeterminate',
      barColor: 'primary'
    })
  })

  it('labels other in-progress phases without fabricating disk info they don\'t have', () => {
    expect(deriveProgressDisplay(Phase.ConvertingDisk, undefined, undefined, undefined).primaryText).toBe(
      'Converting disk format'
    )
    expect(deriveProgressDisplay(Phase.CopyingChangedBlocks, undefined, undefined, undefined).primaryText).toBe(
      'Final changed-block sync'
    )
  })

  it('falls back to the raw phase name for an unmapped in-progress phase', () => {
    expect(deriveProgressDisplay(Phase.AttachingDisksToProxy, undefined, undefined, undefined).primaryText).toBe(
      'Attaching disks to proxy'
    )
  })

  it('falls back to a neutral bar for an unknown/undefined phase', () => {
    expect(deriveProgressDisplay(undefined, undefined, undefined, undefined)).toEqual({
      primaryText: 'Unknown',
      barValue: 0,
      barVariant: 'indeterminate',
      barColor: 'neutral'
    })
  })

  it('flags a sync warning on the bar without touching progress text, unless already failed', () => {
    const inProgress = deriveProgressDisplay(Phase.CopyingBlocks, undefined, '1', 4, 'falling behind')
    expect(inProgress.barColor).toBe('warning')
    expect(inProgress.primaryText).toBe('Disk 2 of 4')

    const failed = deriveProgressDisplay(Phase.Failed, undefined, undefined, undefined, 'falling behind')
    expect(failed.barColor).toBe('error')
  })
})
