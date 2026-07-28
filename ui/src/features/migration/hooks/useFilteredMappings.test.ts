import { describe, expect, it } from 'vitest'
import { filterMappingsBySourceAndTarget, mappingsNeedReconcile } from './useFilteredMappings'

describe('filterMappingsBySourceAndTarget', () => {
  it('drops all mappings when the source list is empty (all VMs deselected, so nothing is stale-safe to keep)', () => {
    const mappings = [{ source: 'VM Network', target: 'net-1' }]
    expect(filterMappingsBySourceAndTarget(mappings, [], ['net-1'])).toEqual([])
  })

  it('keeps mappings untouched when the target list is empty (destination not loaded yet)', () => {
    const mappings = [{ source: 'VM Network', target: 'net-1' }]
    expect(filterMappingsBySourceAndTarget(mappings, ['VM Network'], [])).toEqual(mappings)
  })

  it('keeps a mapping once its source and target both appear in the loaded lists', () => {
    const mappings = [{ source: 'VM Network', target: 'net-1' }]
    expect(
      filterMappingsBySourceAndTarget(mappings, ['VM Network'], ['net-1'])
    ).toEqual(mappings)
  })

  it('drops a mapping whose source is not present once the source list has loaded', () => {
    const mappings = [{ source: 'Stale Network', target: 'net-1' }]
    expect(filterMappingsBySourceAndTarget(mappings, ['VM Network'], ['net-1'])).toEqual([])
  })

  it('drops a mapping whose target is not present once the target list has loaded', () => {
    const mappings = [{ source: 'VM Network', target: 'stale-net' }]
    expect(filterMappingsBySourceAndTarget(mappings, ['VM Network'], ['net-1'])).toEqual([])
  })

  it('returns an empty array when no mappings are given', () => {
    expect(filterMappingsBySourceAndTarget(undefined, ['VM Network'], ['net-1'])).toEqual([])
  })

  it('keeps a mapping still used by a remaining selected VM after another VM is deselected (#2217)', () => {
    const mappings = [
      { source: 'VM Network A', target: 'net-1' },
      { source: 'VM Network B', target: 'net-2' }
    ]
    // VM using "VM Network B" was deselected; VM using "VM Network A" remains selected.
    expect(
      filterMappingsBySourceAndTarget(mappings, ['VM Network A'], ['net-1', 'net-2'])
    ).toEqual([{ source: 'VM Network A', target: 'net-1' }])
  })

  it('drops the only mapping when the single remaining VM is deselected (#2217)', () => {
    const mappings = [{ source: 'VM Datastore', target: 'volume-1' }]
    expect(filterMappingsBySourceAndTarget(mappings, [], ['volume-1'])).toEqual([])
  })
})

describe('mappingsNeedReconcile', () => {
  it('does not flag a reconcile when current is undefined and filtered is empty (fresh form / template prefill still in flight)', () => {
    expect(mappingsNeedReconcile([], undefined)).toBe(false)
  })

  it('does not flag a reconcile when nothing was pruned', () => {
    const mappings = [{ source: 'VM Network', target: 'net-1' }]
    expect(mappingsNeedReconcile(mappings, mappings)).toBe(false)
  })

  it('flags a reconcile when filtering actually pruned an entry', () => {
    const current = [{ source: 'Stale Network', target: 'net-1' }]
    expect(mappingsNeedReconcile([], current)).toBe(true)
  })
})
