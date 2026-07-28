import { describe, expect, it } from 'vitest'
import {
  extractLevel,
  extractSource,
  filterLogLines,
  matchesSearch,
  normalizeLevel,
  parseSearchQuery
} from './logFilter'

const CONTROLLER_LINE =
  '2026-07-27T10:36:49Z\tINFO\tmigration-controller\tReconciling Migration object\t' +
  '{"controller": "migration", "controllerGroup": "vjailbreak.k8s.pf9.io", "controllerKind": "Migration", ' +
  '"Migration": {"name":"migration-windows-11-vtpm-automation-2466-fb547","namespace":"migration-system"}, ' +
  '"namespace": "migration-system", "name": "migration-windows-11-vtpm-automation-2466-fb547", ' +
  '"reconcileID": "935246a7-8480-49a2-a6f0-cbc5527d1c87"}'

const CONTROLLER_ERROR_LINE =
  '2026-07-27T10:36:50Z\tERROR\tmigration-controller\tfailed to reconcile\t' +
  '{"error": "pods \\"v2v-helper\\" already exists", "name": "migration-windows-11-vtpm-automation-2466-fb547"}'

const OTHER_LINE =
  '2026-07-27T12:01:11Z\tINFO\tmigration-controller\tReconciling VMwareCreds object\t' +
  '{"controllerKind": "VMwareCreds", "VMwareCreds": {"name":"vcenter-pune","namespace":"migration-system"}}'

// v2v-helper style line used by the migration details logs tab.
const HELPER_LINE = '12:04:31.221 [v2v-helper] INFO Converting disk 1 of 2'

describe('parseSearchQuery', () => {
  it('returns no terms for blank queries', () => {
    expect(parseSearchQuery('')).toEqual([])
    expect(parseSearchQuery('   ')).toEqual([])
  })

  it('splits on whitespace and lower-cases', () => {
    expect(parseSearchQuery('Foo BAR')).toEqual([
      { text: 'foo', negated: false },
      { text: 'bar', negated: false }
    ])
  })

  it('keeps quoted phrases intact', () => {
    expect(parseSearchQuery('"Reconciling Migration object"')).toEqual([
      { text: 'reconciling migration object', negated: false }
    ])
  })

  it('supports negated tokens and phrases', () => {
    expect(parseSearchQuery('migration -requeuing -"terminal state"')).toEqual([
      { text: 'migration', negated: false },
      { text: 'requeuing', negated: true },
      { text: 'terminal state', negated: true }
    ])
  })

  it('treats an unpaired quote and a lone dash as literal tokens', () => {
    expect(parseSearchQuery('"foo')).toEqual([{ text: '"foo', negated: false }])
    expect(parseSearchQuery('-')).toEqual([{ text: '-', negated: false }])
  })

  it('drops empty quoted phrases', () => {
    expect(parseSearchQuery('""')).toEqual([])
  })

  it('is not affected by a previous call (no sticky regex state)', () => {
    expect(parseSearchQuery('alpha')).toEqual([{ text: 'alpha', negated: false }])
    expect(parseSearchQuery('alpha')).toEqual([{ text: 'alpha', negated: false }])
  })
})

describe('matchesSearch', () => {
  it('matches every line when there are no terms', () => {
    expect(matchesSearch(CONTROLLER_LINE, [])).toBe(true)
  })

  it('requires all positive terms (AND)', () => {
    const terms = parseSearchQuery('reconciling vtpm')
    expect(matchesSearch(CONTROLLER_LINE, terms)).toBe(true)
    expect(matchesSearch(OTHER_LINE, terms)).toBe(false)
  })

  it('excludes lines hitting a negated term', () => {
    const terms = parseSearchQuery('migration-system -vmwarecreds')
    expect(matchesSearch(CONTROLLER_LINE, terms)).toBe(true)
    expect(matchesSearch(OTHER_LINE, terms)).toBe(false)
  })

  it('supports a negation-only query', () => {
    const terms = parseSearchQuery('-vmwarecreds')
    expect(matchesSearch(CONTROLLER_LINE, terms)).toBe(true)
    expect(matchesSearch(OTHER_LINE, terms)).toBe(false)
  })
})

