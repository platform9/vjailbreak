import { useMutation, useQueryClient } from "@tanstack/react-query"
import { triggerAdminCutover } from "./vjailbreakProxy"
import { VJAILBREAK_DEFAULT_NAMESPACE } from "../constants"

export const useTriggerAdminCutover = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({
      namespace = VJAILBREAK_DEFAULT_NAMESPACE,
      migrationName,
    }: {
      namespace?: string
      migrationName: string
    }) => triggerAdminCutover(namespace, migrationName),
    onSuccess: () => {
      // Invalidate migration plans to refresh the UI
      queryClient.invalidateQueries({ queryKey: ["migrationPlans"] })
      queryClient.invalidateQueries({ queryKey: ["migrationPlan"] })
    },
  })
}