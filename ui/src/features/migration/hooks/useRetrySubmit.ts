import { useCallback, useState } from 'react'
import { QueryClient } from '@tanstack/react-query'
import { NavigateFunction } from 'react-router-dom'
import { deleteMigration } from 'src/api/migrations/migrations'
import { createNetworkMappingJson } from 'src/api/network-mapping/helpers'
import { postNetworkMapping } from 'src/api/network-mapping/networkMappings'
import { createStorageMappingJson } from 'src/api/storage-mappings/helpers'
import { postStorageMapping } from 'src/api/storage-mappings/storageMappings'
import { patchVMwareMachine } from 'src/api/vmware-machines/vmwareMachines'
import {
  patchMigrationPlan,
  requestMigrationPlanRetry
} from 'src/features/migration/api/migration-plans/migrationPlans'
import { MigrationPlan } from 'src/features/migration/api/migration-plans/model'
import { patchMigrationTemplate } from 'src/features/migration/api/migration-templates/migrationTemplates'
import { MigrationTemplate, VmData } from 'src/features/migration/api/migration-templates/model'
import { MIGRATIONS_QUERY_KEY } from 'src/hooks/api/useMigrationsQuery'
import { CUTOVER_TYPES } from '../constants'
import type { RetryMigrationConfig } from '../context/MigrationFormContext'
import type { FormValues, SelectedMigrationOptionsType } from '../types'

interface UseRetrySubmitParams {
  retryConfig?: RetryMigrationConfig
  params: Partial<FormValues>
  selectedMigrationOptions: SelectedMigrationOptionsType
  retryPlan: MigrationPlan | undefined
  retryTemplate: MigrationTemplate | undefined
  retryVm: VmData | undefined
  vmK8sName: string | undefined
  selectedFlavorId: string
  selectedPcdClusterName: string
  networkMappingRequired: boolean
  queryClient: QueryClient
  navigate: NavigateFunction
  onClose: () => void
  onSuccess?: (message: string) => void
  reportError: (
    error: Error,
    options?: { context?: string; metadata?: Record<string, unknown> }
  ) => void
}

interface UseRetrySubmitResult {
  retrySubmitting: boolean
  retryError: string | null
  handleRetryWithoutEdit: () => Promise<void>
  handleEditAndRetry: () => Promise<void>
}

