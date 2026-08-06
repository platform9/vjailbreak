import { describe, expect, it } from 'vitest'
import { buildRetryFormState, buildRetryPlanSpec } from './retryFormState'
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
