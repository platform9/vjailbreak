import { describe, expect, it } from 'vitest'
import { CUTOVER_TYPES } from '../constants'
import type { SavedTemplate } from '../api/migration-blueprints/types'
import { buildAdvancedOptionRows, cutoverOptionLabel, DATA_COPY_METHOD_LABEL } from './templateLabels'

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
    expect(DATA_COPY_METHOD_LABEL.hot).toBe('Hot copy')
    expect(DATA_COPY_METHOD_LABEL.cold).toBe('Cold copy')
    expect(DATA_COPY_METHOD_LABEL.mock).toBe('Mock copy')
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

  it('falls back to "Enabled" for periodic sync when no interval is set', () => {
    expect(buildAdvancedOptionRows(makeTemplate({ periodicSyncEnabled: true }))).toEqual([
      { label: 'Periodic sync', value: 'Enabled' }
    ])
  })

  it('surfaces health checks and array offload from the raw spec', () => {
    const rows = buildAdvancedOptionRows(
      makeTemplate({
        spec: {
          displayName: 'Template',
          migrationStrategy: { type: 'hot', performHealthChecks: true, arrayOffload: true }
        }
      })
    )
    expect(rows).toEqual([
      { label: 'Health checks', value: 'Enabled' },
      { label: 'Array offload', value: 'Enabled' }
    ])
  })
})
