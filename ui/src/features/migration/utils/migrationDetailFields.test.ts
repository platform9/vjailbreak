import { beforeEach, describe, expect, it, vi } from 'vitest'
import axios from 'src/api/axios'
import { getMigrationConfigMap } from 'src/api/migrations/migrations'
import { normalizeVmDisks, resolveFlavorDisplay } from './migrationDetailFields'

vi.mock('src/api/axios', () => ({
  default: { get: vi.fn() }
}))

const flavors = [
  { id: 'flavor-1', name: 'm1.small' },
  { id: 'flavor-2', name: 'm1.large' },
]

describe('resolveFlavorDisplay', () => {
  it('shows Auto-assign when no flavor is selected or resolved yet', () => {
    expect(resolveFlavorDisplay({ flavors })).toBe('Auto-assign')
    expect(resolveFlavorDisplay({ configFlavorId: '', selectedFlavorId: '', flavors })).toBe(
      'Auto-assign'
    )
  })

  it('shows the user-selected flavor name resolved from the flavor list', () => {
    expect(resolveFlavorDisplay({ selectedFlavorId: 'flavor-1', flavors })).toBe('m1.small')
  })

  it('prefers the flavor recorded in the migration ConfigMap', () => {
    expect(
      resolveFlavorDisplay({
        configFlavorId: 'flavor-2',
        selectedFlavorId: 'flavor-1',
        flavors,
      })
    ).toBe('m1.large')
  })

  it('marks auto-assigned flavors resolved by the controller', () => {
    expect(resolveFlavorDisplay({ configFlavorId: 'flavor-2', flavors })).toBe(
      'm1.large (auto-assigned)'
    )
  })

  it('falls back to the flavor ID when no name can be resolved', () => {
    expect(resolveFlavorDisplay({ selectedFlavorId: 'flavor-404', flavors })).toBe('flavor-404')
    expect(resolveFlavorDisplay({ configFlavorId: 'flavor-404' })).toBe(
      'flavor-404 (auto-assigned)'
    )
  })
})

describe('normalizeVmDisks', () => {
  it('returns empty for missing or non-array disks', () => {
    expect(normalizeVmDisks(undefined)).toEqual([])
    expect(normalizeVmDisks('disk-1')).toEqual([])
  })

  it('maps disk objects to rows with formatted sizes', () => {
    expect(
      normalizeVmDisks([
        { name: 'Hard disk 1', capacityGB: 33, datastore: 'datastore-nfs' },
        { name: 'Hard disk 2', capacityGB: 2048, datastore: 'ds-2' },
      ])
    ).toEqual([
      { name: 'Hard disk 1', size: '33 GB', datastore: 'datastore-nfs' },
      { name: 'Hard disk 2', size: '2 TB', datastore: 'ds-2' },
    ])
  })

  it('handles legacy string disks without sizes', () => {
    expect(normalizeVmDisks(['Hard disk 1'])).toEqual([
      { name: 'Hard disk 1', size: 'N/A', datastore: 'N/A' },
    ])
  })

  it('fills placeholder names and sizes for incomplete entries', () => {
    expect(normalizeVmDisks([{ capacityGB: 0 }, ''])).toEqual([
      { name: 'Disk 1', size: 'N/A', datastore: 'N/A' },
      { name: 'Disk 2', size: 'N/A', datastore: 'N/A' },
    ])
  })
})

/**
 * The flavor shown above comes from TARGET_FLAVOR_ID in the migration ConfigMap,
 * which the details page fetches through the vpwned k8s proxy. That proxy rejects
 * any path missing from its allowlist with a 403, so the path is pinned here
 * against a mirror of the backend regex in pkg/vpwned/server/k8s_proxy_handler.go
 */
const PROXY_PREFIX = '/dev-api/sdk/vpw/v1/k8s'
const ALLOWED_CONFIGMAP_PATH = /^\/api\/v1\/namespaces\/migration-system\/configmaps(\/[^/?]+)?$/

describe('getMigrationConfigMap — source of the flavor above', () => {
  const mockedGet = vi.mocked(axios.get)

  beforeEach(() => {
    mockedGet.mockReset()
    mockedGet.mockResolvedValue({ data: { TARGET_FLAVOR_ID: 'flavor-1' } })
  })

  it('requests the migration ConfigMap through the k8s proxy', async () => {
    const result = await getMigrationConfigMap('testvm')

    expect(mockedGet).toHaveBeenCalledWith({
      endpoint:
        '/dev-api/sdk/vpw/v1/k8s/api/v1/namespaces/migration-system/configmaps/migration-config-testvm',
    })
    expect(result).toEqual({ data: { TARGET_FLAVOR_ID: 'flavor-1' } })
  })

  it('builds a path the vpwned k8s proxy allowlist accepts', async () => {
    await getMigrationConfigMap('testvm')

    const { endpoint } = mockedGet.mock.calls[0][0]
    expect(endpoint.startsWith(PROXY_PREFIX)).toBe(true)
    expect(endpoint.slice(PROXY_PREFIX.length)).toMatch(ALLOWED_CONFIGMAP_PATH)
  })

  it('honours an explicit namespace', async () => {
    await getMigrationConfigMap('testvm', 'other-ns')

    expect(mockedGet).toHaveBeenCalledWith({
      endpoint:
        '/dev-api/sdk/vpw/v1/k8s/api/v1/namespaces/other-ns/configmaps/migration-config-testvm',
    })
  })
})

