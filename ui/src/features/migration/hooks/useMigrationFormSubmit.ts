import { useState, useCallback } from 'react'
import axios from 'axios'
import { QueryClient } from '@tanstack/react-query'
import { NavigateFunction } from 'react-router-dom'
import { postMigrationPlan } from 'src/features/migration/api/migration-plans/migrationPlans'
import { MigrationPlan } from 'src/features/migration/api/migration-plans/model'
import { createMigrationPlanJson } from 'src/features/migration/api/migration-plans/helpers'
import {
  patchMigrationTemplate,
  deleteMigrationTemplate
} from 'src/features/migration/api/migration-templates/migrationTemplates'
import { MigrationTemplate } from 'src/features/migration/api/migration-templates/model'
import { createNetworkMappingJson } from 'src/api/network-mapping/helpers'
import { postNetworkMapping } from 'src/api/network-mapping/networkMappings'
import { createStorageMappingJson } from 'src/api/storage-mappings/helpers'
import { postStorageMapping } from 'src/api/storage-mappings/storageMappings'
import { createArrayCredsMappingJson } from 'src/api/arraycreds-mapping/helpers'
import { postArrayCredsMapping } from 'src/api/arraycreds-mapping/arrayCredsMapping'
import { VMwareCreds } from 'src/api/vmware-creds/model'
import { deleteVmwareCredentials } from 'src/api/vmware-creds/vmwareCreds'
import { OpenstackCreds } from 'src/api/openstack-creds/model'
import { deleteOpenstackCredentials } from 'src/api/openstack-creds/openstackCreds'
import { MIGRATIONS_QUERY_KEY } from 'src/hooks/api/useMigrationsQuery'
import { VMWARE_MACHINES_BASE_KEY } from 'src/hooks/api/useVMwareMachinesQuery'
import { getRegionNameForOpenstackRef } from 'src/utils/regionNameResolver'
import { AMPLITUDE_EVENTS } from 'src/types/amplitude'
import { CUTOVER_TYPES } from '../constants'
import type { FormValues, SelectedMigrationOptionsType } from '../types'
import { customMetadataToRecord } from '../utils/metadataUtils'

interface UseMigrationFormSubmitParams {
  params: Partial<FormValues>
  selectedMigrationOptions: SelectedMigrationOptionsType
  migrationTemplate: MigrationTemplate | undefined
  vmwareCredentials: VMwareCreds | undefined
  openstackCredentials: OpenstackCreds | undefined
  setMigrationTemplate: React.Dispatch<React.SetStateAction<MigrationTemplate | undefined>>
  setVmwareCredentials: React.Dispatch<React.SetStateAction<VMwareCreds | undefined>>
  setOpenstackCredentials: React.Dispatch<React.SetStateAction<OpenstackCreds | undefined>>
  getFieldErrorsUpdater: (key: string) => (value: string) => void
  reportError: (
    error: Error,
    options?: { context?: string; metadata?: Record<string, unknown> }
  ) => void
  track: (event: string, properties?: Record<string, unknown>) => void
  queryClient: QueryClient
  navigate: NavigateFunction
  onClose: () => void
  onSuccess?: (message: string) => void
  sessionId: string
  networkMappingRequired: boolean
}

interface HandleCloseOptions {
  preserveCredentials?: boolean
}

interface UseMigrationFormSubmitResult {
  submitting: boolean
  handleSubmit: () => Promise<void>
  handleClose: (options?: HandleCloseOptions) => Promise<void>
}

