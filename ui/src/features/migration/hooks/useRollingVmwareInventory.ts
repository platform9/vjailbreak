import { useCallback, useEffect, useMemo, useState } from 'react'
import { getVMwareHosts } from 'src/api/vmware-hosts/vmwareHosts'
import type { VMwareHost } from 'src/api/vmware-hosts/model'
import { getVMwareMachines } from 'src/api/vmware-machines/vmwareMachines'
import type { VMwareMachine } from 'src/api/vmware-machines/model'
import { VJAILBREAK_DEFAULT_NAMESPACE } from 'src/api/constants'
import type { GridRowSelectionModel } from '@mui/x-data-grid'

export type ESXHost = {
  id: string
  name: string
  ip: string
  bmcIp: string
  maasState: string
  vms: number
  state: string
  pcdHostConfigName?: string
  pcdHostConfigId?: string
}

export type VmNetworkInterface = {
  mac: string
  network: string
  ipAddress: string[]
}

export type VM = {
  id: string
  name: string
  ip: string
  esxHost: string
  networks?: string[]
  datastores?: string[]
  cpu?: number
  memory?: number
  powerState: string
  osFamily?: string
  flavor?: string
  targetFlavorId?: string
  ipValidationStatus?: 'pending' | 'valid' | 'invalid' | 'validating'
  ipValidationMessage?: string
  networkInterfaces?: VmNetworkInterface[]
  preserveIp?: Record<number, boolean>
  preserveMac?: Record<number, boolean>
}

export function useRollingVmwareInventory({
  open,
  sourceCluster,
  sourceData,
  selectedVMs,
  setSelectedVMs
}: {
  open: boolean
  sourceCluster: string
  sourceData: Array<{ credName: string; clusters: Array<{ id: string; name: string }> }>
  selectedVMs: GridRowSelectionModel
  setSelectedVMs: (value: GridRowSelectionModel) => void
}) {
  const [loadingHosts, setLoadingHosts] = useState(false)
  const [loadingVMs, setLoadingVMs] = useState(false)
  const [orderedESXHosts, setOrderedESXHosts] = useState<ESXHost[]>([])
  const [vmsWithAssignments, setVmsWithAssignments] = useState<VM[]>([])

  const clusterContext = useMemo(() => {
    if (!sourceCluster) return { credName: '', clusterName: '' }

    const parts = sourceCluster.split(':')
    const credName = parts[0]

    const sourceItem = sourceData.find((item) => item.credName === credName)
    const clusterObj = sourceItem?.clusters.find((cluster) => cluster.id === sourceCluster)
    const clusterName = clusterObj?.name ?? ''

    return { credName, clusterName }
  }, [sourceCluster, sourceData])

  const fetchClusterHosts = useCallback(async () => {
    if (!sourceCluster) return

    setLoadingHosts(true)
    try {
      const { clusterName } = clusterContext

      if (!clusterName) {
        setOrderedESXHosts([])
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
  }, [clusterContext, sourceCluster])

  const fetchClusterVMs = useCallback(async () => {
    if (!sourceCluster) return

    setLoadingVMs(true)
    try {
      const { credName, clusterName } = clusterContext

      if (!clusterName) {
        setVmsWithAssignments([])
        return
      }

      const vmsResponse = await getVMwareMachines(VJAILBREAK_DEFAULT_NAMESPACE, credName)

      const filteredVMs = vmsResponse.items.filter((vm: VMwareMachine) => {
        const clusterLabel = vm.metadata?.labels?.[`vjailbreak.k8s.pf9.io/vmware-cluster`]
        return clusterLabel === clusterName
      })

      const mappedVMs: VM[] = filteredVMs.map((vm: VMwareMachine) => {
        const esxiHost = vm.metadata?.labels?.[`vjailbreak.k8s.pf9.io/esxi-name`] || ''

        const targetFlavorId = vm.spec.targetFlavorId || ''
        const flavorName = targetFlavorId || 'auto-assign'

        const allIPs =
          vm.spec.vms.networkInterfaces && vm.spec.vms.networkInterfaces.length > 0
            ? vm.spec.vms.networkInterfaces
                .flatMap((nic) => (Array.isArray(nic.ipAddress) ? nic.ipAddress : []))
                .filter((ip) => ip && ip.trim() !== '')
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
          networkInterfaces: vm.spec.vms.networkInterfaces as unknown as VmNetworkInterface[]
        }
      })

      // Sort VMs by ESX host order.
      if (orderedESXHosts.length > 0) {
        const esxHostOrder = new Map<string, number>()
        orderedESXHosts.forEach((host, index) => {
          esxHostOrder.set(host.id, index)
        })

        mappedVMs.sort((a, b) => {
          const aHostIndex = esxHostOrder.get(a.esxHost) ?? 999
          const bHostIndex = esxHostOrder.get(b.esxHost) ?? 999
          return aHostIndex - bHostIndex
        })
      }

      setVmsWithAssignments(mappedVMs)

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
  }, [clusterContext, orderedESXHosts, selectedVMs, setSelectedVMs, sourceCluster])

  useEffect(() => {
    if (!open) return

    if (sourceCluster) {
      fetchClusterHosts()
      fetchClusterVMs()
    } else {
      setOrderedESXHosts([])
      setVmsWithAssignments([])
    }
  }, [open, sourceCluster, fetchClusterHosts, fetchClusterVMs])

  return {
    loadingHosts,
    loadingVMs,
    orderedESXHosts,
    setOrderedESXHosts,
    vmsWithAssignments,
    setVmsWithAssignments,
    fetchClusterHosts,
    fetchClusterVMs
  }
}
