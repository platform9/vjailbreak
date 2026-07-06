import { useQuery, UseQueryOptions } from '@tanstack/react-query'
import { getClusterConversionBatches } from 'src/api/cluster-conversion-batches/clusterConversionBatches'
import { ClusterConversionBatch } from 'src/api/cluster-conversion-batches/model'

export const CLUSTER_CONVERSION_BATCHES_QUERY_KEY = ['cluster-conversion-batches']

export const useClusterConversionBatchesQuery = (
  options?: Omit<UseQueryOptions<ClusterConversionBatch[], Error>, 'queryKey' | 'queryFn'>
) => {
  return useQuery<ClusterConversionBatch[], Error>({
    queryKey: CLUSTER_CONVERSION_BATCHES_QUERY_KEY,
    queryFn: () => getClusterConversionBatches(),
    ...options
  })
}
