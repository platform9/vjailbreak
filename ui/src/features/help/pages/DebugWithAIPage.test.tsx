import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import * as debugPromptModule from 'src/api/ai/debugPrompt'

vi.mock('src/api/ai/debugPrompt', () => ({
  fetchDebugPrompt: vi.fn(),
}))

import DebugWithAIPage from './DebugWithAIPage'

const MOCK_RESPONSE = {
  version: '1.0.0',
  last_updated: '2026-09-07',
  prompt: 'test system prompt for debugging',
}

const mockFetch = debugPromptModule.fetchDebugPrompt as ReturnType<typeof vi.fn>

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
      expect(screen.getAllByText(/v1\.0\.0/).length).toBeGreaterThan(0)
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
