import { CUTOVER_TYPES } from '../constants'
import type { DataCopyMethod } from '../api/migration-blueprints/types'

export const DATA_COPY_METHOD_LABEL: Record<DataCopyMethod, string> = {
  hot: 'Hot copy',
  cold: 'Cold copy',
  mock: 'Mock copy'
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
