import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import { Migration, Phase } from '../../api/migrations'
import MigrationPhaseDetail from './MigrationPhaseDetail'

const buildSucceededMigration = (creationTimestamp: string, lastConditionTime: string): Migration =>
  ({
    metadata: { creationTimestamp },
    status: {
      phase: Phase.Succeeded,
      conditions: [
        { type: 'Validated', status: 'True', reason: '', message: '', lastTransitionTime: creationTimestamp },
        { type: 'Migrated', status: 'True', reason: '', message: '', lastTransitionTime: lastConditionTime },
      ],
    },
  }) as unknown as Migration

describe('MigrationPhaseDetail — success banner total duration', () => {
  it('includes seconds for a sub-hour total, matching the "Total Elapsed" stat elsewhere (not trimmed)', () => {
    const { container } = render(
      <MigrationPhaseDetail
        migration={buildSucceededMigration('2026-01-01T00:00:00Z', '2026-01-01T00:50:21Z')}
      />
    )

    // Case-sensitive: h/m/s must stay lowercase in the actual text content, even though
    // the surrounding "Migration complete ... total" label is uppercased via CSS
    // textTransform. Checking full textContent since the duration renders in its own
    // nested span (to isolate it from that CSS rule), splitting the text across nodes.
    expect(container.textContent).toContain('Migration complete · 50m 21s total')
    expect(screen.getByText('50m 21s')).toBeInTheDocument()
  })

  it('shows hours and minutes (no seconds) for an over-an-hour total', () => {
    const { container } = render(
      <MigrationPhaseDetail
        migration={buildSucceededMigration('2026-01-01T00:00:00Z', '2026-01-01T05:25:00Z')}
      />
    )

    expect(container.textContent).toContain('Migration complete · 5h 25m total')
    expect(screen.getByText('5h 25m')).toBeInTheDocument()
  })
})
