import { render, screen, fireEvent, waitFor, act } from '@testing-library/react'
import { vi } from 'vitest'
import AIAnalysisTab from './AIAnalysisTab'
import * as aiAnalysis from 'src/api/ai/aiAnalysis'

vi.mock('src/api/ai/aiAnalysis', () => ({
  analyzeMigration: vi.fn(),
  getAIKeyStatus: vi.fn().mockResolvedValue({ configured: true }),
}))

const mockAnalyze = aiAnalysis.analyzeMigration as ReturnType<typeof vi.fn>

const defaultProps = {
  migrationName: 'migration-my-vm-abc12',
  namespace: 'migration-system',
}

describe('AIAnalysisTab', () => {
  beforeEach(() => vi.clearAllMocks())

  it('shows idle prompt before analysis is triggered', async () => {
    render(<AIAnalysisTab {...defaultProps} />)
    // wait for getAIKeyStatus to resolve
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /analyse with ai/i })).not.toBeDisabled()
    })
    expect(screen.getByText(/click.*analyse with ai/i)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /analyse with ai/i })).toBeInTheDocument()
  })

  it('shows spinner while waiting for response', async () => {
    mockAnalyze.mockImplementation(() => new Promise(() => {}))
    render(<AIAnalysisTab {...defaultProps} />)
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
    render(<AIAnalysisTab {...defaultProps} />)
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
    render(<AIAnalysisTab {...defaultProps} />)
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /analyse with ai/i })).not.toBeDisabled()
    })
    fireEvent.click(screen.getByRole('button', { name: /analyse with ai/i }))
    expect(await screen.findByRole('link', { name: /open github issue/i })).toBeInTheDocument()
    expect(screen.getByText(/collect journalctl logs/i)).toBeInTheDocument()
  })

  it('shows error alert on API failure', async () => {
    mockAnalyze.mockRejectedValue(new Error('Service unavailable'))
    render(<AIAnalysisTab {...defaultProps} />)
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
    })
    render(<AIAnalysisTab {...defaultProps} />)
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /analyse with ai/i })).not.toBeDisabled()
    })
    fireEvent.click(screen.getByRole('button', { name: /analyse with ai/i }))
    await screen.findByText(/dns failure/i)

    const input = screen.getByPlaceholderText(/ask a follow-up/i)
    fireEvent.change(input, { target: { value: 'Why did DNS fail?' } })
    fireEvent.submit(input.closest('form')!)
    expect(await screen.findByText(/follow-up answered/i)).toBeInTheDocument()

    expect(mockAnalyze).toHaveBeenCalledTimes(2)
    const secondCall = mockAnalyze.mock.calls[1][0]
    expect(secondCall.conversation_history.length).toBeGreaterThan(0)
    expect(secondCall.question).toBe('Why did DNS fail?')
  })
})
