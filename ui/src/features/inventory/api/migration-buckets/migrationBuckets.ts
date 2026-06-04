// MigrationBucket data layer.
//
// All access goes through this module behind a single BUCKETS_DATA_SOURCE switch so the
// rest of the UI is agnostic to whether buckets come from an in-memory mock (Phase 2–7,
// for building/Storybook/e2e) or the real k8s API (flipped in T047, Phase 10).

import axios from 'src/api/axios'
import { VJAILBREAK_API_BASE_PATH, VJAILBREAK_DEFAULT_NAMESPACE } from 'src/api/constants'
import {
  MigrationBucket,
  MigrationBucketList,
  MIGRATION_BUCKET_API_VERSION,
  MIGRATION_BUCKET_KIND
} from './model'

/**
 * Data source for buckets.
 * - 'api'  → real MigrationBucket CRs (requires the CRD deployed + RBAC; see Phase 8/10).
 * - 'mock' → in-memory store (for Storybook / offline UI work).
 * NOTE: in 'api' mode the Inventory page errors until the MigrationBucket CRD is applied to the
 * cluster (run `make generate && make manifests` in k8s/migration, then redeploy/apply CRDs).
 */
export const BUCKETS_DATA_SOURCE: 'mock' | 'api' = 'api'

/** Sanitize a user-provided bucket name to a valid k8s (DNS-1123) resource name. */
export const sanitizeBucketName = (name: string): string => {
  const cleaned = name
    .toLowerCase()
    .replace(/[^a-z0-9-]+/g, '-')
    .replace(/-+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 200)
  return cleaned || `bucket-${Date.now()}`
}

const MIGRATION_BUCKETS_RESOURCE = 'migrationbuckets'

// ---------------------------------------------------------------------------
// Real k8s API implementation (mirrors src/api/proxyvms/proxyVMs.ts)
// ---------------------------------------------------------------------------

const resourcePath = (namespace: string) =>
  `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/${MIGRATION_BUCKETS_RESOURCE}`

const apiList = async (namespace = VJAILBREAK_DEFAULT_NAMESPACE): Promise<MigrationBucket[]> => {
  const response = await axios.get<MigrationBucketList>({ endpoint: resourcePath(namespace) })
  return response?.items ?? []
}

const apiCreate = async (
  body: MigrationBucket,
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
): Promise<MigrationBucket> =>
  axios.post<MigrationBucket>({ endpoint: resourcePath(namespace), data: body })

const apiUpdate = async (
  body: MigrationBucket,
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
): Promise<MigrationBucket> =>
  axios.put<MigrationBucket>({
    endpoint: `${resourcePath(namespace)}/${body.metadata.name}`,
    data: body
  })

const apiDelete = async (name: string, namespace = VJAILBREAK_DEFAULT_NAMESPACE): Promise<void> => {
  await axios.del<MigrationBucket>({ endpoint: `${resourcePath(namespace)}/${name}` })
}

// ---------------------------------------------------------------------------
// In-memory mock implementation
// ---------------------------------------------------------------------------

const clone = <T>(value: T): T => JSON.parse(JSON.stringify(value))

const makeBucket = (
  name: string,
  spec: MigrationBucket['spec'],
  phase: NonNullable<MigrationBucket['status']>['phase'] = 'NotMigrated'
): MigrationBucket => ({
  apiVersion: MIGRATION_BUCKET_API_VERSION,
  kind: MIGRATION_BUCKET_KIND,
  metadata: {
    name,
    namespace: VJAILBREAK_DEFAULT_NAMESPACE,
    creationTimestamp: new Date().toISOString()
  },
  spec,
  status: { phase }
})

// Starts empty: the default bucket is auto-created from real discovered VMs (the backend
// reconciler will own this in production; in mock mode the page seeds it once — FR-005/006).
const mockStore: MigrationBucket[] = []

const mockList = async (): Promise<MigrationBucket[]> => clone(mockStore)

const mockCreate = async (body: MigrationBucket): Promise<MigrationBucket> => {
  if (mockStore.some((b) => b.metadata.name === body.metadata.name)) {
    throw new Error(`Bucket "${body.metadata.name}" already exists`)
  }
  const created = makeBucket(body.metadata.name, body.spec, body.status?.phase ?? 'NotMigrated')
  mockStore.push(created)
  return clone(created)
}

const mockUpdate = async (body: MigrationBucket): Promise<MigrationBucket> => {
  const idx = mockStore.findIndex((b) => b.metadata.name === body.metadata.name)
  if (idx === -1) throw new Error(`Bucket "${body.metadata.name}" not found`)
  mockStore[idx] = clone(body)
  return clone(mockStore[idx])
}

const mockDelete = async (name: string): Promise<void> => {
  const idx = mockStore.findIndex((b) => b.metadata.name === name)
  if (idx === -1) return
  if (mockStore[idx].spec.isDefault) throw new Error('The default bucket cannot be deleted')
  mockStore.splice(idx, 1)
}

// ---------------------------------------------------------------------------
// Public API (source-agnostic)
// ---------------------------------------------------------------------------

const isMock = (BUCKETS_DATA_SOURCE as string) === 'mock'

export const listMigrationBuckets = (namespace?: string): Promise<MigrationBucket[]> =>
  isMock ? mockList() : apiList(namespace)

export const createMigrationBucket = (
  body: MigrationBucket,
  namespace?: string
): Promise<MigrationBucket> => (isMock ? mockCreate(body) : apiCreate(body, namespace))

export const updateMigrationBucket = (
  body: MigrationBucket,
  namespace?: string
): Promise<MigrationBucket> => (isMock ? mockUpdate(body) : apiUpdate(body, namespace))

export const deleteMigrationBucket = (name: string, namespace?: string): Promise<void> =>
  isMock ? mockDelete(name) : apiDelete(name, namespace)

/** Helper to build a new bucket object from a spec (used by create flows). */
export const buildMigrationBucket = (
  name: string,
  spec: MigrationBucket['spec']
): MigrationBucket => makeBucket(sanitizeBucketName(name), spec)
