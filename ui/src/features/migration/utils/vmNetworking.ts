/**
 * Minimal structural shape required by the network predicates.
 *
 * Several VM types in the UI (VmData from migration-templates, the local VM
 * type in RollingMigrationForm) share these fields but not their NIC types,
 * so the predicates use a structural shape rather than a concrete import.
 */
interface VmLike {
  networks?: string[]
  networkInterfaces?: unknown[]
}

interface VmIpLike {
  assignedIPs?: string
  ipAddress?: string
  networkInterfaces?: { ipAddress?: string[] | unknown }[]
}

/**
 * A VM "has an interface" when it contributes at least one source network
 * (vm.networks) or has any networkInterfaces entries. NIC-less VMs are
 * legitimate migration targets — they simply don't participate in network
 * mapping or per-VM IP assignment.
 */
export const vmHasInterface = (vm: VmLike): boolean =>
  Boolean(
    (vm.networks && vm.networks.length > 0) ||
    (vm.networkInterfaces && vm.networkInterfaces.length > 0)
  )

/**
 * Returns true when the VM has at least one usable IP address — either an
 * assigned (override) IP, the discovered ipAddress field, or a non-empty
 * IP on one of its NICs.
 *
 * Note: this is intentionally lenient and is meant to gate "does this VM
 * need an IP assigned?" checks; it is NOT a syntactic IP validator.
 */
export const vmHasIp = (vm: VmIpLike): boolean => {
  if (vm.assignedIPs && vm.assignedIPs.trim() !== '') return true
  if (vm.ipAddress && vm.ipAddress !== '—' && vm.ipAddress.trim() !== '') return true
  return Boolean(
    vm.networkInterfaces?.some(
      (nic) =>
        Array.isArray(nic.ipAddress) &&
        (nic.ipAddress as string[]).some((ip) => ip && ip.trim() !== '')
    )
  )
}
