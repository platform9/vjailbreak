import { useMemo } from 'react'
import { Box } from '@mui/material'
import { Banner } from 'src/components'
import type { BucketStatus, InventoryData, MigrationBucket } from '../types'
import DiscoveryCard from './DiscoveryCard'
import BucketCard from './BucketCard'

export interface BucketListProps {
  data: InventoryData
  /** Live status per bucket name, derived from real migrations (T049). */
  statusByBucket?: Record<string, BucketStatus>
  onEdit?: (bucket: MigrationBucket) => void
  onDuplicate?: (bucket: MigrationBucket) => void
  onDelete?: (bucket: MigrationBucket) => void
}

/** Default bucket first, then alphabetical — deterministic ordering for display. */
const orderBuckets = (buckets: MigrationBucket[]): MigrationBucket[] =>
  [...buckets].sort((a, b) => {
    if (a.spec.isDefault !== b.spec.isDefault) return a.spec.isDefault ? -1 : 1
    return a.metadata.name.localeCompare(b.metadata.name)
  })

/**
 * Discovery summary + the stack of bucket cards. When no buckets exist yet (e.g. no
 * eligible VMs for a default bucket), an informational Banner is shown (US1 scenario 6).
 */
export default function BucketList({
  data,
  statusByBucket,
  onEdit,
  onDuplicate,
  onDelete
}: BucketListProps) {
  const buckets = useMemo(() => orderBuckets(data.buckets), [data.buckets])

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
      <DiscoveryCard
        vmCount={data.vms.length}
        credName={data.credName}
        bucketCount={data.buckets.length}
        bucketedVmCount={Object.keys(data.bucketIdByVm).length}
      />

      {buckets.length === 0 ? (
        <Banner
          variant="info"
          title="No buckets yet"
          message="A default bucket is created automatically once eligible VMs are discovered for this credential."
        />
      ) : (
        buckets.map((bucket) => (
          <BucketCard
            key={bucket.metadata.name}
            bucket={bucket}
            statusOverride={statusByBucket?.[bucket.metadata.name]}
            onEdit={onEdit}
            onDuplicate={onDuplicate}
            onDelete={onDelete}
          />
        ))
      )}
    </Box>
  )
}
