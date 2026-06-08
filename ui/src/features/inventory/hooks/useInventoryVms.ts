import { useMemo } from 'react'
import { useVmwareCredentialsQuery } from 'src/hooks/api/useVmwareCredentialsQuery'
import { useOpenstackCredentialsQuery } from 'src/hooks/api/useOpenstackCredentialsQuery'
import { useVMwareMachinesQuery } from 'src/hooks/api/useVMwareMachinesQuery'
import { VmData } from 'src/features/migration/api/migration-templates/model'
import { useMigrationBucketsQuery } from './useMigrationBucketsQuery'
import { BucketIdByVm, InventoryData, InventoryVm, VmPowerState } from '../types'

const VMWARE_CLUSTER_LABEL = 'vjailbreak.k8s.pf9.io/vmware-cluster'

const normalizePowerState = (vmState?: string): VmPowerState => {
  const s = (vmState ?? '').toLowerCase()
  if (s.includes('off') || s.includes('stop')) return 'powered-off'
  if (s.includes('on') || s.includes('run')) return 'powered-on'
  return 'unknown'
}

const toInventoryVm = (vm: VmData): InventoryVm => ({
  id: vm.id,
  name: vm.name,
  vmwareMachineName: vm.vmWareMachineName,
  powerState: normalizePowerState(vm.vmState),
  nicCount: vm.networkInterfaces?.length ?? 0,
  clusterName: vm.labels?.[VMWARE_CLUSTER_LABEL],
  diskCount: vm.disks?.length ?? 0,
  networks: vm.networks ?? [],
  datastores: vm.datastores ?? [],
  osFamily: vm.osFamily
})

/**
 * Planner-facing inventory: discovered VMs (single VMware credential, v1) mapped to
 * InventoryVm, plus a VM→bucket index built from the current MigrationBuckets.
 *
 * Credential gating mirrors useVMwareMachinesQuery (both creds validated) so we reuse it
 * unchanged; relaxing this for inventory-only views is tracked as a clarification.
 */
export const useInventoryVms = (): InventoryData => {
  const vmwareCredsQuery = useVmwareCredentialsQuery(undefined, { staleTime: 0 })
  const openstackCredsQuery = useOpenstackCredentialsQuery(undefined, { staleTime: 0 })
  const bucketsQuery = useMigrationBucketsQuery()

  // v1: single VMware credential — use the first validated one.
  const vmwareCred = useMemo(() => {
    const creds = Array.isArray(vmwareCredsQuery.data) ? vmwareCredsQuery.data : []
    return creds.find((c) => c?.status?.vmwareValidationStatus === 'Succeeded') ?? creds[0]
  }, [vmwareCredsQuery.data])

  const credName = vmwareCred?.metadata?.name
  const vmwareDatacenter = vmwareCred?.spec?.datacenter
  const vmwareCredsValidated = vmwareCred?.status?.vmwareValidationStatus === 'Succeeded'
  const openstackCredsValidated = useMemo(() => {
    const creds = Array.isArray(openstackCredsQuery.data) ? openstackCredsQuery.data : []
    return creds.some((c) => c?.status?.openstackValidationStatus === 'Succeeded')
  }, [openstackCredsQuery.data])

  const vmsQuery = useVMwareMachinesQuery({
    vmwareCredsValidated,
    openstackCredsValidated,
    vmwareCredName: credName,
    enabled: Boolean(credName)
  })

  const vms = useMemo<InventoryVm[]>(
    () => (Array.isArray(vmsQuery.data) ? vmsQuery.data.map(toInventoryVm) : []),
    [vmsQuery.data]
  )

  const byName = useMemo<Record<string, InventoryVm>>(
    () => Object.fromEntries(vms.map((vm) => [vm.name, vm])),
    [vms]
  )

  const buckets = useMemo(
    () => (Array.isArray(bucketsQuery.data) ? bucketsQuery.data : []),
    [bucketsQuery.data]
  )

  // A bucket member may be stored as a display name OR a VM id/key depending on what created the
  // bucket (the UI writes display names; some external tools write the VM id, e.g.
  // "xfs-root-automation-7764"). Index every inventory VM by all the identifiers a bucket might
  // hold so membership resolves consistently regardless of style.
  const vmByAnyKey = useMemo<Record<string, InventoryVm>>(() => {
    const map: Record<string, InventoryVm> = {}
    // Display name is authoritative — register every name first so a synthetic id (vm.id is
    // `${name}-${vmid}`) or machine name can never shadow a real VM's display name.
    for (const vm of vms) map[vm.name] = vm
    for (const vm of vms) {
      if (!(vm.id in map)) map[vm.id] = vm
      if (vm.vmwareMachineName && !(vm.vmwareMachineName in map)) map[vm.vmwareMachineName] = vm
    }
    return map
  }, [vms])

  const bucketIdByVm = useMemo<BucketIdByVm>(() => {
    const index: BucketIdByVm = {}
    for (const bucket of buckets) {
      for (const member of bucket.spec.vms) {
        // Canonicalize to the display name when resolvable (dropdowns/greying key by name);
        // otherwise keep the raw stored value so unresolved members still block.
        const key = vmByAnyKey[member]?.name ?? member
        index[key] = bucket.metadata.name
      }
    }
    return index
  }, [buckets, vmByAnyKey])

  return {
    vms,
    byName,
    bucketIdByVm,
    buckets,
    credName,
    vmwareDatacenter,
    isLoading: vmwareCredsQuery.isLoading || vmsQuery.isLoading || bucketsQuery.isLoading,
    isError: vmwareCredsQuery.isError || vmsQuery.isError || bucketsQuery.isError
  }
}
