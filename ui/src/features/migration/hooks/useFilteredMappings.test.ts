import { describe, expect, it } from 'vitest'
import { renderHook } from '@testing-library/react'
import {
  filterMappingsBySourceAndTarget,
  mappingsEqual,
  mappingsNeedReconcile,
  selectApplicableTemplateMappings,
  recordUserMappingEdits,
  useFilteredMappings
} from './useFilteredMappings'
import type { ResourceMap } from '../types'

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

describe('mappingsEqual', () => {
  it('treats undefined and empty as the same, so a fresh form never triggers a write', () => {
    expect(mappingsEqual([], undefined)).toBe(true)
    expect(mappingsEqual([], [])).toBe(true)
  })

  it('detects a swap that leaves the length unchanged', () => {
    // One entry pruned and one re-applied in the same pass — a length check would miss it.
    const before = [{ source: 'Stale Network', target: 'net-1' }]
    const after = [{ source: 'VM Network', target: 'net-1' }]
    expect(mappingsEqual(after, before)).toBe(false)
  })

  it('matches identical content', () => {
    const a = [
      { source: 'VM Network', target: 'net-1' },
      { source: 'Mgmt Network', target: 'net-2' }
    ]
    const b = [
      { source: 'VM Network', target: 'net-1' },
      { source: 'Mgmt Network', target: 'net-2' }
    ]
    expect(mappingsEqual(a, b)).toBe(true)
  })

  it('is order sensitive', () => {
    const a = [
      { source: 'VM Network', target: 'net-1' },
      { source: 'Mgmt Network', target: 'net-2' }
    ]
    expect(mappingsEqual(a, [a[1], a[0]])).toBe(false)
  })

  it('detects a changed target for the same source', () => {
    expect(
      mappingsEqual(
        [{ source: 'VM Network', target: 'net-2' }],
        [{ source: 'VM Network', target: 'net-1' }]
      )
    ).toBe(false)
  })
})

