import { useMutation, useQueryClient } from '@tanstack/react-query'
import { cloneSavedTemplate, deleteSavedTemplate } from '../mock-templates/mockStore'
import { MIGRATION_TEMPLATES_QUERY_KEY } from './useMigrationTemplatesQuery'

// Delete/clone mutations for the Templates tab detail drawer (US5).
export function useDeleteTemplate() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (name: string) => deleteSavedTemplate(name),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: MIGRATION_TEMPLATES_QUERY_KEY })
    }
  })
}

export function useCloneTemplate() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (name: string) => cloneSavedTemplate(name),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: MIGRATION_TEMPLATES_QUERY_KEY })
    }
  })
}
