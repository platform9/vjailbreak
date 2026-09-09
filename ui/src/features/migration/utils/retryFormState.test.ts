import { describe, expect, it } from 'vitest'
import {
  buildRetryFormState,
  buildRetryPlanSpec,
  buildRetryTemplateSpec
} from './retryFormState'
import { createMigrationPlanJson } from '../api/migration-plans/helpers'
import { pollUntilGone } from '../hooks/useRetrySubmit'
import { CUTOVER_TYPES } from '../constants'
import type { MigrationPlan } from '../api/migration-plans/model'
import type { MigrationTemplate, VmData } from '../api/migration-templates/model'
import type { FormValues, SelectedMigrationOptionsType } from '../types'

const makePlan = (spec: Record<string, unknown> = {}): MigrationPlan =>
  ({
    metadata: { name: 'plan-1', namespace: 'migration-system' },
    spec: {
      migrationTemplate: 'template-1',
      retry: false,
      virtualMachines: [['vm-1']],
      migrationStrategy: { type: 'cold' },
      ...spec
    }
  }) as unknown as MigrationPlan

const makeTemplate = (spec: Record<string, unknown> = {}): MigrationTemplate =>
  ({
    metadata: { name: 'template-1', namespace: 'migration-system' },
    spec: {
      source: { vmwareRef: 'vmware-1', datacenter: 'dc-1' },
      destination: { openstackRef: 'pcd-1' },
      targetPCDClusterName: 'cluster-a',
      ...spec
    }
  }) as unknown as MigrationTemplate

const VM: VmData = { name: 'vm-1', vmKey: 'vm-1' } as unknown as VmData

const formStateInput = (plan: MigrationPlan, template = makeTemplate()) => ({
  plan,
  template,
  vmData: VM,
  clusterName: 'source-cluster',
  datacenter: 'dc-1',
  vmwareRef: 'vmware-1',
  openstackRef: 'pcd-1',
  networkMappings: [],
  storageMappings: [],
  arrayCredsMappings: [],
  pcdData: [{ id: 'pcd-id-1', name: 'cluster-a' }]
})

const SELECTED: SelectedMigrationOptionsType = {
  dataCopyMethod: true,
  dataCopyStartTime: false,
  cutoverOption: false,
  cutoverStartTime: false,
  cutoverEndTime: false,
  postMigrationScript: false
}

describe('buildRetryFormState — StorageAcceleratedCopy mappings', () => {
  // storageCopyMethod is restored from the template, so the form knows it is a
  // StorageAcceleratedCopy migration and validates the datastore -> ArrayCreds mapping.
  // Without arrayCredsMappings restored alongside it, that mapping opens empty and the
  // Retry button stays disabled — the migration cannot be retried at all.
  const sacTemplate = makeTemplate({
    storageCopyMethod: 'StorageAcceleratedCopy',
    arrayCredsMapping: 'arrmap-1'
  })

  it('restores the datastore to ArrayCreds mappings the template referenced', () => {
    const { params } = buildRetryFormState({
      ...formStateInput(makePlan(), sacTemplate),
      arrayCredsMappings: [{ source: 'datastore-1', target: 'array-creds-1' }]
    })

    expect(params.storageCopyMethod).toBe('StorageAcceleratedCopy')
    expect(params.arrayCredsMappings).toEqual([
      { source: 'datastore-1', target: 'array-creds-1' }
    ])
  })

  it('leaves the mappings empty for a normal-copy migration', () => {
    const { params } = buildRetryFormState(formStateInput(makePlan()))
    expect(params.arrayCredsMappings).toEqual([])
  })
})

describe('buildRetryFormState — dataOnly', () => {
  it('restores dataOnly=true from the plan migration strategy', () => {
    const { params } = buildRetryFormState(
      formStateInput(makePlan({ migrationStrategy: { type: 'cold', dataOnly: true } }))
    )
    expect(params.dataOnly).toBe(true)
  })

  it('defaults dataOnly to false when the plan strategy omits it', () => {
    const { params } = buildRetryFormState(formStateInput(makePlan()))
    expect(params.dataOnly).toBe(false)
  })
})

