import { useQuery, UseQueryOptions, UseQueryResult } from '@tanstack/react-query'
import { getArrayCredentialsList } from 'src/api/array-creds/arrayCreds'
import { ArrayCreds } from 'src/api/array-creds/model'

export const ARRAY_CREDS_QUERY_KEY = ['arrayCreds']

type Options = Omit<UseQueryOptions<ArrayCreds[]>, 'queryKey' | 'queryFn'>

export const useArrayCredentialsQuery = (
  namespace?: string,
  options: Options = {}
): UseQueryResult<ArrayCreds[]> => {
  return useQuery<ArrayCreds[]>({
    queryKey: [...ARRAY_CREDS_QUERY_KEY, namespace],
    queryFn: async () => getArrayCredentialsList(namespace),
    staleTime: Infinity,
    refetchOnWindowFocus: true,
    ...options
  })
}
