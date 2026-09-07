import { describe, expect, it, vi, afterEach } from 'vitest'

const MOCK_RESPONSE = {
  version: '1.0.0',
  last_updated: '2026-09-07',
  prompt: 'test system prompt for debugging',
}

describe('fetchDebugPrompt', () => {
  const originalFetch = globalThis.fetch

  afterEach(() => {
    globalThis.fetch = originalFetch
    vi.clearAllMocks()
  })

  it('calls the correct endpoint and parses response', async () => {
    const { fetchDebugPrompt } = await import('./debugPrompt')

    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => MOCK_RESPONSE,
    }) as unknown as typeof fetch

    const result = await fetchDebugPrompt()
    expect(result.version).toBe('1.0.0')
    expect(result.last_updated).toBe('2026-09-07')
    expect(result.prompt).toBe('test system prompt for debugging')

    const callArgs = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls[0]
    expect(callArgs[0]).toContain('/vpw/v1/ai/debug-prompt')
  })

  it('throws on non-200 response', async () => {
    const { fetchDebugPrompt } = await import('./debugPrompt')

    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
    }) as unknown as typeof fetch

    await expect(fetchDebugPrompt()).rejects.toThrow()
  })
})