describe('buildRetryFormState — other prefilled fields', () => {
  it('derives an admin-initiated cutover and preserves plan-level options', () => {
    const { params, selectedOptions, formDefaults } = buildRetryFormState(
      formStateInput(
        makePlan({
          migrationStrategy: {
            type: 'hot',
            adminInitiatedCutOver: true,
            disconnectSourceNetwork: true
          },
          fallbackToDHCP: true,
          securityGroups: ['sg-1'],
          serverGroup: 'sg-group',
          preserveSourceTags: true,
          customMetadata: { owner: 'team-a' }
        })
      )
    )

    expect(params.cutoverOption).toBe(CUTOVER_TYPES.ADMIN_INITIATED)
    expect(params.dataCopyMethod).toBe('hot')
    expect(params.disconnectSourceNetwork).toBe(true)
    expect(params.fallbackToDHCP).toBe(true)
    expect(params.pcdCluster).toBe('pcd-id-1')
    expect(params.preserveSourceTags).toBe(true)
    expect(params.customMetadata).toEqual([{ key: 'owner', value: 'team-a' }])
    expect(selectedOptions.cutoverOption).toBe(true)
    expect(formDefaults.securityGroups).toEqual(['sg-1'])
    expect(formDefaults.serverGroup).toBe('sg-group')
  })

  it('treats the zero time as unset and defaults to an immediate cutover', () => {
    const { params, selectedOptions } = buildRetryFormState(
      formStateInput(
        makePlan({
          migrationStrategy: {
            type: 'cold',
            dataCopyStart: '0001-01-01T00:00:00Z',
            vmCutoverStart: '0001-01-01T00:00:00Z',
            vmCutoverEnd: '0001-01-01T00:00:00Z'
          }
        })
      )
    )

    expect(params.cutoverOption).toBe(CUTOVER_TYPES.IMMEDIATE)
    expect(params.dataCopyStartTime).toBeUndefined()
    expect(selectedOptions.dataCopyStartTime).toBe(false)
    expect(selectedOptions.cutoverOption).toBe(false)
  })

  it('drops the placeholder first-boot script', () => {
    const { params, selectedOptions } = buildRetryFormState(
      formStateInput(makePlan({ firstBootScript: 'echo "Add your startup script here!"' }))
    )
    expect(params.postMigrationScript).toBeUndefined()
    expect(selectedOptions.postMigrationScript).toBe(false)
  })
})

describe('buildRetryPlanSpec — dataOnly', () => {
  it('sends dataOnly=true so the retried plan keeps data-only mode', () => {
    const spec = buildRetryPlanSpec({
      params: { dataOnly: true } as Partial<FormValues>,
      selectedMigrationOptions: SELECTED,
      retryPlan: makePlan({ migrationStrategy: { type: 'cold', dataOnly: true } })
    })
    expect(spec.migrationStrategy.dataOnly).toBe(true)
  })

  it('sends dataOnly=false explicitly when the user unchecks it on retry', () => {
    const spec = buildRetryPlanSpec({
      params: { dataOnly: false } as Partial<FormValues>,
      selectedMigrationOptions: SELECTED,
      retryPlan: makePlan({ migrationStrategy: { type: 'cold', dataOnly: true } })
    })
    expect(spec.migrationStrategy).toHaveProperty('dataOnly', false)
  })
})

