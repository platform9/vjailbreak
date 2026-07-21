export interface AIAnalyzeRequest {
  migration_name: string
  namespace: string
  question?: string
  conversation_history: Array<{ role: 'user' | 'assistant'; content: string }>
}

export interface GitHubIssue {
  should_open: boolean
  title?: string
  body?: string
  prefill_url?: string
  collect_first?: string[]
}

export interface AIAnalyzeResponse {
  root_cause: string | null
  fix_steps: string[]
  summary: string
  confidence: 'high' | 'medium' | 'low' | 'none'
  doc_references: string[]
  github_issue: GitHubIssue
  raw_response: string
  is_followup?: boolean
}
