import type { InventoryVm } from '../types'

/** Which fallback tier produced the default-bucket selection. */
export type DefaultBucketTier =
  | 'poweredOffSingleNic'
  | 'poweredOffFewestNic'
  | 'poweredOnFewestNic'
  | 'none'

export interface DefaultBucketSelection {
  tier: DefaultBucketTier
  vmNames: string[]
}

const byName = (a: string, b: string) => a.localeCompare(b)

const minNicCount = (vms: InventoryVm[]): number =>
  vms.reduce((min, vm) => Math.min(min, vm.nicCount), Number.POSITIVE_INFINITY)

/**
 * Select the VMs for the auto-created default bucket using the resolved fallback chain
 * (spec FR-006):
 *   1. powered-off VMs with a single NIC
 *   2. else powered-off VM(s) with the fewest NICs
 *   3. else powered-on VM(s) with the fewest NICs
 *   4. else none (no default bucket is created)
 *
 * Pure function — mirrors the backend selection (T039) for an FE preview.
 */
export function selectDefaultBucketVms(vms: InventoryVm[]): DefaultBucketSelection {
  const poweredOff = vms.filter((vm) => vm.powerState === 'powered-off')
  const poweredOn = vms.filter((vm) => vm.powerState === 'powered-on')

  // Tier 1: powered-off, single NIC.
  const singleNic = poweredOff.filter((vm) => vm.nicCount === 1)
  if (singleNic.length > 0) {
    return { tier: 'poweredOffSingleNic', vmNames: singleNic.map((vm) => vm.name).sort(byName) }
  }

  // Tier 2: powered-off with the fewest NICs.
  if (poweredOff.length > 0) {
    const min = minNicCount(poweredOff)
    const picked = poweredOff.filter((vm) => vm.nicCount === min)
    return { tier: 'poweredOffFewestNic', vmNames: picked.map((vm) => vm.name).sort(byName) }
  }

  // Tier 3: powered-on with the fewest NICs.
  if (poweredOn.length > 0) {
    const min = minNicCount(poweredOn)
    const picked = poweredOn.filter((vm) => vm.nicCount === min)
    return { tier: 'poweredOnFewestNic', vmNames: picked.map((vm) => vm.name).sort(byName) }
  }

  // Tier 4: nothing eligible.
  return { tier: 'none', vmNames: [] }
}
