import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { downloadDebugBundle } from './debugBundle'
import { getBlob } from '../axios'

vi.mock('../axios', () => ({
  getBlob: vi.fn()
}))

const mockedGetBlob = vi.mocked(getBlob)

const blobResponse = (contentDisposition?: string) =>
  ({
    data: new Blob(['bundle content'], { type: 'text/plain' }),
    headers: contentDisposition ? { 'content-disposition': contentDisposition } : {}
  }) as Awaited<ReturnType<typeof getBlob>>

describe('downloadDebugBundle', () => {
  let clickedDownloads: string[]

  beforeEach(() => {
    clickedDownloads = []
    URL.createObjectURL = vi.fn(() => 'blob:mock-url')
    URL.revokeObjectURL = vi.fn()
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (this: HTMLAnchorElement) {
      clickedDownloads.push(this.download)
    })
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('requests by migration name and uses the server file name', async () => {
    mockedGetBlob.mockResolvedValue(blobResponse('attachment; filename="testvm-pod-2026.txt"'))

    await downloadDebugBundle('migration-testvm')

    expect(mockedGetBlob).toHaveBeenCalledWith({
      endpoint: '/dev-api/sdk/vpw/v1/debug-bundle',
      config: { params: { namespace: 'migration-system', migration: 'migration-testvm' } }
    })
    expect(clickedDownloads).toEqual(['testvm-pod-2026.txt'])
  })

  it('passes the pod parameter and omits an empty migration name', async () => {
    mockedGetBlob.mockResolvedValue(blobResponse())

    await downloadDebugBundle(undefined, 'migration-system', 'v2v-helper-testvm')

    expect(mockedGetBlob).toHaveBeenCalledWith({
      endpoint: '/dev-api/sdk/vpw/v1/debug-bundle',
      config: { params: { namespace: 'migration-system', pod: 'v2v-helper-testvm' } }
    })
  })

  it('falls back to a local file name without Content-Disposition', async () => {
    mockedGetBlob.mockResolvedValue(blobResponse())

    await downloadDebugBundle('migration-testvm')

    expect(clickedDownloads).toEqual(['migration-testvm-debug-bundle.txt'])
  })

  it('propagates request failures to the caller', async () => {
    mockedGetBlob.mockRejectedValue(new Error('HTTP 503'))

    await expect(downloadDebugBundle('migration-testvm')).rejects.toThrow('HTTP 503')
  })
})
