import type { SavedTemplate } from '../api/migration-blueprints/types'

export type TemplateSortKey = 'name' | 'created'

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
    case 'created':
    default:
      return sorted.sort(
        (a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime()
      )
  }
}
