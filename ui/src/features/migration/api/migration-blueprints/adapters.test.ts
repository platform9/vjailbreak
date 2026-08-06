import { describe, expect, it } from 'vitest'
import {
  blueprintToSavedTemplate,
  customMetadataToMap,
  customMetadataToRows,
  savedTemplateInputToBlueprintSpec,
  sanitizeTemplateName,
  uniqueTemplateName
} from './adapters'
import { CUTOVER_TYPES } from '../../constants'
import type { MigrationBlueprint } from 'src/api/migration-blueprints/model'
import type { SaveAsTemplateInput } from './types'

const makeBlueprint = (overrides: Partial<MigrationBlueprint['spec']> = {}): MigrationBlueprint => ({
  apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1',
  kind: 'MigrationBlueprint',
  metadata: {
    name: 'production-rhel-east',
    namespace: 'migration-system',
    creationTimestamp: '2026-06-01T00:00:00Z',
    resourceVersion: '42'
  },
  spec: {
    displayName: 'Production RHEL · East',
    description: 'Standard hot migration',
    vmwareRef: 'vcenter-east-creds',
    vmwareClusterName: 'cluster-east-a',
    pcdRef: 'pcd-east-1-creds',
    targetPCDClusterName: 'cluster-prod-a',
    networkMappings: [{ source: 'vmnet-prod', target: 'net-prod-east-a' }],
    storageMappings: [{ source: 'east-nvme-ds01', target: 'ceph-nvme-east' }],
    arrayCredsMappings: [{ source: 'east-nvme-ds01', target: 'pure-array-1' }],
    storageCopyMethod: 'normal',
    proxyVMRef: { name: 'proxy-vm-1' },
    migrationStrategy: { type: 'hot', disconnectSourceNetwork: true },
    securityGroups: ['default', 'web'],
    serverGroup: 'sg-east',
    fallbackToDHCP: true,
    firstBootScript: 'echo hi',
    advancedOptions: {
      networkPersistence: true,
      removeVMwareTools: true,
      imageProfiles: ['profile-a'],
      periodicSyncInterval: '30m',
      periodicSyncEnabled: true,
      acknowledgeNetworkConflictRisk: true
    },
    postMigrationAction: { suffix: '-migrated', renameVm: true },
    osFamily: 'linuxGuest',
    useGPUFlavor: false,
    ...overrides
  }
})

