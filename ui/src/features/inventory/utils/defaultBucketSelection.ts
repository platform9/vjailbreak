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
 *   4. else none
 *
 * Returns ALL eligible VMs across the credential (they may span clusters). At edit time the
 * source cluster is set to the datacenter's "NO CLUSTER" pseudo-cluster, which surfaces every
 * VM in the grid so they can all be selected/migrated together.
 *
 * Pure function.
 */
export function selectDefaultBucketVms(vms: InventoryVm[]): DefaultBucketSelection {
  const poweredOff = vms.filter((vm) => vm.powerState === 'powered-off')
  const poweredOn = vms.filter((vm) => vm.powerState === 'powered-on')

  let tier: DefaultBucketTier
  let picked: InventoryVm[]

  const singleNic = poweredOff.filter((vm) => vm.nicCount === 1)
  if (singleNic.length > 0) {
    tier = 'poweredOffSingleNic'
    picked = singleNic
  } else if (poweredOff.length > 0) {
    const min = minNicCount(poweredOff)
    tier = 'poweredOffFewestNic'
    picked = poweredOff.filter((vm) => vm.nicCount === min)
  } else if (poweredOn.length > 0) {
    const min = minNicCount(poweredOn)
    tier = 'poweredOnFewestNic'
    picked = poweredOn.filter((vm) => vm.nicCount === min)
  } else {
    return { tier: 'none', vmNames: [] }
  }

  return { tier, vmNames: picked.map((vm) => vm.name).sort(byName) }
}