describe('selectApplicableTemplateMappings', () => {
  const pool: ResourceMap[] = [
    { source: 'VM Network', target: 'Physnet1' },
    { source: 'Mgmt Network', target: 'Physnet1' },
    { source: 'vMotion', target: 'Physnet2' }
  ]
  const allTargets = ['Physnet1', 'Physnet2']
  const none: ReadonlySet<string> = new Set()

  it('applies nothing while no VM is selected — there are no sources yet', () => {
    expect(
      selectApplicableTemplateMappings({
        pool,
        current: [],
        availableSources: [],
        availableTargets: allTargets,
        suppressedSources: none
      })
    ).toEqual([])
  })

  it('applies nothing before the destination has loaded, to avoid fighting the pruning pass', () => {
    expect(
      selectApplicableTemplateMappings({
        pool,
        current: [],
        availableSources: ['VM Network'],
        availableTargets: [],
        suppressedSources: none
      })
    ).toEqual([])
  })

  it('applies the mapping for the one network the selected VM actually uses', () => {
    expect(
      selectApplicableTemplateMappings({
        pool,
        current: [],
        availableSources: ['VM Network'],
        availableTargets: allTargets,
        suppressedSources: none
      })
    ).toEqual([{ source: 'VM Network', target: 'Physnet1' }])
  })

  it('applies every relevant mapping when several VMs are selected', () => {
    expect(
      selectApplicableTemplateMappings({
        pool,
        current: [],
        availableSources: ['VM Network', 'vMotion'],
        availableTargets: allTargets,
        suppressedSources: none
      })
    ).toEqual([
      { source: 'VM Network', target: 'Physnet1' },
      { source: 'vMotion', target: 'Physnet2' }
    ])
  })

  it('preserves many-to-one: two sources sharing one target both apply', () => {
    expect(
      selectApplicableTemplateMappings({
        pool,
        current: [],
        availableSources: ['VM Network', 'Mgmt Network'],
        availableTargets: allTargets,
        suppressedSources: none
      })
    ).toEqual([
      { source: 'VM Network', target: 'Physnet1' },
      { source: 'Mgmt Network', target: 'Physnet1' }
    ])
  })

  it('ignores a source the template never mapped', () => {
    expect(
      selectApplicableTemplateMappings({
        pool,
        current: [],
        availableSources: ['Unmapped Network'],
        availableTargets: allTargets,
        suppressedSources: none
      })
    ).toEqual([])
  })

  it('never overrides a mapping the operator already made for that source', () => {
    expect(
      selectApplicableTemplateMappings({
        pool,
        current: [{ source: 'VM Network', target: 'Physnet2' }],
        availableSources: ['VM Network'],
        availableTargets: allTargets,
        suppressedSources: none
      })
    ).toEqual([])
  })

  it('does not re-apply a source that was already applied and then deleted', () => {
    expect(
      selectApplicableTemplateMappings({
        pool,
        current: [],
        availableSources: ['VM Network'],
        availableTargets: allTargets,
        suppressedSources: new Set(['VM Network'])
      })
    ).toEqual([])
  })

  it('skips a target the destination cluster does not have — this is what prevents an add/prune loop', () => {
    expect(
      selectApplicableTemplateMappings({
        pool: [{ source: 'VM Network', target: 'net-from-another-cloud' }],
        current: [],
        availableSources: ['VM Network'],
        availableTargets: allTargets,
        suppressedSources: none
      })
    ).toEqual([])
  })

  it('applies only the still-relevant entry when one pool target is missing', () => {
    expect(
      selectApplicableTemplateMappings({
        pool: [
          { source: 'VM Network', target: 'Physnet1' },
          { source: 'Mgmt Network', target: 'gone-net' }
        ],
        current: [],
        availableSources: ['VM Network', 'Mgmt Network'],
        availableTargets: allTargets,
        suppressedSources: none
      })
    ).toEqual([{ source: 'VM Network', target: 'Physnet1' }])
  })

  it('applies one entry per source even if the pool holds duplicates', () => {
    expect(
      selectApplicableTemplateMappings({
        pool: [
          { source: 'VM Network', target: 'Physnet1' },
          { source: 'VM Network', target: 'Physnet2' }
        ],
        current: [],
        availableSources: ['VM Network'],
        availableTargets: allTargets,
        suppressedSources: none
      })
    ).toEqual([{ source: 'VM Network', target: 'Physnet1' }])
  })

  it('applies nothing when there is no template pool', () => {
    expect(
      selectApplicableTemplateMappings({
        pool: undefined,
        current: [],
        availableSources: ['VM Network'],
        availableTargets: allTargets,
        suppressedSources: none
      })
    ).toEqual([])
    expect(
      selectApplicableTemplateMappings({
        pool: [],
        current: [],
        availableSources: ['VM Network'],
        availableTargets: allTargets,
        suppressedSources: none
      })
    ).toEqual([])
  })

  it('adds a newly selected VM’s mapping alongside one already applied', () => {
    // Second VM selected after the first was already auto-mapped.
    expect(
      selectApplicableTemplateMappings({
        pool,
        current: [{ source: 'VM Network', target: 'Physnet1' }],
        availableSources: ['VM Network', 'vMotion'],
        availableTargets: allTargets,
        suppressedSources: new Set(['VM Network'])
      })
    ).toEqual([{ source: 'vMotion', target: 'Physnet2' }])
  })

  it('works for datastores exactly as for networks', () => {
    expect(
      selectApplicableTemplateMappings({
        pool: [
          { source: 'datastore-nfs', target: 'nfs-punesimple' },
          { source: 'datastore1', target: 'nfs-punesimple' }
        ],
        current: [],
        availableSources: ['datastore1'],
        availableTargets: ['nfs-punesimple', 'ceph-nvme'],
        suppressedSources: none
      })
    ).toEqual([{ source: 'datastore1', target: 'nfs-punesimple' }])
  })

  it('reaches a fixed point: re-running with the applied result yields no further additions', () => {
    const first = selectApplicableTemplateMappings({
      pool,
      current: [],
      availableSources: ['VM Network', 'Mgmt Network'],
      availableTargets: allTargets,
      suppressedSources: none
    })
    const applied = new Set(first.map((m) => m.source))

    const second = selectApplicableTemplateMappings({
      pool,
      current: first,
      availableSources: ['VM Network', 'Mgmt Network'],
      availableTargets: allTargets,
      suppressedSources: applied
    })

    expect(second).toEqual([])
  })
})

// ---------------------------------------------------------------------------
// Hook-level: applying a template, then selecting VMs
//
// This is where the real defect lived. The pure helpers above can all be correct
// while the effects still destroy a template's mappings, so these drive the actual
// sequence an operator goes through.
// ---------------------------------------------------------------------------

type HookProps = Parameters<typeof useFilteredMappings>[0]

const TEMPLATE_POOL = {
  networkMappings: [
    { source: 'VM Network', target: 'Physnet1' },
    { source: 'Mgmt Network', target: 'Physnet1' },
    { source: 'vMotion', target: 'Physnet2' }
  ],
  storageMappings: [
    { source: 'datastore1', target: 'nfs-punesimple' },
    { source: 'datastore-nfs', target: 'ceph-nvme' }
  ]
}

