/**
 * Returns true when any source network has a subnet-compatibility warning,
 * i.e. at least one selected VM IP does not lie within the subnet of its
 * mapped destination network. Used to block the "Persist source network
 * interfaces" option, which would pin IPs that are invalid on the target.
 */
export function hasAnySubnetMismatch(subnetWarnings: Record<string, string> | undefined): boolean {
  if (!subnetWarnings) return false
  return Object.values(subnetWarnings).some((warning) => Boolean(warning && warning.trim()))
}

interface PreserveIpVm {
  preserveIp?: Record<number, boolean>
  networkInterfaces?: Array<{ preserveIP?: boolean }>
}

/**
 * Returns true when any selected VM has explicitly disabled "Preserve IP" on
 * one of its interfaces (e.g. via the IP edit dialog's "Preserve IP" toggle
 * or "Clear All"). With no IP preserved for that interface, the subnet
 * compatibility check has nothing to validate against and silently no-ops —
 * so this is checked independently to keep "Persist source network
 * interfaces" blocked in that case. Mirrors the vm.preserveIp-overrides-nic
 * precedence used in StandardIpAddressCell.
 */
export function hasAnyPreserveIpDisabled(vms: PreserveIpVm[] | undefined): boolean {
  if (!vms) return false
  return vms.some((vm) => {
    const overrides = vm.preserveIp
    if (vm.networkInterfaces && vm.networkInterfaces.length > 0) {
      return vm.networkInterfaces.some((nic, index) =>
        overrides?.[index] !== undefined ? overrides[index] === false : nic.preserveIP === false
      )
    }
    if (overrides) {
      return Object.values(overrides).some((flag) => flag === false)
    }
    return false
  })
}
