import { describe, expect, it } from 'vitest'
import { filterTemplates, sortTemplates } from './templateFilters'
import { CUTOVER_TYPES } from '../constants'
import type { SavedTemplate } from '../api/migration-blueprints/types'

const makeTemplate = (overrides: Partial<SavedTemplate>): SavedTemplate => ({
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

describe('filterTemplates', () => {
  const templates = [
    makeTemplate({
      name: 'a',
      displayName: 'Production RHEL · East'
    }),
    makeTemplate({
      name: 'b',
      displayName: 'Dev sandbox dry-run'
    }),
    makeTemplate({
      name: 'c',
      displayName: 'My draft config',
      description: 'finance wave'
    })
  ]

  it('returns all templates for an empty query', () => {
    expect(filterTemplates(templates, '')).toHaveLength(3)
  })

  it('matches search query against name', () => {
    expect(filterTemplates(templates, 'RHEL')).toEqual([templates[0]])
  })

  it('matches search query against description', () => {
    expect(filterTemplates(templates, 'finance')).toEqual([templates[2]])
  })

  it('returns empty array when nothing matches', () => {
    expect(filterTemplates(templates, 'nonexistent')).toEqual([])
  })
})
describe('filterTemplates copy method filter', () => {
  const templates = [
    makeTemplate({ name: 'a', displayName: 'Hot one', dataCopyMethod: 'hot' }),
    makeTemplate({ name: 'b', displayName: 'Cold one', dataCopyMethod: 'cold' }),
    makeTemplate({ name: 'c', displayName: 'Mock one', dataCopyMethod: 'mock' })
  ]

  it('returns all templates when filter is "all"', () => {
    expect(filterTemplates(templates, '', 'all')).toHaveLength(3)
  })

  it('returns only templates matching the copy method', () => {
    expect(filterTemplates(templates, '', 'cold')).toEqual([templates[1]])
  })

  it('combines copy method filter with search query', () => {
    expect(filterTemplates(templates, 'one', 'hot')).toEqual([templates[0]])
  })

  it('defaults to "all" when copy method is omitted', () => {
    expect(filterTemplates(templates, '')).toHaveLength(3)
  })
})

describe('sortTemplates', () => {
  const templates = [
    makeTemplate({
      name: 'a',
      displayName: 'Charlie',
      createdAt: '2026-01-01T00:00:00Z'
    }),
    makeTemplate({
      name: 'b',
      displayName: 'Alpha',
      createdAt: '2026-03-01T00:00:00Z'
    }),
    makeTemplate({
      name: 'c',
      displayName: 'Bravo',
      createdAt: '2026-02-01T00:00:00Z'
    })
  ]

  it('does not mutate the input array', () => {
    const copy = [...templates]
    sortTemplates(templates, 'name')
    expect(templates).toEqual(copy)
  })

  it('sorts by name ascending', () => {
    expect(sortTemplates(templates, 'name').map((t) => t.name)).toEqual(['b', 'c', 'a'])
  })

  it('sorts by created descending', () => {
    expect(sortTemplates(templates, 'created').map((t) => t.name)).toEqual(['b', 'c', 'a'])
  })
})
