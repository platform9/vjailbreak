import { useQuery } from '@tanstack/react-query'
import { useVmwareCredentialsQuery } from 'src/hooks/api/useVmwareCredentialsQuery'
import { getVMwareClusters } from 'src/api/vmware-clusters/vmwareClusters'
import { VJAILBREAK_DEFAULT_NAMESPACE } from 'src/api/constants'

// Templates saved before MigrationForm started persisting the vCenter cluster's
// display name instead stored the k8s VMwareCluster object's raw metadata.name
// (format "<clusterName>-<datacenter>-<hash>", or "no-cluster-<datacenter>-<hash>"
// for standalone ESXi hosts) — see sourceClusterLabel in utils/templateLabels.ts.
// The datacenter/hash suffix can't be reliably split back off that string alone, so
// this looks the raw name up against the live VMwareCluster list and resolves it to
// the cluster's clean spec.name instead.
export function useSourceClusterNameLookup(): Record<string, string> {
  const { data: vmwareCreds } = useVmwareCredentialsQuery()
  const credNames = (vmwareCreds || []).map((cred) => cred.metadata?.name).filter(Boolean)

  const { data } = useQuery({
    queryKey: ['source-cluster-name-lookup', credNames],
    queryFn: async () => {
      const lookup: Record<string, string> = {}
      await Promise.all(
        credNames.map(async (credName) => {
          const clustersResponse = await getVMwareClusters(VJAILBREAK_DEFAULT_NAMESPACE, credName)
          clustersResponse.items.forEach((cluster) => {
            lookup[cluster.metadata.name] = cluster.spec.name
          })
        })
      )
      return lookup
    },
    enabled: credNames.length > 0
  })

  return data || {}
}
