import { useState, useMemo } from 'react'
import { GridRowSelectionModel } from '@mui/x-data-grid'
import { vmHasInterface } from 'src/features/migration/utils/vmNetworking'
import { BMConfig } from 'src/api/bmconfig/model'
import { CUTOVER_TYPES } from '../constants'
import type { VM, ESXHost, FieldErrors, SelectedMigrationOptionsType, RollingFormParams } from '../types'
import type { SectionNavItem } from 'src/components'

interface UseRollingFormValidationParams {
  selectedVMs: GridRowSelectionModel
  vmsWithAssignments: VM[]
  orderedESXHosts: ESXHost[]
  vmOSAssignments: Record<string, string>
  selectedMaasConfig: BMConfig | null
  submitting: boolean
  availableVmwareNetworks: string[]
  availableVmwareDatastores: string[]
  selectedMigrationOptions: SelectedMigrationOptionsType
  touchedSections: {
    sourceDestination: boolean
    baremetal: boolean
    hosts: boolean
    vms: boolean
    mapResources: boolean
    security: boolean
    options: boolean
  }
  params: RollingFormParams
  fieldErrors: FieldErrors
}

export function useRollingFormValidation({
  selectedVMs,
  vmsWithAssignments,
  orderedESXHosts,
  vmOSAssignments,
  selectedMaasConfig,
  submitting,
  availableVmwareNetworks,
  availableVmwareDatastores,
  selectedMigrationOptions,
  touchedSections,
  params,
  fieldErrors
}: UseRollingFormValidationParams) {
  const [vmIpValidationError, setVmIpValidationError] = useState<string>('')
  const [esxHostConfigValidationError, setEsxHostConfigValidationError] = useState<string>('')
  const [osValidationError, setOsValidationError] = useState<string>('')
  const [networkMappingError, setNetworkMappingError] = useState<string>('')
  const [storageMappingError, setStorageMappingError] = useState<string>('')

  const esxHostMappingStatus = useMemo(() => {
    const mappedHostsCount = orderedESXHosts.filter((host) => host.pcdHostConfigName).length
    return {
      mapped: mappedHostsCount,
      total: orderedESXHosts.length,
      fullyMapped: mappedHostsCount === orderedESXHosts.length
    }
  }, [orderedESXHosts])

  const vmIpValidation = useMemo(() => {
    if (selectedVMs.length === 0) {
      setVmIpValidationError('Please select VMs to assign IP addresses.')
      return { hasError: true, vmsWithoutIPs: [] }
    }

    const selectedVMsData = vmsWithAssignments.filter((vm) => selectedVMs.includes(vm.id))
    const vmsRequiringIp = selectedVMsData.filter(vmHasInterface)
    const vmsWithoutIPs = vmsRequiringIp.filter((vm) => vm.ip === '—' || !vm.ip)

    if (vmsWithoutIPs.length > 0) {
      const errorMessage = `Cannot proceed with Migration: ${vmsWithoutIPs.length} selected VM${vmsWithoutIPs.length === 1 ? '' : 's'} with network interfaces do not have IP addresses assigned. Please assign IP addresses to all selected VMs before continuing.`
      setVmIpValidationError(errorMessage)
      return { hasError: true, vmsWithoutIPs }
    } else {
      setVmIpValidationError('')
      return { hasError: false, vmsWithoutIPs: [] }
    }
  }, [selectedVMs, vmsWithAssignments])

  const esxHostConfigValidation = useMemo(() => {
    if (orderedESXHosts.length === 0) {
      setEsxHostConfigValidationError('Please select VMs to migrate.')
      return { hasError: true, hostsWithoutConfigs: [] }
    }

    const hostsWithoutConfigs = orderedESXHosts.filter((host) => !host.pcdHostConfigName)

    if (hostsWithoutConfigs.length > 0) {
      const errorMessage = `Cannot proceed with Migration: ${hostsWithoutConfigs.length} ESXi host${hostsWithoutConfigs.length === 1 ? '' : 's'} do not have Host Config assigned. Please assign Host Config to all ESXi hosts before continuing.`
      setEsxHostConfigValidationError(errorMessage)
      return { hasError: true, hostsWithoutConfigs }
    } else {
      setEsxHostConfigValidationError('')
      return { hasError: false, hostsWithoutConfigs: [] }
    }
  }, [orderedESXHosts])

  const osValidation = useMemo(() => {
    if (selectedVMs.length === 0) {
      setOsValidationError('Please select VMs to assign OS.')
      return { hasError: true, vmsWithoutOS: [] }
    }

    const selectedVMsData = vmsWithAssignments.filter((vm) => selectedVMs.includes(vm.id))
    const poweredOffVMsWithoutOS = selectedVMsData.filter((vm) => {
      const assignedOS = vmOSAssignments[vm.id]
      const currentOS = assignedOS || vm.osFamily
      return vm.powerState === 'powered-off' && (!currentOS || currentOS === 'Unknown')
    })

    if (poweredOffVMsWithoutOS.length > 0) {
      const errorMessage = `Cannot proceed with Migration: ${poweredOffVMsWithoutOS.length} powered-off VM${poweredOffVMsWithoutOS.length === 1 ? '' : 's'} do not have Operating System assigned. Please assign OS to all powered-off VMs before continuing.`
      setOsValidationError(errorMessage)
      return { hasError: true, vmsWithoutOS: poweredOffVMsWithoutOS }
    } else {
      setOsValidationError('')
      return { hasError: false, vmsWithoutOS: [] }
    }
  }, [selectedVMs, vmsWithAssignments, vmOSAssignments])

  const isSubmitDisabled = useMemo(() => {
    const networkMappings = params.networkMappings ?? []
    const storageMappings = params.storageMappings ?? []
    const arrayCredsMappings = params.arrayCredsMappings ?? []

    const basicRequirementsMissing =
      !params.vmwareCluster || !params.pcdCluster || !selectedMaasConfig || !selectedVMs.length || submitting

    const storageMappingComplete =
      params.storageCopyMethod === 'StorageAcceleratedCopy'
        ? availableVmwareDatastores.every((d) => arrayCredsMappings.some((m) => m.source === d))
        : availableVmwareDatastores.every((d) => storageMappings.some((m) => m.source === d))

    const mappingsValid = !(
      availableVmwareNetworks.some(
        (network) => !networkMappings.some((mapping) => mapping.source === network)
      ) || !storageMappingComplete
    )

    const postMigrationAction = selectedMigrationOptions.postMigrationAction
    const postMigrationActionSelected = Boolean(
      postMigrationAction &&
        typeof postMigrationAction === 'object' &&
        Object.values(postMigrationAction as Record<string, unknown>).some(Boolean)
    )

    const hasAnyMigrationOptionSelected =
      Boolean(selectedMigrationOptions.dataCopyMethod) ||
      Boolean(selectedMigrationOptions.dataCopyStartTime) ||
      Boolean(selectedMigrationOptions.cutoverOption) ||
      Boolean(selectedMigrationOptions.postMigrationScript) ||
      Boolean(selectedMigrationOptions.osFamily) ||
      Boolean(selectedMigrationOptions.useGPU) ||
      Boolean(selectedMigrationOptions.useFlavorless) ||
      postMigrationActionSelected

    const dataCopyMethodOk =
      !selectedMigrationOptions.dataCopyMethod || Boolean(params.dataCopyMethod)

    const dataCopyStartTimeValue = String(params.dataCopyStartTime ?? '').trim()
    const dataCopyStartTimeOk =
      !selectedMigrationOptions.dataCopyStartTime ||
      (Boolean(dataCopyStartTimeValue) &&
        dataCopyStartTimeValue !== 'undefined' &&
        dataCopyStartTimeValue !== 'null' &&
        !fieldErrors['dataCopyStartTime'])

    const cutoverOk = !selectedMigrationOptions.cutoverOption
      ? true
      : Boolean(
          params.cutoverOption &&
            !fieldErrors['cutoverOption'] &&
            (params.cutoverOption !== CUTOVER_TYPES.TIME_WINDOW ||
              (params.cutoverStartTime &&
                params.cutoverEndTime &&
                !fieldErrors['cutoverStartTime'] &&
                !fieldErrors['cutoverEndTime']))
        )

    const postMigrationScriptOk =
      !selectedMigrationOptions.postMigrationScript ||
      (Boolean(params.postMigrationScript) && !fieldErrors['postMigrationScript'])

    const osFamilyOk = !selectedMigrationOptions.osFamily || Boolean(params.osFamily)

    const pcdOptionsOk =
      (!selectedMigrationOptions.useGPU || typeof params.useGPU === 'boolean') &&
      (!selectedMigrationOptions.useFlavorless || typeof params.useFlavorless === 'boolean')

    const postMigrationActionOk = !postMigrationActionSelected
      ? true
      : Boolean(
          postMigrationAction &&
            typeof postMigrationAction === 'object' &&
            (Boolean(postMigrationAction.renameVm) ||
              Boolean(postMigrationAction.moveToFolder) ||
              !postMigrationAction.suffix ||
              Boolean((params as any)?.postMigrationActionSuffix) ||
              !postMigrationAction.folderName ||
              Boolean((params as any)?.postMigrationActionFolderName))
        )

    const migrationOptionValidated =
      !hasAnyMigrationOptionSelected ||
      (dataCopyMethodOk &&
        dataCopyStartTimeOk &&
        cutoverOk &&
        postMigrationScriptOk &&
        osFamilyOk &&
        pcdOptionsOk &&
        postMigrationActionOk)

    const esxHostConfigValid = !esxHostConfigValidation.hasError
    const ipValidationPassed = !vmIpValidation.hasError
    const osValidationPassed = !osValidation.hasError

    return (
      basicRequirementsMissing ||
      !mappingsValid ||
      !migrationOptionValidated ||
      !esxHostConfigValid ||
      !ipValidationPassed ||
      !osValidationPassed
    )
  }, [
    params.vmwareCluster,
    params.pcdCluster,
    selectedMaasConfig,
    submitting,
    selectedVMs,
    availableVmwareNetworks,
    availableVmwareDatastores,
    selectedMigrationOptions,
    params,
    fieldErrors,
    orderedESXHosts,
    vmIpValidation.hasError,
    esxHostConfigValidation.hasError,
    osValidation.hasError
  ])

  const step1HasErrors = false

  const step2HasErrors = false

  const step3HasErrors = Boolean(touchedSections.hosts && Boolean(esxHostConfigValidationError))

  const step4HasErrors = Boolean(
    touchedSections.vms && Boolean(vmIpValidationError || osValidationError)
  )

  const step5HasErrors = Boolean(
    touchedSections.mapResources && Boolean(networkMappingError || storageMappingError)
  )

  const step6HasErrors = Boolean(
    touchedSections.options &&
      ((selectedMigrationOptions.dataCopyStartTime && fieldErrors['dataCopyStartTime']) ||
        (selectedMigrationOptions.cutoverOption &&
          (fieldErrors['cutoverOption'] ||
            (params.cutoverOption === CUTOVER_TYPES.TIME_WINDOW &&
              (fieldErrors['cutoverStartTime'] || fieldErrors['cutoverEndTime'])))) ||
        (selectedMigrationOptions.postMigrationScript && fieldErrors['postMigrationScript']))
  )

  const hasAnyMigrationOptionSelected = useMemo(() => {
    const postMigrationAction = selectedMigrationOptions.postMigrationAction
    const postMigrationActionSelected = Boolean(
      postMigrationAction &&
        typeof postMigrationAction === 'object' &&
        Object.values(postMigrationAction as Record<string, unknown>).some(Boolean)
    )

    return (
      Boolean(selectedMigrationOptions.dataCopyMethod) ||
      Boolean(selectedMigrationOptions.dataCopyStartTime) ||
      Boolean(selectedMigrationOptions.cutoverOption) ||
      Boolean(selectedMigrationOptions.postMigrationScript) ||
      Boolean(selectedMigrationOptions.osFamily) ||
      Boolean(selectedMigrationOptions.useGPU) ||
      Boolean(selectedMigrationOptions.useFlavorless) ||
      postMigrationActionSelected
    )
  }, [selectedMigrationOptions])

  const areSelectedMigrationOptionsConfigured = useMemo(() => {
    if (!hasAnyMigrationOptionSelected) return false

    const dataCopyStartTimeValue = String(params.dataCopyStartTime ?? '').trim()
    const dataCopyStartTimeOk =
      !selectedMigrationOptions.dataCopyStartTime ||
      (Boolean(dataCopyStartTimeValue) &&
        dataCopyStartTimeValue !== 'undefined' &&
        dataCopyStartTimeValue !== 'null' &&
        !fieldErrors['dataCopyStartTime'])

    const cutoverOk = !selectedMigrationOptions.cutoverOption
      ? true
      : Boolean(
          params.cutoverOption &&
            !fieldErrors['cutoverOption'] &&
            (params.cutoverOption !== CUTOVER_TYPES.TIME_WINDOW ||
              (params.cutoverStartTime &&
                params.cutoverEndTime &&
                !fieldErrors['cutoverStartTime'] &&
                !fieldErrors['cutoverEndTime']))
        )

    const postMigrationScriptOk =
      !selectedMigrationOptions.postMigrationScript ||
      (Boolean(params.postMigrationScript) && !fieldErrors['postMigrationScript'])

    const osFamilyOk = !selectedMigrationOptions.osFamily || Boolean(params.osFamily)

    const pcdOptionsOk =
      (!selectedMigrationOptions.useGPU || typeof params.useGPU === 'boolean') &&
      (!selectedMigrationOptions.useFlavorless || typeof params.useFlavorless === 'boolean')

    const postMigrationAction = selectedMigrationOptions.postMigrationAction
    const postMigrationActionSelected = Boolean(
      postMigrationAction &&
        typeof postMigrationAction === 'object' &&
        Object.values(postMigrationAction as Record<string, unknown>).some(Boolean)
    )

    const postMigrationActionOk = !postMigrationActionSelected
      ? true
      : Boolean(
          postMigrationAction &&
            typeof postMigrationAction === 'object' &&
            (Boolean(postMigrationAction.renameVm) ||
              Boolean(postMigrationAction.moveToFolder) ||
              !postMigrationAction.suffix ||
              Boolean((params as any)?.postMigrationActionSuffix) ||
              !postMigrationAction.folderName ||
              Boolean((params as any)?.postMigrationActionFolderName))
        )

    return (
      dataCopyStartTimeOk &&
      cutoverOk &&
      postMigrationScriptOk &&
      osFamilyOk &&
      pcdOptionsOk &&
      postMigrationActionOk
    )
  }, [
    hasAnyMigrationOptionSelected,
    selectedMigrationOptions,
    params.dataCopyStartTime,
    params.cutoverOption,
    params.cutoverStartTime,
    params.cutoverEndTime,
    params.postMigrationScript,
    params.osFamily,
    params.useGPU,
    params.useFlavorless,
    params,
    fieldErrors
  ])

  const sectionNavItems = useMemo<SectionNavItem[]>(
    () => [
      {
        id: 'source-destination',
        title: 'Source And Destination',
        description: 'Pick clusters',
        status:
          touchedSections.sourceDestination && params.vmwareCluster && params.pcdCluster
            ? 'complete'
            : step1HasErrors
              ? 'attention'
              : 'incomplete'
      },
      {
        id: 'baremetal',
        title: 'Bare Metal Config',
        description: 'Verify configuration',
        status:
          touchedSections.baremetal && selectedMaasConfig
            ? 'complete'
            : step2HasErrors
              ? 'attention'
              : 'incomplete'
      },
      {
        id: 'hosts',
        title: 'ESXi Hosts',
        description: 'Assign host configs',
        status:
          touchedSections.hosts && orderedESXHosts.length > 0
            ? 'complete'
            : step3HasErrors
              ? 'attention'
              : 'incomplete'
      },
      {
        id: 'vms',
        title: 'Select VMs',
        description: 'Choose VMs and required fields',
        status:
          touchedSections.vms && selectedVMs.length > 0
            ? 'complete'
            : step4HasErrors
              ? 'attention'
              : 'incomplete'
      },
      {
        id: 'map-resources',
        title: 'Map Networks And Storage',
        description: 'Map VMware resources to PCD',
        status:
          touchedSections.mapResources &&
          availableVmwareNetworks.every((n) => (params.networkMappings ?? []).some((m) => m.source === n)) &&
          (params.storageCopyMethod === 'StorageAcceleratedCopy'
            ? availableVmwareDatastores.every((d) => (params.arrayCredsMappings ?? []).some((m) => m.source === d))
            : availableVmwareDatastores.every((d) => (params.storageMappings ?? []).some((m) => m.source === d)))
            ? 'complete'
            : step5HasErrors
              ? 'attention'
              : 'incomplete'
      },
      {
        id: 'security',
        title: 'Security Groups & Server Group',
        description: 'Optional placement and security settings',
        status: touchedSections.security ? 'complete' : 'incomplete'
      },
      {
        id: 'options',
        title: 'Migration Options',
        description: 'Scheduling and advanced behavior',
        status: step6HasErrors
          ? 'attention'
          : touchedSections.options &&
              (areSelectedMigrationOptionsConfigured ||
                Boolean(
                  params.disconnectSourceNetwork ||
                    params.fallbackToDHCP ||
                    params.networkPersistence
                )) &&
              !step6HasErrors
            ? 'complete'
            : 'incomplete'
      }
    ],
    [
      params.vmwareCluster,
      params.pcdCluster,
      selectedMaasConfig,
      orderedESXHosts.length,
      esxHostConfigValidation.hasError,
      selectedVMs.length,
      vmIpValidation.hasError,
      osValidation.hasError,
      networkMappingError,
      storageMappingError,
      availableVmwareNetworks,
      availableVmwareDatastores,
      params.networkMappings,
      params.storageMappings,
      params.arrayCredsMappings,
      params.cutoverOption,
      selectedMigrationOptions.cutoverOption,
      selectedMigrationOptions.dataCopyStartTime,
      selectedMigrationOptions.postMigrationScript,
      fieldErrors,
      step1HasErrors,
      step2HasErrors,
      step3HasErrors,
      step4HasErrors,
      step5HasErrors,
      step6HasErrors,
      hasAnyMigrationOptionSelected,
      areSelectedMigrationOptionsConfigured,
      touchedSections,
      params.disconnectSourceNetwork,
      params.fallbackToDHCP,
      params.networkPersistence
    ]
  )

  return {
    vmIpValidationError,
    esxHostConfigValidationError,
    osValidationError,
    networkMappingError,
    storageMappingError,
    setNetworkMappingError,
    setStorageMappingError,
    esxHostMappingStatus,
    vmIpValidation,
    esxHostConfigValidation,
    osValidation,
    isSubmitDisabled,
    step1HasErrors,
    step2HasErrors,
    step3HasErrors,
    step4HasErrors,
    step5HasErrors,
    step6HasErrors,
    hasAnyMigrationOptionSelected,
    areSelectedMigrationOptionsConfigured,
    sectionNavItems
  }
}
