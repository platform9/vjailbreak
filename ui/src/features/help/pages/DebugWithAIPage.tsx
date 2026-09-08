import { useEffect, useState } from 'react'
import {
  Box,
  Button,
  Chip,
  Link,
  Tab,
  Tabs,
  TextField,
  Typography,
} from '@mui/material'
import SmartToyIcon from '@mui/icons-material/SmartToy'
import { fetchDebugPrompt, type DebugPromptResponse } from 'src/api/ai/debugPrompt'

const REPO_SKILL_URL =
  'https://github.com/platform9/vjailbreak/tree/main/.claude/skills/vjb-debug'

function ClaudeCodeTab({ version }: { version: string | null }) {
  return (
    <Box sx={{ maxWidth: 720, mt: 3 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 2 }}>
        <Typography variant="h6">Install the vjb-debug Skill</Typography>
        {version !== null ? (
          <Chip label={`v${version}`} size="small" color="primary" variant="outlined" />
        ) : (
          <Chip label="Version unavailable" size="small" variant="outlined" />
        )}
      </Box>

      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
        The vjb-debug skill gives Claude Code structured, step-by-step guidance for diagnosing
        failed vJailbreak migrations. It uses your own Claude subscription or API key — zero cost
        to Platform9.
      </Typography>

      <Typography variant="subtitle2" gutterBottom>
        Steps
      </Typography>
      <Box component="ol" sx={{ pl: 2, '& li': { mb: 1 } }}>
        <li>
          <Typography variant="body2">
            Clone or download the skill directory from the repository:
          </Typography>
          <Link href={REPO_SKILL_URL} target="_blank" rel="noopener noreferrer" variant="body2">
            {REPO_SKILL_URL}
          </Link>
        </li>
        <li>
          <Typography variant="body2">
            Copy the <code>.claude/skills/vjb-debug/</code> folder into your project&apos;s{' '}
            <code>.claude/skills/</code> directory (or your global{' '}
            <code>~/.claude/skills/</code> folder for access from any project).
          </Typography>
        </li>
        <li>
          <Typography variant="body2">
            In Claude Code, run the skill by typing{' '}
            <code>/vjb-debug</code> in the chat, or ask Claude to &quot;debug this migration&quot;
            and it will activate automatically when it detects a vJailbreak context.
          </Typography>
        </li>
        <li>
          <Typography variant="body2">
            Provide the migration name when prompted. The skill will collect logs and CRD status
            automatically if you have <code>kubectl</code> configured.
          </Typography>
        </li>
      </Box>

      <Typography variant="body2" color="text.secondary" sx={{ mt: 2 }}>
        <strong>Tip:</strong> Compare <code>v{version ?? '?'}</code> with your local skill
        version to know if an update is available.
      </Typography>
    </Box>
  )
}

function OtherAgentsTab({
  prompt,
  loading,
}: {
  prompt: string
  loading: boolean
}) {
  const [copied, setCopied] = useState(false)

  const handleCopy = () => {
    navigator.clipboard.writeText(prompt).catch(() => {})
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <Box sx={{ maxWidth: 720, mt: 3 }}>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
        Copy this system prompt and paste it into any AI chat tool (ChatGPT, Gemini, a local
        LLM, etc.). The prompt encodes the full structured debugging workflow — just describe
        your failed migration after pasting.
      </Typography>

      <Box sx={{ display: 'flex', justifyContent: 'flex-end', mb: 1 }}>
        <Button
          variant="contained"
          size="small"
          onClick={handleCopy}
          disabled={loading || !prompt}
        >
          {copied ? 'Copied!' : 'Copy System Prompt'}
        </Button>
      </Box>

      <TextField
        multiline
        fullWidth
        minRows={12}
        maxRows={30}
        value={loading ? 'Loading prompt…' : prompt}
        InputProps={{ readOnly: true }}
        sx={{ fontFamily: 'monospace', fontSize: '0.8rem' }}
      />
    </Box>
  )
}

export default function DebugWithAIPage() {
  const [tab, setTab] = useState(0)
  const [data, setData] = useState<DebugPromptResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(false)

  useEffect(() => {
    fetchDebugPrompt()
      .then((resp) => {
        setData(resp)
        setLoading(false)
      })
      .catch(() => {
        setError(true)
        setLoading(false)
      })
  }, [])

  const version = error ? null : (data?.version ?? null)

  return (
    <Box sx={{ p: 3 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1 }}>
        <SmartToyIcon color="primary" />
        <Typography variant="h5">Debug with AI</Typography>
      </Box>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
        Use the vjb-debug skill to diagnose failed migrations with your own AI tool — at no cost
        to Platform9.
      </Typography>

      <Tabs value={tab} onChange={(_, v) => setTab(v as number)}>
        <Tab label="Claude Code" />
        <Tab label="Other Agents" />
      </Tabs>

      {tab === 0 && <ClaudeCodeTab version={version} />}
      {tab === 1 && (
        <OtherAgentsTab prompt={data?.prompt ?? ''} loading={loading} />
      )}
    </Box>
  )
}
