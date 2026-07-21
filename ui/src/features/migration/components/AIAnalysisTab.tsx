import { useState, useCallback, useEffect } from 'react'
import {
  Box,
  Button,
  IconButton,
  Typography,
  CircularProgress,
  Alert,
  Divider,
  List,
  ListItem,
  ListItemText,
  Chip,
  TextField,
  Link,
} from '@mui/material'
import AutoFixHighIcon from '@mui/icons-material/AutoFixHigh'
import OpenInNewIcon from '@mui/icons-material/OpenInNew'
import ThumbUpIcon from '@mui/icons-material/ThumbUp'
import ThumbDownIcon from '@mui/icons-material/ThumbDown'
import { ActionButton, Banner, StatusChip } from 'src/components'
import { analyzeMigration, getAIKeyStatus } from 'src/api/ai/aiAnalysis'
import type { AIAnalyzeResponse } from 'src/api/ai/model'
import { trackEvent } from 'src/services/amplitudeService'
import { AMPLITUDE_EVENTS } from 'src/types/amplitude'

interface AIAnalysisTabProps {
  migrationName: string
  namespace: string
}

type ConversationTurn = { role: 'user' | 'assistant'; content: string }

const confidenceTone = {
  high: 'success',
  medium: 'warning',
  low: 'warning',
  none: 'error',
} as const satisfies Record<string, 'success' | 'warning' | 'error'>

