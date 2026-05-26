import { useEffect } from 'react'
import { useWatch, UseFormReturn } from 'react-hook-form'
import type { RollingMigrationRHFValues, RollingFormParams, SelectedMigrationOptionsType } from '../types'

const stringArrayEqual = (a: string[] | undefined, b: string[] | undefined): boolean => {
  if (a === b) return true
  if (!a || !b) return false
  if (a.length !== b.length) return false
  return a.every((v, i) => v === b[i])
}

interface UseRollingFormSyncParams {
  form: UseFormReturn<RollingMigrationRHFValues, any, RollingMigrationRHFValues>
  params: RollingFormParams
  getParamsUpdater: (key: keyof RollingFormParams) => (value: any) => void
  selectedMigrationOptions: SelectedMigrationOptionsType
}

export function useRollingFormSync({
  form,
  params,
  getParamsUpdater,
  selectedMigrationOptions
}: UseRollingFormSyncParams): void {
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

  // params → RHF
  useEffect(() => {
    const nextSecurityGroups = params.securityGroups ?? []
    const nextServerGroup = params.serverGroup ?? ''
    const nextDataCopyStartTime = params.dataCopyStartTime ?? ''
    const nextCutoverStartTime = params.cutoverStartTime ?? ''
    const nextCutoverEndTime = params.cutoverEndTime ?? ''
    const nextPostMigrationActionSuffix = (params as any)?.postMigrationAction?.suffix ?? ''
    const nextPostMigrationActionFolderName = (params as any)?.postMigrationAction?.folderName ?? ''

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
    (params as any)?.postMigrationAction,
  ])

  // RHF → params
  useEffect(() => {
    const next = String(rhfDataCopyStartTime ?? '')
    const normalized = next.trim() ? next.trim() : ''
    const current = String(params.dataCopyStartTime ?? '')
    if (normalized !== current) {
      getParamsUpdater('dataCopyStartTime')(normalized)
    }
  }, [getParamsUpdater, params.dataCopyStartTime, rhfDataCopyStartTime])

  useEffect(() => {
    const next = String(rhfCutoverStartTime ?? '')
    const normalized = next.trim() ? next.trim() : ''
    const current = String(params.cutoverStartTime ?? '')
    if (normalized !== current) {
      getParamsUpdater('cutoverStartTime')(normalized)
    }
  }, [getParamsUpdater, params.cutoverStartTime, rhfCutoverStartTime])

  useEffect(() => {
    const next = String(rhfCutoverEndTime ?? '')
    const normalized = next.trim() ? next.trim() : ''
    const current = String(params.cutoverEndTime ?? '')
    if (normalized !== current) {
      getParamsUpdater('cutoverEndTime')(normalized)
    }
  }, [getParamsUpdater, params.cutoverEndTime, rhfCutoverEndTime])

  useEffect(() => {
    const nextSuffix = String(rhfPostMigrationActionSuffix ?? '')
    const normalized = nextSuffix.trim() ? nextSuffix.trim() : ''
    const current = (params as any)?.postMigrationAction?.suffix ?? ''

    if (normalized !== current && selectedMigrationOptions?.postMigrationAction?.renameVm) {
      getParamsUpdater('postMigrationAction')({
        ...(params as any)?.postMigrationAction,
        suffix: normalized
      })
    }
  }, [
    getParamsUpdater,
    (params as any)?.postMigrationAction,
    rhfPostMigrationActionSuffix,
    selectedMigrationOptions?.postMigrationAction?.renameVm
  ])

  useEffect(() => {
    const nextFolderName = String(rhfPostMigrationActionFolderName ?? '')
    const normalized = nextFolderName.trim() ? nextFolderName.trim() : ''
    const current = (params as any)?.postMigrationAction?.folderName ?? ''

    if (normalized !== current && selectedMigrationOptions?.postMigrationAction?.moveToFolder) {
      getParamsUpdater('postMigrationAction')({
        ...(params as any)?.postMigrationAction,
        folderName: normalized
      })
    }
  }, [
    getParamsUpdater,
    (params as any)?.postMigrationAction,
    rhfPostMigrationActionFolderName,
    selectedMigrationOptions?.postMigrationAction?.moveToFolder
  ])

  useEffect(() => {
    const next = (rhfSecurityGroups ?? []) as string[]
    if (!stringArrayEqual(params.securityGroups ?? [], next)) {
      getParamsUpdater('securityGroups')(next)
    }
  }, [params.securityGroups, rhfSecurityGroups, getParamsUpdater])

  useEffect(() => {
    const next = (rhfServerGroup ?? '') as string
    if ((params.serverGroup ?? '') !== next) {
      getParamsUpdater('serverGroup')(next)
    }
  }, [params.serverGroup, rhfServerGroup, getParamsUpdater])
}
