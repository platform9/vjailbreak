import { beforeEach, describe, expect, it, vi } from 'vitest'

import axios from '../axios'
import { patchMigrationPodLabels, setLDMBootStatus, triggerAdminCutover } from './migrations'

vi.mock('../axios', () => ({
  default: {
    get: vi.fn(),
    patch: vi.fn()
  }
}))

const mockedAxios = vi.mocked(axios, true)

// getMigration and the pod list both go through axios.get, in that order.
const mockPodResolution = (podRef: string | undefined, podNames: string[]) => {
  mockedAxios.get
    .mockResolvedValueOnce({ spec: podRef ? { podRef } : {} })
    .mockResolvedValueOnce({
      items: podNames.map((name) => ({ metadata: { name, namespace: 'migration-system' } }))
    })
}

describe('migration pod label patching', () => {
  beforeEach(() => {
    // resetAllMocks, not clearAllMocks: clearAllMocks only wipes recorded calls and leaves
    // the mockResolvedValueOnce queue behind. Tests that throw before the pod list is
    // fetched consume just one of their two queued values, and the leftover is then served
    // to the next test as its getMigration response.
    vi.resetAllMocks()
    mockedAxios.patch.mockResolvedValue(undefined)
  })

  it('resolves the pod by podRef prefix and merge-patches the labels', async () => {
    mockPodResolution('v2v-helper-abc', ['unrelated-pod', 'v2v-helper-abc-x7k2q'])

    await patchMigrationPodLabels('migration-system', 'mig-1', { someLabel: 'value' })

    expect(mockedAxios.patch).toHaveBeenCalledWith({
      endpoint: expect.stringContaining('/namespaces/migration-system/pods/v2v-helper-abc-x7k2q'),
      data: { metadata: { labels: { someLabel: 'value' } } },
      config: { headers: { 'Content-Type': 'application/merge-patch+json' } }
    })
  })

  it('throws when the migration has no podRef', async () => {
    mockPodResolution(undefined, [])
    await expect(patchMigrationPodLabels('migration-system', 'mig-1', {})).rejects.toThrow(
      'PodRef is empty in migration object'
    )
  })

  it('throws when no pod matches the podRef', async () => {
    mockPodResolution('v2v-helper-abc', ['some-other-pod'])
    await expect(patchMigrationPodLabels('migration-system', 'mig-1', {})).rejects.toThrow(
      'No pod found with name starting with: v2v-helper-abc'
    )
  })

  // The two gates must stay on the same resolve-and-patch path, but must never
  // write each other's label - startCutover drives the cutover flow and
  // ldmBootStatus drives the LDM boot gate.
  it('triggerAdminCutover sets only startCutover', async () => {
    mockPodResolution('v2v-helper-abc', ['v2v-helper-abc-x7k2q'])

    const result = await triggerAdminCutover('migration-system', 'mig-1')

    expect(result.success).toBe(true)
    expect(mockedAxios.patch.mock.calls[0][0].data).toEqual({
      metadata: { labels: { startCutover: 'yes' } }
    })
  })

  it('setLDMBootStatus sets only ldmBootStatus', async () => {
    mockPodResolution('v2v-helper-abc', ['v2v-helper-abc-x7k2q'])

    const result = await setLDMBootStatus('migration-system', 'mig-1', 'success')

    expect(result.success).toBe(true)
    expect(mockedAxios.patch.mock.calls[0][0].data).toEqual({
      metadata: { labels: { ldmBootStatus: 'success' } }
    })
  })

  it('reports failure without throwing when the pod cannot be resolved', async () => {
    mockPodResolution(undefined, [])

    const result = await setLDMBootStatus('migration-system', 'mig-1', 'finish')

    expect(result.success).toBe(false)
    expect(result.message).toBe('PodRef is empty in migration object')
    expect(mockedAxios.patch).not.toHaveBeenCalled()
  })
})
