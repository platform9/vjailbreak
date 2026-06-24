import { useCallback } from 'react'
import { useMutation, QueryClient } from '@tanstack/react-query'
import { NavigateFunction } from 'react-router-dom'
import { deleteMigration, getMigration } from 'src/api/migrations/migrations'
import { createNetworkMappingJson } from 'src/api/network-mapping/helpers'
import { postNetworkMapping } from 'src/api/network-mapping/networkMappings'
import { createStorageMappingJson } from 'src/api/storage-mappings/helpers'
import { postStorageMapping } from 'src/api/storage-mappings/storageMappings'
import { patchVMwareMachine } from 'src/api/vmware-machines/vmwareMachines'
import {
  deleteMigrationPlan,
  patchMigrationPlan,
  postMigrationPlan
} from 'src/features/migration/api/migration-plans/migrationPlans'
import { MigrationPlan } from 'src/features/migration/api/migration-plans/model'
import { postMigrationTemplate } from 'src/features/migration/api/migration-templates/migrationTemplates'
import { MigrationTemplate, VmData } from 'src/features/migration/api/migration-templates/model'
import { MIGRATIONS_QUERY_KEY } from 'src/hooks/api/useMigrationsQuery'
import { CUTOVER_TYPES } from '../constants'
import type { RetryMigrationConfig } from '../context/MigrationFormContext'
import type { FormValues, SelectedMigrationOptionsType } from '../types'

async function pollUntilGone(
  fetcher: () => Promise<unknown>,
  pollIntervalMs = 500,
  timeoutMs = 30000
): Promise<void> {
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    try {
      await fetcher()
      await new Promise<void>((resolve) => setTimeout(resolve, pollIntervalMs))
    } catch {
      return
    }
  }
}

// Trim base name to fit within the Kubernetes 63-char DNS label limit, then strip
// trailing hyphens left by the truncation.
export function makeCloneName(original: string, suffix: string): string {
  const maxBase = 63 - suffix.length
  const base = original.slice(0, maxBase).replace(/-+$/, '')
  return base + suffix
}

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