const ALL_NETWORKS = ['Physnet1', 'Physnet2']
const ALL_VOLUME_TYPES = ['nfs-punesimple', 'ceph-nvme']

const makeProps = (overrides: Partial<HookProps> = {}): HookProps => ({
  params: {},
  vmwareNetworks: [],
  openstackNetworkNames: ALL_NETWORKS,
  vmWareStorage: [],
  openstackStorage: ALL_VOLUME_TYPES,
  arrayCredsNames: [],
  storageCopyMethod: 'normal',
  validatedArrayCreds: [],
  onChange: () => () => {},
  ...overrides
})

// Drives the hook the way the form does: each write is fed back into params on the
// next render, so the test observes the state the operator would actually end up with.
const driveHook = (initial: Partial<HookProps>) => {
  const writes: Array<{ key: string; value: ResourceMap[] }> = []
  let params: HookProps['params'] = initial.params || {}

  const onChange = (key: string) => (value: unknown) => {
    writes.push({ key, value: value as ResourceMap[] })
    params = { ...params, [key]: value }
  }

  let overrides: Partial<HookProps> = {}

  const { rerender, result } = renderHook((props: HookProps) => useFilteredMappings(props), {
    initialProps: makeProps({ ...initial, params, onChange })
  })

  const syncProps = () => rerender(makeProps({ ...initial, ...overrides, params, onChange }))

  return {
    writes,
    currentParams: () => params,
    // Re-render with the latest params plus whatever changed (e.g. VMs now selected).
    advance: (next: Partial<HookProps> = {}) => {
      overrides = { ...overrides, ...next }
      syncProps()
    },
    // A real operator edit goes through the hook's change handler — the only place a
    // manual delete can be told apart from a prune. Re-render first so the handler sees
    // the same params a mounted component would after the previous write.
    userSetNetworkMappings: (next: ResourceMap[]) => {
      syncProps()
      result.current.handleNetworkMappingsChange(next)
      syncProps()
    }
  }
}

