import { useMutation, useQueryClient } from '@tanstack/react-query'
import {
  postMigrationBlueprint,
  putMigrationBlueprint,
  deleteMigrationBlueprint
} from 'src/api/migration-blueprints/migrationBlueprints'
import { createMigrationBlueprintJson } from 'src/api/migration-blueprints/helpers'
import {
  sanitizeTemplateName,
  savedTemplateInputToBlueprintSpec,
  uniqueTemplateName
} from '../api/migration-blueprints/adapters'
import { MIGRATION_TEMPLATES_QUERY_KEY } from './useMigrationTemplatesQuery'
import type { SavedTemplate, SaveAsTemplateInput } from '../api/migration-blueprints/types'

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

// Updates an existing blueprint in place (Edit Template flow) — keeps the k8s object
// name so the same template row is updated rather than a new one created.
export function useUpdateTemplate() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async ({ name, input }: { name: string; input: SaveAsTemplateInput }) => {
      const existing =
        queryClient.getQueryData<SavedTemplate[]>(MIGRATION_TEMPLATES_QUERY_KEY) || []

      const displayNameTaken = existing.some(
        (t) =>
          t.name !== name &&
          t.displayName.trim().toLowerCase() === input.displayName.trim().toLowerCase()
      )
      if (displayNameTaken) {
        throw new Error(`A template named "${input.displayName}" already exists.`)
      }

      const spec = savedTemplateInputToBlueprintSpec(input)
      const body = createMigrationBlueprintJson(name, spec)
      return putMigrationBlueprint(name, body)
    },
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
