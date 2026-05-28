import { useQuery, UseQueryOptions, UseQueryResult } from '@tanstack/react-query'
import { getProxyVMList } from 'src/api/proxyvms/proxyVMs'
import { ProxyVM } from 'src/api/proxyvms/model'

export const PROXY_VMS_QUERY_KEY = ['proxyvms']

type Options = Omit<UseQueryOptions<ProxyVM[]>, 'queryKey' | 'queryFn'>

export const useProxyVMsQuery = (
  namespace = undefined,
  options: Options = {}
): UseQueryResult<ProxyVM[]> => {
  return useQuery<ProxyVM[]>({
    queryKey: [...PROXY_VMS_QUERY_KEY, namespace],
    queryFn: async () => getProxyVMList(namespace),
    staleTime: 0,
    refetchOnWindowFocus: true,
    refetchInterval: (query) => {
      const items = query.state.data ?? []
      const hasActiveItem = items.some(
        (vm) =>
          vm.status?.validationStatus === 'Pending' ||
          vm.status?.validationStatus === 'Verifying'
      )
      return hasActiveItem ? 5000 : false
    },
    ...options
  })
}
