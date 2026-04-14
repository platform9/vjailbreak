import { useCallback } from 'react'
import axios from 'axios'
import type { QueryClient } from '@tanstack/react-query'
import { patchMigrationTemplate } from 'src/features/migration/api/migration-templates/migrationTemplates'
import { createMigrationPlanJson } from 'src/features/migration/api/migration-plans/helpers'
import { postMigrationPlan } from 'src/features/migration/api/migration-plans/migrationPlans'
import type { MigrationPlan } from 'src/features/migration/api/migration-plans/model'
import type { MigrationTemplate } from 'src/features/migration/api/migration-templates/model'
import { AMPLITUDE_EVENTS } from 'src/types/amplitude'
import type { FormValues, SelectedMigrationOptionsType } from 'src/features/migration/types'
import { CUTOVER_TYPES } from 'src/features/migration/constants'
import { buildAssignedIPsPerVM, buildNetworkOverridesPerVM } from 'src/features/migration/utils'
import { createMigrationMappingsResources } from 'src/features/migration/hooks/createMigrationMappingsResources'

type TrackFn = (event: string, properties?: Record<string, unknown>) => void

type ReportErrorFn = (
  error: Error,
  context: {
    context: string
    metadata?: Record<string, unknown>
  }
) => void

type SetErrorFn = (value: { title: string; message: string } | null) => void

type GetFieldErrorsUpdater = (key: string | number) => (value: string) => void

type NavigateFn = (path: string) => void

export function useMigrationSubmit({
  params,
  selectedMigrationOptions,
  migrationTemplate,
  queryClient,
  migrationsQueryKey,
  onSuccess,
  onClose,
  navigate,
  setSubmitting,
  setError,
  getFieldErrorsUpdater,
  track,
  reportError
}: {
  params: FormValues
  selectedMigrationOptions: SelectedMigrationOptionsType
  migrationTemplate: MigrationTemplate | undefined
  queryClient: QueryClient
  migrationsQueryKey: unknown[]
  onSuccess?: (message: string) => void
  onClose: () => void
  navigate: NavigateFn
  setSubmitting: (value: boolean) => void
  setError: SetErrorFn
  getFieldErrorsUpdater: GetFieldErrorsUpdater
  track: TrackFn
  reportError: ReportErrorFn
}) {
  const updateMigrationTemplate = useCallback(
    async (
      template: MigrationTemplate | undefined,
      networkMappings: any,
      storageMappings: any,
      arrayCredsMapping: any = null
    ) => {
      const migrationTemplateName = template?.metadata?.name
      const storageCopyMethod = params.storageCopyMethod || 'normal'

      const updatedMigrationTemplateFields: any = {
        spec: {
          networkMapping: networkMappings.metadata.name,
          storageCopyMethod
        }
      }

      if (storageCopyMethod === 'StorageAcceleratedCopy' && arrayCredsMapping) {
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
    },
    [params.storageCopyMethod, setError]
  )

  const createMigrationPlan = useCallback(
    async (updatedMigrationTemplate?: MigrationTemplate | null): Promise<MigrationPlan> => {
      if (!updatedMigrationTemplate?.metadata?.name) {
        throw new Error('Migration template is not available')
      }

      const postMigrationAction = selectedMigrationOptions.postMigrationAction
        ? params.postMigrationAction
        : undefined

      const vmsToMigrate = (params.vms || []).map((vm) => vm.vmKey || vm.name)

      const assignedIPsPerVM = buildAssignedIPsPerVM(params.vms)
      const networkOverridesPerVM = buildNetworkOverridesPerVM(params.vms)

      const migrationFields = {
        migrationTemplateName: updatedMigrationTemplate?.metadata?.name,
        virtualMachines: vmsToMigrate,
        type: params.dataCopyMethod,
        ...(assignedIPsPerVM && { assignedIPsPerVM }),
        ...(networkOverridesPerVM && { networkOverridesPerVM }),
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
        periodicSyncInterval: params.periodicSyncInterval,
        periodicSyncEnabled: selectedMigrationOptions.periodicSyncEnabled,
        acknowledgeNetworkConflictRisk: params.acknowledgeNetworkConflictRisk
      }

      const body = createMigrationPlanJson(migrationFields)

      try {
        const data = await postMigrationPlan(body)
        track(AMPLITUDE_EVENTS.MIGRATION_CREATED, {
          migrationName: data.metadata?.name,
          migrationTemplateName: updatedMigrationTemplate?.metadata?.name,
          virtualMachineCount: vmsToMigrate?.length || 0,
          migrationType: migrationFields.type,
          hasDataCopyStartTime: !!migrationFields.dataCopyStart,
          hasAdminInitiatedCutover: !!migrationFields.adminInitiatedCutOver,
          hasTimedCutover: !!(migrationFields.vmCutoverStart && migrationFields.vmCutoverEnd),
          postMigrationAction,
          namespace: data.metadata?.namespace
        })

        return data
      } catch (error: unknown) {
        console.error('Error creating migration plan', error)

        track(AMPLITUDE_EVENTS.MIGRATION_CREATION_FAILED, {
          migrationTemplateName: updatedMigrationTemplate?.metadata?.name,
          virtualMachineCount: vmsToMigrate?.length || 0,
          migrationType: migrationFields.type,
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

        if (axios.isAxiosError(error)) {
          errorMessage = error.response?.data?.message || error.message || String(error)
        } else if (error instanceof Error) {
          errorMessage = error.message
        } else {
          errorMessage = String(error)
        }

        setError({
          title: 'Error creating migration plan',
          message: errorMessage
        })

        getFieldErrorsUpdater('migrationPlan')(`Error creating migration plan: ${errorMessage}`)
        throw error
      }
    },
    [getFieldErrorsUpdater, params, reportError, selectedMigrationOptions, setError, track]
  )

  const submit = useCallback(async () => {
    setSubmitting(true)
    setError(null)

    const storageCopyMethod = params.storageCopyMethod || 'normal'

    let mappingResources: {
      networkMapping: any
      storageMapping: any | null
      arrayCredsMapping: any | null
    }

    try {
      mappingResources = await createMigrationMappingsResources({
        networkMappings: params.networkMappings,
        storageMappings: params.storageMappings,
        arrayCredsMappings: params.arrayCredsMappings,
        storageCopyMethod,
        setError,
        getFieldErrorsUpdater,
        reportError
      })
    } catch (_err) {
      setSubmitting(false)
      return
    }

    const updatedMigrationTemplate = await updateMigrationTemplate(
      migrationTemplate,
      mappingResources.networkMapping,
      mappingResources.storageMapping,
      mappingResources.arrayCredsMapping
    )

    await createMigrationPlan(updatedMigrationTemplate)

    setSubmitting(false)
    queryClient.invalidateQueries({ queryKey: migrationsQueryKey as any })

    onSuccess?.('Migration submitted successfully')

    onClose()
    navigate('/dashboard/migrations')
  }, [
    createMigrationPlan,
    getFieldErrorsUpdater,
    migrationTemplate,
    migrationsQueryKey,
    navigate,
    onClose,
    onSuccess,
    params.arrayCredsMappings,
    params.networkMappings,
    params.storageCopyMethod,
    params.storageMappings,
    queryClient,
    reportError,
    setError,
    setSubmitting,
    updateMigrationTemplate
  ])

  return { submit }
}
