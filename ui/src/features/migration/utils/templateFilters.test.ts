import { describe, expect, it } from 'vitest'
import { filterTemplates, sortTemplates } from './templateFilters'
import { CUTOVER_TYPES } from '../constants'
import type { SavedTemplate } from '../mock-templates/types'

const makeTemplate = (overrides: Partial<SavedTemplate>): SavedTemplate => ({
  name: 'template',
  displayName: 'Template',
  createdAt: '2026-01-01T00:00:00Z',
  timesUsed: 0,
  sourceVCenter: 'vcenter.example.com',
  destination: 'pcd-1',
  tenantProject: 'proj',
  targetCluster: 'cluster-a',
  networkMappings: [],
  storageMappings: [],
  dataCopyMethod: 'hot',
  cutoverOption: CUTOVER_TYPES.ADMIN_INITIATED,
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

describe('sortTemplates', () => {
  const templates = [
    makeTemplate({
      name: 'a',
      displayName: 'Charlie',
      timesUsed: 5,
      createdAt: '2026-01-01T00:00:00Z',
      lastUsedAt: '2026-01-10T00:00:00Z'
    }),
    makeTemplate({
      name: 'b',
      displayName: 'Alpha',
      timesUsed: 20,
      createdAt: '2026-03-01T00:00:00Z',
      lastUsedAt: '2026-02-01T00:00:00Z'
    }),
    makeTemplate({
      name: 'c',
      displayName: 'Bravo',
      timesUsed: 1,
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

  it('sorts by timesUsed descending', () => {
    expect(sortTemplates(templates, 'timesUsed').map((t) => t.name)).toEqual(['b', 'a', 'c'])
  })

  it('sorts by created descending', () => {
    expect(sortTemplates(templates, 'created').map((t) => t.name)).toEqual(['b', 'c', 'a'])
  })

  it('sorts by lastUsed descending, treating missing lastUsedAt as oldest', () => {
    expect(sortTemplates(templates, 'lastUsed').map((t) => t.name)).toEqual(['b', 'a', 'c'])
  })
})
