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