describe('preserveSourceTags and customMetadata round-trip', () => {
  it('prefills both from the failed plan', () => {
    const { params } = buildRetryFormState(
      formStateInput(
        makePlan({
          preserveSourceTags: true,
          customMetadata: { owner: 'team-a', env: 'prod' }
        })
      )
    )

    expect(params.preserveSourceTags).toBe(true)
    expect(params.customMetadata).toEqual([
      { key: 'owner', value: 'team-a' },
      { key: 'env', value: 'prod' }
    ])
  })

  it('writes both back onto the retried plan', () => {
    const spec = buildRetryPlanSpec({
      params: {
        preserveSourceTags: true,
        customMetadata: [
          { key: 'owner', value: 'team-a' },
          { key: 'env', value: 'prod' }
        ]
      } as Partial<FormValues>,
      selectedMigrationOptions: SELECTED,
      retryPlan: makePlan()
    })

    expect(spec.preserveSourceTags).toBe(true)
    expect(spec.customMetadata).toEqual({ owner: 'team-a', env: 'prod' })
  })

  it('clears metadata the user removed and defaults tags to false', () => {
    const spec = buildRetryPlanSpec({
      params: { customMetadata: [] } as Partial<FormValues>,
      selectedMigrationOptions: SELECTED,
      retryPlan: makePlan({ preserveSourceTags: true, customMetadata: { owner: 'team-a' } })
    })

    expect(spec.preserveSourceTags).toBe(false)
    expect(spec.customMetadata).toBeNull()
  })

  it('drops blank metadata keys and trims the rest', () => {
    const spec = buildRetryPlanSpec({
      params: {
        customMetadata: [
          { key: '  ', value: 'ignored' },
          { key: ' owner ', value: ' team-a ' }
        ]
      } as Partial<FormValues>,
      selectedMigrationOptions: SELECTED,
      retryPlan: makePlan()
    })

    expect(spec.customMetadata).toEqual({ owner: 'team-a' })
  })

  it('prefills no metadata rows when the plan has an empty map', () => {
    const { params } = buildRetryFormState(formStateInput(makePlan({ customMetadata: {} })))
    expect(params.customMetadata).toBeUndefined()
    expect(params.preserveSourceTags).toBe(false)
  })
})

describe('buildRetryPlanSpec — unchanged behaviour', () => {
  it('falls back to the original plan strategy type and nulls unset times', () => {
    const spec = buildRetryPlanSpec({
      params: {} as Partial<FormValues>,
      selectedMigrationOptions: SELECTED,
      retryPlan: makePlan({ migrationStrategy: { type: 'hot' } })
    })

    expect(spec.migrationStrategy.type).toBe('hot')
    expect(spec.migrationStrategy.dataCopyStart).toBeNull()
    expect(spec.migrationStrategy.vmCutoverStart).toBeNull()
    expect(spec.migrationStrategy.vmCutoverEnd).toBeNull()
    expect(spec.networkOverridesPerVM).toBeNull()
    expect(spec.securityGroups).toBeNull()
  })

  it('emits per-VM network overrides sorted by interface index', () => {
    const spec = buildRetryPlanSpec({
      params: {
        vms: [
          {
            name: 'vm-1',
            vmKey: 'vm-1',
            preserveIp: { 1: false, 0: true },
            networkInterfaces: [{ ipAddress: [] }, { ipAddress: [' 10.0.0.5 '] }]
          }
        ]
      } as unknown as Partial<FormValues>,
      selectedMigrationOptions: SELECTED,
      retryPlan: makePlan()
    })

    expect(spec.networkOverridesPerVM).toEqual({
      'vm-1': [
        { interfaceIndex: 0, preserveIP: true, preserveMAC: true },
        { interfaceIndex: 1, preserveIP: false, preserveMAC: true, UserAssignedIP: '10.0.0.5' }
      ]
    })
  })

  it('honours the time-window cutover selection', () => {
    const spec = buildRetryPlanSpec({
      params: {
        cutoverOption: CUTOVER_TYPES.TIME_WINDOW,
        cutoverStartTime: '2026-08-06T10:00:00Z',
        cutoverEndTime: '2026-08-06T12:00:00Z'
      } as Partial<FormValues>,
      selectedMigrationOptions: { ...SELECTED, cutoverOption: true },
      retryPlan: makePlan()
    })

    expect(spec.migrationStrategy.adminInitiatedCutOver).toBe(false)
    expect(spec.migrationStrategy.vmCutoverStart).toBe('2026-08-06T10:00:00Z')
    expect(spec.migrationStrategy.vmCutoverEnd).toBe('2026-08-06T12:00:00Z')
  })
})

// ─── Replacement MigrationTemplate ────────────────────────────────────────────

