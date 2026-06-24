import { useQuery, UseQueryOptions, UseQueryResult } from '@tanstack/react-query'
import { listSSHKeyPairs } from 'src/api/sshKeyPairs/sshKeyPairs'
import { SSHKeyPair } from 'src/api/sshKeyPairs/model'

export const SSH_KEY_PAIRS_QUERY_KEY = ['ssh-keypairs']

type Options = Omit<UseQueryOptions<SSHKeyPair[]>, 'queryKey' | 'queryFn'>

export const useSSHKeyPairsQuery = (
  namespace = undefined,
  options: Options = {}
): UseQueryResult<SSHKeyPair[]> => {
  return useQuery<SSHKeyPair[]>({
    queryKey: [...SSH_KEY_PAIRS_QUERY_KEY, namespace],
    queryFn: async () => listSSHKeyPairs(namespace),
    staleTime: 0,
    refetchOnWindowFocus: true,
    ...options
  })
}
