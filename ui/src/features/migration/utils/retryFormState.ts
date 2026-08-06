import { MigrationPlan } from 'src/features/migration/api/migration-plans/model'
import { MigrationTemplate, VmData } from 'src/features/migration/api/migration-templates/model'
import { CUTOVER_TYPES } from '../constants'
import type {
  FormValues,
  MigrationDrawerRHFValues,
  SelectedMigrationOptionsType
} from '../types'
import { customMetadataToRecord } from './metadataUtils'

const ZERO_TIME = '0001-01-01T00:00:00Z'
export const DEFAULT_FIRSTBOOT_SCRIPT = 'echo "Add your startup script here!"'

export const isSetTime = (value?: string) => Boolean(value && value !== ZERO_TIME)

export interface RetryFormStateInput {
  plan: MigrationPlan
  template: MigrationTemplate
  vmData: VmData
  clusterName: string
  datacenter: string
  vmwareRef: string
  openstackRef: string
  networkMappings: Array<{ source: string; target: string }>
  storageMappings: Array<{ source: string; target: string }>
  pcdData: Array<{ id: string; name?: string }>
}

export interface RetryFormState {
  params: Partial<FormValues>
  selectedOptions: Partial<SelectedMigrationOptionsType>
  formDefaults: MigrationDrawerRHFValues
}

// Reverse-maps a failed migration's MigrationPlan + MigrationTemplate back into the
// migration form's state so the retry drawer opens pre-populated with exactly what the
// original migration ran with. Pure: every value is derived from the inputs.
export function buildRetryFormState({
  plan,
  template,
  vmData,
  clusterName,
  datacenter,
  vmwareRef,
  openstackRef,
  networkMappings,
  storageMappings,
  pcdData
}: RetryFormStateInput): RetryFormState {
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

  const params: Partial<FormValues> = {
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
    // Data-only is part of the plan's migration strategy, so it must be restored here or
    // the retry silently downgrades to a full migration that creates an OpenStack VM.
    dataOnly: strategy?.dataOnly || false,
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
    }),
    preserveSourceTags: plan.spec?.preserveSourceTags || false,
    ...(plan.spec?.customMetadata &&
      Object.keys(plan.spec.customMetadata).length > 0 && {
        customMetadata: Object.entries(plan.spec.customMetadata).map(([key, value]) => ({
          key,
          value
        }))
      })
  }

  const selectedOptions: Partial<SelectedMigrationOptionsType> = {
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
  }

  const formDefaults: MigrationDrawerRHFValues = {
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
  }

  return { params, selectedOptions, formDefaults }
}

export interface RetryPlanSpecInput {
  params: Partial<FormValues>
  selectedMigrationOptions: SelectedMigrationOptionsType
  retryPlan: MigrationPlan | undefined
}

// Builds the MigrationPlan spec for the replacement plan created by edit-and-retry.
// Pure: mirrors useMigrationFormSubmit's plan payload for the single retrying VM.
export function buildRetryPlanSpec({
  params,
  selectedMigrationOptions,
  retryPlan
}: RetryPlanSpecInput) {
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
      disconnectSourceNetwork: params.disconnectSourceNetwork || false,
      // Always sent (not omitted when false) so unchecking data-only on retry clears the
      // flag instead of the merge keeping the failed migration's value.
      dataOnly: params.dataOnly || false
    },
    securityGroups: params.securityGroups?.length ? params.securityGroups : null,
    serverGroup: params.serverGroup || null,
    fallbackToDHCP: params.fallbackToDHCP || false,
    // Tags and metadata are prefilled into the retry form from the failed plan, so they
    // must be written back too — otherwise a retry silently drops them. null clears
    // metadata the user removed rather than leaving the stale map in place.
    preserveSourceTags: params.preserveSourceTags || false,
    customMetadata: customMetadataToRecord(params.customMetadata) ?? null,
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
}
