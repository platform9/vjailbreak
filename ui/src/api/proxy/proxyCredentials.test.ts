import { beforeEach, describe, expect, it, vi } from 'vitest'

import api from '../axios'
import { getProxyCredsStatus, saveProxyCreds, deleteProxyCreds } from './proxyCredentials'

vi.mock('../axios', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    del: vi.fn()
  }
}))

const mockedApi = vi.mocked(api, true)

describe('proxyCredentials API', () => {
  beforeEach(() => {
    vi.resetAllMocks()
  })

  it('getProxyCredsStatus GETs the credentials endpoint', async () => {
    mockedApi.get.mockResolvedValue({ configured: true, https_override: false })

    const result = await getProxyCredsStatus()

    expect(mockedApi.get).toHaveBeenCalledWith({
      endpoint: '/dev-api/sdk/vpw/v1/proxy/credentials'
    })
    expect(result).toEqual({ configured: true, https_override: false })
  })

  it('saveProxyCreds POSTs the shared credentials payload', async () => {
    mockedApi.post.mockResolvedValue({ configured: true, https_override: false })

    const result = await saveProxyCreds({
      username: 'proxyuser',
      password: 'proxypass',
      https_override: false
    })

    expect(mockedApi.post).toHaveBeenCalledWith({
      endpoint: '/dev-api/sdk/vpw/v1/proxy/credentials',
      data: {
        username: 'proxyuser',
        password: 'proxypass',
        https_override: false
      }
    })
    expect(result).toEqual({ configured: true, https_override: false })
  })

  it('saveProxyCreds POSTs distinct HTTPS credentials when overriding', async () => {
    mockedApi.post.mockResolvedValue({ configured: true, https_override: true })

    await saveProxyCreds({
      username: 'proxyuser',
      password: 'proxypass',
      https_override: true,
      https_username: 'httpsuser',
      https_password: 'httpspass'
    })

    expect(mockedApi.post).toHaveBeenCalledWith({
      endpoint: '/dev-api/sdk/vpw/v1/proxy/credentials',
      data: {
        username: 'proxyuser',
        password: 'proxypass',
        https_override: true,
        https_username: 'httpsuser',
        https_password: 'httpspass'
      }
    })
  })

  it('deleteProxyCreds DELETEs the credentials endpoint', async () => {
    mockedApi.del.mockResolvedValue({ configured: false, https_override: false })

    const result = await deleteProxyCreds()

    expect(mockedApi.del).toHaveBeenCalledWith({
      endpoint: '/dev-api/sdk/vpw/v1/proxy/credentials'
    })
    expect(result).toEqual({ configured: false, https_override: false })
  })
})
