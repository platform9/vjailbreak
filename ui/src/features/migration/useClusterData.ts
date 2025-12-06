import { useState, useEffect } from 'react'
import { getVmwareCredentialsList } from 'src/api/vmware-creds/vmwareCreds'
import { getVMwareClusters } from 'src/api/vmware-clusters/vmwareClusters'
import { getPCDClusters } from 'src/api/pcd-clusters'
import { getOpenstackCredentials } from 'src/api/openstack-creds/openstackCreds'
import { VJAILBREAK_DEFAULT_NAMESPACE } from 'src/api/constants'
import { VMwareCluster } from 'src/api/vmware-clusters/model'
import { PCDCluster } from 'src/api/pcd-clusters/model'

export interface SourceDataItem {
  credName: string
  datacenter: string
  vcenterName: string
  clusters: {
    id: string
    name: string
    displayName: string
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

export const useClusterData = (autoFetch: boolean = true): UseClusterDataReturn => {
  const [sourceData, setSourceData] = useState<SourceDataItem[]>([])
  const [pcdData, setPcdData] = useState<PcdDataItem[]>([])
  const [loadingVMware, setLoadingVMware] = useState(false)
  const [loadingPCD, setLoadingPCD] = useState(false)
  const [error, setError] = useState<string | null>(null)
  // Removed fetchSecrets function - no longer needed
  const fetchSourceData = async () => {
    setLoadingVMware(true)
    setError(null)
    try {
      const vmwareCreds = await getVmwareCredentialsList(VJAILBREAK_DEFAULT_NAMESPACE)

      if (!vmwareCreds || vmwareCreds.length === 0) {
        setSourceData([])
        return
      }

      const transformedData: SourceDataItem[] = vmwareCreds.map((cred) => {
        const credName = cred.metadata?.name || 'Unknown'
        const vcenterName = cred.spec?.hostName || credName
        const datacenter = cred.spec.datacenter || ''

        return {
          credName,
          datacenter,
          vcenterName,
          clusters: []
        }
      })

      const sourceDataPromises = transformedData.map(async (item) => {
        const clustersResponse = await getVMwareClusters(VJAILBREAK_DEFAULT_NAMESPACE, item.credName)

        const clusters = clustersResponse.items.map((cluster: VMwareCluster) => ({
          id: `${item.credName}:${cluster.metadata.name}`,
          name: cluster.metadata.name,
          displayName: cluster.spec.name
        }))

        return {
          credName: item.credName,
          datacenter: item.datacenter,
          vcenterName: item.vcenterName,
          clusters
        }
      })

      const newSourceData = await Promise.all(sourceDataPromises)
      setSourceData(newSourceData.filter((item) => item.clusters.length > 0))
    } catch (error) {
      console.error('Failed to fetch VMware cluster data:', error)
      setError('Failed to fetch VMware cluster data')
    } finally {
      setLoadingVMware(false)
    }
  }

  const fetchPcdData = async () => {
    setLoadingPCD(true)
    setError(null)
    try {
      const pcdClusters = await getPCDClusters(VJAILBREAK_DEFAULT_NAMESPACE)

      if (!pcdClusters || pcdClusters.items.length === 0) {
        setPcdData([])
        return
      }

      const clusterDataPromises = pcdClusters.items.map(async (cluster: PCDCluster) => {
        const clusterName = cluster.spec.clusterName
        const openstackCredName =
          cluster.metadata.labels?.['vjailbreak.k8s.pf9.io/openstackcreds'] || ''

        let tenantName = ''
        if (openstackCredName) {
          try {
            const openstackCreds = await getOpenstackCredentials(
              openstackCredName,
              VJAILBREAK_DEFAULT_NAMESPACE
            )
            tenantName = openstackCreds?.spec?.projectName || ''
          } catch (error) {
            console.error(`Failed to fetch OpenStack credentials for ${openstackCredName}:`, error)
          }
        }

        return {
          id: openstackCredName + ' - ' + tenantName + ' - ' + clusterName,
          name: clusterName,
          openstackCredName: openstackCredName,
          tenantName: tenantName
        }
      })

      const clusterData = await Promise.all(clusterDataPromises)
      setPcdData(clusterData)
    } catch (error) {
      console.error('Failed to fetch PCD clusters:', error)
      setError('Failed to fetch PCD clusters')
    } finally {
      setLoadingPCD(false)
    }
  }

  const refetchAll = async () => {
    // Fetch source and PCD data in parallel
    await Promise.all([fetchSourceData(), fetchPcdData()])
  }

  // Auto-fetch on mount if enabled
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
    refetchAll
  }
}
