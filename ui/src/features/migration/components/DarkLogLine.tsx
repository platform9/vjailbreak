import { Box } from '@mui/material'

// ─── Dark log theme — shared by the Migration Details Logs tab and the log drawers ──
export const LOG_BG = '#0d1117'
export const LOG_TEXT = '#c9d1d9'
export const LOG_NUM = '#484f58'
export const LOG_TS = '#8b949e'

const LOG_RE =
  /^(\d{2}:\d{2}:\d{2}[.\d]*)\s+\[?(\w[\w-]*)\]?\s+(ERROR|FATAL|WARN|WARNING|INFO|DEBUG|TRACE|SUCCESS|SUCCEEDED|FAILED)\s+(.*)$/i

export function extractLevel(line: string): string | null {
  const m = line.match(
    /\b(ERROR|FATAL|WARN|WARNING|INFO|DEBUG|TRACE|SUCCESS|SUCCEEDED|FAILED)\b/i
  )
  return m ? m[1].toUpperCase() : null
}

export function normalizeLevel(raw: string): 'ERROR' | 'WARN' | 'INFO' | 'DEBUG' | 'SUCCESS' | 'OTHER' {
  if (/ERROR|FATAL|FAIL/.test(raw)) return 'ERROR'
  if (/WARN/.test(raw)) return 'WARN'
  if (raw === 'INFO') return 'INFO'
  if (raw === 'DEBUG') return 'DEBUG'
  if (/SUCCESS|SUCCEED/.test(raw)) return 'SUCCESS'
  return 'OTHER'
}

export function extractSource(line: string): string | null {
  const m = line.match(/^\d{2}:\d{2}:\d{2}[.\d]*\s+\[?(\w[\w-]*)\]?/)
  return m ? m[1] : null
}

// Level color for text (no badge, just colored text)
export function levelTextColor(lvl: string): string {
  if (/ERROR|FATAL|FAIL/.test(lvl)) return '#f85149'
  if (/WARN/.test(lvl)) return '#e3b341'
  if (lvl === 'INFO') return '#79c0ff'
  if (lvl === 'DEBUG') return '#8b949e'
  if (/SUCCESS|SUCCEED/.test(lvl)) return '#3fb950'
  return '#c9d1d9'
}

export default function DarkLogLine({ line, index }: { line: string; index: number }) {
  const m = line.match(LOG_RE)

  if (m) {
    const [, ts, source, level, msg] = m
    const lvl = level.toUpperCase()
    return (
      <Box
        sx={{
          display: 'flex',
          alignItems: 'baseline',
          gap: '8px',
          py: '1px',
          px: 1.5,
          fontFamily: '"Fira Code","SF Mono","Consolas",monospace',
          fontSize: '0.72rem',
          lineHeight: 1.75,
          '&:hover': { bgcolor: 'rgba(255,255,255,0.04)' },
        }}
      >
        <span
          style={{
            color: LOG_NUM,
            userSelect: 'none',
            minWidth: 28,
            textAlign: 'right',
            flexShrink: 0,
          }}
        >
          {String(index + 1).padStart(3, '0')}
        </span>
        <span style={{ color: LOG_TS, flexShrink: 0 }}>{ts}</span>
        <span style={{ color: '#79c0ff', flexShrink: 0 }}>[{source}]</span>
        <span
          style={{
            color: levelTextColor(lvl),
            fontWeight: 700,
            flexShrink: 0,
            minWidth: 52,
          }}
        >
          {lvl}
        </span>
        <span style={{ color: LOG_TEXT, wordBreak: 'break-all', flex: 1 }}>{msg}</span>
      </Box>
    )
  }

  return (
    <Box
      sx={{
        display: 'flex',
        alignItems: 'baseline',
        gap: '8px',
        py: '1px',
        px: 1.5,
        fontFamily: '"Fira Code","SF Mono","Consolas",monospace',
        fontSize: '0.72rem',
        lineHeight: 1.75,
        '&:hover': { bgcolor: 'rgba(255,255,255,0.04)' },
      }}
    >
      <span
        style={{
          color: LOG_NUM,
          userSelect: 'none',
          minWidth: 28,
          textAlign: 'right',
          flexShrink: 0,
        }}
      >
        {String(index + 1).padStart(3, '0')}
      </span>
      <span style={{ color: LOG_TEXT, wordBreak: 'break-all', flex: 1 }}>{line}</span>
    </Box>
  )
}