describe('blueprintToSavedTemplate', () => {
  it('flattens the blueprint spec into display fields', () => {
    const blueprint = makeBlueprint()
    const result = blueprintToSavedTemplate(blueprint)

    expect(result).toMatchObject({
      name: 'production-rhel-east',
      resourceVersion: '42',
      displayName: 'Production RHEL · East',
      sourceVCenter: 'vcenter-east-creds',
      sourceCluster: 'cluster-east-a',
      destination: 'pcd-east-1-creds',
      targetCluster: 'cluster-prod-a',
      dataCopyMethod: 'hot',
      cutoverOption: CUTOVER_TYPES.IMMEDIATE,
      osFamily: 'linuxGuest',
      useGPU: false,
      arrayCredsMappings: [{ source: 'east-nvme-ds01', target: 'pure-array-1' }],
      storageCopyMethod: 'normal',
      proxyVMRef: 'proxy-vm-1',
      disconnectSourceNetwork: true,
      securityGroups: ['default', 'web'],
      serverGroup: 'sg-east',
      fallbackToDHCP: true,
      firstBootScript: 'echo hi',
      networkPersistence: true,
      removeVMwareTools: true,
      imageProfiles: ['profile-a'],
      periodicSyncInterval: '30m',
      periodicSyncEnabled: true,
      acknowledgeNetworkConflictRisk: true,
      postMigrationAction: { suffix: '-migrated', renameVm: true }
    })
    expect(result.spec).toBe(blueprint.spec)
  })

  it('defaults advanced-option fields when advancedOptions is absent', () => {
    const blueprint = makeBlueprint()
    delete blueprint.spec.advancedOptions
    delete blueprint.spec.arrayCredsMappings
    delete blueprint.spec.proxyVMRef

    const result = blueprintToSavedTemplate(blueprint)
    expect(result).toMatchObject({
      arrayCredsMappings: [],
      proxyVMRef: '',
      networkPersistence: false,
      removeVMwareTools: false,
      imageProfiles: [],
      periodicSyncInterval: '',
      periodicSyncEnabled: false,
      acknowledgeNetworkConflictRisk: false
    })
  })

  it('defaults resourceVersion to empty string when metadata omits it', () => {
    const blueprint = makeBlueprint()
    delete blueprint.metadata.resourceVersion

    const result = blueprintToSavedTemplate(blueprint)
    expect(result.resourceVersion).toBe('')
  })

  it('reads a scheduled data copy start time from the strategy', () => {
    const result = blueprintToSavedTemplate(
      makeBlueprint({
        migrationStrategy: { type: 'hot', dataCopyStart: '2026-08-01T10:00:00Z' }
      })
    )
    expect(result.dataCopyStartTime).toBe('2026-08-01T10:00:00Z')
  })

  it('ignores the k8s zero-time sentinel for dataCopyStartTime', () => {
    const result = blueprintToSavedTemplate(
      makeBlueprint({
        migrationStrategy: { type: 'hot', dataCopyStart: '0001-01-01T00:00:00Z' }
      })
    )
    expect(result.dataCopyStartTime).toBe('')
  })

  it('derives admin-initiated cutover from the strategy', () => {
    const result = blueprintToSavedTemplate(
      makeBlueprint({ migrationStrategy: { type: 'cold', adminInitiatedCutOver: true } })
    )
    expect(result.cutoverOption).toBe(CUTOVER_TYPES.ADMIN_INITIATED)
  })

  it('derives time-window cutover from set cutover times', () => {
    const result = blueprintToSavedTemplate(
      makeBlueprint({
        migrationStrategy: {
          type: 'cold',
          vmCutoverStart: '2026-07-01T00:00:00Z',
          vmCutoverEnd: '2026-07-02T00:00:00Z'
        }
      })
    )
    expect(result.cutoverOption).toBe(CUTOVER_TYPES.TIME_WINDOW)
  })

  it('ignores the k8s zero-time sentinel when deriving cutover option', () => {
    const result = blueprintToSavedTemplate(
      makeBlueprint({
        migrationStrategy: {
          type: 'cold',
          vmCutoverStart: '0001-01-01T00:00:00Z'
        }
      })
    )
    expect(result.cutoverOption).toBe(CUTOVER_TYPES.IMMEDIATE)
  })

  it('defaults missing collections to empty arrays', () => {
    const blueprint = makeBlueprint()
    delete blueprint.spec.networkMappings
    delete blueprint.spec.storageMappings

    const result = blueprintToSavedTemplate(blueprint)
    expect(result.networkMappings).toEqual([])
    expect(result.storageMappings).toEqual([])
  })
})

