// Shape of the entries `useClusterData` publishes as `pcdData`, narrowed to the fields
// needed to identify a cluster. Kept structural so pure callers don't have to import the
// hook module.
export interface PcdClusterOption {
  id: string
  name?: string
  openstackCredName?: string
}

/**
 * Resolve a PCD cluster by name *within the credential that owns it*.
 *
 * PCD cluster names are unique per credential, not globally — two OpenstackCreds can each
 * expose a cluster called "cluster-1". Matching on name alone returns whichever happens to
 * come first, so a retried migration or an applied template can silently land on another
 * tenant's cluster.
 *
 * Falls back to a name-only match when the credential is unknown or has no cluster by that
 * name, so migrations whose credential was renamed or whose PCDCluster is missing the
 * `vjailbreak.k8s.pf9.io/openstackcreds` label still resolve to something selectable
 * instead of leaving the dropdown empty.
 */
export function findPcdClusterByName<T extends PcdClusterOption>(
  pcdData: T[],
  clusterName: string | undefined,
  openstackCredName: string | undefined
): T | undefined {
  if (!clusterName) return undefined

  if (openstackCredName) {
    const scoped = pcdData.find(
      (cluster) => cluster.name === clusterName && cluster.openstackCredName === openstackCredName
    )
    if (scoped) return scoped
  }

  return pcdData.find((cluster) => cluster.name === clusterName)
}
