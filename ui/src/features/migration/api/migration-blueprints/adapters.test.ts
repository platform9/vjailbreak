import { describe, expect, it } from 'vitest'
import {
  blueprintToSavedTemplate,
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
