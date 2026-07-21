import { useEffect, useRef } from 'react'
import { useWatch, UseFormReturn } from 'react-hook-form'
import type { FormValues, SelectedMigrationOptionsType, MigrationDrawerRHFValues } from '../types'

const stringArrayEqual = (a: string[] | undefined, b: string[] | undefined): boolean => {
  if (a === b) return true
  if (!a || !b) return false
  if (a.length !== b.length) return false
  return a.every((v, i) => v === b[i])
}

interface UseFormSyncParams {
  form: UseFormReturn<MigrationDrawerRHFValues, any, MigrationDrawerRHFValues>
  params: Partial<FormValues>
  getParamsUpdater: <K extends keyof FormValues>(key: K) => (value: FormValues[K]) => void
  selectedMigrationOptions: SelectedMigrationOptionsType
}

export function useFormSync({
  form,
  params,
  getParamsUpdater,
  selectedMigrationOptions
}: UseFormSyncParams): void {
  const rhfSecurityGroups = useWatch({ control: form.control, name: 'securityGroups' })
  const rhfServerGroup = useWatch({ control: form.control, name: 'serverGroup' })
  const rhfDataCopyStartTime = useWatch({ control: form.control, name: 'dataCopyStartTime' })
  const rhfCutoverStartTime = useWatch({ control: form.control, name: 'cutoverStartTime' })
  const rhfCutoverEndTime = useWatch({ control: form.control, name: 'cutoverEndTime' })
  const rhfPostMigrationActionSuffix = useWatch({
    control: form.control,
    name: 'postMigrationActionSuffix'
  })
  const rhfPostMigrationActionFolderName = useWatch({
    control: form.control,
    name: 'postMigrationActionFolderName'
  })

  const suppressEchoRef = useRef(false)

  // params → RHF
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

    let changed = false

    if (!stringArrayEqual(currentSecurityGroups, nextSecurityGroups)) {
      form.setValue('securityGroups', nextSecurityGroups)
      changed = true
    }
    if (currentServerGroup !== nextServerGroup) {
      form.setValue('serverGroup', nextServerGroup)
      changed = true
    }

    if (currentDataCopyStartTime !== nextDataCopyStartTime) {
      form.setValue('dataCopyStartTime', nextDataCopyStartTime)
      changed = true
    }
    if (currentCutoverStartTime !== nextCutoverStartTime) {
      form.setValue('cutoverStartTime', nextCutoverStartTime)
      changed = true
    }
    if (currentCutoverEndTime !== nextCutoverEndTime) {
      form.setValue('cutoverEndTime', nextCutoverEndTime)
      changed = true
    }
    if (currentPostMigrationActionSuffix !== nextPostMigrationActionSuffix) {
      form.setValue('postMigrationActionSuffix', nextPostMigrationActionSuffix)
      changed = true
    }
    if (currentPostMigrationActionFolderName !== nextPostMigrationActionFolderName) {
      form.setValue('postMigrationActionFolderName', nextPostMigrationActionFolderName)
      changed = true
    }

    if (changed) suppressEchoRef.current = true
  }, [
    form,
    params.securityGroups,
    params.serverGroup,
    params.dataCopyStartTime,
    params.cutoverStartTime,
    params.cutoverEndTime,
    params.postMigrationAction
  ])

  // RHF → params
  useEffect(() => {
    if (suppressEchoRef.current) return
    const nextDataCopyStartTime = (rhfDataCopyStartTime ?? '') as string
    if ((params.dataCopyStartTime ?? '') !== nextDataCopyStartTime) {
      getParamsUpdater('dataCopyStartTime')(nextDataCopyStartTime)
    }
  }, [getParamsUpdater, params.dataCopyStartTime, rhfDataCopyStartTime])

  useEffect(() => {
    if (suppressEchoRef.current) return
    const nextCutoverStartTime = (rhfCutoverStartTime ?? '') as string
    if ((params.cutoverStartTime ?? '') !== nextCutoverStartTime) {
      getParamsUpdater('cutoverStartTime')(nextCutoverStartTime)
    }
  }, [getParamsUpdater, params.cutoverStartTime, rhfCutoverStartTime])

  useEffect(() => {
    if (suppressEchoRef.current) return
    const nextCutoverEndTime = (rhfCutoverEndTime ?? '') as string
    if ((params.cutoverEndTime ?? '') !== nextCutoverEndTime) {
      getParamsUpdater('cutoverEndTime')(nextCutoverEndTime)
    }
  }, [getParamsUpdater, params.cutoverEndTime, rhfCutoverEndTime])

  useEffect(() => {
    if (suppressEchoRef.current) return
    const nextSuffix = String(rhfPostMigrationActionSuffix ?? '')
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
    rhfPostMigrationActionSuffix,
    selectedMigrationOptions.postMigrationAction?.renameVm
  ])

  useEffect(() => {
    if (suppressEchoRef.current) return
    const nextFolderName = String(rhfPostMigrationActionFolderName ?? '')
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
    rhfPostMigrationActionFolderName,
    selectedMigrationOptions.postMigrationAction?.moveToFolder
  ])

  useEffect(() => {
    if (suppressEchoRef.current) return
    const next = (rhfSecurityGroups ?? []) as string[]
    if (!stringArrayEqual(params.securityGroups ?? [], next)) {
      getParamsUpdater('securityGroups')(next)
    }
  }, [params.securityGroups, rhfSecurityGroups, getParamsUpdater])

  useEffect(() => {
    if (suppressEchoRef.current) return
    const next = (rhfServerGroup ?? '') as string
    if ((params.serverGroup ?? '') !== next) {
      getParamsUpdater('serverGroup')(next)
    }
  }, [params.serverGroup, rhfServerGroup, getParamsUpdater])

  useEffect(() => {
    suppressEchoRef.current = false
  })
}
