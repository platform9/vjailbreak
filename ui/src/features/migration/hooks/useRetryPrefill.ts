import { useEffect, useState } from 'react'
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
  const [prefillLoading, setPrefillLoading] = useState(false)
  const [blockingError, setBlockingError] = useState<string | null>(null)
  const [retryPlan, setRetryPlan] = useState<MigrationPlan | undefined>(undefined)
  const [retryTemplate, setRetryTemplate] = useState<MigrationTemplate | undefined>(undefined)
  const [retryVm, setRetryVm] = useState<VmData | undefined>(undefined)
  const [vmK8sName, setVmK8sName] = useState<string | undefined>(undefined)
  const [sourceCluster, setSourceCluster] = useState('')

  useEffect(() => {
    if (!open || !retryConfig) return
    let cancelled = false

    const fail = (message: string) => {
      if (!cancelled) setBlockingError(message)
    }

    const load = async () => {
      setPrefillLoading(true)
      setBlockingError(null)
      try {
        const { migrationName, namespace } = retryConfig

        const migration = await getMigration(migrationName, namespace).catch(() => undefined)
        if (!migration) {
          fail(`Migration "${migrationName}" no longer exists. Refresh the migrations list.`)
          return
        }

        const planName = migration.spec?.migrationPlan || retryConfig.planName
        const plan = await getMigrationPlan(planName, namespace).catch(() => undefined)
        if (!plan) {
          fail(`Migration plan "${planName}" no longer exists; this migration cannot be retried.`)
          return
        }

        const templateName = plan.spec?.migrationTemplate
        const template = templateName
          ? await getMigrationTemplate(templateName, namespace).catch(() => undefined)
          : undefined
        if (!template) {
          fail(
            `Migration template "${templateName}" referenced by the plan no longer exists; this migration cannot be retried.`
          )
          return
        }

        const vmwareRef = template.spec?.source?.vmwareRef
        const openstackRef = template.spec?.destination?.openstackRef
        const vmwareCreds = vmwareRef
          ? await getVmwareCredentials(vmwareRef).catch(() => undefined)
          : undefined
        if (!vmwareCreds) {
          fail(
            `VMware credentials "${vmwareRef}" used by this migration no longer exist. Restore them before retrying.`
          )
          return
        }
        const openstackCreds = openstackRef
          ? await getOpenstackCredentials(openstackRef).catch(() => undefined)
          : undefined
        if (!openstackCreds) {
          fail(
            `OpenStack credentials "${openstackRef}" used by this migration no longer exist. Restore them before retrying.`
          )
          return
        }

        // Migration objects are named "migration-<vm-k8s-name>" by the controller.
        const machineName = migrationName.replace(/^migration-/, '')
        const machine = await getVMwareMachine(machineName, namespace).catch(() => undefined)
        if (!machine) {
          fail(
            `Source VM "${retryConfig.vmName}" is no longer present in the inventory; this migration cannot be retried.`
          )
          return
        }

        // Missing mappings are not blocking: the user re-maps in the form and
        // "Edit & Retry" creates fresh mapping resources.
        const networkMappings = template.spec?.networkMapping
          ? await getNetworkMapping(template.spec.networkMapping, namespace)
              .then((nm) => nm?.spec?.networks ?? [])
              .catch(() => [])
          : []
        const storageMappings = template.spec?.storageMapping
          ? await getStorageMapping(template.spec.storageMapping, namespace)
              .then((sm) => sm?.spec?.storages ?? [])
              .catch(() => [])
          : []

        if (cancelled) return

        // Seed the existing template so the form never auto-creates a temporary one.
        setMigrationTemplate(template)

        const rawVmData = mapToVmData([machine])[0]

        // Restore existing per-VM IP overrides from the plan into vmData so the VM
        // selection table shows the current IP state and buildPlanPatchSpec can detect
        // whether the user changed anything.
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

        // Match the datacenter source order used by the Migration Details drawer:
        // annotation on VMwareMachine takes priority, template spec is the fallback.
        const machineAnnotations = (machine.metadata as { annotations?: Record<string, string> })?.annotations
        const datacenter =
          machineAnnotations?.['vjailbreak.k8s.pf9.io/datacenter'] ||
          template.spec?.source?.datacenter ||
          ''
        const clusterName = machine.spec?.vms?.clusterName || ''
        setSourceCluster(clusterName)
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
          useFlavorless: template.spec?.useFlavorless || false,
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

        setRetryPlan(plan)
        setRetryTemplate(template)
        setRetryVm(vmData)
        setVmK8sName(machineName)
      } catch (err) {
        fail(
          `Failed to load the configuration of the failed migration: ${
            err instanceof Error ? err.message : String(err)
          }`
        )
      } finally {
        if (!cancelled) setPrefillLoading(false)
      }
    }

    load()
    return () => {
      cancelled = true
    }
    // Intentionally limited deps: prefill runs once per drawer open for a retry target.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, retryConfig])

  return { prefillLoading, blockingError, retryPlan, retryTemplate, retryVm, vmK8sName, sourceCluster }
}
