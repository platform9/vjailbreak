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

// ─── Destination tenant ───────────────────────────────────────────────────────

const OPENSTACKCREDS_LABEL = 'vjailbreak.k8s.pf9.io/openstackcreds'

interface OpenstackCredsLike {
  metadata?: { name?: string }
  spec?: { projectName?: string; pcdHostConfig?: Array<{ clusterName?: string }> }
}

interface PcdClusterLike {
  metadata?: { labels?: Record<string, string> }
  spec?: { clusterName?: string }
}

/**
 * Build a resolver for a migration's destination tenant (the OpenStack project name).
 *
 * The credential reference is authoritative: it is stored on the MigrationTemplate as
 * `destination.openstackRef`, so it identifies the tenant exactly. Deriving the tenant from
 * the cluster *name* instead is ambiguous — two credentials can expose a cluster with the
 * same name, and a name-keyed map silently keeps whichever was iterated last, which is how
 * the migrations table came to show one credential beside another credential's tenant.
 *
 * Resolution order:
 *   1. `openstackRef` → that credential's projectName
 *   2. cluster name → owning credential (label) → projectName
 *   3. cluster name → projectName via the credential's pcdHostConfig
 *   4. 'N/A'
 *
 * Steps 2 and 3 are kept only as fallbacks, for migrations whose template predates the ref
 * or whose credential has no projectName yet.
 */
export function buildDestinationTenantResolver(
  openstackCredsList: OpenstackCredsLike[] | null | undefined,
  pcdClusters: PcdClusterLike[] | null | undefined
): (openstackRef?: string, clusterName?: string) => string {
  const creds = openstackCredsList || []

  const projectNameByCred = new Map<string, string>()
  for (const cred of creds) {
    const name = String(cred?.metadata?.name || '').trim()
    const projectName = String(cred?.spec?.projectName || '').trim()
    if (name && projectName) projectNameByCred.set(name, projectName)
  }

  const credByClusterName = new Map<string, string>()
  for (const cluster of pcdClusters || []) {
    const clusterName = String(cluster?.spec?.clusterName || '').trim()
    const credName = String(cluster?.metadata?.labels?.[OPENSTACKCREDS_LABEL] || '').trim()
    if (clusterName && credName && !credByClusterName.has(clusterName)) {
      credByClusterName.set(clusterName, credName)
    }
  }

  const projectNameByHostConfigCluster = new Map<string, string>()
  for (const cred of creds) {
    const projectName = String(cred?.spec?.projectName || '').trim()
    if (!projectName) continue
    for (const cfg of cred?.spec?.pcdHostConfig || []) {
      const clusterName = String(cfg?.clusterName || '').trim()
      if (clusterName && !projectNameByHostConfigCluster.has(clusterName)) {
        projectNameByHostConfigCluster.set(clusterName, projectName)
      }
    }
  }

  return (openstackRef?: string, clusterName?: string): string => {
    const ref = String(openstackRef || '').trim()
    if (ref && ref !== 'N/A') {
      const fromRef = projectNameByCred.get(ref)
      if (fromRef) return fromRef
    }

    const cluster = String(clusterName || '').trim()
    if (!cluster || cluster === 'N/A') return 'N/A'

    const credName = credByClusterName.get(cluster)
    if (credName) {
      const fromCluster = projectNameByCred.get(credName)
      if (fromCluster) return fromCluster
    }

    return projectNameByHostConfigCluster.get(cluster) || 'N/A'
  }
}
