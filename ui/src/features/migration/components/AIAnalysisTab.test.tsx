import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import { vi } from 'vitest'
import AIAnalysisTab from './AIAnalysisTab'
import * as aiAnalysis from 'src/api/ai/aiAnalysis'

vi.mock('src/api/ai/aiAnalysis', () => ({
  analyzeMigration: vi.fn(),
  getAIKeyStatus: vi.fn().mockResolvedValue({ configured: true }),
}))

const mockAnalyze = aiAnalysis.analyzeMigration as ReturnType<typeof vi.fn>
const mockGetKeyStatus = aiAnalysis.getAIKeyStatus as ReturnType<typeof vi.fn>

const defaultProps = {
  migrationName: 'migration-my-vm-abc12',
  namespace: 'migration-system',
}

const renderTab = (props = defaultProps) =>
  render(
    <MemoryRouter>
      <AIAnalysisTab {...props} />
    </MemoryRouter>
  )

describe('AIAnalysisTab', () => {
  beforeEach(() => vi.clearAllMocks())

  it('shows idle prompt before analysis is triggered', async () => {
    renderTab()
    // wait for getAIKeyStatus to resolve
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /analyse with ai/i })).not.toBeDisabled()
    })
    expect(screen.getByText(/click.*analyse with ai/i)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /analyse with ai/i })).toBeInTheDocument()
  })

  it('shows spinner while waiting for response', async () => {
    mockAnalyze.mockImplementation(() => new Promise(() => {}))
    renderTab()
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /analyse with ai/i })).not.toBeDisabled()
    })
    fireEvent.click(screen.getByRole('button', { name: /analyse with ai/i }))
    expect(await screen.findByText(/analysing logs/i)).toBeInTheDocument()
  })

  it('shows root cause and fix steps on high confidence response', async () => {
    mockAnalyze.mockResolvedValue({
      root_cause: 'ESXi host esxi-02 unreachable',
      fix_steps: ['Add esxi-02 to /etc/hosts', 'Retry migration'],
      summary: 'DNS resolution failed during disk copy',
      confidence: 'high',
      doc_references: [],
      github_issue: { should_open: false },
      raw_response: '',
    })
    renderTab()
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /analyse with ai/i })).not.toBeDisabled()
    })
    fireEvent.click(screen.getByRole('button', { name: /analyse with ai/i }))
    expect(await screen.findByText(/esxi host esxi-02 unreachable/i)).toBeInTheDocument()
    expect(screen.getByText(/add esxi-02 to \/etc\/hosts/i)).toBeInTheDocument()
    expect(screen.getByText(/retry migration/i)).toBeInTheDocument()
  })

  it('shows github issue button and checklist when confidence is none', async () => {
    mockAnalyze.mockResolvedValue({
      root_cause: null,
      fix_steps: [],
      summary: 'Unable to determine root cause',
      confidence: 'none',
      doc_references: [],
      github_issue: {
        should_open: true,
        title: 'Migration failure: migration-my-vm-abc12',
        prefill_url: 'https://github.com/platform9/vjailbreak/issues/new?title=...',
        collect_first: ['Collect journalctl logs', 'Note ESXi version'],
      },
      raw_response: '',
    })
    renderTab()
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /analyse with ai/i })).not.toBeDisabled()
    })
    fireEvent.click(screen.getByRole('button', { name: /analyse with ai/i }))
    expect(await screen.findByRole('link', { name: /open github issue/i })).toBeInTheDocument()
    expect(screen.getByText(/collect journalctl logs/i)).toBeInTheDocument()
  })

  it('shows error alert on API failure', async () => {
    mockAnalyze.mockRejectedValue(new Error('Service unavailable'))
    renderTab()
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /analyse with ai/i })).not.toBeDisabled()
    })
    fireEvent.click(screen.getByRole('button', { name: /analyse with ai/i }))
    expect(await screen.findByRole('alert')).toBeInTheDocument()
    expect(screen.getByText(/ai service unavailable/i)).toBeInTheDocument()
  })

  it('sends follow-up question and appends to conversation', async () => {
    const initial = {
      root_cause: 'DNS failure',
      fix_steps: ['add to /etc/hosts'],
      summary: 'DNS issue',
      confidence: 'high',
      doc_references: [],
      github_issue: { should_open: false },
      raw_response: 'DNS failure response',
    }
    mockAnalyze.mockResolvedValueOnce(initial).mockResolvedValueOnce({
      ...initial,
      root_cause: 'Follow-up answered',
      raw_response: 'Follow-up answered in the conversation thread',
    })
    renderTab()
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /analyse with ai/i })).not.toBeDisabled()
    })
    fireEvent.click(screen.getByRole('button', { name: /analyse with ai/i }))
    await screen.findByText(/dns failure/i)

    const input = screen.getByPlaceholderText(/ask a follow-up/i)
    fireEvent.change(input, { target: { value: 'Why did DNS fail?' } })
    fireEvent.submit(input.closest('form')!)

    // A follow-up appends to the conversation thread; it does not replace the analysis
    // panel, so assert on the rendered turn (raw_response) rather than root_cause.
    expect(
      await screen.findByText(/follow-up answered in the conversation thread/i)
    ).toBeInTheDocument()
    expect(screen.getByText('Why did DNS fail?')).toBeInTheDocument()

    expect(mockAnalyze).toHaveBeenCalledTimes(2)
    const secondCall = mockAnalyze.mock.calls[1][0]
    expect(secondCall.conversation_history.length).toBeGreaterThan(0)
    expect(secondCall.question).toBe('Why did DNS fail?')
  })

  it('disables Send button when follow-up input is empty', async () => {
    mockAnalyze.mockResolvedValueOnce({
      root_cause: 'DNS failure',
      fix_steps: ['add to /etc/hosts'],
      summary: 'DNS issue',
      confidence: 'high',
      doc_references: [],
      github_issue: { should_open: false },
      raw_response: 'initial response',
    })
    renderTab()
    // Wait for key status check to complete, then trigger analysis
    await waitFor(() => expect(screen.getByRole('button', { name: /analyse with ai/i })).not.toBeDisabled())
    fireEvent.click(screen.getByRole('button', { name: /analyse with ai/i }))
    await screen.findByText(/dns failure/i)

    // Empty input — Send should be disabled
    const sendButton = screen.getByRole('button', { name: /send/i })
    expect(sendButton).toBeDisabled()
  })

  it('maintains conversation history across multiple analyses', async () => {
    const firstResponse = {
      root_cause: 'First analysis',
      fix_steps: [],
      summary: 'First',
      confidence: 'high' as const,
      doc_references: [],
      github_issue: { should_open: false },
      raw_response: 'first response',
    }
    const secondResponse = { ...firstResponse, root_cause: 'Second analysis', raw_response: 'second response' }
    mockAnalyze.mockResolvedValueOnce(firstResponse).mockResolvedValueOnce(secondResponse)

    renderTab()
    await waitFor(() => expect(screen.getByRole('button', { name: /analyse with ai/i })).not.toBeDisabled())
    fireEvent.click(screen.getByRole('button', { name: /analyse with ai/i }))
    await screen.findByText(/first analysis/i)

    // Click Analyse again — history should be cleared and second analysis runs with empty history
    fireEvent.click(screen.getByRole('button', { name: /analyse with ai/i }))
    await screen.findByText(/second analysis/i)

    expect(mockAnalyze).toHaveBeenCalledTimes(2)
    const secondCall = mockAnalyze.mock.calls[1][0]
    // handleAnalyse starts a fresh conversation: it clears history and passes an explicit
    // empty override, so a re-analysis never carries the previous exchange.
    expect(secondCall.conversation_history).toHaveLength(0)
    expect(secondCall.question).toBeUndefined()
  })

  it('second API call includes correct conversation_history shape', async () => {
    const initial = {
      root_cause: 'DNS failure',
      fix_steps: ['add to /etc/hosts'],
      summary: 'DNS issue',
      confidence: 'high' as const,
      doc_references: [],
      github_issue: { should_open: false },
      raw_response: 'initial assistant response',
    }
    mockAnalyze.mockResolvedValueOnce(initial).mockResolvedValueOnce({
      ...initial,
      root_cause: 'Follow-up answered',
      raw_response: 'follow-up response text',
    })

    renderTab()
    await waitFor(() => expect(screen.getByRole('button', { name: /analyse with ai/i })).not.toBeDisabled())
    fireEvent.click(screen.getByRole('button', { name: /analyse with ai/i }))
    await screen.findByText(/dns failure/i)

    const input = screen.getByPlaceholderText(/ask a follow-up/i)
    fireEvent.change(input, { target: { value: 'What does this mean?' } })
    fireEvent.submit(input.closest('form')!)
    await screen.findByText(/follow-up response text/i)

    expect(mockAnalyze).toHaveBeenCalledTimes(2)
    const secondCall = mockAnalyze.mock.calls[1][0]
    // The initial analysis seeds two turns: the implied user request, and a human-readable
    // rendering of the analysis (root cause / fix steps / summary) rather than raw_response,
    // so follow-ups do not prompt the model with JSON.
    expect(secondCall.conversation_history).toHaveLength(2)
    expect(secondCall.conversation_history[0].role).toBe('user')
    expect(secondCall.conversation_history[0].content).toBe('Analyse this failed migration')
    expect(secondCall.conversation_history[1].role).toBe('assistant')
    expect(secondCall.conversation_history[1].content).toContain('Root cause: DNS failure')
    expect(secondCall.conversation_history[1].content).toContain('add to /etc/hosts')
    expect(secondCall.conversation_history[1].content).toContain('Summary: DNS issue')
    expect(secondCall.question).toBe('What does this mean?')
  })

  it('navigates to the AI tab of Global Settings when the key is not configured', async () => {
    mockGetKeyStatus.mockResolvedValueOnce({ configured: false })
    render(
      <MemoryRouter initialEntries={['/dashboard/migrations/migration-my-vm-abc12']}>
        <Routes>
          <Route path="/dashboard/migrations/:migrationName" element={<AIAnalysisTab {...defaultProps} />} />
          <Route path="/dashboard/global-settings" element={<div>Global Settings Page</div>} />
        </Routes>
      </MemoryRouter>
    )

    const link = await screen.findByRole('button', { name: /configure in settings/i })
    fireEvent.click(link)

    expect(await screen.findByText('Global Settings Page')).toBeInTheDocument()
  })
})