describe('buildRetryTemplateSpec', () => {
  const ORIGINAL = {
    source: { vmwareRef: 'vmware-1', datacenter: 'dc-1' },
    destination: { openstackRef: 'pcd-1' },
    networkMapping: 'netmap-old',
    storageMapping: 'stormap-old',
    targetPCDClusterName: 'cluster-a',
    storageCopyMethod: 'HotAdd',
    proxyVMRef: { name: 'proxy-1' },
    osFamily: 'linuxGuest'
  } as unknown as MigrationTemplate['spec']

  const build = (params: Partial<FormValues>, overrides = {}) =>
    buildRetryTemplateSpec({
      originalTemplateSpec: ORIGINAL,
      params,
      selectedPcdClusterName: 'cluster-a',
      ...overrides
    })

  it('inherits the immutable source and destination refs', () => {
    const spec = build({ storageCopyMethod: 'HotAdd', proxyVMRef: 'proxy-1' })
    expect(spec.source).toEqual({ vmwareRef: 'vmware-1', datacenter: 'dc-1' })
    expect(spec.destination).toEqual({ openstackRef: 'pcd-1' })
    expect(spec.osFamily).toBe('linuxGuest')
  })

  it('keeps the ProxyVM for a HotAdd retry', () => {
    const spec = build({ storageCopyMethod: 'HotAdd', proxyVMRef: 'proxy-2' })
    expect(spec.proxyVMRef).toEqual({ name: 'proxy-2' })
  })

  it('drops the inherited ProxyVM when the retry leaves HotAdd', () => {
    // Without this the new template keeps pointing at a ProxyVM it no longer uses.
    const spec = build({ storageCopyMethod: 'normal' })
    expect(spec.proxyVMRef).toBeUndefined()
    expect(JSON.parse(JSON.stringify(spec))).not.toHaveProperty('proxyVMRef')
  })

  it('points at newly created mappings and leaves the originals alone otherwise', () => {
    expect(build({ storageCopyMethod: 'normal' }).networkMapping).toBe('netmap-old')

    const spec = build(
      { storageCopyMethod: 'normal' },
      { newNetworkMappingName: 'netmap-new', newStorageMappingName: 'stormap-new' }
    )
    expect(spec.networkMapping).toBe('netmap-new')
    expect(spec.storageMapping).toBe('stormap-new')
  })

  it('points at a newly created ArrayCredsMapping for storage-accelerated copy', () => {
    const spec = build(
      { storageCopyMethod: 'StorageAcceleratedCopy' },
      { newArrayCredsMappingName: 'arrmap-new' }
    )
    expect(spec.arrayCredsMapping).toBe('arrmap-new')
    expect(spec.storageCopyMethod).toBe('StorageAcceleratedCopy')
  })

  it('falls back to the original target cluster when the form has not resolved one', () => {
    const spec = buildRetryTemplateSpec({
      originalTemplateSpec: ORIGINAL,
      params: {},
      selectedPcdClusterName: ''
    })
    expect(spec.targetPCDClusterName).toBe('cluster-a')
  })
})

// ─── Waiting for the failed Migration to actually go away ─────────────────────

describe('pollUntilGone', () => {
  const notFound = { response: { status: 404 } }
  const fast = { pollIntervalMs: 1, timeoutMs: 60, resourceLabel: 'Migration "m1"' }

  it('resolves as soon as the resource 404s', async () => {
    let calls = 0
    await expect(
      pollUntilGone(() => {
        calls += 1
        return calls < 3 ? Promise.resolve({}) : Promise.reject(notFound)
      }, fast)
    ).resolves.toBeUndefined()
    expect(calls).toBe(3)
  })

  it('does not mistake a server error for a successful delete', async () => {
    // A 500 used to be swallowed and read as "gone", so the retry proceeded while the old
    // Migration was still running against the same VM.
    await expect(
      pollUntilGone(() => Promise.reject({ response: { status: 500 } }), fast)
    ).rejects.toThrow(/still present/)
  })

  it('rejects instead of proceeding when the resource never disappears', async () => {
    await expect(pollUntilGone(() => Promise.resolve({}), fast)).rejects.toThrow(
      /Migration "m1" was still present after/
    )
  })

  it('reports the last error it saw while waiting', async () => {
    await expect(
      pollUntilGone(() => Promise.reject(new Error('network down')), fast)
    ).rejects.toThrow(/network down/)
  })
})

// ─── Retry/create plan-spec parity ────────────────────────────────────────────

