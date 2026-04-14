import type {
  FieldErrors,
  FormValues,
  SelectedMigrationOptionsType
} from 'src/features/migration/types'
import {
  getCutoverOk,
  getDataCopyStartTimeOk,
  getPcdOptionsOk,
  getPeriodicSyncOk,
  getPostMigrationActionOk,
  getPostMigrationActionSelected,
  getPostMigrationScriptOk
} from 'src/features/migration/utils/migrationOptionsValidationCore'

export function getHasAnyMigrationOptionSelected({
  selectedMigrationOptions,
  removeVMwareTools
}: {
  selectedMigrationOptions: SelectedMigrationOptionsType
  removeVMwareTools: boolean | undefined
}) {
  const postMigrationActionSelected = getPostMigrationActionSelected(
    selectedMigrationOptions.postMigrationAction
  )

  return (
    Boolean(selectedMigrationOptions.dataCopyMethod) ||
    Boolean(selectedMigrationOptions.dataCopyStartTime) ||
    Boolean(selectedMigrationOptions.cutoverOption) ||
    Boolean(selectedMigrationOptions.postMigrationScript) ||
    Boolean(removeVMwareTools) ||
    Boolean(selectedMigrationOptions.useGPU) ||
    Boolean(selectedMigrationOptions.useFlavorless) ||
    Boolean(selectedMigrationOptions.periodicSyncEnabled) ||
    postMigrationActionSelected
  )
}

export function getAreSelectedMigrationOptionsConfigured({
  hasAnyMigrationOptionSelected,
  selectedMigrationOptions,
  params,
  fieldErrors
}: {
  hasAnyMigrationOptionSelected: boolean
  selectedMigrationOptions: SelectedMigrationOptionsType
  params: FormValues
  fieldErrors: FieldErrors
}) {
  if (!hasAnyMigrationOptionSelected) return false

  const dataCopyStartTimeOk = getDataCopyStartTimeOk({
    selectedMigrationOptions,
    params,
    fieldErrors
  })

  const cutoverOk = getCutoverOk({
    selectedMigrationOptions,
    params,
    fieldErrors,
    includePeriodicSyncChecks: true
  })

  const periodicSyncOk = getPeriodicSyncOk({
    selectedMigrationOptions,
    params,
    fieldErrors
  })

  const postMigrationScriptOk = getPostMigrationScriptOk({
    selectedMigrationOptions,
    params,
    fieldErrors
  })

  const pcdOptionsOk = getPcdOptionsOk({
    selectedMigrationOptions,
    params
  })

  const postMigrationActionOk = getPostMigrationActionOk({
    selectedMigrationOptions,
    params
  })

  return (
    dataCopyStartTimeOk &&
    cutoverOk &&
    periodicSyncOk &&
    postMigrationScriptOk &&
    pcdOptionsOk &&
    postMigrationActionOk
  )
}

export function getStep5Complete({
  isTouched,
  areSelectedMigrationOptionsConfigured,
  params,
  step5HasErrors
}: {
  isTouched: boolean
  areSelectedMigrationOptionsConfigured: boolean
  params: FormValues
  step5HasErrors: boolean
}) {
  return Boolean(
    isTouched &&
      (areSelectedMigrationOptionsConfigured ||
        Boolean(
          params.disconnectSourceNetwork ||
            params.fallbackToDHCP ||
            params.networkPersistence ||
            params.removeVMwareTools
        )) &&
      !step5HasErrors
  )
}
