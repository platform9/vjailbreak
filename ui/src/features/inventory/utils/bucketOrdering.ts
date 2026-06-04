import type { InventoryVm, MigrationBucket } from '../types'

export interface BucketScore {
  /** Success score f = (PO+1)/(PON+1) — higher = more likely to succeed. */
  f: number
  /** Most frequent NIC count among the bucket's VMs (lower = simpler). */
  modeNic: number
  /** Total disk count — FE proxy for size (no capacityGB in the UI model). */
  size: number
  poweredOff: number
  poweredOn: number
}

/**
 * Score a bucket from its VMs (DESIGN §9.2). The mode of NIC counts is found via a
 * counting/radix tally over the small bounded NIC integer — O(V).
 */
export function scoreBucket(
  bucket: MigrationBucket,
  vmByName: Record<string, InventoryVm>
): BucketScore {
  let poweredOff = 0
  let poweredOn = 0
  let size = 0
  const nicTally: number[] = []

  for (const name of bucket.spec.vms) {
    const vm = vmByName[name]
    if (!vm) continue
    if (vm.powerState === 'powered-off') poweredOff += 1
    else if (vm.powerState === 'powered-on') poweredOn += 1
    size += vm.diskCount
    nicTally[vm.nicCount] = (nicTally[vm.nicCount] ?? 0) + 1
  }

  // Mode of NIC counts (index with the highest tally); 0 when no VMs are mapped.
  let modeNic = 0
  let best = -1
  for (let nic = 0; nic < nicTally.length; nic++) {
    const count = nicTally[nic] ?? 0
    if (count > best) {
      best = count
      modeNic = nic
    }
  }

  return { f: (poweredOff + 1) / (poweredOn + 1), modeNic, size, poweredOff, poweredOn }
}

/**
 * Order buckets success-first (DESIGN §9.2), as a stable multi-key sort:
 *   1. f descending (more powered-off relative to powered-on)
 *   2. modeNic ascending (fewer NICs = simpler)
 *   3. size ascending (smaller = quick win)
 *   4. name ascending (deterministic tie-break)
 *
 * Pure. The auto-created default bucket (all powered-off, single-NIC) naturally leads.
 */
export function orderBucketsBySuccess(
  buckets: MigrationBucket[],
  vmByName: Record<string, InventoryVm>
): MigrationBucket[] {
  return [...buckets].sort((a, b) => {
    const sa = scoreBucket(a, vmByName)
    const sb = scoreBucket(b, vmByName)
    if (sb.f !== sa.f) return sb.f - sa.f
    if (sa.modeNic !== sb.modeNic) return sa.modeNic - sb.modeNic
    if (sa.size !== sb.size) return sa.size - sb.size
    return a.metadata.name.localeCompare(b.metadata.name)
  })
}