// buildRetryPlanSpec is a hand-maintained mirror of the create path's plan payload
// (createMigrationPlanJson). Every field added to one and forgotten in the other is a
// silent data-loss bug on retry — that is how dataOnly, preserveSourceTags and
// customMetadata each went missing. This guard fails the moment the two drift.
describe('buildRetryPlanSpec — parity with the create path', () => {
  // Set on the plan by useRetrySubmit itself, not by buildRetryPlanSpec.
  const CALL_SITE_KEYS = ['migrationTemplate', 'retry', 'virtualMachines']

  const createSpec = createMigrationPlanJson({
    name: 'plan-1',
    migrationTemplateName: 'template-1',
    virtualMachines: ['vm-1'],
    type: 'hot',
    dataCopyStart: '2026-08-06T10:00:00Z',
    vmCutoverStart: '2026-08-06T11:00:00Z',
    vmCutoverEnd: '2026-08-06T12:00:00Z',
    adminInitiatedCutOver: false,
    disconnectSourceNetwork: true,
    dataOnly: true,
    fallbackToDHCP: true,
    securityGroups: ['sg-1'],
    serverGroup: 'sg-group',
    postMigrationScript: 'echo hi',
    postMigrationAction: { renameVm: true, suffix: '-old', moveToFolder: true, folderName: 'f' },
    periodicSyncInterval: '30m',
    periodicSyncEnabled: true,
    networkPersistence: true,
    removeVMwareTools: true,
    acknowledgeNetworkConflictRisk: true,
    imageProfiles: ['profile-1'],
    networkOverridesPerVM: { 'vm-1': [{ interfaceIndex: 0, preserveIP: true, preserveMAC: true }] },
    preserveSourceTags: true,
    customMetadata: { owner: 'team-a' }
  }).spec as Record<string, unknown>

  const retrySpec = buildRetryPlanSpec({
    params: {
      dataCopyMethod: 'hot',
      dataCopyStartTime: '2026-08-06T10:00:00Z',
      cutoverOption: CUTOVER_TYPES.TIME_WINDOW,
      cutoverStartTime: '2026-08-06T11:00:00Z',
      cutoverEndTime: '2026-08-06T12:00:00Z',
      disconnectSourceNetwork: true,
      dataOnly: true,
      fallbackToDHCP: true,
      securityGroups: ['sg-1'],
      serverGroup: 'sg-group',
      postMigrationScript: 'echo hi',
      postMigrationAction: { renameVm: true, suffix: '-old', moveToFolder: true, folderName: 'f' },
      periodicSyncInterval: '30m',
      networkPersistence: true,
      removeVMwareTools: true,
      acknowledgeNetworkConflictRisk: true,
      imageProfiles: ['profile-1'],
      preserveSourceTags: true,
      customMetadata: [{ key: 'owner', value: 'team-a' }],
      vms: [
        {
          name: 'vm-1',
          vmKey: 'vm-1',
          preserveIp: { 0: true },
          networkInterfaces: [{ ipAddress: ['10.0.0.5'] }]
        }
      ]
    } as unknown as Partial<FormValues>,
    selectedMigrationOptions: {
      dataCopyMethod: true,
      dataCopyStartTime: true,
      cutoverOption: true,
      cutoverStartTime: true,
      cutoverEndTime: true,
      postMigrationScript: true,
      periodicSyncEnabled: true,
      postMigrationAction: { renameVm: true, suffix: true, moveToFolder: true, folderName: true }
    },
    retryPlan: makePlan()
  }) as unknown as Record<string, unknown>

  const missing = (from: Record<string, unknown>, against: Record<string, unknown>) =>
    Object.keys(from)
      .filter((key) => !CALL_SITE_KEYS.includes(key))
      .filter((key) => !(key in against))

  it('sets every top-level plan spec field the create path sets', () => {
    expect(missing(createSpec, retrySpec)).toEqual([])
  })

  it('sets every migrationStrategy field the create path sets', () => {
    expect(
      missing(
        createSpec.migrationStrategy as Record<string, unknown>,
        retrySpec.migrationStrategy as Record<string, unknown>
      )
    ).toEqual([])
  })

  it('sets every advancedOptions field the create path sets', () => {
    expect(
      missing(
        createSpec.advancedOptions as Record<string, unknown>,
        retrySpec.advancedOptions as Record<string, unknown>
      )
    ).toEqual([])
  })
})
