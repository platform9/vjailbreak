export type LogLevel = 'ERROR' | 'WARN' | 'INFO' | 'DEBUG' | 'TRACE' | 'SUCCESS' | 'OTHER'

const LEVEL_RE = /\b(ERROR|FATAL|WARN|WARNING|INFO|DEBUG|TRACE|SUCCESS|SUCCEEDED|FAILED)\b/i

const SOURCE_RE = /^\d{2}:\d{2}:\d{2}[.\d]*\s+\[?(\w[\w-]*)\]?/

/** First level keyword found in the line, upper-cased, or null when the line has none. */
export function extractLevel(line: string): string | null {
  const m = line.match(LEVEL_RE)
  return m ? m[1].toUpperCase() : null
}

/** Collapse level aliases (FATAL, WARNING, SUCCEEDED, …) onto the filterable set. */
export function normalizeLevel(raw: string): LogLevel {
  if (/ERROR|FATAL|FAIL/.test(raw)) return 'ERROR'
  if (/WARN/.test(raw)) return 'WARN'
  if (raw === 'INFO') return 'INFO'
  if (raw === 'DEBUG') return 'DEBUG'
  if (raw === 'TRACE') return 'TRACE'
  if (/SUCCESS|SUCCEED/.test(raw)) return 'SUCCESS'
  return 'OTHER'
}

/** Component name from a `HH:MM:SS [source] …` prefixed line, or null. */
export function extractSource(line: string): string | null {
  const m = line.match(SOURCE_RE)
  return m ? m[1] : null
}

export interface SearchTerm {
  /** Already lower-cased, so callers compare against a lower-cased line. */
  text: string
  negated: boolean
}

// Either an optionally-negated quoted phrase, or a bare whitespace-delimited token.
const TERM_RE = /(-?)"([^"]*)"|(\S+)/g

export function parseSearchQuery(query: string): SearchTerm[] {
  const terms: SearchTerm[] = []
  TERM_RE.lastIndex = 0

  let match: RegExpExecArray | null
  while ((match = TERM_RE.exec(query)) !== null) {
    const [, quoteNegation, phrase, bareToken] = match

    if (phrase !== undefined) {
      if (phrase) terms.push({ text: phrase.toLowerCase(), negated: quoteNegation === '-' })
      continue
    }

    // A lone "-" is not a negation, it is a token.
    const negated = bareToken.length > 1 && bareToken.startsWith('-')
    const text = negated ? bareToken.slice(1) : bareToken
    if (text) terms.push({ text: text.toLowerCase(), negated })
  }

  return terms
}

export function matchesSearch(line: string, terms: SearchTerm[]): boolean {
  if (terms.length === 0) return true

  const haystack = line.toLowerCase()
  for (const term of terms) {
    const present = haystack.includes(term.text)
    if (term.negated === present) return false
  }
  return true
}

export interface LogFilterOptions {
  /** Raw query string straight from the search box. */
  search?: string
  /** A LogLevel, or 'ALL' to disable the level filter. */
  level?: string
  /** A source name, or 'ALL' to disable the source filter. */
  source?: string
}

/**
 * Apply search + level + source filters. Returns the original array (same reference)
 * when nothing is filtered, so callers can rely on referential stability.
 */
export function filterLogLines(
  logs: string[],
  { search = '', level = 'ALL', source = 'ALL' }: LogFilterOptions = {}
): string[] {
  const terms = parseSearchQuery(search)
  if (terms.length === 0 && level === 'ALL' && source === 'ALL') return logs

  return logs.filter((line) => {
    if (level !== 'ALL') {
      const raw = extractLevel(line)
      if (!raw || normalizeLevel(raw) !== level) return false
    }
    if (source !== 'ALL' && extractSource(line) !== source) return false
    return matchesSearch(line, terms)
  })
}