describe('savedTemplateInputToBlueprintSpec', () => {
  const baseInput: SaveAsTemplateInput = {
    displayName: 'Test Template',
    sourceVCenter: 'vcenter.example.com',
    sourceCluster: 'cluster-a-source',
    destination: 'pcd-1',
    targetCluster: 'cluster-a',
    networkMappings: [],
    storageMappings: [],
    dataCopyMethod: 'hot',
    cutoverOption: CUTOVER_TYPES.IMMEDIATE
  }

  it('maps flattened input fields to the blueprint spec shape', () => {
    const spec = savedTemplateInputToBlueprintSpec(baseInput)
    expect(spec).toMatchObject({
      displayName: 'Test Template',
      vmwareRef: 'vcenter.example.com',
      vmwareClusterName: 'cluster-a-source',
      pcdRef: 'pcd-1',
      targetPCDClusterName: 'cluster-a',
      migrationStrategy: { type: 'hot', adminInitiatedCutOver: false }
    })
  })

  it('omits vmwareClusterName when sourceCluster is blank', () => {
    const spec = savedTemplateInputToBlueprintSpec({ ...baseInput, sourceCluster: '' })
    expect(spec.vmwareClusterName).toBeUndefined()
  })

  it('sets adminInitiatedCutOver when cutoverOption is admin-initiated', () => {
    const spec = savedTemplateInputToBlueprintSpec({
      ...baseInput,
      cutoverOption: CUTOVER_TYPES.ADMIN_INITIATED
    })
    expect(spec.migrationStrategy?.adminInitiatedCutOver).toBe(true)
  })

  it('omits optional fields left blank', () => {
    const spec = savedTemplateInputToBlueprintSpec({
      ...baseInput,
      description: undefined,
      osFamily: undefined
    })
    expect(spec.description).toBeUndefined()
    expect(spec.osFamily).toBeUndefined()
  })

  it('maps security groups, server group, and disconnectSourceNetwork', () => {
    const spec = savedTemplateInputToBlueprintSpec({
      ...baseInput,
      securityGroups: ['default', 'web'],
      serverGroup: 'sg-east',
      disconnectSourceNetwork: true
    })
    expect(spec.securityGroups).toEqual(['default', 'web'])
    expect(spec.serverGroup).toBe('sg-east')
    expect(spec.migrationStrategy?.disconnectSourceNetwork).toBe(true)
  })

  it('bundles advanced options into a single advancedOptions object', () => {
    const spec = savedTemplateInputToBlueprintSpec({
      ...baseInput,
      networkPersistence: true,
      removeVMwareTools: true,
      imageProfiles: ['profile-a'],
      periodicSyncInterval: '30m',
      periodicSyncEnabled: true,
      acknowledgeNetworkConflictRisk: true
    })
    expect(spec.advancedOptions).toEqual({
      networkPersistence: true,
      removeVMwareTools: true,
      imageProfiles: ['profile-a'],
      periodicSyncInterval: '30m',
      periodicSyncEnabled: true,
      acknowledgeNetworkConflictRisk: true
    })
  })

  it('omits advancedOptions entirely when no advanced option is set', () => {
    const spec = savedTemplateInputToBlueprintSpec(baseInput)
    expect(spec.advancedOptions).toBeUndefined()
  })

  it('maps a scheduled data copy start time into the strategy', () => {
    const spec = savedTemplateInputToBlueprintSpec({
      ...baseInput,
      dataCopyStartTime: '2026-08-01T10:00:00Z'
    })
    expect(spec.migrationStrategy?.dataCopyStart).toBe('2026-08-01T10:00:00Z')
  })

  it('omits dataCopyStart when no start time was scheduled', () => {
    const spec = savedTemplateInputToBlueprintSpec(baseInput)
    expect(spec.migrationStrategy?.dataCopyStart).toBeUndefined()
  })

  it('maps proxyVMRef, arrayCredsMappings, storageCopyMethod, firstBootScript, and postMigrationAction', () => {
    const spec = savedTemplateInputToBlueprintSpec({
      ...baseInput,
      proxyVMRef: 'proxy-vm-1',
      arrayCredsMappings: [{ source: 'ds-1', target: 'array-1' }],
      storageCopyMethod: 'HotAdd',
      firstBootScript: 'echo hi',
      postMigrationAction: { suffix: '-migrated', renameVm: true }
    })
    expect(spec.proxyVMRef).toEqual({ name: 'proxy-vm-1' })
    expect(spec.arrayCredsMappings).toEqual([{ source: 'ds-1', target: 'array-1' }])
    expect(spec.storageCopyMethod).toBe('HotAdd')
    expect(spec.firstBootScript).toBe('echo hi')
    expect(spec.postMigrationAction).toEqual({ suffix: '-migrated', renameVm: true })
  })
})

