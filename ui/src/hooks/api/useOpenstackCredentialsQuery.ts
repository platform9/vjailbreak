import { useQuery, useQueryClient, UseQueryOptions, UseQueryResult } from "@tanstack/react-query"
import { getOpenstackCredentialsList } from "src/api/openstack-creds/openstackCreds"
import { OpenstackCreds } from "src/api/openstack-creds/model"

export const OPENSTACK_CREDS_QUERY_KEY = ["openstackCreds"]

type Options = Omit<UseQueryOptions<OpenstackCreds[]>, "queryKey" | "queryFn">

/**
 * Hook to fetch OpenStack credentials with proper TypeScript types
 */
export const useOpenstackCredentialsQuery = (
  namespace?: string,
  options: Options = {}
): UseQueryResult<OpenstackCreds[]> => {
  // Initialize query client for the refresh function
  useQueryClient()
  
  return useQuery<OpenstackCreds[]>({
    queryKey: [...OPENSTACK_CREDS_QUERY_KEY, namespace],
    queryFn: async () => {
      try {
        const creds = await getOpenstackCredentialsList(namespace)
        // Filter out any credentials that might be in the process of being deleted
        return creds.filter(cred => !('deletionTimestamp' in (cred.metadata as any)))
      } catch (error) {
        console.error("Error fetching OpenStack credentials:", error)
        return []
      }
    },
    staleTime: 5 * 60 * 1000, // 5 minutes
    refetchOnWindowFocus: true,
    ...options,
  })
}

/**
 * Function to manually refresh OpenStack credentials
 */
export const refreshOpenstackCredentials = (namespace?: string) => {
  const queryClient = useQueryClient()
  queryClient.invalidateQueries({ 
    queryKey: [...OPENSTACK_CREDS_QUERY_KEY, namespace].filter(Boolean) 
  })
}