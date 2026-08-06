import { describe, expect, it } from 'vitest'
import { CUTOVER_TYPES } from '../constants'
import type { SavedTemplate } from '../api/migration-blueprints/types'
import {
  buildAdvancedOptionRows,
  buildTemplateMappingRows,
  countTemplateMappings,
  cutoverOptionLabel,
  DATA_COPY_METHOD_LABEL,
  sourceClusterLabel,
  storageCopyMethodLabel
} from './templateLabels'

const makeTemplate = (overrides: Partial<SavedTemplate> = {}): SavedTemplate => ({
  name: 'template',
  resourceVersion: '1',
  displayName: 'Template',
  createdAt: '2026-01-01T00:00:00Z',
  sourceVCenter: 'vcenter.example.com',
  sourceCluster: '',
  destination: 'pcd-1',
  targetCluster: 'cluster-a',
  networkMappings: [],
  storageMappings: [],
  arrayCredsMappings: [],
  dataCopyMethod: 'hot',
  dataCopyStartTime: '',
  storageCopyMethod: 'normal',
  proxyVMRef: '',
  cutoverOption: CUTOVER_TYPES.ADMIN_INITIATED,
  cutoverStartTime: '',
  cutoverEndTime: '',
  dataOnly: false,
  copyOnly: false,
  preserveSourceTags: false,
  customMetadata: [],
  disconnectSourceNetwork: false,
  fallbackToDHCP: false,
  securityGroups: [],
  serverGroup: '',
  firstBootScript: '',
  networkPersistence: false,
  removeVMwareTools: false,
  imageProfiles: [],
  periodicSyncInterval: '',
  periodicSyncEnabled: false,
  acknowledgeNetworkConflictRisk: false,
  spec: { displayName: 'Template' },
  ...overrides
})

describe('DATA_COPY_METHOD_LABEL', () => {
  it('has a label for every data copy method', () => {
    expect(DATA_COPY_METHOD_LABEL.hot).toBe('Hot')
    expect(DATA_COPY_METHOD_LABEL.cold).toBe('Cold')
    expect(DATA_COPY_METHOD_LABEL.mock).toBe('Mock')
  })
})

describe('sourceClusterLabel', () => {
  it('passes through undefined/empty unchanged', () => {
    expect(sourceClusterLabel(undefined)).toBeUndefined()
    expect(sourceClusterLabel('')).toBe('')
  })

  it('collapses the standalone-host placeholder to "No cluster"', () => {
    expect(sourceClusterLabel('no-cluster-prison-cebef')).toBe('No cluster')
    expect(sourceClusterLabel('NO-CLUSTER-abc-12345')).toBe('No cluster')
  })

  it('strips a trailing 5-char k8s-object-name hash from a real cluster name', () => {
    expect(sourceClusterLabel('prod-cluster-a3f9c')).toBe('prod-cluster')
  })

  it('leaves an already-clean cluster name unchanged', () => {
    expect(sourceClusterLabel('prod-cluster')).toBe('prod-cluster')
  })

  it('prefers a live lookup match over string cleanup, since the datacenter cannot be reliably split off', () => {
    expect(
      sourceClusterLabel('prod-cluster-prison-a3f9c', { 'prod-cluster-prison-a3f9c': 'prod-cluster' })
    ).toBe('prod-cluster')
    expect(
      sourceClusterLabel('no-cluster-prison-cebef', { 'no-cluster-prison-cebef': 'NO CLUSTER' })
    ).toBe('NO CLUSTER')
  })

  it('falls back to string cleanup when the raw name has no lookup match', () => {
    expect(sourceClusterLabel('prod-cluster-prison-a3f9c', {})).toBe('prod-cluster-prison')
    expect(sourceClusterLabel('prod-cluster-prison-a3f9c', { other: 'x' })).toBe('prod-cluster-prison')
  })
})

describe('cutoverOptionLabel', () => {
  it('labels immediate cutover', () => {
    expect(cutoverOptionLabel(CUTOVER_TYPES.IMMEDIATE)).toBe('Immediate cutover')
  })

  it('labels admin-initiated cutover', () => {
    expect(cutoverOptionLabel(CUTOVER_TYPES.ADMIN_INITIATED)).toBe('Admin cutover')
  })

  it('labels time-window cutover', () => {
    expect(cutoverOptionLabel(CUTOVER_TYPES.TIME_WINDOW)).toBe('Time window cutover')
  })

  it('defaults to immediate cutover for undefined input', () => {
    expect(cutoverOptionLabel(undefined)).toBe('Immediate cutover')
  })

  it('defaults to immediate cutover for an unrecognized value', () => {
    expect(cutoverOptionLabel('nonsense')).toBe('Immediate cutover')
  })
})

