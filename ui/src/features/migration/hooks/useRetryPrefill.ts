import { useQuery } from '@tanstack/react-query'
import { useEffect } from 'react'
import type { UseFormReturn } from 'react-hook-form'
import { getMigration } from 'src/api/migrations/migrations'
import { getMigrationPlan } from 'src/features/migration/api/migration-plans/migrationPlans'
import { MigrationPlan } from 'src/features/migration/api/migration-plans/model'
import { getMigrationTemplate } from 'src/features/migration/api/migration-templates/migrationTemplates'
import { MigrationTemplate, VmData } from 'src/features/migration/api/migration-templates/model'
import { getNetworkMapping } from 'src/api/network-mapping/networkMappings'
import { getStorageMapping } from 'src/api/storage-mappings/storageMappings'
import { getVmwareCredentials } from 'src/api/vmware-creds/vmwareCreds'
import { getOpenstackCredentials } from 'src/api/openstack-creds/openstackCreds'
import { getVMwareMachine, mapToVmData } from 'src/api/vmware-machines/vmwareMachines'
import { CUTOVER_TYPES } from '../constants'
import type { RetryMigrationConfig } from '../context/MigrationFormContext'
import type {
  FormValues,
  MigrationDrawerRHFValues,
  SelectedMigrationOptionsType
} from '../types'

const ZERO_TIME = '0001-01-01T00:00:00Z'
const DEFAULT_FIRSTBOOT_SCRIPT = 'echo "Add your startup script here!"'

const isSetTime = (value?: string) => Boolean(value && value !== ZERO_TIME)

interface RetryResources {
  plan: MigrationPlan
  template: MigrationTemplate
  vmData: VmData
  machineName: string
  clusterName: string
  datacenter: string
  vmwareRef: string
  openstackRef: string
  networkMappings: Array<{ source: string; target: string }>
  storageMappings: Array<{ source: string; target: string }>
}

export interface RetryPrefillState {
  prefillLoading: boolean
  blockingError: string | null
  retryPlan: MigrationPlan | undefined
  retryTemplate: MigrationTemplate | undefined
  retryVm: VmData | undefined
  vmK8sName: string | undefined
  sourceCluster: string
}

interface UseRetryPrefillParams {
  open: boolean
  retryConfig?: RetryMigrationConfig
  pcdData: Array<{ id: string; name?: string }>
  updateParams: (values: Partial<FormValues>) => void
  updateSelectedOptions: (values: Partial<SelectedMigrationOptionsType>) => void
  form: UseFormReturn<MigrationDrawerRHFValues>
  setMigrationTemplate: React.Dispatch<React.SetStateAction<MigrationTemplate | undefined>>
}

