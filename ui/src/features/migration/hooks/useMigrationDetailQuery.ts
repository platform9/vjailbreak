import { useQuery, UseQueryResult } from '@tanstack/react-query'
import { getMigration } from '../api/migrations'
import { Migration, Phase } from '../api/migrations'
import { VJAILBREAK_DEFAULT_NAMESPACE } from 'src/api/constants'
import { FIVE_SECONDS, THIRTY_SECONDS } from 'src/constants'

const TERMINAL_PHASES = new Set<Phase>([Phase.Succeeded, Phase.Failed, Phase.ValidationFailed])

export const migrationDetailQueryKey = (migrationName: string) => ['migration', migrationName]

export const useMigrationDetailQuery = (
  migrationName: string,
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
): UseQueryResult<Migration> => {
  return useQuery<Migration>({
    queryKey: migrationDetailQueryKey(migrationName),
    queryFn: () => getMigration(migrationName, namespace),
    enabled: !!migrationName,
    staleTime: 0,
    refetchOnWindowFocus: true,
    refetchInterval: (query) => {
      const phase = query?.state?.data?.status?.phase as Phase | undefined
      if (phase && TERMINAL_PHASES.has(phase)) return THIRTY_SECONDS
      return FIVE_SECONDS
    }
  })
}