// Implements the two retry actions. "Retry without editing" only annotates the plan and
// deletes the failed Migration (no configuration writes). "Edit & Retry" persists the
// form's edits to the plan's own resources first, then triggers the same retry. The
// annotation must land before the Migration delete so the controller resets the plan
// even after validation failures.
export function useRetrySubmit({
  retryConfig,
  params,
  selectedMigrationOptions,
  retryPlan,
  retryTemplate,
  retryVm,
  vmK8sName,
  selectedFlavorId,
  selectedPcdClusterName,
  networkMappingRequired,
  queryClient,
  navigate,
  onClose,
  onSuccess,
  reportError
}: UseRetrySubmitParams): UseRetrySubmitResult {
  const [retrySubmitting, setRetrySubmitting] = useState(false)
  const [retryError, setRetryError] = useState<string | null>(null)

  const finishRetry = useCallback(
    (message: string) => {
      queryClient.invalidateQueries({ queryKey: MIGRATIONS_QUERY_KEY })
      onSuccess?.(message)
      onClose()
      navigate('/dashboard/migrations')
    },
    [queryClient, onSuccess, onClose, navigate]
  )

  const triggerRetry = useCallback(async () => {
    if (!retryConfig) throw new Error('Retry context is missing')
    const planName = retryPlan?.metadata?.name || retryConfig.planName
    await requestMigrationPlanRetry(planName, retryConfig.namespace)
    await deleteMigration(retryConfig.migrationName, retryConfig.namespace)
  }, [retryConfig, retryPlan])

  const handleRetryWithoutEdit = useCallback(async () => {
    setRetrySubmitting(true)
    setRetryError(null)
    try {
      await triggerRetry()
      finishRetry(`Retry started for ${retryConfig?.vmName}`)
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err)
      setRetryError(`Failed to start retry: ${message}`)
      reportError(err as Error, { context: 'retry-without-edit' })
    } finally {
      setRetrySubmitting(false)
    }
  }, [triggerRetry, finishRetry, retryConfig, reportError])

  const buildPlanPatchSpec = useCallback(() => {
    const timeWindow =
      selectedMigrationOptions.cutoverOption && params.cutoverOption === CUTOVER_TYPES.TIME_WINDOW

    // Build networkOverridesPerVM from params.vms (same logic as useMigrationFormSubmit).
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

    // Cleared optional fields are sent as explicit nulls: merge-patch removes the key so
    // the recreated Migration does not silently keep stale values.
    return {
      migrationStrategy: {
        type: params.dataCopyMethod || retryPlan?.spec?.migrationStrategy?.type || 'cold',
        adminInitiatedCutOver: Boolean(
          selectedMigrationOptions.cutoverOption &&
            params.cutoverOption === CUTOVER_TYPES.ADMIN_INITIATED
        ),
        dataCopyStart:
          (selectedMigrationOptions.dataCopyStartTime && params.dataCopyStartTime) || null,
        vmCutoverStart: (timeWindow && params.cutoverStartTime) || null,
        vmCutoverEnd: (timeWindow && params.cutoverEndTime) || null,
        disconnectSourceNetwork: params.disconnectSourceNetwork || false
      },
      securityGroups: params.securityGroups?.length ? params.securityGroups : null,
      serverGroup: params.serverGroup || null,
      fallbackToDHCP: params.fallbackToDHCP || false,
      firstBootScript:
        (selectedMigrationOptions.postMigrationScript && params.postMigrationScript) || null,
      postMigrationAction: selectedMigrationOptions.postMigrationAction
        ? params.postMigrationAction
        : null,
      // Send overrides when present; null explicitly clears any existing overrides
      // that the user removed (merge-patch omit would silently keep stale values).
      networkOverridesPerVM: Object.keys(networkOverridesPerVM).length > 0
        ? networkOverridesPerVM
        : null,
      advancedOptions: {
        periodicSyncEnabled: Boolean(selectedMigrationOptions.periodicSyncEnabled),
        periodicSyncInterval: params.periodicSyncInterval || null,
        networkPersistence: Boolean(params.networkPersistence),
        removeVMwareTools: Boolean(params.removeVMwareTools),
        acknowledgeNetworkConflictRisk: Boolean(params.acknowledgeNetworkConflictRisk),
        imageProfiles: params.imageProfiles?.length ? params.imageProfiles : null
      }
    }
  }, [params, selectedMigrationOptions, retryPlan])

  const handleEditAndRetry = useCallback(async () => {
    if (!retryConfig || !retryTemplate) return
    setRetrySubmitting(true)
    setRetryError(null)
    try {
      const namespace = retryConfig.namespace
      const planName = retryPlan?.metadata?.name || retryConfig.planName
      const templateName = retryTemplate.metadata?.name

      // 1. Fresh mapping resources (same pattern as the standard form: mappings are
      //    immutable per submission, the template points at the latest ones).
      const templateSpec: Record<string, unknown> = {
        storageCopyMethod: params.storageCopyMethod || 'normal',
        useGPUFlavor: params.useGPU || false,
        useFlavorless: Boolean(selectedMigrationOptions.useFlavorless),
        targetPCDClusterName:
          selectedPcdClusterName || retryTemplate?.spec?.targetPCDClusterName || ''
      }

      if (networkMappingRequired && params.networkMappings?.length) {
        const created = await postNetworkMapping(
          createNetworkMappingJson({ networkMappings: params.networkMappings }),
          namespace
        )
        templateSpec.networkMapping = created.metadata.name
      }

      if (params.storageCopyMethod !== 'StorageAcceleratedCopy' && params.storageMappings?.length) {
        const created = await postStorageMapping(
          createStorageMappingJson({ storageMappings: params.storageMappings }),
          namespace
        )
        templateSpec.storageMapping = created.metadata.name
      }

      if (params.storageCopyMethod === 'HotAdd' && params.proxyVMRef) {
        templateSpec.proxyVMRef = { name: params.proxyVMRef }
      }

      // 2. Template, 3. plan: all configuration writes land before the retry trigger.
      await patchMigrationTemplate(templateName, { spec: templateSpec })
      await patchMigrationPlan(planName, { spec: buildPlanPatchSpec() }, namespace)

      // 4. Per-VM flavor override.
      if (vmK8sName && selectedFlavorId && selectedFlavorId !== (retryVm?.targetFlavorId || '')) {
        await patchVMwareMachine(vmK8sName, { spec: { targetFlavorId: selectedFlavorId } }, namespace)
      }

      // 5. Trigger the retry only after every edit is persisted.
      await triggerRetry()
      finishRetry(`Configuration updated, retry started for ${retryConfig.vmName}`)
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err)
      setRetryError(`Failed to apply edits and retry: ${message}`)
      reportError(err as Error, {
        context: 'edit-and-retry',
        metadata: { plan: retryPlan?.metadata?.name, template: retryTemplate?.metadata?.name }
      })
    } finally {
      setRetrySubmitting(false)
    }
  }, [
    retryConfig,
    retryTemplate,
    retryPlan,
    retryVm,
    vmK8sName,
    selectedFlavorId,
    selectedPcdClusterName,
    params,
    selectedMigrationOptions,
    networkMappingRequired,
    buildPlanPatchSpec,
    triggerRetry,
    finishRetry,
    reportError
  ])

  return { retrySubmitting, retryError, handleRetryWithoutEdit, handleEditAndRetry }
}
