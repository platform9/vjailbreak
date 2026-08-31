import type { StorageCopyMethod } from '../types'

// Copy methods that never open VDDK. Mirrors CopyMethodRequiresVDDK in k8s/migration/pkg/utils/migrationutils.go.
export const VDDK_EXEMPT_COPY_METHODS: StorageCopyMethod[] = ['StorageAcceleratedCopy', 'HotAdd']

export const MISSING_VDDK_MESSAGE =
  'The selected data copy method needs the VMware VDDK library. Upload it from Global Settings → VDDK Upload, or switch to a copy method that does not use VDDK.'

// Whether this copy method needs VDDK on the appliance; unset means "normal".
export function copyMethodRequiresVddk(storageCopyMethod?: string): boolean {
  return !VDDK_EXEMPT_COPY_METHODS.includes(storageCopyMethod as StorageCopyMethod)
}

// Blocks only on a confirmed-missing VDDK; `undefined` (loading or endpoint failed) fails open.
export function blocksOnMissingVddk(
  storageCopyMethod: string | undefined,
  vddkUploaded: boolean | undefined
): boolean {
  if (vddkUploaded !== false) return false
  return copyMethodRequiresVddk(storageCopyMethod)
}
