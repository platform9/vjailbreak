import type { SavedTemplate } from '../mock-templates/types'

export type TemplateSortKey = 'lastUsed' | 'name' | 'timesUsed' | 'created'

export function filterTemplates(templates: SavedTemplate[], query: string): SavedTemplate[] {
  const trimmedQuery = query.trim().toLowerCase()

  return templates.filter((template) => {
    if (!trimmedQuery) return true

    return [template.displayName, template.description].some((field) =>
      field?.toLowerCase().includes(trimmedQuery)
    )
  })
}

export function sortTemplates(
  templates: SavedTemplate[],
  sortKey: TemplateSortKey
): SavedTemplate[] {
  const sorted = [...templates]

  switch (sortKey) {
    case 'name':
      return sorted.sort((a, b) => a.displayName.localeCompare(b.displayName))
    case 'timesUsed':
      return sorted.sort((a, b) => b.timesUsed - a.timesUsed)
    case 'created':
      return sorted.sort(
        (a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime()
      )
    case 'lastUsed':
    default:
      return sorted.sort((a, b) => {
        const aTime = a.lastUsedAt ? new Date(a.lastUsedAt).getTime() : 0
        const bTime = b.lastUsedAt ? new Date(b.lastUsedAt).getTime() : 0
        return bTime - aTime
      })
  }
}
