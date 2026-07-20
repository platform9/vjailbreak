import { useMutation, useQueryClient } from '@tanstack/react-query'
import { postMigrationBlueprint } from 'src/api/migration-blueprints/migrationBlueprints'
import { createMigrationBlueprintJson } from 'src/api/migration-blueprints/helpers'
import {
  savedTemplateInputToBlueprintSpec,
  sanitizeTemplateName,
  uniqueTemplateName
} from '../api/migration-blueprints/adapters'
import { MIGRATION_TEMPLATES_QUERY_KEY } from './useMigrationTemplatesQuery'
import type { SavedTemplate, SaveAsTemplateInput } from '../api/migration-blueprints/types'

export function useSaveAsTemplate() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (input: SaveAsTemplateInput) => {
      const existing =
        queryClient.getQueryData<SavedTemplate[]>(MIGRATION_TEMPLATES_QUERY_KEY) || []

      const displayNameTaken = existing.some(
        (t) => t.displayName.trim().toLowerCase() === input.displayName.trim().toLowerCase()
      )
      if (displayNameTaken) {
        throw new Error(`A template named "${input.displayName}" already exists.`)
      }

      const name = uniqueTemplateName(
        sanitizeTemplateName(input.displayName),
        existing.map((t) => t.name)
      )
      const spec = savedTemplateInputToBlueprintSpec(input)
      const body = createMigrationBlueprintJson(name, spec)
      return postMigrationBlueprint(body)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: MIGRATION_TEMPLATES_QUERY_KEY })
    }
  })
}