describe('useFilteredMappings - applying a template then selecting VMs', () => {
  it('auto-applies only the mappings for the networks the selected VM uses', () => {
    // Template just applied: every saved mapping is in params, but no VMs are selected
    // yet so none of those sources exist.
    const harness = driveHook({
      params: { networkMappings: TEMPLATE_POOL.networkMappings },
      templatePool: TEMPLATE_POOL
    })

    // Pruned to empty, because nothing is selected — the pool is what saves them.
    expect(harness.currentParams().networkMappings).toEqual([])

    // Operator selects a VM that uses "VM Network".
    harness.advance({ vmwareNetworks: ['VM Network'] })

    expect(harness.currentParams().networkMappings).toEqual([
      { source: 'VM Network', target: 'Physnet1' }
    ])
  })

  it('auto-applies mappings for several VMs, including two sources sharing one target', () => {
    const harness = driveHook({
      params: { networkMappings: TEMPLATE_POOL.networkMappings },
      templatePool: TEMPLATE_POOL
    })

    harness.advance({ vmwareNetworks: ['VM Network', 'Mgmt Network'] })

    expect(harness.currentParams().networkMappings).toEqual([
      { source: 'VM Network', target: 'Physnet1' },
      { source: 'Mgmt Network', target: 'Physnet1' }
    ])
  })

  it('adds the second VM’s mapping without disturbing the first', () => {
    const harness = driveHook({
      params: { networkMappings: TEMPLATE_POOL.networkMappings },
      templatePool: TEMPLATE_POOL
    })

    harness.advance({ vmwareNetworks: ['VM Network'] })
    harness.advance({ vmwareNetworks: ['VM Network', 'vMotion'] })

    expect(harness.currentParams().networkMappings).toEqual([
      { source: 'VM Network', target: 'Physnet1' },
      { source: 'vMotion', target: 'Physnet2' }
    ])
  })

  it('auto-applies datastore mappings the same way', () => {
    const harness = driveHook({
      params: { storageMappings: TEMPLATE_POOL.storageMappings },
      templatePool: TEMPLATE_POOL
    })

    harness.advance({ vmWareStorage: ['datastore1'] })

    expect(harness.currentParams().storageMappings).toEqual([
      { source: 'datastore1', target: 'nfs-punesimple' }
    ])
  })

  it('leaves a source the template never mapped for the operator to fill in', () => {
    const harness = driveHook({
      params: { networkMappings: TEMPLATE_POOL.networkMappings },
      templatePool: TEMPLATE_POOL
    })

    harness.advance({ vmwareNetworks: ['VM Network', 'Unmapped Network'] })

    expect(harness.currentParams().networkMappings).toEqual([
      { source: 'VM Network', target: 'Physnet1' }
    ])
  })

  it('settles — no further writes once everything applicable is applied', () => {
    const harness = driveHook({
      params: { networkMappings: TEMPLATE_POOL.networkMappings },
      templatePool: TEMPLATE_POOL
    })

    harness.advance({ vmwareNetworks: ['VM Network'] })
    const writeCount = harness.writes.length

    harness.advance({ vmwareNetworks: ['VM Network'] })
    harness.advance({ vmwareNetworks: ['VM Network'] })

    expect(harness.writes.length).toBe(writeCount)
  })

  it('does not resurrect a mapping the operator deleted by hand', () => {
    const harness = driveHook({
      params: { networkMappings: TEMPLATE_POOL.networkMappings },
      templatePool: TEMPLATE_POOL
    })

    harness.advance({ vmwareNetworks: ['VM Network'] })
    expect(harness.currentParams().networkMappings).toHaveLength(1)

    // Operator clears it through the table, which records the deliberate removal.
    harness.userSetNetworkMappings([])
    harness.advance({ vmwareNetworks: ['VM Network'] })

    expect(harness.currentParams().networkMappings).toEqual([])
  })

  it('re-applies the mapping when the VM is de-selected and selected again', () => {
    const harness = driveHook({
      params: { networkMappings: TEMPLATE_POOL.networkMappings },
      templatePool: TEMPLATE_POOL
    })

    harness.advance({ vmwareNetworks: ['VM Network'] })
    expect(harness.currentParams().networkMappings).toEqual([
      { source: 'VM Network', target: 'Physnet1' }
    ])

    // VM de-selected — its source disappears, so the mapping is pruned.
    harness.advance({ vmwareNetworks: [] })
    expect(harness.currentParams().networkMappings).toEqual([])

    // Same VM selected again — the mapping must come back.
    harness.advance({ vmwareNetworks: ['VM Network'] })
    expect(harness.currentParams().networkMappings).toEqual([
      { source: 'VM Network', target: 'Physnet1' }
    ])
  })

  it('survives repeated select/de-select cycles', () => {
    const harness = driveHook({
      params: { networkMappings: TEMPLATE_POOL.networkMappings },
      templatePool: TEMPLATE_POOL
    })

    for (let cycle = 0; cycle < 3; cycle += 1) {
      harness.advance({ vmwareNetworks: ['VM Network'] })
      expect(harness.currentParams().networkMappings).toEqual([
        { source: 'VM Network', target: 'Physnet1' }
      ])
      harness.advance({ vmwareNetworks: [] })
      expect(harness.currentParams().networkMappings).toEqual([])
    }
  })

  it('keeps only the remaining VM’s mapping when one of two VMs is de-selected', () => {
    const harness = driveHook({
      params: { networkMappings: TEMPLATE_POOL.networkMappings },
      templatePool: TEMPLATE_POOL
    })

    harness.advance({ vmwareNetworks: ['VM Network', 'vMotion'] })
    expect(harness.currentParams().networkMappings).toHaveLength(2)

    // The VM using vMotion is de-selected.
    harness.advance({ vmwareNetworks: ['VM Network'] })
    expect(harness.currentParams().networkMappings).toEqual([
      { source: 'VM Network', target: 'Physnet1' }
    ])
  })

  it('still honours a manual delete across a de-select/re-select cycle', () => {
    const harness = driveHook({
      params: { networkMappings: TEMPLATE_POOL.networkMappings },
      templatePool: TEMPLATE_POOL
    })

    harness.advance({ vmwareNetworks: ['VM Network'] })
    harness.userSetNetworkMappings([])

    harness.advance({ vmwareNetworks: [] })
    harness.advance({ vmwareNetworks: ['VM Network'] })

    expect(harness.currentParams().networkMappings).toEqual([])
  })

  it('re-applies again once the operator maps that source back by hand', () => {
    const harness = driveHook({
      params: { networkMappings: TEMPLATE_POOL.networkMappings },
      templatePool: TEMPLATE_POOL
    })

    harness.advance({ vmwareNetworks: ['VM Network'] })
    harness.userSetNetworkMappings([])
    // Re-adding by hand clears the suppression…
    harness.userSetNetworkMappings([{ source: 'VM Network', target: 'Physnet2' }])
    harness.advance({ vmwareNetworks: ['VM Network'] })
    expect(harness.currentParams().networkMappings).toEqual([
      { source: 'VM Network', target: 'Physnet2' }
    ])

    // …so a later de-select/re-select restores it from the template again.
    harness.advance({ vmwareNetworks: [] })
    harness.advance({ vmwareNetworks: ['VM Network'] })
    expect(harness.currentParams().networkMappings).toEqual([
      { source: 'VM Network', target: 'Physnet1' }
    ])
  })

  it('re-applies datastore mappings across a de-select/re-select cycle', () => {
    const harness = driveHook({
      params: { storageMappings: TEMPLATE_POOL.storageMappings },
      templatePool: TEMPLATE_POOL
    })

    harness.advance({ vmWareStorage: ['datastore1'] })
    expect(harness.currentParams().storageMappings).toEqual([
      { source: 'datastore1', target: 'nfs-punesimple' }
    ])

    harness.advance({ vmWareStorage: [] })
    expect(harness.currentParams().storageMappings).toEqual([])

    harness.advance({ vmWareStorage: ['datastore1'] })
    expect(harness.currentParams().storageMappings).toEqual([
      { source: 'datastore1', target: 'nfs-punesimple' }
    ])
  })

  it('does not revert a target the operator re-pointed after it was auto-applied', () => {
    const harness = driveHook({
      params: { networkMappings: TEMPLATE_POOL.networkMappings },
      templatePool: TEMPLATE_POOL
    })

    // VM selected → template's mapping lands.
    harness.advance({ vmwareNetworks: ['VM Network'] })
    expect(harness.currentParams().networkMappings).toEqual([
      { source: 'VM Network', target: 'Physnet1' }
    ])

    // Operator re-points it at a different target.
    harness.currentParams().networkMappings = [{ source: 'VM Network', target: 'Physnet2' }]
    harness.advance({ vmwareNetworks: ['VM Network'] })

    expect(harness.currentParams().networkMappings).toEqual([
      { source: 'VM Network', target: 'Physnet2' }
    ])
  })

  it('behaves exactly as before when no template was applied', () => {
    // Regression guard for the plain New Migration flow: prune only, never add.
    const harness = driveHook({
      params: { networkMappings: [{ source: 'Stale Network', target: 'Physnet1' }] }
    })

    expect(harness.currentParams().networkMappings).toEqual([])

    harness.advance({ vmwareNetworks: ['VM Network'] })

    expect(harness.currentParams().networkMappings).toEqual([])
  })

  it('writes nothing at all for a fresh form with no mappings and no template', () => {
    const harness = driveHook({ params: {} })
    harness.advance({ vmwareNetworks: ['VM Network'] })

    expect(harness.writes).toEqual([])
  })
})