export default function AIAnalysisTab({ migrationName, namespace }: AIAnalysisTabProps) {
  const [loading, setLoading] = useState(false)
  const [followUpLoading, setFollowUpLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [result, setResult] = useState<AIAnalyzeResponse | null>(null)
  const [history, setHistory] = useState<ConversationTurn[]>([])
  const [followUp, setFollowUp] = useState('')
  const [keyConfigured, setKeyConfigured] = useState<boolean | null>(null)
  const [feedback, setFeedback] = useState<'positive' | 'negative' | null>(null)

  useEffect(() => {
    getAIKeyStatus()
      .then((s) => setKeyConfigured(s.configured))
      .catch(() => setKeyConfigured(false))
  }, [])

  const runAnalysis = useCallback(async (question?: string, historyOverride?: ConversationTurn[]) => {
    const isFollowUp = !!question
    if (isFollowUp) {
      setFollowUpLoading(true)
    } else {
      setLoading(true)
    }
    setError(null)
    try {
      const resp = await analyzeMigration({
        migration_name: migrationName,
        namespace,
        question: question || undefined,
        conversation_history: historyOverride ?? history,
      })
      if (isFollowUp) {
        setHistory((prev) => [
          ...prev,
          { role: 'user', content: question },
          { role: 'assistant', content: resp.raw_response },
        ])
      } else {
        // Store human-readable analysis in history so Claude doesn't
        // pattern-match and return JSON for follow-up questions
        const analysisText = [
          resp.root_cause ? `Root cause: ${resp.root_cause}` : null,
          resp.fix_steps.length > 0
            ? `Fix steps:\n${resp.fix_steps.map((s, i) => `${i + 1}. ${s}`).join('\n')}`
            : null,
          resp.summary ? `Summary: ${resp.summary}` : null,
        ]
          .filter(Boolean)
          .join('\n\n') || resp.raw_response
        setHistory([
          { role: 'user', content: 'Analyse this failed migration' },
          { role: 'assistant', content: analysisText },
        ])
        setResult(resp)
        trackEvent(AMPLITUDE_EVENTS.AI_ANALYSIS_TRIGGERED, {
          migration_name: migrationName,
          namespace,
          confidence: resp.confidence,
          root_cause: resp.root_cause ?? undefined,
        })
      }
      setFollowUp('')
    } catch {
      setError('AI service unavailable. Check vjailbreak-ai deployment or API key configuration.')
    } finally {
      setLoading(false)
      setFollowUpLoading(false)
    }
  }, [migrationName, namespace, history])

  const handleAnalyse = useCallback(() => {
    setResult(null)
    setHistory([])
    setFeedback(null)
    runAnalysis(undefined, [])
  }, [runAnalysis])

  const handleFeedback = useCallback((value: 'positive' | 'negative') => {
    setFeedback(value)
    trackEvent(AMPLITUDE_EVENTS.AI_ANALYSIS_FEEDBACK, {
      migration_name: migrationName,
      namespace,
      feedback: value,
      confidence: result?.confidence,
      root_cause: result?.root_cause ?? undefined,
    })
  }, [migrationName, namespace, result])

  const handleFollowUp = useCallback(
    (e: React.FormEvent) => {
      e.preventDefault()
      if (followUp.trim()) runAnalysis(followUp.trim())
    },
    [followUp, runAnalysis]
  )

  if (!result && !loading && !followUpLoading && !error) {
    if (keyConfigured === false) {
      return (
        <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center', py: 6, gap: 2 }}>
          <AutoFixHighIcon sx={{ fontSize: 48, color: 'text.secondary' }} />
          <Typography color="text.secondary">
            Anthropic API key not configured.{' '}
            <Link href="/settings?tab=ai">Configure in Settings →</Link>
          </Typography>
        </Box>
      )
    }
    return (
      <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center', py: 6, gap: 2 }}>
        <AutoFixHighIcon sx={{ fontSize: 48, color: 'text.secondary' }} />
        <Typography color="text.secondary">
          Click &quot;Analyse with AI&quot; to diagnose this failed migration
        </Typography>
        <Button
          variant="contained"
          startIcon={<AutoFixHighIcon />}
          onClick={handleAnalyse}
          disabled={keyConfigured === null}
        >
          Analyse with AI
        </Button>
      </Box>
    )
  }

  if (loading) {
    return (
      <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'center', py: 6, gap: 2 }}>
        <CircularProgress size={24} />
        <Typography color="text.secondary">Analysing logs with AI...</Typography>
      </Box>
    )
  }

  if (error) {
    return (
      <Box sx={{ p: 2 }}>
        <Banner
          variant="error"
          message={error}
          actionLabel="Retry"
          onAction={handleAnalyse}
          actionProps={{ size: 'small', variant: 'outlined' }}
        />
      </Box>
    )
  }

  return (
    <Box sx={{ p: 2, display: 'flex', flexDirection: 'column', gap: 2 }}>
      <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <Button size="small" startIcon={<AutoFixHighIcon />} onClick={handleAnalyse} variant="outlined">
          Analyse with AI
        </Button>
        {result && (
          <StatusChip
            label={`${result.confidence} confidence`}
            tone={confidenceTone[result.confidence]}
            size="small"
          />
        )}
      </Box>

      {result?.confidence === 'none' ? (
        <Alert severity="warning" icon={false}>
          <Typography variant="subtitle2" gutterBottom>
            Could not determine root cause automatically.
          </Typography>
          {result.github_issue?.collect_first && result.github_issue.collect_first.length > 0 && (
            <>
              <Typography variant="body2" sx={{ mb: 1 }}>
                Before opening an issue, collect the following:
              </Typography>
              <List dense disablePadding>
                {result.github_issue.collect_first.map((item, i) => (
                  <ListItem key={i} disableGutters sx={{ py: 0 }}>
                    <ListItemText primary={`□ ${item}`} primaryTypographyProps={{ variant: 'body2' }} />
                  </ListItem>
                ))}
              </List>
            </>
          )}
          {result.github_issue?.prefill_url && (
            <Box sx={{ mt: 1 }}>
              <Button
                size="small"
                variant="outlined"
                endIcon={<OpenInNewIcon />}
                component="a"
                href={result.github_issue.prefill_url}
                target="_blank"
                rel="noopener"
                onClick={() => trackEvent(AMPLITUDE_EVENTS.AI_GITHUB_ISSUE_OPENED, {
                  migration_name: migrationName,
                  confidence: result.confidence,
                  root_cause: result.root_cause ?? undefined,
                })}
              >
                Open GitHub Issue
              </Button>
            </Box>
          )}
        </Alert>
      ) : (
        result && (
          <>
            {result.root_cause && (
              <Box>
                <Typography variant="subtitle2" gutterBottom>Root Cause</Typography>
                <Typography variant="body2">{result.root_cause}</Typography>
              </Box>
            )}

            {result.fix_steps.length > 0 && (
              <>
                <Divider />
                <Box>
                  <Typography variant="subtitle2" gutterBottom>Fix Steps</Typography>
                  <List dense disablePadding>
                    {result.fix_steps.map((step, i) => (
                      <ListItem key={i} disableGutters>
                        <ListItemText primary={`${i + 1}. ${step}`} />
                      </ListItem>
                    ))}
                  </List>
                </Box>
              </>
            )}

            {result.doc_references.length > 0 && (
              <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap' }}>
                {result.doc_references.map((url, i) => (
                  <Chip
                    key={i}
                    label={new URL(url).hostname}
                    size="small"
                    component="a"
                    href={url}
                    target="_blank"
                    clickable
                    icon={<OpenInNewIcon />}
                  />
                ))}
              </Box>
            )}

            <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
                <Typography variant="caption" color="text.secondary">Was this helpful?</Typography>
                <IconButton
                  size="small"
                  onClick={() => handleFeedback('positive')}
                  color={feedback === 'positive' ? 'success' : 'default'}
                  aria-label="helpful"
                >
                  <ThumbUpIcon fontSize="small" />
                </IconButton>
                <IconButton
                  size="small"
                  onClick={() => handleFeedback('negative')}
                  color={feedback === 'negative' ? 'error' : 'default'}
                  aria-label="not helpful"
                >
                  <ThumbDownIcon fontSize="small" />
                </IconButton>
              </Box>
              <Button
                size="small"
                variant="outlined"
                endIcon={<OpenInNewIcon />}
                component="a"
                href={
                  result.github_issue?.prefill_url ||
                  `https://github.com/platform9/vjailbreak/issues/new?title=${encodeURIComponent(result.root_cause || `Migration failure: ${migrationName}`)}`
                }
                target="_blank"
                rel="noopener"
                onClick={() => trackEvent(AMPLITUDE_EVENTS.AI_GITHUB_ISSUE_OPENED, {
                  migration_name: migrationName,
                  confidence: result.confidence,
                  root_cause: result.root_cause ?? undefined,
                })}
              >
                Open GitHub Issue
              </Button>
            </Box>
          </>
        )
      )}

      <Divider />

      {history.length > 2 && (
        <>
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
            {history.slice(2).map((turn, i) => (
              <Box
                key={i}
                sx={{
                  px: 1.5,
                  py: 1,
                  borderRadius: 1,
                  bgcolor: turn.role === 'user' ? 'action.hover' : 'background.paper',
                  border: turn.role === 'assistant' ? '1px solid' : 'none',
                  borderColor: 'divider',
                }}
              >
                <Typography variant="caption" color="text.secondary" sx={{ fontWeight: 600 }}>
                  {turn.role === 'user' ? 'You' : 'AI'}
                </Typography>
                <Typography variant="body2" sx={{ mt: 0.5, whiteSpace: 'pre-wrap' }}>
                  {turn.content}
                </Typography>
              </Box>
            ))}
          </Box>
          <Divider />
        </>
      )}

      <Box component="form" onSubmit={handleFollowUp} sx={{ display: 'flex', gap: 1 }}>
        <TextField
          size="small"
          fullWidth
          placeholder="Ask a follow-up question..."
          value={followUp}
          onChange={(e) => setFollowUp(e.target.value)}
          disabled={followUpLoading}
        />
        <ActionButton type="submit" size="small" loading={followUpLoading} disabled={!followUp.trim()}>
          Send
        </ActionButton>
      </Box>
    </Box>
  )
}