// Loads the failed migration's plan, template, mappings, credentials and VM, and
// reverse-maps them into the migration form state so the retry form opens fully
// pre-populated. Read-only: performs no writes.
export function useRetryPrefill({
  open,
  retryConfig,
  pcdData,
  updateParams,
  updateSelectedOptions,
  form,
  setMigrationTemplate
}: UseRetryPrefillParams): RetryPrefillState {
  const { migrationName, namespace } = retryConfig ?? {}

  const query = useQuery<RetryResources, Error>({
    queryKey: ['retry-prefill', migrationName, namespace],
    enabled: open && Boolean(retryConfig),
    staleTime: 0,
    retry: false,
    refetchOnWindowFocus: false,
    queryFn: async (): Promise<RetryResources> => {
      const migration = await getMigration(migrationName!, namespace!).catch(() => undefined)
      if (!migration) {
        throw new Error(
          `Migration "${migrationName}" no longer exists. Refresh the migrations list.`
        )
      }

      const planName = migration.spec?.migrationPlan || retryConfig!.planName
      const plan = await getMigrationPlan(planName, namespace!).catch(() => undefined)
      if (!plan) {
        throw new Error(
          `Migration plan "${planName}" no longer exists; this migration cannot be retried.`
        )
      }

      const templateName = plan.spec?.migrationTemplate
      const template = templateName
        ? await getMigrationTemplate(templateName, namespace!).catch(() => undefined)
        : undefined
      if (!template) {
        throw new Error(
          `Migration template "${templateName}" referenced by the plan no longer exists; this migration cannot be retried.`
        )
      }

      const vmwareRef = template.spec?.source?.vmwareRef
      const openstackRef = template.spec?.destination?.openstackRef

      const vmwareCreds = vmwareRef
        ? await getVmwareCredentials(vmwareRef).catch(() => undefined)
        : undefined
      if (!vmwareCreds) {
        throw new Error(
          `VMware credentials "${vmwareRef}" used by this migration no longer exist. Restore them before retrying.`
        )
      }
      const openstackCreds = openstackRef
        ? await getOpenstackCredentials(openstackRef).catch(() => undefined)
        : undefined
      if (!openstackCreds) {
        throw new Error(
          `OpenStack credentials "${openstackRef}" used by this migration no longer exist. Restore them before retrying.`
        )
      }

      const machineName = migrationName!.replace(/^migration-/, '')
      const machine = await getVMwareMachine(machineName, namespace!).catch(() => undefined)
      if (!machine) {
        throw new Error(
          `Source VM "${retryConfig!.vmName}" is no longer present in the inventory; this migration cannot be retried.`
        )
      }

      // Fetch mappings in parallel — missing mappings are not blocking.
      const [networkMappings, storageMappings] = await Promise.all([
        template.spec?.networkMapping
          ? getNetworkMapping(template.spec.networkMapping, namespace!)
              .then((nm) => nm?.spec?.networks ?? [])
              .catch(() => [])
          : Promise.resolve([]),
        template.spec?.storageMapping
          ? getStorageMapping(template.spec.storageMapping, namespace!)
              .then((sm) => sm?.spec?.storages ?? [])
              .catch(() => [])
          : Promise.resolve([])
      ])

      const rawVmData = mapToVmData([machine])[0]
      const vmKey = rawVmData.vmKey || rawVmData.name
      const existingOverrides = plan.spec?.networkOverridesPerVM?.[vmKey]
      let vmData: VmData = rawVmData as unknown as VmData
      if (existingOverrides?.length) {
        const preserveIp: Record<number, boolean> = {}
        const preserveMac: Record<number, boolean> = {}
        const networkInterfaces = (rawVmData.networkInterfaces || []).map((nic) => ({ ...nic }))
        existingOverrides.forEach((override) => {
          preserveIp[override.interfaceIndex] = override.preserveIP
          preserveMac[override.interfaceIndex] = override.preserveMAC
          if (!override.preserveIP && override.UserAssignedIP) {
            const nic = networkInterfaces[override.interfaceIndex]
            if (nic) {
              networkInterfaces[override.interfaceIndex] = {
                ...nic,
                ipAddress: override.UserAssignedIP.split(',')
              }
            }
          }
        })
        vmData = { ...vmData, preserveIp, preserveMac, networkInterfaces }
      }

      const machineAnnotations = (
        machine.metadata as { annotations?: Record<string, string> }
      )?.annotations
      const datacenter =
        machineAnnotations?.['vjailbreak.k8s.pf9.io/datacenter'] ||
        template.spec?.source?.datacenter ||
        ''
      const clusterName = machine.spec?.vms?.clusterName || ''

      return {
        plan,
        template,
        vmData,
        machineName,
        clusterName,
        datacenter,
        vmwareRef: vmwareRef || '',
        openstackRef: openstackRef || '',
        networkMappings,
        storageMappings
      }
    }
  })

  // Seed form state once when query data arrives. pcdData is intentionally omitted from
  // deps — MigrationForm's pcdCluster re-resolve effect handles the name→id swap when
  // pcdData loads after the drawer opens.
  useEffect(() => {
    if (!query.data) return
    const {
      plan,
      template,
      vmData,
      clusterName,
      datacenter,
      vmwareRef,
      openstackRef,
      networkMappings,
      storageMappings
    } = query.data
    const strategy = plan.spec?.migrationStrategy
    const advanced = plan.spec?.advancedOptions
    const pcd = pcdData.find((p) => p.name === template.spec?.targetPCDClusterName)

    const cutoverOption = strategy?.adminInitiatedCutOver
      ? CUTOVER_TYPES.ADMIN_INITIATED
      : isSetTime(strategy?.vmCutoverStart) || isSetTime(strategy?.vmCutoverEnd)
        ? CUTOVER_TYPES.TIME_WINDOW
        : CUTOVER_TYPES.IMMEDIATE

    const firstBootScript =
      plan.spec?.firstBootScript && plan.spec.firstBootScript !== DEFAULT_FIRSTBOOT_SCRIPT
        ? plan.spec.firstBootScript
        : undefined

    setMigrationTemplate(template)

    updateParams({
      vmwareCreds: { existingCredName: vmwareRef, datacenter } as FormValues['vmwareCreds'],
      openstackCreds: { existingCredName: openstackRef } as FormValues['openstackCreds'],
      vmwareCluster: `${vmwareRef}:${datacenter}:${clusterName}`,
      pcdCluster: pcd?.id || template.spec?.targetPCDClusterName || '',
      vms: [vmData],
      networkMappings,
      storageMappings,
      storageCopyMethod: (template.spec?.storageCopyMethod ||
        'normal') as FormValues['storageCopyMethod'],
      ...(template.spec?.proxyVMRef?.name && { proxyVMRef: template.spec.proxyVMRef.name }),
      dataCopyMethod: strategy?.type || 'cold',
      useGPU: template.spec?.useGPUFlavor || false,
      disconnectSourceNetwork: strategy?.disconnectSourceNetwork || false,
      fallbackToDHCP: plan.spec?.fallbackToDHCP || false,
      securityGroups: plan.spec?.securityGroups ?? [],
      serverGroup: plan.spec?.serverGroup ?? '',
      ...(isSetTime(strategy?.dataCopyStart) && { dataCopyStartTime: strategy?.dataCopyStart }),
      cutoverOption,
      ...(isSetTime(strategy?.vmCutoverStart) && { cutoverStartTime: strategy?.vmCutoverStart }),
      ...(isSetTime(strategy?.vmCutoverEnd) && { cutoverEndTime: strategy?.vmCutoverEnd }),
      ...(firstBootScript && { postMigrationScript: firstBootScript }),
      ...(plan.spec?.postMigrationAction && {
        postMigrationAction: plan.spec.postMigrationAction
      }),
      ...(typeof advanced?.networkPersistence === 'boolean' && {
        networkPersistence: advanced.networkPersistence
      }),
      ...(typeof advanced?.removeVMwareTools === 'boolean' && {
        removeVMwareTools: advanced.removeVMwareTools
      }),
      ...(typeof advanced?.acknowledgeNetworkConflictRisk === 'boolean' && {
        acknowledgeNetworkConflictRisk: advanced.acknowledgeNetworkConflictRisk
      }),
      ...(advanced?.imageProfiles?.length && { imageProfiles: advanced.imageProfiles }),
      ...(advanced?.periodicSyncInterval && {
        periodicSyncInterval: advanced.periodicSyncInterval
      })
    })

    updateSelectedOptions({
      dataCopyMethod: true,
      dataCopyStartTime: isSetTime(strategy?.dataCopyStart),
      cutoverOption: cutoverOption !== CUTOVER_TYPES.IMMEDIATE,
      cutoverStartTime: isSetTime(strategy?.vmCutoverStart),
      cutoverEndTime: isSetTime(strategy?.vmCutoverEnd),
      postMigrationScript: Boolean(firstBootScript),
      useGPU: template.spec?.useGPUFlavor || false,
      periodicSyncEnabled: advanced?.periodicSyncEnabled || false,
      ...(plan.spec?.postMigrationAction && {
        postMigrationAction: {
          renameVm: Boolean(plan.spec.postMigrationAction.renameVm),
          suffix: Boolean(plan.spec.postMigrationAction.suffix),
          moveToFolder: Boolean(plan.spec.postMigrationAction.moveToFolder),
          folderName: Boolean(plan.spec.postMigrationAction.folderName)
        }
      })
    })

    form.reset({
      securityGroups: plan.spec?.securityGroups ?? [],
      serverGroup: plan.spec?.serverGroup ?? '',
      dataCopyStartTime: isSetTime(strategy?.dataCopyStart)
        ? (strategy?.dataCopyStart as string)
        : '',
      cutoverStartTime: isSetTime(strategy?.vmCutoverStart)
        ? (strategy?.vmCutoverStart as string)
        : '',
      cutoverEndTime: isSetTime(strategy?.vmCutoverEnd) ? (strategy?.vmCutoverEnd as string) : '',
      postMigrationActionSuffix: plan.spec?.postMigrationAction?.suffix ?? '',
      postMigrationActionFolderName: plan.spec?.postMigrationAction?.folderName ?? ''
    })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [query.data, pcdData])

  return {
    prefillLoading: query.isLoading,
    blockingError: query.isError ? query.error.message : null,
    retryPlan: query.data?.plan,
    retryTemplate: query.data?.template,
    retryVm: query.data?.vmData,
    vmK8sName: query.data?.machineName,
    sourceCluster: query.data?.clusterName ?? ''
  }
}
