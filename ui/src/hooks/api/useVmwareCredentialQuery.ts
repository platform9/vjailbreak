import { useQuery, UseQueryOptions, UseQueryResult } from '@tanstack/react-query'
import { getVmwareCredentials } from 'src/api/vmware-creds/vmwareCreds'
import { VMwareCreds } from 'src/api/vmware-creds/model'

export const vmwareCredQueryKey = (name: string, namespace?: string) =>
  ['vmwareCred', name, namespace]

type Options = Omit<UseQueryOptions<VMwareCreds>, 'queryKey' | 'queryFn'>

export const useVmwareCredentialQuery = (
  name: string,
  namespace?: string,
  options: Options = {}
): UseQueryResult<VMwareCreds> => {
  return useQuery<VMwareCreds>({
    queryKey: vmwareCredQueryKey(name, namespace),
    queryFn: async () => getVmwareCredentials(name, namespace),
    staleTime: Infinity,
    refetchOnWindowFocus: true,
    ...options,
  })
}
