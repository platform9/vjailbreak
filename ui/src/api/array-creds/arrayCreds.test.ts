import { beforeEach, describe, expect, it, vi } from 'vitest'

import axios from '../axios'
import { createArrayCredsWithSecret, updateArrayCredsWithSecret } from './arrayCreds'

vi.mock('../axios', () => ({
  default: {
    post: vi.fn(),
    patch: vi.fn()
  }
}))

const mockedAxios = vi.mocked(axios, true)

describe('vantaraConfig round-trip', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockedAxios.post.mockResolvedValue(undefined)
    mockedAxios.patch.mockResolvedValue(undefined)
  })

  it('includes vantaraConfig in the create request when vendorType is vantara', async () => {
    await createArrayCredsWithSecret(
      'vantara-1',
      'vantara-1-array-secret',
      'vantara',
      { volumeType: 'vt-1', cinderBackendName: 'vantara-backend' },
      undefined,
      { poolId: '5', restPort: '8443' }
    )

    expect(mockedAxios.post).toHaveBeenCalledWith({
      endpoint: expect.stringContaining('/arraycreds'),
      data: expect.objectContaining({
        spec: expect.objectContaining({
          vendorType: 'vantara',
          vantaraConfig: { poolId: '5', restPort: '8443' }
        })
      })
    })
  })

  it('omits vantaraConfig from the create request when fields are empty', async () => {
    await createArrayCredsWithSecret(
      'vantara-1',
      'vantara-1-array-secret',
      'vantara',
      { volumeType: 'vt-1', cinderBackendName: 'vantara-backend' },
      undefined,
      { poolId: '', restPort: '' }
    )

    const body = (mockedAxios.post.mock.calls[0][0] as { data: { spec: Record<string, unknown> } })
      .data
    expect(body.spec.vantaraConfig).toBeUndefined()
  })

  it('omits vantaraConfig from the create request for other vendors', async () => {
    await createArrayCredsWithSecret(
      'pure-1',
      'pure-1-array-secret',
      'pure',
      { volumeType: 'vt-1', cinderBackendName: 'pure-backend' },
      undefined,
      { poolId: '5', restPort: '8443' }
    )

    const body = (mockedAxios.post.mock.calls[0][0] as { data: { spec: Record<string, unknown> } })
      .data
    expect(body.spec.vantaraConfig).toBeUndefined()
  })

  it('includes vantaraConfig in the update request when vendorType is vantara', async () => {
    await updateArrayCredsWithSecret(
      'vantara-1',
      'vantara-1-array-secret',
      'vantara',
      { volumeType: 'vt-1', cinderBackendName: 'vantara-backend' },
      undefined,
      { poolId: '5', restPort: '8443' }
    )

    expect(mockedAxios.patch).toHaveBeenCalledWith({
      endpoint: expect.stringContaining('/arraycreds/vantara-1'),
      data: expect.objectContaining({
        spec: expect.objectContaining({
          vantaraConfig: { poolId: '5', restPort: '8443' }
        })
      }),
      config: { headers: { 'Content-Type': 'application/merge-patch+json' } }
    })
  })
})
