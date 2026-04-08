import { useQuery, UseQueryResult } from '@tanstack/react-query'

import type { MigrationPlan } from 'src/api/migration-plans/model'
import { getMigrationPlan } from 'src/api/migration-plans/migrationPlans'
import type { MigrationTemplate } from 'src/api/migration-templates/model'
import { getMigrationTemplate } from 'src/api/migration-templates/migrationTemplates'
import type { NetworkMapping } from 'src/api/network-mapping/model'
import { getNetworkMapping } from 'src/api/network-mapping/networkMappings'
import type { OpenstackCreds } from 'src/api/openstack-creds/model'
import { getOpenstackCredentialsList } from 'src/api/openstack-creds/openstackCreds'
import type { ArrayCredsMapping } from 'src/api/arraycreds-mapping/model'
import { getArrayCredsMapping } from 'src/api/arraycreds-mapping/arrayCredsMapping'
import type { VMwareCreds } from 'src/api/vmware-creds/model'
import { getVmwareCredentialsList } from 'src/api/vmware-creds/vmwareCreds'
import type { RdmDisk } from 'src/api/rdm-disks/model'
import { getRdmDisksList } from 'src/api/rdm-disks/rdmDisks'
import type { StorageMapping } from 'src/api/storage-mappings/model'
import { getStorageMapping } from 'src/api/storage-mappings/storageMappings'
import type { VMwareMachine } from 'src/api/vmware-machines/model'
import { getVMwareMachine, getVMwareMachines } from 'src/api/vmware-machines/vmwareMachines'
import type { PCDCluster, PCDClusterList } from 'src/api/pcd-clusters/model'
import { getPCDClusters } from 'src/api/pcd-clusters/pcdClusters'
import type { Migration } from 'src/features/migration/api/migrations'

export interface MigrationDetailResources {
  migrationPlan: MigrationPlan | null
  migrationTemplate: MigrationTemplate | null
  vmwareCredsRef: string | null
  openstackCredsRef: string | null
  vmwareCredsMissingRef: boolean
  openstackCredsMissingRef: boolean
  vmwareCredsCount: number
  openstackCredsCount: number
  vmwareCreds: VMwareCreds | null
  openstackCreds: OpenstackCreds | null
  openstackCredsList: OpenstackCreds[] | null
  pcdClusters: PCDCluster[] | null
  networkMapping: NetworkMapping | null
  storageMapping: StorageMapping | null
  arrayCredsMapping: ArrayCredsMapping | null
  vmwareMachine: VMwareMachine | null
  rdmDisks: RdmDisk[]
}

