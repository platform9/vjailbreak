import type { UseFormReturn } from 'react-hook-form'
import { useMigrationFormRHFParamsSync } from 'src/features/migration/hooks/useMigrationFormRHFParamsSync'

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
  getParamsUpdater: (key: string) => (value: unknown) => void
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
  useMigrationFormRHFParamsSync({
    form: form as any,
    params: params as any,
    getParamsUpdater,
    selectedMigrationOptions: selectedMigrationOptions as any,
    rhfValues,
    enableSecurityPlacementSync: false
  })
}
