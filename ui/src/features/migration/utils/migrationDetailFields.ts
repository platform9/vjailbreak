import { formatDiskSize } from 'src/utils'
import type { VMwareDiskEntry } from 'src/api/vmware-machines/model'
import type { OpenStackFlavor } from 'src/api/openstack-creds/model'

/**
 * Resolves the flavor label shown on the migration details page.
 *
 * Precedence: TARGET_FLAVOR_ID from the migration's ConfigMap (set by the controller for
 * both user-selected and auto-assigned flavors), falling back to the user-selected flavor
 * on the VMwareMachine spec. The ID is mapped to a display name via the OpenstackCreds
 * flavor list. When no flavor is resolvable yet, the flavor will be auto-assigned at
 * migration time.
 */
export function resolveFlavorDisplay({
  configFlavorId,
  selectedFlavorId,
  flavors,
}: {
  configFlavorId?: string
  selectedFlavorId?: string
  flavors?: Pick<OpenStackFlavor, 'id' | 'name'>[]
}): string {
  const resolvedId = configFlavorId || selectedFlavorId || ''
  if (!resolvedId) return 'Auto-assign'
  const resolvedName = (flavors || []).find((f) => f?.id === resolvedId)?.name || resolvedId
  return selectedFlavorId ? resolvedName : `${resolvedName} (auto-assigned)`
}

export interface VmDiskRow {
  name: string
  size: string
  datastore: string
}

/**
 * Normalizes VMwareMachine.spec.vms.disks entries into display rows.
 * Handles both the current object shape ({ name, capacityGB, datastore }) and
 * the legacy plain-string shape from older VMwareMachine CRs.
 */
export function normalizeVmDisks(disks: unknown): VmDiskRow[] {
  if (!Array.isArray(disks)) return []
  return (disks as VMwareDiskEntry[]).map((d, idx) => {
    if (typeof d === 'string') {
      return { name: d || `Disk ${idx + 1}`, size: 'N/A', datastore: 'N/A' }
    }
    return {
      name: d?.name || `Disk ${idx + 1}`,
      size:
        typeof d?.capacityGB === 'number' && d.capacityGB > 0
          ? formatDiskSize(d.capacityGB * 1024 ** 3)
          : 'N/A',
      datastore: d?.datastore || 'N/A',
    }
  })
}
