import {
  useQuery,
  useQueryClient,
  UseQueryOptions,
  UseQueryResult,
} from "@tanstack/react-query"
import { getOpenstackCredentialsList } from "src/api/openstack-creds/openstackCreds"
import { OpenstackCreds } from "src/api/openstack-creds/model"

export const OPENSTACK_CREDS_QUERY_KEY = ["openstackCreds"]

type Options = Omit<UseQueryOptions<OpenstackCreds[]>, "queryKey" | "queryFn">

export const useOpenstackCredentialsQuery = (
  namespace = undefined,
  options: Options = {}
): UseQueryResult<OpenstackCreds[]> => {
  const queryClient = useQueryClient()
  
  return useQuery<OpenstackCreds[]>({
    queryKey: [...OPENSTACK_CREDS_QUERY_KEY, namespace],
    queryFn: async () => {
      try {
        const creds = await getOpenstackCredentialsList(namespace)
        // Filter out any credentials that might be in the process of being deleted
        return creds.filter(cred => cred.metadata.deletionTimestamp === undefined)
      } catch (error) {
        console.error("Error fetching OpenStack credentials:", error)
        return []
      }
    },
    staleTime: 5 * 60 * 1000, // 5 minutes instead of Infinity
    refetchOnWindowFocus: true,
    ...options,
  })
}

// Export the refresh function for manual refreshes
export const refreshOpenstackCredentials = (namespace?: string) => {
  const queryClient = useQueryClient()
  queryClient.invalidateQueries({ 
    queryKey: [...OPENSTACK_CREDS_QUERY_KEY, namespace] 
  })
}