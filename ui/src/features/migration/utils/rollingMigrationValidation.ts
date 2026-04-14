import { isMappingComplete } from 'src/features/migration/utils/mappings'
import { CUTOVER_TYPES } from 'src/features/migration/constants'

type FieldErrors = Record<string, string>

type SelectedMigrationOptions = {
  dataCopyMethod: boolean
  dataCopyStartTime: boolean
  cutoverOption: boolean
  postMigrationScript: boolean
  osFamily: boolean
  useGPU?: boolean
  useFlavorless?: boolean
  postMigrationAction?: {
    suffix?: boolean
    folderName?: boolean
    renameVm?: boolean
    moveToFolder?: boolean
  }
}

type Params = {
  dataCopyMethod?: string
  dataCopyStartTime?: string
  cutoverOption?: string
  cutoverStartTime?: string
  cutoverEndTime?: string
  postMigrationScript?: string
  osFamily?: string
  useGPU?: boolean
  useFlavorless?: boolean
  disconnectSourceNetwork?: boolean
  fallbackToDHCP?: boolean
  networkPersistence?: boolean
  postMigrationAction?: {
    suffix?: string
    folderName?: string
  }
  storageCopyMethod?: 'normal' | 'StorageAcceleratedCopy'
}

export function getRollingHasAnyMigrationOptionSelected(input: {
  selectedMigrationOptions: SelectedMigrationOptions
}) {
  const { selectedMigrationOptions } = input

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
}

export function getRollingMigrationOptionValidated(input: {
  hasAnyMigrationOptionSelected: boolean
  selectedMigrationOptions: SelectedMigrationOptions
  params: Params
  fieldErrors: FieldErrors
}) {
  const { hasAnyMigrationOptionSelected, selectedMigrationOptions, params, fieldErrors } = input

  if (!hasAnyMigrationOptionSelected) return true

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

  const postMigrationScriptValue = String(params.postMigrationScript ?? '').trim()
  const postMigrationScriptOk =
    !selectedMigrationOptions.postMigrationScript ||
    (postMigrationScriptValue !== '' && !fieldErrors['postMigrationScript'])

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

  return Boolean(
    dataCopyMethodOk &&
      dataCopyStartTimeOk &&
      cutoverOk &&
      postMigrationScriptOk &&
      postMigrationActionOk
  )
}

export function getRollingAreSelectedMigrationOptionsConfigured(input: {
  selectedMigrationOptions: SelectedMigrationOptions
  params: Params
  fieldErrors: FieldErrors
}) {
  const { selectedMigrationOptions, params, fieldErrors } = input

  const hasAnyMigrationOptionSelected = getRollingHasAnyMigrationOptionSelected({
    selectedMigrationOptions
  })

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

  const postMigrationScriptValue = String(params.postMigrationScript ?? '').trim()
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

  return Boolean(
    dataCopyStartTimeOk &&
      cutoverOk &&
      postMigrationScriptOk &&
      pcdOptionsOk &&
      postMigrationActionOk
  )
}

export function getRollingIsSubmitDisabled(input: {
  sourceCluster: string | undefined
  destinationPCD: string | undefined
  selectedMaasConfig: unknown
  selectedVMsLength: number
  submitting: boolean
  params: Params
  selectedMigrationOptions: SelectedMigrationOptions
  fieldErrors: FieldErrors
  availableVmwareNetworks: string[]
  availableVmwareDatastores: string[]
  networkMappings: Array<{ source: string; target: string }>
  storageMappings: Array<{ source: string; target: string }>
  arrayCredsMappings: Array<{ source: string; target: string }>
}) {
  const {
    sourceCluster,
    destinationPCD,
    selectedMaasConfig,
    selectedVMsLength,
    submitting,
    params,
    selectedMigrationOptions,
    fieldErrors,
    availableVmwareNetworks,
    availableVmwareDatastores,
    networkMappings,
    storageMappings,
    arrayCredsMappings
  } = input

  const basicRequirementsMissing =
    !sourceCluster || !destinationPCD || !selectedMaasConfig || !selectedVMsLength || submitting

  const storageMappingComplete =
    params.storageCopyMethod === 'StorageAcceleratedCopy'
      ? isMappingComplete(availableVmwareDatastores, arrayCredsMappings)
      : isMappingComplete(availableVmwareDatastores, storageMappings)

  const mappingsValid =
    isMappingComplete(availableVmwareNetworks, networkMappings) && storageMappingComplete

  const hasAnyMigrationOptionSelected = getRollingHasAnyMigrationOptionSelected({
    selectedMigrationOptions
  })

  const migrationOptionValidated = getRollingMigrationOptionValidated({
    hasAnyMigrationOptionSelected,
    selectedMigrationOptions,
    params,
    fieldErrors
  })

  return basicRequirementsMissing || !mappingsValid || !migrationOptionValidated
}

export function getRollingStep6Complete(input: {
  isTouched: boolean
  areSelectedMigrationOptionsConfigured: boolean
  params: Params
  step6HasErrors: boolean
}) {
  const { isTouched, areSelectedMigrationOptionsConfigured, params, step6HasErrors } = input

  return Boolean(
    isTouched &&
      (areSelectedMigrationOptionsConfigured ||
        Boolean(
          params.disconnectSourceNetwork || params.fallbackToDHCP || params.networkPersistence
        )) &&
      !step6HasErrors
  )
}

export function getRollingStep6HasErrors(input: {
  isTouched: boolean
  selectedMigrationOptions: SelectedMigrationOptions
  params: Params
  fieldErrors: FieldErrors
}) {
  const { isTouched, selectedMigrationOptions, params, fieldErrors } = input

  return Boolean(
    isTouched &&
      ((selectedMigrationOptions.dataCopyStartTime && fieldErrors['dataCopyStartTime']) ||
        (selectedMigrationOptions.cutoverOption &&
          (fieldErrors['cutoverOption'] ||
            (params.cutoverOption === CUTOVER_TYPES.TIME_WINDOW &&
              (fieldErrors['cutoverStartTime'] || fieldErrors['cutoverEndTime'])))) ||
        (selectedMigrationOptions.postMigrationScript && fieldErrors['postMigrationScript']))
  )
}