describe('filterLogLines search (issue #2214 regression)', () => {
  const logs = [CONTROLLER_LINE, CONTROLLER_ERROR_LINE, OTHER_LINE]

  it('finds a literal substring buried deep inside a long line', () => {
    // The exact term the reporter typed. Must not silently return zero lines.
    expect(filterLogLines(logs, { search: 'windows-11-vtpm-automation' })).toEqual([
      CONTROLLER_LINE,
      CONTROLLER_ERROR_LINE
    ])
  })

  it('finds a literal substring regardless of how far into the line it sits', () => {
    const term = 'needle-9f2c'
    for (const offset of [0, 40, 120, 400, 1200, 4000]) {
      const line = `${'x'.repeat(offset)}${term} trailing text`
      expect(filterLogLines([line], { search: term }), `offset ${offset}`).toEqual([line])
    }
  })

  it('is case-insensitive in both directions', () => {
    expect(filterLogLines(logs, { search: 'WINDOWS-11-VTPM' })).toHaveLength(2)
    expect(filterLogLines(logs, { search: 'vmwarecreds' })).toEqual([OTHER_LINE])
  })

  it('never invents matches for text that is absent', () => {
    // Fuzzy search used to score near-misses as hits; literal search must not.
    expect(filterLogLines(logs, { search: 'windows-10-vtpm-automation' })).toEqual([])
    expect(filterLogLines([OTHER_LINE], { search: 'ERROR' })).toEqual([])
    expect(filterLogLines(logs, { search: 'zzz-not-present' })).toEqual([])
  })

  it('treats a fully quoted query as a literal phrase', () => {
    expect(filterLogLines(logs, { search: '"Reconciling Migration object"' })).toEqual([
      CONTROLLER_LINE
    ])
    expect(filterLogLines(logs, { search: '"Reconciling object Migration"' })).toEqual([])
  })

  it('matches tokens in any order', () => {
    expect(filterLogLines(logs, { search: 'vtpm reconciling' })).toEqual([
      filterLogLines(logs, { search: 'reconciling vtpm' })[0]
    ])
  })

  it('matches text containing regex metacharacters literally', () => {
    expect(filterLogLines(logs, { search: 'pods \\"v2v-helper\\" already exists' })).toEqual([
      CONTROLLER_ERROR_LINE
    ])
    expect(filterLogLines(logs, { search: '(unclosed' })).toEqual([])
  })

  it('ignores whitespace-only queries and returns the input untouched', () => {
    expect(filterLogLines(logs, { search: '   ' })).toBe(logs)
    expect(filterLogLines(logs)).toBe(logs)
  })

  it('scales to a full buffer without dropping matches', () => {
    const many = Array.from({ length: 5000 }, (_, i) =>
      i % 2 === 0 ? `${CONTROLLER_LINE} seq=${i}` : `${OTHER_LINE} seq=${i}`
    )
    expect(filterLogLines(many, { search: 'windows-11-vtpm-automation' })).toHaveLength(2500)
  })
})

describe('filterLogLines level filter', () => {
  const logs = [CONTROLLER_LINE, CONTROLLER_ERROR_LINE, OTHER_LINE, HELPER_LINE]

  it('passes everything through for ALL', () => {
    expect(filterLogLines(logs, { level: 'ALL' })).toBe(logs)
  })

  it('keeps only the requested level', () => {
    expect(filterLogLines(logs, { level: 'ERROR' })).toEqual([CONTROLLER_ERROR_LINE])
    expect(filterLogLines(logs, { level: 'INFO' })).toEqual([
      CONTROLLER_LINE,
      OTHER_LINE,
      HELPER_LINE
    ])
  })

  it('drops lines with no detectable level', () => {
    expect(filterLogLines(['plain text with no level'], { level: 'INFO' })).toEqual([])
  })

  it('combines with search', () => {
    expect(filterLogLines(logs, { level: 'ERROR', search: 'windows-11-vtpm-automation' })).toEqual([
      CONTROLLER_ERROR_LINE
    ])
    expect(filterLogLines(logs, { level: 'ERROR', search: 'vmwarecreds' })).toEqual([])
  })
})

describe('filterLogLines source filter', () => {
  it('keeps only the requested source', () => {
    const logs = [HELPER_LINE, '12:04:32.100 [virt-v2v] INFO copying disk', CONTROLLER_LINE]
    expect(filterLogLines(logs, { source: 'v2v-helper' })).toEqual([HELPER_LINE])
    expect(filterLogLines(logs, { source: 'ALL' })).toBe(logs)
  })
})

describe('level helpers', () => {
  it('extracts the level from controller and helper line formats', () => {
    expect(extractLevel(CONTROLLER_LINE)).toBe('INFO')
    expect(extractLevel(CONTROLLER_ERROR_LINE)).toBe('ERROR')
    expect(extractLevel(HELPER_LINE)).toBe('INFO')
    expect(extractLevel('no level here')).toBeNull()
  })

  it('normalizes aliases', () => {
    expect(normalizeLevel('FATAL')).toBe('ERROR')
    expect(normalizeLevel('FAILED')).toBe('ERROR')
    expect(normalizeLevel('WARNING')).toBe('WARN')
    expect(normalizeLevel('SUCCEEDED')).toBe('SUCCESS')
    expect(normalizeLevel('INFO')).toBe('INFO')
    expect(normalizeLevel('NOTICE')).toBe('OTHER')
  })

  it('extracts the source only from prefixed lines', () => {
    expect(extractSource(HELPER_LINE)).toBe('v2v-helper')
    expect(extractSource(CONTROLLER_LINE)).toBeNull()
  })
})
