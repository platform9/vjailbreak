import { useState, useEffect } from 'react'
import { GridRowSelectionModel } from '@mui/x-data-grid'
import { getVMwareHosts } from 'src/api/vmware-hosts/vmwareHosts'
import { VMwareHost } from 'src/api/vmware-hosts/model'
import { getVMwareMachines } from 'src/api/vmware-machines/vmwareMachines'
import { VMwareMachine } from 'src/api/vmware-machines/model'
import { VJAILBREAK_DEFAULT_NAMESPACE } from 'src/api/constants'
import { getBMConfigList, getBMConfig } from 'src/api/bmconfig/bmconfig'
import { BMConfig } from 'src/api/bmconfig/model'
import { getOpenstackCredentials } from 'src/api/openstack-creds/openstackCreds'
import { OpenstackCreds } from 'src/api/openstack-creds/model'
import { SourceDataItem } from './useClusterData'
import type { ESXHost, VM } from '../types'

interface UseRollingFormDataParams {
  open: boolean
  sourceCluster: string
  sourceData: SourceDataItem[]
  selectedVMs: GridRowSelectionModel
  setSelectedVMs: React.Dispatch<React.SetStateAction<GridRowSelectionModel>>
}

export function useRollingFormData({
  open,
  sourceCluster,
  sourceData,
  selectedVMs,
  setSelectedVMs
}: UseRollingFormDataParams) {
  const [loadingHosts, setLoadingHosts] = useState(false)
  const [loadingVMs, setLoadingVMs] = useState(false)

  const [orderedESXHosts, setOrderedESXHosts] = useState<ESXHost[]>([])
  const [vmsWithAssignments, setVmsWithAssignments] = useState<VM[]>([])

  const [maasConfigs, setMaasConfigs] = useState<BMConfig[]>([])
  const [selectedMaasConfig, setSelectedMaasConfig] = useState<BMConfig | null>(null)
  const [loadingMaasConfig, setLoadingMaasConfig] = useState(false)

  const [openstackCredData, setOpenstackCredData] = useState<OpenstackCreds | null>(null)
  const [loadingOpenstackDetails, setLoadingOpenstackDetails] = useState(false)

  const fetchMaasConfigs = async () => {
    try {
      setLoadingMaasConfig(true)
      const configs = await getBMConfigList(VJAILBREAK_DEFAULT_NAMESPACE)
      if (configs && configs.length > 0) {
        setMaasConfigs(configs)
        try {
          const config = await getBMConfig(configs[0].metadata.name, VJAILBREAK_DEFAULT_NAMESPACE)
          setSelectedMaasConfig(config)
        } catch (error) {
          console.error(`Failed to fetch Bare Metal config:`, error)
        }
      }
    } catch (error) {
      console.error('Failed to fetch Bare Metal configs:', error)
    } finally {
      setLoadingMaasConfig(false)
    }
  }

  const fetchClusterHosts = async () => {
    if (!sourceCluster) return

    setLoadingHosts(true)
    try {
      const parts = sourceCluster.split(':')
      const credName = parts[0]

      const sourceItem = sourceData.find((item) => item.credName === credName)
      const clusterObj = sourceItem?.clusters.find((cluster) => cluster.id === sourceCluster)
      const clusterName = clusterObj?.name

      if (!clusterName) {
        setOrderedESXHosts([])
        setLoadingHosts(false)
        return
      }

      const hostsResponse = await getVMwareHosts(
        VJAILBREAK_DEFAULT_NAMESPACE,
        // credName,
        '',
        clusterName
      )

      const mappedHosts: ESXHost[] = hostsResponse.items.map((host: VMwareHost) => ({
        id: host.metadata.name,
        name: host.spec.name,
        ip: '',
        bmcIp: '',
        maasState: 'Unknown',
        vms: host.status?.vmCount || 0,
        state: host.status?.state || 'Active',
        pcdHostConfigId: host.spec.hostConfigId
      }))

      setOrderedESXHosts(mappedHosts)
    } catch (error) {
      console.error('Failed to fetch cluster hosts:', error)
    } finally {
      setLoadingHosts(false)
    }
  }

  const fetchClusterVMs = async () => {
    if (!sourceCluster) return

    setLoadingVMs(true)
    try {
      const parts = sourceCluster.split(':')
      const credName = parts[0]

      const sourceItem = sourceData.find((item) => item.credName === credName)
      const clusterObj = sourceItem?.clusters.find((cluster) => cluster.id === sourceCluster)
      const clusterName = clusterObj?.name

      if (!clusterName) {
        setVmsWithAssignments([])
        setLoadingVMs(false)
        return
      }

      const vmsResponse = await getVMwareMachines(VJAILBREAK_DEFAULT_NAMESPACE, credName)

      const filteredVMs = vmsResponse.items.filter((vm: VMwareMachine) => {
        const clusterLabel = vm.metadata?.labels?.[`vjailbreak.k8s.pf9.io/vmware-cluster`]
        return clusterLabel === clusterName
      })

      const mappedVMs: VM[] = filteredVMs.map((vm: VMwareMachine) => {
        const esxiHost = vm.metadata?.labels?.[`vjailbreak.k8s.pf9.io/esxi-name`] || ''

        // Get flavor information from the VM spec
        const targetFlavorId = vm.spec.targetFlavorId || ''
        // We'll resolve flavor names later when openstackFlavors is available
        const flavorName = targetFlavorId || 'auto-assign'

        if (vm.spec.vms.name == 'nvidia-bcm-router') {
          console.log(vm.spec.vms.networkInterfaces)
        }

        // Get all IP addresses from network interfaces in comma-separated format
        const allIPs =
          vm.spec.vms.networkInterfaces && vm.spec.vms.networkInterfaces.length > 0
            ? vm.spec.vms.networkInterfaces
                .flatMap((nic) => (Array.isArray(nic.ipAddress) ? nic.ipAddress : []))
                .filter((ip) => ip && ip.trim() !== '') // Filter out empty/null IPs
                .join(', ')
            : vm.spec.vms.ipAddress || vm.spec.vms.assignedIp || '—'

        return {
          id: vm.metadata.name,
          name: vm.spec.vms.name || vm.metadata.name,
          ip: allIPs || '—',
          esxHost: esxiHost,
          networks: vm.spec.vms.networks,
          datastores: vm.spec.vms.datastores,
          cpu: vm.spec.vms.cpu,
          memory: vm.spec.vms.memory,
          osFamily: vm.spec.vms.osFamily,
          flavor: flavorName,
          targetFlavorId: targetFlavorId,
          powerState: vm.status.powerState === 'running' ? 'powered-on' : 'powered-off',
          ipValidationStatus: 'pending',
          ipValidationMessage: '',
          networkInterfaces: vm.spec.vms.networkInterfaces
        }
      })

      setVmsWithAssignments(mappedVMs)

      // Clean up persistent selection - remove VMs that no longer exist
      const availableVmIds = new Set(mappedVMs.map((vm) => vm.id))
      const cleanedSelection = selectedVMs.filter((vmId) => availableVmIds.has(String(vmId)))

      if (cleanedSelection.length !== selectedVMs.length) {
        setSelectedVMs(cleanedSelection)
      }
    } catch (error) {
      console.error('Failed to fetch cluster VMs:', error)
      setVmsWithAssignments([])
    } finally {
      setLoadingVMs(false)
    }
  }

  const fetchOpenstackCredentialDetails = async (credName: string) => {
    if (!credName) return

    setLoadingOpenstackDetails(true)
    try {
      const response = await getOpenstackCredentials(credName)
      setOpenstackCredData(response)
    } catch (error) {
      console.error('Failed to fetch OpenStack credential details:', error)
    } finally {
      setLoadingOpenstackDetails(false)
    }
  }

  const clearOpenstackCredData = () => {
    setOpenstackCredData(null)
  }

  useEffect(() => {
    if (open) {
      fetchMaasConfigs()
    }
  }, [open])

  useEffect(() => {
    if (sourceCluster) {
      fetchClusterHosts()
      fetchClusterVMs()
    }
  }, [sourceCluster])

  useEffect(() => {
    if (orderedESXHosts.length > 0 && vmsWithAssignments.length > 0) {
      const esxHostOrder = new Map()
      orderedESXHosts.forEach((host, index) => {
        esxHostOrder.set(host.id, index)
      })

      const sortedVMs = [...vmsWithAssignments].sort((a, b) => {
        const aHostIndex = esxHostOrder.get(a.esxHost) ?? 999
        const bHostIndex = esxHostOrder.get(b.esxHost) ?? 999
        return aHostIndex - bHostIndex
      })

      setVmsWithAssignments(sortedVMs)
    }
  }, [orderedESXHosts])

  return {
    loadingHosts,
    loadingVMs,
    orderedESXHosts,
    setOrderedESXHosts,
    vmsWithAssignments,
    setVmsWithAssignments,
    maasConfigs,
    selectedMaasConfig,
    loadingMaasConfig,
    openstackCredData,
    loadingOpenstackDetails,
    fetchMaasConfigs,
    fetchClusterHosts,
    fetchClusterVMs,
    fetchOpenstackCredentialDetails,
    clearOpenstackCredData
  }
}