describe('buildAdvancedOptionRows', () => {
  it('returns no rows when nothing advanced is set', () => {
    expect(buildAdvancedOptionRows(makeTemplate())).toEqual([])
  })

  it('includes a row per set option, with its actual value', () => {
    const rows = buildAdvancedOptionRows(
      makeTemplate({
        serverGroup: 'sg-1',
        securityGroups: ['group-a', 'group-b'],
        imageProfiles: ['default-linux'],
        firstBootScript: 'echo hello',
        networkPersistence: true,
        removeVMwareTools: true,
        disconnectSourceNetwork: true,
        fallbackToDHCP: true,
        periodicSyncEnabled: true,
        periodicSyncInterval: '1h35m',
        useGPU: true,
        acknowledgeNetworkConflictRisk: true
      })
    )

    expect(rows).toEqual([
      { label: 'Server group', value: 'sg-1' },
      { label: 'Security groups', value: 'group-a, group-b' },
      { label: 'Image profiles', value: 'default-linux' },
      { label: 'Post-migration script', value: 'echo hello' },
      { label: 'Network persistence', value: 'Enabled' },
      { label: 'Remove VMware Tools', value: 'Enabled' },
      { label: 'Disconnect source network', value: 'Enabled' },
      { label: 'Fallback to DHCP', value: 'Enabled' },
      { label: 'Periodic sync', value: 'Every 1h35m' },
      { label: 'GPU flavor', value: 'Enabled' },
      { label: 'Network conflict risk', value: 'Acknowledged' }
    ])
  })

  it('shows the rename suffix when set, else a plain "Yes"', () => {
    const withSuffix = buildAdvancedOptionRows(
      makeTemplate({ postMigrationAction: { renameVm: true, suffix: '-migrated' } })
    )
    expect(withSuffix).toEqual([{ label: 'Rename VM', value: 'Add suffix "-migrated"' }])

    const withoutSuffix = buildAdvancedOptionRows(
      makeTemplate({ postMigrationAction: { renameVm: true } })
    )
    expect(withoutSuffix).toEqual([{ label: 'Rename VM', value: 'Yes' }])
  })

  it('shows the target folder name when set, else a plain "Yes"', () => {
    const withFolder = buildAdvancedOptionRows(
      makeTemplate({ postMigrationAction: { moveToFolder: true, folderName: 'archive' } })
    )
    expect(withFolder).toEqual([{ label: 'Move to folder', value: 'archive' }])

    const withoutFolder = buildAdvancedOptionRows(
      makeTemplate({ postMigrationAction: { moveToFolder: true } })
    )
    expect(withoutFolder).toEqual([{ label: 'Move to folder', value: 'Yes' }])
  })

  it('resolves server/security group ids to names when a group lookup is given', () => {
    const rows = buildAdvancedOptionRows(
      makeTemplate({ serverGroup: 'sg-1', securityGroups: ['group-a', 'group-b'] }),
      {
        serverGroups: { 'sg-1': 'anti-affinity-group' },
        securityGroups: { 'group-a': 'default', 'group-b': 'web-servers' }
      }
    )

    expect(rows).toEqual([
      { label: 'Server group', value: 'anti-affinity-group' },
      { label: 'Security groups', value: 'default, web-servers' }
    ])
  })

  it('falls back to the raw id when a group has no matching name', () => {
    const rows = buildAdvancedOptionRows(
      makeTemplate({ serverGroup: 'sg-1', securityGroups: ['group-a'] }),
      { serverGroups: {}, securityGroups: {} }
    )

    expect(rows).toEqual([
      { label: 'Server group', value: 'sg-1' },
      { label: 'Security groups', value: 'group-a' }
    ])
  })

  it('falls back to "Enabled" for periodic sync when no interval is set', () => {
    expect(buildAdvancedOptionRows(makeTemplate({ periodicSyncEnabled: true }))).toEqual([
      { label: 'Periodic sync', value: 'Enabled' }
    ])
  })

  it('surfaces health checks from the raw spec, and never advertises the inert arrayOffload field', () => {
    const rows = buildAdvancedOptionRows(
      makeTemplate({
        spec: {
          displayName: 'Template',
          migrationStrategy: { type: 'hot', performHealthChecks: true, arrayOffload: true }
        }
      })
    )
    // arrayOffload has no producer or consumer in the repo; showing it would imply a
    // configurable setting that does nothing.
    expect(rows).toEqual([{ label: 'Health checks', value: 'Enabled' }])
  })
})

