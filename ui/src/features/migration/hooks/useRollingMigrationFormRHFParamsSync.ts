import { useEffect } from 'react'
import type { UseFormReturn } from 'react-hook-form'

type GetParamsUpdater = (key: string) => (value: unknown) => void

export function useRollingMigrationFormRHFParamsSync({
  form,
  params,
  getParamsUpdater,
  selectedMigrationOptions,
  rhfValues
}: {
  form: UseFormReturn<any, any, any>
  params: {
    dataCopyStartTime?: string
    cutoverStartTime?: string
    cutoverEndTime?: string
    postMigrationAction?: {
      suffix?: string
      folderName?: string
    }
  }
  getParamsUpdater: GetParamsUpdater
  selectedMigrationOptions: {
    postMigrationAction?: {
      renameVm?: boolean
      moveToFolder?: boolean
    }
  }
  rhfValues: {
    dataCopyStartTime: string
    cutoverStartTime: string
    cutoverEndTime: string
    postMigrationActionSuffix: string
    postMigrationActionFolderName: string
  }
}) {
  const {
    dataCopyStartTime,
    cutoverStartTime,
    cutoverEndTime,
    postMigrationActionSuffix,
    postMigrationActionFolderName
  } = rhfValues

  useEffect(() => {
    const nextDataCopyStartTime = params.dataCopyStartTime ?? ''
    const nextCutoverStartTime = params.cutoverStartTime ?? ''
    const nextCutoverEndTime = params.cutoverEndTime ?? ''
    const nextPostMigrationActionSuffix = params?.postMigrationAction?.suffix ?? ''
    const nextPostMigrationActionFolderName = params?.postMigrationAction?.folderName ?? ''

    const currentDataCopyStartTime = form.getValues('dataCopyStartTime') ?? ''
    const currentCutoverStartTime = form.getValues('cutoverStartTime') ?? ''
    const currentCutoverEndTime = form.getValues('cutoverEndTime') ?? ''
    const currentPostMigrationActionSuffix = form.getValues('postMigrationActionSuffix') ?? ''
    const currentPostMigrationActionFolderName = form.getValues('postMigrationActionFolderName') ?? ''

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
    params.dataCopyStartTime,
    params.cutoverStartTime,
    params.cutoverEndTime,
    params?.postMigrationAction?.suffix,
    params?.postMigrationAction?.folderName,
    form
  ])

  useEffect(() => {
    const next = String(dataCopyStartTime ?? '')
    const normalized = next.trim() ? next.trim() : ''
    const current = String(params.dataCopyStartTime ?? '')
    if (normalized !== current) {
      getParamsUpdater('dataCopyStartTime')(normalized)
    }
  }, [getParamsUpdater, params.dataCopyStartTime, dataCopyStartTime])

  useEffect(() => {
    const next = String(cutoverStartTime ?? '')
    const normalized = next.trim() ? next.trim() : ''
    const current = String(params.cutoverStartTime ?? '')
    if (normalized !== current) {
      getParamsUpdater('cutoverStartTime')(normalized)
    }
  }, [getParamsUpdater, params.cutoverStartTime, cutoverStartTime])

  useEffect(() => {
    const next = String(cutoverEndTime ?? '')
    const normalized = next.trim() ? next.trim() : ''
    const current = String(params.cutoverEndTime ?? '')
    if (normalized !== current) {
      getParamsUpdater('cutoverEndTime')(normalized)
    }
  }, [getParamsUpdater, params.cutoverEndTime, cutoverEndTime])

  useEffect(() => {
    const nextSuffix = String(postMigrationActionSuffix ?? '')
    const normalized = nextSuffix.trim() ? nextSuffix.trim() : ''
    const current = params?.postMigrationAction?.suffix ?? ''

    if (normalized !== current && selectedMigrationOptions?.postMigrationAction?.renameVm) {
      getParamsUpdater('postMigrationAction')({
        ...params?.postMigrationAction,
        suffix: normalized
      })
    }
  }, [
    getParamsUpdater,
    params?.postMigrationAction,
    params?.postMigrationAction?.suffix,
    postMigrationActionSuffix,
    selectedMigrationOptions?.postMigrationAction?.renameVm
  ])

  useEffect(() => {
    const nextFolderName = String(postMigrationActionFolderName ?? '')
    const normalized = nextFolderName.trim() ? nextFolderName.trim() : ''
    const current = params?.postMigrationAction?.folderName ?? ''

    if (normalized !== current && selectedMigrationOptions?.postMigrationAction?.moveToFolder) {
      getParamsUpdater('postMigrationAction')({
        ...params?.postMigrationAction,
        folderName: normalized
      })
    }
  }, [
    getParamsUpdater,
    params?.postMigrationAction,
    params?.postMigrationAction?.folderName,
    postMigrationActionFolderName,
    selectedMigrationOptions?.postMigrationAction?.moveToFolder
  ])
}
