import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'

// Mock the API module before importing the component
vi.mock('src/api/ai/debugPrompt', () => ({
  fetchDebugPrompt: vi.fn(),
}))

import { fetchDebugPrompt } from 'src/api/ai/debugPrompt'
import DebugWithAIPage from './DebugWithAIPage'

const MOCK_RESPONSE = {
  version: '1.0.0',
  last_updated: '2026-09-07',
  prompt: 'test system prompt for debugging',
}

const mockFetch = vi.mocked(fetchDebugPrompt)

describe('DebugWithAIPage', () => {
  beforeEach(() => {
    mockFetch.mockResolvedValue(MOCK_RESPONSE)
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('renders Claude Code tab as active by default', async () => {
    render(<DebugWithAIPage />)
    await waitFor(() => {
      const tab = screen.getByRole('tab', { name: /claude code/i })
      expect(tab).toHaveAttribute('aria-selected', 'true')
    })
  })

  it('shows version badge from API response', async () => {
    render(<DebugWithAIPage />)
    await waitFor(() => {
      expect(screen.getByText(/v1\.0\.0/)).toBeTruthy()
    })
  })

  it('shows Other Agents tab', async () => {
    render(<DebugWithAIPage />)
    await waitFor(() => {
      expect(screen.getByRole('tab', { name: /other agents/i })).toBeTruthy()
    })
  })

  it('shows version unavailable when API fails', async () => {
    mockFetch.mockRejectedValue(new Error('network error'))
    render(<DebugWithAIPage />)
    await waitFor(() => {
      expect(screen.getByText(/version unavailable/i)).toBeTruthy()
    })
  })
})

// ── fetchDebugPrompt utility tests (Constitution Principle XI) ────────────────
describe('fetchDebugPrompt', () => {
  const originalFetch = globalThis.fetch

  afterEach(() => {
    globalThis.fetch = originalFetch
    vi.clearAllMocks()
  })

  it('calls the correct endpoint and parses response', async () => {
    // Unmock for this describe block — use the real module directly
    vi.unmock('src/api/ai/debugPrompt')
    const { fetchDebugPrompt: realFetch } = await import('src/api/ai/debugPrompt')

    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => MOCK_RESPONSE,
    }) as unknown as typeof fetch

    const result = await realFetch()
    expect(result.version).toBe('1.0.0')
    expect(result.last_updated).toBe('2026-09-07')
    expect(result.prompt).toBe('test system prompt for debugging')

    const callArgs = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls[0]
    expect(callArgs[0]).toContain('/vpw/v1/ai/debug-prompt')
  })

  it('throws on non-200 response', async () => {
    vi.unmock('src/api/ai/debugPrompt')
    const { fetchDebugPrompt: realFetch } = await import('src/api/ai/debugPrompt')

    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
    }) as unknown as typeof fetch

    await expect(realFetch()).rejects.toThrow()
  })
})