describe('sanitizeTemplateName', () => {
  it('lowercases and hyphenates the display name', () => {
    expect(sanitizeTemplateName('Production RHEL · East')).toBe('production-rhel-east')
  })

  it('strips leading/trailing hyphens produced by punctuation', () => {
    expect(sanitizeTemplateName('!!Weird Name!!')).toBe('weird-name')
  })

  it('falls back to "template" when nothing alphanumeric remains', () => {
    expect(sanitizeTemplateName('!!!')).toBe('template')
  })
})

describe('uniqueTemplateName', () => {
  it('returns the base name when there is no collision', () => {
    expect(uniqueTemplateName('my-template', [])).toBe('my-template')
  })

  it('appends an incrementing suffix until unique', () => {
    expect(uniqueTemplateName('my-template', ['my-template', 'my-template-2'])).toBe(
      'my-template-3'
    )
  })
})

// ---------------------------------------------------------------------------
// Full round-trip fidelity
//
// Guards the whole chain at once: anything an operator can set in the form must
// survive save → blueprint spec → re-read. A field added to the form but not wired
// into both adapters fails here rather than silently vanishing on "Use template".
// ---------------------------------------------------------------------------

describe('save → spec → load round-trip', () => {
  const fullInput: SaveAsTemplateInput = {
    displayName: 'Everything Set',
    description: 'every option exercised',
    sourceVCenter: 'vcenter-east-creds',
    sourceCluster: 'cluster-east-a',
    destination: 'pcd-east-1-creds',
    targetCluster: 'cluster-prod-a',
    networkMappings: [{ source: 'VM Network', target: 'Physnet1' }],
    storageMappings: [{ source: 'datastore1', target: 'nfs-punesimple' }],
    arrayCredsMappings: [{ source: 'datastore1', target: 'pure-array-1' }],
    dataCopyMethod: 'hot',
    dataCopyStartTime: '2026-08-19T01:15:00Z',
    storageCopyMethod: 'normal',
    proxyVMRef: 'proxy-vm-1',
    cutoverOption: CUTOVER_TYPES.ADMIN_INITIATED,
    disconnectSourceNetwork: true,
    fallbackToDHCP: true,
    dataOnly: true,
    copyOnly: true,
    preserveSourceTags: true,
    customMetadata: [
      { key: 'env', value: 'prod' },
      { key: 'owner', value: 'platform-ops' }
    ],
    securityGroups: ['all-open', 'default'],
    serverGroup: 'aff',
    firstBootScript: 'echo "post-migration setup"',
    networkPersistence: true,
    removeVMwareTools: true,
    imageProfiles: ['default-linux'],
    periodicSyncInterval: '1h',
    periodicSyncEnabled: true,
    acknowledgeNetworkConflictRisk: true,
    postMigrationAction: {
      renameVm: true,
      suffix: '_migrated_to_pcd',
      moveToFolder: true,
      folderName: 'vjailbreakedVMs'
    },
    osFamily: 'linuxGuest',
    useGPU: true
  }

  const roundTrip = (input: SaveAsTemplateInput) =>
    blueprintToSavedTemplate({
      apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1',
      kind: 'MigrationBlueprint',
      metadata: { name: 'everything-set', resourceVersion: '7' },
      spec: savedTemplateInputToBlueprintSpec(input)
    })

  it('preserves the data copy method and its schedule', () => {
    const result = roundTrip(fullInput)
    expect(result.dataCopyMethod).toBe('hot')
    expect(result.dataCopyStartTime).toBe('2026-08-19T01:15:00Z')
  })

  it('preserves an admin-initiated cutover', () => {
    expect(roundTrip(fullInput).cutoverOption).toBe(CUTOVER_TYPES.ADMIN_INITIATED)
  })

  it('preserves a time-window cutover with both of its times', () => {
    const result = roundTrip({
      ...fullInput,
      cutoverOption: CUTOVER_TYPES.TIME_WINDOW,
      cutoverStartTime: '2026-08-19T02:00:00Z',
      cutoverEndTime: '2026-08-19T04:00:00Z'
    })

    expect(result.cutoverOption).toBe(CUTOVER_TYPES.TIME_WINDOW)
    expect(result.cutoverStartTime).toBe('2026-08-19T02:00:00Z')
    expect(result.cutoverEndTime).toBe('2026-08-19T04:00:00Z')
  })

  it('preserves periodic sync and its interval', () => {
    const result = roundTrip(fullInput)
    expect(result.periodicSyncEnabled).toBe(true)
    expect(result.periodicSyncInterval).toBe('1h')
  })

  it('preserves preserve-source-tags and custom metadata rows', () => {
    const result = roundTrip(fullInput)
    expect(result.preserveSourceTags).toBe(true)
    expect(result.customMetadata).toEqual([
      { key: 'env', value: 'prod' },
      { key: 'owner', value: 'platform-ops' }
    ])
  })

  it('preserves the data-only option', () => {
    expect(roundTrip(fullInput).dataOnly).toBe(true)
  })

  it('preserves the copy-only option', () => {
    expect(roundTrip(fullInput).copyOnly).toBe(true)
  })

  it('preserves the first-boot script verbatim', () => {
    expect(roundTrip(fullInput).firstBootScript).toBe('echo "post-migration setup"')
  })

  it('preserves image profiles, security groups and server group', () => {
    const result = roundTrip(fullInput)
    expect(result.imageProfiles).toEqual(['default-linux'])
    expect(result.securityGroups).toEqual(['all-open', 'default'])
    expect(result.serverGroup).toBe('aff')
  })

  it('preserves every post-migration action field', () => {
    expect(roundTrip(fullInput).postMigrationAction).toEqual({
      renameVm: true,
      suffix: '_migrated_to_pcd',
      moveToFolder: true,
      folderName: 'vjailbreakedVMs'
    })
  })

  it('preserves the remaining toggles', () => {
    const result = roundTrip(fullInput)
    expect(result.disconnectSourceNetwork).toBe(true)
    expect(result.fallbackToDHCP).toBe(true)
    expect(result.networkPersistence).toBe(true)
    expect(result.removeVMwareTools).toBe(true)
    expect(result.acknowledgeNetworkConflictRisk).toBe(true)
    expect(result.useGPU).toBe(true)
    expect(result.osFamily).toBe('linuxGuest')
  })

  it('preserves source, destination and all three mapping kinds', () => {
    const result = roundTrip(fullInput)
    expect(result.sourceVCenter).toBe('vcenter-east-creds')
    expect(result.sourceCluster).toBe('cluster-east-a')
    expect(result.destination).toBe('pcd-east-1-creds')
    expect(result.targetCluster).toBe('cluster-prod-a')
    expect(result.networkMappings).toEqual([{ source: 'VM Network', target: 'Physnet1' }])
    expect(result.storageMappings).toEqual([{ source: 'datastore1', target: 'nfs-punesimple' }])
    expect(result.arrayCredsMappings).toEqual([{ source: 'datastore1', target: 'pure-array-1' }])
    expect(result.storageCopyMethod).toBe('normal')
    expect(result.proxyVMRef).toBe('proxy-vm-1')
  })

  it('leaves every toggle off when nothing was set', () => {
    const result = roundTrip({
      displayName: 'Bare',
      sourceVCenter: '',
      destination: '',
      targetCluster: '',
      networkMappings: [],
      storageMappings: [],
      dataCopyMethod: 'cold',
      cutoverOption: CUTOVER_TYPES.IMMEDIATE
    })

    expect(result.dataOnly).toBe(false)
    expect(result.copyOnly).toBe(false)
    expect(result.preserveSourceTags).toBe(false)
    expect(result.customMetadata).toEqual([])
    expect(result.cutoverStartTime).toBe('')
    expect(result.cutoverEndTime).toBe('')
    expect(result.dataCopyStartTime).toBe('')
    expect(result.periodicSyncEnabled).toBe(false)
  })
})

