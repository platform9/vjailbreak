import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
  cloneSavedTemplate,
  createSavedTemplate,
  deleteSavedTemplate,
  fetchSavedTemplates,
  markSavedTemplateUsed
} from './mockStore'
import { CUTOVER_TYPES } from '../constants'
import type { SaveAsTemplateInput } from './types'

const baseInput: SaveAsTemplateInput = {
  displayName: 'Test Template',
  sourceVCenter: 'vcenter.example.com',
  destination: 'pcd-1',
  tenantProject: 'proj',
  targetCluster: 'cluster-a',
  networkMappings: [],
  storageMappings: [],
  dataCopyMethod: 'hot',
  cutoverOption: CUTOVER_TYPES.IMMEDIATE
}

describe('mockStore', () => {
  beforeEach(() => {
    vi.useRealTimers()
  })

  it('creates a template and returns it in the list', async () => {
    const before = await fetchSavedTemplates()
    const created = await createSavedTemplate(baseInput)
    const after = await fetchSavedTemplates()

    expect(after).toHaveLength(before.length + 1)
    expect(after[0]).toBe(created)
    expect(created.displayName).toBe('Test Template')
    expect(created.timesUsed).toBe(0)

    await deleteSavedTemplate(created.name)
  })

  it('rejects a duplicate display name', async () => {
    const created = await createSavedTemplate({ ...baseInput, displayName: 'Duplicate Name' })
    await expect(
      createSavedTemplate({ ...baseInput, displayName: 'Duplicate Name' })
    ).rejects.toThrow(/already exists/)
    await deleteSavedTemplate(created.name)
  })

  it('deletes a template', async () => {
    const created = await createSavedTemplate({ ...baseInput, displayName: 'To Delete' })
    await deleteSavedTemplate(created.name)
    const after = await fetchSavedTemplates()
    expect(after.find((t) => t.name === created.name)).toBeUndefined()
  })

  it('clones a template with an independent name and reset usage stats', async () => {
    const original = await createSavedTemplate({ ...baseInput, displayName: 'Original' })
    await markSavedTemplateUsed(original.name)

    const clone = await cloneSavedTemplate(original.name)

    expect(clone.name).not.toBe(original.name)
    expect(clone.displayName).toBe('Original (copy)')
    expect(clone.timesUsed).toBe(0)
    expect(clone.lastUsedAt).toBeUndefined()
    expect(clone.networkMappings).toEqual(original.networkMappings)

    await deleteSavedTemplate(original.name)
    await deleteSavedTemplate(clone.name)
  })

  it('throws when cloning a template that no longer exists', async () => {
    await expect(cloneSavedTemplate('does-not-exist')).rejects.toThrow(/no longer exists/)
  })

  it('increments timesUsed and sets lastUsedAt on markSavedTemplateUsed', async () => {
    const created = await createSavedTemplate({ ...baseInput, displayName: 'Usage Test' })
    await markSavedTemplateUsed(created.name)

    const [updated] = (await fetchSavedTemplates()).filter((t) => t.name === created.name)
    expect(updated.timesUsed).toBe(1)
    expect(updated.lastUsedAt).toBeDefined()

    await deleteSavedTemplate(created.name)
  })
})
