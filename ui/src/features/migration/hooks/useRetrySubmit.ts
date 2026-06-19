import { useCallback } from 'react'
import { useMutation, QueryClient } from '@tanstack/react-query'
import { NavigateFunction } from 'react-router-dom'
import { deleteMigration } from 'src/api/migrations/migrations'
import { createNetworkMappingJson } from 'src/api/network-mapping/helpers'
import { postNetworkMapping } from 'src/api/network-mapping/networkMappings'
import { createStorageMappingJson } from 'src/api/storage-mappings/helpers'
import { postStorageMapping } from 'src/api/storage-mappings/storageMappings'
import { patchVMwareMachine } from 'src/api/vmware-machines/vmwareMachines'
import {
  deleteMigrationPlan,
  patchMigrationPlan,
  postMigrationPlan,
  requestMigrationPlanRetry
} from 'src/features/migration/api/migration-plans/migrationPlans'
import { MigrationPlan } from 'src/features/migration/api/migration-plans/model'
import {
  patchMigrationTemplate,
  postMigrationTemplate
} from 'src/features/migration/api/migration-templates/migrationTemplates'
import { MigrationTemplate, VmData } from 'src/features/migration/api/migration-templates/model'
import { MIGRATIONS_QUERY_KEY } from 'src/hooks/api/useMigrationsQuery'
import { CUTOVER_TYPES } from '../constants'
import type { RetryMigrationConfig } from '../context/MigrationFormContext'
import type { FormValues, SelectedMigrationOptionsType } from '../types'

const RETRY_CLONE_OF_ANNOTATION = 'vjailbreak.k8s.pf9.io/retry-clone-of'
const RETRY_CLONE_VM_ANNOTATION = 'vjailbreak.k8s.pf9.io/retry-clone-vm'