describe('customMetadata conversion', () => {
  it('drops rows with a blank key so a half-filled row never becomes an entry', () => {
    expect(
      customMetadataToMap([
        { key: 'env', value: 'prod' },
        { key: '  ', value: 'ignored' },
        { key: '', value: '' }
      ])
    ).toEqual({ env: 'prod' })
  })

  it('trims keys and keeps empty values', () => {
    expect(customMetadataToMap([{ key: '  owner  ', value: '' }])).toEqual({ owner: '' })
  })

  it('handles undefined in both directions', () => {
    expect(customMetadataToMap(undefined)).toEqual({})
    expect(customMetadataToRows(undefined)).toEqual([])
  })

  it('round-trips a populated map', () => {
    const rows = [
      { key: 'env', value: 'prod' },
      { key: 'tier', value: 'web' }
    ]
    expect(customMetadataToRows(customMetadataToMap(rows))).toEqual(rows)
  })
})

describe('round-trip for the non-normal storage copy methods', () => {
  const base: SaveAsTemplateInput = {
    displayName: 'Copy method',
    sourceVCenter: 'vcenter-east-creds',
    destination: 'pcd-east-1-creds',
    targetCluster: 'cluster-prod-a',
    networkMappings: [{ source: 'VM Network', target: 'Physnet1' }],
    storageMappings: [],
    dataCopyMethod: 'cold',
    cutoverOption: CUTOVER_TYPES.IMMEDIATE
  }

  const roundTrip = (input: SaveAsTemplateInput) =>
    blueprintToSavedTemplate({
      apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1',
      kind: 'MigrationBlueprint',
      metadata: { name: 'copy-method', resourceVersion: '3' },
      spec: savedTemplateInputToBlueprintSpec(input)
    })

  it('preserves vJailbreak-accelerated copy with its proxy VM and storage mappings', () => {
    const result = roundTrip({
      ...base,
      storageCopyMethod: 'HotAdd',
      proxyVMRef: 'proxy-vm-1',
      storageMappings: [{ source: 'datastore1', target: 'nfs-punesimple' }]
    })

    expect(result.storageCopyMethod).toBe('HotAdd')
    expect(result.proxyVMRef).toBe('proxy-vm-1')
    expect(result.storageMappings).toEqual([{ source: 'datastore1', target: 'nfs-punesimple' }])
  })

  it('preserves storage-accelerated copy with its datastore→array mappings', () => {
    const result = roundTrip({
      ...base,
      storageCopyMethod: 'StorageAcceleratedCopy',
      arrayCredsMappings: [
        { source: 'datastore1', target: 'pure-array-1' },
        { source: 'datastore2', target: 'netapp-array-2' }
      ]
    })

    expect(result.storageCopyMethod).toBe('StorageAcceleratedCopy')
    expect(result.arrayCredsMappings).toEqual([
      { source: 'datastore1', target: 'pure-array-1' },
      { source: 'datastore2', target: 'netapp-array-2' }
    ])
  })

  it('keeps many-to-one on the array side — two datastores can share one array credential', () => {
    const result = roundTrip({
      ...base,
      storageCopyMethod: 'StorageAcceleratedCopy',
      arrayCredsMappings: [
        { source: 'datastore1', target: 'pure-array-1' },
        { source: 'datastore2', target: 'pure-array-1' }
      ]
    })

    expect(result.arrayCredsMappings.map((m) => m.target)).toEqual(['pure-array-1', 'pure-array-1'])
  })

  it('defaults to normal copy with no proxy VM when nothing was chosen', () => {
    const result = roundTrip(base)
    expect(result.storageCopyMethod).toBe('normal')
    expect(result.proxyVMRef).toBe('')
    expect(result.arrayCredsMappings).toEqual([])
  })
})
