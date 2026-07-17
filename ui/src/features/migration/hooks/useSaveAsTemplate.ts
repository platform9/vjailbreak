import { useMutation, useQueryClient } from '@tanstack/react-query'
import { createSavedTemplate } from '../mock-templates/mockStore'
import { MIGRATION_TEMPLATES_QUERY_KEY } from './useMigrationTemplatesQuery'
import type { SaveAsTemplateInput } from '../mock-templates/types'

export function useSaveAsTemplate() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (input: SaveAsTemplateInput) => createSavedTemplate(input),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: MIGRATION_TEMPLATES_QUERY_KEY })
    }
  })
}