// Trim base name to fit within the Kubernetes 63-char DNS label limit imposed by
// the clone suffix, then strip trailing hyphens left by the truncation.
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

  // Single-VM plan path: patch the existing plan/template/mappings in place, then
  // trigger retry via annotation + Migration delete.
  const patchInPlace = useCallback(async () => {
    if (!retryConfig || !retryTemplate) return
    const namespace = retryConfig.namespace
    const planName = retryPlan?.metadata?.name || retryConfig.planName
    const templateName = retryTemplate.metadata?.name

    // 1. Fresh mapping resources (same pattern as the standard form: mappings are
    //    immutable per submission, the template points at the latest ones).
    const templateSpec: Record<string, unknown> = {
      storageCopyMethod: params.storageCopyMethod || 'normal',
      useGPUFlavor: params.useGPU || false,
      useFlavorless: Boolean(selectedMigrationOptions.useFlavorless),
      targetPCDClusterName: selectedPcdClusterName || retryTemplate?.spec?.targetPCDClusterName || ''
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
      await patchVMwareMachine(
        vmK8sName,
        { spec: { targetFlavorId: selectedFlavorId } },
        namespace
      )
    }

    // 5. Trigger the retry only after every edit is persisted.
    await triggerRetry()
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
    triggerRetry
  ])

  // Multi-VM plan path: create a clone plan containing only the retrying VM (with
  // edited config), remove the VM from the original plan, then delete the failed
  // Migration. The controller picks up the clone plan as a normal 1-VM plan.
  //
  // Order matters: clone plan first → patch original → delete Migration. This prevents
  // the controller from recreating the Migration under the original plan before the VM
  // has been removed from it.
  const cloneAndRetry = useCallback(async () => {
    if (!retryConfig || !retryTemplate || !retryPlan) return
    const namespace = retryConfig.namespace
    const suffix = `-r-${Date.now().toString(36).slice(-6)}`
    const retryingVMKey = retryVm?.vmKey || retryVm?.name || retryConfig.vmName

    // 1. Clone NetworkMapping.
    let cloneNetworkMappingName: string | undefined
    if (networkMappingRequired && params.networkMappings?.length) {
      const created = await postNetworkMapping(
        createNetworkMappingJson({ networkMappings: params.networkMappings }),
        namespace
      )
      cloneNetworkMappingName = created.metadata.name
    }

    // 2. Clone StorageMapping.
    let cloneStorageMappingName: string | undefined
    if (params.storageCopyMethod !== 'StorageAcceleratedCopy' && params.storageMappings?.length) {
      const created = await postStorageMapping(
        createStorageMappingJson({ storageMappings: params.storageMappings }),
        namespace
      )
      cloneStorageMappingName = created.metadata.name
    }

    // 3. POST clone MigrationTemplate: inherit immutable source/destination fields from
    //    the original, override everything the user may have edited.
    const originalTemplateSpec = retryTemplate.spec || {}
    const cloneTemplateSpec = {
      ...originalTemplateSpec,
      ...(cloneNetworkMappingName !== undefined && { networkMapping: cloneNetworkMappingName }),
      ...(cloneStorageMappingName !== undefined && { storageMapping: cloneStorageMappingName }),
      storageCopyMethod: params.storageCopyMethod || 'normal',
      useGPUFlavor: params.useGPU || false,
      useFlavorless: Boolean(selectedMigrationOptions.useFlavorless),
      targetPCDClusterName: selectedPcdClusterName || originalTemplateSpec.targetPCDClusterName || '',
      ...(params.storageCopyMethod === 'HotAdd' && params.proxyVMRef
        ? { proxyVMRef: { name: params.proxyVMRef } }
        : {})
    }
    const cloneTemplateName = makeCloneName(retryTemplate.metadata.name, suffix)
    const cloneTemplate = await postMigrationTemplate(
      {
        apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1',
        kind: 'MigrationTemplate',
        metadata: { name: cloneTemplateName },
        spec: cloneTemplateSpec
      },
      namespace
    )

    // 4. POST clone MigrationPlan: single-VM, edited spec, audit-trail annotations.
    //    networkOverridesPerVM is filtered to only the retrying VM so other VMs' overrides
    //    from the form state do not leak into the clone plan.
    const planPatch = buildPlanPatchSpec()
    const filteredOverrides = planPatch.networkOverridesPerVM
      ? Object.fromEntries(
          Object.entries(planPatch.networkOverridesPerVM).filter(([key]) => key === retryingVMKey)
        )
      : null
    const cloneOverrides =
      filteredOverrides && Object.keys(filteredOverrides).length > 0 ? filteredOverrides : null

    const clonePlanName = makeCloneName(retryPlan.metadata.name, suffix)
    const clonePlan: MigrationPlan = await postMigrationPlan(
      {
        apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1',
        kind: 'MigrationPlan',
        metadata: {
          name: clonePlanName,
          annotations: {
            [RETRY_CLONE_OF_ANNOTATION]: retryPlan.metadata.name,
            [RETRY_CLONE_VM_ANNOTATION]: retryingVMKey
          }
        },
        spec: {
          ...planPatch,
          migrationTemplate: cloneTemplate.metadata.name,
          virtualMachines: [[retryingVMKey]],
          retry: false,
          networkOverridesPerVM: cloneOverrides
        }
      },
      namespace
    )

    // 5. PATCH original plan to remove the retrying VM. On failure, delete the clone
    //    plan so Kubernetes GC cascades to any Migration the controller already spawned.
    const updatedVMs = (retryPlan.spec?.virtualMachines || [])
      .map((batch) => batch.filter((v) => v !== retryingVMKey))
      .filter((batch) => batch.length > 0)
    try {
      await patchMigrationPlan(
        retryPlan.metadata.name,
        { spec: { virtualMachines: updatedVMs } },
        namespace
      )
    } catch (err) {
      await deleteMigrationPlan(clonePlan.metadata.name, namespace).catch(() => undefined)
      throw err
    }

    // 6. Per-VM flavor override.
    if (vmK8sName && selectedFlavorId && selectedFlavorId !== (retryVm?.targetFlavorId || '')) {
      await patchVMwareMachine(
        vmK8sName,
        { spec: { targetFlavorId: selectedFlavorId } },
        namespace
      )
    }

    // 7. DELETE the failed Migration last so the controller cannot recreate it under
    //    the original plan between step 4 (clone plan created) and step 5 (VM removed).
    await deleteMigration(retryConfig.migrationName, namespace)
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
      const isMultiVMPlan = (retryPlan?.spec?.virtualMachines?.flat()?.length ?? 0) > 1
      if (isMultiVMPlan) {
        await cloneAndRetry()
      } else {
        await patchInPlace()
      }
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
