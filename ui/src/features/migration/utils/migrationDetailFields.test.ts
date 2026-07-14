import { describe, expect, it } from 'vitest'
import { normalizeVmDisks, resolveFlavorDisplay } from './migrationDetailFields'

const flavors = [
  { id: 'flavor-1', name: 'm1.small' },
  { id: 'flavor-2', name: 'm1.large' },
]

describe('resolveFlavorDisplay', () => {
  it('shows Auto-assign when no flavor is selected or resolved yet', () => {
    expect(resolveFlavorDisplay({ flavors })).toBe('Auto-assign')
    expect(resolveFlavorDisplay({ configFlavorId: '', selectedFlavorId: '', flavors })).toBe(
      'Auto-assign'
    )
  })

  it('shows the user-selected flavor name resolved from the flavor list', () => {
    expect(resolveFlavorDisplay({ selectedFlavorId: 'flavor-1', flavors })).toBe('m1.small')
  })

  it('prefers the flavor recorded in the migration ConfigMap', () => {
    expect(
      resolveFlavorDisplay({
        configFlavorId: 'flavor-2',
        selectedFlavorId: 'flavor-1',
        flavors,
      })
    ).toBe('m1.large')
  })

  it('marks auto-assigned flavors resolved by the controller', () => {
    expect(resolveFlavorDisplay({ configFlavorId: 'flavor-2', flavors })).toBe(
      'm1.large (auto-assigned)'
    )
  })

  it('falls back to the flavor ID when no name can be resolved', () => {
    expect(resolveFlavorDisplay({ selectedFlavorId: 'flavor-404', flavors })).toBe('flavor-404')
    expect(resolveFlavorDisplay({ configFlavorId: 'flavor-404' })).toBe(
      'flavor-404 (auto-assigned)'
    )
  })
})

describe('normalizeVmDisks', () => {
  it('returns empty for missing or non-array disks', () => {
    expect(normalizeVmDisks(undefined)).toEqual([])
    expect(normalizeVmDisks('disk-1')).toEqual([])
  })

  it('maps disk objects to rows with formatted sizes', () => {
    expect(
      normalizeVmDisks([
        { name: 'Hard disk 1', capacityGB: 33, datastore: 'datastore-nfs' },
        { name: 'Hard disk 2', capacityGB: 2048, datastore: 'ds-2' },
      ])
    ).toEqual([
      { name: 'Hard disk 1', size: '33 GB', datastore: 'datastore-nfs' },
      { name: 'Hard disk 2', size: '2 TB', datastore: 'ds-2' },
    ])
  })

  it('handles legacy string disks without sizes', () => {
    expect(normalizeVmDisks(['Hard disk 1'])).toEqual([
      { name: 'Hard disk 1', size: 'N/A', datastore: 'N/A' },
    ])
  })

  it('fills placeholder names and sizes for incomplete entries', () => {
    expect(normalizeVmDisks([{ capacityGB: 0 }, ''])).toEqual([
      { name: 'Disk 1', size: 'N/A', datastore: 'N/A' },
      { name: 'Disk 2', size: 'N/A', datastore: 'N/A' },
    ])
  })
})
