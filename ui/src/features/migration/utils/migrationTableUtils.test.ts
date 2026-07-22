import { describe, expect, it } from 'vitest'
import { matchesDateFilter } from './migrationTableUtils'

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
