import type { VmData } from '../api/migration-templates/model'
import type { KeyValuePair } from '../types'

// Nova rejects metadata keys/values longer than 255 characters
export const METADATA_MAX_LENGTH = 255

/**
 * Convert the key-value editor rows into the MigrationPlan spec's customMetadata map.
 * Blank keys are dropped, keys/values are trimmed, and the last occurrence of a
 * duplicate key wins (matches how the rows read top-to-bottom).
 */
export const customMetadataToRecord = (
  pairs?: KeyValuePair[]
): Record<string, string> | undefined => {
  if (!pairs || pairs.length === 0) return undefined
  const record: Record<string, string> = {}
  pairs.forEach(({ key, value }) => {
    const trimmedKey = key.trim()
    if (!trimmedKey) return
    record[trimmedKey] = value.trim()
  })
  return Object.keys(record).length > 0 ? record : undefined
}

/**
 * Count of tag + custom attribute entries a VM would carry over.
 */
export const countVmSourceEntries = (vm: Pick<VmData, 'tags' | 'customAttributes'>): number =>
  Object.keys(vm.tags || {}).length + Object.keys(vm.customAttributes || {}).length

/**
 * Totals for the preview accordion badge: how many of the selected VMs have
 * source tags/attributes and how many entries exist across all of them.
 */
export const summarizeSourceEntries = (
  vms: Pick<VmData, 'tags' | 'customAttributes'>[]
): { vmCount: number; entryCount: number } => {
  let entryCount = 0
  vms.forEach((vm) => {
    entryCount += countVmSourceEntries(vm)
  })
  return { vmCount: vms.length, entryCount }
}