export const useMigrationDetailResourcesQuery = ({
  open,
  migration
}: {
  open: boolean
  migration: Migration | null
}): UseQueryResult<MigrationDetailResources, Error> => {
  return useQuery<MigrationDetailResources>({
    queryKey: ['migration-detail', migration?.metadata?.namespace, migration?.metadata?.name],
    enabled: open && Boolean(migration?.metadata?.name),
    refetchOnWindowFocus: false,
    staleTime: 0,
    queryFn: async (): Promise<MigrationDetailResources> => {
      if (!migration) {
        return {
          migrationPlan: null,
          migrationTemplate: null,
          vmwareCredsRef: null,
          openstackCredsRef: null,
          vmwareCredsMissingRef: false,
          openstackCredsMissingRef: false,
          vmwareCredsCount: -1,
          openstackCredsCount: -1,
          vmwareCreds: null,
          openstackCreds: null,
          openstackCredsList: null,
          pcdClusters: null,
          networkMapping: null,
          storageMapping: null,
          arrayCredsMapping: null,
          vmwareMachine: null,
          rdmDisks: []
        }
      }

      const namespace = migration?.metadata?.namespace
      const migrationSpec = migration?.spec as any
      const vmName = (migrationSpec?.vmName as string) || ''
      const vmStableId =
        (migrationSpec?.vmId as string) ||
        (migrationSpec?.vmID as string) ||
        (migrationSpec?.vmwareMachine as string) ||
        (migrationSpec?.vmwareMachineName as string) ||
        (migrationSpec?.vmwareMachineRef as string) ||
        ''
      const migrationPlanName =
        (migrationSpec?.migrationPlan as string) ||
        (migration?.metadata as any)?.labels?.migrationplan

      const safeGet = async <T>(fn: () => Promise<T>): Promise<T | null> => {
        try {
          return await fn()
        } catch (error) {
          console.error('Error in safeGet:', error)
          return null
        }
      }

      const migrationPlan = migrationPlanName
        ? await safeGet(() => getMigrationPlan(migrationPlanName, namespace))
        : null

      const migrationTemplateName = (migrationPlan?.spec as any)?.migrationTemplate as
        | string
        | undefined
      const migrationTemplate = migrationTemplateName
        ? await safeGet(() => getMigrationTemplate(migrationTemplateName, namespace))
        : null

      const templateSpec = (migrationTemplate?.spec as any) || {}
      const openstackRef = templateSpec?.destination?.openstackRef as string | undefined
      const networkMappingName = templateSpec?.networkMapping as string | undefined
      const storageMappingName = templateSpec?.storageMapping as string | undefined
      const arrayCredsMappingName = templateSpec?.arrayCredsMapping as string | undefined
      const vmwareRef = templateSpec?.source?.vmwareRef as string | undefined

      const [
        vmwareCredsList,
        openstackCredsList,
        pcdClustersList,
        networkMapping,
        storageMapping,
        arrayCredsMapping
      ] = await Promise.all([
        safeGet(() => getVmwareCredentialsList(namespace)),
        safeGet(() => getOpenstackCredentialsList(namespace)),
        safeGet(() => getPCDClusters(namespace)),
        networkMappingName
          ? safeGet(() => getNetworkMapping(networkMappingName, namespace))
          : Promise.resolve(null),
        storageMappingName
          ? safeGet(() => getStorageMapping(storageMappingName, namespace))
          : Promise.resolve(null),
        arrayCredsMappingName
          ? safeGet(() => getArrayCredsMapping(arrayCredsMappingName, namespace))
          : Promise.resolve(null)
      ])

      const vmwareCredsByName = new Map(
        (Array.isArray(vmwareCredsList) ? vmwareCredsList : []).map((c: any) => [
          String(c?.metadata?.name || ''),
          c
        ])
      )
      const openstackCredsByName = new Map(
        (Array.isArray(openstackCredsList) ? openstackCredsList : []).map((c: any) => [
          String(c?.metadata?.name || ''),
          c
        ])
      )

      const vmwareCreds = vmwareRef
        ? (vmwareCredsByName.get(vmwareRef) as VMwareCreds | undefined) || null
        : null
      const openstackCreds = openstackRef
        ? (openstackCredsByName.get(openstackRef) as OpenstackCreds | undefined) || null
        : null

      const vmwareCredsMissingRef = Boolean(vmwareRef) && !vmwareCreds
      const openstackCredsMissingRef = Boolean(openstackRef) && !openstackCreds

      const vmwareCredsCount = Array.isArray(vmwareCredsList)
        ? vmwareCredsList.length
        : vmwareCredsList === null
          ? -1
          : 0
      const openstackCredsCount = Array.isArray(openstackCredsList)
        ? openstackCredsList.length
        : openstackCredsList === null
          ? -1
          : 0

      let vmwareMachine: VMwareMachine | null = null
      if (vmName) {
        vmwareMachine = await safeGet(() => getVMwareMachine(vmName, namespace))
      }

      const vmwareMachinesList =
        !vmwareMachine ? await safeGet(() => getVMwareMachines(namespace, vmwareRef)) : null
      const vmwareMachines = vmwareMachinesList?.items || []

      if (!vmwareMachine && vmwareMachines.length) {
        vmwareMachine =
          vmwareMachines.find((m) => vmStableId && m?.metadata?.name === vmStableId) ||
          vmwareMachines.find((m) => vmName && m?.metadata?.name === vmName) ||
          vmwareMachines.find((m) => {
            const vms = (m?.spec as any)?.vms
            const reconstructed =
              vms?.name && vms?.vmid
                ? `${vms.name}-${String(vms.vmid).replace(/^vm-/, '')}`
                : vms?.name
            return vmName && reconstructed === vmName
          }) ||
          vmwareMachines.find((m) => vmName && (m?.spec as any)?.vms?.name === vmName) ||
          null
      }

      const rdmDiskNames = ((vmwareMachine?.spec as any)?.vms?.rdmDisks as string[]) || []
      const effectiveVmName = vmName || ((vmwareMachine?.spec as any)?.vms?.name as string) || ''
      const allRdmDisks = effectiveVmName ? await safeGet(() => getRdmDisksList(namespace)) : null
      const rdmDisks = (allRdmDisks || []).filter((d: any) => {
        const ownerVMs = (d?.spec?.ownerVMs as string[]) || []
        const diskName = (d?.spec?.diskName as string) || ''
        const metaName = (d?.metadata?.name as string) || ''
        return (
          (effectiveVmName && ownerVMs.includes(effectiveVmName)) ||
          (rdmDiskNames.length &&
            (rdmDiskNames.includes(metaName) || rdmDiskNames.includes(diskName)))
        )
      })

      return {
        migrationPlan,
        migrationTemplate,
        vmwareCredsRef: vmwareRef || null,
        openstackCredsRef: openstackRef || null,
        vmwareCredsMissingRef,
        openstackCredsMissingRef,
        vmwareCredsCount,
        openstackCredsCount,
        vmwareCreds,
        openstackCreds,
        openstackCredsList: Array.isArray(openstackCredsList) ? openstackCredsList : null,
        pcdClusters: (pcdClustersList as PCDClusterList | null)?.items || null,
        networkMapping,
        storageMapping,
        arrayCredsMapping,
        vmwareMachine,
        rdmDisks
      }
    }
  })
}
