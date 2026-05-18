import { useState, useMemo } from 'react'
import { GridRowSelectionModel } from '@mui/x-data-grid'
import { vmHasInterface } from 'src/features/migration/utils/vmNetworking'
import { BMConfig } from 'src/api/bmconfig/model'
import { CUTOVER_TYPES } from '../constants'
import type { VM, ESXHost, FieldErrors, ResourceMap, SelectedMigrationOptionsType, RollingFormParams } from '../types'

interface UseRollingFormValidationParams {
  selectedVMs: GridRowSelectionModel
  vmsWithAssignments: VM[]
  orderedESXHosts: ESXHost[]
  vmOSAssignments: Record<string, string>
  sourceCluster: string
  destinationPCD: string
  selectedMaasConfig: BMConfig | null
  submitting: boolean
  availableVmwareNetworks: string[]
  networkMappings: ResourceMap[]
  availableVmwareDatastores: string[]
  storageMappings: ResourceMap[]
  arrayCredsMappings: ResourceMap[]
  selectedMigrationOptions: SelectedMigrationOptionsType
  params: RollingFormParams
  fieldErrors: FieldErrors
}

export function useRollingFormValidation({
  selectedVMs,
  vmsWithAssignments,
  orderedESXHosts,
  vmOSAssignments,
  sourceCluster,
  destinationPCD,
  selectedMaasConfig,
  submitting,
  availableVmwareNetworks,
  networkMappings,
  availableVmwareDatastores,
  storageMappings,
  arrayCredsMappings,
  selectedMigrationOptions,
  params,
  fieldErrors
}: UseRollingFormValidationParams) {
  const [vmIpValidationError, setVmIpValidationError] = useState<string>('')
  const [esxHostConfigValidationError, setEsxHostConfigValidationError] = useState<string>('')
  const [osValidationError, setOsValidationError] = useState<string>('')

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
    const basicRequirementsMissing =
      !sourceCluster || !destinationPCD || !selectedMaasConfig || !selectedVMs.length || submitting

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
    sourceCluster,
    destinationPCD,
    selectedMaasConfig,
    submitting,
    selectedVMs,
    availableVmwareNetworks,
    networkMappings,
    availableVmwareDatastores,
    storageMappings,
    arrayCredsMappings,
    selectedMigrationOptions,
    params,
    fieldErrors,
    orderedESXHosts,
    vmIpValidation.hasError,
    esxHostConfigValidation.hasError,
    osValidation.hasError
  ])

  return {
    vmIpValidationError,
    esxHostConfigValidationError,
    osValidationError,
    esxHostMappingStatus,
    vmIpValidation,
    esxHostConfigValidation,
    osValidation,
    isSubmitDisabled
  }
}
