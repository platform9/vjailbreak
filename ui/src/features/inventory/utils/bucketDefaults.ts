import type { OpenstackCreds } from 'src/api/openstack-creds/model'
import type { SourceDataItem } from 'src/features/migration/hooks/useClusterData'
import type { BucketMapping, MigrationBucketConfig } from '../api/migration-buckets/model'
import type { InventoryVm } from '../types'

/**
 * Find the "NO CLUSTER" source-cluster dropdown id (a VMwareCluster whose name starts with
 * `no-cluster-`). Selecting it surfaces every VM in the datacenter, so a cross-cluster bucket
 * can have all its VMs shown/selected at once. Returns the first one found (single-DC case).
 */
export const findNoClusterSourceClusterId = (
  sourceData: SourceDataItem[]
): string | undefined => {
  for (const dc of sourceData) {
    const noCluster = dc.clusters.find(
      (c) =>
        c.name.toLowerCase().startsWith('no-cluster-') ||
        c.displayName?.toUpperCase() === 'NO CLUSTER'
    )
    if (noCluster) return noCluster.id
  }
  return undefined
}

const distinct = (values: (string | undefined)[]): string[] =>
  Array.from(new Set(values.filter((v): v is string => Boolean(v))))

/**
 * Build the auto-default migration configuration for a bucket (FR-014/FR-015):
 *  - source cluster   : derived from the bucket's VMs
 *  - destination (PCD) : first entry in OpenstackCreds.spec.pcdHostConfig
 *  - network mapping   : every distinct source network → first destination network
 *  - storage mapping   : every distinct source datastore → first destination volume type
 *  - security groups / server group / advanced options : unselected
 *
 * Mapping every source to the first target (rather than only the first source) avoids leaving
 * any source network/datastore unmapped while still honoring "first destination" as the default.
 *
 * Pure function — safe to unit test and to call when opening the editor for a new bucket.
 */
export function buildBucketConfigDefaults(
  bucketVms: InventoryVm[],
  openstackCreds?: OpenstackCreds
): MigrationBucketConfig {
  // NOTE: sourceCluster (VMware) and pcdCluster (destination) are intentionally NOT set here.
  // At creation time the cluster lists aren't loaded and the right identifiers aren't known
  // (the destination dropdown uses PCDCluster CRs, not `pcdHostConfig`). They are resolved from
  // live data when the bucket is opened in the editor (MigrationConfigForm autoDefaults).
  const firstNetwork = openstackCreds?.status?.openstack?.networks?.[0]?.name
  const firstVolumeType = openstackCreds?.status?.openstack?.volumeTypes?.[0]

  const sourceNetworks = distinct(bucketVms.flatMap((vm) => vm.networks))
  const sourceDatastores = distinct(bucketVms.flatMap((vm) => vm.datastores))

  const networkMappings: BucketMapping[] = firstNetwork
    ? sourceNetworks.map((source) => ({ source, target: firstNetwork }))
    : []
  const storageMappings: BucketMapping[] = firstVolumeType
    ? sourceDatastores.map((source) => ({ source, target: firstVolumeType }))
    : []

  return {
    networkMappings,
    storageMappings,
    securityGroups: [],
    serverGroup: undefined,
    advancedOptions: {}
  }
}