describe('buildAdvancedOptionRows — options added after the #428 merge', () => {
  const rowFor = (template: Parameters<typeof buildAdvancedOptionRows>[0], label: string) =>
    buildAdvancedOptionRows(template).find((row) => row.label === label)

  it('surfaces the data-only option', () => {
    expect(rowFor(makeTemplate({ dataOnly: true }), 'Data only (no VM creation)')).toEqual({
      label: 'Data only (no VM creation)',
      value: 'Enabled'
    })
  })

  it('surfaces the copy-only option', () => {
    expect(rowFor(makeTemplate({ copyOnly: true }), 'Copy only (no conversion)')).toEqual({
      label: 'Copy only (no conversion)',
      value: 'Enabled'
    })
  })

  it('hides the copy-only option when it is off', () => {
    expect(rowFor(makeTemplate({ copyOnly: false }), 'Copy only (no conversion)')).toBeUndefined()
  })

  it('surfaces preserve-source-tags', () => {
    expect(rowFor(makeTemplate({ preserveSourceTags: true }), 'Preserve source tags')).toEqual({
      label: 'Preserve source tags',
      value: 'Enabled'
    })
  })

  it('renders custom metadata as key=value pairs', () => {
    expect(
      rowFor(
        makeTemplate({
          customMetadata: [
            { key: 'env', value: 'prod' },
            { key: 'owner', value: 'platform-ops' }
          ]
        }),
        'Custom metadata'
      )
    ).toEqual({ label: 'Custom metadata', value: 'env=prod, owner=platform-ops' })
  })

  it('surfaces the scheduled data copy start time', () => {
    expect(
      rowFor(makeTemplate({ dataCopyStartTime: '2026-08-19T01:15' }), 'Data copy starts')?.value
    ).toBe('2026-08-19T01:15')
  })

  it('surfaces both ends of a cutover window', () => {
    const template = makeTemplate({
      cutoverStartTime: '2026-08-19T02:00',
      cutoverEndTime: '2026-08-19T04:00'
    })
    expect(rowFor(template, 'Cutover window starts')?.value).toBe('2026-08-19T02:00')
    expect(rowFor(template, 'Cutover window ends')?.value).toBe('2026-08-19T04:00')
  })

  it('surfaces the proxy VM used for accelerated copy', () => {
    expect(rowFor(makeTemplate({ proxyVMRef: 'proxy-vm-1' }), 'Proxy VM')?.value).toBe('proxy-vm-1')
  })

  it('omits every one of them when unset, so a simple template stays uncluttered', () => {
    const labels = buildAdvancedOptionRows(makeTemplate()).map((row) => row.label)
    expect(labels).not.toContain('Data only (no VM creation)')
    expect(labels).not.toContain('Preserve source tags')
    expect(labels).not.toContain('Custom metadata')
    expect(labels).not.toContain('Data copy starts')
    expect(labels).not.toContain('Cutover window starts')
    expect(labels).not.toContain('Proxy VM')
  })
})

describe('storage copy methods other than normal', () => {
  it('labels all three copy methods', () => {
    expect(storageCopyMethodLabel('normal')).toBeTruthy()
    expect(storageCopyMethodLabel('StorageAcceleratedCopy')).toBeTruthy()
    expect(storageCopyMethodLabel('HotAdd')).toBeTruthy()
  })

  it('surfaces datastore→array-credential mappings, which storage-accelerated copy uses instead of volume types', () => {
    const rows = buildTemplateMappingRows(
      makeTemplate({
        storageCopyMethod: 'StorageAcceleratedCopy',
        storageMappings: [],
        arrayCredsMappings: [{ source: 'datastore1', target: 'pure-array-1' }]
      })
    )

    expect(rows).toEqual([{ kind: 'Storage array', source: 'datastore1', target: 'pure-array-1' }])
  })

  it('lists network, storage and array mappings together', () => {
    const rows = buildTemplateMappingRows(
      makeTemplate({
        networkMappings: [{ source: 'VM Network', target: 'Physnet1' }],
        storageMappings: [{ source: 'datastore1', target: 'nfs-punesimple' }],
        arrayCredsMappings: [{ source: 'datastore2', target: 'pure-array-1' }]
      })
    )

    expect(rows.map((row) => row.kind)).toEqual(['Network', 'Storage', 'Storage array'])
  })

  it('counts array mappings too, so a storage-accelerated template does not read as "0 mappings"', () => {
    expect(
      countTemplateMappings(
        makeTemplate({
          networkMappings: [{ source: 'VM Network', target: 'Physnet1' }],
          storageMappings: [],
          arrayCredsMappings: [
            { source: 'datastore1', target: 'pure-array-1' },
            { source: 'datastore2', target: 'pure-array-1' }
          ]
        })
      )
    ).toBe(3)
  })

  it('counts nothing for a template with no mappings at all', () => {
    expect(countTemplateMappings(makeTemplate())).toBe(0)
  })

  it('shows the proxy VM that vJailbreak-accelerated copy requires', () => {
    const rows = buildAdvancedOptionRows(
      makeTemplate({ storageCopyMethod: 'HotAdd', proxyVMRef: 'proxy-vm-1' })
    )
    expect(rows.find((row) => row.label === 'Proxy VM')?.value).toBe('proxy-vm-1')
  })
})
