import { useState, useEffect } from "react"
import { getVmwareCredentialsList } from "src/api/vmware-creds/vmwareCreds"
import { getVMwareClusters } from "src/api/vmware-clusters/vmwareClusters"
import { getPCDClusters } from "src/api/pcd-clusters"
import { getSecret, getSecrets } from "src/api/secrets/secrets"
import { VJAILBREAK_DEFAULT_NAMESPACE } from "src/api/constants"
import { VMwareCreds } from "src/api/vmware-creds/model"
import { VMwareCluster } from "src/api/vmware-clusters/model"
import { PCDCluster } from "src/api/pcd-clusters/model"

export interface SourceDataItem {
  credName: string
  datacenter: string
  vcenterName: string
  clusters: {
    id: string
    name: string
  }[]
}

export interface PcdDataItem {
  id: string
  name: string
  openstackCredName: string
  tenantName: string
}

interface ClusterDataState {
  sourceData: SourceDataItem[]
  pcdData: PcdDataItem[]
  loadingVMware: boolean
  loadingPCD: boolean
  error: string | null
}

interface ClusterDataActions {
  refetchSourceData: () => Promise<void>
  refetchPcdData: () => Promise<void>
  refetchAll: () => Promise<void>
}

export type UseClusterDataReturn = ClusterDataState & ClusterDataActions

export const useClusterData = (
  autoFetch: boolean = true
): UseClusterDataReturn => {
  const [sourceData, setSourceData] = useState<SourceDataItem[]>([])
  const [pcdData, setPcdData] = useState<PcdDataItem[]>([])
  const [loadingVMware, setLoadingVMware] = useState(false)
  const [loadingPCD, setLoadingPCD] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const fetchSourceData = async () => {
    setLoadingVMware(true)
    setError(null)
    try {
      const vmwareCreds = await getVmwareCredentialsList(
        VJAILBREAK_DEFAULT_NAMESPACE
      )

      if (!vmwareCreds || vmwareCreds.length === 0) {
        setSourceData([])
        return
      }

      const sourceDataPromises = vmwareCreds.map(async (cred: VMwareCreds) => {
        const credName = cred.metadata.name
        const datacenter = cred.spec.datacenter || credName
        let vcenterName = credName

        if (cred.spec.secretRef?.name) {
          try {
            const secret = await getSecret(
              cred.spec.secretRef.name,
              VJAILBREAK_DEFAULT_NAMESPACE
            )
            if (secret?.data?.VCENTER_HOST) {
              vcenterName = atob(secret.data.VCENTER_HOST)
            }
          } catch (error) {
            console.error(
              `Failed to fetch secret for credential ${credName}:`,
              error
            )
          }
        }

        const clustersResponse = await getVMwareClusters(
          VJAILBREAK_DEFAULT_NAMESPACE,
          credName
        )

        const clusters = clustersResponse.items.map(
          (cluster: VMwareCluster) => ({
            id: `${credName}:${cluster.metadata.name}`,
            name: cluster.spec.name,
          })
        )

        return {
          credName,
          datacenter,
          vcenterName,
          clusters,
        }
      })

      const newSourceData = await Promise.all(sourceDataPromises)
      setSourceData(newSourceData.filter((item) => item.clusters.length > 0))
    } catch (error) {
      console.error("Failed to fetch VMware cluster data:", error)
      setError("Failed to fetch VMware cluster data")
    } finally {
      setLoadingVMware(false)
    }
  }

  const fetchPcdData = async () => {
    setLoadingPCD(true)
    setError(null)
    try {
        const pcdClusters = await getPCDClusters(VJAILBREAK_DEFAULT_NAMESPACE)

        if (!pcdClusters || !pcdClusters.items || pcdClusters.items.length === 0) {
            setPcdData([])
            return
        }

        const currentSecrets = await getSecrets(VJAILBREAK_DEFAULT_NAMESPACE)

        const clusterDataPromises = pcdClusters.items.map(
            async (cluster: PCDCluster) => {
                const clusterName = cluster.spec.clusterName
                const openstackCredName =
                    cluster.metadata.labels?.["vjailbreak.k8s.pf9.io/openstackcreds"] || ""

                let tenantName = ""

                if (openstackCredName) {
                    // ✅ Re-introducing this comment to suppress the type error, just like in your original code.
                    // @ts-expect-error - currentSecrets is a SecretList and its type is not fully inferred here.
                    const secret = currentSecrets?.items?.find((s) =>
                        s?.metadata?.name?.includes(openstackCredName)
                    )

                    if (secret?.data?.OS_TENANT_NAME) {
                        tenantName = atob(secret.data.OS_TENANT_NAME)
                    } else if (secret?.data?.OS_PROJECT_NAME) {
                        tenantName = atob(secret.data.OS_PROJECT_NAME)
                    }
                }

                return {
                    id: openstackCredName + " - " + tenantName + " - " + clusterName,
                    name: clusterName,
                    openstackCredName: openstackCredName,
                    tenantName: tenantName,
                }
            }
        )

        const clusterData = await Promise.all(clusterDataPromises)
        setPcdData(clusterData)
        
    } catch (error) {
        console.error("Failed to fetch PCD clusters:", error)
        setError("Failed to fetch PCD clusters")
    } finally {
        setLoadingPCD(false)
    }
}

  const refetchAll = async () => {
    await Promise.all([fetchSourceData(), fetchPcdData()])
  }

  useEffect(() => {
    if (autoFetch) {
      refetchAll()
    }
  }, [autoFetch])

  return {
    sourceData,
    pcdData,
    loadingVMware,
    loadingPCD,
    error,
    refetchSourceData: fetchSourceData,
    refetchPcdData: fetchPcdData,
    refetchAll,
  }
}