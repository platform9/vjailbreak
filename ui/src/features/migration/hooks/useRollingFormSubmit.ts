import { GridRowSelectionModel } from '@mui/x-data-grid'
import { NavigateFunction } from 'react-router-dom'
import { BMConfig } from 'src/api/bmconfig/model'
import { OpenstackCreds } from 'src/api/openstack-creds/model'
import { patchVMwareHost } from 'src/api/vmware-hosts/vmwareHosts'
import { VJAILBREAK_DEFAULT_NAMESPACE } from 'src/api/constants'
import { createNetworkMappingJson } from 'src/api/network-mapping/helpers'
import { postNetworkMapping } from 'src/api/network-mapping/networkMappings'
import { createStorageMappingJson } from 'src/api/storage-mappings/helpers'
import { postStorageMapping } from 'src/api/storage-mappings/storageMappings'
import { createArrayCredsMappingJson } from 'src/api/arraycreds-mapping/helpers'
import { postArrayCredsMapping } from 'src/api/arraycreds-mapping/arrayCredsMapping'
import { createMigrationTemplateJson } from 'src/features/migration/api/migration-templates/helpers'
import {
  patchMigrationTemplate,
  postMigrationTemplate
} from 'src/features/migration/api/migration-templates/migrationTemplates'
import {
  createRollingMigrationPlanJson,
  postRollingMigrationPlan,
  VMSequence,
  ClusterMapping
} from 'src/api/rolling-migration-plans'
import { CUTOVER_TYPES } from '../constants'
import { AMPLITUDE_EVENTS } from 'src/types/amplitude'
import type { AmplitudeEventName, EventProperties } from 'src/types/amplitude'
import type { VM, ESXHost, SelectedMigrationOptionsType, RollingFormParams } from '../types'
import { customMetadataToRecord } from '../utils/metadataUtils'
import type { SourceDataItem, PcdDataItem } from './useClusterData'
import type { ErrorContext } from 'src/services/errorReporting'

interface UseRollingFormSubmitParams {
  selectedVMs: GridRowSelectionModel
  vmsWithAssignments: VM[]
  selectedMaasConfig: BMConfig | null
  orderedESXHosts: ESXHost[]
  openstackCredData: OpenstackCreds | null
  sourceData: SourceDataItem[]
  pcdData: PcdDataItem[]
  availableVmwareNetworks: string[]
  availableVmwareDatastores: string[]
  params: RollingFormParams
  selectedMigrationOptions: SelectedMigrationOptionsType
  selectedVMwareCredName: string
  selectedPcdCredName: string
  submitting: boolean
  setSubmitting: (v: boolean) => void
  onClose: () => void
  navigate: NavigateFunction
  track: (eventName: AmplitudeEventName, properties?: EventProperties) => void
  reportError: (error: Error, additionalContext?: ErrorContext) => void
  setNetworkMappingError: (msg: string) => void
  setStorageMappingError: (msg: string) => void
}

