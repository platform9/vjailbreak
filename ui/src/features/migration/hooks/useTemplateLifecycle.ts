import { useMutation, useQueryClient } from '@tanstack/react-query'
import {
  postMigrationBlueprint,
  deleteMigrationBlueprint
} from 'src/api/migration-blueprints/migrationBlueprints'
import { createMigrationBlueprintJson } from 'src/api/migration-blueprints/helpers'
import { sanitizeTemplateName, uniqueTemplateName } from '../api/migration-blueprints/adapters'
import { MIGRATION_TEMPLATES_QUERY_KEY } from './useMigrationTemplatesQuery'
import type { SavedTemplate } from '../api/migration-blueprints/types'

// Delete/clone mutations for the Templates tab detail drawer (US5).
export function useDeleteTemplate() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (name: string) => deleteMigrationBlueprint(name),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: MIGRATION_TEMPLATES_QUERY_KEY })
    }
  })
}

export function useCloneTemplate() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (template: SavedTemplate) => {
      const existing =
        queryClient.getQueryData<SavedTemplate[]>(MIGRATION_TEMPLATES_QUERY_KEY) || []
      const existingDisplayNames = new Set(existing.map((t) => t.displayName))

      const baseDisplayName = `${template.displayName} (copy)`
      let displayName = baseDisplayName
      let suffix = 2
      while (existingDisplayNames.has(displayName)) {
        displayName = `${baseDisplayName} ${suffix}`
        suffix += 1
      }

      const name = uniqueTemplateName(
        sanitizeTemplateName(displayName),
        existing.map((t) => t.name)
      )
      const body = createMigrationBlueprintJson(name, { ...template.spec, displayName })
      return postMigrationBlueprint(body)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: MIGRATION_TEMPLATES_QUERY_KEY })
    }
  })
}
