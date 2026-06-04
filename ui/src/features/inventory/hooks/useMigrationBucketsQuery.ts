import {
  useMutation,
  useQuery,
  useQueryClient,
  UseQueryOptions,
  UseQueryResult
} from '@tanstack/react-query'
import { TEN_SECONDS } from 'src/constants'
import { MIGRATION_BUCKETS_QUERY_KEY } from '../constants'
import { MigrationBucket } from '../api/migration-buckets/model'
import {
  createMigrationBucket,
  deleteMigrationBucket,
  listMigrationBuckets,
  updateMigrationBucket
} from '../api/migration-buckets/migrationBuckets'

export const MIGRATION_BUCKETS_KEY = [MIGRATION_BUCKETS_QUERY_KEY] as const

type QueryOptions = Omit<UseQueryOptions<MigrationBucket[]>, 'queryKey' | 'queryFn'>

/** List MigrationBuckets (source-agnostic: mock or real API). */
export const useMigrationBucketsQuery = (
  options: QueryOptions = {}
): UseQueryResult<MigrationBucket[]> =>
  useQuery<MigrationBucket[]>({
    queryKey: MIGRATION_BUCKETS_KEY,
    queryFn: () => listMigrationBuckets(),
    staleTime: 0,
    // Poll so newly created buckets (default bucket, duplicates, external changes) appear
    // without a manual refresh.
    refetchInterval: TEN_SECONDS,
    refetchOnWindowFocus: false,
    placeholderData: [],
    ...options
  })

/** Create a bucket and refresh the list. */
export const useCreateBucket = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (bucket: MigrationBucket) => createMigrationBucket(bucket),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: MIGRATION_BUCKETS_KEY })
  })
}

/** Update a bucket and refresh the list. */
export const useUpdateBucket = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (bucket: MigrationBucket) => updateMigrationBucket(bucket),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: MIGRATION_BUCKETS_KEY })
  })
}

/** Delete a bucket by name and refresh the list. */
export const useDeleteBucket = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (name: string) => deleteMigrationBucket(name),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: MIGRATION_BUCKETS_KEY })
  })
}