export function useMigrationFormSubmit({
  params,
  selectedMigrationOptions,
  migrationTemplate,
  vmwareCredentials,
  openstackCredentials,
  setMigrationTemplate,
  setVmwareCredentials,
  setOpenstackCredentials,
  getFieldErrorsUpdater,
  reportError,
  track,
  queryClient,
  navigate,
  onClose,
  onSuccess,
  sessionId,
  networkMappingRequired
}: UseMigrationFormSubmitParams): UseMigrationFormSubmitResult {
  const [submitting, setSubmitting] = useState(false)
  const [, setError] = useState<{ title: string; message: string } | null>(null)

  const createNetworkMapping = async (networkMappingParams) => {
    const body = createNetworkMappingJson({
      networkMappings: networkMappingParams
    })

    try {
      const data = postNetworkMapping(body)
      return data
    } catch (err) {
      setError({
        title: 'Error creating network mapping',
        message: axios.isAxiosError(err) ? err?.response?.data?.message : ''
      })
      getFieldErrorsUpdater('networksMapping')(
        'Error creating network mapping : ' +
          (axios.isAxiosError(err) ? err?.response?.data?.message : err)
      )
    }
  }

  const createStorageMapping = async (storageMappingsParams) => {
    const body = createStorageMappingJson({
      storageMappings: storageMappingsParams
    })
    try {
      const data = postStorageMapping(body)
      return data
    } catch (err) {
      console.error('Error creating storage mapping', err)
      reportError(err as Error, {
        context: 'storage-mapping-creation',
        metadata: {
          storageMappingsParams: storageMappingsParams,
          action: 'create-storage-mapping'
        }
      })
      setError({
        title: 'Error creating storage mapping',
        message: axios.isAxiosError(err) ? err?.response?.data?.message : ''
      })
      getFieldErrorsUpdater('storageMapping')(
        'Error creating storage mapping : ' +
          (axios.isAxiosError(err) ? err?.response?.data?.message : err)
      )
    }
  }

  const createArrayCredsMapping = async (
    arrayCredsMappingsParams: { source: string; target: string }[]
  ) => {
    const body = createArrayCredsMappingJson({
      mappings: arrayCredsMappingsParams
    })
    try {
      const data = await postArrayCredsMapping(body)
      return data
    } catch (err) {
      console.error('Error creating ArrayCreds mapping', err)
      reportError(err as Error, {
        context: 'arraycreds-mapping-creation',
        metadata: {
          arrayCredsMappingsParams: arrayCredsMappingsParams,
          action: 'create-arraycreds-mapping'
        }
      })
      setError({
        title: 'Error creating ArrayCreds mapping',
        message: axios.isAxiosError(err) ? err?.response?.data?.message : ''
      })
      getFieldErrorsUpdater('storageMapping')(
        'Error creating ArrayCreds mapping : ' +
          (axios.isAxiosError(err) ? err?.response?.data?.message : err)
      )
    }
  }

  const updateMigrationTemplate = async (
    tmpl,
    networkMappings,
    storageMappings,
    arrayCredsMapping: any = null
  ) => {
    const migrationTemplateName = tmpl?.metadata?.name
    const storageCopyMethod = params.storageCopyMethod || 'normal'

    const updatedMigrationTemplateFields: any = {
      spec: {
        ...(networkMappings?.metadata?.name && { networkMapping: networkMappings.metadata.name }),
        storageCopyMethod
      }
    }

    if (storageCopyMethod === 'HotAdd') {
      if (params.proxyVMRef) {
        updatedMigrationTemplateFields.spec.proxyVMRef = { name: params.proxyVMRef }
      }
      if (storageMappings) {
        updatedMigrationTemplateFields.spec.storageMapping = storageMappings.metadata.name
      }
    } else if (storageCopyMethod === 'StorageAcceleratedCopy' && arrayCredsMapping) {
      updatedMigrationTemplateFields.spec.arrayCredsMapping = arrayCredsMapping.metadata.name
    } else if (storageMappings) {
      updatedMigrationTemplateFields.spec.storageMapping = storageMappings.metadata.name
    }

    try {
      const data = await patchMigrationTemplate(
        migrationTemplateName,
        updatedMigrationTemplateFields
      )
      return data
    } catch (err) {
      setError({
        title: 'Error updating migration template',
        message: axios.isAxiosError(err) ? err?.response?.data?.message : ''
      })
    }
  }

  const createMigrationPlan = async (
    updatedMigrationTemplate?: MigrationTemplate | null
  ): Promise<MigrationPlan> => {
    if (!updatedMigrationTemplate?.metadata?.name) {
      throw new Error('Migration template is not available')
    }

    const postMigrationAction = selectedMigrationOptions.postMigrationAction
      ? params.postMigrationAction
      : undefined

    const vmsToMigrate = (params.vms || []).map((vm) => vm.vmKey || vm.name)

    const networkOverridesPerVM: Record<
      string,
      Array<{
        interfaceIndex: number
        preserveIP: boolean
        preserveMAC: boolean
        UserAssignedIP?: string
      }>
    > = {}
    if (params.vms) {
      params.vms.forEach((vm) => {
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

        networkOverridesPerVM[vm.vmKey || vm.name] = Array.from(indices)
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
    }

    const migrationFields = {
      migrationTemplateName: updatedMigrationTemplate?.metadata?.name,
      virtualMachines: vmsToMigrate,
      type: params.dataCopyMethod,
      ...(Object.keys(networkOverridesPerVM).length > 0 && { networkOverridesPerVM }),
      ...(selectedMigrationOptions.dataCopyStartTime &&
        params?.dataCopyStartTime && {
          dataCopyStart: params.dataCopyStartTime
        }),
      ...(selectedMigrationOptions.cutoverOption &&
        params.cutoverOption === CUTOVER_TYPES.ADMIN_INITIATED && {
          adminInitiatedCutOver: true
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
      ...(postMigrationAction && { postMigrationAction }),
      ...(params.securityGroups &&
        params.securityGroups.length > 0 && {
          securityGroups: params.securityGroups
        }),
      ...(params.serverGroup && {
        serverGroup: params.serverGroup
      }),
      disconnectSourceNetwork: params.disconnectSourceNetwork || false,
      ...(params.dataOnly ? { dataOnly: true } : {}),
      fallbackToDHCP: params.fallbackToDHCP || false,
      ...(selectedMigrationOptions.postMigrationScript &&
        params.postMigrationScript && {
          postMigrationScript: params.postMigrationScript
        }),
      ...(typeof params.networkPersistence === 'boolean' && {
        networkPersistence: params.networkPersistence
      }),
      ...(typeof params.removeVMwareTools === 'boolean' && {
        removeVMwareTools: params.removeVMwareTools
      }),
      ...(Array.isArray(params.imageProfiles) &&
        params.imageProfiles.length > 0 && {
          imageProfiles: params.imageProfiles
        }),
      periodicSyncInterval: params.periodicSyncInterval,
      periodicSyncEnabled: selectedMigrationOptions.periodicSyncEnabled,
      acknowledgeNetworkConflictRisk: params.acknowledgeNetworkConflictRisk,
      preserveSourceTags: params.preserveSourceTags || false,
      ...(customMetadataToRecord(params.customMetadata) && {
        customMetadata: customMetadataToRecord(params.customMetadata)
      })
    }

    const body = createMigrationPlanJson(migrationFields)

    const regionName = await getRegionNameForOpenstackRef(
      openstackCredentials?.metadata?.name,
      openstackCredentials?.metadata?.namespace
    )

    try {
      const data = await postMigrationPlan(body)
      const virtualMachines = (data as any)?.spec?.virtualMachines

      const extractedVmNames: string[] = !Array.isArray(virtualMachines)
        ? []
        : virtualMachines.flatMap((entry: unknown) => {
            if (typeof entry === 'string') return [entry]

            if (Array.isArray(entry)) {
              return entry.filter(
                (name: unknown): name is string => typeof name === 'string' && name.length > 0
              )
            }

            return []
          })

      const vmNames: string[] =
        extractedVmNames.length > 0
          ? extractedVmNames
          : Array.isArray(vmsToMigrate)
            ? vmsToMigrate.filter(
                (vm: unknown): vm is string => typeof vm === 'string' && vm.length > 0
              )
            : []

      vmNames.forEach((vmName) => {
        track(AMPLITUDE_EVENTS.MIGRATION_CREATED, {
          migrationName: data.metadata?.name,
          migrationTemplateName: updatedMigrationTemplate?.metadata?.name,
          virtualMachineCount: vmNames.length,
          vmName,
          regionName,
          migrationType: migrationFields.type,
          hasDataCopyStartTime: !!migrationFields.dataCopyStart,
          hasAdminInitiatedCutover: !!migrationFields.adminInitiatedCutOver,
          hasTimedCutover: !!(migrationFields.vmCutoverStart && migrationFields.vmCutoverEnd),
          postMigrationAction,
          namespace: data.metadata?.namespace
        })
      })

      return data
    } catch (error: unknown) {
      console.error('Error creating migration plan', error)

      track(AMPLITUDE_EVENTS.MIGRATION_CREATION_FAILED, {
        migrationTemplateName: updatedMigrationTemplate?.metadata?.name,
        virtualMachineCount: vmsToMigrate?.length || 0,
        migrationType: migrationFields.type,
        regionName,
        errorMessage: error instanceof Error ? error.message : String(error),
        stage: 'creation'
      })

      reportError(error as Error, {
        context: 'migration-plan-creation',
        metadata: {
          migrationFields: migrationFields,
          action: 'create-migration-plan'
        }
      })

      let errorMessage = 'An unknown error occurred'
      let errorResponse: {
        status?: number
        statusText?: string
        data?: unknown
        config?: {
          url?: string
          method?: string
          data?: unknown
        }
      } = {}

      if (axios.isAxiosError(error)) {
        errorMessage = error.response?.data?.message || error.message || String(error)
        errorResponse = {
          status: error.response?.status,
          statusText: error.response?.statusText,
          data: error.response?.data,
          config: {
            url: error.config?.url,
            method: error.config?.method,
            data: error.config?.data
          }
        }
      } else if (error instanceof Error) {
        errorMessage = error.message
      } else {
        errorMessage = String(error)
      }

      console.error('Error details:', errorResponse)

      setError({
        title: 'Error creating migration plan',
        message: errorMessage
      })

      getFieldErrorsUpdater('migrationPlan')(`Error creating migration plan: ${errorMessage}`)
      throw error
    }
  }

  const handleSubmit = useCallback(async () => {
    setSubmitting(true)
    setError(null)

    const storageCopyMethod = params.storageCopyMethod || 'normal'

    let networkMappings: any = null
    if (networkMappingRequired) {
      networkMappings = await createNetworkMapping(params.networkMappings)
      if (!networkMappings) {
        setSubmitting(false)
        return
      }
    }

    let storageMappings: any = null
    let arrayCredsMapping: any = null

    if (storageCopyMethod === 'HotAdd') {
      storageMappings = await createStorageMapping(params.storageMappings)
      if (!storageMappings) {
        setSubmitting(false)
        return
      }
    } else if (storageCopyMethod === 'StorageAcceleratedCopy') {
      arrayCredsMapping = await createArrayCredsMapping(params.arrayCredsMappings || [])
      if (!arrayCredsMapping) {
        setSubmitting(false)
        return
      }
    } else {
      storageMappings = await createStorageMapping(params.storageMappings)
      if (!storageMappings) {
        setSubmitting(false)
        return
      }
    }

    const updatedMigrationTemplate = await updateMigrationTemplate(
      migrationTemplate,
      networkMappings,
      storageMappings,
      arrayCredsMapping
    )

    await createMigrationPlan(updatedMigrationTemplate)

    setSubmitting(false)
    queryClient.invalidateQueries({ queryKey: MIGRATIONS_QUERY_KEY })

    onSuccess?.('Migration submitted successfully')

    onClose()
    navigate('/dashboard/migrations')
  }, [
    params.networkMappings,
    params.storageMappings,
    params.arrayCredsMappings,
    params.storageCopyMethod,
    migrationTemplate,
    createNetworkMapping,
    createStorageMapping,
    createArrayCredsMapping,
    updateMigrationTemplate,
    createMigrationPlan,
    queryClient,
    onClose,
    onSuccess,
    navigate
  ])

  const handleClose = useCallback(async (options?: HandleCloseOptions) => {
    try {
      setMigrationTemplate(undefined)
      setVmwareCredentials(undefined)
      setOpenstackCredentials(undefined)
      setError(null)

      queryClient.invalidateQueries({ queryKey: [VMWARE_MACHINES_BASE_KEY, sessionId] })
      queryClient.removeQueries({ queryKey: [VMWARE_MACHINES_BASE_KEY, sessionId] })

      onClose()

      if (migrationTemplate?.metadata?.name) {
        await deleteMigrationTemplate(migrationTemplate.metadata.name)
      }

      if (options?.preserveCredentials) return

      if (vmwareCredentials?.metadata?.name && !params.vmwareCreds?.existingCredName) {
        await deleteVmwareCredentials(vmwareCredentials.metadata.name)
      }

      if (openstackCredentials?.metadata?.name && !params.openstackCreds?.existingCredName) {
        await deleteOpenstackCredentials(openstackCredentials.metadata.name)
      }
    } catch (err) {
      console.error('Error cleaning up resources', err)
      reportError(err as Error, {
        context: 'resource-cleanup',
        metadata: {
          migrationTemplateName: migrationTemplate?.metadata?.name,
          vmwareCredentialsName: vmwareCredentials?.metadata?.name,
          openstackCredentialsName: openstackCredentials?.metadata?.name,
          action: 'cleanup-resources'
        }
      })
      onClose()
    }
  }, [
    migrationTemplate,
    vmwareCredentials,
    openstackCredentials,
    setMigrationTemplate,
    setVmwareCredentials,
    setOpenstackCredentials,
    queryClient,
    sessionId,
    onClose,
    params.vmwareCreds,
    params.openstackCreds
  ])

  return { submitting, handleSubmit, handleClose }
}