// Implements the two retry actions.
// "Retry without editing": just deletes the failed Migration — controller recreates it unchanged.
// "Edit & Retry": removes the VM from the old plan (or deletes the plan if it's the last VM),
// deletes the old Migration, then creates a fresh set of resources (NetworkMapping,
// StorageMapping, MigrationTemplate, MigrationPlan) with the user's edits applied.
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
    await deleteMigration(retryConfig.migrationName, retryConfig.namespace)
  }, [retryConfig])

  const buildPlanPatchSpec = useCallback(() => {
    const timeWindow =
      selectedMigrationOptions.cutoverOption && params.cutoverOption === CUTOVER_TYPES.TIME_WINDOW

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
      networkOverridesPerVM:
        Object.keys(networkOverridesPerVM).length > 0 ? networkOverridesPerVM : null,
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

  const performEditAndRetry = useCallback(async () => {
    if (!retryConfig || !retryTemplate || !retryPlan) return
    const namespace = retryConfig.namespace
    const retryingVMKey = retryVm?.vmKey || retryVm?.name || retryConfig.vmName

    // 1. Remove the retrying VM from the old plan. If it's the last VM, delete the whole plan.
    const isLastVMInPlan = (retryPlan.spec?.virtualMachines?.flat()?.length ?? 0) <= 1
    if (isLastVMInPlan) {
      await deleteMigrationPlan(retryPlan.metadata.name, namespace)
    } else {
      const updatedVMs = (retryPlan.spec?.virtualMachines || [])
        .map((batch) => batch.filter((v) => v !== retryingVMKey))
        .filter((batch) => batch.length > 0)
      await patchMigrationPlan(
        retryPlan.metadata.name,
        { spec: { virtualMachines: updatedVMs } },
        namespace
      )
    }

    // 2. Delete the failed Migration, then wait for it to be fully gone before creating
    //    new resources. Plan entry removal is already confirmed by step 1's PATCH.
    //    GC cascades to owned ConfigMaps and Job once the Migration object disappears.
    await deleteMigration(retryConfig.migrationName, namespace)
    await pollUntilGone(() => getMigration(retryConfig.migrationName, namespace))

    // 3. Create new NetworkMapping.
    let newNetworkMappingName: string | undefined
    if (networkMappingRequired && params.networkMappings?.length) {
      const created = await postNetworkMapping(
        createNetworkMappingJson({ networkMappings: params.networkMappings }),
        namespace
      )
      newNetworkMappingName = created.metadata.name
    }

    // 4. Create new StorageMapping.
    let newStorageMappingName: string | undefined
    if (params.storageCopyMethod !== 'StorageAcceleratedCopy' && params.storageMappings?.length) {
      const created = await postStorageMapping(
        createStorageMappingJson({ storageMappings: params.storageMappings }),
        namespace
      )
      newStorageMappingName = created.metadata.name
    }

    // 5. POST new MigrationTemplate: inherit immutable source/destination fields from
    //    the original, override everything the user may have edited.
    const originalTemplateSpec = retryTemplate.spec || {}
    const newTemplateSpec = {
      ...originalTemplateSpec,
      ...(newNetworkMappingName !== undefined && { networkMapping: newNetworkMappingName }),
      ...(newStorageMappingName !== undefined && { storageMapping: newStorageMappingName }),
      storageCopyMethod: params.storageCopyMethod || 'normal',
      useGPUFlavor: params.useGPU || false,
      useFlavorless: Boolean(selectedMigrationOptions.useFlavorless),
      targetPCDClusterName: selectedPcdClusterName || originalTemplateSpec.targetPCDClusterName || '',
      ...(params.storageCopyMethod === 'HotAdd' && params.proxyVMRef
        ? { proxyVMRef: { name: params.proxyVMRef } }
        : {})
    }
    const newTemplate = await postMigrationTemplate(
      {
        apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1',
        kind: 'MigrationTemplate',
        metadata: { name: makeCloneName(retryTemplate.metadata.name.replace(/-r$/, ''), '-r') },
        spec: newTemplateSpec
      },
      namespace
    )

    // 6. POST new MigrationPlan: single VM, fresh configuration with the user's edits.
    //    networkOverridesPerVM is filtered to only the retrying VM so other VMs' overrides
    //    from the form state do not leak into the new plan.
    const planPatch = buildPlanPatchSpec()
    const filteredOverrides = planPatch.networkOverridesPerVM
      ? Object.fromEntries(
          Object.entries(planPatch.networkOverridesPerVM).filter(([key]) => key === retryingVMKey)
        )
      : null
    const newPlanOverrides =
      filteredOverrides && Object.keys(filteredOverrides).length > 0 ? filteredOverrides : null

    await postMigrationPlan(
      {
        apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1',
        kind: 'MigrationPlan',
        metadata: {
          name: isLastVMInPlan
            ? retryPlan.metadata.name
            : makeCloneName(retryPlan.metadata.name.replace(/-r$/, ''), '-r')
        },
        spec: {
          ...planPatch,
          migrationTemplate: newTemplate.metadata.name,
          virtualMachines: [[retryingVMKey]],
          retry: false,
          networkOverridesPerVM: newPlanOverrides
        }
      },
      namespace
    )

    // 7. Per-VM flavor override.
    if (vmK8sName && selectedFlavorId && selectedFlavorId !== (retryVm?.targetFlavorId || '')) {
      await patchVMwareMachine(
        vmK8sName,
        { spec: { targetFlavorId: selectedFlavorId } },
        namespace
      )
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
    buildPlanPatchSpec
  ])

  const retryWithoutEditMutation = useMutation({
    mutationFn: triggerRetry,
    onSuccess: () => finishRetry(`Retry started for ${retryConfig?.vmName}`),
    onError: (err: Error) => reportError(err, { context: 'retry-without-edit' })
  })

  const editAndRetryMutation = useMutation({
    mutationFn: async () => {
      if (!retryConfig || !retryTemplate) return
      await performEditAndRetry()
    },
    onSuccess: () =>
      finishRetry(`Configuration updated, retry started for ${retryConfig?.vmName}`),
    onError: (err: Error) =>
      reportError(err, {
        context: 'edit-and-retry',
        metadata: { plan: retryPlan?.metadata?.name, template: retryTemplate?.metadata?.name }
      })
  })

  const retrySubmitting =
    retryWithoutEditMutation.isPending || editAndRetryMutation.isPending

  const retryError =
    (retryWithoutEditMutation.error
      ? `Failed to start retry: ${retryWithoutEditMutation.error.message}`
      : null) ??
    (editAndRetryMutation.error
      ? `Failed to apply edits and retry: ${editAndRetryMutation.error.message}`
      : null)

  return {
    retrySubmitting,
    retryError,
    handleRetryWithoutEdit: () => retryWithoutEditMutation.mutateAsync(),
    handleEditAndRetry: () => editAndRetryMutation.mutateAsync()
  }
}