export function useRollingFormSubmit({
  selectedVMs,
  vmsWithAssignments,
  selectedMaasConfig,
  orderedESXHosts,
  openstackCredData,
  sourceData,
  pcdData,
  availableVmwareNetworks,
  availableVmwareDatastores,
  params,
  selectedMigrationOptions,
  selectedVMwareCredName,
  selectedPcdCredName,
  submitting,
  setSubmitting,
  onClose,
  navigate,
  track,
  reportError,
  setNetworkMappingError,
  setStorageMappingError
}: UseRollingFormSubmitParams) {

  const handleSubmit = async () => {
    setSubmitting(true)

    const sourceCluster = params.vmwareCluster ?? ''
    const destinationPCD = params.pcdCluster ?? ''
    const networkMappings = params.networkMappings ?? []
    const storageMappings = params.storageMappings ?? []
    const arrayCredsMappings = params.arrayCredsMappings ?? []

    const storageCopyMethod = (params.storageCopyMethod || 'normal') as
      | 'normal'
      | 'StorageAcceleratedCopy'
      | 'HotAdd'

    if (selectedVMs.length > 0) {
      if (
        availableVmwareNetworks.some(
          (network) => !networkMappings.some((mapping) => mapping.source === network)
        )
      ) {
        setNetworkMappingError('All networks from selected VMs must be mapped')
        setSubmitting(false)
        return
      }

      if (storageCopyMethod === 'StorageAcceleratedCopy') {
        if (
          availableVmwareDatastores.some(
            (datastore) => !arrayCredsMappings.some((mapping) => mapping.source === datastore)
          )
        ) {
          setStorageMappingError('All datastores from selected VMs must be mapped')
          setSubmitting(false)
          return
        }
      } else {
        if (
          availableVmwareDatastores.some(
            (datastore) => !storageMappings.some((mapping) => mapping.source === datastore)
          )
        ) {
          setStorageMappingError('All datastores from selected VMs must be mapped')
          setSubmitting(false)
          return
        }
      }

      if (storageCopyMethod === 'HotAdd' && !params.proxyVMRef) {
        setStorageMappingError('Please select a vJailbreak Proxy VM to use for Accelerated Copy data copy')
        setSubmitting(false)
        return
      }
    } else if (sourceCluster && destinationPCD) {
      alert('Please select at least one VM to migrate')
      setSubmitting(false)
      return
    }

    try {
      const parts = sourceCluster.split(':')
      const credName = parts[0]

      const sourceItem = sourceData.find((item) => item.credName === credName)
      const clusterObj = sourceItem?.clusters.find((cluster) => cluster.id === sourceCluster)
      const clusterName = clusterObj?.name || ''

      const selectedVMsData = vmsWithAssignments
        .filter((vm) => selectedVMs.includes(vm.id))
        .map((vm) => ({
          vmName: vm.name
        })) as VMSequence[]

      const networkOverridesPerVM: Record<
        string,
        Array<{
          interfaceIndex: number
          preserveIP: boolean
          preserveMAC: boolean
          UserAssignedIP?: string
        }>
      > = {}
      vmsWithAssignments
        .filter((vm) => selectedVMs.includes(vm.id))
        .forEach((vm) => {
          const preserveIp = vm.preserveIp || {}
          const preserveMac = vm.preserveMac || {}
          const nicAssignedIps: Record<number, string> = {}

          ;(vm.networkInterfaces || []).forEach((nic, index) => {
            const assigned = (Array.isArray(nic.ipAddress) ? nic.ipAddress : [])
              .map((ip) => ip?.trim())
              .filter((ip): ip is string => Boolean(ip))
            if (assigned.length > 0) {
              nicAssignedIps[index] = assigned.join(',')
            }
          })
          const indices = new Set<string>([
            ...Object.keys(preserveIp),
            ...Object.keys(preserveMac),
            ...Object.keys(nicAssignedIps)
          ])

          if (indices.size === 0) return

          networkOverridesPerVM[vm.name] = Array.from(indices)
            .map((indexStr) => {
              const interfaceIndex = Number(indexStr)
              const ipFlag = preserveIp[interfaceIndex]
              const macFlag = preserveMac[interfaceIndex]
              const preserveIP = ipFlag !== false
              const preserveMAC = macFlag !== false
              const userAssigned = !preserveIP ? nicAssignedIps[interfaceIndex] : undefined
              return {
                interfaceIndex,
                preserveIP,
                preserveMAC,
                ...(userAssigned ? { UserAssignedIP: userAssigned } : {})
              }
            })
            .sort((a, b) => a.interfaceIndex - b.interfaceIndex)
        })

      // Create cluster mapping between VMware cluster and PCD cluster
      const selectedPCD = pcdData.find((p) => p.id === destinationPCD)
      const pcdClusterName = selectedPCD?.name || ''
      const targetPCDClusterName = selectedPCD?.name

      const clusterMapping: ClusterMapping[] = [
        {
          vmwareClusterName: clusterName,
          pcdClusterName: pcdClusterName
        }
      ]

      // Update VMware hosts with their host config IDs
      const hostsToUpdate = orderedESXHosts.filter((host) => host.pcdHostConfigName)

      for (const host of hostsToUpdate) {
        try {
          const availablePcdHostConfigs = openstackCredData?.spec?.pcdHostConfig || []
          const selectedPcdConfig = availablePcdHostConfigs.find(
            (config) => config.name === host.pcdHostConfigName
          )
          const hostConfigId = selectedPcdConfig ? selectedPcdConfig.id : host.pcdHostConfigName

          if (hostConfigId) {
            console.log(`Updating host ${host.name} with hostConfigId: ${hostConfigId}`)
            await patchVMwareHost(host.id, hostConfigId, VJAILBREAK_DEFAULT_NAMESPACE)
          }
        } catch (error) {
          console.error(`Failed to update host config for ${host.name}:`, error)
          reportError(error as Error, {
            context: 'host-config-update',
            metadata: {
              hostId: host.id,
              hostName: host.name,
              hostConfigId: host.pcdHostConfigName,
              action: 'host-config-update'
            }
          })
          // Continue with other hosts even if one fails
        }
      }

      // 1. Create network mapping
      const networkMappingJson = createNetworkMappingJson({
        networkMappings: networkMappings.map((mapping) => ({
          source: mapping.source,
          target: mapping.target
        }))
      })
      const networkMappingResponse = await postNetworkMapping(networkMappingJson)

      // 2. Create storage mapping
      let storageMappingResponse: any = null
      let arrayCredsMappingResponse: any = null

      if (storageCopyMethod === 'HotAdd') {
        const storageMappingJson = createStorageMappingJson({
          storageMappings: storageMappings.map((mapping) => ({
            source: mapping.source,
            target: mapping.target
          }))
        })
        storageMappingResponse = await postStorageMapping(storageMappingJson)
      } else if (storageCopyMethod === 'StorageAcceleratedCopy') {
        const arrayCredsMappingJson = createArrayCredsMappingJson({
          mappings: arrayCredsMappings.map((mapping) => ({
            source: mapping.source,
            target: mapping.target
          }))
        })
        arrayCredsMappingResponse = await postArrayCredsMapping(arrayCredsMappingJson)
      } else {
        const storageMappingJson = createStorageMappingJson({
          storageMappings: storageMappings.map((mapping) => ({
            source: mapping.source,
            target: mapping.target
          }))
        })
        storageMappingResponse = await postStorageMapping(storageMappingJson)
      }

      // 3. Create migration template
      const migrationTemplateJson = createMigrationTemplateJson({
        vmwareRef: selectedVMwareCredName,
        openstackRef: selectedPcdCredName,
        networkMapping: networkMappingResponse.metadata.name,
        ...(storageMappingResponse?.metadata?.name && {
          storageMapping: storageMappingResponse.metadata.name
        }),
        targetPCDClusterName: targetPCDClusterName
      })
      const migrationTemplateResponse = await postMigrationTemplate(migrationTemplateJson)

      // Update template to include storageCopyMethod and mapping selection
      if (migrationTemplateResponse?.metadata?.name) {
        await patchMigrationTemplate(migrationTemplateResponse.metadata.name, {
          spec: {
            networkMapping: networkMappingResponse.metadata.name,
            storageCopyMethod,
            ...(storageCopyMethod === 'HotAdd' && {
              ...(params.proxyVMRef && { proxyVMRef: { name: params.proxyVMRef } }),
              ...(storageMappingResponse?.metadata?.name && {
                storageMapping: storageMappingResponse.metadata.name
              })
            }),
            ...(storageCopyMethod === 'StorageAcceleratedCopy' &&
              arrayCredsMappingResponse?.metadata?.name && {
                arrayCredsMapping: arrayCredsMappingResponse.metadata.name
              }),
            ...(storageCopyMethod !== 'StorageAcceleratedCopy' &&
              storageCopyMethod !== 'HotAdd' &&
              storageMappingResponse?.metadata?.name && {
                storageMapping: storageMappingResponse.metadata.name
              })
          }
        })
      }

      // 4. Create rolling migration plan with the template
      const migrationPlanJson = createRollingMigrationPlanJson({
        clusterName,
        vms: selectedVMsData,
        clusterMapping,
        bmConfigRef: {
          name: selectedMaasConfig?.metadata.name || ''
        },
        ...(Object.keys(networkOverridesPerVM).length > 0 && { networkOverridesPerVM }),
        migrationStrategy: {
          adminInitiatedCutOver:
            selectedMigrationOptions.cutoverOption &&
            params.cutoverOption === CUTOVER_TYPES.ADMIN_INITIATED,
          healthCheckPort: '443',
          performHealthChecks: false,
          type: selectedMigrationOptions.dataCopyMethod
            ? (params.dataCopyMethod as string)
            : 'cold',
          ...(selectedMigrationOptions.dataCopyStartTime &&
            params.dataCopyStartTime && {
              dataCopyStart: params.dataCopyStartTime
            }),
          ...(selectedMigrationOptions.cutoverOption &&
            params.cutoverOption === CUTOVER_TYPES.TIME_WINDOW &&
            params.cutoverStartTime && {
              vmCutoverStart: params.cutoverStartTime
            }),
          ...(selectedMigrationOptions.cutoverOption &&
            params.cutoverOption === CUTOVER_TYPES.TIME_WINDOW &&
            params.cutoverEndTime && {
              vmCutoverEnd: params.cutoverEndTime
            }),
          ...(params.dataOnly ? { dataOnly: true } : {})
        },
        migrationTemplate: migrationTemplateResponse.metadata.name,
        namespace: VJAILBREAK_DEFAULT_NAMESPACE,
        preserveSourceTags: Boolean(params.preserveSourceTags),
        ...(customMetadataToRecord(params.customMetadata) && {
          customMetadata: customMetadataToRecord(params.customMetadata)
        })
      })

      await postRollingMigrationPlan(migrationPlanJson, VJAILBREAK_DEFAULT_NAMESPACE)

      const regionName = openstackCredData?.metadata?.labels?.['vjailbreak.k8s.pf9.io/region-name']

      // Track successful cluster conversion creation
      track(AMPLITUDE_EVENTS.ROLLING_MIGRATION_CREATED, {
        clusterMigrationName: clusterName,
        sourceCluster: clusterObj?.name,
        destinationCluster: selectedPCD?.name,
        vmwareCredential: selectedVMwareCredName,
        pcdCredential: selectedPcdCredName,
        maasConfig: selectedMaasConfig?.metadata.name,
        virtualMachineCount: selectedVMsData?.length || 0,
        esxHostCount: orderedESXHosts?.length || 0,
        networkMappingCount: networkMappings?.length || 0,
        storageMappingCount:
          storageCopyMethod === 'StorageAcceleratedCopy'
            ? arrayCredsMappings?.length || 0
            : storageMappings?.length || 0,
        migrationType: params.dataCopyMethod || 'cold',
        hasAdminInitiatedCutover:
          selectedMigrationOptions.cutoverOption &&
          params.cutoverOption === CUTOVER_TYPES.ADMIN_INITIATED,
        hasTimedCutover:
          selectedMigrationOptions.cutoverOption &&
          params.cutoverOption === CUTOVER_TYPES.TIME_WINDOW,
        migrationTemplate: migrationTemplateResponse.metadata.name,
        regionName,
        namespace: VJAILBREAK_DEFAULT_NAMESPACE
      })

      onClose()
      navigate('/dashboard/cluster-conversions')
    } catch (error) {
      console.error('Failed to submit rolling migration plan:', error)

      // Track cluster conversion failure
      const parts = sourceCluster.split(':')
      const credName = parts[0]
      const sourceItem = sourceData.find((item) => item.credName === credName)
      const clusterObj = sourceItem?.clusters.find((cluster) => cluster.id === sourceCluster)
      const selectedPCD = pcdData.find((p) => p.id === destinationPCD)
      const selectedVMsData = vmsWithAssignments.filter((vm) => selectedVMs.includes(vm.id))

      const regionName = openstackCredData?.metadata?.labels?.['vjailbreak.k8s.pf9.io/region-name']

      track(AMPLITUDE_EVENTS.ROLLING_MIGRATION_SUBMISSION_FAILED, {
        clusterMigrationName: clusterObj?.name,
        sourceCluster: clusterObj?.name,
        destinationCluster: selectedPCD?.name,
        vmwareCredential: selectedVMwareCredName,
        pcdCredential: selectedPcdCredName,
        virtualMachineCount: selectedVMsData?.length || 0,
        esxHostCount: orderedESXHosts?.length || 0,
        regionName,
        errorMessage: error instanceof Error ? error.message : String(error),
        stage: 'creation'
      })

      reportError(error as Error, {
        context: 'rolling-migration-plan-submission',
        metadata: {
          sourceCluster: sourceCluster,
          destinationPCD: destinationPCD,
          selectedVMwareCredName: selectedVMwareCredName,
          selectedPcdCredName: selectedPcdCredName,
          action: 'rolling-migration-plan-submission'
        }
      })
      const errorMessage = error instanceof Error ? error.message : 'Unknown error'
      alert(`Failed to submit rolling migration plan: ${errorMessage}`)
    } finally {
      setSubmitting(false)
    }
  }

  const handleClose = () => {
    if (!submitting) {
      onClose()
    }
  }

  return {
    handleSubmit,
    handleClose
  }
}
