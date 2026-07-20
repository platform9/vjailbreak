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
  metadata: { name: 'production-rhel-east', namespace: 'migration-system', creationTimestamp: '2026-06-01T00:00:00Z' },
  spec: {
    displayName: 'Production RHEL · East',
    description: 'Standard hot migration',
    vmwareRef: 'vcenter-east-creds',
    pcdRef: 'pcd-east-1-creds',
    targetPCDClusterName: 'cluster-prod-a',
    networkMappings: [{ source: 'vmnet-prod', target: 'net-prod-east-a' }],
    storageMappings: [{ source: 'east-nvme-ds01', target: 'ceph-nvme-east' }],
    migrationStrategy: { type: 'hot' },
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
      displayName: 'Production RHEL · East',
      sourceVCenter: 'vcenter-east-creds',
      destination: 'pcd-east-1-creds',
      targetCluster: 'cluster-prod-a',
      dataCopyMethod: 'hot',
      cutoverOption: CUTOVER_TYPES.IMMEDIATE,
      osFamily: 'linuxGuest',
      useGPU: false
    })
    expect(result.spec).toBe(blueprint.spec)
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
      pcdRef: 'pcd-1',
      targetPCDClusterName: 'cluster-a',
      migrationStrategy: { type: 'hot', adminInitiatedCutOver: false }
    })
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
