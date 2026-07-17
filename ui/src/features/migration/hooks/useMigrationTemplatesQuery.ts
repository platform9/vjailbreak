import { useQuery } from '@tanstack/react-query'
import { fetchSavedTemplates } from '../mock-templates/mockStore'

export const MIGRATION_TEMPLATES_QUERY_KEY = ['migration-templates', 'saved']

// Backed by an in-memory mock store today (see mock-templates/mockStore.ts) — swap
// fetchSavedTemplates() for a real getSavedMigrationTemplatesList() API call once the
// backend CRD fields exist, without changing any component that consumes this hook.
export function useMigrationTemplatesQuery() {
  return useQuery({
    queryKey: MIGRATION_TEMPLATES_QUERY_KEY,
    queryFn: fetchSavedTemplates,
    staleTime: 0
  })
}
