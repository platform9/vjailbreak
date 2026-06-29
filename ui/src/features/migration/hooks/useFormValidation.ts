import { useMemo } from 'react'
import { uniq } from 'ramda'
import { flatten } from 'ramda'
import { useRdmConfigValidation } from 'src/hooks/useRdmConfigValidation'
import { useNetworkMappingValidation } from 'src/hooks/useNetworkMappingValidation'
import { RdmDisk } from 'src/api/rdm-disks/model'
import { OpenstackCreds, PCDNetworkInfo } from 'src/api/openstack-creds/model'
import { isNilOrEmpty } from 'src/utils'
import type { SectionNavItem } from 'src/components'
import { CUTOVER_TYPES } from '../constants'
import type { FormValues, FieldErrors, SelectedMigrationOptionsType } from '../types'

const stringsCompareFn = (a: string, b: string) => a.toLowerCase().localeCompare(b.toLowerCase())

interface UseFormValidationParams {
  params: Partial<FormValues>
  fieldErrors: FieldErrors
  selectedMigrationOptions: SelectedMigrationOptionsType
  vmwareCredsValidated: boolean
  openstackCredsValidated: boolean
  rdmDisks: RdmDisk[]
  openstackCredentials: OpenstackCreds | undefined
  touchedSections: { options: boolean }
}

interface UseFormValidationResult {
  availableVmwareNetworks: string[]
  availableVmwareDatastores: string[]
  sortedOpenstackNetworks: PCDNetworkInfo[]
  sortedOpenstackVolumeTypes: string[]
  migrationOptionValidated: boolean
  vmValidation: { hasError: boolean; errorMessage: string }
  rdmValidation: ReturnType<typeof useRdmConfigValidation>
  storageValidation: boolean
  networkMappingRequired: boolean
  disableSubmit: boolean
  isStep1Complete: boolean
  isStep2Complete: boolean
  isStep3Complete: boolean
  step4Complete: boolean
  step5Complete: boolean
  step1HasErrors: boolean
  step2HasErrors: boolean
  step3HasErrors: boolean
  step5HasErrors: boolean
  unmappedNetworksCount: number
  unmappedStorageCount: number
  hasAnyMigrationOptionSelected: boolean
  areSelectedMigrationOptionsConfigured: boolean
  sectionNavItems: SectionNavItem[]
}

