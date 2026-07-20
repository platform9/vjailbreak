import { alpha, type Theme } from '@mui/material/styles'
import { CUTOVER_TYPES, OS_TYPES_OPTIONS, STORAGE_COPY_METHOD_OPTIONS } from '../constants'
import type { DataCopyMethod } from '../api/migration-blueprints/types'
import type { MigrationBlueprintSpec } from 'src/api/migration-blueprints/model'

export const DATA_COPY_METHOD_LABEL: Record<DataCopyMethod, string> = {
  hot: 'Hot copy',
  cold: 'Cold copy',
  mock: 'Mock copy'
}

// Palette key per copy method — mirrors the hot/cold/mock dot colors used on the
// Migrations table's Name column, so the two views read consistently.
export const DATA_COPY_METHOD_CHIP_COLOR: Record<DataCopyMethod, 'warning' | 'info' | 'error'> = {
  hot: 'warning',
  cold: 'info',
  mock: 'error'
}

// Soft pastel chip look (tinted background, saturated text, no border) for the
// copy-method chip — matches the design's "Hot copy"/"Cold copy" pill styling,
// as opposed to MUI's solid-filled default Chip colors used elsewhere.
export function dataCopyMethodChipSx(method: DataCopyMethod) {
  const colorKey = DATA_COPY_METHOD_CHIP_COLOR[method]
  return (theme: Theme) => ({
    bgcolor: alpha(theme.palette[colorKey].main, theme.palette.mode === 'dark' ? 0.24 : 0.14),
    color: theme.palette.mode === 'dark' ? theme.palette[colorKey].light : theme.palette[colorKey].dark,
    fontWeight: 600,
    border: 'none'
  })
}

export const CUTOVER_OPTION_LABEL: Record<string, string> = {
  [CUTOVER_TYPES.IMMEDIATE]: 'Immediate cutover',
  [CUTOVER_TYPES.ADMIN_INITIATED]: 'Admin cutover',
  [CUTOVER_TYPES.TIME_WINDOW]: 'Time window cutover'
}

export function cutoverOptionLabel(cutoverOption: string | undefined): string {
  if (!cutoverOption) return CUTOVER_OPTION_LABEL[CUTOVER_TYPES.IMMEDIATE]
  return CUTOVER_OPTION_LABEL[cutoverOption] ?? 'Immediate cutover'
}

export function guestOsLabel(osFamily: string | undefined): string {
  const option = OS_TYPES_OPTIONS.find((opt) => opt.value === osFamily)
  return option?.label ?? 'Auto-detect'
}

export function storageCopyMethodLabel(storageCopyMethod: string | undefined): string | undefined {
  return STORAGE_COPY_METHOD_OPTIONS.find((opt) => opt.value === storageCopyMethod)?.label
}

// Summarizes the "Advanced" options row on the template detail drawer from whichever
// advanced flags are actually set on the blueprint spec — no fabricated data.
export function deriveAdvancedOptionsSummary(spec: MigrationBlueprintSpec | undefined): string {
  if (!spec) return 'None'

  const flags: Array<[boolean | undefined, string]> = [
    [Boolean(spec.firstBootScript), 'Post-migration script'],
    [spec.postMigrationAction?.renameVm, 'Rename VM'],
    [spec.postMigrationAction?.moveToFolder, 'Move to folder'],
    [spec.advancedOptions?.removeVMwareTools, 'Remove VMware Tools'],
    [spec.advancedOptions?.periodicSyncEnabled, 'Periodic sync'],
    [spec.migrationStrategy?.performHealthChecks, 'Health checks'],
    [spec.migrationStrategy?.arrayOffload, 'Array offload'],
    [spec.migrationStrategy?.disconnectSourceNetwork, 'Disconnect source network']
  ]

  const active = flags.filter(([enabled]) => enabled).map(([, label]) => label)
  return active.length > 0 ? active.join(' · ') : 'None'
}
