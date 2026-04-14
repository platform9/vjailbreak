import { CUTOVER_TYPES } from 'src/features/migration/constants'

type FieldErrors = Record<string, string>

type PostMigrationActionSelection = {
  suffix?: boolean
  folderName?: boolean
  renameVm?: boolean
  moveToFolder?: boolean
}

type PostMigrationActionParams = {
  suffix?: string
  folderName?: string
}

type SelectedMigrationOptionsCore = {
  dataCopyStartTime?: boolean
  cutoverOption?: boolean
  postMigrationScript?: boolean
  useGPU?: boolean
  useFlavorless?: boolean
  periodicSyncEnabled?: boolean
  postMigrationAction?: PostMigrationActionSelection
}

type ParamsCore = {
  dataCopyStartTime?: string
  cutoverOption?: string
  cutoverStartTime?: string
  cutoverEndTime?: string
  postMigrationScript?: string
  useGPU?: boolean
  useFlavorless?: boolean
  periodicSyncInterval?: string
  postMigrationAction?: PostMigrationActionParams
}

export function getPostMigrationActionSelected(postMigrationAction: unknown) {
  return Boolean(
    postMigrationAction &&
      typeof postMigrationAction === 'object' &&
      Object.values(postMigrationAction as Record<string, unknown>).some(Boolean)
  )
}

export function getDataCopyStartTimeOk(input: {
  selectedMigrationOptions: SelectedMigrationOptionsCore
  params: ParamsCore
  fieldErrors: FieldErrors
}) {
  const { selectedMigrationOptions, params, fieldErrors } = input

  const dataCopyStartTimeValue = String(params.dataCopyStartTime ?? '').trim()
  return (
    !selectedMigrationOptions.dataCopyStartTime ||
    (Boolean(dataCopyStartTimeValue) &&
      dataCopyStartTimeValue !== 'undefined' &&
      dataCopyStartTimeValue !== 'null' &&
      !fieldErrors['dataCopyStartTime'])
  )
}

export function getCutoverOk(input: {
  selectedMigrationOptions: SelectedMigrationOptionsCore
  params: ParamsCore
  fieldErrors: FieldErrors
  includePeriodicSyncChecks: boolean
}) {
  const { selectedMigrationOptions, params, fieldErrors, includePeriodicSyncChecks } = input

  const periodicSyncIntervalValue = String(params.periodicSyncInterval ?? '').trim()

  return !selectedMigrationOptions.cutoverOption
    ? true
    : Boolean(
        params.cutoverOption &&
          !fieldErrors['cutoverOption'] &&
          (params.cutoverOption !== CUTOVER_TYPES.TIME_WINDOW ||
            (params.cutoverStartTime &&
              params.cutoverEndTime &&
              !fieldErrors['cutoverStartTime'] &&
              !fieldErrors['cutoverEndTime'])) &&
          (!includePeriodicSyncChecks ||
            params.cutoverOption !== CUTOVER_TYPES.ADMIN_INITIATED ||
            !selectedMigrationOptions.periodicSyncEnabled ||
            (Boolean(periodicSyncIntervalValue) &&
              periodicSyncIntervalValue !== 'undefined' &&
              periodicSyncIntervalValue !== 'null' &&
              !fieldErrors['periodicSyncInterval']))
      )
}

export function getPeriodicSyncOk(input: {
  selectedMigrationOptions: SelectedMigrationOptionsCore
  params: ParamsCore
  fieldErrors: FieldErrors
}) {
  const { selectedMigrationOptions, params, fieldErrors } = input

  const periodicSyncIntervalValue = String(params.periodicSyncInterval ?? '').trim()
  return (
    !selectedMigrationOptions.periodicSyncEnabled ||
    (Boolean(periodicSyncIntervalValue) &&
      periodicSyncIntervalValue !== 'undefined' &&
      periodicSyncIntervalValue !== 'null' &&
      !fieldErrors['periodicSyncInterval'])
  )
}

export function getPostMigrationScriptOk(input: {
  selectedMigrationOptions: SelectedMigrationOptionsCore
  params: ParamsCore
  fieldErrors: FieldErrors
}) {
  const { selectedMigrationOptions, params, fieldErrors } = input

  const postMigrationScriptValue = String(params.postMigrationScript ?? '').trim()
  return (
    !selectedMigrationOptions.postMigrationScript ||
    (postMigrationScriptValue !== '' && !fieldErrors['postMigrationScript'])
  )
}

export function getPcdOptionsOk(input: {
  selectedMigrationOptions: SelectedMigrationOptionsCore
  params: ParamsCore
}) {
  const { selectedMigrationOptions, params } = input

  return (
    (!selectedMigrationOptions.useGPU || typeof params.useGPU === 'boolean') &&
    (!selectedMigrationOptions.useFlavorless || typeof params.useFlavorless === 'boolean')
  )
}

export function getPostMigrationActionOk(input: {
  selectedMigrationOptions: SelectedMigrationOptionsCore
  params: ParamsCore
}) {
  const { selectedMigrationOptions, params } = input

  const postMigrationAction = selectedMigrationOptions.postMigrationAction
  const postMigrationActionSelected = getPostMigrationActionSelected(postMigrationAction)

  return !postMigrationActionSelected
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
}
