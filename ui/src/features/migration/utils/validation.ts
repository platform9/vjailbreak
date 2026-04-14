import { isMappingComplete } from './mappings'

export type FieldErrors = Record<string, string>

export type VmValidation = {
  hasError: boolean
}

export type RdmValidation = {
  hasValidationError: boolean
  hasConfigError: boolean
}

export type MigrationFormValidationInput = {
  vmwareCredsValidated: boolean
  openstackCredsValidated: boolean
  params: {
    vms?: unknown[]
    networkMappings?: Array<{ source: string; target: string }>
    storageMappings?: Array<{ source: string; target: string }>
    arrayCredsMappings?: Array<{ source: string; target: string }>
    storageCopyMethod?: 'normal' | 'StorageAcceleratedCopy'
    vmwareCluster?: string
    pcdCluster?: string
    dataCopyMethod?: string
    acknowledgeNetworkConflictRisk?: unknown
  }
  availableVmwareNetworks: string[]
  availableVmwareDatastores: string[]
  fieldErrors: FieldErrors
  migrationOptionValidated: boolean
  vmValidation: VmValidation
  rdmValidation: RdmValidation
}

export const getStorageMappingValid = (input: {
  storageCopyMethod: 'normal' | 'StorageAcceleratedCopy'
  availableVmwareDatastores: string[]
  storageMappings?: Array<{ source: string; target: string }>
  arrayCredsMappings?: Array<{ source: string; target: string }>
}): boolean => {
  const { storageCopyMethod, availableVmwareDatastores, storageMappings, arrayCredsMappings } = input

  if (storageCopyMethod === 'StorageAcceleratedCopy') {
    return Array.isArray(arrayCredsMappings) &&
      arrayCredsMappings.length > 0 &&
      isMappingComplete(availableVmwareDatastores, arrayCredsMappings)
  }

  return Array.isArray(storageMappings) &&
    storageMappings.length > 0 &&
    isMappingComplete(availableVmwareDatastores, storageMappings)
}

export const getDisableSubmit = (input: MigrationFormValidationInput): boolean => {
  const {
    vmwareCredsValidated,
    openstackCredsValidated,
    params,
    availableVmwareNetworks,
    availableVmwareDatastores,
    migrationOptionValidated,
    vmValidation,
    rdmValidation
  } = input

  const storageCopyMethod = params.storageCopyMethod || 'normal'
  const storageMappingValid = getStorageMappingValid({
    storageCopyMethod,
    availableVmwareDatastores,
    storageMappings: params.storageMappings,
    arrayCredsMappings: params.arrayCredsMappings
  })

  return (
    !vmwareCredsValidated ||
    !openstackCredsValidated ||
    !Array.isArray(params.vms) ||
    params.vms.length === 0 ||
    !Array.isArray(params.networkMappings) ||
    params.networkMappings.length === 0 ||
    !params.vmwareCluster ||
    !params.pcdCluster ||
    !isMappingComplete(availableVmwareNetworks, params.networkMappings) ||
    !storageMappingValid ||
    !migrationOptionValidated ||
    (params.dataCopyMethod === 'mock' && !Boolean(params.acknowledgeNetworkConflictRisk)) ||
    vmValidation.hasError ||
    rdmValidation.hasValidationError
  )
}

export const getStepCompletion = (input: {
  params: {
    vmwareCluster?: string
    pcdCluster?: string
    vms?: unknown[]
    securityGroups?: string[]
    serverGroup?: string
    storageCopyMethod?: 'normal' | 'StorageAcceleratedCopy'
    networkMappings?: Array<{ source: string; target: string }>
    storageMappings?: Array<{ source: string; target: string }>
    arrayCredsMappings?: Array<{ source: string; target: string }>
  }
  fieldErrors: FieldErrors
  availableVmwareNetworks: string[]
  availableVmwareDatastores: string[]
  vmValidation: VmValidation
  rdmValidation: { hasConfigError: boolean }
}): {
  isStep1Complete: boolean
  isStep2Complete: boolean
  isStep3Complete: boolean
  step4Complete: boolean
} => {
  const {
    params,
    fieldErrors,
    availableVmwareNetworks,
    availableVmwareDatastores,
    vmValidation,
    rdmValidation
  } = input

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

  const currentStorageCopyMethod = params.storageCopyMethod || 'normal'
  const storageMapped =
    currentStorageCopyMethod === 'StorageAcceleratedCopy'
      ? isMappingComplete(availableVmwareDatastores, params.arrayCredsMappings)
      : isMappingComplete(availableVmwareDatastores, params.storageMappings)

  const isStep3Complete = Boolean(
    (params.vms?.length || 0) > 0 &&
      !fieldErrors['networksMapping'] &&
      !fieldErrors['storageMapping'] &&
      isMappingComplete(availableVmwareNetworks, params.networkMappings) &&
      storageMapped
  )

  const step4Complete = Boolean(
    (Array.isArray(params.securityGroups) && params.securityGroups.length > 0) || params.serverGroup
  )

  return { isStep1Complete, isStep2Complete, isStep3Complete, step4Complete }
}
