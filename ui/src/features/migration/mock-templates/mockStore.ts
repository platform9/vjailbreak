import { mockSavedTemplates } from './mockData'
import type { SavedTemplate, SaveAsTemplateInput } from './types'

// In-memory stand-in for the future MigrationTemplate list/create/delete/clone API.
// Module-scoped so the "server" state survives across component remounts within a
// session, exactly like the real k8s API would. Swap the bodies of these functions
// for real axios calls (see plan.md's api/migration-templates/migrationTemplates.ts
// contract) without changing any caller.

let store: SavedTemplate[] = [...mockSavedTemplates]

const sanitizeToName = (displayName: string) =>
  displayName
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/(^-|-$)/g, '') || 'template'

const uniqueName = (base: string) => {
  let candidate = base
  let suffix = 2
  while (store.some((t) => t.name === candidate)) {
    candidate = `${base}-${suffix}`
    suffix += 1
  }
  return candidate
}

export async function fetchSavedTemplates(): Promise<SavedTemplate[]> {
  return store
}

export async function createSavedTemplate(input: SaveAsTemplateInput): Promise<SavedTemplate> {
  const displayNameTaken = store.some(
    (t) => t.displayName.trim().toLowerCase() === input.displayName.trim().toLowerCase()
  )
  if (displayNameTaken) {
    throw new Error(`A template named "${input.displayName}" already exists.`)
  }

  const template: SavedTemplate = {
    ...input,
    name: uniqueName(sanitizeToName(input.displayName)),
    createdAt: new Date().toISOString(),
    timesUsed: 0
  }
  store = [template, ...store]
  return template
}

export async function deleteSavedTemplate(name: string): Promise<void> {
  store = store.filter((t) => t.name !== name)
}

export async function cloneSavedTemplate(name: string): Promise<SavedTemplate> {
  const source = store.find((t) => t.name === name)
  if (!source) throw new Error(`Template "${name}" no longer exists.`)

  const baseDisplayName = `${source.displayName} (copy)`
  let displayName = baseDisplayName
  let suffix = 2
  while (store.some((t) => t.displayName === displayName)) {
    displayName = `${baseDisplayName} ${suffix}`
    suffix += 1
  }

  const clone: SavedTemplate = {
    ...source,
    name: uniqueName(sanitizeToName(displayName)),
    displayName,
    createdAt: new Date().toISOString(),
    timesUsed: 0,
    lastUsedAt: undefined
  }
  store = [clone, ...store]
  return clone
}

export async function markSavedTemplateUsed(name: string): Promise<void> {
  store = store.map((t) =>
    t.name === name ? { ...t, timesUsed: t.timesUsed + 1, lastUsedAt: new Date().toISOString() } : t
  )
}