describe('recordUserMappingEdits', () => {
  it('suppresses a source the edit removed', () => {
    const suppressed = new Set<string>()
    recordUserMappingEdits(suppressed, [{ source: 'VM Network', target: 'Physnet1' }], [])
    expect([...suppressed]).toEqual(['VM Network'])
  })

  it('clears suppression for a source the edit re-added', () => {
    const suppressed = new Set(['VM Network'])
    recordUserMappingEdits(suppressed, [], [{ source: 'VM Network', target: 'Physnet2' }])
    expect(suppressed.has('VM Network')).toBe(false)
  })

  it('leaves a re-pointed source unsuppressed — the source never went away', () => {
    const suppressed = new Set<string>()
    recordUserMappingEdits(
      suppressed,
      [{ source: 'VM Network', target: 'Physnet1' }],
      [{ source: 'VM Network', target: 'Physnet2' }]
    )
    expect(suppressed.size).toBe(0)
  })

  it('handles an undefined previous list', () => {
    const suppressed = new Set<string>()
    recordUserMappingEdits(suppressed, undefined, [{ source: 'VM Network', target: 'Physnet1' }])
    expect(suppressed.size).toBe(0)
  })

  it('suppresses only the removed source when several are mapped', () => {
    const suppressed = new Set<string>()
    recordUserMappingEdits(
      suppressed,
      [
        { source: 'VM Network', target: 'Physnet1' },
        { source: 'vMotion', target: 'Physnet2' }
      ],
      [{ source: 'VM Network', target: 'Physnet1' }]
    )
    expect([...suppressed]).toEqual(['vMotion'])
  })
})