export function useFormValidation({
  params,
  fieldErrors,
  selectedMigrationOptions,
  vmwareCredsValidated,
  openstackCredsValidated,
  rdmDisks,
  openstackCredentials,
  touchedSections
}: UseFormValidationParams): UseFormValidationResult {
  const availableVmwareNetworks = useMemo(() => {
    if (params.vms === undefined) return []
    return uniq(flatten(params.vms.map((vm) => vm.networks || []))).sort(stringsCompareFn)
  }, [params.vms])

  const availableVmwareDatastores = useMemo(() => {
    if (params.vms === undefined) return []
    return uniq(flatten(params.vms.map((vm) => vm.datastores || []))).sort(stringsCompareFn)
  }, [params.vms])

  const { required: networkMappingRequired } = useNetworkMappingValidation({
    selectedVMs: params.vms || [],
    networkMappings: params.networkMappings || [],
    availableVmwareNetworks
  })

  const sortedOpenstackNetworks = useMemo(() => {
    const networks = openstackCredentials?.status?.openstack?.networks || []
    if (!Array.isArray(networks) || networks.length === 0) return []

    return (networks as PCDNetworkInfo[])
      .filter((n) => n && typeof n.name === 'string')
      .slice()
      .sort((a, b) => stringsCompareFn(a?.name, b?.name))
  }, [openstackCredentials?.status?.openstack?.networks])

  const sortedOpenstackVolumeTypes = useMemo(
    () => (openstackCredentials?.status?.openstack?.volumeTypes || []).sort(stringsCompareFn),
    [openstackCredentials?.status?.openstack?.volumeTypes]
  )

  const migrationOptionValidated = useMemo(() => {
    return Object.keys(selectedMigrationOptions).every((key) => {
      if (key === 'postMigrationAction') {
        return true
      }
      if (key === 'periodicSyncEnabled' && selectedMigrationOptions.periodicSyncEnabled) {
        return params?.periodicSyncInterval !== '' && fieldErrors['periodicSyncInterval'] === ''
      }
      if (key === 'dataCopyStartTime' && selectedMigrationOptions.dataCopyStartTime) {
        const value = String(params?.dataCopyStartTime ?? '').trim()
        return value !== '' && !fieldErrors['dataCopyStartTime']
      }
      if (key === 'postMigrationScript' && selectedMigrationOptions.postMigrationScript) {
        const value = String(params?.postMigrationScript ?? '').trim()
        return value !== '' && !fieldErrors['postMigrationScript']
      }
      if (selectedMigrationOptions[key as keyof typeof selectedMigrationOptions]) {
        return params?.[key as keyof typeof params] !== undefined && !fieldErrors[key]
      }
      return true
    })
  }, [selectedMigrationOptions, params, fieldErrors])

  const vmValidation = useMemo(() => {
    if (!params.vms || params.vms.length === 0) {
      return { hasError: false, errorMessage: '' }
    }

    const poweredOffVMs = params.vms.filter((vm) => {
      const powerState = vm.vmState === 'running' ? 'powered-on' : 'powered-off'
      return powerState === 'powered-off'
    })

    const poweredOnVMs = params.vms.filter((vm) => {
      const powerState = vm.vmState === 'running' ? 'powered-on' : 'powered-off'
      return powerState === 'powered-on'
    })

    const vmsWithoutOSAssigned = poweredOffVMs
      .filter((vm) => !vm.osFamily || vm.osFamily === 'Unknown' || vm.osFamily.trim() === '')
      .concat(
        poweredOnVMs.filter(
          (vm) => !vm.osFamily || vm.osFamily === 'Unknown' || vm.osFamily.trim() === ''
        )
      )

    if (vmsWithoutOSAssigned.length > 0) {
      let errorMessage = 'Cannot proceed with migration: '
      const issues: string[] = []

      if (vmsWithoutOSAssigned.length > 0) {
        issues.push(
          `We could not detect the operating system for ${vmsWithoutOSAssigned.length} VM${
            vmsWithoutOSAssigned.length === 1 ? '' : 's'
          }`
        )
      }

      errorMessage +=
        issues.join(' and ') + '. Please assign the required information before continuing.'

      return { hasError: true, errorMessage }
    }

    return { hasError: false, errorMessage: '' }
  }, [params.vms])

  const rdmValidation = useRdmConfigValidation({
    selectedVMs: params.vms || [],
    rdmDisks: rdmDisks,
    backendVolumeTypeMap: openstackCredentials?.status?.openstack?.backendVolumeTypeMap
  })

  const storageCopyMethod = params.storageCopyMethod || 'normal'

  const storageValidation =
    storageCopyMethod === 'HotAdd'
      ? Boolean(params.proxyVMRef) &&
        !isNilOrEmpty(params.storageMappings) &&
        !availableVmwareDatastores.some(
          (datastore) => !params.storageMappings?.some((mapping) => mapping.source === datastore)
        )
      : storageCopyMethod === 'StorageAcceleratedCopy'
        ? !isNilOrEmpty(params.arrayCredsMappings) &&
          !availableVmwareDatastores.some(
            (datastore) =>
              !params.arrayCredsMappings?.some((mapping) => mapping.source === datastore)
          )
        : !isNilOrEmpty(params.storageMappings) &&
          !availableVmwareDatastores.some(
            (datastore) => !params.storageMappings?.some((mapping) => mapping.source === datastore)
          )

  const disableSubmit =
    !vmwareCredsValidated ||
    !openstackCredsValidated ||
    isNilOrEmpty(params.vms) ||
    (networkMappingRequired && isNilOrEmpty(params.networkMappings)) ||
    isNilOrEmpty(params.vmwareCluster) ||
    isNilOrEmpty(params.pcdCluster) ||
    availableVmwareNetworks.some(
      (network) => !params.networkMappings?.some((mapping) => mapping.source === network)
    ) ||
    !storageValidation ||
    !migrationOptionValidated ||
    (params.dataCopyMethod === 'mock' && !Boolean(params['acknowledgeNetworkConflictRisk'])) ||
    vmValidation.hasError ||
    rdmValidation.hasValidationError

  const isStep1Complete = Boolean(
    params.vmwareCluster &&
      params.pcdCluster &&
      !fieldErrors['vmwareCluster'] &&
      !fieldErrors['pcdCluster'] &&
      !fieldErrors['vmwareCreds'] &&
      !fieldErrors['openstackCreds']
  )

  const isStep2Complete = Boolean(
    (params.vms?.length || 0) > 0 &&
      !fieldErrors['vms'] &&
      !vmValidation.hasError &&
      !rdmValidation.hasConfigError
  )

  const isStep3Complete = useMemo(() => {
    if (!params.vms || params.vms.length === 0) return false
    if (fieldErrors['networksMapping'] || fieldErrors['storageMapping']) return false

    const networkMapped =
      availableVmwareNetworks.length === 0 ||
      availableVmwareNetworks.every((network) =>
        (params.networkMappings || []).some((m) => m.source === network)
      )

    const currentStorageCopyMethod = params.storageCopyMethod || 'normal'
    const storageMapped =
      currentStorageCopyMethod === 'HotAdd'
        ? Boolean(params.proxyVMRef) &&
          availableVmwareDatastores.every((datastore) =>
            (params.storageMappings || []).some((m) => m.source === datastore)
          )
        : currentStorageCopyMethod === 'StorageAcceleratedCopy'
          ? availableVmwareDatastores.every((datastore) =>
              (params.arrayCredsMappings || []).some((m) => m.source === datastore)
            )
          : availableVmwareDatastores.every((datastore) =>
              (params.storageMappings || []).some((m) => m.source === datastore)
            )

    return networkMapped && storageMapped
  }, [
    params.vms,
    params.networkMappings,
    params.storageMappings,
    params.arrayCredsMappings,
    params.storageCopyMethod,
    params.proxyVMRef,
    availableVmwareNetworks,
    availableVmwareDatastores,
    fieldErrors
  ])

  const unmappedNetworksCount = useMemo(() => {
    return availableVmwareNetworks.filter(
      (network) => !(params.networkMappings || []).some((m) => m.source === network)
    ).length
  }, [availableVmwareNetworks, params.networkMappings])

  const unmappedStorageCount = useMemo(() => {
    const currentStorageCopyMethod = params.storageCopyMethod || 'normal'
    if (currentStorageCopyMethod === 'HotAdd') return 0
    if (currentStorageCopyMethod === 'StorageAcceleratedCopy') {
      return availableVmwareDatastores.filter(
        (ds) => !(params.arrayCredsMappings || []).some((m) => m.source === ds)
      ).length
    }
    return availableVmwareDatastores.filter(
      (ds) => !(params.storageMappings || []).some((m) => m.source === ds)
    ).length
  }, [
    availableVmwareDatastores,
    params.storageMappings,
    params.arrayCredsMappings,
    params.storageCopyMethod
  ])

  const step1HasErrors = Boolean(
    fieldErrors['vmwareCluster'] ||
      fieldErrors['pcdCluster'] ||
      fieldErrors['vmwareCreds'] ||
      fieldErrors['openstackCreds']
  )

  const step2HasErrors = Boolean(
    fieldErrors['vms'] || vmValidation.hasError || rdmValidation.hasConfigError
  )

  const step3HasErrors = Boolean(fieldErrors['networksMapping'] || fieldErrors['storageMapping'])

  const step4Complete = Boolean(
    (params.securityGroups && params.securityGroups.length > 0) ||
      params.serverGroup ||
      (params.imageProfiles && params.imageProfiles.length > 0)
  )

  const step5HasErrors = Boolean(
    (selectedMigrationOptions.dataCopyStartTime && fieldErrors['dataCopyStartTime']) ||
      (selectedMigrationOptions.cutoverOption &&
        (fieldErrors['cutoverOption'] ||
          (params.cutoverOption === CUTOVER_TYPES.TIME_WINDOW &&
            (fieldErrors['cutoverStartTime'] || fieldErrors['cutoverEndTime'])) ||
          (params.cutoverOption === CUTOVER_TYPES.ADMIN_INITIATED &&
            selectedMigrationOptions.periodicSyncEnabled &&
            fieldErrors['periodicSyncInterval']))) ||
      (selectedMigrationOptions.postMigrationScript && fieldErrors['postMigrationScript'])
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
      Boolean(params.removeVMwareTools) ||
      Boolean(selectedMigrationOptions.useGPU) ||
      Boolean(selectedMigrationOptions.useFlavorless) ||
      Boolean(selectedMigrationOptions.periodicSyncEnabled) ||
      postMigrationActionSelected
    )
  }, [selectedMigrationOptions, params.removeVMwareTools])

  const areSelectedMigrationOptionsConfigured = useMemo(() => {
    if (!hasAnyMigrationOptionSelected) return false

    const dataCopyStartTimeValue = String(params.dataCopyStartTime ?? '').trim()
    const periodicSyncIntervalValue = String(params.periodicSyncInterval ?? '').trim()
    const postMigrationScriptValue = String(params.postMigrationScript ?? '').trim()

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
                !fieldErrors['cutoverEndTime'])) &&
            (params.cutoverOption !== CUTOVER_TYPES.ADMIN_INITIATED ||
              !selectedMigrationOptions.periodicSyncEnabled ||
              (Boolean(periodicSyncIntervalValue) &&
                periodicSyncIntervalValue !== 'undefined' &&
                periodicSyncIntervalValue !== 'null' &&
                !fieldErrors['periodicSyncInterval']))
        )

    const periodicSyncOk =
      !selectedMigrationOptions.periodicSyncEnabled ||
      (Boolean(periodicSyncIntervalValue) &&
        periodicSyncIntervalValue !== 'undefined' &&
        periodicSyncIntervalValue !== 'null' &&
        !fieldErrors['periodicSyncInterval'])

    const postMigrationScriptOk =
      !selectedMigrationOptions.postMigrationScript ||
      (postMigrationScriptValue !== '' && !fieldErrors['postMigrationScript'])

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
              Boolean(params.postMigrationAction?.suffix) ||
              !postMigrationAction.folderName ||
              Boolean(params.postMigrationAction?.folderName))
        )

    return (
      dataCopyStartTimeOk &&
      cutoverOk &&
      periodicSyncOk &&
      postMigrationScriptOk &&
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
    params.periodicSyncInterval,
    params.postMigrationScript,
    params.useGPU,
    params.useFlavorless,
    params.postMigrationAction,
    fieldErrors
  ])

  const anyNetworkOptionSet = Boolean(
    params.disconnectSourceNetwork ||
      params.fallbackToDHCP ||
      params.networkPersistence ||
      params.removeVMwareTools
  )

  const step5Complete = Boolean(
    (touchedSections.options || anyNetworkOptionSet) &&
      (areSelectedMigrationOptionsConfigured || anyNetworkOptionSet) &&
      !step5HasErrors
  )

  const sectionNavItems = useMemo<SectionNavItem[]>(
    () => [
      {
        id: 'source-destination',
        title: 'Source And Destination',
        description: 'Pick clusters and credentials',
        status: isStep1Complete ? 'complete' : step1HasErrors ? 'attention' : 'incomplete'
      },
      {
        id: 'select-vms',
        title: 'Select VMs',
        description: 'Choose VMs and assign required fields',
        status: isStep2Complete ? 'complete' : step2HasErrors ? 'attention' : 'incomplete'
      },
      {
        id: 'map-resources',
        title: 'Map Networks And Storage',
        description: 'Map VMware networks/datastores to PCD',
        status: isStep3Complete ? 'complete' : step3HasErrors ? 'attention' : 'incomplete'
      },
      {
        id: 'security',
        title: 'Security And Placement',
        description: 'Security groups and server group',
        status: step4Complete ? 'complete' : 'incomplete'
      },
      {
        id: 'options',
        title: 'Migration Options',
        description: 'Scheduling and advanced behavior',
        status: step5HasErrors ? 'attention' : step5Complete ? 'complete' : 'incomplete'
      }
    ],
    [
      isStep1Complete,
      isStep2Complete,
      isStep3Complete,
      step4Complete,
      step1HasErrors,
      step2HasErrors,
      step3HasErrors,
      step5HasErrors,
      step5Complete
    ]
  )

  return {
    availableVmwareNetworks,
    availableVmwareDatastores,
    sortedOpenstackNetworks,
    sortedOpenstackVolumeTypes,
    migrationOptionValidated,
    vmValidation,
    rdmValidation,
    storageValidation,
    networkMappingRequired,
    disableSubmit,
    isStep1Complete,
    isStep2Complete,
    isStep3Complete,
    step4Complete,
    step5Complete,
    step1HasErrors,
    step2HasErrors,
    step3HasErrors,
    step5HasErrors,
    unmappedNetworksCount,
    unmappedStorageCount,
    hasAnyMigrationOptionSelected,
    areSelectedMigrationOptionsConfigured,
    sectionNavItems
  }
}
