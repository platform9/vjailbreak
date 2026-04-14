import type { FieldErrors, FormValues, SelectedMigrationOptionsType } from 'src/features/migration/types'
import { CUTOVER_TYPES } from 'src/features/migration/constants'

export function getHasAnyMigrationOptionSelected({
  selectedMigrationOptions,
  removeVMwareTools
}: {
  selectedMigrationOptions: SelectedMigrationOptionsType
  removeVMwareTools: boolean | undefined
}) {
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
