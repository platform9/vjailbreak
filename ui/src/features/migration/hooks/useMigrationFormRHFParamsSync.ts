import { useEffect } from 'react'
import type { UseFormReturn } from 'react-hook-form'
import type { FormValues, SelectedMigrationOptionsType } from 'src/features/migration/types'

type MigrationDrawerRHFValues = {
  securityGroups: string[]
  serverGroup: string
  dataCopyStartTime: string
  cutoverStartTime: string
  cutoverEndTime: string
  postMigrationActionSuffix: string
  postMigrationActionFolderName: string
}

const stringArrayEqual = (a: string[] | undefined, b: string[] | undefined) => {
  if (a === b) return true
  if (!a || !b) return false
  if (a.length !== b.length) return false
  return a.every((v, i) => v === b[i])
}

export function useMigrationFormRHFParamsSync({
  form,
  params,
  getParamsUpdater,
  selectedMigrationOptions,
  rhfValues
}: {
  form: UseFormReturn<MigrationDrawerRHFValues, any, MigrationDrawerRHFValues>
  params: FormValues
  getParamsUpdater: (key: string) => (value: unknown) => void
  selectedMigrationOptions: SelectedMigrationOptionsType
  rhfValues: {
    securityGroups: unknown
    serverGroup: unknown
    dataCopyStartTime: unknown
    cutoverStartTime: unknown
    cutoverEndTime: unknown
    postMigrationActionSuffix: unknown
    postMigrationActionFolderName: unknown
  }
}) {
  useEffect(() => {
    const nextSecurityGroups = params.securityGroups ?? []
    const nextServerGroup = params.serverGroup ?? ''

    const nextDataCopyStartTime = params.dataCopyStartTime ?? ''
    const nextCutoverStartTime = params.cutoverStartTime ?? ''
    const nextCutoverEndTime = params.cutoverEndTime ?? ''
    const nextPostMigrationActionSuffix = params.postMigrationAction?.suffix ?? ''
    const nextPostMigrationActionFolderName = params.postMigrationAction?.folderName ?? ''

    const currentSecurityGroups = form.getValues('securityGroups') ?? []
    const currentServerGroup = form.getValues('serverGroup') ?? ''
    const currentDataCopyStartTime = form.getValues('dataCopyStartTime') ?? ''
    const currentCutoverStartTime = form.getValues('cutoverStartTime') ?? ''
    const currentCutoverEndTime = form.getValues('cutoverEndTime') ?? ''
    const currentPostMigrationActionSuffix = form.getValues('postMigrationActionSuffix') ?? ''
    const currentPostMigrationActionFolderName =
      form.getValues('postMigrationActionFolderName') ?? ''

    if (!stringArrayEqual(currentSecurityGroups, nextSecurityGroups)) {
      form.setValue('securityGroups', nextSecurityGroups)
    }
    if (currentServerGroup !== nextServerGroup) {
      form.setValue('serverGroup', nextServerGroup)
    }

    if (currentDataCopyStartTime !== nextDataCopyStartTime) {
      form.setValue('dataCopyStartTime', nextDataCopyStartTime)
    }
    if (currentCutoverStartTime !== nextCutoverStartTime) {
      form.setValue('cutoverStartTime', nextCutoverStartTime)
    }
    if (currentCutoverEndTime !== nextCutoverEndTime) {
      form.setValue('cutoverEndTime', nextCutoverEndTime)
    }
    if (currentPostMigrationActionSuffix !== nextPostMigrationActionSuffix) {
      form.setValue('postMigrationActionSuffix', nextPostMigrationActionSuffix)
    }
    if (currentPostMigrationActionFolderName !== nextPostMigrationActionFolderName) {
      form.setValue('postMigrationActionFolderName', nextPostMigrationActionFolderName)
    }
  }, [
    form,
    params.securityGroups,
    params.serverGroup,
    params.dataCopyStartTime,
    params.cutoverStartTime,
    params.cutoverEndTime,
    params.postMigrationAction
  ])

  useEffect(() => {
    const nextDataCopyStartTime = (rhfValues.dataCopyStartTime ?? '') as string
    if ((params.dataCopyStartTime ?? '') !== nextDataCopyStartTime) {
      getParamsUpdater('dataCopyStartTime')(nextDataCopyStartTime)
    }
  }, [getParamsUpdater, params.dataCopyStartTime, rhfValues.dataCopyStartTime])

  useEffect(() => {
    const nextCutoverStartTime = (rhfValues.cutoverStartTime ?? '') as string
    if ((params.cutoverStartTime ?? '') !== nextCutoverStartTime) {
      getParamsUpdater('cutoverStartTime')(nextCutoverStartTime)
    }
  }, [getParamsUpdater, params.cutoverStartTime, rhfValues.cutoverStartTime])

  useEffect(() => {
    const nextCutoverEndTime = (rhfValues.cutoverEndTime ?? '') as string
    if ((params.cutoverEndTime ?? '') !== nextCutoverEndTime) {
      getParamsUpdater('cutoverEndTime')(nextCutoverEndTime)
    }
  }, [getParamsUpdater, params.cutoverEndTime, rhfValues.cutoverEndTime])

  useEffect(() => {
    const nextSuffix = String(rhfValues.postMigrationActionSuffix ?? '')
    const normalized = nextSuffix.trim() ? nextSuffix.trim() : ''
    const current = params.postMigrationAction?.suffix ?? ''

    const renameEnabled = !!selectedMigrationOptions.postMigrationAction?.renameVm
    if (!renameEnabled) return

    if (current !== normalized) {
      getParamsUpdater('postMigrationAction')({
        ...params.postMigrationAction,
        suffix: normalized ? normalized : undefined
      })
    }
  }, [
    getParamsUpdater,
    params.postMigrationAction,
    rhfValues.postMigrationActionSuffix,
    selectedMigrationOptions.postMigrationAction?.renameVm
  ])

  useEffect(() => {
    const nextFolderName = String(rhfValues.postMigrationActionFolderName ?? '')
    const normalized = nextFolderName.trim() ? nextFolderName.trim() : ''
    const current = params.postMigrationAction?.folderName ?? ''

    const moveToFolderEnabled = !!selectedMigrationOptions.postMigrationAction?.moveToFolder
    if (!moveToFolderEnabled) return

    if (current !== normalized) {
      getParamsUpdater('postMigrationAction')({
        ...params.postMigrationAction,
        folderName: normalized ? normalized : undefined
      })
    }
  }, [
    getParamsUpdater,
    params.postMigrationAction,
    rhfValues.postMigrationActionFolderName,
    selectedMigrationOptions.postMigrationAction?.moveToFolder
  ])

  useEffect(() => {
    const next = (rhfValues.securityGroups ?? []) as string[]
    if (!stringArrayEqual(params.securityGroups ?? [], next)) {
      getParamsUpdater('securityGroups')(next)
    }
  }, [params.securityGroups, rhfValues.securityGroups, getParamsUpdater])

  useEffect(() => {
    const next = (rhfValues.serverGroup ?? '') as string
    if ((params.serverGroup ?? '') !== next) {
      getParamsUpdater('serverGroup')(next)
    }
  }, [params.serverGroup, rhfValues.serverGroup, getParamsUpdater])
}
