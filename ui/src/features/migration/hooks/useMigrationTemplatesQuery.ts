import { useQuery } from '@tanstack/react-query'
import { getMigrationBlueprintsList } from 'src/api/migration-blueprints/migrationBlueprints'
import { blueprintToSavedTemplate } from '../api/migration-blueprints/adapters'

export const MIGRATION_TEMPLATES_QUERY_KEY = ['migration-templates', 'saved']

export function useMigrationTemplatesQuery() {
  return useQuery({
    queryKey: MIGRATION_TEMPLATES_QUERY_KEY,
    queryFn: async () => {
      const blueprints = await getMigrationBlueprintsList()
      return blueprints.map(blueprintToSavedTemplate)
    },
    staleTime: 0
  })
}
