import type { BucketIdByVm } from '../types'

/** Is this VM already assigned to a bucket? */
export const isVmBucketed = (vmName: string, bucketIdByVm: BucketIdByVm): boolean =>
  Boolean(bucketIdByVm[vmName])

/**
 * Is this VM unavailable for `targetBucketName`? A VM is blocked if it belongs to a
 * *different* bucket. (VMs in the target bucket itself remain selectable when editing.)
 */
export const isVmBlocked = (
  vmName: string,
  bucketIdByVm: BucketIdByVm,
  targetBucketName?: string
): boolean => {
  const owner = bucketIdByVm[vmName]
  return Boolean(owner) && owner !== targetBucketName
}

/**
 * Enforce VM uniqueness across buckets (FR-013). Throws if any selected VM already
 * belongs to a different bucket. Used by both the duplicate and edit flows so the rule
 * holds regardless of entry point.
 */
export const assertVmsUnique = (
  vmNames: string[],
  bucketIdByVm: BucketIdByVm,
  targetBucketName?: string
): void => {
  const conflicts = vmNames.filter((name) => isVmBlocked(name, bucketIdByVm, targetBucketName))
  if (conflicts.length > 0) {
    throw new Error(`These VMs already belong to another bucket: ${conflicts.join(', ')}`)
  }
}

/** Validate a bucket's VM set: must be non-empty (FR-012) and unique (FR-013). */
export const validateBucketVms = (
  vmNames: string[],
  bucketIdByVm: BucketIdByVm,
  targetBucketName?: string
): string | null => {
  if (vmNames.length === 0) return 'A bucket must contain at least one VM.'
  const conflicts = vmNames.filter((name) => isVmBlocked(name, bucketIdByVm, targetBucketName))
  if (conflicts.length > 0) {
    return `These VMs already belong to another bucket: ${conflicts.join(', ')}`
  }
  return null
}
